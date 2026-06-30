package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/tiktok-clone/api-gateway/internal/config"
)

// AuditEvent is the structured log entry written to Kafka.
type AuditEvent struct {
	// Core fields
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"` // "http_request"

	// Request metadata
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Query        string            `json:"query,omitempty"`
	StatusCode   int               `json:"status_code"`
	Latency      int64             `json:"latency_ms"`
	RequestSize  int64             `json:"request_size_bytes"`
	ResponseSize int64             `json:"response_size_bytes"`
	Headers      map[string]string `json:"headers,omitempty"`

	// Client info
	ClientIP  string `json:"client_ip"`
	UserAgent string `json:"user_agent"`

	// Identity (populated when authenticated)
	UserID    string `json:"user_id,omitempty"`
	UserRole  string `json:"user_role,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`

	// Routing
	TargetService string `json:"target_service,omitempty"`

	// Security flags
	WAFBlocked  bool   `json:"waf_blocked"`
	RateLimited bool   `json:"rate_limited"`
	BlockReason string `json:"block_reason,omitempty"`

	// Error info
	Error string `json:"error,omitempty"`
}

// responseWriter wraps gin.ResponseWriter to capture status code and body size.
type responseWriter struct {
	gin.ResponseWriter
	statusCode int
	bodySize   int64
}

func newResponseWriter(w gin.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bodySize += int64(n)
	return n, err
}

func (rw *responseWriter) WriteString(s string) (int, error) {
	n, err := rw.ResponseWriter.WriteString(s)
	rw.bodySize += int64(n)
	return n, err
}

// AuditLogger sends structured audit events to a Kafka topic.
type AuditLogger struct {
	producer sarama.AsyncProducer
	topic    string
	pool     sync.Pool // reuse AuditEvent allocations
	errCh    chan error
	wg       sync.WaitGroup
}

// NewAuditLogger creates a new AuditLogger connected to Kafka.
func NewAuditLogger(cfg *config.KafkaConfig) (*AuditLogger, error) {
	scfg := sarama.NewConfig()
	scfg.Version = sarama.V2_8_0_0
	scfg.Producer.RequiredAcks = sarama.WaitForLocal
	scfg.Producer.Compression = sarama.CompressionSnappy
	scfg.Producer.Flush.Frequency = 100 * time.Millisecond
	scfg.Producer.Flush.MaxMessages = 500
	scfg.Producer.Return.Errors = true
	scfg.Producer.Return.Successes = false // fire-and-forget for audit logs

	// SASL configuration
	if cfg.SASLUsername != "" {
		scfg.Net.SASL.Enable = true
		scfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		if strings.EqualFold(cfg.SASLMechanism, "SCRAM-SHA-256") {
			scfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		} else if strings.EqualFold(cfg.SASLMechanism, "SCRAM-SHA-512") {
			scfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		}
		scfg.Net.SASL.User = cfg.SASLUsername
		scfg.Net.SASL.Password = cfg.SASLPassword
	}

	if cfg.TLSEnabled {
		scfg.Net.TLS.Enable = true
	}

	producer, err := sarama.NewAsyncProducer(cfg.Brokers, scfg)
	if err != nil {
		return nil, fmt.Errorf("creating Kafka producer: %w", err)
	}

	al := &AuditLogger{
		producer: producer,
		topic:    cfg.AuditTopic,
		pool: sync.Pool{
			New: func() interface{} { return &AuditEvent{} },
		},
		errCh: make(chan error, 100),
	}

	// Drain the Errors channel to prevent deadlock.
	al.wg.Add(1)
	go al.drainErrors()

	return al, nil
}

func (al *AuditLogger) drainErrors() {
	defer al.wg.Done()
	for err := range al.producer.Errors() {
		log.Printf("[audit] Kafka producer error: %v", err)
	}
}

// Close flushes and closes the Kafka producer gracefully.
func (al *AuditLogger) Close() error {
	err := al.producer.Close()
	al.wg.Wait()
	return err
}

// Middleware returns a gin.HandlerFunc that captures request/response metadata
// and publishes an AuditEvent to Kafka after the request completes.
func (al *AuditLogger) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Wrap the response writer to capture status and size.
		rw := newResponseWriter(c.Writer)
		c.Writer = rw

		// Capture request body size without reading it (body may be large).
		var requestSize int64
		if c.Request.ContentLength > 0 {
			requestSize = c.Request.ContentLength
		}

		// For small bodies (< 8KB) record the actual size by peeking.
		if c.Request.Body != nil && requestSize == 0 {
			bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, 8192))
			if err == nil {
				requestSize = int64(len(bodyBytes))
				c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		// Process the request.
		c.Next()

		latency := time.Since(start)

		// Build the audit event.
		event := al.pool.Get().(*AuditEvent)
		defer al.pool.Put(event)

		// Reset the struct for reuse.
		*event = AuditEvent{}

		event.EventID = generateEventID(c)
		event.Timestamp = start.UTC()
		event.EventType = "http_request"
		event.Method = c.Request.Method
		event.Path = c.FullPath()
		if event.Path == "" {
			event.Path = c.Request.URL.Path
		}
		event.Query = c.Request.URL.RawQuery
		event.StatusCode = rw.statusCode
		event.Latency = latency.Milliseconds()
		event.RequestSize = requestSize
		event.ResponseSize = rw.bodySize
		event.ClientIP = c.ClientIP()
		event.UserAgent = c.Request.UserAgent()
		event.TargetService = c.GetString("target_service")

		// Capture identity from Gin context (populated by JWT middleware).
		if uid, ok := c.Get(GinKeyUserID); ok {
			event.UserID = fmt.Sprintf("%v", uid)
		}
		if role, ok := c.Get(GinKeyRole); ok {
			event.UserRole = fmt.Sprintf("%v", role)
		}
		if did, ok := c.Get(GinKeyDeviceID); ok {
			event.DeviceID = fmt.Sprintf("%v", did)
		}
		if claims, ok := c.Get(GinKeyClaims); ok {
			if tc, ok := claims.(*TikTokClaims); ok {
				event.SessionID = tc.SessionID
			}
		}

		// Security flags.
		if wafBlock := c.GetHeader("X-WAF-Block-Reason"); wafBlock != "" {
			event.WAFBlocked = true
			event.BlockReason = wafBlock
		}
		if rw.statusCode == http.StatusTooManyRequests {
			event.RateLimited = true
		}

		// Capture relevant security-related request headers.
		event.Headers = captureHeaders(c.Request)

		// First error from Gin's error chain.
		if len(c.Errors) > 0 {
			event.Error = c.Errors.Last().Error()
		}

		// Publish to Kafka (non-blocking).
		al.publish(event)
	}
}

// publish serializes the event and sends it to the Kafka topic asynchronously.
func (al *AuditLogger) publish(event *AuditEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[audit] failed to marshal event: %v", err)
		return
	}

	// Use UserID as partition key so a user's events land on the same partition.
	var key sarama.Encoder
	if event.UserID != "" {
		key = sarama.StringEncoder(event.UserID)
	} else {
		key = sarama.StringEncoder(event.ClientIP)
	}

	// Non-blocking send; drop if channel is full (audit logs must not block API).
	select {
	case al.producer.Input() <- &sarama.ProducerMessage{
		Topic:     al.topic,
		Key:       key,
		Value:     sarama.ByteEncoder(data),
		Timestamp: event.Timestamp,
	}:
	default:
		log.Printf("[audit] Kafka input channel full; dropping event %s", event.EventID)
	}
}

// captureHeaders returns a map of security-relevant request headers.
func captureHeaders(r *http.Request) map[string]string {
	keep := []string{
		"X-Forwarded-For",
		"X-Real-IP",
		"X-Request-ID",
		"Content-Type",
		"Origin",
		"Referer",
	}
	headers := make(map[string]string, len(keep))
	for _, name := range keep {
		if v := r.Header.Get(name); v != "" {
			headers[name] = v
		}
	}
	return headers
}

// generateEventID builds a unique event identifier from request metadata.
func generateEventID(c *gin.Context) string {
	// Prefer an incoming request ID from a tracing header.
	if rid := c.GetHeader("X-Request-ID"); rid != "" {
		return rid
	}
	return fmt.Sprintf("%d-%s-%s",
		time.Now().UnixNano(),
		c.ClientIP(),
		c.Request.URL.Path,
	)
}
