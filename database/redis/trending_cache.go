package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Trending key patterns (all sorted sets, score = weighted engagement):
//
//	trending:videos:{window}         -> videoID members    (ZSET)
//	trending:hashtags:{window}       -> hashtag members    (ZSET)
//	trending:sounds:{window}         -> soundID members    (ZSET)
//	trending:creators:{window}       -> userID members     (ZSET)
//	trending:meta:video:{videoID}    -> JSON TrendingVideo (STRING)
//	trending:meta:hashtag:{tag}      -> JSON TrendingHashtag (STRING)
//	trending:meta:sound:{soundID}    -> JSON TrendingSound (STRING)
//	trending:update_lock             -> mutex for score refresh job (STRING)
//	trending:checkpoint:{window}     -> unix ts of last full refresh (STRING)
//	trending:velocity:{videoID}      -> sorted set of hourly view increments (ZSET, short TTL)
const (
	// Time window identifiers used as key suffixes.
	WindowHourly = "1h"
	WindowDaily  = "24h"
	WindowWeekly = "7d"

	// Cache lifetimes for each window.
	TrendingTTLHourly = 2 * time.Hour
	TrendingTTLDaily  = 26 * time.Hour
	TrendingTTLWeekly = 8 * 24 * time.Hour

	// Top-N limits: sorted sets are trimmed to these sizes on every update.
	TrendingVideoTopN   = 100
	TrendingHashtagTopN = 200
	TrendingSoundTopN   = 100
	TrendingCreatorTopN = 200

	// VelocityWindowDuration is the look-back for momentum calculation.
	VelocityWindowDuration = 6 * time.Hour
	// VelocityTTL is the key lifetime for per-video velocity sorted sets.
	VelocityTTL = 12 * time.Hour

	keyTrendingVideos      = "trending:videos:%s"
	keyTrendingHashtags    = "trending:hashtags:%s"
	keyTrendingSounds      = "trending:sounds:%s"
	keyTrendingCreators    = "trending:creators:%s"
	keyTrendingMetaVideo   = "trending:meta:video:%s"
	keyTrendingMetaHashtag = "trending:meta:hashtag:%s"
	keyTrendingMetaSound   = "trending:meta:sound:%s"
	keyTrendingLock        = "trending:update_lock"
	keyTrendingCheckpoint  = "trending:checkpoint:%s"
	keyTrendingVelocity    = "trending:velocity:%s"
)

// TrendingVideo captures all data surfaced on the trending page for a video.
type TrendingVideo struct {
	VideoID      string    `json:"video_id"`
	AuthorID     string    `json:"author_id"`
	AuthorHandle string    `json:"author_handle"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Description  string    `json:"description"`
	ViewCount    int64     `json:"view_count"`
	LikeCount    int64     `json:"like_count"`
	ShareCount   int64     `json:"share_count"`
	CommentCount int64     `json:"comment_count"`
	// TrendScore is the computed ranking signal; higher = more trending.
	TrendScore float64   `json:"trend_score"`
	// VelocityScore is the rate of engagement gain; high velocity = "going viral".
	VelocityScore float64   `json:"velocity_score"`
	CreatedAt     time.Time `json:"created_at"`
	// Rank is populated on read, not stored.
	Rank int `json:"rank,omitempty"`
}

// TrendingHashtag holds aggregate data for a hashtag challenge.
type TrendingHashtag struct {
	Tag        string  `json:"tag"`
	VideoCount int64   `json:"video_count"`
	ViewCount  int64   `json:"view_count"`
	// ChallengeID is non-empty when this hashtag is part of a creator challenge.
	ChallengeID string  `json:"challenge_id,omitempty"`
	TrendScore  float64 `json:"trend_score"`
	// Rank is populated on read.
	Rank int `json:"rank,omitempty"`
}

// TrendingSound holds aggregate data for a viral audio track.
type TrendingSound struct {
	SoundID    string  `json:"sound_id"`
	Title      string  `json:"title"`
	Artist     string  `json:"artist"`
	CoverURL   string  `json:"cover_url"`
	VideoCount int64   `json:"video_count"`
	// ViewCount is the total views across all videos using this sound.
	ViewCount  int64   `json:"view_count"`
	TrendScore float64 `json:"trend_score"`
	Rank       int     `json:"rank,omitempty"`
}

// TrendingCache manages trending sorted sets for multiple time windows.
type TrendingCache struct {
	client *goredis.Client
	// luaBulkUpsert updates scores across all windows and trims to top-N atomically.
	luaBulkUpsert *goredis.Script
}

// NewTrendingCache constructs a TrendingCache and pre-compiles Lua scripts.
func NewTrendingCache(client *goredis.Client) *TrendingCache {
	return &TrendingCache{
		client: client,
		luaBulkUpsert: goredis.NewScript(`
			-- KEYS[1..3]  = sorted set keys for 1h, 24h, 7d windows
			-- ARGV[1]     = top-N limit
			-- ARGV[2..4]  = TTL seconds for each window (0 = persist)
			-- ARGV[5]     = score (float string)
			-- ARGV[6]     = member (videoID / tag / soundID)
			local top_n = tonumber(ARGV[1])
			local ttls  = {tonumber(ARGV[2]), tonumber(ARGV[3]), tonumber(ARGV[4])}
			local score  = ARGV[5]
			local member = ARGV[6]
			for i = 1, #KEYS do
				redis.call('ZADD', KEYS[i], score, member)
				redis.call('ZREMRANGEBYRANK', KEYS[i], 0, -(top_n + 1))
				if ttls[i] and ttls[i] > 0 then
					redis.call('EXPIRE', KEYS[i], ttls[i])
				end
			end
			return 1
		`),
	}
}

// ----------------------------------------------------------------------------
// Scoring
// ----------------------------------------------------------------------------

// trendScore computes a time-decay ranking score modelled after the Hacker News
// algorithm but weighted for video engagement.
//
//	score = engagement / (ageHours + 2)^gravity
//	engagement = views + 2*likes + 3*shares + 1.5*comments
//
// gravity=1.8 makes scores decay faster than HN (gravity=1.8) so viral content
// rotates out of trending within ~24 hours for normal videos.
func trendScore(views, likes, shares, comments int64, createdAt time.Time) float64 {
	const gravity = 1.8
	engagement := float64(views) + 2*float64(likes) + 3*float64(shares) + 1.5*float64(comments)
	ageHours := time.Since(createdAt).Hours()
	if ageHours < 0 {
		ageHours = 0
	}
	return engagement / math.Pow(ageHours+2, gravity)
}

// windowTTL maps a window string to its Redis key expiry.
func windowTTL(window string) time.Duration {
	switch window {
	case WindowHourly:
		return TrendingTTLHourly
	case WindowDaily:
		return TrendingTTLDaily
	case WindowWeekly:
		return TrendingTTLWeekly
	default:
		return TrendingTTLDaily
	}
}

func allWindows() []string {
	return []string{WindowHourly, WindowDaily, WindowWeekly}
}

// ----------------------------------------------------------------------------
// Videos
// ----------------------------------------------------------------------------

// RecordVideoEngagement updates a video's trend score across all time windows
// and persists its metadata. Call this on every significant engagement event
// (view, like, share, comment). The call is idempotent — safe to call repeatedly
// with cumulative totals.
func (tc *TrendingCache) RecordVideoEngagement(ctx context.Context, v TrendingVideo) error {
	score := trendScore(v.ViewCount, v.LikeCount, v.ShareCount, v.CommentCount, v.CreatedAt)
	v.TrendScore = score

	metaKey := fmt.Sprintf(keyTrendingMetaVideo, v.VideoID)
	raw, _ := json.Marshal(v)

	keys := []string{
		fmt.Sprintf(keyTrendingVideos, WindowHourly),
		fmt.Sprintf(keyTrendingVideos, WindowDaily),
		fmt.Sprintf(keyTrendingVideos, WindowWeekly),
	}

	scoreStr := fmt.Sprintf("%v", score)
	if err := tc.luaBulkUpsert.Run(ctx, tc.client, keys,
		TrendingVideoTopN,
		int(TrendingTTLHourly.Seconds()),
		int(TrendingTTLDaily.Seconds()),
		int(TrendingTTLWeekly.Seconds()),
		scoreStr,
		v.VideoID,
	).Err(); err != nil {
		return fmt.Errorf("trending bulk upsert video: %w", err)
	}

	return tc.client.Set(ctx, metaKey, raw, TrendingTTLWeekly).Err()
}

// GetTrendingVideos returns the top N trending videos for a given window.
// limit is capped at TrendingVideoTopN.
func (tc *TrendingCache) GetTrendingVideos(ctx context.Context, window string, limit int) ([]TrendingVideo, error) {
	if limit <= 0 || limit > TrendingVideoTopN {
		limit = TrendingVideoTopN
	}
	key := fmt.Sprintf(keyTrendingVideos, window)
	results, err := tc.client.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrange trending videos [%s]: %w", window, err)
	}
	videos, err := tc.hydrateVideos(ctx, results)
	if err != nil {
		return nil, err
	}
	for i := range videos {
		videos[i].Rank = i + 1
	}
	return videos, nil
}

// GetVideoRank returns the rank (1-based) and score of a video within a window.
// Returns rank=-1 if the video is not in the trending set.
func (tc *TrendingCache) GetVideoRank(ctx context.Context, videoID, window string) (rank int64, score float64, err error) {
	key := fmt.Sprintf(keyTrendingVideos, window)
	rank, err = tc.client.ZRevRank(ctx, key, videoID).Result()
	if err != nil {
		return -1, 0, err
	}
	score, err = tc.client.ZScore(ctx, key, videoID).Result()
	return rank + 1, score, err
}

// RemoveVideo removes a video from all trending sorted sets.
// Used on moderation takedown, deletion, or manual de-trending.
func (tc *TrendingCache) RemoveVideo(ctx context.Context, videoID string) error {
	pipe := tc.client.Pipeline()
	for _, w := range allWindows() {
		pipe.ZRem(ctx, fmt.Sprintf(keyTrendingVideos, w), videoID)
	}
	pipe.Del(ctx, fmt.Sprintf(keyTrendingMetaVideo, videoID))
	pipe.Del(ctx, fmt.Sprintf(keyTrendingVelocity, videoID))
	_, err := pipe.Exec(ctx)
	return err
}

// ----------------------------------------------------------------------------
// Velocity / viral momentum tracking
// ----------------------------------------------------------------------------

// RecordViewBurst records a view count increment at the current timestamp for
// velocity calculation. The sorted set stores hourly increments scored by unix
// timestamp so old buckets age out automatically.
func (tc *TrendingCache) RecordViewBurst(ctx context.Context, videoID string, viewDelta int64) error {
	key := fmt.Sprintf(keyTrendingVelocity, videoID)
	now := time.Now().UnixMilli()
	member := fmt.Sprintf("%d:%d", now, viewDelta) // unique member per burst

	pipe := tc.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(now), Member: member})
	// Keep only the last VelocityWindowDuration of data.
	cutoff := float64(time.Now().Add(-VelocityWindowDuration).UnixMilli())
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%v", cutoff))
	pipe.Expire(ctx, key, VelocityTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// GetVelocityScore computes the total views recorded within VelocityWindowDuration.
// A high velocity score indicates rapid recent growth.
func (tc *TrendingCache) GetVelocityScore(ctx context.Context, videoID string) (float64, error) {
	key := fmt.Sprintf(keyTrendingVelocity, videoID)
	cutoff := float64(time.Now().Add(-VelocityWindowDuration).UnixMilli())

	results, err := tc.client.ZRangeByScore(ctx, key, &goredis.ZRangeBy{
		Min: fmt.Sprintf("%v", cutoff),
		Max: "+inf",
	}).Result()
	if err != nil {
		return 0, err
	}

	var total float64
	for _, member := range results {
		var ts, delta int64
		fmt.Sscanf(member, "%d:%d", &ts, &delta)
		total += float64(delta)
	}
	return total, nil
}

// ----------------------------------------------------------------------------
// Hashtags
// ----------------------------------------------------------------------------

// IncrHashtag atomically updates a hashtag's view and video counts, then
// recomputes its trend score and upserts it into all window sorted sets.
// Uses WATCH/multi-exec optimistic locking for the read-modify-write on metadata.
func (tc *TrendingCache) IncrHashtag(ctx context.Context, tag string, viewDelta, videoDelta int64) error {
	metaKey := fmt.Sprintf(keyTrendingMetaHashtag, tag)

	return tc.client.Watch(ctx, func(tx *goredis.Tx) error {
		raw, err := tx.Get(ctx, metaKey).Bytes()
		var ht TrendingHashtag
		if err == nil {
			_ = json.Unmarshal(raw, &ht)
		}
		ht.Tag = tag
		ht.ViewCount += viewDelta
		ht.VideoCount += videoDelta
		// Hashtag score: views + 5× video_count (new videos carry more signal).
		ht.TrendScore = float64(ht.ViewCount) + 5*float64(ht.VideoCount)

		updated, _ := json.Marshal(ht)
		scoreStr := fmt.Sprintf("%v", ht.TrendScore)

		_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.Set(ctx, metaKey, updated, TrendingTTLWeekly)
			for _, w := range allWindows() {
				key := fmt.Sprintf(keyTrendingHashtags, w)
				pipe.ZAdd(ctx, key, goredis.Z{Score: ht.TrendScore, Member: tag})
				pipe.ZRemRangeByRank(ctx, key, 0, int64(-TrendingHashtagTopN-1))
				pipe.Expire(ctx, key, windowTTL(w))
			}
			_ = scoreStr // used above inline
			return nil
		})
		return err
	}, metaKey)
}

// GetTrendingHashtags returns the top N hashtags for a time window.
func (tc *TrendingCache) GetTrendingHashtags(ctx context.Context, window string, limit int) ([]TrendingHashtag, error) {
	if limit <= 0 || limit > TrendingHashtagTopN {
		limit = TrendingHashtagTopN
	}
	key := fmt.Sprintf(keyTrendingHashtags, window)
	results, err := tc.client.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrange trending hashtags [%s]: %w", window, err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	metaKeys := make([]string, len(results))
	for i, z := range results {
		metaKeys[i] = fmt.Sprintf(keyTrendingMetaHashtag, z.Member.(string))
	}
	rawValues, err := tc.client.MGet(ctx, metaKeys...).Result()
	if err != nil {
		return nil, err
	}

	tags := make([]TrendingHashtag, 0, len(results))
	for i, v := range rawValues {
		var ht TrendingHashtag
		if v != nil {
			_ = json.Unmarshal([]byte(v.(string)), &ht)
		} else {
			ht = TrendingHashtag{
				Tag:        results[i].Member.(string),
				TrendScore: results[i].Score,
			}
		}
		ht.Rank = i + 1
		tags = append(tags, ht)
	}
	return tags, nil
}

// GetHashtagRank returns the rank and trend score of a hashtag within a window.
func (tc *TrendingCache) GetHashtagRank(ctx context.Context, tag, window string) (int64, float64, error) {
	key := fmt.Sprintf(keyTrendingHashtags, window)
	rank, err := tc.client.ZRevRank(ctx, key, tag).Result()
	if err != nil {
		return -1, 0, err
	}
	score, err := tc.client.ZScore(ctx, key, tag).Result()
	return rank + 1, score, err
}

// RemoveHashtag removes a hashtag from all window sorted sets (e.g. on ban).
func (tc *TrendingCache) RemoveHashtag(ctx context.Context, tag string) error {
	pipe := tc.client.Pipeline()
	for _, w := range allWindows() {
		pipe.ZRem(ctx, fmt.Sprintf(keyTrendingHashtags, w), tag)
	}
	pipe.Del(ctx, fmt.Sprintf(keyTrendingMetaHashtag, tag))
	_, err := pipe.Exec(ctx)
	return err
}

// ----------------------------------------------------------------------------
// Sounds
// ----------------------------------------------------------------------------

// RecordSoundUsage updates a sound's trend score when a new video uses it or
// when views accumulate. Uses WATCH for safe read-modify-write.
func (tc *TrendingCache) RecordSoundUsage(ctx context.Context, sound TrendingSound) error {
	metaKey := fmt.Sprintf(keyTrendingMetaSound, sound.SoundID)

	return tc.client.Watch(ctx, func(tx *goredis.Tx) error {
		raw, err := tx.Get(ctx, metaKey).Bytes()
		var s TrendingSound
		if err == nil {
			_ = json.Unmarshal(raw, &s)
		} else {
			s = sound
			s.VideoCount = 0
			s.ViewCount = 0
		}
		s.VideoCount += sound.VideoCount
		s.ViewCount += sound.ViewCount
		// Sound score: video_count carries more weight than raw views
		// because a sound being used in many videos signals cultural relevance.
		s.TrendScore = float64(s.VideoCount)*10 + float64(s.ViewCount)*0.001

		updated, _ := json.Marshal(s)

		_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.Set(ctx, metaKey, updated, TrendingTTLWeekly)
			for _, w := range allWindows() {
				key := fmt.Sprintf(keyTrendingSounds, w)
				pipe.ZAdd(ctx, key, goredis.Z{Score: s.TrendScore, Member: s.SoundID})
				pipe.ZRemRangeByRank(ctx, key, 0, int64(-TrendingSoundTopN-1))
				pipe.Expire(ctx, key, windowTTL(w))
			}
			return nil
		})
		return err
	}, metaKey)
}

// GetTrendingSounds returns the top N trending sounds for a time window.
func (tc *TrendingCache) GetTrendingSounds(ctx context.Context, window string, limit int) ([]TrendingSound, error) {
	if limit <= 0 || limit > TrendingSoundTopN {
		limit = TrendingSoundTopN
	}
	key := fmt.Sprintf(keyTrendingSounds, window)
	results, err := tc.client.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrange trending sounds [%s]: %w", window, err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	metaKeys := make([]string, len(results))
	for i, z := range results {
		metaKeys[i] = fmt.Sprintf(keyTrendingMetaSound, z.Member.(string))
	}
	rawValues, err := tc.client.MGet(ctx, metaKeys...).Result()
	if err != nil {
		return nil, err
	}

	sounds := make([]TrendingSound, 0, len(results))
	for i, v := range rawValues {
		var s TrendingSound
		if v != nil {
			_ = json.Unmarshal([]byte(v.(string)), &s)
		} else {
			s = TrendingSound{SoundID: results[i].Member.(string), TrendScore: results[i].Score}
		}
		s.Rank = i + 1
		sounds = append(sounds, s)
	}
	return sounds, nil
}

// ----------------------------------------------------------------------------
// Trending creators
// ----------------------------------------------------------------------------

// UpdateCreatorTrendScore sets or increments a creator's trend score.
// score should incorporate follower gains, engagement rate, and video performance.
func (tc *TrendingCache) UpdateCreatorTrendScore(ctx context.Context, userID string, score float64, window string) error {
	key := fmt.Sprintf(keyTrendingCreators, window)
	pipe := tc.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: score, Member: userID})
	pipe.ZRemRangeByRank(ctx, key, 0, int64(-TrendingCreatorTopN-1))
	pipe.Expire(ctx, key, windowTTL(window))
	_, err := pipe.Exec(ctx)
	return err
}

// IncrCreatorTrendScore atomically increments a creator's score.
// Use this for incremental event-driven updates (e.g. per new follower).
func (tc *TrendingCache) IncrCreatorTrendScore(ctx context.Context, userID string, delta float64, window string) error {
	key := fmt.Sprintf(keyTrendingCreators, window)
	return tc.client.ZIncrBy(ctx, key, delta, userID).Err()
}

// GetTrendingCreators returns ranked creator IDs with their scores.
func (tc *TrendingCache) GetTrendingCreators(ctx context.Context, window string, limit int) ([]goredis.Z, error) {
	if limit <= 0 || limit > TrendingCreatorTopN {
		limit = TrendingCreatorTopN
	}
	key := fmt.Sprintf(keyTrendingCreators, window)
	return tc.client.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
}

// ----------------------------------------------------------------------------
// Bulk refresh (called by a background job every hour)
// ----------------------------------------------------------------------------

// BulkUpdateVideoScores recalculates scores for a list of videos and applies
// them atomically via pipelined ZADD + ZREMRANGEBYRANK across all windows.
// This is the primary mechanism for the hourly trending refresh job.
func (tc *TrendingCache) BulkUpdateVideoScores(ctx context.Context, videos []TrendingVideo) error {
	if len(videos) == 0 {
		return nil
	}

	type windowUpdate struct {
		key     string
		members []goredis.Z
		ttl     time.Duration
		topN    int64
	}
	byWindow := map[string]*windowUpdate{
		WindowHourly: {key: fmt.Sprintf(keyTrendingVideos, WindowHourly), ttl: TrendingTTLHourly, topN: TrendingVideoTopN},
		WindowDaily:  {key: fmt.Sprintf(keyTrendingVideos, WindowDaily), ttl: TrendingTTLDaily, topN: TrendingVideoTopN},
		WindowWeekly: {key: fmt.Sprintf(keyTrendingVideos, WindowWeekly), ttl: TrendingTTLWeekly, topN: TrendingVideoTopN},
	}

	pipe := tc.client.Pipeline()
	for i := range videos {
		score := trendScore(
			videos[i].ViewCount, videos[i].LikeCount,
			videos[i].ShareCount, videos[i].CommentCount,
			videos[i].CreatedAt,
		)
		videos[i].TrendScore = score
		raw, _ := json.Marshal(videos[i])
		pipe.Set(ctx, fmt.Sprintf(keyTrendingMetaVideo, videos[i].VideoID), raw, TrendingTTLWeekly)

		for _, w := range allWindows() {
			byWindow[w].members = append(byWindow[w].members, goredis.Z{
				Score:  score,
				Member: videos[i].VideoID,
			})
		}
	}

	for _, wu := range byWindow {
		if len(wu.members) == 0 {
			continue
		}
		pipe.ZAdd(ctx, wu.key, wu.members...)
		pipe.ZRemRangeByRank(ctx, wu.key, 0, -(wu.topN+1))
		pipe.Expire(ctx, wu.key, wu.ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// SetRefreshCheckpoint records the unix timestamp of the last successful
// trending refresh for a window. Used by monitoring to detect stale data.
func (tc *TrendingCache) SetRefreshCheckpoint(ctx context.Context, window string) error {
	key := fmt.Sprintf(keyTrendingCheckpoint, window)
	return tc.client.Set(ctx, key, time.Now().Unix(), windowTTL(window)*2).Err()
}

// GetRefreshCheckpoint returns the last successful refresh timestamp.
// Returns zero time if no checkpoint has been set.
func (tc *TrendingCache) GetRefreshCheckpoint(ctx context.Context, window string) (time.Time, error) {
	key := fmt.Sprintf(keyTrendingCheckpoint, window)
	ts, err := tc.client.Get(ctx, key).Int64()
	if err != nil {
		return time.Time{}, nil //nolint:nilerr
	}
	return time.Unix(ts, 0).UTC(), nil
}

// ----------------------------------------------------------------------------
// Distributed update lock
// ----------------------------------------------------------------------------

// AcquireUpdateLock acquires the trending-update mutex for the given duration.
// Returns (true, nil) if the lock was acquired, (false, nil) if already held.
func (tc *TrendingCache) AcquireUpdateLock(ctx context.Context, value string, ttl time.Duration) (bool, error) {
	return tc.client.SetNX(ctx, keyTrendingLock, value, ttl).Result()
}

// ReleaseUpdateLock releases the update mutex only if still owned by value.
func (tc *TrendingCache) ReleaseUpdateLock(ctx context.Context, value string) error {
	script := goredis.NewScript(`
		if redis.call('GET', KEYS[1]) == ARGV[1] then
			return redis.call('DEL', KEYS[1])
		end
		return 0
	`)
	return script.Run(ctx, tc.client, []string{keyTrendingLock}, value).Err()
}

// ----------------------------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------------------------

// hydrateVideos fetches TrendingVideo metadata for a slice of scored members.
func (tc *TrendingCache) hydrateVideos(ctx context.Context, results []goredis.Z) ([]TrendingVideo, error) {
	if len(results) == 0 {
		return nil, nil
	}
	keys := make([]string, len(results))
	for i, z := range results {
		keys[i] = fmt.Sprintf(keyTrendingMetaVideo, z.Member.(string))
	}
	rawValues, err := tc.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget trending video meta: %w", err)
	}

	videos := make([]TrendingVideo, 0, len(rawValues))
	for i, v := range rawValues {
		var tv TrendingVideo
		if v != nil {
			if jsonErr := json.Unmarshal([]byte(v.(string)), &tv); jsonErr != nil {
				continue
			}
		} else {
			// Metadata missing; populate stub from sorted-set member.
			tv = TrendingVideo{
				VideoID:    results[i].Member.(string),
				TrendScore: results[i].Score,
			}
		}
		videos = append(videos, tv)
	}
	return videos, nil
}
