package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/recommendation-service/internal/config"
	"github.com/tiktok-clone/recommendation-service/internal/models"
)

// RankingService executes the multi-stage ranking pipeline:
//
//  1. Feature hydration – attach VideoFeatures to each candidate.
//  2. Seen-video filter – drop candidates already watched.
//  3. Coarse ranking – fast linear model; retain top-CoarseRankSize items.
//  4. Fine ranking – feature-weighted model over coarse survivors.
//  5. Diversity injection – enforce max-consecutive-same-creator rule.
//  6. Trim to FinalFeedSize.
type RankingService struct {
	cfg          *config.Config
	featureStore *FeatureStore
	embedding    *EmbeddingService
	rdb          redis.UniversalClient
	logger       *zap.Logger
}

// NewRankingService constructs a RankingService.
func NewRankingService(
	cfg *config.Config,
	featureStore *FeatureStore,
	embedding *EmbeddingService,
	rdb redis.UniversalClient,
	logger *zap.Logger,
) *RankingService {
	return &RankingService{
		cfg:          cfg,
		featureStore: featureStore,
		embedding:    embedding,
		rdb:          rdb,
		logger:       logger,
	}
}

// Rank takes a raw candidate pool and the requesting user's features, and
// returns the final ranked feed.
func (r *RankingService) Rank(
	ctx context.Context,
	candidates []*models.CandidateVideo,
	user *models.UserFeatures,
	experimentID string,
) ([]*models.RankedResult, error) {

	if len(candidates) == 0 {
		return nil, nil
	}

	// ---- Stage 1: hydrate video features --------------------------------
	if err := r.hydrateFeatures(ctx, candidates); err != nil {
		// Log but do not abort; we can rank with partial features.
		r.logger.Warn("feature hydration partially failed", zap.Error(err))
	}

	// ---- Stage 2: filter already-seen videos ----------------------------
	candidates, err := r.filterSeen(ctx, user.UserID, candidates)
	if err != nil {
		r.logger.Warn("filter-seen failed, proceeding without filter", zap.Error(err))
	}

	// ---- Stage 3: coarse ranking ----------------------------------------
	coarseRanked := r.coarseRank(candidates, user)

	limit := r.cfg.Recommendation.CoarseRankSize
	if len(coarseRanked) > limit {
		coarseRanked = coarseRanked[:limit]
	}

	// ---- Stage 4: fine ranking ------------------------------------------
	fineRanked := r.fineRank(coarseRanked, user)

	// ---- Stage 5: diversity injection -----------------------------------
	diversified := r.injectDiversity(fineRanked)

	// ---- Stage 6: trim to FinalFeedSize ---------------------------------
	size := r.cfg.Recommendation.FinalFeedSize
	if len(diversified) > size {
		diversified = diversified[:size]
	}

	// Assign positions and attach experiment ID.
	for i, item := range diversified {
		item.Position = i
		item.ExperimentID = experimentID
	}

	return diversified, nil
}

// -----------------------------------------------------------------
// Stage 1 – Feature hydration
// -----------------------------------------------------------------

func (r *RankingService) hydrateFeatures(
	ctx context.Context,
	candidates []*models.CandidateVideo,
) error {
	// Collect IDs that still need features.
	needed := make([]string, 0, len(candidates))
	indexByID := make(map[string][]*models.CandidateVideo, len(candidates))
	for _, c := range candidates {
		if c.Features == nil {
			needed = append(needed, c.VideoID)
			indexByID[c.VideoID] = append(indexByID[c.VideoID], c)
		}
	}
	if len(needed) == 0 {
		return nil
	}

	featureMap, err := r.featureStore.GetVideoFeaturesBatch(ctx, needed)
	if err != nil {
		return fmt.Errorf("batch video feature fetch: %w", err)
	}

	for id, features := range featureMap {
		for _, c := range indexByID[id] {
			c.Features = features
			c.CreatorID = features.CreatorID
		}
	}
	return nil
}

// -----------------------------------------------------------------
// Stage 2 – Filter already-seen videos
// -----------------------------------------------------------------

func (r *RankingService) filterSeen(
	ctx context.Context,
	userID string,
	candidates []*models.CandidateVideo,
) ([]*models.CandidateVideo, error) {
	seenKey := userSeenKey(userID)

	// Use a pipelined SISMEMBER batch.
	pipe := r.rdb.Pipeline()
	cmds := make([]*redis.BoolCmd, len(candidates))
	for i, c := range candidates {
		cmds[i] = pipe.SIsMember(ctx, seenKey, c.VideoID)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return candidates, fmt.Errorf("pipeline SIsMember: %w", err)
	}

	filtered := candidates[:0]
	for i, cmd := range cmds {
		seen, err := cmd.Result()
		if err != nil {
			// On error, include the candidate (fail open).
			filtered = append(filtered, candidates[i])
			continue
		}
		if !seen {
			filtered = append(filtered, candidates[i])
		}
	}
	return filtered, nil
}

// MarkSeen adds a set of video IDs to the user's seen-set with a rolling TTL.
func (r *RankingService) MarkSeen(ctx context.Context, userID string, videoIDs []string) error {
	seenKey := userSeenKey(userID)
	ttl := r.cfg.Recommendation.SeenVideoTTL

	pipe := r.rdb.Pipeline()
	members := make([]interface{}, len(videoIDs))
	for i, id := range videoIDs {
		members[i] = id
	}
	pipe.SAdd(ctx, seenKey, members...)
	pipe.Expire(ctx, seenKey, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// -----------------------------------------------------------------
// Stage 3 – Coarse ranking
// -----------------------------------------------------------------
// score = engagement_rate * 0.5 + freshness * 0.3 + relevance * 0.2

func (r *RankingService) coarseRank(
	candidates []*models.CandidateVideo,
	user *models.UserFeatures,
) []*models.RankedResult {

	cfg := r.cfg.Recommendation
	wE := cfg.CoarseWeightEngagement
	wF := cfg.CoarseWeightFreshness
	wR := cfg.CoarseWeightRelevance

	results := make([]*models.RankedResult, 0, len(candidates))

	for _, c := range candidates {
		engScore := r.computeEngagementScore(c.Features)
		freshScore := r.computeFreshnessScore(c.Features)
		relScore := r.computeRelevanceScore(c, user)

		coarse := wE*engScore + wF*freshScore + wR*relScore

		results = append(results, &models.RankedResult{
			VideoID:         c.VideoID,
			CreatorID:       c.CreatorID,
			CoarseScore:     coarse,
			EngagementScore: engScore,
			FreshnessScore:  freshScore,
			RelevanceScore:  relScore,
			Source:          c.Source,
			VideoFeatures:   c.Features,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CoarseScore > results[j].CoarseScore
	})
	return results
}

// computeEngagementScore converts a video's engagement rate and counts into a
// normalised score in [0, 1].  We use a softmax-like squash to prevent
// viral outliers from dominating.
func (r *RankingService) computeEngagementScore(f *models.VideoFeatures) float64 {
	if f == nil {
		return 0
	}
	// Weighted engagement rate is already clamped to [0,1] in VideoFeatures.
	base := f.EngagementRate
	// Add a Bayesian prior of 0.05 (global average) to smooth low-view items.
	prior := 0.05
	priorWeight := 100.0
	smoothed := (base*float64(f.ViewCount) + prior*priorWeight) /
		(float64(f.ViewCount) + priorWeight)
	// Squash using square root to compress dynamic range.
	return math.Sqrt(smoothed)
}

// computeFreshnessScore returns an exponentially decaying freshness in [0, 1].
func (r *RankingService) computeFreshnessScore(f *models.VideoFeatures) float64 {
	if f == nil {
		return 0
	}
	age := time.Since(f.PublishedAt).Hours()
	if age < 0 {
		age = 0
	}
	halfLife := r.cfg.Recommendation.FreshnessHalfLifeHours
	return math.Exp(-0.693 * age / halfLife)
}

// computeRelevanceScore measures how well the video matches the user via
// embedding cosine similarity plus category affinity.
func (r *RankingService) computeRelevanceScore(
	c *models.CandidateVideo,
	user *models.UserFeatures,
) float64 {
	if c.Features == nil || len(c.Features.Embedding) == 0 || len(user.Embedding) == 0 {
		// Fall back to category affinity alone.
		return r.categoryAffinity(c.Features, user)
	}
	cos := CosineSimilarity(user.Embedding, c.Features.Embedding)
	// Rescale cosine from [-1, 1] to [0, 1].
	cosSc := (cos + 1) / 2

	catSc := r.categoryAffinity(c.Features, user)

	// Blend: 70% embedding similarity, 30% category affinity.
	return 0.7*cosSc + 0.3*catSc
}

func (r *RankingService) categoryAffinity(f *models.VideoFeatures, user *models.UserFeatures) float64 {
	if f == nil {
		return 0
	}
	score, ok := user.LikedCategories[f.Category]
	if !ok {
		return 0
	}
	return score
}

// -----------------------------------------------------------------
// Stage 4 – Fine ranking
// -----------------------------------------------------------------
// The fine ranker applies a richer feature-weighted model on top of coarse
// scores, incorporating user-video interaction signals such as creator
// affinity and language match.

// fineRankWeights defines the multiplicative factors applied to individual
// signals in the fine-ranking formula.  These would typically be learned
// offline (e.g., gradient boosted tree or factorisation machine) but are
// expressed here as explicit weights for transparency and controllability.
type fineRankWeights struct {
	CoarseScore        float64
	CreatorAffinity    float64
	LanguageMatch      float64
	DeviceBoost        float64
	TrendingBoost      float64
	WatchCompletionEst float64
}

var defaultFineWeights = fineRankWeights{
	CoarseScore:        0.45,
	CreatorAffinity:    0.20,
	LanguageMatch:      0.10,
	DeviceBoost:        0.05,
	TrendingBoost:      0.10,
	WatchCompletionEst: 0.10,
}

func (r *RankingService) fineRank(
	items []*models.RankedResult,
	user *models.UserFeatures,
) []*models.RankedResult {

	w := defaultFineWeights

	for _, item := range items {
		var fine float64
		fine += w.CoarseScore * item.CoarseScore
		fine += w.CreatorAffinity * r.creatorAffinity(item.CreatorID, user)
		fine += w.LanguageMatch * r.languageMatchScore(item.VideoFeatures, user)
		fine += w.DeviceBoost * r.deviceBoostScore(item.VideoFeatures, user)
		fine += w.TrendingBoost * r.trendingBoostScore(item.VideoFeatures)
		fine += w.WatchCompletionEst * r.watchCompletionEstimate(item.VideoFeatures, user)
		item.FinalScore = fine
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].FinalScore > items[j].FinalScore
	})
	return items
}

// creatorAffinity returns a normalised score representing how much the user
// interacts with a specific creator.
func (r *RankingService) creatorAffinity(creatorID string, user *models.UserFeatures) float64 {
	for _, id := range user.FollowedCreators {
		if id == creatorID {
			return 1.0
		}
	}
	return 0.0
}

// languageMatchScore returns 1 if the video language matches the user's
// preferred language, else 0.2 (penalise but don't eliminate).
func (r *RankingService) languageMatchScore(f *models.VideoFeatures, user *models.UserFeatures) float64 {
	if f == nil || f.LanguageCode == "" || user.LanguageCode == "" {
		return 0.5 // neutral when unknown
	}
	if f.LanguageCode == user.LanguageCode {
		return 1.0
	}
	return 0.2
}

// deviceBoostScore applies a boost for short videos on mobile (portrait UX)
// and a slight penalty for very long videos on small screens.
func (r *RankingService) deviceBoostScore(f *models.VideoFeatures, user *models.UserFeatures) float64 {
	if f == nil {
		return 0.5
	}
	isMobile := user.DeviceType == models.DeviceMobile || user.DeviceType == models.DeviceUnknown
	dur := f.Duration
	switch {
	case isMobile && dur <= 60:
		return 1.0
	case isMobile && dur <= 180:
		return 0.7
	case isMobile:
		return 0.4
	case dur <= 600:
		return 0.8
	default:
		return 0.6
	}
}

// trendingBoostScore blends the pre-computed trending score from the analytics
// pipeline into the fine-rank.
func (r *RankingService) trendingBoostScore(f *models.VideoFeatures) float64 {
	if f == nil {
		return 0
	}
	// TrendingScore is already normalised to [0, 1] by the analytics pipeline.
	return f.TrendingScore
}

// watchCompletionEstimate predicts whether the user is likely to watch the
// video to completion, based on duration vs the user's historical device type.
// A full ML model would live here in production; this is a heuristic proxy.
func (r *RankingService) watchCompletionEstimate(f *models.VideoFeatures, user *models.UserFeatures) float64 {
	if f == nil {
		return 0.5
	}
	// Empirical observation: mobile users have ~70% completion for ≤30s videos,
	// dropping by ~15% per additional minute.
	dur := f.Duration
	base := 0.70
	if user.DeviceType == models.DeviceDesktop || user.DeviceType == models.DeviceTV {
		base = 0.60
	}
	minDecrease := (dur - 30) / 60 * 0.15
	if minDecrease < 0 {
		minDecrease = 0
	}
	est := base - minDecrease
	if est < 0.05 {
		est = 0.05
	}
	if est > 1 {
		est = 1
	}
	return est
}

// -----------------------------------------------------------------
// Stage 5 – Diversity injection
// -----------------------------------------------------------------
// Reorders the ranked list so that no more than MaxConsecutiveSameCreator
// videos from the same creator appear consecutively.

func (r *RankingService) injectDiversity(items []*models.RankedResult) []*models.RankedResult {
	maxRun := r.cfg.Recommendation.MaxConsecutiveSameCreator
	if maxRun <= 0 {
		return items
	}

	// Bucket items by creator.
	byCreator := make(map[string][]*models.RankedResult, len(items))
	for _, item := range items {
		byCreator[item.CreatorID] = append(byCreator[item.CreatorID], item)
	}

	// Priority queue (max heap on FinalScore) for round-robin selection.
	// We use a simple slice-based heap.
	type bucket struct {
		creatorID string
		items     []*models.RankedResult
	}

	buckets := make([]bucket, 0, len(byCreator))
	for creatorID, vids := range byCreator {
		// Items within each creator bucket are already sorted descending by FinalScore.
		buckets = append(buckets, bucket{creatorID, vids})
	}

	result := make([]*models.RankedResult, 0, len(items))
	consecutiveCount := 0
	lastCreator := ""

	for len(result) < len(items) {
		// Find the highest-scoring item that doesn't violate the run constraint.
		bestIdx := -1
		var bestScore float64

		for i, b := range buckets {
			if len(b.items) == 0 {
				continue
			}
			top := b.items[0]
			// If we've hit the run limit, skip items from the same creator.
			if top.CreatorID == lastCreator && consecutiveCount >= maxRun {
				continue
			}
			if bestIdx == -1 || top.FinalScore > bestScore {
				bestIdx = i
				bestScore = top.FinalScore
			}
		}

		if bestIdx == -1 {
			// All remaining items are from the same creator; reset constraint and
			// include the next best regardless.
			consecutiveCount = 0
			lastCreator = ""
			// Find absolute best.
			for i, b := range buckets {
				if len(b.items) == 0 {
					continue
				}
				if bestIdx == -1 || b.items[0].FinalScore > bestScore {
					bestIdx = i
					bestScore = b.items[0].FinalScore
				}
			}
			if bestIdx == -1 {
				break
			}
		}

		chosen := buckets[bestIdx].items[0]
		buckets[bestIdx].items = buckets[bestIdx].items[1:]

		if chosen.CreatorID == lastCreator {
			consecutiveCount++
		} else {
			consecutiveCount = 1
			lastCreator = chosen.CreatorID
		}
		result = append(result, chosen)
	}

	return result
}

// -----------------------------------------------------------------
// Redis key helpers
// -----------------------------------------------------------------

func userSeenKey(userID string) string {
	return fmt.Sprintf("rec:seen:%s", userID)
}

// -----------------------------------------------------------------
// Batch feature enrichment (used externally by the handler)
// -----------------------------------------------------------------

// EnrichWithFeatures populates VideoFeatures on a slice of RankedResults in
// parallel.  It is called after ranking when the handler needs full metadata
// for the API response.
func (r *RankingService) EnrichWithFeatures(
	ctx context.Context,
	items []*models.RankedResult,
) error {
	needed := make([]string, 0, len(items))
	byID := make(map[string][]*models.RankedResult, len(items))
	for _, item := range items {
		if item.VideoFeatures == nil {
			needed = append(needed, item.VideoID)
			byID[item.VideoID] = append(byID[item.VideoID], item)
		}
	}
	if len(needed) == 0 {
		return nil
	}
	featureMap, err := r.featureStore.GetVideoFeaturesBatch(ctx, needed)
	if err != nil {
		return err
	}
	for id, f := range featureMap {
		for _, item := range byID[id] {
			item.VideoFeatures = f
		}
	}
	return nil
}

// -----------------------------------------------------------------
// Concurrent video feature fetcher
// -----------------------------------------------------------------

// batchFetchVideoFeatures fetches video features for a list of IDs, returning
// only the successfully retrieved ones.  Errors are logged but not propagated
// so partial results are always returned.
func (r *RankingService) batchFetchVideoFeatures(
	ctx context.Context,
	videoIDs []string,
) map[string]*models.VideoFeatures {
	var (
		mu  sync.Mutex
		out = make(map[string]*models.VideoFeatures, len(videoIDs))
		wg  sync.WaitGroup
	)

	// Chunk the IDs to avoid creating too many goroutines.
	const chunkSize = 50
	for i := 0; i < len(videoIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(videoIDs) {
			end = len(videoIDs)
		}
		chunk := videoIDs[i:end]
		wg.Add(1)
		go func(ids []string) {
			defer wg.Done()
			m, err := r.featureStore.GetVideoFeaturesBatch(ctx, ids)
			if err != nil {
				r.logger.Warn("batch feature fetch chunk failed", zap.Error(err))
				return
			}
			mu.Lock()
			for k, v := range m {
				out[k] = v
			}
			mu.Unlock()
		}(chunk)
	}
	wg.Wait()
	return out
}
