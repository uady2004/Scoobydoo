package esclient

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaConfig holds Kafka consumer configuration.
type KafkaConfig struct {
	Brokers        []string
	GroupID        string
	Topics         []string
	MinBytes       int
	MaxBytes       int
	CommitInterval time.Duration
	StartOffset    int64 // kafka.FirstOffset or kafka.LastOffset
	MaxAttempts    int
	DialTimeout    time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

// DefaultKafkaConfig returns sensible Kafka defaults.
func DefaultKafkaConfig() KafkaConfig {
	return KafkaConfig{
		MinBytes:       1e3,        // 1 KB
		MaxBytes:       10e6,       // 10 MB
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
		MaxAttempts:    3,
		DialTimeout:    10 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}
}

// IndexerEvent represents a Kafka event that triggers an ES index operation.
type IndexerEvent struct {
	EventType string          `json:"event_type"` // created | updated | deleted
	Index     string          `json:"index"`
	ID        string          `json:"id"`
	Routing   string          `json:"routing,omitempty"`
	Document  json.RawMessage `json:"document,omitempty"`
	Partial   json.RawMessage `json:"partial,omitempty"`  // for updates
	Version   int64           `json:"version,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// IndexerMetrics tracks indexer operational statistics.
type IndexerMetrics struct {
	EventsConsumed  atomic.Int64
	EventsIndexed   atomic.Int64
	EventsDropped   atomic.Int64
	EventsErrored   atomic.Int64
	BatchesFlushed  atomic.Int64
	LastFlushAt     atomic.Int64 // UnixNano
}

// RealTimeIndexer consumes Kafka events and writes them to Elasticsearch.
type RealTimeIndexer struct {
	client      *SearchClient
	kafkaCfg    KafkaConfig
	readers     []*kafka.Reader
	bulkIndexer *ConcurrentBulkIndexer
	metrics     IndexerMetrics
	transforms  map[string]TransformFunc
	filters     []FilterFunc
	logger      *zap.Logger
	mu          sync.RWMutex
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// TransformFunc transforms a raw IndexerEvent before indexing.
// Return nil document to skip the event.
type TransformFunc func(event IndexerEvent) (index, id string, doc interface{}, err error)

// FilterFunc returns false to drop the event before processing.
type FilterFunc func(event IndexerEvent) bool

// RealTimeIndexerConfig configures the RealTimeIndexer.
type RealTimeIndexerConfig struct {
	Kafka        KafkaConfig
	BulkIndexer  BulkIndexerConfig
	Transforms   map[string]TransformFunc // keyed by event.Index
	Filters      []FilterFunc
	WorkerCount  int
	ErrorBackoff time.Duration
}

// NewRealTimeIndexer builds and starts the RealTimeIndexer.
func NewRealTimeIndexer(
	client *SearchClient,
	cfg RealTimeIndexerConfig,
	logger *zap.Logger,
) (*RealTimeIndexer, error) {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.ErrorBackoff <= 0 {
		cfg.ErrorBackoff = 500 * time.Millisecond
	}

	cfg.BulkIndexer.OnError = func(ctx context.Context, err error) {
		logger.Error("bulk indexer error", zap.Error(err))
	}
	cfg.BulkIndexer.OnSuccess = func(ctx context.Context, item BulkItemResult) {
		logger.Debug("bulk indexed", zap.String("index", item.Index), zap.String("id", item.ID))
	}

	bulkIndexer := NewConcurrentBulkIndexer(client, cfg.BulkIndexer)

	ri := &RealTimeIndexer{
		client:      client,
		kafkaCfg:    cfg.Kafka,
		bulkIndexer: bulkIndexer,
		transforms:  cfg.Transforms,
		filters:     cfg.Filters,
		logger:      logger,
	}

	if len(ri.transforms) == 0 {
		ri.transforms = defaultTransforms()
	}

	return ri, nil
}

// defaultTransforms provides pass-through transforms for known indices.
func defaultTransforms() map[string]TransformFunc {
	passthrough := func(event IndexerEvent) (string, string, interface{}, error) {
		if event.Document == nil {
			return "", "", nil, fmt.Errorf("empty document for event %s/%s", event.Index, event.ID)
		}
		var doc map[string]interface{}
		if err := json.Unmarshal(event.Document, &doc); err != nil {
			return "", "", nil, fmt.Errorf("unmarshal document: %w", err)
		}
		return event.Index, event.ID, doc, nil
	}

	return map[string]TransformFunc{
		"users":    passthrough,
		"videos":   passthrough,
		"hashtags": passthrough,
		"products": passthrough,
		"sounds":   passthrough,
	}
}

// Start begins consuming Kafka topics in background goroutines.
func (ri *RealTimeIndexer) Start(ctx context.Context) error {
	ctx, ri.cancel = context.WithCancel(ctx)

	for _, topic := range ri.kafkaCfg.Topics {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:        ri.kafkaCfg.Brokers,
			GroupID:        ri.kafkaCfg.GroupID,
			Topic:          topic,
			MinBytes:       ri.kafkaCfg.MinBytes,
			MaxBytes:       ri.kafkaCfg.MaxBytes,
			CommitInterval: ri.kafkaCfg.CommitInterval,
			StartOffset:    ri.kafkaCfg.StartOffset,
			MaxAttempts:    ri.kafkaCfg.MaxAttempts,
			Logger:         kafka.LoggerFunc(func(msg string, args ...interface{}) {
				ri.logger.Debug(fmt.Sprintf(msg, args...))
			}),
			ErrorLogger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
				ri.logger.Error(fmt.Sprintf(msg, args...))
			}),
		})

		ri.mu.Lock()
		ri.readers = append(ri.readers, reader)
		ri.mu.Unlock()

		ri.wg.Add(1)
		go ri.consumeTopic(ctx, reader, topic)
	}

	ri.logger.Info("real-time indexer started",
		zap.Strings("topics", ri.kafkaCfg.Topics),
		zap.String("group_id", ri.kafkaCfg.GroupID),
	)

	return nil
}

// Stop gracefully shuts down the indexer, flushing all pending events.
func (ri *RealTimeIndexer) Stop(ctx context.Context) error {
	if ri.cancel != nil {
		ri.cancel()
	}

	done := make(chan struct{})
	go func() {
		ri.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		ri.logger.Warn("indexer stop timed out, some events may be lost")
	}

	ri.mu.RLock()
	readers := ri.readers
	ri.mu.RUnlock()

	var lastErr error
	for _, r := range readers {
		if err := r.Close(); err != nil {
			lastErr = err
			ri.logger.Error("close kafka reader", zap.Error(err))
		}
	}

	if err := ri.bulkIndexer.Close(ctx); err != nil {
		lastErr = err
		ri.logger.Error("close bulk indexer", zap.Error(err))
	}

	return lastErr
}

// Metrics returns a snapshot of current indexer statistics.
func (ri *RealTimeIndexer) Metrics() map[string]int64 {
	return map[string]int64{
		"events_consumed":  ri.metrics.EventsConsumed.Load(),
		"events_indexed":   ri.metrics.EventsIndexed.Load(),
		"events_dropped":   ri.metrics.EventsDropped.Load(),
		"events_errored":   ri.metrics.EventsErrored.Load(),
		"batches_flushed":  ri.metrics.BatchesFlushed.Load(),
		"last_flush_at":    ri.metrics.LastFlushAt.Load(),
	}
}

// consumeTopic runs in its own goroutine and processes messages from a single Kafka topic.
func (ri *RealTimeIndexer) consumeTopic(ctx context.Context, reader *kafka.Reader, topic string) {
	defer ri.wg.Done()

	ri.logger.Info("starting topic consumer", zap.String("topic", topic))

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				ri.logger.Info("topic consumer shutting down", zap.String("topic", topic))
				return
			}
			ri.logger.Error("read kafka message",
				zap.String("topic", topic),
				zap.Error(err),
			)
			ri.metrics.EventsErrored.Add(1)
			continue
		}

		ri.metrics.EventsConsumed.Add(1)

		if err := ri.processMessage(ctx, msg); err != nil {
			ri.logger.Error("process message",
				zap.String("topic", topic),
				zap.ByteString("key", msg.Key),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
			ri.metrics.EventsErrored.Add(1)
		}
	}
}

// processMessage decodes a Kafka message and routes it to the appropriate ES operation.
func (ri *RealTimeIndexer) processMessage(ctx context.Context, msg kafka.Message) error {
	var event IndexerEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshal kafka message: %w", err)
	}

	// Apply filters
	for _, filter := range ri.filters {
		if !filter(event) {
			ri.metrics.EventsDropped.Add(1)
			return nil
		}
	}

	switch event.EventType {
	case "created", "indexed":
		return ri.handleUpsert(ctx, event, BulkActionIndex)
	case "updated":
		return ri.handleUpdate(ctx, event)
	case "deleted":
		return ri.handleDelete(ctx, event)
	default:
		ri.logger.Warn("unknown event type",
			zap.String("event_type", event.EventType),
			zap.String("index", event.Index),
			zap.String("id", event.ID),
		)
		ri.metrics.EventsDropped.Add(1)
		return nil
	}
}

// handleUpsert indexes or creates a document.
func (ri *RealTimeIndexer) handleUpsert(ctx context.Context, event IndexerEvent, action BulkAction) error {
	ri.mu.RLock()
	transform, ok := ri.transforms[event.Index]
	ri.mu.RUnlock()

	var (
		index string
		id    string
		doc   interface{}
		err   error
	)

	if ok {
		index, id, doc, err = transform(event)
	} else {
		// Default: pass document through as-is
		index = event.Index
		id = event.ID
		var raw map[string]interface{}
		if err = json.Unmarshal(event.Document, &raw); err != nil {
			return fmt.Errorf("unmarshal default document: %w", err)
		}
		doc = raw
	}

	if err != nil {
		ri.metrics.EventsErrored.Add(1)
		return fmt.Errorf("transform event: %w", err)
	}
	if doc == nil {
		ri.metrics.EventsDropped.Add(1)
		return nil
	}

	item := BulkItem{
		Action:   action,
		Index:    index,
		ID:       id,
		Routing:  event.Routing,
		Document: doc,
	}

	if err := ri.bulkIndexer.Add(ctx, item); err != nil {
		return fmt.Errorf("enqueue bulk item: %w", err)
	}

	ri.metrics.EventsIndexed.Add(1)
	return nil
}

// handleUpdate partially updates a document.
func (ri *RealTimeIndexer) handleUpdate(ctx context.Context, event IndexerEvent) error {
	// If partial is provided, use it; otherwise fall back to full upsert
	if len(event.Partial) > 0 {
		var partial map[string]interface{}
		if err := json.Unmarshal(event.Partial, &partial); err != nil {
			return fmt.Errorf("unmarshal partial update: %w", err)
		}

		retryCount := 3
		item := BulkItem{
			Action:          BulkActionUpdate,
			Index:           event.Index,
			ID:              event.ID,
			Routing:         event.Routing,
			Document:        partial,
			RetryOnConflict: &retryCount,
		}

		if err := ri.bulkIndexer.Add(ctx, item); err != nil {
			return fmt.Errorf("enqueue update item: %w", err)
		}
		ri.metrics.EventsIndexed.Add(1)
		return nil
	}

	// Fall back to full document upsert
	return ri.handleUpsert(ctx, event, BulkActionIndex)
}

// handleDelete removes a document from the index.
func (ri *RealTimeIndexer) handleDelete(ctx context.Context, event IndexerEvent) error {
	item := BulkItem{
		Action:  BulkActionDelete,
		Index:   event.Index,
		ID:      event.ID,
		Routing: event.Routing,
	}

	if err := ri.bulkIndexer.Add(ctx, item); err != nil {
		return fmt.Errorf("enqueue delete item: %w", err)
	}

	ri.metrics.EventsIndexed.Add(1)
	return nil
}

// RegisterTransform registers or replaces a transform function for a named index.
func (ri *RealTimeIndexer) RegisterTransform(index string, fn TransformFunc) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.transforms[index] = fn
}

// AddFilter appends a filter function to the pipeline.
func (ri *RealTimeIndexer) AddFilter(fn FilterFunc) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.filters = append(ri.filters, fn)
}

// -----------------------------------------------------------------
// Built-in domain-specific transform functions
// -----------------------------------------------------------------

// VideoTransform enriches a video event before indexing,
// computing derived fields such as engagement rate.
func VideoTransform(event IndexerEvent) (string, string, interface{}, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(event.Document, &doc); err != nil {
		return "", "", nil, fmt.Errorf("unmarshal video document: %w", err)
	}

	// Compute engagement_rate if counts are present
	viewCount, _ := doc["view_count"].(float64)
	likeCount, _ := doc["like_count"].(float64)
	commentCount, _ := doc["comment_count"].(float64)
	shareCount, _ := doc["share_count"].(float64)

	if viewCount > 0 {
		engagementRate := (likeCount + commentCount + shareCount) / viewCount
		doc["engagement_rate"] = engagementRate
	}

	// Stamp indexing time
	doc["indexed_at"] = time.Now().UTC().Format(time.RFC3339Nano)

	return event.Index, event.ID, doc, nil
}

// HashtagTransform computes velocity for trending score calculations.
func HashtagTransform(event IndexerEvent) (string, string, interface{}, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(event.Document, &doc); err != nil {
		return "", "", nil, fmt.Errorf("unmarshal hashtag document: %w", err)
	}

	// Default velocity to 0 if not provided
	if _, ok := doc["velocity"]; !ok {
		doc["velocity"] = 0.0
	}

	doc["indexed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return event.Index, event.ID, doc, nil
}

// UserTransform computes profile_score from engagement signals.
func UserTransform(event IndexerEvent) (string, string, interface{}, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(event.Document, &doc); err != nil {
		return "", "", nil, fmt.Errorf("unmarshal user document: %w", err)
	}

	followerCount, _ := doc["follower_count"].(float64)
	videoCount, _ := doc["video_count"].(float64)
	likeCount, _ := doc["like_count"].(float64)

	// Simple profile score heuristic
	score := (followerCount * 0.5) + (videoCount * 10) + (likeCount * 0.1)
	doc["profile_score"] = score
	doc["indexed_at"] = time.Now().UTC().Format(time.RFC3339Nano)

	return event.Index, event.ID, doc, nil
}

// ProductTransform normalises price and computes popularity_score.
func ProductTransform(event IndexerEvent) (string, string, interface{}, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(event.Document, &doc); err != nil {
		return "", "", nil, fmt.Errorf("unmarshal product document: %w", err)
	}

	soldCount, _ := doc["sold_count"].(float64)
	rating, _ := doc["rating"].(float64)
	ratingCount, _ := doc["rating_count"].(float64)

	// Bayesian-style popularity score
	if ratingCount > 0 {
		score := (soldCount * 0.4) + (rating * ratingCount * 0.6)
		doc["popularity_score"] = score
	}

	doc["indexed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return event.Index, event.ID, doc, nil
}

// SoundTransform computes trending flag and trending_score.
func SoundTransform(event IndexerEvent) (string, string, interface{}, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(event.Document, &doc); err != nil {
		return "", "", nil, fmt.Errorf("unmarshal sound document: %w", err)
	}

	usageCount, _ := doc["usage_count"].(float64)
	velocity, _ := doc["velocity"].(float64)

	trendingScore := (usageCount * 0.3) + (velocity * 0.7)
	doc["trending_score"] = trendingScore
	doc["trending"] = trendingScore > 1000.0

	doc["indexed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return event.Index, event.ID, doc, nil
}

// DomainTransforms returns the full set of production transform functions.
func DomainTransforms() map[string]TransformFunc {
	return map[string]TransformFunc{
		"users":    UserTransform,
		"videos":   VideoTransform,
		"hashtags": HashtagTransform,
		"products": ProductTransform,
		"sounds":   SoundTransform,
	}
}

// -----------------------------------------------------------------
// Built-in filter functions
// -----------------------------------------------------------------

// ActiveContentFilter drops events for deleted or banned documents
// where the document itself signals it should be removed from search.
func ActiveContentFilter(event IndexerEvent) bool {
	if event.EventType == "deleted" {
		return true // always allow delete events through
	}
	if len(event.Document) == 0 {
		return false
	}

	var doc struct {
		IsDeleted bool `json:"is_deleted"`
		IsBanned  bool `json:"is_banned"`
	}
	if err := json.Unmarshal(event.Document, &doc); err != nil {
		return true // if we can't parse, allow through and let ES reject it
	}
	return !doc.IsDeleted && !doc.IsBanned
}

// PrivacyFilter drops non-public content from the public search index.
func PrivacyFilter(event IndexerEvent) bool {
	if event.EventType == "deleted" {
		return true
	}
	if len(event.Document) == 0 {
		return true
	}

	var doc struct {
		PrivacyLevel string `json:"privacy_level"`
	}
	if err := json.Unmarshal(event.Document, &doc); err != nil {
		return true
	}
	// Only allow public content
	return doc.PrivacyLevel == "" || doc.PrivacyLevel == "public"
}
