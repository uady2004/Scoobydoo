package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Leaderboard key patterns:
//
//	lb:creators:{period}          -> sorted set: userID -> composite score  (ZSET)
//	lb:gifters:{period}           -> sorted set: userID -> total diamonds   (ZSET)
//	lb:livestream:active          -> sorted set: streamID -> viewer count   (ZSET, short TTL)
//	lb:livestream:today           -> sorted set: streamID -> peak viewers   (ZSET, 26 h TTL)
//	lb:creators:meta:{userID}     -> JSON CreatorEntry                      (STRING)
//	lb:gifters:meta:{userID}      -> JSON GifterEntry                       (STRING)
//	lb:livestream:meta:{streamID} -> JSON LiveStreamEntry                   (STRING)
//	lb:reset_lock:{period}        -> distributed mutex for period reset     (STRING)
//	lb:gifts:history:{userID}     -> sorted set: giftEventID -> unix ts     (ZSET)
//	lb:creators:daily_gain        -> sorted set: userID -> follower gain today (ZSET)
const (
	// Period identifiers used as key suffixes.
	PeriodDaily   = "daily"
	PeriodWeekly  = "weekly"
	PeriodMonthly = "monthly"
	PeriodAllTime = "alltime"

	// LeaderboardTopN is the maximum entries kept in each sorted set.
	LeaderboardTopN = 100

	// TTLs for each period leaderboard.
	LBTTLDaily   = 26 * time.Hour
	LBTTLWeekly  = 8 * 24 * time.Hour
	LBTTLMonthly = 32 * 24 * time.Hour
	// LBTTLAllTime = 0 means no expiry — key must be persisted.

	// LBMetaTTL is the TTL for metadata hash/string keys.
	LBMetaTTL = 24 * time.Hour

	// LiveStreamActiveRefreshInterval is how often active streams must
	// refresh their viewer count or be considered stale.
	LiveStreamActiveRefreshInterval = 5 * time.Minute

	keyLBCreators      = "lb:creators:%s"
	keyLBGifters       = "lb:gifters:%s"
	keyLBLiveActive    = "lb:livestream:active"
	keyLBLiveToday     = "lb:livestream:today"
	keyLBCreatorMeta   = "lb:creators:meta:%s"
	keyLBGifterMeta    = "lb:gifters:meta:%s"
	keyLBLiveMeta      = "lb:livestream:meta:%s"
	keyLBResetLock     = "lb:reset_lock:%s"
	keyLBGiftHistory   = "lb:gifts:history:%s"
	keyLBCreatorGain   = "lb:creators:daily_gain"
)

// CreatorEntry holds all fields surfaced in the creator leaderboard UI.
type CreatorEntry struct {
	UserID        string    `json:"user_id"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"display_name"`
	AvatarURL     string    `json:"avatar_url"`
	FollowerCount int64     `json:"follower_count"`
	LikeCount     int64     `json:"like_count"`
	VideoCount    int64     `json:"video_count"`
	// Score is the composite leaderboard ranking signal:
	//   score = followers*0.4 + likes*0.3 + views*0.2 + shares*0.1
	Score     float64   `json:"score"`
	Rank      int64     `json:"rank"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GifterEntry holds all fields surfaced in the gifter leaderboard UI.
type GifterEntry struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	// TotalGifts is the cumulative diamond value sent in the leaderboard period.
	TotalGifts int64     `json:"total_gifts"`
	// GiftCount is the total number of individual gift sends.
	GiftCount  int64     `json:"gift_count"`
	Score      float64   `json:"score"`
	Rank       int64     `json:"rank"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// LiveStreamEntry holds real-time and peak viewer data for a live stream.
type LiveStreamEntry struct {
	StreamID     string    `json:"stream_id"`
	HostUserID   string    `json:"host_user_id"`
	HostHandle   string    `json:"host_handle"`
	Title        string    `json:"title"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Category     string    `json:"category,omitempty"`
	ViewerCount  int64     `json:"viewer_count"`
	PeakViewers  int64     `json:"peak_viewers"`
	GiftCount    int64     `json:"gift_count"`
	DiamondTotal int64     `json:"diamond_total"`
	StartedAt    time.Time `json:"started_at"`
	IsLive       bool      `json:"is_live"`
	// Rank is populated on read.
	Rank int `json:"rank,omitempty"`
}

// GiftEvent records a single gifting transaction.
type GiftEvent struct {
	EventID   string    `json:"event_id"`
	SenderID  string    `json:"sender_id"`
	RecipientID string  `json:"recipient_id"`
	GiftType  string    `json:"gift_type"`
	Diamonds  int64     `json:"diamonds"`
	StreamID  string    `json:"stream_id,omitempty"`
	SentAt    time.Time `json:"sent_at"`
}

// LeaderboardCache manages all leaderboard sorted sets for creators, gifters,
// and live streams.
type LeaderboardCache struct {
	client *goredis.Client
	// luaRecordGift atomically increments gifter scores across all periods
	// and trims each sorted set to LeaderboardTopN in a single execution.
	luaRecordGift *goredis.Script
}

// NewLeaderboardCache constructs a LeaderboardCache and pre-compiles Lua scripts.
func NewLeaderboardCache(client *goredis.Client) *LeaderboardCache {
	return &LeaderboardCache{
		client: client,
		luaRecordGift: goredis.NewScript(`
			-- KEYS[1..4] = lb:gifters:{daily,weekly,monthly,alltime}
			-- ARGV[1]    = userID (sorted set member)
			-- ARGV[2]    = diamond delta (integer string)
			-- ARGV[3]    = top-N limit
			-- ARGV[4..7] = TTL seconds per period (0 = no expiry)
			local user_id  = ARGV[1]
			local diamonds = tonumber(ARGV[2])
			local top_n    = tonumber(ARGV[3])
			local scores   = {}
			for i = 1, #KEYS do
				local new_score = redis.call('ZINCRBY', KEYS[i], diamonds, user_id)
				redis.call('ZREMRANGEBYRANK', KEYS[i], 0, -(top_n + 1))
				local ttl = tonumber(ARGV[3 + i])
				if ttl and ttl > 0 then
					redis.call('EXPIRE', KEYS[i], ttl)
				end
				scores[i] = new_score
			end
			return scores
		`),
	}
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// lbTTL returns the Redis expiry for a given period.
// Returns 0 for PeriodAllTime (persist indefinitely).
func lbTTL(period string) time.Duration {
	switch period {
	case PeriodDaily:
		return LBTTLDaily
	case PeriodWeekly:
		return LBTTLWeekly
	case PeriodMonthly:
		return LBTTLMonthly
	default:
		return 0 // alltime — no expiry
	}
}

func allPeriods() []string {
	return []string{PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodAllTime}
}

// ----------------------------------------------------------------------------
// Top Creators
// ----------------------------------------------------------------------------

// UpsertCreator sets a creator's score across all period leaderboards and
// stores their display metadata. Call this after significant profile events
// (new follower, video like, etc.) or on a scheduled refresh.
//
// score should be pre-computed by the caller as:
//
//	score = followers*0.4 + likes*0.3 + views*0.2 + shares*0.1
func (lc *LeaderboardCache) UpsertCreator(ctx context.Context, entry CreatorEntry) error {
	entry.UpdatedAt = time.Now().UTC()
	metaKey := fmt.Sprintf(keyLBCreatorMeta, entry.UserID)
	raw, _ := json.Marshal(entry)

	pipe := lc.client.Pipeline()
	pipe.Set(ctx, metaKey, raw, LBMetaTTL)
	for _, period := range allPeriods() {
		key := fmt.Sprintf(keyLBCreators, period)
		pipe.ZAdd(ctx, key, goredis.Z{Score: entry.Score, Member: entry.UserID})
		pipe.ZRemRangeByRank(ctx, key, 0, int64(-LeaderboardTopN-1))
		if ttl := lbTTL(period); ttl > 0 {
			pipe.Expire(ctx, key, ttl)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

// IncrCreatorScore atomically adds delta to a creator's score in all period
// leaderboards. Use for event-driven increments (e.g. per new follower gained).
func (lc *LeaderboardCache) IncrCreatorScore(ctx context.Context, userID string, delta float64) error {
	pipe := lc.client.Pipeline()
	for _, period := range allPeriods() {
		pipe.ZIncrBy(ctx, fmt.Sprintf(keyLBCreators, period), delta, userID)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// RecordFollowerGain records a per-creator daily follower count increase for
// the "fastest growing" sub-leaderboard. Score = total new followers today.
func (lc *LeaderboardCache) RecordFollowerGain(ctx context.Context, userID string, delta int64) error {
	pipe := lc.client.Pipeline()
	pipe.ZIncrBy(ctx, keyLBCreatorGain, float64(delta), userID)
	// Trim to top 200 fastest growing; key expires at end of the day.
	pipe.ZRemRangeByRank(ctx, keyLBCreatorGain, 0, -201)
	pipe.Expire(ctx, keyLBCreatorGain, LBTTLDaily)
	_, err := pipe.Exec(ctx)
	return err
}

// GetFastestGrowingCreators returns creator IDs sorted by today's follower gain.
func (lc *LeaderboardCache) GetFastestGrowingCreators(ctx context.Context, limit int) ([]goredis.Z, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return lc.client.ZRevRangeWithScores(ctx, keyLBCreatorGain, 0, int64(limit-1)).Result()
}

// GetTopCreators returns the top N creators for a period, fully hydrated with
// display metadata. Rank is populated on the returned entries (1-based).
func (lc *LeaderboardCache) GetTopCreators(ctx context.Context, period string, limit int) ([]CreatorEntry, error) {
	if limit <= 0 || limit > LeaderboardTopN {
		limit = LeaderboardTopN
	}
	key := fmt.Sprintf(keyLBCreators, period)
	results, err := lc.client.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrange creators [%s]: %w", period, err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	metaKeys := make([]string, len(results))
	for i, z := range results {
		metaKeys[i] = fmt.Sprintf(keyLBCreatorMeta, z.Member.(string))
	}
	rawValues, err := lc.client.MGet(ctx, metaKeys...).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]CreatorEntry, 0, len(results))
	for i, v := range rawValues {
		var e CreatorEntry
		if v != nil {
			_ = json.Unmarshal([]byte(v.(string)), &e)
		} else {
			e.UserID = results[i].Member.(string)
		}
		e.Score = results[i].Score
		e.Rank = int64(i) + 1
		entries = append(entries, e)
	}
	return entries, nil
}

// GetCreatorRank returns a creator's 1-based rank and score within a period.
// Returns rank=-1 if the creator is not on the leaderboard.
func (lc *LeaderboardCache) GetCreatorRank(ctx context.Context, userID, period string) (rank int64, score float64, err error) {
	key := fmt.Sprintf(keyLBCreators, period)
	rank, err = lc.client.ZRevRank(ctx, key, userID).Result()
	if err != nil {
		return -1, 0, err
	}
	score, err = lc.client.ZScore(ctx, key, userID).Result()
	return rank + 1, score, err
}

// RemoveCreator removes a user from all creator leaderboards (e.g. on ban).
func (lc *LeaderboardCache) RemoveCreator(ctx context.Context, userID string) error {
	pipe := lc.client.Pipeline()
	for _, period := range allPeriods() {
		pipe.ZRem(ctx, fmt.Sprintf(keyLBCreators, period), userID)
	}
	pipe.Del(ctx, fmt.Sprintf(keyLBCreatorMeta, userID))
	pipe.ZRem(ctx, keyLBCreatorGain, userID)
	_, err := pipe.Exec(ctx)
	return err
}

// ----------------------------------------------------------------------------
// Top Gifters
// ----------------------------------------------------------------------------

// RecordGift atomically increments a gifter's diamond total across all period
// leaderboards using a Lua script for a single round-trip.
// It also appends the event to the gifter's recent history sorted set.
func (lc *LeaderboardCache) RecordGift(ctx context.Context, event GiftEvent) error {
	keys := []string{
		fmt.Sprintf(keyLBGifters, PeriodDaily),
		fmt.Sprintf(keyLBGifters, PeriodWeekly),
		fmt.Sprintf(keyLBGifters, PeriodMonthly),
		fmt.Sprintf(keyLBGifters, PeriodAllTime),
	}

	if err := lc.luaRecordGift.Run(ctx, lc.client, keys,
		event.SenderID,
		event.Diamonds,
		LeaderboardTopN,
		int(LBTTLDaily.Seconds()),
		int(LBTTLWeekly.Seconds()),
		int(LBTTLMonthly.Seconds()),
		0, // alltime — no expiry
	).Err(); err != nil {
		return fmt.Errorf("record gift lua: %w", err)
	}

	// Append to rolling history (last 90 days).
	histKey := fmt.Sprintf(keyLBGiftHistory, event.SenderID)
	pipe := lc.client.Pipeline()
	pipe.ZAdd(ctx, histKey, goredis.Z{
		Score:  float64(event.SentAt.Unix()),
		Member: event.EventID,
	})
	// Trim to 1000 most recent events.
	pipe.ZRemRangeByRank(ctx, histKey, 0, -1001)
	pipe.Expire(ctx, histKey, 90*24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// SetGifterMeta stores or refreshes display information for a gifter.
func (lc *LeaderboardCache) SetGifterMeta(ctx context.Context, entry GifterEntry) error {
	entry.UpdatedAt = time.Now().UTC()
	raw, _ := json.Marshal(entry)
	return lc.client.Set(ctx, fmt.Sprintf(keyLBGifterMeta, entry.UserID), raw, LBMetaTTL).Err()
}

// GetTopGifters returns the top N gifters for a period with full metadata.
func (lc *LeaderboardCache) GetTopGifters(ctx context.Context, period string, limit int) ([]GifterEntry, error) {
	if limit <= 0 || limit > LeaderboardTopN {
		limit = LeaderboardTopN
	}
	key := fmt.Sprintf(keyLBGifters, period)
	results, err := lc.client.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrange gifters [%s]: %w", period, err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	metaKeys := make([]string, len(results))
	for i, z := range results {
		metaKeys[i] = fmt.Sprintf(keyLBGifterMeta, z.Member.(string))
	}
	rawValues, err := lc.client.MGet(ctx, metaKeys...).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]GifterEntry, 0, len(results))
	for i, v := range rawValues {
		var e GifterEntry
		if v != nil {
			_ = json.Unmarshal([]byte(v.(string)), &e)
		} else {
			e.UserID = results[i].Member.(string)
		}
		e.Score = results[i].Score
		e.Rank = int64(i) + 1
		entries = append(entries, e)
	}
	return entries, nil
}

// GetGifterRank returns a gifter's 1-based rank and total diamond score within a period.
func (lc *LeaderboardCache) GetGifterRank(ctx context.Context, userID, period string) (int64, float64, error) {
	key := fmt.Sprintf(keyLBGifters, period)
	rank, err := lc.client.ZRevRank(ctx, key, userID).Result()
	if err != nil {
		return -1, 0, err
	}
	score, err := lc.client.ZScore(ctx, key, userID).Result()
	return rank + 1, score, err
}

// GetGifterDiamondsInPeriod returns a user's cumulative diamond total for a period.
func (lc *LeaderboardCache) GetGifterDiamondsInPeriod(ctx context.Context, userID, period string) (float64, error) {
	score, err := lc.client.ZScore(ctx, fmt.Sprintf(keyLBGifters, period), userID).Result()
	if err != nil {
		return 0, err
	}
	return score, nil
}

// GetGiftHistory returns the most recent N gift event IDs for a user.
// Call RecordGift with full event data and store events separately if full
// history details are needed.
func (lc *LeaderboardCache) GetGiftHistory(ctx context.Context, userID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	key := fmt.Sprintf(keyLBGiftHistory, userID)
	return lc.client.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
}

// ----------------------------------------------------------------------------
// Live Stream Rankings
// ----------------------------------------------------------------------------

// UpdateLiveStreamViewers updates the real-time viewer count for an active stream.
// This is called every few seconds by the stream server. The active set auto-
// expires if not refreshed within LiveStreamActiveRefreshInterval.
//
// Peak viewers (today's maximum) are tracked separately using the GT flag so
// only higher values overwrite the stored peak.
func (lc *LeaderboardCache) UpdateLiveStreamViewers(ctx context.Context, streamID string, viewerCount int64) error {
	pipe := lc.client.Pipeline()
	pipe.ZAdd(ctx, keyLBLiveActive, goredis.Z{
		Score:  float64(viewerCount),
		Member: streamID,
	})
	// GT: only update peak if new count is strictly greater.
	pipe.ZAddArgs(ctx, keyLBLiveToday, goredis.ZAddArgs{
		GT:      true,
		Members: []goredis.Z{{Score: float64(viewerCount), Member: streamID}},
	})
	// Active set: streams that stop calling this will auto-expire and drop off
	// the leaderboard within LiveStreamActiveRefreshInterval.
	pipe.Expire(ctx, keyLBLiveActive, LiveStreamActiveRefreshInterval)
	pipe.Expire(ctx, keyLBLiveToday, LBTTLDaily)
	_, err := pipe.Exec(ctx)
	return err
}

// IncrLiveStreamGifts increments the gift counter and diamond total for a stream.
// Used to rank streams by gifting activity, not just viewer count.
func (lc *LeaderboardCache) IncrLiveStreamGifts(ctx context.Context, streamID string, diamonds int64) error {
	metaKey := fmt.Sprintf(keyLBLiveMeta, streamID)
	raw, err := lc.client.Get(ctx, metaKey).Bytes()
	if err != nil {
		return nil // stream not cached, skip
	}
	var entry LiveStreamEntry
	if jsonErr := json.Unmarshal(raw, &entry); jsonErr != nil {
		return nil
	}
	entry.GiftCount++
	entry.DiamondTotal += diamonds
	updated, _ := json.Marshal(entry)
	return lc.client.Set(ctx, metaKey, updated, LBTTLDaily).Err()
}

// UpsertLiveStreamMeta stores or updates the metadata for a live stream.
// Call at stream start and whenever title/thumbnail changes.
func (lc *LeaderboardCache) UpsertLiveStreamMeta(ctx context.Context, entry LiveStreamEntry) error {
	raw, _ := json.Marshal(entry)
	return lc.client.Set(ctx, fmt.Sprintf(keyLBLiveMeta, entry.StreamID), raw, LBTTLDaily).Err()
}

// GetTopLiveStreams returns the top N currently-live streams by viewer count.
func (lc *LeaderboardCache) GetTopLiveStreams(ctx context.Context, limit int) ([]LiveStreamEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	results, err := lc.client.ZRevRangeWithScores(ctx, keyLBLiveActive, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrange live active: %w", err)
	}
	return lc.hydrateLiveStreams(ctx, results)
}

// GetTopLiveStreamsByPeak returns streams with the highest peak viewer counts today.
func (lc *LeaderboardCache) GetTopLiveStreamsByPeak(ctx context.Context, limit int) ([]LiveStreamEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	results, err := lc.client.ZRevRangeWithScores(ctx, keyLBLiveToday, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrange live peak today: %w", err)
	}
	return lc.hydrateLiveStreams(ctx, results)
}

// EndLiveStream removes a stream from the active leaderboard when the host ends
// the broadcast. The stream remains in the today-peak leaderboard.
func (lc *LeaderboardCache) EndLiveStream(ctx context.Context, streamID string) error {
	pipe := lc.client.Pipeline()
	pipe.ZRem(ctx, keyLBLiveActive, streamID)

	// Mark as not live in the metadata.
	metaKey := fmt.Sprintf(keyLBLiveMeta, streamID)
	raw, err := lc.client.Get(ctx, metaKey).Bytes()
	if err == nil {
		var entry LiveStreamEntry
		if jsonErr := json.Unmarshal(raw, &entry); jsonErr == nil {
			entry.IsLive = false
			updated, _ := json.Marshal(entry)
			pipe.Set(ctx, metaKey, updated, LBTTLDaily)
		}
	}
	_, execErr := pipe.Exec(ctx)
	return execErr
}

// GetLiveStreamViewerCount returns the current viewer count for a stream.
func (lc *LeaderboardCache) GetLiveStreamViewerCount(ctx context.Context, streamID string) (int64, error) {
	score, err := lc.client.ZScore(ctx, keyLBLiveActive, streamID).Result()
	if err != nil {
		return 0, err
	}
	return int64(score), nil
}

// GetLiveStreamRank returns the current 1-based rank of a stream by viewer count.
func (lc *LeaderboardCache) GetLiveStreamRank(ctx context.Context, streamID string) (int64, error) {
	rank, err := lc.client.ZRevRank(ctx, keyLBLiveActive, streamID).Result()
	if err != nil {
		return -1, err
	}
	return rank + 1, nil
}

// ActiveStreamCount returns the number of currently active live streams.
func (lc *LeaderboardCache) ActiveStreamCount(ctx context.Context) (int64, error) {
	return lc.client.ZCard(ctx, keyLBLiveActive).Result()
}

// ----------------------------------------------------------------------------
// Period reset
// ----------------------------------------------------------------------------

// ResetPeriodLeaderboard clears a leaderboard at the start of a new period.
// Acquires a distributed lock to prevent double-reset from multiple instances.
//
// boardType must be one of "creators" or "gifters".
func (lc *LeaderboardCache) ResetPeriodLeaderboard(ctx context.Context, boardType, period, lockValue string) error {
	lockKey := fmt.Sprintf(keyLBResetLock, period)
	acquired, err := lc.client.SetNX(ctx, lockKey, lockValue, 60*time.Second).Result()
	if err != nil {
		return fmt.Errorf("acquire reset lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("leaderboard reset already in progress for period %s", period)
	}
	defer lc.client.Del(ctx, lockKey) //nolint:errcheck

	var key string
	switch boardType {
	case "creators":
		key = fmt.Sprintf(keyLBCreators, period)
	case "gifters":
		key = fmt.Sprintf(keyLBGifters, period)
	default:
		return fmt.Errorf("unknown leaderboard type %q", boardType)
	}
	return lc.client.Del(ctx, key).Err()
}

// SnapshotLeaderboard stores a point-in-time snapshot of a leaderboard to a
// timestamped key before the period reset. Useful for historical records and
// notifications ("You ranked #3 last week").
func (lc *LeaderboardCache) SnapshotLeaderboard(ctx context.Context, boardType, period string) error {
	var srcKey string
	switch boardType {
	case "creators":
		srcKey = fmt.Sprintf(keyLBCreators, period)
	case "gifters":
		srcKey = fmt.Sprintf(keyLBGifters, period)
	default:
		return fmt.Errorf("unknown leaderboard type %q", boardType)
	}

	snapKey := fmt.Sprintf("lb:snapshot:%s:%s:%d", boardType, period, time.Now().Unix())
	// COPY preserves the sorted set; set destination TTL to 90 days.
	if err := lc.client.Copy(ctx, srcKey, snapKey, 0, false).Err(); err != nil {
		return fmt.Errorf("snapshot leaderboard: %w", err)
	}
	return lc.client.Expire(ctx, snapKey, 90*24*time.Hour).Err()
}

// ----------------------------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------------------------

// hydrateLiveStreams fetches LiveStreamEntry metadata for a slice of scored members.
func (lc *LeaderboardCache) hydrateLiveStreams(ctx context.Context, results []goredis.Z) ([]LiveStreamEntry, error) {
	if len(results) == 0 {
		return nil, nil
	}
	keys := make([]string, len(results))
	for i, z := range results {
		keys[i] = fmt.Sprintf(keyLBLiveMeta, z.Member.(string))
	}
	rawValues, err := lc.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget live stream meta: %w", err)
	}

	streams := make([]LiveStreamEntry, 0, len(rawValues))
	for i, v := range rawValues {
		var e LiveStreamEntry
		if v != nil {
			_ = json.Unmarshal([]byte(v.(string)), &e)
		} else {
			e.StreamID = results[i].Member.(string)
		}
		// Authoritative viewer count comes from the sorted-set score,
		// not the potentially-stale metadata field.
		e.ViewerCount = int64(results[i].Score)
		e.Rank = i + 1
		streams = append(streams, e)
	}
	return streams, nil
}
