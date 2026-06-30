// Package workers - TrendingUpdater runs on an hourly schedule and calls
// TrendingService.RecalculateAll to refresh the Redis trending sorted sets
// from the latest Postgres engagement counts.
//
// It also consumes real-time VideoEvent messages from Kafka and performs
// single-video score updates whenever a video crosses a viral threshold so
// that the trending feed reacts within seconds to breakout content.
//
// Worker lifecycle:
//
//	updater := workers.NewTrendingUpdater(trendingSvc, cfg, logger)
//	go updater.Run(ctx)
package workers

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/tiktok-clone/feed-service/internal/models"
	"github.com/tiktok-clone/feed-service/internal/services"
)

// ---- TrendingUpdater --------------------------------------------------------

// TrendingUpdater periodically recalculates trending scores and reacts to
// real-time engagement events from Kafka.
type TrendingUpdater struct {
	trendingSvc *services.TrendingService
	logger      *zap.Logger
	interval    time.Duration

	// eventCh receives VideoEvent structs decoded from Kafka messages.
	// The Kafka consumer loop (startKafkaConsumer) sends to this channel;
	// the main Run loop drains it for hot-path single-video recalculations.
	eventCh chan models.VideoEvent

	// viralThreshold is the minimum raw engagement count delta within a single
	// Kafka poll that triggers an immediate single-video recalculation, rather
	// than waiting for the next hourly full pass.
	viralThreshold int64
}

// TrendingUpdaterConfig holds tuneable parameters.
type TrendingUpdaterConfig struct {
	// Interval is how often a full RecalculateAll pass runs (default 1 hour).
	Interval time.Duration
	// KafkaBrokers is the list of Kafka broker addresses.
	KafkaBrokers []string
	// VideoEventsTopic is the Kafka topic for video engagement events.
	VideoEventsTopic string
	// ConsumerGroup is the Kafka consumer group ID.
	ConsumerGroup string
	// ViralThreshold is the minimum event burst count to trigger an immediate
	// recalculation (default 1000 events within an interval).
	ViralThreshold int64
	// EventChannelSize is the internal channel buffer size (default 10000).
	EventChannelSize int
}

// NewTrendingUpdater constructs a TrendingUpdater.
func NewTrendingUpdater(
	trendingSvc *services.TrendingService,
	cfg TrendingUpdaterConfig,
	logger *zap.Logger,
) *TrendingUpdater {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Hour
	}
	if cfg.ViralThreshold <= 0 {
		cfg.ViralThreshold = 1_000
	}
	if cfg.EventChannelSize <= 0 {
		cfg.EventChannelSize = 10_000
	}
	return &TrendingUpdater{
		trendingSvc:    trendingSvc,
		logger:         logger,
		interval:       cfg.Interval,
		eventCh:        make(chan models.VideoEvent, cfg.EventChannelSize),
		viralThreshold: cfg.ViralThreshold,
	}
}

// Run starts the trending updater. It blocks until ctx is cancelled.
// Safe to call in a goroutine.
func (u *TrendingUpdater) Run(ctx context.Context) {
	u.logger.Info("trending updater started",
		zap.Duration("interval", u.interval),
		zap.Int64("viral_threshold", u.viralThreshold),
	)

	// Run a full recalculation immediately on startup.
	u.runFullRecalculation(ctx)

	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	// eventCounts tracks how many events we have seen per videoID since the
	// last full recalculation. When the count exceeds viralThreshold we
	// immediately recalculate that video's score.
	eventCounts := make(map[string]int64)

	for {
		select {
		case <-ctx.Done():
			u.logger.Info("trending updater stopping")
			return

		case <-ticker.C:
			// Hourly full recalculation.
			// Reset per-video event counters; the full pass will pick up all
			// accumulated counts from Postgres.
			eventCounts = make(map[string]int64)
			u.runFullRecalculation(ctx)

		case evt, ok := <-u.eventCh:
			if !ok {
				return
			}
			u.handleEvent(ctx, evt, eventCounts)
		}
	}
}

// RunOnce triggers a single full recalculation pass synchronously.
// Exported for testing and CLI one-shot invocations.
func (u *TrendingUpdater) RunOnce(ctx context.Context) error {
	return u.trendingSvc.RecalculateAll(ctx)
}

// SendEvent enqueues a VideoEvent for processing. Non-blocking: if the
// internal channel is full the event is dropped with a warning log.
func (u *TrendingUpdater) SendEvent(evt models.VideoEvent) {
	select {
	case u.eventCh <- evt:
	default:
		u.logger.Warn("trending updater: event channel full, dropping event",
			zap.String("video_id", evt.VideoID),
			zap.String("event_type", evt.EventType),
		)
	}
}

// ProcessRawEvent decodes a raw JSON-encoded VideoEvent payload and enqueues
// it. Used by Kafka consumer bridges.
func (u *TrendingUpdater) ProcessRawEvent(payload []byte) {
	var evt models.VideoEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		u.logger.Warn("trending updater: failed to decode event",
			zap.Error(err),
		)
		return
	}
	u.SendEvent(evt)
}

// runFullRecalculation executes RecalculateAll with error logging.
func (u *TrendingUpdater) runFullRecalculation(ctx context.Context) {
	start := time.Now()
	u.logger.Info("trending updater: starting full recalculation")

	recalcCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := u.trendingSvc.RecalculateAll(recalcCtx); err != nil {
		u.logger.Error("trending updater: full recalculation failed",
			zap.Error(err),
			zap.Duration("elapsed", time.Since(start)),
		)
		return
	}

	u.logger.Info("trending updater: full recalculation complete",
		zap.Duration("elapsed", time.Since(start)),
	)
}

// handleEvent processes a single VideoEvent. It increments the per-video
// event counter and triggers an immediate single-video recalculation if the
// count crosses the viral threshold.
func (u *TrendingUpdater) handleEvent(
	ctx context.Context,
	evt models.VideoEvent,
	eventCounts map[string]int64,
) {
	eventCounts[evt.VideoID]++

	if eventCounts[evt.VideoID] >= u.viralThreshold {
		// Reset the counter for this video to avoid continuous re-triggering
		// every single subsequent event.
		eventCounts[evt.VideoID] = 0

		u.logger.Info("trending updater: viral threshold crossed, recalculating video",
			zap.String("video_id", evt.VideoID),
		)

		recalcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := u.trendingSvc.RecalculateVideo(recalcCtx, evt.VideoID, ""); err != nil {
			u.logger.Warn("trending updater: single-video recalculation failed",
				zap.String("video_id", evt.VideoID),
				zap.Error(err),
			)
		}
	}
}
