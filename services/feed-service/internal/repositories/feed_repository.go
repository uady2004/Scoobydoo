// Package repositories provides data-access implementations for the feed
// service, backed by Redis (sorted sets for feeds and sets for deduplication)
// and PostgreSQL/PostGIS (video metadata, nearby queries, category queries).
package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/feed-service/internal/models"
)

// ---- Redis key patterns -----------------------------------------------------

const (
	// keyForYouFeed is the Redis sorted-set key for a user's For-You feed.
	// Format: feed:foryou:{userID}
	keyForYouFeed = "feed:foryou:%s"
	// keyFollowingFeed is the Redis sorted-set for a user's following feed.
	// Format: feed:following:{userID}
	keyFollowingFeed = "feed:following:%s"
	// keyTrendingFeed is the global trending sorted set.
	// Format: feed:trending:global
	keyTrendingFeed = "feed:trending:global"
	// keyTrendingCategory is the category-scoped trending sorted set.
	// Format: feed:trending:cat:{category}
	keyTrendingCategory = "feed:trending:cat:%s"
	// keyExploreFeed is a sorted set for explore (category-based) feeds.
	// Format: feed:explore:{category}
	keyExploreFeed = "feed:explore:%s"
	// keyNearbyGeo stores video IDs in a Redis GEO set.
	// Format: feed:nearby:geo
	keyNearbyGeo = "feed:nearby:geo"
	// keySeenVideos tracks which video IDs a user has already seen in a session.
	// Format: feed:seen:{userID}:{sessionID}
	keySeenVideos = "feed:seen:%s:%s"
	// keyPrecomputeMeta stores PrecomputeMeta JSON for a user's feed.
	// Format: feed:meta:{feedType}:{userID}
	keyPrecomputeMeta = "feed:meta:%s:%s"
	// keyVideoMeta caches JSON-encoded FeedItem stubs for video metadata.
	// Format: video:meta:{videoID}
	keyVideoMeta = "video:meta:%s"
	// keyActiveUsers is a Redis sorted set of user IDs scored by last-active
	// timestamp, used by the precompute worker to target frequent users.
	keyActiveUsers = "feed:active_users"

	// videoMetaCacheTTL is the default TTL for individual video metadata blobs.
	videoMetaCacheTTL = 30 * time.Minute
)

// ---- FeedRepository ---------------------------------------------------------

// FeedRepository is the single data-access object for the feed service.
// It deliberately combines Redis and Postgres access: keeping them together
// makes it easier to reason about cache/DB consistency without introducing an
// extra layer.
type FeedRepository struct {
	rdb    redis.UniversalClient
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewFeedRepository constructs a FeedRepository.
func NewFeedRepository(rdb redis.UniversalClient, db *pgxpool.Pool, logger *zap.Logger) *FeedRepository {
	return &FeedRepository{rdb: rdb, db: db, logger: logger}
}

// ---- For-You feed -----------------------------------------------------------

// GetForYouFeed returns a page of feed items from the user's For-You sorted set
// in Redis using ZREVRANGEBYSCORE cursor-based pagination.
//
// The sorted set is keyed by descending score so higher scores appear first.
// Returns (items, nextCursor, error). nextCursor is nil when there is no next page.
func (r *FeedRepository) GetForYouFeed(
	ctx context.Context,
	userID string,
	cursor *models.FeedCursor,
	limit int,
) ([]*models.FeedItem, *models.FeedCursor, error) {
	key := fmt.Sprintf(keyForYouFeed, userID)
	return r.getFromSortedSet(ctx, key, models.FeedTypeForYou, cursor, limit)
}

// GetFollowingFeed returns a page from the user's following feed sorted set.
func (r *FeedRepository) GetFollowingFeed(
	ctx context.Context,
	userID string,
	cursor *models.FeedCursor,
	limit int,
) ([]*models.FeedItem, *models.FeedCursor, error) {
	key := fmt.Sprintf(keyFollowingFeed, userID)
	return r.getFromSortedSet(ctx, key, models.FeedTypeFollowing, cursor, limit)
}

// GetTrendingFeed returns a page from the global trending sorted set.
func (r *FeedRepository) GetTrendingFeed(
	ctx context.Context,
	cursor *models.FeedCursor,
	limit int,
) ([]*models.FeedItem, *models.FeedCursor, error) {
	return r.getFromSortedSet(ctx, keyTrendingFeed, models.FeedTypeTrending, cursor, limit)
}

// GetNearbyFeed queries PostGIS for videos uploaded within radiusKm of
// (lat, lon), then returns FeedItems enriched with distance. Cursor pagination
// is implemented using OFFSET based on the encoded score field.
func (r *FeedRepository) GetNearbyFeed(
	ctx context.Context,
	userID string,
	lat, lon, radiusKm float64,
	cursor *models.FeedCursor,
	limit int,
) ([]*models.FeedItem, *models.FeedCursor, error) {
	offset := 0
	if cursor != nil {
		// For nearby feeds, Score carries the OFFSET into the result set.
		offset = int(cursor.Score)
	}

	const query = `
		SELECT
			v.id,
			v.user_id,
			u.username,
			u.display_name,
			COALESCE(u.avatar_url, '') AS avatar_url,
			COALESCE(u.is_verified, false) AS is_verified,
			COALESCE(v.title, '') AS title,
			COALESCE(v.description, '') AS description,
			COALESCE(v.thumbnail_url, '') AS thumbnail_url,
			COALESCE(v.video_url, '') AS video_url,
			COALESCE(v.duration_seconds, 0) AS duration_seconds,
			COALESCE(v.view_count, 0) AS view_count,
			COALESCE(v.like_count, 0) AS like_count,
			COALESCE(v.comment_count, 0) AS comment_count,
			COALESCE(v.share_count, 0) AS share_count,
			COALESCE(v.tags, ARRAY[]::text[]) AS tags,
			COALESCE(v.category, '') AS category,
			v.created_at,
			ST_Y(v.location::geometry) AS latitude,
			ST_X(v.location::geometry) AS longitude,
			ST_Distance(
				v.location::geography,
				ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography
			) / 1000.0 AS distance_km
		FROM videos v
		JOIN users u ON u.id = v.user_id
		WHERE
			v.status = 'published'
			AND v.location IS NOT NULL
			AND ST_DWithin(
				v.location::geography,
				ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography,
				$3 * 1000
			)
		ORDER BY distance_km ASC, v.created_at DESC
		LIMIT $4 OFFSET $5`

	rows, err := r.db.Query(ctx, query, lat, lon, radiusKm, limit+1, offset)
	if err != nil {
		return nil, nil, fmt.Errorf("nearby feed query: %w", err)
	}
	defer rows.Close()

	items, err := r.scanFeedItemsWithDistance(rows)
	if err != nil {
		return nil, nil, err
	}

	var nextCursor *models.FeedCursor
	if len(items) > limit {
		items = items[:limit]
		nextCursor = &models.FeedCursor{
			Score:     float64(offset + limit),
			VideoID:   items[len(items)-1].VideoID,
			FeedType:  models.FeedTypeNearby,
			Timestamp: time.Now(),
		}
	}

	for _, item := range items {
		item.FeedType = models.FeedTypeNearby
	}

	return items, nextCursor, nil
}

// GetExploreFeed returns a page from the explore (all-categories) sorted set,
// mixing content from multiple categories by rotating through them.
func (r *FeedRepository) GetExploreFeed(
	ctx context.Context,
	cursor *models.FeedCursor,
	limit int,
) ([]*models.FeedItem, *models.FeedCursor, error) {
	key := fmt.Sprintf(keyExploreFeed, "all")
	return r.getFromSortedSet(ctx, key, models.FeedTypeExplore, cursor, limit)
}

// GetCategoryFeed returns a page from the category-specific explore sorted set.
func (r *FeedRepository) GetCategoryFeed(
	ctx context.Context,
	category string,
	cursor *models.FeedCursor,
	limit int,
) ([]*models.FeedItem, *models.FeedCursor, error) {
	key := fmt.Sprintf(keyExploreFeed, category)
	return r.getFromSortedSet(ctx, key, models.FeedTypeCategory, cursor, limit)
}

// ---- Cache invalidation / precompute ----------------------------------------

// InvalidateFeed removes the cached feed sorted set for a user so it will be
// rebuilt on the next request.
func (r *FeedRepository) InvalidateFeed(ctx context.Context, userID string, ft models.FeedType) error {
	var key string
	switch ft {
	case models.FeedTypeForYou:
		key = fmt.Sprintf(keyForYouFeed, userID)
	case models.FeedTypeFollowing:
		key = fmt.Sprintf(keyFollowingFeed, userID)
	default:
		return fmt.Errorf("InvalidateFeed: unsupported feed type %q", ft)
	}
	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("invalidate feed %s/%s: %w", ft, userID, err)
	}
	// Remove precompute metadata.
	metaKey := fmt.Sprintf(keyPrecomputeMeta, ft, userID)
	_ = r.rdb.Del(ctx, metaKey)
	return nil
}

// PrecomputeFeed stores a pre-built list of video IDs into the Redis sorted set
// for a user. Scores are the floating-point rank scores from the recommendation
// engine (higher = shown first). Items are stored with FeedScore as the member
// score, enabling ZREVRANGEBYSCORE cursor pagination.
func (r *FeedRepository) PrecomputeFeed(
	ctx context.Context,
	userID string,
	ft models.FeedType,
	items []*models.FeedItem,
	ttl time.Duration,
) error {
	var key string
	switch ft {
	case models.FeedTypeForYou:
		key = fmt.Sprintf(keyForYouFeed, userID)
	case models.FeedTypeFollowing:
		key = fmt.Sprintf(keyFollowingFeed, userID)
	default:
		return fmt.Errorf("PrecomputeFeed: unsupported feed type %q", ft)
	}

	pipe := r.rdb.Pipeline()

	// Delete old set.
	pipe.Del(ctx, key)

	if len(items) > 0 {
		members := make([]redis.Z, 0, len(items))
		for i, item := range items {
			// Score: use FeedScore if set, otherwise use insertion-order rank.
			score := item.FeedScore
			if score == 0 {
				score = float64(len(items) - i)
			}
			members = append(members, redis.Z{
				Score:  score,
				Member: item.VideoID,
			})
		}
		pipe.ZAdd(ctx, key, members...)
		pipe.Expire(ctx, key, ttl)

		// Cache individual video metadata for fast retrieval later.
		for _, item := range items {
			metaJSON, err := json.Marshal(item)
			if err == nil {
				vmKey := fmt.Sprintf(keyVideoMeta, item.VideoID)
				pipe.Set(ctx, vmKey, metaJSON, ttl+5*time.Minute)
			}
		}
	}

	// Write precompute metadata for staleness detection.
	meta := &models.PrecomputeMeta{
		UserID:     userID,
		FeedType:   ft,
		ComputedAt: time.Now(),
		ExpiresAt:  time.Now().Add(ttl),
		VideoCount: len(items),
	}
	metaJSON, _ := json.Marshal(meta)
	metaKey := fmt.Sprintf(keyPrecomputeMeta, ft, userID)
	pipe.Set(ctx, metaKey, metaJSON, ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("precompute feed %s/%s: %w", ft, userID, err)
	}
	return nil
}

// HasPrecomputedFeed reports whether a non-expired pre-computed feed exists for
// the user.
func (r *FeedRepository) HasPrecomputedFeed(ctx context.Context, userID string, ft models.FeedType) (bool, error) {
	key := fmt.Sprintf(keyPrecomputeMeta, ft, userID)
	exists, err := r.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// ---- Trending sorted set management -----------------------------------------

// UpsertTrendingScore sets the trending score for videoID in both the global
// trending set and the category-specific set.
func (r *FeedRepository) UpsertTrendingScore(
	ctx context.Context,
	videoID string,
	category string,
	score float64,
) error {
	pipe := r.rdb.Pipeline()
	pipe.ZAdd(ctx, keyTrendingFeed, redis.Z{Score: score, Member: videoID})
	if category != "" {
		catKey := fmt.Sprintf(keyTrendingCategory, category)
		pipe.ZAdd(ctx, catKey, redis.Z{Score: score, Member: videoID})
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert trending score %s: %w", videoID, err)
	}
	return nil
}

// RemoveTrendingEntry removes a video from the trending sorted sets.
func (r *FeedRepository) RemoveTrendingEntry(ctx context.Context, videoID, category string) error {
	pipe := r.rdb.Pipeline()
	pipe.ZRem(ctx, keyTrendingFeed, videoID)
	if category != "" {
		pipe.ZRem(ctx, fmt.Sprintf(keyTrendingCategory, category), videoID)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// TrimTrendingSet keeps only the top-N entries in the trending sorted set to
// prevent unbounded growth. ZREMRANGEBYRANK removes lowest-scoring members.
func (r *FeedRepository) TrimTrendingSet(ctx context.Context, keepTop int64) error {
	if err := r.rdb.ZRemRangeByRank(ctx, keyTrendingFeed, 0, -(keepTop+1)).Err(); err != nil {
		return fmt.Errorf("trim trending set: %w", err)
	}
	return nil
}

// GetTrendingVideoIDs returns at most `limit` video IDs from the global trending
// set in descending score order, using ZREVRANGEBYSCORE with maxScore as upper bound.
func (r *FeedRepository) GetTrendingVideoIDs(
	ctx context.Context,
	maxScore float64,
	limit int,
) ([]redis.Z, error) {
	maxScoreStr := "+inf"
	if maxScore < 1e18 { // sentinel for "no upper bound"
		maxScoreStr = strconv.FormatFloat(maxScore, 'f', -1, 64)
	}
	opt := &redis.ZRangeBy{
		Min:    "-inf",
		Max:    maxScoreStr,
		Offset: 0,
		Count:  int64(limit),
	}
	return r.rdb.ZRevRangeByScoreWithScores(ctx, keyTrendingFeed, opt).Result()
}

// ---- Deduplication ----------------------------------------------------------

// MarkVideosAsSeen records video IDs in the per-user-session seen-set so they
// are not shown again during the same session using Redis SADD.
// Returns the number of new IDs added.
func (r *FeedRepository) MarkVideosAsSeen(
	ctx context.Context,
	userID, sessionID string,
	videoIDs []string,
	ttl time.Duration,
) (int64, error) {
	if len(videoIDs) == 0 {
		return 0, nil
	}
	key := fmt.Sprintf(keySeenVideos, userID, sessionID)
	members := make([]interface{}, len(videoIDs))
	for i, id := range videoIDs {
		members[i] = id
	}
	added, err := r.rdb.SAdd(ctx, key, members...).Result()
	if err != nil {
		return 0, fmt.Errorf("mark seen: %w", err)
	}
	// Refresh TTL on every write.
	_ = r.rdb.Expire(ctx, key, ttl)
	return added, nil
}

// FilterSeenVideos returns the subset of videoIDs that the user has NOT yet
// seen in the given session. Uses SMISMEMBER for a single round-trip.
func (r *FeedRepository) FilterSeenVideos(
	ctx context.Context,
	userID, sessionID string,
	videoIDs []string,
) ([]string, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}
	key := fmt.Sprintf(keySeenVideos, userID, sessionID)
	members := make([]interface{}, len(videoIDs))
	for i, id := range videoIDs {
		members[i] = id
	}
	results, err := r.rdb.SMIsMember(ctx, key, members...).Result()
	if err != nil {
		return nil, fmt.Errorf("filter seen: %w", err)
	}
	unseen := make([]string, 0, len(videoIDs))
	for i, seen := range results {
		if !seen {
			unseen = append(unseen, videoIDs[i])
		}
	}
	return unseen, nil
}

// ClearSeenVideos deletes the seen-set for a user session (used when the
// session is explicitly reset).
func (r *FeedRepository) ClearSeenVideos(ctx context.Context, userID, sessionID string) error {
	key := fmt.Sprintf(keySeenVideos, userID, sessionID)
	return r.rdb.Del(ctx, key).Err()
}

// ---- Active users -----------------------------------------------------------

// TrackActiveUser updates the user's last-active timestamp in the active-users
// sorted set. Used by the precompute worker to find high-frequency users.
func (r *FeedRepository) TrackActiveUser(ctx context.Context, userID string) error {
	return r.rdb.ZAdd(ctx, keyActiveUsers, redis.Z{
		Score:  float64(time.Now().UnixMilli()),
		Member: userID,
	}).Err()
}

// GetActiveUsers returns up to `limit` user IDs that were most recently active.
func (r *FeedRepository) GetActiveUsers(ctx context.Context, limit int64) ([]string, error) {
	return r.rdb.ZRevRange(ctx, keyActiveUsers, 0, limit-1).Result()
}

// ---- Video metadata (DB) ----------------------------------------------------

// GetVideosByIDs fetches lightweight FeedItem stubs for the supplied video IDs.
// It checks the Redis cache first; only missing IDs are fetched from Postgres.
// Results maintain the same order as the input slice.
func (r *FeedRepository) GetVideosByIDs(
	ctx context.Context,
	videoIDs []string,
	requestingUserID string,
) ([]*models.FeedItem, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}

	// Check cache first.
	items, missing := r.fetchVideoMetaFromCache(ctx, videoIDs)

	if len(missing) > 0 {
		dbItems, err := r.fetchVideoMetaFromDB(ctx, missing, requestingUserID)
		if err != nil {
			return nil, err
		}
		// Merge and cache.
		for _, item := range dbItems {
			items[item.VideoID] = item
			r.cacheVideoMeta(ctx, item)
		}
	}

	// Re-order to match input slice.
	result := make([]*models.FeedItem, 0, len(videoIDs))
	for _, id := range videoIDs {
		if item, ok := items[id]; ok {
			result = append(result, item)
		}
	}
	return result, nil
}

// fetchVideoMetaFromCache attempts to retrieve FeedItem JSON blobs from Redis.
// Returns a map of found items and a slice of IDs not found in cache.
func (r *FeedRepository) fetchVideoMetaFromCache(
	ctx context.Context,
	videoIDs []string,
) (map[string]*models.FeedItem, []string) {
	keys := make([]string, len(videoIDs))
	for i, id := range videoIDs {
		keys[i] = fmt.Sprintf(keyVideoMeta, id)
	}
	vals, err := r.rdb.MGet(ctx, keys...).Result()
	found := make(map[string]*models.FeedItem)
	var missing []string
	if err != nil {
		return found, videoIDs
	}
	for i, val := range vals {
		if val == nil {
			missing = append(missing, videoIDs[i])
			continue
		}
		s, ok := val.(string)
		if !ok {
			missing = append(missing, videoIDs[i])
			continue
		}
		var item models.FeedItem
		if err := json.Unmarshal([]byte(s), &item); err != nil {
			missing = append(missing, videoIDs[i])
			continue
		}
		found[item.VideoID] = &item
	}
	return found, missing
}

// cacheVideoMeta stores a FeedItem JSON blob in Redis with a default TTL.
func (r *FeedRepository) cacheVideoMeta(ctx context.Context, item *models.FeedItem) {
	b, err := json.Marshal(item)
	if err != nil {
		return
	}
	key := fmt.Sprintf(keyVideoMeta, item.VideoID)
	_ = r.rdb.Set(ctx, key, b, videoMetaCacheTTL)
}

// fetchVideoMetaFromDB fetches video metadata rows from Postgres for the given
// list of IDs and returns them as FeedItem stubs.
func (r *FeedRepository) fetchVideoMetaFromDB(
	ctx context.Context,
	videoIDs []string,
	requestingUserID string,
) ([]*models.FeedItem, error) {
	const query = `
		SELECT
			v.id,
			v.user_id,
			u.username,
			u.display_name,
			COALESCE(u.avatar_url, '') AS avatar_url,
			COALESCE(u.is_verified, false) AS is_verified,
			COALESCE(v.title, '') AS title,
			COALESCE(v.description, '') AS description,
			COALESCE(v.thumbnail_url, '') AS thumbnail_url,
			COALESCE(v.video_url, '') AS video_url,
			COALESCE(v.duration_seconds, 0) AS duration_seconds,
			COALESCE(v.view_count, 0) AS view_count,
			COALESCE(v.like_count, 0) AS like_count,
			COALESCE(v.comment_count, 0) AS comment_count,
			COALESCE(v.share_count, 0) AS share_count,
			COALESCE(v.tags, ARRAY[]::text[]) AS tags,
			COALESCE(v.category, '') AS category,
			v.created_at,
			COALESCE(
				EXISTS(SELECT 1 FROM video_likes vl WHERE vl.video_id = v.id AND vl.user_id = $2),
				false
			) AS is_liked,
			COALESCE(
				EXISTS(SELECT 1 FROM video_saves vs WHERE vs.video_id = v.id AND vs.user_id = $2),
				false
			) AS is_saved,
			COALESCE(
				EXISTS(SELECT 1 FROM follows f WHERE f.follower_id = $2 AND f.following_id = v.user_id),
				false
			) AS is_following,
			CASE WHEN v.location IS NOT NULL
				THEN ST_Y(v.location::geometry) ELSE NULL END AS latitude,
			CASE WHEN v.location IS NOT NULL
				THEN ST_X(v.location::geometry) ELSE NULL END AS longitude
		FROM videos v
		JOIN users u ON u.id = v.user_id
		WHERE v.id = ANY($1) AND v.status = 'published'`

	rows, err := r.db.Query(ctx, query, videoIDs, requestingUserID)
	if err != nil {
		return nil, fmt.Errorf("fetch video meta: %w", err)
	}
	defer rows.Close()

	return r.scanFeedItems(rows)
}

// ---- Following feed populate ------------------------------------------------

// GetFollowingVideoIDs returns the most recent video IDs published by accounts
// that userID follows, suitable for populating the following feed cache.
func (r *FeedRepository) GetFollowingVideoIDs(
	ctx context.Context,
	userID string,
	since time.Time,
	limit int,
) ([]*models.FeedItem, error) {
	const query = `
		SELECT
			v.id,
			v.user_id,
			u.username,
			u.display_name,
			COALESCE(u.avatar_url, '') AS avatar_url,
			COALESCE(u.is_verified, false) AS is_verified,
			COALESCE(v.title, '') AS title,
			COALESCE(v.description, '') AS description,
			COALESCE(v.thumbnail_url, '') AS thumbnail_url,
			COALESCE(v.video_url, '') AS video_url,
			COALESCE(v.duration_seconds, 0) AS duration_seconds,
			COALESCE(v.view_count, 0) AS view_count,
			COALESCE(v.like_count, 0) AS like_count,
			COALESCE(v.comment_count, 0) AS comment_count,
			COALESCE(v.share_count, 0) AS share_count,
			COALESCE(v.tags, ARRAY[]::text[]) AS tags,
			COALESCE(v.category, '') AS category,
			v.created_at,
			false AS is_liked,
			false AS is_saved,
			true AS is_following,
			NULL::double precision AS latitude,
			NULL::double precision AS longitude
		FROM videos v
		JOIN users u ON u.id = v.user_id
		JOIN follows f ON f.following_id = v.user_id
		WHERE
			f.follower_id = $1
			AND v.status = 'published'
			AND v.created_at >= $2
		ORDER BY v.created_at DESC
		LIMIT $3`

	rows, err := r.db.Query(ctx, query, userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get following video IDs: %w", err)
	}
	defer rows.Close()
	return r.scanFeedItems(rows)
}

// ---- Category feed (DB) -----------------------------------------------------

// GetCategoryVideoIDs fetches recently published videos in a given category,
// ordered by trending score, for populating category/explore feed caches.
func (r *FeedRepository) GetCategoryVideoIDs(
	ctx context.Context,
	category string,
	since time.Time,
	limit int,
) ([]*models.FeedItem, error) {
	const query = `
		SELECT
			v.id,
			v.user_id,
			u.username,
			u.display_name,
			COALESCE(u.avatar_url, '') AS avatar_url,
			COALESCE(u.is_verified, false) AS is_verified,
			COALESCE(v.title, '') AS title,
			COALESCE(v.description, '') AS description,
			COALESCE(v.thumbnail_url, '') AS thumbnail_url,
			COALESCE(v.video_url, '') AS video_url,
			COALESCE(v.duration_seconds, 0) AS duration_seconds,
			COALESCE(v.view_count, 0) AS view_count,
			COALESCE(v.like_count, 0) AS like_count,
			COALESCE(v.comment_count, 0) AS comment_count,
			COALESCE(v.share_count, 0) AS share_count,
			COALESCE(v.tags, ARRAY[]::text[]) AS tags,
			COALESCE(v.category, '') AS category,
			v.created_at,
			false AS is_liked,
			false AS is_saved,
			false AS is_following,
			NULL::double precision AS latitude,
			NULL::double precision AS longitude
		FROM videos v
		JOIN users u ON u.id = v.user_id
		WHERE
			v.category = $1
			AND v.status = 'published'
			AND v.created_at >= $2
		ORDER BY
			(v.view_count * 0.4 + v.like_count * 0.3 + v.share_count * 0.2 + v.comment_count * 0.1) DESC,
			v.created_at DESC
		LIMIT $3`

	rows, err := r.db.Query(ctx, query, category, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get category video IDs: %w", err)
	}
	defer rows.Close()
	return r.scanFeedItems(rows)
}

// GetVideoEngagementStats fetches raw engagement counters for a list of video
// IDs. Used by the trending updater to compute fresh scores.
func (r *FeedRepository) GetVideoEngagementStats(
	ctx context.Context,
	videoIDs []string,
) ([]*models.TrendingEntry, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}
	const query = `
		SELECT
			v.id,
			COALESCE(v.view_count, 0) AS view_count,
			COALESCE(v.like_count, 0) AS like_count,
			COALESCE(v.comment_count, 0) AS comment_count,
			COALESCE(v.share_count, 0) AS share_count,
			COALESCE(v.category, '') AS category,
			v.created_at
		FROM videos v
		WHERE v.id = ANY($1) AND v.status = 'published'`

	rows, err := r.db.Query(ctx, query, videoIDs)
	if err != nil {
		return nil, fmt.Errorf("get engagement stats: %w", err)
	}
	defer rows.Close()

	var entries []*models.TrendingEntry
	for rows.Next() {
		e := &models.TrendingEntry{}
		if err := rows.Scan(
			&e.VideoID,
			&e.Views,
			&e.Likes,
			&e.Comments,
			&e.Shares,
			&e.Category,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan engagement stats: %w", err)
		}
		e.LastUpdatedAt = time.Now()
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("engagement stats rows: %w", err)
	}
	return entries, nil
}

// GetRecentVideoIDs returns IDs for videos published in the last `windowHours`
// hours, used to seed the trending recalculation.
func (r *FeedRepository) GetRecentVideoIDs(ctx context.Context, windowHours int, limit int) ([]string, error) {
	since := time.Now().Add(-time.Duration(windowHours) * time.Hour)
	const query = `
		SELECT id FROM videos
		WHERE status = 'published' AND created_at >= $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent video IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan recent video ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ---- Precompute active user scanning ----------------------------------------

// GetUsersNeedingPrecompute returns user IDs whose pre-computed feed has either
// expired or never been computed. It scans the active-users sorted set and
// checks existence of the feed metadata key.
func (r *FeedRepository) GetUsersNeedingPrecompute(
	ctx context.Context,
	ft models.FeedType,
	batchSize int64,
) ([]string, error) {
	// Fetch 3x candidates so we have enough after filtering out fresh ones.
	candidates, err := r.rdb.ZRevRange(ctx, keyActiveUsers, 0, batchSize*3-1).Result()
	if err != nil {
		return nil, fmt.Errorf("get active users: %w", err)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// Batch-check which ones have a valid precompute meta key.
	pipe := r.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(candidates))
	for i, uid := range candidates {
		key := fmt.Sprintf(keyPrecomputeMeta, ft, uid)
		cmds[i] = pipe.Exists(ctx, key)
	}
	_, _ = pipe.Exec(ctx)

	var needsPrecompute []string
	for i, cmd := range cmds {
		if cmd.Val() == 0 {
			needsPrecompute = append(needsPrecompute, candidates[i])
		}
		if int64(len(needsPrecompute)) >= batchSize {
			break
		}
	}
	return needsPrecompute, nil
}

// ---- Nearby geo index -------------------------------------------------------

// AddVideoToGeoIndex adds a video to the Redis GEO index used for fast nearby
// queries at the cache layer. Falls back to PostGIS for precise queries.
func (r *FeedRepository) AddVideoToGeoIndex(ctx context.Context, videoID string, lat, lon float64) error {
	return r.rdb.GeoAdd(ctx, keyNearbyGeo, &redis.GeoLocation{
		Name:      videoID,
		Latitude:  lat,
		Longitude: lon,
	}).Err()
}

// ---- Internal helpers -------------------------------------------------------

// getFromSortedSet fetches a paginated slice from a Redis sorted set using
// ZREVRANGEBYSCORE in descending order. This is the core pagination primitive
// shared by all feed types backed by Redis sorted sets.
//
// The cursor encodes the last score seen; using the exclusive "(" prefix avoids
// re-emitting the boundary item across pages.
func (r *FeedRepository) getFromSortedSet(
	ctx context.Context,
	key string,
	ft models.FeedType,
	cursor *models.FeedCursor,
	limit int,
) ([]*models.FeedItem, *models.FeedCursor, error) {
	maxScore := "+inf"
	if cursor != nil {
		// Exclusive upper bound: "(score" in ZRANGEBYSCORE syntax skips the
		// cursor item itself and prevents duplicate delivery on page boundaries.
		maxScore = fmt.Sprintf("(%s", strconv.FormatFloat(cursor.Score, 'f', 10, 64))
	}

	opt := &redis.ZRangeBy{
		Min:   "-inf",
		Max:   maxScore,
		Count: int64(limit + 1), // fetch one extra to detect whether another page exists
	}
	zs, err := r.rdb.ZRevRangeByScoreWithScores(ctx, key, opt).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("ZREVRANGEBYSCORE %s: %w", key, err)
	}

	hasMore := len(zs) > limit
	if hasMore {
		zs = zs[:limit]
	}

	// Extract video IDs in score order.
	videoIDs := make([]string, len(zs))
	scoreMap := make(map[string]float64, len(zs))
	for i, z := range zs {
		id := z.Member.(string)
		videoIDs[i] = id
		scoreMap[id] = z.Score
	}

	// Fetch metadata for the video IDs (cache-first).
	items, missing := r.fetchVideoMetaFromCache(ctx, videoIDs)

	if len(missing) > 0 {
		dbItems, err := r.fetchVideoMetaFromDB(ctx, missing, "")
		if err != nil {
			r.logger.Warn("failed to fetch video meta from db",
				zap.String("feed_key", key),
				zap.Error(err),
			)
		} else {
			for _, item := range dbItems {
				items[item.VideoID] = item
				r.cacheVideoMeta(ctx, item)
			}
		}
	}

	// Reconstruct ordered slice, assigning scores and feed type.
	result := make([]*models.FeedItem, 0, len(videoIDs))
	for _, id := range videoIDs {
		item, ok := items[id]
		if !ok {
			continue // video deleted or unpublished
		}
		item.FeedScore = scoreMap[id]
		item.FeedType = ft
		result = append(result, item)
	}

	var nextCursor *models.FeedCursor
	if hasMore && len(result) > 0 {
		last := result[len(result)-1]
		nextCursor = &models.FeedCursor{
			Score:     last.FeedScore,
			VideoID:   last.VideoID,
			FeedType:  ft,
			Timestamp: time.Now(),
		}
	}

	return result, nextCursor, nil
}

// scanFeedItems reads rows from a query that returns the standard video+user
// column set (without distance) and returns FeedItem slices.
func (r *FeedRepository) scanFeedItems(rows pgx.Rows) ([]*models.FeedItem, error) {
	var items []*models.FeedItem
	for rows.Next() {
		item := &models.FeedItem{}
		var lat, lon *float64
		var isLiked, isSaved, isFollowing bool
		if err := rows.Scan(
			&item.VideoID,
			&item.Author.ID,
			&item.Author.Username,
			&item.Author.DisplayName,
			&item.Author.AvatarURL,
			&item.Author.IsVerified,
			&item.Title,
			&item.Description,
			&item.ThumbnailURL,
			&item.VideoURL,
			&item.Duration,
			&item.Stats.Views,
			&item.Stats.Likes,
			&item.Stats.Comments,
			&item.Stats.Shares,
			&item.Tags,
			&item.Category,
			&item.CreatedAt,
			&isLiked,
			&isSaved,
			&isFollowing,
			&lat,
			&lon,
		); err != nil {
			return nil, fmt.Errorf("scan feed item: %w", err)
		}
		item.IsLiked = isLiked
		item.IsSaved = isSaved
		item.Author.IsFollowing = isFollowing
		if lat != nil && lon != nil {
			item.Location = &models.GeoPoint{Latitude: *lat, Longitude: *lon}
		}
		item.FeaturedAt = time.Now()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed items rows: %w", err)
	}
	return items, nil
}

// scanFeedItemsWithDistance is like scanFeedItems but also reads a distance_km
// column, used only by the nearby feed query.
func (r *FeedRepository) scanFeedItemsWithDistance(rows pgx.Rows) ([]*models.FeedItem, error) {
	var items []*models.FeedItem
	for rows.Next() {
		item := &models.FeedItem{}
		var lat, lon, distKm float64
		if err := rows.Scan(
			&item.VideoID,
			&item.Author.ID,
			&item.Author.Username,
			&item.Author.DisplayName,
			&item.Author.AvatarURL,
			&item.Author.IsVerified,
			&item.Title,
			&item.Description,
			&item.ThumbnailURL,
			&item.VideoURL,
			&item.Duration,
			&item.Stats.Views,
			&item.Stats.Likes,
			&item.Stats.Comments,
			&item.Stats.Shares,
			&item.Tags,
			&item.Category,
			&item.CreatedAt,
			&lat,
			&lon,
			&distKm,
		); err != nil {
			return nil, fmt.Errorf("scan nearby feed item: %w", err)
		}
		item.Location = &models.GeoPoint{Latitude: lat, Longitude: lon}
		item.DistanceKm = &distKm
		item.FeaturedAt = time.Now()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nearby feed items rows: %w", err)
	}
	return items, nil
}
