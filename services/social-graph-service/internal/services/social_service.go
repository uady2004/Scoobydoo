package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/social-graph-service/internal/models"
	"github.com/tiktok-clone/social-graph-service/internal/repositories"
)

// marshalJSON and unmarshalJSON are thin wrappers kept in the services package
// so the suggestion service (same package) can use them without an import cycle.
func marshalJSON(v any) ([]byte, error)      { return json.Marshal(v) }
func unmarshalJSON(data []byte, v any) error { return json.Unmarshal(data, v) }

// ---------------------------------------------------------------------------
// Redis key templates
// ---------------------------------------------------------------------------

const (
	// redisFollowerCountKey is an integer key tracking follower count for a user.
	redisFollowerCountKey = "social:follower_count:%s"
	// redisFollowingCountKey is an integer key tracking following count for a user.
	redisFollowingCountKey = "social:following_count:%s"
	// redisFollowerZSetKey is a sorted set of follower IDs with follow-timestamp scores.
	// ZADD social:followers:<followeeID> <unix_ts> <followerID>
	redisFollowerZSetKey = "social:followers:%s"
	// redisFollowingZSetKey is a sorted set of followee IDs with follow-timestamp scores.
	// ZADD social:following:<followerID> <unix_ts> <followeeID>
	redisFollowingZSetKey = "social:following:%s"
)

// ---------------------------------------------------------------------------
// TopicConfig
// ---------------------------------------------------------------------------

// TopicConfig holds the Kafka topic names used by SocialService.
type TopicConfig struct {
	// UserFollowed is published when a follow edge is created.
	UserFollowed string
	// UserUnfollowed is published when a follow edge is removed.
	UserUnfollowed string
	// FeedInvalidate triggers feed-service to evict the follower's cached feed.
	FeedInvalidate string
	// NotifyFollow triggers the notification-service to push a follow notification.
	NotifyFollow string
}

// ---------------------------------------------------------------------------
// SocialService
// ---------------------------------------------------------------------------

// SocialService implements the business logic for the social graph.
// It co-ordinates the database (via GraphRepository), Redis (for counters and
// sorted-set lists), Kafka (for async event fan-out), and the
// SuggestionService (for BFS-based friend recommendations).
type SocialService struct {
	repo                   repositories.GraphRepository
	redis                  *redis.Client
	producer               sarama.SyncProducer
	suggestionSvc          *SuggestionService
	logger                 *zap.Logger
	topics                 TopicConfig
	notificationServiceURL string
	defaultPageSize        int
	maxPageSize            int
	counterTTL             time.Duration
	httpClient             *http.Client
}

// NewSocialService constructs a SocialService with all its dependencies.
func NewSocialService(
	repo repositories.GraphRepository,
	redisClient *redis.Client,
	producer sarama.SyncProducer,
	suggestionSvc *SuggestionService,
	logger *zap.Logger,
	topics TopicConfig,
	notificationServiceURL string,
	defaultPageSize int,
	maxPageSize int,
	counterTTL time.Duration,
) *SocialService {
	return &SocialService{
		repo:                   repo,
		redis:                  redisClient,
		producer:               producer,
		suggestionSvc:          suggestionSvc,
		logger:                 logger,
		topics:                 topics,
		notificationServiceURL: notificationServiceURL,
		defaultPageSize:        defaultPageSize,
		maxPageSize:            maxPageSize,
		counterTTL:             counterTTL,
		httpClient:             &http.Client{Timeout: 5 * time.Second},
	}
}

// ---------------------------------------------------------------------------
// Follow / Unfollow
// ---------------------------------------------------------------------------

// Follow creates a follow edge from followerID to followeeID.
//
// On success it:
//  1. Inserts the edge into PostgreSQL via the repository.
//  2. ZADD followerID into social:followers:<followeeID> (score = unix timestamp).
//  3. ZADD followeeID into social:following:<followerID> (score = unix timestamp).
//  4. INCR the follower count for followeeID.
//  5. INCR the following count for followerID.
//  6. Asynchronously publishes a UserFollowed Kafka event (user.followed topic),
//     a feed-invalidation event (feed.invalidate topic), and a notification
//     event (notification.follow topic).
//  7. Asynchronously invalidates suggestion caches for both parties.
func (s *SocialService) Follow(ctx context.Context, followerID, followeeID string) (*models.Follow, error) {
	follow, err := s.repo.Follow(ctx, followerID, followeeID)
	if err != nil {
		return nil, err
	}

	// Update Redis sorted-set follower lists. Score = follow timestamp (Unix seconds)
	// so ZREVRANGEBYSCORE gives newest followers first.
	ts := float64(follow.CreatedAt.Unix())

	pipe := s.redis.Pipeline()
	followerZKey := fmt.Sprintf(redisFollowerZSetKey, followeeID)
	followingZKey := fmt.Sprintf(redisFollowingZSetKey, followerID)

	// Add the follower to the followee's follower set.
	pipe.ZAdd(ctx, followerZKey, redis.Z{Score: ts, Member: followerID})
	// Add the followee to the follower's following set.
	pipe.ZAdd(ctx, followingZKey, redis.Z{Score: ts, Member: followeeID})

	// Increment counters. INCR initialises at 0 if absent.
	followerCountKey := fmt.Sprintf(redisFollowerCountKey, followeeID)
	followingCountKey := fmt.Sprintf(redisFollowingCountKey, followerID)
	pipe.Incr(ctx, followerCountKey)
	pipe.Incr(ctx, followingCountKey)

	// Refresh TTLs so rarely-followed users don't leak stale keys forever.
	pipe.Expire(ctx, followerZKey, s.counterTTL)
	pipe.Expire(ctx, followingZKey, s.counterTTL)
	pipe.Expire(ctx, followerCountKey, s.counterTTL)
	pipe.Expire(ctx, followingCountKey, s.counterTTL)

	if _, pipeErr := pipe.Exec(ctx); pipeErr != nil {
		// Non-fatal: PostgreSQL is the source of truth.
		s.logger.Warn("follow: redis pipeline failed",
			zap.String("follower_id", followerID),
			zap.String("followee_id", followeeID),
			zap.Error(pipeErr),
		)
	}

	// Emit Kafka events and invalidate caches asynchronously so the HTTP
	// response is not delayed by broker round-trips.
	event := models.FollowEvent{
		EventType:  "followed",
		FollowerID: followerID,
		FolloweeID: followeeID,
		OccurredAt: follow.CreatedAt,
	}
	go s.publishFollowEvent(event)

	go func() {
		_ = s.suggestionSvc.InvalidateSuggestionsCache(context.Background(), followerID)
		_ = s.suggestionSvc.InvalidateSuggestionsCache(context.Background(), followeeID)
	}()

	return follow, nil
}

// Unfollow removes the follow edge from followerID to followeeID.
//
// On success it:
//  1. Deletes the edge from PostgreSQL.
//  2. ZREM followerID from social:followers:<followeeID>.
//  3. ZREM followeeID from social:following:<followerID>.
//  4. Decrements the follower/following counters with a floor of 0 via Lua.
//  5. Asynchronously publishes a UserUnfollowed Kafka event and a
//     feed-invalidation event.
//  6. Asynchronously invalidates suggestion caches for both parties.
func (s *SocialService) Unfollow(ctx context.Context, followerID, followeeID string) error {
	if err := s.repo.Unfollow(ctx, followerID, followeeID); err != nil {
		return err
	}

	pipe := s.redis.Pipeline()

	followerZKey := fmt.Sprintf(redisFollowerZSetKey, followeeID)
	followingZKey := fmt.Sprintf(redisFollowingZSetKey, followerID)
	pipe.ZRem(ctx, followerZKey, followerID)
	pipe.ZRem(ctx, followingZKey, followeeID)

	if _, pipeErr := pipe.Exec(ctx); pipeErr != nil {
		s.logger.Warn("unfollow: redis pipeline failed",
			zap.String("follower_id", followerID),
			zap.String("followee_id", followeeID),
			zap.Error(pipeErr),
		)
	}

	// Decrement counters via a Lua script that floors at zero, preventing
	// negative counter values caused by event duplication or race conditions.
	followerCountKey := fmt.Sprintf(redisFollowerCountKey, followeeID)
	followingCountKey := fmt.Sprintf(redisFollowingCountKey, followerID)
	if err := decrementFloorZeroScript.Run(ctx, s.redis, []string{followerCountKey}).Err(); err != nil && !errors.Is(err, redis.Nil) {
		s.logger.Warn("unfollow: decrement follower count failed",
			zap.String("key", followerCountKey),
			zap.Error(err),
		)
	}
	if err := decrementFloorZeroScript.Run(ctx, s.redis, []string{followingCountKey}).Err(); err != nil && !errors.Is(err, redis.Nil) {
		s.logger.Warn("unfollow: decrement following count failed",
			zap.String("key", followingCountKey),
			zap.Error(err),
		)
	}

	event := models.FollowEvent{
		EventType:  "unfollowed",
		FollowerID: followerID,
		FolloweeID: followeeID,
		OccurredAt: time.Now().UTC(),
	}
	go s.publishFollowEvent(event)

	go func() {
		_ = s.suggestionSvc.InvalidateSuggestionsCache(context.Background(), followerID)
		_ = s.suggestionSvc.InvalidateSuggestionsCache(context.Background(), followeeID)
	}()

	return nil
}

// decrementFloorZeroScript decrements a Redis integer key but clamps the
// result at zero to prevent negative counters.
var decrementFloorZeroScript = redis.NewScript(`
local v = redis.call("DECR", KEYS[1])
if v < 0 then
	redis.call("SET", KEYS[1], 0)
	return 0
end
return v`)

// ---------------------------------------------------------------------------
// List operations (paginated)
// ---------------------------------------------------------------------------

// GetFollowers returns a paginated list of users who follow targetID.
func (s *SocialService) GetFollowers(ctx context.Context, targetID string, limit, offset int) (*models.FollowListResponse, error) {
	limit, offset = s.normalisePagination(limit, offset)

	follows, total, err := s.repo.GetFollowers(ctx, targetID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get_followers: %w", err)
	}

	items := make([]models.FollowWithUser, len(follows))
	for i, f := range follows {
		items[i] = models.FollowWithUser{
			Follow: f,
			// UserSummary is populated with the follower's ID. In a fully wired
			// deployment this would be enriched via a gRPC call to user-service.
			User: models.UserSummary{UserID: f.FollowerID},
		}
	}

	return &models.FollowListResponse{
		Users: items,
		Pagination: models.PaginationMeta{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: int64(offset+limit) < total,
		},
	}, nil
}

// GetFollowing returns a paginated list of users that viewerID follows.
func (s *SocialService) GetFollowing(ctx context.Context, viewerID string, limit, offset int) (*models.FollowListResponse, error) {
	limit, offset = s.normalisePagination(limit, offset)

	follows, total, err := s.repo.GetFollowing(ctx, viewerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get_following: %w", err)
	}

	items := make([]models.FollowWithUser, len(follows))
	for i, f := range follows {
		items[i] = models.FollowWithUser{
			Follow:      f,
			User:        models.UserSummary{UserID: f.FolloweeID},
			IsFollowing: true, // viewer follows all items in this list by definition
		}
	}

	return &models.FollowListResponse{
		Users: items,
		Pagination: models.PaginationMeta{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: int64(offset+limit) < total,
		},
	}, nil
}

// GetMutualFollowers returns a paginated list of users who follow both userID
// and targetID.
func (s *SocialService) GetMutualFollowers(ctx context.Context, userID, targetID string, limit, offset int) (*models.FollowListResponse, error) {
	limit, offset = s.normalisePagination(limit, offset)

	follows, total, err := s.repo.GetMutualFollowers(ctx, userID, targetID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get_mutual_followers: %w", err)
	}

	items := make([]models.FollowWithUser, len(follows))
	for i, f := range follows {
		items[i] = models.FollowWithUser{
			Follow: f,
			User:   models.UserSummary{UserID: f.FollowerID},
		}
	}

	return &models.FollowListResponse{
		Users: items,
		Pagination: models.PaginationMeta{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: int64(offset+limit) < total,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Relationship checks
// ---------------------------------------------------------------------------

// CheckRelationship builds a full Relationship view between userID and targetID
// by making four parallel database lookups (two IsFollowing, two IsBlocked).
func (s *SocialService) CheckRelationship(ctx context.Context, userID, targetID string) (*models.Relationship, error) {
	if userID == targetID {
		return &models.Relationship{
			UserID:   userID,
			TargetID: targetID,
			Status:   models.RelationshipNone,
		}, nil
	}

	isFollowing, err := s.repo.IsFollowing(ctx, userID, targetID)
	if err != nil {
		return nil, fmt.Errorf("check_relationship: is_following: %w", err)
	}

	isFollowedBy, err := s.repo.IsFollowing(ctx, targetID, userID)
	if err != nil {
		return nil, fmt.Errorf("check_relationship: is_followed_by: %w", err)
	}

	isBlocking, err := s.repo.IsBlocked(ctx, userID, targetID)
	if err != nil {
		return nil, fmt.Errorf("check_relationship: is_blocking: %w", err)
	}

	isBlockedBy, err := s.repo.IsBlocked(ctx, targetID, userID)
	if err != nil {
		return nil, fmt.Errorf("check_relationship: is_blocked_by: %w", err)
	}

	rel := &models.Relationship{
		UserID:       userID,
		TargetID:     targetID,
		IsFollowing:  isFollowing,
		IsFollowedBy: isFollowedBy,
		IsBlocking:   isBlocking,
		IsBlockedBy:  isBlockedBy,
	}
	rel.DeriveStatus()
	return rel, nil
}

// ---------------------------------------------------------------------------
// Counters (Redis-first, DB fallback)
// ---------------------------------------------------------------------------

// GetFollowerCount returns the follower count for userID. Redis is checked
// first; on a cache miss the value is read from PostgreSQL and the cache is
// warmed up.
func (s *SocialService) GetFollowerCount(ctx context.Context, userID string) (int64, error) {
	key := fmt.Sprintf(redisFollowerCountKey, userID)
	val, err := s.redis.Get(ctx, key).Int64()
	if err == nil {
		return val, nil
	}
	if !errors.Is(err, redis.Nil) {
		s.logger.Warn("get_follower_count: redis error",
			zap.String("user_id", userID),
			zap.Error(err),
		)
	}

	// Database fallback.
	count, dbErr := s.repo.GetFollowerCount(ctx, userID)
	if dbErr != nil {
		return 0, fmt.Errorf("get_follower_count: db: %w", dbErr)
	}

	// Warm the cache.
	_ = s.redis.Set(ctx, key, strconv.FormatInt(count, 10), s.counterTTL).Err()
	return count, nil
}

// GetFollowingCount returns the following count for userID from Redis, falling
// back to PostgreSQL on a cache miss.
func (s *SocialService) GetFollowingCount(ctx context.Context, userID string) (int64, error) {
	key := fmt.Sprintf(redisFollowingCountKey, userID)
	val, err := s.redis.Get(ctx, key).Int64()
	if err == nil {
		return val, nil
	}
	if !errors.Is(err, redis.Nil) {
		s.logger.Warn("get_following_count: redis error",
			zap.String("user_id", userID),
			zap.Error(err),
		)
	}

	count, dbErr := s.repo.GetFollowingCount(ctx, userID)
	if dbErr != nil {
		return 0, fmt.Errorf("get_following_count: db: %w", dbErr)
	}
	_ = s.redis.Set(ctx, key, strconv.FormatInt(count, 10), s.counterTTL).Err()
	return count, nil
}

// ---------------------------------------------------------------------------
// Friend suggestions (delegates to SuggestionService)
// ---------------------------------------------------------------------------

// GetFriendSuggestions returns BFS-based friend suggestions for viewerID.
func (s *SocialService) GetFriendSuggestions(ctx context.Context, viewerID string) (*models.SuggestionListResponse, error) {
	suggestions, err := s.suggestionSvc.GetSuggestions(ctx, viewerID)
	if err != nil {
		return nil, fmt.Errorf("get_friend_suggestions: %w", err)
	}
	return &models.SuggestionListResponse{
		Suggestions: suggestions,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

// ---------------------------------------------------------------------------
// Block operations
// ---------------------------------------------------------------------------

// BlockUser blocks blockedID on behalf of blockerID and cleans up all
// associated Redis caches.
func (s *SocialService) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	if err := s.repo.BlockUser(ctx, blockerID, blockedID); err != nil {
		return fmt.Errorf("block_user: %w", err)
	}

	// Invalidate sorted-set lists and counter caches for both parties since
	// the block operation removes follow edges in both directions.
	pipe := s.redis.Pipeline()
	for _, uid := range []string{blockerID, blockedID} {
		pipe.Del(ctx, fmt.Sprintf(redisFollowerZSetKey, uid))
		pipe.Del(ctx, fmt.Sprintf(redisFollowingZSetKey, uid))
		pipe.Del(ctx, fmt.Sprintf(redisFollowerCountKey, uid))
		pipe.Del(ctx, fmt.Sprintf(redisFollowingCountKey, uid))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.Warn("block_user: redis cleanup failed", zap.Error(err))
	}

	go func() {
		_ = s.suggestionSvc.InvalidateSuggestionsCache(context.Background(), blockerID)
		_ = s.suggestionSvc.InvalidateSuggestionsCache(context.Background(), blockedID)
	}()

	return nil
}

// GetBlockList returns a paginated list of users that userID has blocked.
func (s *SocialService) GetBlockList(ctx context.Context, userID string, limit, offset int) ([]models.Block, int64, error) {
	limit, offset = s.normalisePagination(limit, offset)
	return s.repo.GetBlockList(ctx, userID, limit, offset)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// publishFollowEvent marshals a FollowEvent to JSON and sends it to the
// appropriate Kafka topics. Any error is logged but not propagated; the
// function is designed to be run in a goroutine.
//
// Topics published to:
//   - user.followed / user.unfollowed — consumed by downstream services that
//     react to graph changes (analytics, search indexing, etc.)
//   - feed.invalidate — consumed by feed-service to evict the follower's feed
//     cache so new content from the followee appears immediately.
//   - notification.follow (follows only) — consumed by notification-service to
//     push a "X started following you" notification to the followee.
func (s *SocialService) publishFollowEvent(event models.FollowEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		s.logger.Error("follow event: marshal failed", zap.Error(err))
		return
	}

	// Primary event topic.
	topic := s.topics.UserFollowed
	if event.EventType == "unfollowed" {
		topic = s.topics.UserUnfollowed
	}

	msgs := []*sarama.ProducerMessage{
		{
			Topic: topic,
			Key:   sarama.StringEncoder(event.FollowerID),
			Value: sarama.ByteEncoder(data),
		},
		{
			// Feed invalidation: the follower's home feed must refresh so they
			// start seeing (or stop seeing) the followee's content.
			Topic: s.topics.FeedInvalidate,
			Key:   sarama.StringEncoder(event.FollowerID),
			Value: sarama.ByteEncoder(data),
		},
	}

	if event.EventType == "followed" {
		msgs = append(msgs, &sarama.ProducerMessage{
			// Notify the followee that they have a new follower.
			Topic: s.topics.NotifyFollow,
			Key:   sarama.StringEncoder(event.FolloweeID),
			Value: sarama.ByteEncoder(data),
		})
	}

	for _, msg := range msgs {
		if _, _, sendErr := s.producer.SendMessage(msg); sendErr != nil {
			s.logger.Error("follow event: kafka send failed",
				zap.String("topic", msg.Topic),
				zap.String("event_type", event.EventType),
				zap.Error(sendErr),
			)
		} else {
			s.logger.Debug("follow event published",
				zap.String("topic", msg.Topic),
				zap.String("event_type", event.EventType),
				zap.String("follower_id", event.FollowerID),
				zap.String("followee_id", event.FolloweeID),
			)
		}
	}
}

// normalisePagination clamps limit to [1, maxPageSize] and ensures offset >= 0.
func (s *SocialService) normalisePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = s.defaultPageSize
	}
	if limit > s.maxPageSize {
		limit = s.maxPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
