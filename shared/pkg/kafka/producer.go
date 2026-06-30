// Package kafka provides Sarama-based Kafka producer and consumer helpers for
// the TikTok-clone platform.
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
)

// ---- Producer configuration -------------------------------------------------

// ProducerConfig holds settings for the async producer.
type ProducerConfig struct {
	// Brokers is the list of broker addresses (host:port).
	Brokers []string
	// ClientID is sent to Kafka on every request. Defaults to "tiktok-producer".
	ClientID string
	// RequiredAcks controls when the broker acknowledges a message.
	//   0 = NoResponse, 1 = WaitForLocal (default), -1 = WaitForAll
	RequiredAcks sarama.RequiredAcks
	// MaxRetries is the number of times to retry sending a failed message.
	// Defaults to 5.
	MaxRetries int
	// RetryBackoff is the wait between retries. Defaults to 250 ms.
	RetryBackoff time.Duration
	// CompressionCodec sets the compression algorithm. Defaults to Snappy.
	CompressionCodec sarama.CompressionCodec
	// MaxMessageBytes is the max size of a single message. Defaults to 1 MiB.
	MaxMessageBytes int
	// Idempotent enables exactly-once delivery at the producer level.
	Idempotent bool
	// FlushFrequency is how often the producer flushes buffered messages.
	// Defaults to 100 ms.
	FlushFrequency time.Duration
	// FlushMessages is the max number of buffered messages before a flush.
	FlushMessages int
	// DLQTopic is the dead-letter queue topic. Messages that exceed MaxRetries
	// are forwarded here. Empty string disables DLQ.
	DLQTopic string
}

func (c *ProducerConfig) defaults() {
	if c.ClientID == "" {
		c.ClientID = "tiktok-producer"
	}
	if c.RequiredAcks == 0 {
		c.RequiredAcks = sarama.WaitForLocal
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 5
	}
	if c.RetryBackoff == 0 {
		c.RetryBackoff = 250 * time.Millisecond
	}
	if c.CompressionCodec == sarama.CompressionNone {
		c.CompressionCodec = sarama.CompressionSnappy
	}
	if c.MaxMessageBytes == 0 {
		c.MaxMessageBytes = 1 << 20 // 1 MiB
	}
	if c.FlushFrequency == 0 {
		c.FlushFrequency = 100 * time.Millisecond
	}
	if c.FlushMessages == 0 {
		c.FlushMessages = 100
	}
}

// ---- Message ----------------------------------------------------------------

// Message is the envelope sent to Kafka.
type Message struct {
	// Topic is the destination Kafka topic.
	Topic string
	// Key is the optional partition key (nil = random partition).
	Key []byte
	// Value is the raw message payload.
	Value []byte
	// Headers are optional Kafka message headers.
	Headers []sarama.RecordHeader
	// Metadata is passed through to the success/error channel and can be used
	// to correlate acknowledgements with application state.
	Metadata interface{}
}

// NewJSONMessage creates a Message by JSON-encoding value.
func NewJSONMessage(topic string, key []byte, value interface{}, headers ...sarama.RecordHeader) (*Message, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("kafka: marshalling message: %w", err)
	}
	return &Message{
		Topic:   topic,
		Key:     key,
		Value:   b,
		Headers: headers,
	}, nil
}

// ---- Producer ---------------------------------------------------------------

// Producer is an asynchronous Kafka producer with retry and DLQ support.
type Producer struct {
	cfg      ProducerConfig
	producer sarama.AsyncProducer
	// dlq is a synchronous producer used only for DLQ writes.
	dlq    sarama.SyncProducer
	stopCh chan struct{}
	doneCh chan struct{}
	errors chan *ProducerError
}

// ProducerError carries a failed message together with the underlying error.
type ProducerError struct {
	Msg *Message
	Err error
}

func (e *ProducerError) Error() string {
	return fmt.Sprintf("kafka producer error on topic %q: %v", e.Msg.Topic, e.Err)
}

// NewProducer creates and starts an async Kafka producer.
func NewProducer(cfg ProducerConfig) (*Producer, error) {
	cfg.defaults()

	sc, err := buildProducerConfig(cfg)
	if err != nil {
		return nil, err
	}

	ap, err := sarama.NewAsyncProducer(cfg.Brokers, sc)
	if err != nil {
		return nil, fmt.Errorf("kafka: creating async producer: %w", err)
	}

	p := &Producer{
		cfg:      cfg,
		producer: ap,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		errors:   make(chan *ProducerError, 256),
	}

	// Create a DLQ synchronous producer if configured.
	if cfg.DLQTopic != "" {
		dlqCfg := sarama.NewConfig()
		dlqCfg.Producer.RequiredAcks = sarama.WaitForAll
		dlqCfg.Producer.Return.Successes = true
		sp, err := sarama.NewSyncProducer(cfg.Brokers, dlqCfg)
		if err != nil {
			_ = ap.Close()
			return nil, fmt.Errorf("kafka: creating DLQ producer: %w", err)
		}
		p.dlq = sp
	}

	go p.handleResponses()
	return p, nil
}

// Send enqueues a message for async delivery.
// Returns ErrProducerClosed if the producer has been shut down.
func (p *Producer) Send(msg *Message) error {
	select {
	case <-p.stopCh:
		return ErrProducerClosed
	default:
	}

	pm := &sarama.ProducerMessage{
		Topic:    msg.Topic,
		Value:    sarama.ByteEncoder(msg.Value),
		Headers:  msg.Headers,
		Metadata: msg.Metadata,
	}
	if len(msg.Key) > 0 {
		pm.Key = sarama.ByteEncoder(msg.Key)
	}

	select {
	case p.producer.Input() <- pm:
		return nil
	case <-p.stopCh:
		return ErrProducerClosed
	}
}

// SendJSON is a convenience method that JSON-encodes value before sending.
func (p *Producer) SendJSON(topic string, key []byte, value interface{}) error {
	msg, err := NewJSONMessage(topic, key, value)
	if err != nil {
		return err
	}
	return p.Send(msg)
}

// SendSync sends a message and waits for acknowledgement (blocks until the
// broker confirms or an error is returned). Uses a temporary sync producer.
func (p *Producer) SendSync(ctx context.Context, msg *Message) (partition int32, offset int64, err error) {
	sc, err := buildProducerConfig(p.cfg)
	if err != nil {
		return 0, 0, err
	}
	sc.Producer.Return.Successes = true
	sp, err := sarama.NewSyncProducer(p.cfg.Brokers, sc)
	if err != nil {
		return 0, 0, fmt.Errorf("kafka: creating sync producer: %w", err)
	}
	defer sp.Close()

	pm := &sarama.ProducerMessage{
		Topic:   msg.Topic,
		Value:   sarama.ByteEncoder(msg.Value),
		Headers: msg.Headers,
	}
	if len(msg.Key) > 0 {
		pm.Key = sarama.ByteEncoder(msg.Key)
	}

	return sp.SendMessage(pm)
}

// Errors returns a channel of delivery failures that the caller should drain.
func (p *Producer) Errors() <-chan *ProducerError { return p.errors }

// Close shuts down the producer gracefully, flushing all buffered messages.
func (p *Producer) Close() error {
	close(p.stopCh)
	<-p.doneCh
	var errs []error
	if err := p.producer.Close(); err != nil {
		errs = append(errs, err)
	}
	if p.dlq != nil {
		if err := p.dlq.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("kafka: closing producer: %v", errs)
	}
	return nil
}

// handleResponses drains the async producer's success and error channels.
func (p *Producer) handleResponses() {
	defer close(p.doneCh)
	for {
		select {
		case _, ok := <-p.producer.Successes():
			if !ok {
				return
			}
			// Success — nothing to do; the async producer already confirmed
			// delivery.

		case perr, ok := <-p.producer.Errors():
			if !ok {
				return
			}
			val, _ := perr.Msg.Value.Encode()
			failed := &Message{
				Topic:    perr.Msg.Topic,
				Value:    val,
				Metadata: perr.Msg.Metadata,
			}
			if perr.Msg.Key != nil {
				k, _ := perr.Msg.Key.Encode()
				failed.Key = k
			}

			// Forward to DLQ if configured.
			if p.dlq != nil && p.cfg.DLQTopic != "" {
				dlqMsg := &sarama.ProducerMessage{
					Topic: p.cfg.DLQTopic,
					Value: sarama.ByteEncoder(val),
					Headers: []sarama.RecordHeader{
						{Key: []byte("original-topic"), Value: []byte(perr.Msg.Topic)},
						{Key: []byte("error"), Value: []byte(perr.Err.Error())},
					},
				}
				_, _, _ = p.dlq.SendMessage(dlqMsg)
			}

			// Surface the error to the caller.
			select {
			case p.errors <- &ProducerError{Msg: failed, Err: perr.Err}:
			default:
				// Drop if the caller isn't reading errors; avoid blocking.
			}
		}
	}
}

// ---- helpers ----------------------------------------------------------------

func buildProducerConfig(cfg ProducerConfig) (*sarama.Config, error) {
	sc := sarama.NewConfig()
	sc.ClientID = cfg.ClientID
	sc.Producer.RequiredAcks = cfg.RequiredAcks
	sc.Producer.Retry.Max = cfg.MaxRetries
	sc.Producer.Retry.Backoff = cfg.RetryBackoff
	sc.Producer.Compression = cfg.CompressionCodec
	sc.Producer.MaxMessageBytes = cfg.MaxMessageBytes
	sc.Producer.Return.Successes = true
	sc.Producer.Return.Errors = true
	sc.Producer.Flush.Frequency = cfg.FlushFrequency
	sc.Producer.Flush.Messages = cfg.FlushMessages
	if cfg.Idempotent {
		sc.Producer.Idempotent = true
		sc.Net.MaxOpenRequests = 1
		sc.Producer.RequiredAcks = sarama.WaitForAll
	}
	if err := sc.Validate(); err != nil {
		return nil, fmt.Errorf("kafka: invalid producer config: %w", err)
	}
	return sc, nil
}

// ---- Sentinel errors --------------------------------------------------------

var (
	// ErrProducerClosed is returned when Send is called after Close.
	ErrProducerClosed = errors.New("kafka: producer is closed")
)
