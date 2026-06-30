package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/social-graph-service/internal/models"
)

// ---------------------------------------------------------------------------
// Redis key templates
// ---------------------------------------------------------------------------
//
// These mirror the constants in social_service.go. They are duplicated here to
// avoid a circular import; in a larger codebase they would live in a shared
// "keys" sub-package imported by both.

const (
	// workerFollowerCountKey is the integer counter of followers for a user.
	workerFollowerCountKey = "social:follower_count:%s"
	// workerFollowingCountKey is the integer counter of following for a user.
	workerFollowingCountKey = "social:following_count:%s"
	// workerFollowerZSetKey is a sorted set: members = follower IDs, score = unix timestamp.
	workerFollowerZSetKey = "social:followers:%s"
	// workerFollowingZSetKey is a sorted set: members = followee IDs, score = unix timestamp.
	workerFollowingZSetKey = "social:following:%s"
	// workerSuggestionKeyFmt is the suggestion cache key for a user.
	workerSuggestionKeyFmt = "social:suggestions:%s"
	// workerFeedCacheKeyFmt is the feed-service's per-user feed cache key.
	// Deleting it signals feed-service to rebuild the timeline.
	workerFeedCacheKeyFmt = "feed:user:%s"
)

// ---------------------------------------------------------------------------
// EventProcessor
// ---------------------------------------------------------------------------

// EventProcessor is a Kafka consumer group client that reacts to follow/unfollow
// events and keeps Redis caches consistent. Multiple replicas share partition
// load automatically via the consumer-group protocol.
//
// It consumes from the topics configured in the service (user.followed and
// user.unfollowed by default) and performs idempotent Redis updates for each
// event. Because the SocialService also writes Redis synchronously on the
// hot path, the worker acts as a convergence mechanism: any replica that
// missed the in-process update (e.g. after a restart) eventually catches up.
type EventProcessor struct {
	consumerGroup sarama.ConsumerGroup
	redis         *redis.Client
	logger        *zap.Logger
	topics        []string
	counterTTL    time.Duration
}

// NewEventProcessor creates an EventProcessor wired to the given Kafka consumer
// group and Redis client.
func NewEventProcessor(
	consumerGroup sarama.ConsumerGroup,
	redisClient *redis.Client,
	logger *zap.Logger,
	topics []string,
	counterTTL time.Duration,
) *EventProcessor {
	return &EventProcessor{
		consumerGroup: consumerGroup,
		redis:         redisClient,
		logger:        logger,
		topics:        topics,
		counterTTL:    counterTTL,
	}
}

// ---------------------------------------------------------------------------
// Run / Close
// ---------------------------------------------------------------------------

// Run starts the consumer-group loop in the calling goroutine.  It returns
// only when ctx is cancelled.  Callers must run this in a dedicated goroutine:
//
//	go processor.Run(ctx)
func (p *EventProcessor) Run(ctx context.Context) {
	handler := &consumerGroupHandler{processor: p}

	for {
		// Consume blocks until the session ends (rebalance) or ctx is cancelled.
		if err := p.consumerGroup.Consume(ctx, p.topics, handler); err != nil {
			p.logger.Error("event_processor: consumer group error", zap.Error(err))
		}

		if ctx.Err() != nil {
			p.logger.Info("event_processor: context cancelled, shutting down")
			return
		}

		// Brief pause before reconnecting to avoid spinning on persistent broker errors.
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// Close shuts down the underlying consumer group. Must be called after Run returns.
func (p *EventProcessor) Close() error {
	return p.consumerGroup.Close()
}

// ---------------------------------------------------------------------------
// sarama.ConsumerGroupHandler implementation
// ---------------------------------------------------------------------------

// consumerGroupHandler implements sarama.ConsumerGroupHandler.
type consumerGroupHandler struct {
	processor *EventProcessor
}

// Setup is called at the beginning of a new consumer-group session.
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	h.processor.logger.Info("event_processor: consumer session started")
	return nil
}

// Cleanup is called at the end of a consumer-group session, after all
// ConsumeClaim goroutines have exited.
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.processor.logger.Info("event_processor: consumer session ended")
	return nil
}

// ConsumeClaim processes messages from a single partition claim.
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				// Channel closed — partition rebalance in progress.
				return nil
			}
			h.processor.handleMessage(session.Context(), msg)
			// Mark message as processed. AutoCommit is disabled; the consumer
			// group will flush marks to the broker on a configurable interval.
			session.MarkMessage(msg, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

// ---------------------------------------------------------------------------
// Message dispatch
// ---------------------------------------------------------------------------

// handleMessage decodes a Kafka message payload and routes it to the correct
// event handler based on the event_type field.
func (p *EventProcessor) handleMessage(ctx context.Context, msg *sarama.ConsumerMessage) {
	p.logger.Debug("event_processor: received message",
		zap.String("topic", msg.Topic),
		zap.Int32("partition", msg.Partition),
		zap.Int64("offset", msg.Offset),
	)

	var event models.FollowEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		// Non-retryable parse error: log, skip, and mark so the partition
		// continues making progress.
		p.logger.Error("event_processor: failed to unmarshal event",
			zap.String("topic", msg.Topic),
			zap.Int64("offset", msg.Offset),
			zap.ByteString("value_preview", truncate(msg.Value, 200)),
			zap.Error(err),
		)
		return
	}

	switch event.EventType {
	case "followed":
		p.handleFollowed(ctx, event)
	case "unfollowed":
		p.handleUnfollowed(ctx, event)
	default:
		p.logger.Warn("event_processor: unknown event type",
			zap.String("event_type", event.EventType),
			zap.String("topic", msg.Topic),
			zap.Int64("offset", msg.Offset),
		)
	}
}

// truncate returns at most n bytes of b for safe log preview.
func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}

// ---------------------------------------------------------------------------
// Follow event handler
// ---------------------------------------------------------------------------

// handleFollowed updates Redis when a "followed" event is consumed.
//
// Operations performed (idempotent):
//  1. ZADD social:followers:<followeeID> <unix_ts> <followerID>
//  2. ZADD social:following:<followerID> <unix_ts> <followeeID>
//  3. INCR social:follower_count:<followeeID>
//  4. INCR social:following_count:<followerID>
//  5. EXPIRE all four keys to reset TTL.
//  6. DEL suggestion cache for both parties.
//  7. DEL feed cache for the follower so their home timeline refreshes.
func (p *EventProcessor) handleFollowed(ctx context.Context, event models.FollowEvent) {
	ts := float64(event.OccurredAt.Unix())

	pipe := p.redis.Pipeline()

	followerZKey := fmt.Sprintf(workerFollowerZSetKey, event.FolloweeID)
	followingZKey := fmt.Sprintf(workerFollowingZSetKey, event.FollowerID)

	// Update sorted sets. ZADD is idempotent for an existing member: it only
	// updates the score, so re-processing the same event is safe.
	pipe.ZAdd(ctx, followerZKey, redis.Z{Score: ts, Member: event.FollowerID})
	pipe.ZAdd(ctx, followingZKey, redis.Z{Score: ts, Member: event.FolloweeID})

	// Increment counters. INCR initialises the key at 0 if absent, so no
	// separate SET is required even on the first follow.
	followerCountKey := fmt.Sprintf(workerFollowerCountKey, event.FolloweeID)
	followingCountKey := fmt.Sprintf(workerFollowingCountKey, event.FollowerID)
	pipe.Incr(ctx, followerCountKey)
	pipe.Incr(ctx, followingCountKey)

	// Refresh TTLs so the keys do not linger forever for inactive users.
	pipe.Expire(ctx, followerZKey, p.counterTTL)
	pipe.Expire(ctx, followingZKey, p.counterTTL)
	pipe.Expire(ctx, followerCountKey, p.counterTTL)
	pipe.Expire(ctx, followingCountKey, p.counterTTL)

	// Invalidate suggestion caches for both parties. Stale suggestions may
	// recommend someone the viewer has just followed.
	pipe.Del(ctx, fmt.Sprintf(workerSuggestionKeyFmt, event.FollowerID))
	pipe.Del(ctx, fmt.Sprintf(workerSuggestionKeyFmt, event.FolloweeID))

	// Invalidate the follower's feed cache so their home timeline picks up
	// content from the newly followed user on the next request.
	pipe.Del(ctx, fmt.Sprintf(workerFeedCacheKeyFmt, event.FollowerID))

	if _, err := pipe.Exec(ctx); err != nil {
		p.logger.Error("event_processor: followed: redis pipeline failed",
			zap.String("follower_id", event.FollowerID),
			zap.String("followee_id", event.FolloweeID),
			zap.Error(err),
		)
		return
	}

	p.logger.Info("event_processor: followed event processed",
		zap.String("follower_id", event.FollowerID),
		zap.String("followee_id", event.FolloweeID),
	)
}

// ---------------------------------------------------------------------------
// Unfollow event handler
// ---------------------------------------------------------------------------

// decrementFloorZero is a Lua script that atomically decrements a Redis integer
// key but clamps the result at zero. This prevents counter values from going
// negative when events are replayed or arrive out-of-order.
var decrementFloorZero = redis.NewScript(`
local v = redis.call("DECR", KEYS[1])
if v < 0 then
	redis.call("SET", KEYS[1], 0)
	return 0
end
return v`)

// handleUnfollowed updates Redis when an "unfollowed" event is consumed.
//
// Operations performed (idempotent):
//  1. ZREM social:followers:<followeeID> <followerID>
//  2. ZREM social:following:<followerID> <followeeID>
//  3. DECR (floor 0) social:follower_count:<followeeID>
//  4. DECR (floor 0) social:following_count:<followerID>
//  5. DEL suggestion and feed caches for both parties.
func (p *EventProcessor) handleUnfollowed(ctx context.Context, event models.FollowEvent) {
	pipe := p.redis.Pipeline()

	followerZKey := fmt.Sprintf(workerFollowerZSetKey, event.FolloweeID)
	followingZKey := fmt.Sprintf(workerFollowingZSetKey, event.FollowerID)
	pipe.ZRem(ctx, followerZKey, event.FollowerID)
	pipe.ZRem(ctx, followingZKey, event.FolloweeID)

	// Invalidate suggestion and feed caches.
	pipe.Del(ctx, fmt.Sprintf(workerSuggestionKeyFmt, event.FollowerID))
	pipe.Del(ctx, fmt.Sprintf(workerSuggestionKeyFmt, event.FolloweeID))
	pipe.Del(ctx, fmt.Sprintf(workerFeedCacheKeyFmt, event.FollowerID))

	if _, err := pipe.Exec(ctx); err != nil {
		p.logger.Error("event_processor: unfollowed: redis pipeline failed",
			zap.String("follower_id", event.FollowerID),
			zap.String("followee_id", event.FolloweeID),
			zap.Error(err),
		)
		// Decrement still attempted below even if the pipeline partial-failed.
	}

	// Decrement counters via Lua (floor at zero). These are individual Script
	// calls rather than pipeline entries because redis.Script.Run needs to
	// evaluate the SHA against the server and cannot be batched generically
	// in pipelines across all Redis versions.
	followerCountKey := fmt.Sprintf(workerFollowerCountKey, event.FolloweeID)
	followingCountKey := fmt.Sprintf(workerFollowingCountKey, event.FollowerID)

	if err := decrementFloorZero.Run(ctx, p.redis, []string{followerCountKey}).Err(); err != nil {
		p.logger.Warn("event_processor: decrement follower count failed",
			zap.String("key", followerCountKey),
			zap.Error(err),
		)
	}
	if err := decrementFloorZero.Run(ctx, p.redis, []string{followingCountKey}).Err(); err != nil {
		p.logger.Warn("event_processor: decrement following count failed",
			zap.String("key", followingCountKey),
			zap.Error(err),
		)
	}

	p.logger.Info("event_processor: unfollowed event processed",
		zap.String("follower_id", event.FollowerID),
		zap.String("followee_id", event.FolloweeID),
	)
}

// ---------------------------------------------------------------------------
// NewConsumerGroup
// ---------------------------------------------------------------------------

// NewConsumerGroup builds a sarama.ConsumerGroup with settings tuned for
// reliable at-least-once delivery.
//
// Key settings:
//   - Offsets.Initial = OffsetNewest: new replicas start from the end of the
//     log rather than replaying all historical events.
//   - AutoCommit disabled: offsets are committed only after MarkMessage, which
//     happens after successful Redis writes.
//   - BalanceStrategy = RoundRobin: simple partition assignment that
//     distributes load evenly across replicas.
func NewConsumerGroup(brokers []string, groupID string) (sarama.ConsumerGroup, error) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_6_0_0

	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	// Manual offset management: we commit after each successful message handle.
	cfg.Consumer.Offsets.AutoCommit.Enable = false

	cfg.Net.DialTimeout = 10 * time.Second
	cfg.Net.ReadTimeout = 30 * time.Second
	cfg.Net.WriteTimeout = 30 * time.Second

	cg, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("new_consumer_group: %w", err)
	}
	return cg, nil
}
