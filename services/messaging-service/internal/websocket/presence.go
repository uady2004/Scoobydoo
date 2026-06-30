package websocket

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// onlineKeyTTL is how long a user's online presence key lives without refresh.
	onlineKeyTTL = 5 * time.Minute

	// typingKeyTTL is how long a typing indicator remains set before it auto-expires.
	// The client is expected to emit typing_stop before this, but the TTL acts as a
	// safety net for dropped connections.
	typingKeyTTL = 3 * time.Second

	// lastSeenKeyPrefix is the Redis key prefix for storing last-seen timestamps.
	lastSeenKeyPrefix = "presence:lastseen:"

	// onlineKeyPrefix is the Redis key prefix for marking a user as online.
	onlineKeyPrefix = "presence:online:"

	// typingKeyPrefix is the Redis key prefix for typing indicators.
	// Key format: presence:typing:<conversationID>:<userID>
	typingKeyPrefix = "presence:typing:"
)

// PresenceService manages user online/offline state and typing indicators in Redis.
//
// Keys used:
//
//	presence:online:<userID>       — string "1", TTL 5m (refreshed by heartbeat)
//	presence:lastseen:<userID>     — RFC3339 timestamp, no expiry
//	presence:typing:<convID>:<uid> — string "1", TTL 3s
type PresenceService struct {
	rdb *redis.Client
	log *zap.Logger
}

// NewPresenceService returns a new PresenceService backed by the given Redis client.
func NewPresenceService(rdb *redis.Client, log *zap.Logger) *PresenceService {
	return &PresenceService{rdb: rdb, log: log}
}

// ---------------------------------------------------------------------------
// Online / offline
// ---------------------------------------------------------------------------

// SetOnline marks the user as online in Redis.
// It should be called when a WebSocket connection is established and periodically
// refreshed to prevent TTL expiry.
func (p *PresenceService) SetOnline(userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := onlineKeyPrefix + userID.String()
	if err := p.rdb.Set(ctx, key, "1", onlineKeyTTL).Err(); err != nil {
		p.log.Warn("presence: failed to set online", zap.String("user_id", userID.String()), zap.Error(err))
	}
}

// SetOffline removes the user's online key and records the current time as
// their last-seen timestamp. It should be called when all WebSocket connections
// for a user are closed.
func (p *PresenceService) SetOffline(userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	onlineKey := onlineKeyPrefix + userID.String()
	lastSeenKey := lastSeenKeyPrefix + userID.String()
	now := time.Now().UTC().Format(time.RFC3339)

	pipe := p.rdb.Pipeline()
	pipe.Del(ctx, onlineKey)
	pipe.Set(ctx, lastSeenKey, now, 0) // persist indefinitely
	if _, err := pipe.Exec(ctx); err != nil {
		p.log.Warn("presence: failed to set offline", zap.String("user_id", userID.String()), zap.Error(err))
	}
}

// IsOnline returns true when the user's online key exists in Redis.
func (p *PresenceService) IsOnline(ctx context.Context, userID uuid.UUID) (bool, error) {
	key := onlineKeyPrefix + userID.String()
	val, err := p.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("presence: IsOnline: %w", err)
	}
	return val > 0, nil
}

// LastSeen returns the time the user was last seen online.
// Returns zero time if no record exists.
func (p *PresenceService) LastSeen(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	key := lastSeenKeyPrefix + userID.String()
	val, err := p.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("presence: LastSeen: %w", err)
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, fmt.Errorf("presence: parse last seen: %w", err)
	}
	return t, nil
}

// RefreshOnline resets the TTL of the user's online key. Call it on any activity
// (e.g., WebSocket heartbeat) to prevent premature expiry.
func (p *PresenceService) RefreshOnline(userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := onlineKeyPrefix + userID.String()
	if err := p.rdb.Expire(ctx, key, onlineKeyTTL).Err(); err != nil {
		p.log.Warn("presence: failed to refresh online TTL",
			zap.String("user_id", userID.String()), zap.Error(err))
	}
}

// ---------------------------------------------------------------------------
// Typing indicators
// ---------------------------------------------------------------------------

// SetTyping marks the user as currently typing in the given conversation.
// The key auto-expires after typingKeyTTL (3 seconds) as a safety net.
func (p *PresenceService) SetTyping(userID, conversationID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := typingKey(conversationID, userID)
	if err := p.rdb.Set(ctx, key, "1", typingKeyTTL).Err(); err != nil {
		p.log.Warn("presence: failed to set typing",
			zap.String("user_id", userID.String()),
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
	}
}

// ClearTyping removes the user's typing indicator for a conversation.
// Should be called on typing_stop or when the connection closes.
func (p *PresenceService) ClearTyping(userID, conversationID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := typingKey(conversationID, userID)
	if err := p.rdb.Del(ctx, key).Err(); err != nil {
		p.log.Warn("presence: failed to clear typing",
			zap.String("user_id", userID.String()),
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
	}
}

// IsTyping returns true when the user currently has an active typing indicator
// in the given conversation.
func (p *PresenceService) IsTyping(ctx context.Context, userID, conversationID uuid.UUID) (bool, error) {
	key := typingKey(conversationID, userID)
	val, err := p.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("presence: IsTyping: %w", err)
	}
	return val > 0, nil
}

// TypingUsers returns the list of user IDs currently typing in the given conversation.
// It performs a SCAN with the conversation-scoped prefix to find active keys.
func (p *PresenceService) TypingUsers(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	pattern := typingKeyPrefix + conversationID.String() + ":*"
	var cursor uint64
	var userIDs []uuid.UUID

	for {
		keys, nextCursor, err := p.rdb.Scan(ctx, cursor, pattern, 50).Result()
		if err != nil {
			return nil, fmt.Errorf("presence: TypingUsers scan: %w", err)
		}
		for _, k := range keys {
			// Key format: presence:typing:<convID>:<userID>
			prefix := typingKeyPrefix + conversationID.String() + ":"
			if len(k) <= len(prefix) {
				continue
			}
			uidStr := k[len(prefix):]
			uid, err := uuid.Parse(uidStr)
			if err == nil {
				userIDs = append(userIDs, uid)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return userIDs, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func typingKey(conversationID, userID uuid.UUID) string {
	return typingKeyPrefix + conversationID.String() + ":" + userID.String()
}
