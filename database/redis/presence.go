package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Presence key patterns:
//
//	presence:{userID}              -> JSON PresenceInfo blob, TTL = PresenceTTL (STRING)
//	presence:online                -> sorted set userID -> last heartbeat unix ts (ZSET)
//	presence:room:{roomID}         -> sorted set userID -> joined unix ts         (ZSET)
//	typing:{ctxType}:{ctxID}       -> sorted set userID -> typing_started unix ms (ZSET)
//	presence:friends:{userID}      -> sorted set friendID -> last_seen unix ts    (ZSET, short TTL)
//	presence:watch:{videoID}       -> sorted set userID -> watch_start unix ts    (ZSET, short TTL)
//	presence:live:{streamID}       -> sorted set userID -> joined unix ts         (ZSET, short TTL)
const (
	// HeartbeatInterval is the expected client keep-alive cadence.
	HeartbeatInterval = 30 * time.Second

	// OnlineThreshold is the maximum age of a heartbeat to still be considered "online".
	// = HeartbeatInterval × 3 to tolerate two missed heartbeats before marking away.
	OnlineThreshold = 90 * time.Second

	// PresenceTTL is the lifetime of per-user presence STRING keys.
	// Longer than OnlineThreshold so last-seen remains queryable after going offline.
	PresenceTTL = 5 * time.Minute

	// OfflineRetentionTTL is how long the presence STRING key is kept after a user
	// explicitly goes offline, to serve "last seen N minutes ago" display.
	OfflineRetentionTTL = 30 * 24 * time.Hour

	// TypingTTL is how long a typing indicator persists without a refresh.
	TypingTTL = 10 * time.Second

	// RoomPresenceTTL is the TTL for room membership sorted sets.
	RoomPresenceTTL = 24 * time.Hour

	// FriendsPresenceTTL is the cache lifetime for a user's online-friends list.
	FriendsPresenceTTL = 2 * time.Minute

	// WatchPresenceTTL is the TTL for the concurrent-viewers sorted set on a video.
	WatchPresenceTTL = 10 * time.Minute

	// LivePresenceTTL is the TTL for the live-stream viewer sorted set.
	LivePresenceTTL = 5 * time.Minute

	// Status constants surfaced to the client.
	StatusOnline  = "online"
	StatusAway    = "away"
	StatusOffline = "offline"

	// Context type constants for typing indicators.
	ContextDM       = "dm"        // direct message thread
	ContextLiveRoom = "live_room" // live stream chat
	ContextComments = "comments"  // video comment section

	keyPresence        = "presence:%s"
	keyPresenceOnline  = "presence:online"
	keyPresenceRoom    = "presence:room:%s"
	keyTyping          = "typing:%s:%s"
	keyFriendsPresence = "presence:friends:%s"
	keyWatchPresence   = "presence:watch:%s"
	keyLivePresence    = "presence:live:%s"
)

// PresenceInfo is stored per user in a Redis STRING key.
type PresenceInfo struct {
	UserID    string    `json:"user_id"`
	Status    string    `json:"status"`
	LastSeen  time.Time `json:"last_seen"`
	// Device is "mobile", "web", or "desktop".
	Device    string    `json:"device"`
	// Platform is "ios", "android", or "web".
	Platform  string    `json:"platform"`
	SessionID string    `json:"session_id"`
	// AppVersion allows the backend to know what client version is active.
	AppVersion string   `json:"app_version,omitempty"`
}

// FriendPresence is returned when listing a user's online friends.
type FriendPresence struct {
	UserID   string    `json:"user_id"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen"`
}

// TypingEvent describes a user currently typing in a context.
type TypingEvent struct {
	UserID    string    `json:"user_id"`
	ContextID string    `json:"context_id"`
	StartedAt time.Time `json:"started_at"`
}

// PresenceTracker manages online status, room presence, concurrent viewers,
// and typing indicators.
type PresenceTracker struct {
	client *goredis.Client
	// luaHeartbeat updates both the presence STRING and the online ZSET in one round-trip.
	luaHeartbeat *goredis.Script
}

// NewPresenceTracker constructs a PresenceTracker and pre-compiles Lua scripts.
func NewPresenceTracker(client *goredis.Client) *PresenceTracker {
	return &PresenceTracker{
		client: client,
		luaHeartbeat: goredis.NewScript(`
			-- KEYS[1] = presence:{userID} (STRING)
			-- KEYS[2] = presence:online   (ZSET)
			-- ARGV[1] = serialised JSON blob
			-- ARGV[2] = unix timestamp seconds (score for ZSET)
			-- ARGV[3] = presence TTL in seconds
			-- ARGV[4] = userID (ZSET member)
			redis.call('SET', KEYS[1], ARGV[1], 'EX', tonumber(ARGV[3]))
			redis.call('ZADD', KEYS[2], tonumber(ARGV[2]), ARGV[4])
			return 1
		`),
	}
}

// ----------------------------------------------------------------------------
// Heartbeat / online status
// ----------------------------------------------------------------------------

// Heartbeat marks a user as online and refreshes their presence data.
// Clients must call this endpoint every HeartbeatInterval.
// Uses a Lua script so both the STRING and the ZSET are updated atomically.
func (pt *PresenceTracker) Heartbeat(ctx context.Context, info PresenceInfo) error {
	now := time.Now().UTC()
	info.LastSeen = now
	info.Status = StatusOnline

	raw, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal presence: %w", err)
	}

	presenceKey := fmt.Sprintf(keyPresence, info.UserID)
	return pt.luaHeartbeat.Run(ctx, pt.client,
		[]string{presenceKey, keyPresenceOnline},
		string(raw),
		now.Unix(),
		int(PresenceTTL.Seconds()),
		info.UserID,
	).Err()
}

// SetOffline explicitly marks a user as offline (called on logout or WebSocket
// disconnect). Updates the presence STRING to reflect offline status but keeps
// the key for OfflineRetentionTTL so "last seen" remains queryable.
func (pt *PresenceTracker) SetOffline(ctx context.Context, userID string) error {
	presenceKey := fmt.Sprintf(keyPresence, userID)

	raw, err := pt.client.Get(ctx, presenceKey).Bytes()
	pipe := pt.client.TxPipeline()

	if err == nil {
		var info PresenceInfo
		if jsonErr := json.Unmarshal(raw, &info); jsonErr == nil {
			info.Status = StatusOffline
			info.LastSeen = time.Now().UTC()
			updated, _ := json.Marshal(info)
			pipe.Set(ctx, presenceKey, updated, OfflineRetentionTTL)
		}
	} else if !errors.Is(err, goredis.Nil) {
		return fmt.Errorf("get presence for offline: %w", err)
	}

	pipe.ZRem(ctx, keyPresenceOnline, userID)
	_, execErr := pipe.Exec(ctx)
	return execErr
}

// GetPresence returns the current presence info for a user.
// The Status field is derived from the ZSET heartbeat age, not the stored value,
// to ensure it reflects network-level reality.
func (pt *PresenceTracker) GetPresence(ctx context.Context, userID string) (*PresenceInfo, error) {
	key := fmt.Sprintf(keyPresence, userID)
	raw, err := pt.client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return &PresenceInfo{UserID: userID, Status: StatusOffline}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get presence: %w", err)
	}

	var info PresenceInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("unmarshal presence: %w", err)
	}

	// Derive live status from the ZSET score (heartbeat age).
	score, zsErr := pt.client.ZScore(ctx, keyPresenceOnline, userID).Result()
	switch {
	case errors.Is(zsErr, goredis.Nil):
		info.Status = StatusOffline
	case zsErr == nil:
		age := time.Since(time.Unix(int64(score), 0))
		if age <= OnlineThreshold {
			info.Status = StatusOnline
		} else {
			info.Status = StatusAway
		}
	}
	return &info, nil
}

// IsOnline returns true if the user has sent a heartbeat within OnlineThreshold.
func (pt *PresenceTracker) IsOnline(ctx context.Context, userID string) (bool, error) {
	score, err := pt.client.ZScore(ctx, keyPresenceOnline, userID).Result()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return time.Since(time.Unix(int64(score), 0)) <= OnlineThreshold, nil
}

// GetBulkPresence fetches presence for multiple users in a single round-trip.
// ZMSCORE is used to check heartbeat ages in a second single round-trip.
func (pt *PresenceTracker) GetBulkPresence(ctx context.Context, userIDs []string) (map[string]*PresenceInfo, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	keys := make([]string, len(userIDs))
	for i, uid := range userIDs {
		keys[i] = fmt.Sprintf(keyPresence, uid)
	}

	rawValues, err := pt.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget bulk presence: %w", err)
	}

	result := make(map[string]*PresenceInfo, len(userIDs))
	for i, v := range rawValues {
		uid := userIDs[i]
		if v == nil {
			result[uid] = &PresenceInfo{UserID: uid, Status: StatusOffline}
			continue
		}
		var info PresenceInfo
		if jsonErr := json.Unmarshal([]byte(v.(string)), &info); jsonErr != nil {
			result[uid] = &PresenceInfo{UserID: uid, Status: StatusOffline}
			continue
		}
		result[uid] = &info
	}

	// Derive live statuses from the ZSET in one ZMSCORE call (Redis 6.2+).
	userIDInterfaces := make([]interface{}, len(userIDs))
	for i, uid := range userIDs {
		userIDInterfaces[i] = uid
	}
	scores, zsErr := pt.client.ZMScore(ctx, keyPresenceOnline, userIDs...).Result()
	if zsErr == nil {
		now := time.Now()
		for i, uid := range userIDs {
			if scores[i] == 0 {
				result[uid].Status = StatusOffline
				continue
			}
			age := now.Sub(time.Unix(int64(scores[i]), 0))
			if age <= OnlineThreshold {
				result[uid].Status = StatusOnline
			} else {
				result[uid].Status = StatusAway
			}
		}
	}
	return result, nil
}

// PruneStaleOnlineUsers removes users from the online ZSET whose heartbeat
// has exceeded PresenceTTL. Call from a background goroutine every minute.
// Returns the number of stale users removed.
func (pt *PresenceTracker) PruneStaleOnlineUsers(ctx context.Context) (int64, error) {
	cutoff := float64(time.Now().Add(-PresenceTTL).Unix())
	return pt.client.ZRemRangeByScore(ctx, keyPresenceOnline,
		"-inf", fmt.Sprintf("%v", cutoff),
	).Result()
}

// CountOnlineUsers returns the number of users with a heartbeat within OnlineThreshold.
func (pt *PresenceTracker) CountOnlineUsers(ctx context.Context) (int64, error) {
	cutoff := float64(time.Now().Add(-OnlineThreshold).Unix())
	return pt.client.ZCount(ctx, keyPresenceOnline,
		fmt.Sprintf("%v", cutoff), "+inf",
	).Result()
}

// ----------------------------------------------------------------------------
// Room presence (live stream chat rooms, DM group rooms)
// ----------------------------------------------------------------------------

// JoinRoom adds a user to a room's presence sorted set.
// Score = unix timestamp so members can be ordered by join time.
func (pt *PresenceTracker) JoinRoom(ctx context.Context, roomID, userID string) error {
	key := fmt.Sprintf(keyPresenceRoom, roomID)
	pipe := pt.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{
		Score:  float64(time.Now().Unix()),
		Member: userID,
	})
	pipe.Expire(ctx, key, RoomPresenceTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// LeaveRoom removes a user from a room.
func (pt *PresenceTracker) LeaveRoom(ctx context.Context, roomID, userID string) error {
	return pt.client.ZRem(ctx, fmt.Sprintf(keyPresenceRoom, roomID), userID).Err()
}

// RefreshRoomMembership updates the user's join timestamp (keep-alive for room presence).
func (pt *PresenceTracker) RefreshRoomMembership(ctx context.Context, roomID, userID string) error {
	key := fmt.Sprintf(keyPresenceRoom, roomID)
	pipe := pt.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(time.Now().Unix()), Member: userID})
	pipe.Expire(ctx, key, RoomPresenceTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// GetRoomMembers returns all user IDs currently in the room, ordered by join time.
func (pt *PresenceTracker) GetRoomMembers(ctx context.Context, roomID string) ([]string, error) {
	return pt.client.ZRange(ctx, fmt.Sprintf(keyPresenceRoom, roomID), 0, -1).Result()
}

// GetRoomMembersWithJoinTime returns members with their join timestamps.
func (pt *PresenceTracker) GetRoomMembersWithJoinTime(ctx context.Context, roomID string) ([]goredis.Z, error) {
	return pt.client.ZRangeWithScores(ctx, fmt.Sprintf(keyPresenceRoom, roomID), 0, -1).Result()
}

// CountRoomMembers returns the number of users currently in a room.
func (pt *PresenceTracker) CountRoomMembers(ctx context.Context, roomID string) (int64, error) {
	return pt.client.ZCard(ctx, fmt.Sprintf(keyPresenceRoom, roomID)).Result()
}

// PruneRoomMembers removes stale room members (those who have not refreshed
// their membership within maxAge). Returns the count removed.
func (pt *PresenceTracker) PruneRoomMembers(ctx context.Context, roomID string, maxAge time.Duration) (int64, error) {
	cutoff := float64(time.Now().Add(-maxAge).Unix())
	key := fmt.Sprintf(keyPresenceRoom, roomID)
	return pt.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%v", cutoff)).Result()
}

// ClearRoom deletes a room's presence set entirely (on room closure).
func (pt *PresenceTracker) ClearRoom(ctx context.Context, roomID string) error {
	return pt.client.Del(ctx, fmt.Sprintf(keyPresenceRoom, roomID)).Err()
}

// IsInRoom reports whether a user is currently a member of a room.
func (pt *PresenceTracker) IsInRoom(ctx context.Context, roomID, userID string) (bool, error) {
	_, err := pt.client.ZScore(ctx, fmt.Sprintf(keyPresenceRoom, roomID), userID).Result()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	return err == nil, err
}

// ----------------------------------------------------------------------------
// Concurrent video viewers (for view-count display on video cards)
// ----------------------------------------------------------------------------

// StartWatching records that a user has started watching a video.
func (pt *PresenceTracker) StartWatching(ctx context.Context, videoID, userID string) error {
	key := fmt.Sprintf(keyWatchPresence, videoID)
	pipe := pt.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(time.Now().Unix()), Member: userID})
	pipe.Expire(ctx, key, WatchPresenceTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// StopWatching removes a user from the video's active-viewer set.
func (pt *PresenceTracker) StopWatching(ctx context.Context, videoID, userID string) error {
	return pt.client.ZRem(ctx, fmt.Sprintf(keyWatchPresence, videoID), userID).Err()
}

// CountActiveViewers returns the number of users actively watching a video.
// Viewers whose entry is older than WatchPresenceTTL are excluded by score filter.
func (pt *PresenceTracker) CountActiveViewers(ctx context.Context, videoID string) (int64, error) {
	cutoff := float64(time.Now().Add(-WatchPresenceTTL).Unix())
	return pt.client.ZCount(ctx, fmt.Sprintf(keyWatchPresence, videoID),
		fmt.Sprintf("%v", cutoff), "+inf",
	).Result()
}

// ----------------------------------------------------------------------------
// Live stream presence (real-time viewer list for host dashboard)
// ----------------------------------------------------------------------------

// JoinLiveStream records a viewer joining a live stream.
func (pt *PresenceTracker) JoinLiveStream(ctx context.Context, streamID, userID string) error {
	key := fmt.Sprintf(keyLivePresence, streamID)
	pipe := pt.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(time.Now().Unix()), Member: userID})
	pipe.Expire(ctx, key, LivePresenceTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// LeaveLiveStream removes a viewer from the live stream presence set.
func (pt *PresenceTracker) LeaveLiveStream(ctx context.Context, streamID, userID string) error {
	return pt.client.ZRem(ctx, fmt.Sprintf(keyLivePresence, streamID), userID).Err()
}

// CountLiveViewers returns the number of viewers in a live stream.
func (pt *PresenceTracker) CountLiveViewers(ctx context.Context, streamID string) (int64, error) {
	return pt.client.ZCard(ctx, fmt.Sprintf(keyLivePresence, streamID)).Result()
}

// GetRecentLiveViewers returns the most recently-joined viewer IDs.
func (pt *PresenceTracker) GetRecentLiveViewers(ctx context.Context, streamID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	return pt.client.ZRevRange(ctx, fmt.Sprintf(keyLivePresence, streamID), 0, int64(limit-1)).Result()
}

// RefreshLivePresence updates the user's timestamp in the live stream set (keep-alive).
func (pt *PresenceTracker) RefreshLivePresence(ctx context.Context, streamID, userID string) error {
	key := fmt.Sprintf(keyLivePresence, streamID)
	pipe := pt.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(time.Now().Unix()), Member: userID})
	pipe.Expire(ctx, key, LivePresenceTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// ----------------------------------------------------------------------------
// Typing indicators
// ----------------------------------------------------------------------------

// StartTyping marks a user as currently typing in a context.
// contextType is one of the Context* constants; contextID is the thread/video/room ID.
// Typing indicators expire automatically after TypingTTL without a refresh call.
func (pt *PresenceTracker) StartTyping(ctx context.Context, contextType, contextID, userID string) error {
	key := fmt.Sprintf(keyTyping, contextType, contextID)
	pipe := pt.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{
		Score:  float64(time.Now().UnixMilli()),
		Member: userID,
	})
	pipe.Expire(ctx, key, TypingTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// StopTyping removes a user's typing indicator immediately (on message send or blur).
func (pt *PresenceTracker) StopTyping(ctx context.Context, contextType, contextID, userID string) error {
	return pt.client.ZRem(ctx, fmt.Sprintf(keyTyping, contextType, contextID), userID).Err()
}

// GetTypingUsers returns the list of users currently typing in a context.
// Stale indicators older than TypingTTL are pruned atomically in the same pipeline.
func (pt *PresenceTracker) GetTypingUsers(ctx context.Context, contextType, contextID string) ([]TypingEvent, error) {
	key := fmt.Sprintf(keyTyping, contextType, contextID)
	cutoffMS := float64(time.Now().Add(-TypingTTL).UnixMilli())

	pipe := pt.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%v", cutoffMS))
	activeCmd := pipe.ZRangeWithScores(ctx, key, 0, -1)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("get typing users pipeline: %w", err)
	}

	results, err := activeCmd.Result()
	if err != nil {
		return nil, err
	}

	events := make([]TypingEvent, 0, len(results))
	for _, z := range results {
		events = append(events, TypingEvent{
			UserID:    z.Member.(string),
			ContextID: contextID,
			StartedAt: time.UnixMilli(int64(z.Score)),
		})
	}
	return events, nil
}

// IsTyping reports whether a specific user is currently typing in a context.
func (pt *PresenceTracker) IsTyping(ctx context.Context, contextType, contextID, userID string) (bool, error) {
	key := fmt.Sprintf(keyTyping, contextType, contextID)
	score, err := pt.client.ZScore(ctx, key, userID).Result()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Check the indicator hasn't expired.
	age := time.Since(time.UnixMilli(int64(score)))
	return age <= TypingTTL, nil
}

// ----------------------------------------------------------------------------
// Friends online list (cached)
// ----------------------------------------------------------------------------

// CacheFriendPresence stores the list of a user's currently online friends.
// friendIDs must already be filtered to only include online users (Status == online).
func (pt *PresenceTracker) CacheFriendPresence(ctx context.Context, userID string, onlineFriendIDs []string) error {
	key := fmt.Sprintf(keyFriendsPresence, userID)
	now := float64(time.Now().Unix())

	members := make([]goredis.Z, len(onlineFriendIDs))
	for i, fid := range onlineFriendIDs {
		members[i] = goredis.Z{Score: now, Member: fid}
	}

	pipe := pt.client.TxPipeline()
	pipe.Del(ctx, key)
	if len(members) > 0 {
		pipe.ZAdd(ctx, key, members...)
	}
	pipe.Expire(ctx, key, FriendsPresenceTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// GetOnlineFriends returns cached online friend IDs. Returns nil if the cache
// has expired (caller should rebuild from the database).
func (pt *PresenceTracker) GetOnlineFriends(ctx context.Context, userID string) ([]string, error) {
	key := fmt.Sprintf(keyFriendsPresence, userID)
	members, err := pt.client.ZRange(ctx, key, 0, -1).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	return members, err
}

// InvalidateFriendPresenceCache clears the friends presence cache for a user.
// Call this when any friend's online status changes.
func (pt *PresenceTracker) InvalidateFriendPresenceCache(ctx context.Context, userID string) error {
	return pt.client.Del(ctx, fmt.Sprintf(keyFriendsPresence, userID)).Err()
}

// InvalidateFriendPresenceCaches clears the friends presence cache for multiple users.
// Used when a user goes online/offline (all mutual friends' caches must be invalidated).
func (pt *PresenceTracker) InvalidateFriendPresenceCaches(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	keys := make([]string, len(userIDs))
	for i, uid := range userIDs {
		keys[i] = fmt.Sprintf(keyFriendsPresence, uid)
	}
	return pt.client.Del(ctx, keys...).Err()
}

// ----------------------------------------------------------------------------
// Last-seen timestamps
// ----------------------------------------------------------------------------

// GetLastSeen returns the last known activity timestamp for a user.
// Returns zero time if no presence record exists.
func (pt *PresenceTracker) GetLastSeen(ctx context.Context, userID string) (time.Time, error) {
	key := fmt.Sprintf(keyPresence, userID)
	raw, err := pt.client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	var info PresenceInfo
	if jsonErr := json.Unmarshal(raw, &info); jsonErr != nil {
		return time.Time{}, jsonErr
	}
	return info.LastSeen, nil
}

// GetBulkLastSeen returns last-seen timestamps for multiple users in one call.
func (pt *PresenceTracker) GetBulkLastSeen(ctx context.Context, userIDs []string) (map[string]time.Time, error) {
	infos, err := pt.GetBulkPresence(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(userIDs))
	for uid, info := range infos {
		out[uid] = info.LastSeen
	}
	return out, nil
}
