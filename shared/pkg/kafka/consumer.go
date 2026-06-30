package kafka

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

// ---- Consumer configuration -------------------------------------------------

// ConsumerConfig holds settings for the consumer group.
type ConsumerConfig struct {
	// Brokers is the list of broker addresses (host:port).
	Brokers []string
	// GroupID is the Kafka consumer group identifier.
	GroupID string
	// Topics is the list of topics to subscribe to.
	Topics []string
	// ClientID sent on every request. Defaults to "tiktok-consumer".
	ClientID string
	// InitialOffset sets where to start consuming when no offset is committed.
	//   sarama.OffsetNewest (default) or sarama.OffsetOldest.
	InitialOffset int64
	// MaxRetries before a failed message is forwarded to the DLQ. Defaults to 3.
	MaxRetries int
	// RetryBackoff between handler retries. Defaults to 500 ms.
	RetryBackoff time.Duration
	// DLQTopic is the dead-letter queue topic for messages that exhaust retries.
	// Empty string disables DLQ.
	DLQTopic string
	// DLQProducer must be supplied when DLQTopic is set.
	DLQProducer *Producer
	// SessionTimeout is the consumer group session timeout. Defaults to 30 s.
	SessionTimeout time.Duration
	// HeartbeatInterval is the consumer group heartbeat interval. Defaults to 3 s.
	HeartbeatInterval time.Duration
}

func (c *ConsumerConfig) defaults() {
	if c.ClientID == "" {
		c.ClientID = "tiktok-consumer"
	}
	if c.InitialOffset == 0 {
		c.InitialOffset = sarama.OffsetNewest
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.RetryBackoff == 0 {
		c.RetryBackoff = 500 * time.Millisecond
	}
	if c.SessionTimeout == 0 {
		c.SessionTimeout = 30 * time.Second
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 3 * time.Second
	}
}

// ---- Handler ----------------------------------------------------------------

// Handler is the callback invoked for each Kafka message.
// Returning a non-nil error will trigger a retry (up to MaxRetries).
type Handler func(ctx context.Context, msg *sarama.ConsumerMessage) error

// ---- Consumer ---------------------------------------------------------------

// Consumer wraps a sarama ConsumerGroup and provides graceful shutdown,
// retry logic, and optional dead-letter queue forwarding.
type Consumer struct {
	cfg     ConsumerConfig
	group   sarama.ConsumerGroup
	handler Handler
	wg      sync.WaitGroup
}

// NewConsumer creates a Consumer. Call Run to start processing.
func NewConsumer(cfg ConsumerConfig, handler Handler) (*Consumer, error) {
	cfg.defaults()

	sc := sarama.NewConfig()
	sc.ClientID = cfg.ClientID
	sc.Consumer.Group.Session.Timeout = cfg.SessionTimeout
	sc.Consumer.Group.Heartbeat.Interval = cfg.HeartbeatInterval
	sc.Consumer.Offsets.Initial = cfg.InitialOffset
	sc.Consumer.Return.Errors = true
	sc.Consumer.Offsets.AutoCommit.Enable = false // We commit manually after success.

	if err := sc.Validate(); err != nil {
		return nil, fmt.Errorf("kafka: invalid consumer config: %w", err)
	}

	group, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, sc)
	if err != nil {
		return nil, fmt.Errorf("kafka: creating consumer group: %w", err)
	}

	return &Consumer{
		cfg:     cfg,
		group:   group,
		handler: handler,
	}, nil
}

// Run starts the consumer group session loop. It blocks until ctx is cancelled,
// after which it drains in-flight messages and returns.
func (c *Consumer) Run(ctx context.Context) error {
	h := &groupHandler{
		cfg:     c.cfg,
		handler: c.handler,
	}

	// Drain consumer group errors in the background.
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for err := range c.group.Errors() {
			log.Printf("kafka consumer group error: %v", err)
		}
	}()

	var retErr error
	for {
		// Consume re-joins on rebalance automatically.
		if err := c.group.Consume(ctx, c.cfg.Topics, h); err != nil {
			retErr = fmt.Errorf("kafka: consume loop: %w", err)
			break
		}
		if ctx.Err() != nil {
			break
		}
	}

	if err := c.group.Close(); err != nil {
		retErr = fmt.Errorf("kafka: closing consumer group: %w", err)
	}
	c.wg.Wait()
	return retErr
}

// Close shuts down the consumer group immediately. Prefer cancelling the
// context passed to Run for a graceful shutdown.
func (c *Consumer) Close() error {
	return c.group.Close()
}

// ---- sarama ConsumerGroupHandler implementation -----------------------------

type groupHandler struct {
	cfg     ConsumerConfig
	handler Handler
}

// Setup is called at the start of a new consumer group session.
func (h *groupHandler) Setup(sarama.ConsumerGroupSession) error { return nil }

// Cleanup is called at the end of a consumer group session.
func (h *groupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim processes messages from a single partition claim.
func (h *groupHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for msg := range claim.Messages() {
		if err := h.processWithRetry(session.Context(), msg); err != nil {
			// Forward to DLQ on exhausted retries.
			h.sendToDLQ(msg, err)
		}
		// Always mark the message so we don't reprocess it on restart,
		// even if we sent it to the DLQ.
		session.MarkMessage(msg, "")
		session.Commit()
	}
	return nil
}

// processWithRetry calls the user Handler up to MaxRetries times.
func (h *groupHandler) processWithRetry(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var lastErr error
	for attempt := 0; attempt <= h.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(h.cfg.RetryBackoff):
			}
		}
		if err := h.handler(ctx, msg); err != nil {
			lastErr = err
			log.Printf("kafka: handler error (attempt %d/%d) on topic %q partition %d offset %d: %v",
				attempt+1, h.cfg.MaxRetries+1,
				msg.Topic, msg.Partition, msg.Offset, err)
			continue
		}
		return nil // success
	}
	return fmt.Errorf("kafka: message failed after %d attempts: %w", h.cfg.MaxRetries+1, lastErr)
}

// sendToDLQ forwards a message to the dead-letter queue topic.
func (h *groupHandler) sendToDLQ(msg *sarama.ConsumerMessage, reason error) {
	if h.cfg.DLQTopic == "" || h.cfg.DLQProducer == nil {
		return
	}
	dlqMsg := &Message{
		Topic: h.cfg.DLQTopic,
		Key:   msg.Key,
		Value: msg.Value,
		Headers: []sarama.RecordHeader{
			{Key: []byte("original-topic"), Value: []byte(msg.Topic)},
			{Key: []byte("original-partition"), Value: []byte(fmt.Sprintf("%d", msg.Partition))},
			{Key: []byte("original-offset"), Value: []byte(fmt.Sprintf("%d", msg.Offset))},
			{Key: []byte("error"), Value: []byte(reason.Error())},
		},
	}
	if err := h.cfg.DLQProducer.Send(dlqMsg); err != nil {
		log.Printf("kafka: failed to send message to DLQ %q: %v", h.cfg.DLQTopic, err)
	}
}
