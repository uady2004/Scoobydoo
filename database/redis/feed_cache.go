package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Feed key patterns:
//
//	foryou_feed:{userID}         -> sorted set videoID -> recommendation score (ZSET)
//	following_feed:{userID}      -> sorted set videoID -> creation unix ts     (ZSET)
//	feed_meta:{ownerID}:{videoID}-> serialised FeedItem JSON                   (STRING)
//	feed_cursor:{userID}         -> last scroll position (float64 score)       (STRING)
//	feed_lock:{userID}           -> distributed mutex while rebuilding         (STRING)
//	feed_new:{userID}            -> counter of unseen new videos               (STRING, TTL)
//	feed_seen:{userID}           -> set of videoIDs the user has seen          (SET, TTL)
const (
	// FeedWindowSize is the maximum number of videos kept in each sorted-set feed.
	// Old entries (lowest score) are trimmed when this limit is exceeded.
	FeedWindowSize = 200

	// FeedTTL is the cache lifetime for both the foryou and following feeds.
	// A rebuild is triggered on a miss.
	FeedTTL = 2 * time.Hour

	// FeedLockTTL is the maximum duration a feed-rebuild lock is held.
	FeedLockTTL = 30 * time.Second

	// FeedItemMetaTTL is the per-item metadata TTL.
	FeedItemMetaTTL = 24 * time.Hour

	// FeedPageSize is the default number of items returned per page.
	FeedPageSize = 20

	// FeedSeenSetTTL limits how long the "seen" dedup set lives.
	FeedSeenSetTTL = 48 * time.Hour

	// FeedFanOutChunkSize controls the pipeline batch size during fan-out writes.
	FeedFanOutChunkSize = 200

	keyForYouFeed    = "foryou_feed:%s"
	keyFollowingFeed = "following_feed:%s"
	keyFeedMeta      = "feed_meta:%s:%s"
	keyFeedCursor    = "feed_cursor:%s"
	keyFeedLock      = "feed_lock:%s"
	keyFeedNew       = "feed_new:%s"
	keyFeedSeen      = "feed_seen:%s"
)

// FeedItem is the metadata stored alongside every feed entry.
// It is serialised as JSON and stored at keyFeedMeta so feed pages can be
// hydrated without hitting the primary database on every scroll.
type FeedItem struct {
	VideoID      string    `json:"video_id"`
	AuthorID     string    `json:"author_id"`
	AuthorHandle string    `json:"author_handle"`
	ThumbnailURL string    `json:"thumbnail_url"`
	VideoURL     string    `json:"video_url,omitempty"`
	Description  string    `json:"description"`
	Duration     int       `json:"duration_seconds"`
	LikeCount    int64     `json:"like_count"`
	CommentCount int64     `json:"comment_count"`
	ShareCount   int64     `json:"share_count"`
	ViewCount    int64     `json:"view_count"`
	SoundID      string    `json:"sound_id"`
	Hashtags     []string  `json:"hashtags"`
	CreatedAt    time.Time `json:"created_at"`
	// Score is the recommendation signal used to rank For-You feed entries.
	// Higher = more relevant. Computed by the recommendation service.
	Score float64 `json:"score"`
	// IsSponsored marks paid promotional content so the UI can show a label.
	IsSponsored bool `json:"is_sponsored,omitempty"`
	// AllowDuet and AllowStitch control creator-permission flags cached here
	// to avoid extra DB lookups during feed rendering.
	AllowDuet   bool `json:"allow_duet"`
	AllowStitch bool `json:"allow_stitch"`
}

// FeedPage is the response type for paginated feed queries.
type FeedPage struct {
	Items      []FeedItem `json:"items"`
	NextCursor float64    `json:"next_cursor"` // pass this as cursor in the next request; 0 = end of feed
	Total      int64      `json:"total"`       // total entries in the feed sorted set
}

// FeedCache manages pre-computed video feeds for users.
type FeedCache struct {
	client *goredis.Client
	// luaFanOut is a pre-compiled script that fans a single video out to a
	// batch of follower feed sorted sets in one round-trip.
	luaFanOut *goredis.Script
	// luaReleaseLock is the standard compare-and-delete distributed lock release.
	luaReleaseLock *goredis.Script
}

// NewFeedCache constructs a FeedCache and pre-compiles all Lua scripts.
func NewFeedCache(client *goredis.Client) *FeedCache {
	return &FeedCache{
		client: client,
		luaFanOut: goredis.NewScript(`
			-- KEYS[1..N] = following_feed:{userID} keys
			-- ARGV[1]    = score (float string)
			-- ARGV[2]    = videoID (member)
			-- ARGV[3]    = window size (integer)
			-- ARGV[4]    = TTL in seconds (integer)
			local score    = ARGV[1]
			local vid      = ARGV[2]
			local window   = tonumber(ARGV[3])
			local ttl_secs = tonumber(ARGV[4])
			local updated  = 0
			for i = 1, #KEYS do
				redis.call('ZADD', KEYS[i], score, vid)
				redis.call('ZREMRANGEBYRANK', KEYS[i], 0, -(window + 1))
				redis.call('EXPIRE', KEYS[i], ttl_secs)
				updated = updated + 1
			end
			return updated
		`),
		luaReleaseLock: goredis.NewScript(`
			if redis.call('GET', KEYS[1]) == ARGV[1] then
				return redis.call('DEL', KEYS[1])
			end
			return 0
		`),
	}
}

// ----------------------------------------------------------------------------
// For-You Feed (recommendation-based)
// ----------------------------------------------------------------------------

// SetForYouFeed atomically replaces the For-You feed for a user with a new
// pre-ranked list of items. items must be sorted by Score descending.
// Excess items beyond FeedWindowSize are discarded before writing.
func (fc *FeedCache) SetForYouFeed(ctx context.Context, userID string, items []FeedItem) error {
	if len(items) > FeedWindowSize {
		items = items[:FeedWindowSize]
	}

	key := fmt.Sprintf(keyForYouFeed, userID)
	members := make([]goredis.Z, 0, len(items))
	for i := range items {
		members = append(members, goredis.Z{
			Score:  items[i].Score,
			Member: items[i].VideoID,
		})
		if err := fc.setFeedItemMeta(ctx, items[i].AuthorID, &items[i]); err != nil {
			return fmt.Errorf("set feed item meta [%s]: %w", items[i].VideoID, err)
		}
	}

	pipe := fc.client.TxPipeline()
	pipe.Del(ctx, key)
	if len(members) > 0 {
		pipe.ZAdd(ctx, key, members...)
	}
	pipe.Expire(ctx, key, FeedTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// GetForYouPage returns a paginated slice of the For-You feed.
//
// cursor is the exclusive upper-bound score from the previous page's last item.
// Pass 0 for the first page. Returns FeedPage.NextCursor == 0 when no more items remain.
func (fc *FeedCache) GetForYouPage(ctx context.Context, userID string, cursor float64, limit int) (*FeedPage, error) {
	if limit <= 0 || limit > FeedWindowSize {
		limit = FeedPageSize
	}
	key := fmt.Sprintf(keyForYouFeed, userID)

	upperBound := "+inf"
	if cursor > 0 {
		// Exclusive lower bound using the open-interval notation.
		upperBound = fmt.Sprintf("(%v", cursor)
	}

	results, err := fc.client.ZRevRangeByScoreWithScores(ctx, key, &goredis.ZRangeBy{
		Max:    upperBound,
		Min:    "-inf",
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrangebyscore foryou: %w", err)
	}

	total, _ := fc.client.ZCard(ctx, key).Result()
	items, nextCursor, err := fc.hydrateResults(ctx, userID, results)
	if err != nil {
		return nil, err
	}

	return &FeedPage{Items: items, NextCursor: nextCursor, Total: total}, nil
}

// AppendToForYouFeed inserts a single new video at the top of the feed and
// trims the sorted set to FeedWindowSize by removing the lowest-scored entry.
// Used by the recommendation service when a high-relevance video becomes available.
func (fc *FeedCache) AppendToForYouFeed(ctx context.Context, userID string, item FeedItem) error {
	key := fmt.Sprintf(keyForYouFeed, userID)
	pipe := fc.client.TxPipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: item.Score, Member: item.VideoID})
	// Keep only the top FeedWindowSize entries (remove lowest-scored).
	pipe.ZRemRangeByRank(ctx, key, 0, int64(-FeedWindowSize-1))
	pipe.Expire(ctx, key, FeedTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	return fc.setFeedItemMeta(ctx, item.AuthorID, &item)
}

// RemoveFromForYouFeed removes a video from a user's For-You feed.
// Used when a video is deleted, taken down by moderation, or marked not interested.
func (fc *FeedCache) RemoveFromForYouFeed(ctx context.Context, userID, videoID string) error {
	return fc.client.ZRem(ctx, fmt.Sprintf(keyForYouFeed, userID), videoID).Err()
}

// ----------------------------------------------------------------------------
// Following Feed (chronological — fan-out-on-write)
// ----------------------------------------------------------------------------

// PushToFollowingFeed prepends a new video to all followers' following feeds.
// This implements the fan-out-on-write strategy: called immediately after a
// creator publishes a video so followers see it without a feed rebuild.
//
// followerIDs is processed in FeedFanOutChunkSize batches via a Lua script to
// avoid issuing thousands of individual ZADD commands.
func (fc *FeedCache) PushToFollowingFeed(ctx context.Context, followerIDs []string, item FeedItem) error {
	// Use nanosecond-precision unix time as the score so ordering is stable
	// even when multiple creators post within the same second.
	score := float64(item.CreatedAt.UnixNano()) / 1e9

	// Pre-write the metadata once using the author's namespace.
	if err := fc.setFeedItemMeta(ctx, item.AuthorID, &item); err != nil {
		return fmt.Errorf("push to following: set meta: %w", err)
	}

	scoreStr := fmt.Sprintf("%v", score)
	windowStr := fmt.Sprintf("%d", FeedWindowSize)
	ttlStr := fmt.Sprintf("%d", int(FeedTTL.Seconds()))

	for start := 0; start < len(followerIDs); start += FeedFanOutChunkSize {
		end := start + FeedFanOutChunkSize
		if end > len(followerIDs) {
			end = len(followerIDs)
		}
		chunk := followerIDs[start:end]

		keys := make([]string, len(chunk))
		for i, uid := range chunk {
			keys[i] = fmt.Sprintf(keyFollowingFeed, uid)
		}

		if err := fc.luaFanOut.Run(ctx, fc.client, keys,
			scoreStr, item.VideoID, windowStr, ttlStr,
		).Err(); err != nil && !errors.Is(err, goredis.Nil) {
			return fmt.Errorf("fan-out lua [%d:%d]: %w", start, end, err)
		}
	}
	return nil
}

// GetFollowingPage returns a paginated slice of the chronological following feed.
//
// cursor is the exclusive score lower bound from the previous page's last item.
// Pass 0 for the first page.
func (fc *FeedCache) GetFollowingPage(ctx context.Context, userID string, cursor float64, limit int) (*FeedPage, error) {
	if limit <= 0 || limit > FeedWindowSize {
		limit = FeedPageSize
	}
	key := fmt.Sprintf(keyFollowingFeed, userID)

	upperBound := "+inf"
	if cursor > 0 {
		upperBound = fmt.Sprintf("(%v", cursor)
	}

	results, err := fc.client.ZRevRangeByScoreWithScores(ctx, key, &goredis.ZRangeBy{
		Max:    upperBound,
		Min:    "-inf",
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrangebyscore following: %w", err)
	}

	total, _ := fc.client.ZCard(ctx, key).Result()
	// Following feed metadata is stored under the author's ID; we do not know
	// the author without looking up the meta — use a fallback empty ownerID so
	// hydrateResults tries the videoID as a key suffix.
	items, nextCursor, err := fc.hydrateResults(ctx, userID, results)
	if err != nil {
		return nil, err
	}
	return &FeedPage{Items: items, NextCursor: nextCursor, Total: total}, nil
}

// RemoveFromFollowingFeed removes a video from a user's following feed.
func (fc *FeedCache) RemoveFromFollowingFeed(ctx context.Context, userID, videoID string) error {
	return fc.client.ZRem(ctx, fmt.Sprintf(keyFollowingFeed, userID), videoID).Err()
}

// ----------------------------------------------------------------------------
// Feed invalidation
// ----------------------------------------------------------------------------

// InvalidateUserFeed removes both feed sorted sets for a user, forcing a full
// rebuild on the next request. Call this after major profile or privacy changes.
func (fc *FeedCache) InvalidateUserFeed(ctx context.Context, userID string) error {
	pipe := fc.client.Pipeline()
	pipe.Del(ctx, fmt.Sprintf(keyForYouFeed, userID))
	pipe.Del(ctx, fmt.Sprintf(keyFollowingFeed, userID))
	pipe.Del(ctx, fmt.Sprintf(keyFeedCursor, userID))
	pipe.Del(ctx, fmt.Sprintf(keyFeedNew, userID))
	_, err := pipe.Exec(ctx)
	return err
}

// InvalidateVideoFromFeeds removes a specific video from the cached feeds of
// a list of affected users. Used when a video is deleted or taken down.
//
// affectedUserIDs is chunked into batches of 500 to bound pipeline size.
func (fc *FeedCache) InvalidateVideoFromFeeds(ctx context.Context, videoID string, affectedUserIDs []string) error {
	const chunkSize = 500
	for start := 0; start < len(affectedUserIDs); start += chunkSize {
		end := start + chunkSize
		if end > len(affectedUserIDs) {
			end = len(affectedUserIDs)
		}
		pipe := fc.client.Pipeline()
		for _, uid := range affectedUserIDs[start:end] {
			pipe.ZRem(ctx, fmt.Sprintf(keyForYouFeed, uid), videoID)
			pipe.ZRem(ctx, fmt.Sprintf(keyFollowingFeed, uid), videoID)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("invalidate video from feeds [%d:%d]: %w", start, end, err)
		}
	}
	return nil
}

// InvalidateFollowerFeeds removes all following-feed caches for a list of
// follower user IDs. Used when a creator account is banned or deleted.
func (fc *FeedCache) InvalidateFollowerFeeds(ctx context.Context, followerIDs []string) error {
	if len(followerIDs) == 0 {
		return nil
	}
	keys := make([]string, len(followerIDs))
	for i, uid := range followerIDs {
		keys[i] = fmt.Sprintf(keyFollowingFeed, uid)
	}
	return fc.client.Del(ctx, keys...).Err()
}

// ----------------------------------------------------------------------------
// Distributed feed-rebuild lock
// ----------------------------------------------------------------------------

// AcquireFeedLock tries to acquire a mutex so only one goroutine rebuilds the
// feed at a time. lockValue should be a unique token (e.g. a UUID) so the
// caller can safely release its own lock.
//
// Returns true if the lock was acquired.
func (fc *FeedCache) AcquireFeedLock(ctx context.Context, userID, lockValue string) (bool, error) {
	key := fmt.Sprintf(keyFeedLock, userID)
	return fc.client.SetNX(ctx, key, lockValue, FeedLockTTL).Result()
}

// ReleaseFeedLock releases the lock only if the caller is still the owner.
// Uses a Lua script for atomic compare-and-delete.
func (fc *FeedCache) ReleaseFeedLock(ctx context.Context, userID, lockValue string) error {
	key := fmt.Sprintf(keyFeedLock, userID)
	return fc.luaReleaseLock.Run(ctx, fc.client, []string{key}, lockValue).Err()
}

// IsFeedLocked reports whether a rebuild is currently in progress for a user.
func (fc *FeedCache) IsFeedLocked(ctx context.Context, userID string) (bool, error) {
	key := fmt.Sprintf(keyFeedLock, userID)
	err := fc.client.Get(ctx, key).Err()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ----------------------------------------------------------------------------
// Feed cursor (pagination bookmark)
// ----------------------------------------------------------------------------

// SetFeedCursor persists the user's last read position (the score of the last
// item seen) so the client can resume scrolling across sessions.
func (fc *FeedCache) SetFeedCursor(ctx context.Context, userID string, score float64) error {
	return fc.client.Set(ctx, fmt.Sprintf(keyFeedCursor, userID), score, FeedTTL).Err()
}

// GetFeedCursor retrieves the last read position. Returns 0 if not set.
func (fc *FeedCache) GetFeedCursor(ctx context.Context, userID string) (float64, error) {
	val, err := fc.client.Get(ctx, fmt.Sprintf(keyFeedCursor, userID)).Float64()
	if errors.Is(err, goredis.Nil) {
		return 0, nil
	}
	return val, err
}

// ----------------------------------------------------------------------------
// New video notifications
// ----------------------------------------------------------------------------

// IncrNewVideoCount increments the unseen-video counter for a user.
// Used to show the "N new videos" banner without loading the full feed.
func (fc *FeedCache) IncrNewVideoCount(ctx context.Context, userID string, delta int64) error {
	pipe := fc.client.Pipeline()
	pipe.IncrBy(ctx, fmt.Sprintf(keyFeedNew, userID), delta)
	pipe.Expire(ctx, fmt.Sprintf(keyFeedNew, userID), FeedTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// GetAndResetNewVideoCount returns the current unseen count and resets it to 0.
// Call this when the user opens the feed.
func (fc *FeedCache) GetAndResetNewVideoCount(ctx context.Context, userID string) (int64, error) {
	key := fmt.Sprintf(keyFeedNew, userID)
	count, err := fc.client.GetDel(ctx, key).Int64()
	if errors.Is(err, goredis.Nil) {
		return 0, nil
	}
	return count, err
}

// ----------------------------------------------------------------------------
// Seen-video deduplication
// ----------------------------------------------------------------------------

// MarkVideoSeen records that a user has seen a video so it is not served again.
// The seen set has a fixed TTL (FeedSeenSetTTL) after which it is recycled.
func (fc *FeedCache) MarkVideoSeen(ctx context.Context, userID, videoID string) error {
	key := fmt.Sprintf(keyFeedSeen, userID)
	pipe := fc.client.Pipeline()
	pipe.SAdd(ctx, key, videoID)
	pipe.Expire(ctx, key, FeedSeenSetTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// MarkVideosSeen records multiple videos as seen in a single pipeline.
func (fc *FeedCache) MarkVideosSeen(ctx context.Context, userID string, videoIDs []string) error {
	if len(videoIDs) == 0 {
		return nil
	}
	key := fmt.Sprintf(keyFeedSeen, userID)
	members := make([]interface{}, len(videoIDs))
	for i, vid := range videoIDs {
		members[i] = vid
	}
	pipe := fc.client.Pipeline()
	pipe.SAdd(ctx, key, members...)
	pipe.Expire(ctx, key, FeedSeenSetTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// IsVideoSeen reports whether the user has already seen a video.
func (fc *FeedCache) IsVideoSeen(ctx context.Context, userID, videoID string) (bool, error) {
	return fc.client.SIsMember(ctx, fmt.Sprintf(keyFeedSeen, userID), videoID).Result()
}

// FilterSeenVideos returns only the video IDs from the input slice that the
// user has NOT yet seen, preserving order.
func (fc *FeedCache) FilterSeenVideos(ctx context.Context, userID string, videoIDs []string) ([]string, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}
	key := fmt.Sprintf(keyFeedSeen, userID)
	members := make([]interface{}, len(videoIDs))
	for i, vid := range videoIDs {
		members[i] = vid
	}
	seen, err := fc.client.SMIsMember(ctx, key, members...).Result()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(videoIDs))
	for i, wasSeen := range seen {
		if !wasSeen {
			out = append(out, videoIDs[i])
		}
	}
	return out, nil
}

// SeenVideoCount returns how many distinct videos the user has seen.
func (fc *FeedCache) SeenVideoCount(ctx context.Context, userID string) (int64, error) {
	return fc.client.SCard(ctx, fmt.Sprintf(keyFeedSeen, userID)).Result()
}

// ClearSeenVideos resets the seen-video set, allowing the feed to replay content.
func (fc *FeedCache) ClearSeenVideos(ctx context.Context, userID string) error {
	return fc.client.Del(ctx, fmt.Sprintf(keyFeedSeen, userID)).Err()
}

// ----------------------------------------------------------------------------
// Feed statistics helpers
// ----------------------------------------------------------------------------

// FeedSize returns the number of videos currently in the For-You feed.
func (fc *FeedCache) FeedSize(ctx context.Context, userID string) (int64, error) {
	return fc.client.ZCard(ctx, fmt.Sprintf(keyForYouFeed, userID)).Result()
}

// FollowingFeedSize returns the number of videos in the following feed.
func (fc *FeedCache) FollowingFeedSize(ctx context.Context, userID string) (int64, error) {
	return fc.client.ZCard(ctx, fmt.Sprintf(keyFollowingFeed, userID)).Result()
}

// IsFeedCached reports whether a user's For-You feed exists in Redis.
func (fc *FeedCache) IsFeedCached(ctx context.Context, userID string) (bool, error) {
	n, err := fc.client.Exists(ctx, fmt.Sprintf(keyForYouFeed, userID)).Result()
	return n > 0, err
}

// UpdateVideoEngagement refreshes the cached engagement counters for a video
// stored in any user's feed metadata. This avoids stale like/view counts when
// scrolling without a full DB query.
func (fc *FeedCache) UpdateVideoEngagement(ctx context.Context, authorID, videoID string, likes, comments, shares, views int64) error {
	key := fmt.Sprintf(keyFeedMeta, authorID, videoID)
	raw, err := fc.client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil // not in cache, nothing to update
	}
	if err != nil {
		return err
	}
	var item FeedItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return err
	}
	item.LikeCount = likes
	item.CommentCount = comments
	item.ShareCount = shares
	item.ViewCount = views
	updated, _ := json.Marshal(item)
	return fc.client.Set(ctx, key, updated, FeedItemMetaTTL).Err()
}

// ----------------------------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------------------------

// setFeedItemMeta persists FeedItem JSON to a STRING key scoped to ownerID.
// ownerID is the video author's ID — metadata is stored once per video, not
// per follower, to keep memory usage O(videos) rather than O(followers×videos).
func (fc *FeedCache) setFeedItemMeta(ctx context.Context, ownerID string, item *FeedItem) error {
	key := fmt.Sprintf(keyFeedMeta, ownerID, item.VideoID)
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return fc.client.Set(ctx, key, raw, FeedItemMetaTTL).Err()
}

// hydrateResults fetches FeedItem metadata for a slice of scored members,
// returning the enriched items and the score of the last item (next page cursor).
// Items whose metadata has expired are omitted silently.
func (fc *FeedCache) hydrateResults(ctx context.Context, ownerID string, results []goredis.Z) ([]FeedItem, float64, error) {
	if len(results) == 0 {
		return nil, 0, nil
	}

	keys := make([]string, len(results))
	for i, z := range results {
		keys[i] = fmt.Sprintf(keyFeedMeta, ownerID, z.Member.(string))
	}

	rawValues, err := fc.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("mget feed meta: %w", err)
	}

	items := make([]FeedItem, 0, len(rawValues))
	var lowestScore float64
	for i, v := range rawValues {
		lowestScore = results[i].Score
		if v == nil {
			// Metadata expired; skip but do not error — the caller will see
			// fewer items than requested and can request more.
			continue
		}
		var item FeedItem
		if err := json.Unmarshal([]byte(v.(string)), &item); err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, lowestScore, nil
}
