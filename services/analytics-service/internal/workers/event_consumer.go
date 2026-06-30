package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/ClickHouse/clickhouse-go/v2"
	"go.uber.org/zap"

	"github.com/tiktok-clone/analytics-service/internal/config"
)

// ---------------------------------------------------------------------------
// Event payload types consumed from Kafka
// ---------------------------------------------------------------------------

// VideoViewedEvent is emitted by the video-service when a user watches a video.
type VideoViewedEvent struct {
	EventID             string    `json:"event_id"`
	VideoID             string    `json:"video_id"`
	CreatorID           string    `json:"creator_id"`
	ViewerID            string    `json:"viewer_id"`
	ViewedAt            time.Time `json:"viewed_at"`
	WatchDurationSeconds int64   `json:"watch_duration_seconds"`
	CompletionPct       float64   `json:"completion_pct"`
	Source              string    `json:"source"` // fyp | following | search | profile
	CountryCode         string    `json:"country_code"`
	DeviceType          string    `json:"device_type"`
	AppVersion          string    `json:"app_version"`
}

// EngagementEvent is emitted for likes, comments, shares, bookmarks.
type EngagementEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"` // like | unlike | comment | share | bookmark
	VideoID   string    `json:"video_id"`
	CreatorID string    `json:"creator_id"`
	UserID    string    `json:"user_id"`
	OccurredAt time.Time `json:"occurred_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// AdImpressionEvent is emitted by the ads-service for ad events.
type AdImpressionEvent struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"` // impression | click | conversion | view | complete
	CampaignID string    `json:"campaign_id"`
	AdID       string    `json:"ad_id"`
	VideoID    string    `json:"video_id"`
	UserID     string    `json:"user_id"`
	OccurredAt time.Time `json:"occurred_at"`
	SpendUSD   float64   `json:"spend_usd"`
	RevenueUSD float64   `json:"revenue_usd"`
	CountryCode string   `json:"country_code"`
}

// LiveEvent is emitted by the livestream-service.
type LiveEvent struct {
	EventID            string    `json:"event_id"`
	EventType          string    `json:"event_type"` // join | leave | gift | comment | share | follow
	LiveID             string    `json:"live_id"`
	CreatorID          string    `json:"creator_id"`
	ViewerID           string    `json:"viewer_id"`
	OccurredAt         time.Time `json:"occurred_at"`
	ConcurrentViewers  int64     `json:"concurrent_viewers"`
	WatchSeconds       int64     `json:"watch_seconds"`
	GiftValueUSD       float64   `json:"gift_value_usd"`
}

// ---------------------------------------------------------------------------
// Batch buffers
// ---------------------------------------------------------------------------

type viewBatch struct {
	mu     sync.Mutex
	events []VideoViewedEvent
}

type engagementBatch struct {
	mu     sync.Mutex
	events []EngagementEvent
}

type adBatch struct {
	mu     sync.Mutex
	events []AdImpressionEvent
}

type liveBatch struct {
	mu     sync.Mutex
	events []LiveEvent
}

// ---------------------------------------------------------------------------
// EventConsumer
// ---------------------------------------------------------------------------

// EventConsumer is a Sarama ConsumerGroup handler that ingests Kafka events
// into ClickHouse via batched inserts.
type EventConsumer struct {
	cfg    *config.Config
	ch     clickhouse.Conn
	logger *zap.Logger

	views       viewBatch
	engagements engagementBatch
	ads         adBatch
	lives       liveBatch

	flushTicker *time.Ticker
	stopCh      chan struct{}
}

// NewEventConsumer creates a new EventConsumer.
func NewEventConsumer(
	cfg *config.Config,
	ch clickhouse.Conn,
	logger *zap.Logger,
) *EventConsumer {
	c := &EventConsumer{
		cfg:    cfg,
		ch:     ch,
		logger: logger,
		stopCh: make(chan struct{}),
	}
	c.flushTicker = time.NewTicker(cfg.ClickHouse.BatchTimeout)
	return c
}

// Start launches the background flusher goroutine.
func (c *EventConsumer) Start(ctx context.Context) {
	go c.flushLoop(ctx)
}

// Stop signals the flusher to stop and performs a final flush.
func (c *EventConsumer) Stop() {
	close(c.stopCh)
	c.flushTicker.Stop()
	// Final flush with a short deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c.flushAll(ctx)
}

// ---------------------------------------------------------------------------
// Sarama ConsumerGroupHandler interface
// ---------------------------------------------------------------------------

func (c *EventConsumer) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (c *EventConsumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim processes messages from a single topic partition.
func (c *EventConsumer) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	topics := c.cfg.Kafka.Topics
	for msg := range claim.Messages() {
		switch msg.Topic {
		case topics.VideoViewed:
			c.handleVideoViewed(msg)
		case topics.Engagement:
			c.handleEngagement(msg)
		case topics.AdImpression:
			c.handleAdImpression(msg)
		case topics.LiveEvents:
			c.handleLiveEvent(msg)
		default:
			c.logger.Warn("unknown topic", zap.String("topic", msg.Topic))
		}
		session.MarkMessage(msg, "")

		// Flush when buffer is full.
		c.maybeFlush(session.Context())
	}
	return nil
}

// ---------------------------------------------------------------------------
// Message handlers
// ---------------------------------------------------------------------------

func (c *EventConsumer) handleVideoViewed(msg *sarama.ConsumerMessage) {
	var ev VideoViewedEvent
	if err := json.Unmarshal(msg.Value, &ev); err != nil {
		c.logger.Error("unmarshal VideoViewedEvent", zap.Error(err))
		return
	}
	c.views.mu.Lock()
	c.views.events = append(c.views.events, ev)
	c.views.mu.Unlock()
}

func (c *EventConsumer) handleEngagement(msg *sarama.ConsumerMessage) {
	var ev EngagementEvent
	if err := json.Unmarshal(msg.Value, &ev); err != nil {
		c.logger.Error("unmarshal EngagementEvent", zap.Error(err))
		return
	}
	c.engagements.mu.Lock()
	c.engagements.events = append(c.engagements.events, ev)
	c.engagements.mu.Unlock()
}

func (c *EventConsumer) handleAdImpression(msg *sarama.ConsumerMessage) {
	var ev AdImpressionEvent
	if err := json.Unmarshal(msg.Value, &ev); err != nil {
		c.logger.Error("unmarshal AdImpressionEvent", zap.Error(err))
		return
	}
	c.ads.mu.Lock()
	c.ads.events = append(c.ads.events, ev)
	c.ads.mu.Unlock()
}

func (c *EventConsumer) handleLiveEvent(msg *sarama.ConsumerMessage) {
	var ev LiveEvent
	if err := json.Unmarshal(msg.Value, &ev); err != nil {
		c.logger.Error("unmarshal LiveEvent", zap.Error(err))
		return
	}
	c.lives.mu.Lock()
	c.lives.events = append(c.lives.events, ev)
	c.lives.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Flush logic
// ---------------------------------------------------------------------------

func (c *EventConsumer) maybeFlush(ctx context.Context) {
	batchSize := c.cfg.ClickHouse.BatchSize
	c.views.mu.Lock()
	viewsFull := len(c.views.events) >= batchSize
	c.views.mu.Unlock()

	c.engagements.mu.Lock()
	engFull := len(c.engagements.events) >= batchSize
	c.engagements.mu.Unlock()

	if viewsFull || engFull {
		c.flushAll(ctx)
	}
}

func (c *EventConsumer) flushLoop(ctx context.Context) {
	for {
		select {
		case <-c.flushTicker.C:
			c.flushAll(ctx)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *EventConsumer) flushAll(ctx context.Context) {
	c.flushViews(ctx)
	c.flushEngagements(ctx)
	c.flushAds(ctx)
	c.flushLiveEvents(ctx)
}

// flushViews batch-inserts video_views into ClickHouse.
func (c *EventConsumer) flushViews(ctx context.Context) {
	c.views.mu.Lock()
	events := c.views.events
	c.views.events = nil
	c.views.mu.Unlock()

	if len(events) == 0 {
		return
	}

	batch, err := c.ch.PrepareBatch(ctx, `
		INSERT INTO video_views
		(event_id, video_id, creator_id, viewer_id, viewed_at,
		 watch_duration_seconds, completion_pct, source, country_code,
		 device_type, app_version)
		VALUES
	`)
	if err != nil {
		c.logger.Error("prepare video_views batch", zap.Error(err))
		return
	}

	for _, ev := range events {
		if err := batch.Append(
			ev.EventID,
			ev.VideoID,
			ev.CreatorID,
			ev.ViewerID,
			ev.ViewedAt,
			ev.WatchDurationSeconds,
			ev.CompletionPct,
			ev.Source,
			ev.CountryCode,
			ev.DeviceType,
			ev.AppVersion,
		); err != nil {
			c.logger.Error("append video_view row", zap.Error(err))
		}
	}

	if err := batch.Send(); err != nil {
		c.logger.Error("send video_views batch", zap.Error(err), zap.Int("count", len(events)))
		return
	}
	c.logger.Info("flushed video_views", zap.Int("count", len(events)))
}

// flushEngagements batch-inserts engagement_events into ClickHouse.
func (c *EventConsumer) flushEngagements(ctx context.Context) {
	c.engagements.mu.Lock()
	events := c.engagements.events
	c.engagements.events = nil
	c.engagements.mu.Unlock()

	if len(events) == 0 {
		return
	}

	batch, err := c.ch.PrepareBatch(ctx, `
		INSERT INTO engagement_events
		(event_id, event_type, video_id, creator_id, user_id, occurred_at)
		VALUES
	`)
	if err != nil {
		c.logger.Error("prepare engagement_events batch", zap.Error(err))
		return
	}

	for _, ev := range events {
		if err := batch.Append(
			ev.EventID,
			ev.EventType,
			ev.VideoID,
			ev.CreatorID,
			ev.UserID,
			ev.OccurredAt,
		); err != nil {
			c.logger.Error("append engagement row", zap.Error(err))
		}
	}

	if err := batch.Send(); err != nil {
		c.logger.Error("send engagement_events batch", zap.Error(err), zap.Int("count", len(events)))
		return
	}
	c.logger.Info("flushed engagement_events", zap.Int("count", len(events)))
}

// flushAds batch-inserts ad_events into ClickHouse.
func (c *EventConsumer) flushAds(ctx context.Context) {
	c.ads.mu.Lock()
	events := c.ads.events
	c.ads.events = nil
	c.ads.mu.Unlock()

	if len(events) == 0 {
		return
	}

	batch, err := c.ch.PrepareBatch(ctx, `
		INSERT INTO ad_events
		(event_id, event_type, campaign_id, ad_id, video_id,
		 user_id, occurred_at, spend_usd, revenue_usd, country_code)
		VALUES
	`)
	if err != nil {
		c.logger.Error("prepare ad_events batch", zap.Error(err))
		return
	}

	for _, ev := range events {
		if err := batch.Append(
			ev.EventID,
			ev.EventType,
			ev.CampaignID,
			ev.AdID,
			ev.VideoID,
			ev.UserID,
			ev.OccurredAt,
			ev.SpendUSD,
			ev.RevenueUSD,
			ev.CountryCode,
		); err != nil {
			c.logger.Error("append ad_event row", zap.Error(err))
		}
	}

	if err := batch.Send(); err != nil {
		c.logger.Error("send ad_events batch", zap.Error(err), zap.Int("count", len(events)))
		return
	}
	c.logger.Info("flushed ad_events", zap.Int("count", len(events)))
}

// flushLiveEvents batch-inserts live_events into ClickHouse.
func (c *EventConsumer) flushLiveEvents(ctx context.Context) {
	c.lives.mu.Lock()
	events := c.lives.events
	c.lives.events = nil
	c.lives.mu.Unlock()

	if len(events) == 0 {
		return
	}

	batch, err := c.ch.PrepareBatch(ctx, `
		INSERT INTO live_events
		(event_id, event_type, live_id, creator_id, viewer_id,
		 occurred_at, concurrent_viewers, watch_seconds, gift_value_usd)
		VALUES
	`)
	if err != nil {
		c.logger.Error("prepare live_events batch", zap.Error(err))
		return
	}

	for _, ev := range events {
		if err := batch.Append(
			ev.EventID,
			ev.EventType,
			ev.LiveID,
			ev.CreatorID,
			ev.ViewerID,
			ev.OccurredAt,
			ev.ConcurrentViewers,
			ev.WatchSeconds,
			ev.GiftValueUSD,
		); err != nil {
			c.logger.Error("append live_event row", zap.Error(err))
		}
	}

	if err := batch.Send(); err != nil {
		c.logger.Error("send live_events batch", zap.Error(err), zap.Int("count", len(events)))
		return
	}
	c.logger.Info("flushed live_events", zap.Int("count", len(events)))
}

// RunConsumerGroup starts the Sarama consumer group loop (blocking).
func RunConsumerGroup(
	ctx context.Context,
	cfg *config.Config,
	handler sarama.ConsumerGroupHandler,
	logger *zap.Logger,
) error {
	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V2_8_0_0
	saramaCfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}

	cg, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup, saramaCfg)
	if err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}
	defer cg.Close()

	topics := []string{
		cfg.Kafka.Topics.VideoViewed,
		cfg.Kafka.Topics.Engagement,
		cfg.Kafka.Topics.AdImpression,
		cfg.Kafka.Topics.LiveEvents,
	}

	for {
		if err := cg.Consume(ctx, topics, handler); err != nil {
			logger.Error("consumer group error", zap.Error(err))
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}
