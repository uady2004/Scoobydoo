// Package workers contains background goroutines that maintain the feed
// service's Redis data structures asynchronously.
//
// FeedPrecomputeWorker runs on a configurable interval and pre-builds the
// For-You and Following feed sorted sets in Redis for the most active users.
// Pre-computing feeds for frequent users means their first page-load always
// gets a cache hit, keeping p99 latency low.
//
// Worker lifecycle:
//
//	worker := workers.NewFeedPrecomputeWorker(feedSvc, repo, cfg, logger)
//	go worker.Run(ctx)        // blocks until ctx is cancelled
package workers

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/tiktok-clone/feed-service/internal/models"
	"github.com/tiktok-clone/feed-service/internal/repositories"
	"github.com/tiktok-clone/feed-service/internal/services"
)

// ---- FeedPrecomputeWorker ---------------------------------------------------

// FeedPrecomputeWorker pre-builds feed caches for active users on a timer.
type FeedPrecomputeWorker struct {
	feedSvc  *services.FeedService
	repo     *repositories.FeedRepository
	logger   *zap.Logger
	interval time.Duration
	// batchSize is the maximum number of users to precompute per tick.
	batchSize int64
	// concurrency is the number of parallel goroutines per batch.
	concurrency int
}

// FeedPrecomputeConfig holds tuneable parameters for the worker.
type FeedPrecomputeConfig struct {
	// Interval is how often the worker runs (default 5 minutes).
	Interval time.Duration
	// BatchSize is the max users processed per run (default 500).
	BatchSize int
	// Concurrency is the number of parallel precompute goroutines (default 10).
	Concurrency int
}

// NewFeedPrecomputeWorker constructs a FeedPrecomputeWorker.
func NewFeedPrecomputeWorker(
	feedSvc *services.FeedService,
	repo *repositories.FeedRepository,
	cfg FeedPrecomputeConfig,
	logger *zap.Logger,
) *FeedPrecomputeWorker {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	return &FeedPrecomputeWorker{
		feedSvc:     feedSvc,
		repo:        repo,
		logger:      logger,
		interval:    cfg.Interval,
		batchSize:   int64(cfg.BatchSize),
		concurrency: cfg.Concurrency,
	}
}

// Run starts the precompute loop. It blocks until ctx is cancelled.
// Safe to call in a goroutine.
func (w *FeedPrecomputeWorker) Run(ctx context.Context) {
	w.logger.Info("feed precompute worker started",
		zap.Duration("interval", w.interval),
		zap.Int64("batch_size", w.batchSize),
		zap.Int("concurrency", w.concurrency),
	)

	// Run once immediately on startup so we don't wait a full interval.
	w.runOnce(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("feed precompute worker stopping")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// RunOnce executes a single precompute pass. Exported for testing and
// one-shot CLI invocations.
func (w *FeedPrecomputeWorker) RunOnce(ctx context.Context) {
	w.runOnce(ctx)
}

// runOnce executes a single precompute pass over both For-You and Following
// feed types.
func (w *FeedPrecomputeWorker) runOnce(ctx context.Context) {
	start := time.Now()
	w.logger.Info("feed precompute: starting pass")

	forYouCount := w.precomputeFeedType(ctx, models.FeedTypeForYou)
	followingCount := w.precomputeFeedType(ctx, models.FeedTypeFollowing)

	w.logger.Info("feed precompute: pass complete",
		zap.Int("foryou_computed", forYouCount),
		zap.Int("following_computed", followingCount),
		zap.Duration("elapsed", time.Since(start)),
	)
}

// precomputeFeedType runs precompute for a single feed type and returns the
// number of users whose feeds were successfully rebuilt.
func (w *FeedPrecomputeWorker) precomputeFeedType(ctx context.Context, ft models.FeedType) int {
	userIDs, err := w.repo.GetUsersNeedingPrecompute(ctx, ft, w.batchSize)
	if err != nil {
		w.logger.Error("failed to get users needing precompute",
			zap.String("feed_type", string(ft)),
			zap.Error(err),
		)
		return 0
	}
	if len(userIDs) == 0 {
		return 0
	}

	w.logger.Info("feed precompute: users to process",
		zap.String("feed_type", string(ft)),
		zap.Int("count", len(userIDs)),
	)

	// Use a semaphore to limit concurrency.
	sem := make(chan struct{}, w.concurrency)
	var (
		mu      sync.Mutex
		success int
		wg      sync.WaitGroup
	)

	for _, uid := range userIDs {
		uid := uid // capture loop variable
		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			// Each goroutine gets a child context with a per-user timeout so a
			// single slow recommendation call cannot block the whole batch.
			userCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			var precomputeErr error
			switch ft {
			case models.FeedTypeForYou:
				precomputeErr = w.feedSvc.PrecomputeForYouFeed(userCtx, uid)
			case models.FeedTypeFollowing:
				precomputeErr = w.feedSvc.PrecomputeFollowingFeed(userCtx, uid)
			}

			if precomputeErr != nil {
				w.logger.Warn("precompute failed for user",
					zap.String("user_id", uid),
					zap.String("feed_type", string(ft)),
					zap.Error(precomputeErr),
				)
				return
			}

			mu.Lock()
			success++
			mu.Unlock()
		}()
	}

	wg.Wait()
	return success
}
