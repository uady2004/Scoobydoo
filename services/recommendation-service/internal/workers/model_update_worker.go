package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/recommendation-service/internal/config"
	"github.com/tiktok-clone/recommendation-service/internal/models"
	"github.com/tiktok-clone/recommendation-service/internal/services"
)

// ModelUpdateWorker consumes engagement events from Kafka and periodically
// rebuilds the item-item collaborative-filtering similarity matrix stored in
// Redis sorted sets.
//
// Architecture:
//
//	Kafka engagement-events topic
//	        │
//	        ▼
//	  consumerLoop()          – processes each message, accumulates events in
//	  ┌──────────────┐          the in-memory interactionBuffer
//	  │ interaction  │
//	  │  buffer      │◄──────  rebuildMatrix() is called every
//	  └──────────────┘          MatrixUpdateInterval; it computes item-item
//	        │                   cosine similarity from the buffer and writes
//	        ▼                   top-K neighbours to Redis.
//	  Redis (rec:cf:similar:*)
//
// The in-memory buffer maps videoID → map[userID]score.  This is a standard
// user-item interaction matrix stored in a column-per-item layout for fast
// cosine-similarity computation between item pairs.
type ModelUpdateWorker struct {
	cfg          *config.Config
	rdb          redis.UniversalClient
	featureStore *services.FeatureStore
	logger       *zap.Logger

	// interactionBuffer: videoID → userID → accumulated engagement score
	mu                sync.RWMutex
	interactionBuffer map[string]map[string]float64
	// bufferStartTime is when the current window began.
	bufferStartTime time.Time

	// Sarama consumer group.
	consumerGroup sarama.ConsumerGroup
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewModelUpdateWorker constructs a ModelUpdateWorker and connects to Kafka.
func NewModelUpdateWorker(
	cfg *config.Config,
	rdb redis.UniversalClient,
	featureStore *services.FeatureStore,
	logger *zap.Logger,
) (*ModelUpdateWorker, error) {

	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V2_8_0_0
	saramaCfg.Consumer.Group.Session.Timeout = cfg.Kafka.SessionTimeout
	saramaCfg.Consumer.Group.Heartbeat.Interval = cfg.Kafka.HeartbeatInterval
	saramaCfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaCfg.Consumer.Return.Errors = true

	cg, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer group: %w", err)
	}

	return &ModelUpdateWorker{
		cfg:               cfg,
		rdb:               rdb,
		featureStore:      featureStore,
		logger:            logger,
		interactionBuffer: make(map[string]map[string]float64),
		bufferStartTime:   time.Now(),
		consumerGroup:     cg,
		stopCh:            make(chan struct{}),
	}, nil
}

// Start launches the Kafka consume loop and the periodic matrix-rebuild
// goroutine.  It blocks until Stop() is called.
func (w *ModelUpdateWorker) Start(ctx context.Context) error {
	w.logger.Info("model update worker starting",
		zap.Strings("brokers", w.cfg.Kafka.Brokers),
		zap.String("topic", w.cfg.Kafka.EngagementTopic),
		zap.Duration("rebuild_interval", w.cfg.ModelUpdate.MatrixUpdateInterval))

	// Log consumer group errors in the background.
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for err := range w.consumerGroup.Errors() {
			w.logger.Error("kafka consumer group error", zap.Error(err))
		}
	}()

	// Periodic matrix rebuild.
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.rebuildLoop(ctx)
	}()

	// Kafka consume loop (re-joins on rebalance automatically).
	handler := &engagementConsumerHandler{worker: w}
	topics := []string{w.cfg.Kafka.EngagementTopic}

	for {
		select {
		case <-w.stopCh:
			return nil
		default:
		}
		if err := w.consumerGroup.Consume(ctx, topics, handler); err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				return nil
			}
			w.logger.Error("consume error, retrying in 5s", zap.Error(err))
			select {
			case <-w.stopCh:
				return nil
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// Stop signals the worker to shut down and waits for goroutines to exit.
func (w *ModelUpdateWorker) Stop() {
	close(w.stopCh)
	w.consumerGroup.Close() //nolint:errcheck
	w.wg.Wait()
}

// =============================================================================
// Kafka consumer group handler
// =============================================================================

type engagementConsumerHandler struct {
	worker *ModelUpdateWorker
}

func (h *engagementConsumerHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *engagementConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *engagementConsumerHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.worker.processMessage(session.Context(), msg)
			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

// =============================================================================
// Message processing
// =============================================================================

// processMessage deserialises a Kafka message into an EngagementEvent and
// accumulates it in the in-memory interaction buffer.
func (w *ModelUpdateWorker) processMessage(ctx context.Context, msg *sarama.ConsumerMessage) {
	var ev models.EngagementEvent
	if err := json.Unmarshal(msg.Value, &ev); err != nil {
		w.logger.Warn("unmarshal engagement event failed",
			zap.ByteString("raw", msg.Value),
			zap.Error(err))
		return
	}

	// Validate required fields.
	if ev.UserID == "" || ev.VideoID == "" {
		return
	}

	// Skip negative-signal events (skips) from the CF matrix to avoid
	// polluting collaborative item similarity with negative co-occurrences.
	if ev.Score <= 0 {
		// Still persist to feature store for ranking signals.
		_ = w.featureStore.RecordEngagement(ctx, &ev)
		return
	}

	// Persist the engagement to the feature store (liked set, co-user set).
	if err := w.featureStore.RecordEngagement(ctx, &ev); err != nil {
		w.logger.Warn("feature store engagement record failed",
			zap.String("user_id", ev.UserID),
			zap.String("video_id", ev.VideoID),
			zap.Error(err))
	}

	// Check event age; skip if outside the configured lookback window.
	windowStart := time.Now().AddDate(0, 0, -w.cfg.ModelUpdate.EngagementWindowDays)
	if ev.Timestamp.Before(windowStart) {
		return
	}

	w.mu.Lock()
	if _, ok := w.interactionBuffer[ev.VideoID]; !ok {
		w.interactionBuffer[ev.VideoID] = make(map[string]float64)
	}
	w.interactionBuffer[ev.VideoID][ev.UserID] += ev.Score
	w.mu.Unlock()
}

// =============================================================================
// Collaborative filtering matrix rebuild
// =============================================================================

// rebuildLoop triggers a matrix rebuild on the configured interval.
func (w *ModelUpdateWorker) rebuildLoop(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.ModelUpdate.MatrixUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			n, err := w.rebuildMatrix(ctx)
			if err != nil {
				w.logger.Error("matrix rebuild failed", zap.Error(err))
			} else {
				w.logger.Info("CF matrix rebuilt",
					zap.Int("items_updated", n),
					zap.Duration("elapsed", time.Since(start)))
			}
		}
	}
}

// rebuildMatrix reads the current interaction buffer, computes item-item cosine
// similarities, and writes the top-K similar items per video to Redis.
//
// Algorithm:
//
//  1. For each video i, build an interaction vector v_i where v_i[userID] = score.
//  2. For every pair (i, j) of co-interacted items (i.e., at least one shared user),
//     compute cosine similarity = dot(v_i, v_j) / (|v_i| * |v_j|).
//  3. For each video i, keep the top-K most similar neighbours.
//  4. Atomically write the sorted set to Redis.
//
// This is O(V^2 * U) in the worst case.  In practice the candidate reduction
// via MinInteractionsForItem and the engagement window bound the matrix size.
func (w *ModelUpdateWorker) rebuildMatrix(ctx context.Context) (int, error) {
	// Snapshot the buffer under a read lock so consumption can continue.
	w.mu.RLock()
	snapshot := make(map[string]map[string]float64, len(w.interactionBuffer))
	for videoID, users := range w.interactionBuffer {
		usersCopy := make(map[string]float64, len(users))
		for userID, score := range users {
			usersCopy[userID] = score
		}
		snapshot[videoID] = usersCopy
	}
	w.mu.RUnlock()

	if len(snapshot) == 0 {
		return 0, nil
	}

	// Filter out items with fewer than MinInteractionsForItem events.
	minInteractions := w.cfg.ModelUpdate.MinInteractionsForItem
	filtered := make(map[string]map[string]float64, len(snapshot))
	for videoID, users := range snapshot {
		if len(users) >= minInteractions {
			filtered[videoID] = users
		}
	}

	if len(filtered) == 0 {
		return 0, nil
	}

	w.logger.Debug("rebuilding CF matrix",
		zap.Int("items", len(filtered)))

	// Precompute L2 norms.
	norms := make(map[string]float64, len(filtered))
	for videoID, users := range filtered {
		norm := 0.0
		for _, score := range users {
			norm += score * score
		}
		norms[videoID] = math.Sqrt(norm)
	}

	// Build an inverted index: userID → []videoID for fast pair enumeration.
	userIndex := make(map[string][]string, 10000)
	for videoID, users := range filtered {
		for userID := range users {
			userIndex[userID] = append(userIndex[userID], videoID)
		}
	}

	// Compute pairwise cosine similarities via the inverted index.
	// sim[i][j] = Σ_u v_i[u] * v_j[u] / (|v_i| * |v_j|)
	type pairKey struct{ a, b string }
	dotProducts := make(map[pairKey]float64, len(filtered)*10)

	for _, videoList := range userIndex {
		for ai := 0; ai < len(videoList); ai++ {
			for bi := ai + 1; bi < len(videoList); bi++ {
				a := videoList[ai]
				b := videoList[bi]
				// Ensure canonical order so we only store each pair once.
				if a > b {
					a, b = b, a
				}
				key := pairKey{a, b}
				// We need to find the shared user's scores.
				// The dot-product contribution for this pair from this user.
				// We do it when iterating over users below.
				dotProducts[key] = 0 // initialise
			}
		}
	}

	// Re-populate dot products by iterating over users once more.
	for userID, videoList := range userIndex {
		for ai := 0; ai < len(videoList); ai++ {
			for bi := ai + 1; bi < len(videoList); bi++ {
				a := videoList[ai]
				b := videoList[bi]
				if a > b {
					a, b = b, a
				}
				key := pairKey{a, b}
				scoreA := filtered[a][userID]
				scoreB := filtered[b][userID]
				dotProducts[key] += scoreA * scoreB
			}
		}
	}

	// For each video, collect its top-K neighbours.
	type simEntry struct {
		videoID string
		sim     float64
	}

	topK := w.cfg.ModelUpdate.TopKSimilarItems
	neighbours := make(map[string][]simEntry, len(filtered))

	for pair, dot := range dotProducts {
		normA := norms[pair.a]
		normB := norms[pair.b]
		if normA == 0 || normB == 0 {
			continue
		}
		sim := dot / (normA * normB)
		if sim <= 0 {
			continue
		}
		neighbours[pair.a] = append(neighbours[pair.a], simEntry{pair.b, sim})
		neighbours[pair.b] = append(neighbours[pair.b], simEntry{pair.a, sim})
	}

	// Write top-K to Redis using pipelines.
	pipe := w.rdb.Pipeline()
	updatedCount := 0

	for videoID, sims := range neighbours {
		// Sort descending by similarity and keep top-K.
		sort.Slice(sims, func(i, j int) bool {
			return sims[i].sim > sims[j].sim
		})
		if len(sims) > topK {
			sims = sims[:topK]
		}

		key := cfSimilarKey(videoID)
		// Atomically replace the sorted set.
		pipe.Del(ctx, key)
		members := make([]redis.Z, len(sims))
		for i, s := range sims {
			members[i] = redis.Z{Score: s.sim, Member: s.videoID}
		}
		pipe.ZAdd(ctx, key, members...)
		// Keep CF data for twice the engagement window.
		ttl := time.Duration(w.cfg.ModelUpdate.EngagementWindowDays*2) * 24 * time.Hour
		pipe.Expire(ctx, key, ttl)
		updatedCount++

		// Flush every 500 items to avoid oversized pipelines.
		if updatedCount%500 == 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				w.logger.Warn("pipeline flush failed during matrix write", zap.Error(err))
			}
			pipe = w.rdb.Pipeline()
		}
	}

	// Final flush.
	if _, err := pipe.Exec(ctx); err != nil {
		w.logger.Warn("final pipeline flush failed during matrix write", zap.Error(err))
	}

	// Prune the in-memory buffer: remove events older than the lookback window
	// by resetting the buffer and letting it refill from Kafka.
	// This is a coarse pruning; a production system would use time-bucketed
	// sliding windows.
	windowAge := time.Since(w.bufferStartTime)
	maxAge := time.Duration(w.cfg.ModelUpdate.EngagementWindowDays) * 24 * time.Hour
	if windowAge > maxAge {
		w.mu.Lock()
		w.interactionBuffer = make(map[string]map[string]float64)
		w.bufferStartTime = time.Now()
		w.mu.Unlock()
		w.logger.Info("interaction buffer reset after window expiry",
			zap.Duration("window_age", windowAge))
	}

	return updatedCount, nil
}

// ForceRebuild triggers an immediate matrix rebuild outside of the normal
// schedule.  Useful for testing or administrative tooling.
func (w *ModelUpdateWorker) ForceRebuild(ctx context.Context) (int, error) {
	return w.rebuildMatrix(ctx)
}

// BufferStats returns diagnostic information about the current interaction
// buffer without blocking the consume loop.
func (w *ModelUpdateWorker) BufferStats() (items int, totalInteractions int) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	items = len(w.interactionBuffer)
	for _, users := range w.interactionBuffer {
		totalInteractions += len(users)
	}
	return
}

// cfSimilarKey returns the Redis key for the item-item CF sorted set for a
// given video.  This must match the key used by the CandidateGenerator.
func cfSimilarKey(videoID string) string {
	return fmt.Sprintf("rec:cf:similar:%s", videoID)
}
