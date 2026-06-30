// Package services contains the business-logic layer for the feed service.
// FeedService orchestrates feed construction by delegating to the repository
// layer (Redis / Postgres), the recommendation service (gRPC), and the social
// graph service (gRPC).
package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/tiktok-clone/feed-service/internal/models"
	"github.com/tiktok-clone/feed-service/internal/repositories"
)

// ---- Recommendation service client interface --------------------------------

// RecommendationClient is the interface the feed service uses to call the
// recommendation service. The concrete implementation uses gRPC but tests can
// supply a mock.
type RecommendationClient interface {
	// GetRecommendations returns a ranked list of video IDs for the given user.
	GetRecommendations(ctx context.Context, userID string, limit int) ([]RecommendedVideo, error)
}

// RecommendedVideo is a video ID with a recommendation score from the ML model.
type RecommendedVideo struct {
	VideoID string
	Score   float64
}

// ---- Social graph client interface ------------------------------------------

// SocialGraphClient is the interface the feed service uses to call the
// social-graph service.
type SocialGraphClient interface {
	// GetFollowing returns the list of user IDs that userID follows.
	GetFollowing(ctx context.Context, userID string) ([]string, error)
}

// ---- FeedService ------------------------------------------------------------

// FeedService is the main business-logic object for all feed operations.
type FeedService struct {
	repo        *repositories.FeedRepository
	recommend   RecommendationClient
	socialGraph SocialGraphClient
	logger      *zap.Logger

	defaultLimit int
	maxLimit     int

	// Cache TTLs.
	forYouTTL    time.Duration
	followingTTL time.Duration
	trendingTTL  time.Duration
	nearbyTTL    time.Duration
	exploreTTL   time.Duration
	dedupTTL     time.Duration

	// Geo defaults.
	nearbyDefault float64
	nearbyMax     float64
}

// FeedServiceConfig holds the parameters used to initialise FeedService.
type FeedServiceConfig struct {
	DefaultLimit    int
	MaxLimit        int
	ForYouTTL       time.Duration
	FollowingTTL    time.Duration
	TrendingTTL     time.Duration
	NearbyTTL       time.Duration
	ExploreTTL      time.Duration
	DedupTTL        time.Duration
	NearbyDefaultKm float64
	NearbyMaxKm     float64
}

// NewFeedService constructs a FeedService with all dependencies injected.
func NewFeedService(
	repo *repositories.FeedRepository,
	recommend RecommendationClient,
	socialGraph SocialGraphClient,
	cfg FeedServiceConfig,
	logger *zap.Logger,
) *FeedService {
	if cfg.DefaultLimit <= 0 {
		cfg.DefaultLimit = 20
	}
	if cfg.MaxLimit <= 0 {
		cfg.MaxLimit = 50
	}
	if cfg.ForYouTTL == 0 {
		cfg.ForYouTTL = 10 * time.Minute
	}
	if cfg.FollowingTTL == 0 {
		cfg.FollowingTTL = 5 * time.Minute
	}
	if cfg.TrendingTTL == 0 {
		cfg.TrendingTTL = 15 * time.Minute
	}
	if cfg.NearbyTTL == 0 {
		cfg.NearbyTTL = 5 * time.Minute
	}
	if cfg.ExploreTTL == 0 {
		cfg.ExploreTTL = 10 * time.Minute
	}
	if cfg.DedupTTL == 0 {
		cfg.DedupTTL = 24 * time.Hour
	}
	if cfg.NearbyDefaultKm == 0 {
		cfg.NearbyDefaultKm = 10.0
	}
	if cfg.NearbyMaxKm == 0 {
		cfg.NearbyMaxKm = 100.0
	}
	return &FeedService{
		repo:          repo,
		recommend:     recommend,
		socialGraph:   socialGraph,
		logger:        logger,
		defaultLimit:  cfg.DefaultLimit,
		maxLimit:      cfg.MaxLimit,
		forYouTTL:     cfg.ForYouTTL,
		followingTTL:  cfg.FollowingTTL,
		trendingTTL:   cfg.TrendingTTL,
		nearbyTTL:     cfg.NearbyTTL,
		exploreTTL:    cfg.ExploreTTL,
		dedupTTL:      cfg.DedupTTL,
		nearbyDefault: cfg.NearbyDefaultKm,
		nearbyMax:     cfg.NearbyMaxKm,
	}
}

// ---- GetForYouFeed ----------------------------------------------------------

// GetForYouFeed returns the personalised For-You feed for a user.
//
// Strategy:
//  1. If a pre-computed feed exists in Redis, serve from there (cache hit).
//  2. Otherwise call the recommendation service, persist the result to Redis,
//     then serve the first page (cache miss / cold start).
//
// Deduplication: videos already seen in the session are filtered out before
// returning; the returned IDs are then marked as seen via Redis SADD.
func (s *FeedService) GetForYouFeed(ctx context.Context, req *models.FeedRequest) (*models.FeedPage, error) {
	limit := s.clampLimit(req.Limit)

	cursor, err := models.DecodeFeedCursor(req.Cursor)
	if err != nil {
		return nil, fmt.Errorf("for-you feed: invalid cursor: %w", err)
	}
	if cursor != nil && cursor.FeedType != models.FeedTypeForYou {
		return nil, fmt.Errorf("for-you feed: cursor mismatch (got %q)", cursor.FeedType)
	}

	hasCache, _ := s.repo.HasPrecomputedFeed(ctx, req.UserID, models.FeedTypeForYou)
	if !hasCache {
		if err := s.buildForYouFeedCache(ctx, req.UserID); err != nil {
			s.logger.Warn("failed to build for-you feed cache; falling back to trending",
				zap.String("user_id", req.UserID),
				zap.Error(err),
			)
		}
	}

	// Track user activity for precompute prioritisation (fire-and-forget).
	go func() {
		trackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.repo.TrackActiveUser(trackCtx, req.UserID)
	}()

	// Fetch extra items to allow for deduplication shrinkage.
	items, nextCursor, err := s.repo.GetForYouFeed(ctx, req.UserID, cursor, limit+20)
	if err != nil {
		return nil, fmt.Errorf("for-you feed: %w", err)
	}

	items, err = s.deduplicateAndMark(ctx, req, items, limit)
	if err != nil {
		s.logger.Warn("deduplication failed", zap.Error(err))
	}

	return s.buildPage(items, nextCursor, limit, models.FeedTypeForYou), nil
}

// buildForYouFeedCache calls the recommendation service and persists the result
// to Redis so subsequent requests are served from cache.
func (s *FeedService) buildForYouFeedCache(ctx context.Context, userID string) error {
	recs, err := s.recommend.GetRecommendations(ctx, userID, 200)
	if err != nil {
		return fmt.Errorf("recommendation service: %w", err)
	}
	if len(recs) == 0 {
		// Fall back to trending videos when the model has no data for the user.
		return s.backfillWithTrending(ctx, userID)
	}

	videoIDs := make([]string, len(recs))
	scoreMap := make(map[string]float64, len(recs))
	for i, rec := range recs {
		videoIDs[i] = rec.VideoID
		scoreMap[rec.VideoID] = rec.Score
	}

	fetchedItems, err := s.repo.GetVideosByIDs(ctx, videoIDs, userID)
	if err != nil {
		return fmt.Errorf("fetch video meta: %w", err)
	}

	items := make([]*models.FeedItem, 0, len(fetchedItems))
	for _, item := range fetchedItems {
		item.FeedScore = scoreMap[item.VideoID]
		items = append(items, item)
	}

	return s.repo.PrecomputeFeed(ctx, userID, models.FeedTypeForYou, items, s.forYouTTL)
}

// backfillWithTrending populates a user's For-You cache with trending videos
// when the recommendation model has insufficient signal (e.g. new users).
func (s *FeedService) backfillWithTrending(ctx context.Context, userID string) error {
	// Use a very large score to retrieve all trending items (no upper bound).
	zs, err := s.repo.GetTrendingVideoIDs(ctx, math.MaxFloat64, 200)
	if err != nil {
		return err
	}
	videoIDs := make([]string, len(zs))
	scoreMap := make(map[string]float64, len(zs))
	for i, z := range zs {
		id := z.Member.(string)
		videoIDs[i] = id
		scoreMap[id] = z.Score
	}
	fetchedItems, err := s.repo.GetVideosByIDs(ctx, videoIDs, userID)
	if err != nil {
		return err
	}
	for _, item := range fetchedItems {
		item.FeedScore = scoreMap[item.VideoID]
	}
	return s.repo.PrecomputeFeed(ctx, userID, models.FeedTypeForYou, fetchedItems, s.forYouTTL)
}

// ---- GetFollowingFeed -------------------------------------------------------

// GetFollowingFeed returns the chronologically ordered feed of videos from
// accounts that the requesting user follows.
//
// Strategy:
//  1. Check Redis for a cached following feed sorted set.
//  2. On cache miss, query Postgres for recent videos from followed accounts
//     (via the social-graph join), store them in a sorted set scored by
//     Unix timestamp, and serve the first page.
func (s *FeedService) GetFollowingFeed(ctx context.Context, req *models.FeedRequest) (*models.FeedPage, error) {
	limit := s.clampLimit(req.Limit)

	cursor, err := models.DecodeFeedCursor(req.Cursor)
	if err != nil {
		return nil, fmt.Errorf("following feed: invalid cursor: %w", err)
	}
	if cursor != nil && cursor.FeedType != models.FeedTypeFollowing {
		return nil, fmt.Errorf("following feed: cursor mismatch (got %q)", cursor.FeedType)
	}

	hasCache, _ := s.repo.HasPrecomputedFeed(ctx, req.UserID, models.FeedTypeFollowing)
	if !hasCache {
		if err := s.buildFollowingFeedCache(ctx, req.UserID); err != nil {
			s.logger.Warn("failed to build following feed cache",
				zap.String("user_id", req.UserID),
				zap.Error(err),
			)
		}
	}

	items, nextCursor, err := s.repo.GetFollowingFeed(ctx, req.UserID, cursor, limit+20)
	if err != nil {
		return nil, fmt.Errorf("following feed: %w", err)
	}

	items, err = s.deduplicateAndMark(ctx, req, items, limit)
	if err != nil {
		s.logger.Warn("deduplication failed", zap.Error(err))
	}

	return s.buildPage(items, nextCursor, limit, models.FeedTypeFollowing), nil
}

// buildFollowingFeedCache queries Postgres for recent videos from followed
// accounts and stores them in Redis as a score=UnixTimestamp sorted set.
func (s *FeedService) buildFollowingFeedCache(ctx context.Context, userID string) error {
	since := time.Now().Add(-72 * time.Hour) // look back 3 days
	items, err := s.repo.GetFollowingVideoIDs(ctx, userID, since, 500)
	if err != nil {
		return err
	}
	// Score by publication time (Unix seconds) so cursor pagination is stable
	// and the feed is naturally chronological.
	for _, item := range items {
		item.FeedScore = float64(item.CreatedAt.Unix())
	}
	return s.repo.PrecomputeFeed(ctx, userID, models.FeedTypeFollowing, items, s.followingTTL)
}

// ---- GetTrendingFeed --------------------------------------------------------

// GetTrendingFeed returns the globally trending videos. The trending sorted set
// is maintained by the TrendingService background worker and is pre-populated
// in Redis; this method just paginates over it.
func (s *FeedService) GetTrendingFeed(ctx context.Context, req *models.FeedRequest) (*models.FeedPage, error) {
	limit := s.clampLimit(req.Limit)

	cursor, err := models.DecodeFeedCursor(req.Cursor)
	if err != nil {
		return nil, fmt.Errorf("trending feed: invalid cursor: %w", err)
	}
	if cursor != nil && cursor.FeedType != models.FeedTypeTrending {
		return nil, fmt.Errorf("trending feed: cursor mismatch (got %q)", cursor.FeedType)
	}

	items, nextCursor, err := s.repo.GetTrendingFeed(ctx, cursor, limit+20)
	if err != nil {
		return nil, fmt.Errorf("trending feed: %w", err)
	}

	items, err = s.deduplicateAndMark(ctx, req, items, limit)
	if err != nil {
		s.logger.Warn("deduplication failed", zap.Error(err))
	}

	return s.buildPage(items, nextCursor, limit, models.FeedTypeTrending), nil
}

// ---- GetNearbyFeed ----------------------------------------------------------

// GetNearbyFeed returns videos uploaded near the user's current location.
// Coordinates are supplied per-request; a default radius of NearbyDefaultKm is
// used when the caller omits it. Queries PostGIS for accurate geo-distance
// ordering.
func (s *FeedService) GetNearbyFeed(ctx context.Context, req *models.FeedRequest) (*models.FeedPage, error) {
	limit := s.clampLimit(req.Limit)

	if req.RadiusKm <= 0 {
		req.RadiusKm = s.nearbyDefault
	}
	if req.RadiusKm > s.nearbyMax {
		req.RadiusKm = s.nearbyMax
	}

	cursor, err := models.DecodeFeedCursor(req.Cursor)
	if err != nil {
		return nil, fmt.Errorf("nearby feed: invalid cursor: %w", err)
	}
	if cursor != nil && cursor.FeedType != models.FeedTypeNearby {
		return nil, fmt.Errorf("nearby feed: cursor mismatch (got %q)", cursor.FeedType)
	}

	items, nextCursor, err := s.repo.GetNearbyFeed(ctx,
		req.UserID, req.Latitude, req.Longitude, req.RadiusKm,
		cursor, limit+20,
	)
	if err != nil {
		return nil, fmt.Errorf("nearby feed: %w", err)
	}

	items, err = s.deduplicateAndMark(ctx, req, items, limit)
	if err != nil {
		s.logger.Warn("deduplication failed", zap.Error(err))
	}

	return s.buildPage(items, nextCursor, limit, models.FeedTypeNearby), nil
}

// ---- GetExploreFeed ---------------------------------------------------------

// GetExploreFeed returns the category-based discovery feed. Content is sourced
// from the explore sorted set which is updated by the precompute worker.
// When req.Category is set, a category-specific sorted set is used instead.
func (s *FeedService) GetExploreFeed(ctx context.Context, req *models.FeedRequest) (*models.FeedPage, error) {
	limit := s.clampLimit(req.Limit)

	cursor, err := models.DecodeFeedCursor(req.Cursor)
	if err != nil {
		return nil, fmt.Errorf("explore feed: invalid cursor: %w", err)
	}
	if cursor != nil &&
		cursor.FeedType != models.FeedTypeExplore &&
		cursor.FeedType != models.FeedTypeCategory {
		return nil, fmt.Errorf("explore feed: cursor mismatch (got %q)", cursor.FeedType)
	}

	var (
		items      []*models.FeedItem
		nextCursor *models.FeedCursor
	)

	if req.Category != "" {
		items, nextCursor, err = s.repo.GetCategoryFeed(ctx, req.Category, cursor, limit+20)
	} else {
		items, nextCursor, err = s.repo.GetExploreFeed(ctx, cursor, limit+20)
	}
	if err != nil {
		return nil, fmt.Errorf("explore feed: %w", err)
	}

	items, err = s.deduplicateAndMark(ctx, req, items, limit)
	if err != nil {
		s.logger.Warn("deduplication failed", zap.Error(err))
	}

	ft := models.FeedTypeExplore
	if req.Category != "" {
		ft = models.FeedTypeCategory
	}

	return s.buildPage(items, nextCursor, limit, ft), nil
}

// ---- Feed invalidation ------------------------------------------------------

// InvalidateUserFeed invalidates all feed caches for a user (called e.g. when
// new content from a followed creator is published).
func (s *FeedService) InvalidateUserFeed(ctx context.Context, userID string, ft models.FeedType) error {
	return s.repo.InvalidateFeed(ctx, userID, ft)
}

// ---- Cross-feed deduplication -----------------------------------------------

// deduplicateAndMark filters out videos that the user has already seen in their
// current session and marks the remaining ones as seen. It returns at most
// `limit` items from the input slice.
//
// The seen-set is persisted in Redis using SADD, keyed by userID+sessionID.
// If the sessionID is empty (unauthenticated user), deduplication is skipped.
func (s *FeedService) deduplicateAndMark(
	ctx context.Context,
	req *models.FeedRequest,
	items []*models.FeedItem,
	limit int,
) ([]*models.FeedItem, error) {
	if req.SessionID == "" || len(items) == 0 {
		if len(items) > limit {
			return items[:limit], nil
		}
		return items, nil
	}

	videoIDs := make([]string, len(items))
	for i, item := range items {
		videoIDs[i] = item.VideoID
	}

	unseen, err := s.repo.FilterSeenVideos(ctx, req.UserID, req.SessionID, videoIDs)
	if err != nil {
		// Non-fatal: return unfiltered items on dedup failure.
		if len(items) > limit {
			return items[:limit], err
		}
		return items, err
	}

	// Build a set of unseen IDs for O(1) lookup.
	unseenSet := make(map[string]struct{}, len(unseen))
	for _, id := range unseen {
		unseenSet[id] = struct{}{}
	}

	// Filter items to only unseen, up to limit.
	filtered := make([]*models.FeedItem, 0, limit)
	for _, item := range items {
		if _, ok := unseenSet[item.VideoID]; ok {
			filtered = append(filtered, item)
			if len(filtered) == limit {
				break
			}
		}
	}

	// Mark the returned items as seen so they won't appear again this session.
	if len(filtered) > 0 {
		returnedIDs := make([]string, len(filtered))
		for i, item := range filtered {
			returnedIDs[i] = item.VideoID
		}
		if _, markErr := s.repo.MarkVideosAsSeen(
			ctx, req.UserID, req.SessionID, returnedIDs, s.dedupTTL,
		); markErr != nil {
			s.logger.Warn("failed to mark videos as seen",
				zap.String("user_id", req.UserID),
				zap.Error(markErr),
			)
		}
	}

	return filtered, nil
}

// ---- Helpers ----------------------------------------------------------------

// clampLimit returns a page size within [1, maxLimit].
func (s *FeedService) clampLimit(requested int) int {
	if requested <= 0 {
		return s.defaultLimit
	}
	if requested > s.maxLimit {
		return s.maxLimit
	}
	return requested
}

// buildPage assembles a FeedPage from the items returned by a repository call.
func (s *FeedService) buildPage(
	items []*models.FeedItem,
	nextCursor *models.FeedCursor,
	limit int,
	ft models.FeedType,
) *models.FeedPage {
	// Ensure we never return more than limit items.
	if len(items) > limit {
		items = items[:limit]
	}
	page := &models.FeedPage{
		Items:       items,
		HasMore:     nextCursor != nil,
		Count:       len(items),
		FeedType:    ft,
		GeneratedAt: time.Now(),
	}
	if nextCursor != nil {
		encoded, err := nextCursor.Encode()
		if err == nil {
			page.NextCursor = encoded
		}
	}
	return page
}

// ---- Feed precompute (called by worker) -------------------------------------

// PrecomputeForYouFeed rebuilds the For-You feed cache for a specific user.
// This is called by the FeedPrecomputeWorker for active users.
func (s *FeedService) PrecomputeForYouFeed(ctx context.Context, userID string) error {
	return s.buildForYouFeedCache(ctx, userID)
}

// PrecomputeFollowingFeed rebuilds the following feed cache for a specific user.
func (s *FeedService) PrecomputeFollowingFeed(ctx context.Context, userID string) error {
	return s.buildFollowingFeedCache(ctx, userID)
}
