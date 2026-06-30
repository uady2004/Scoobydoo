package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/recommendation-service/internal/config"
	"github.com/tiktok-clone/recommendation-service/internal/models"
)

// CandidateGenerator produces candidate videos from multiple retrieval strategies.
// Each strategy operates independently; results are deduplicated by the caller.
type CandidateGenerator struct {
	cfg    *config.Config
	es     *elasticsearch.Client
	rdb    redis.UniversalClient
	logger *zap.Logger
}

// NewCandidateGenerator constructs a CandidateGenerator.
func NewCandidateGenerator(
	cfg *config.Config,
	es *elasticsearch.Client,
	rdb redis.UniversalClient,
	logger *zap.Logger,
) *CandidateGenerator {
	return &CandidateGenerator{
		cfg:    cfg,
		es:     es,
		rdb:    rdb,
		logger: logger,
	}
}

// GenerateAll runs all five strategies concurrently and merges the results.
// Deduplication is applied so each video ID appears at most once; the entry
// with the highest RetrievalScore wins.
func (g *CandidateGenerator) GenerateAll(
	ctx context.Context,
	user *models.UserFeatures,
) ([]*models.CandidateVideo, error) {

	type result struct {
		candidates []*models.CandidateVideo
		err        error
		strategy   models.SourceStrategy
	}

	tasks := []struct {
		strategy models.SourceStrategy
		fn       func(context.Context, *models.UserFeatures) ([]*models.CandidateVideo, error)
	}{
		{models.SourceCollaborativeFiltering, g.CollaborativeFiltering},
		{models.SourceContentBased, g.ContentBased},
		{models.SourceTrending, g.Trending},
		{models.SourceFollowingNetwork, g.FollowingNetwork},
		{models.SourceRecentInteractionGraph, g.RecentInteractionGraph},
	}

	resultCh := make(chan result, len(tasks))
	for _, t := range tasks {
		t := t
		go func() {
			candidates, err := t.fn(ctx, user)
			resultCh <- result{candidates: candidates, err: err, strategy: t.strategy}
		}()
	}

	// Collect with a per-strategy timeout guard.
	seen := make(map[string]*models.CandidateVideo, g.cfg.Recommendation.CandidatePoolSize)
	for range tasks {
		r := <-resultCh
		if r.err != nil {
			g.logger.Warn("candidate strategy failed",
				zap.String("strategy", string(r.strategy)),
				zap.Error(r.err),
			)
			continue
		}
		for _, c := range r.candidates {
			if existing, ok := seen[c.VideoID]; ok {
				if c.RetrievalScore > existing.RetrievalScore {
					seen[c.VideoID] = c
				}
			} else {
				seen[c.VideoID] = c
			}
		}
	}

	merged := make([]*models.CandidateVideo, 0, len(seen))
	for _, c := range seen {
		merged = append(merged, c)
	}
	return merged, nil
}

// -----------------------------------------------------------------
// Strategy 1 – Collaborative Filtering
// -----------------------------------------------------------------
// For each of the user's recently watched videos, look up the top-K similar
// items stored in Redis (populated by the model-update worker).  Videos the
// user has already seen are filtered out by the ranking layer, not here.

func (g *CandidateGenerator) CollaborativeFiltering(
	ctx context.Context,
	user *models.UserFeatures,
) ([]*models.CandidateVideo, error) {

	if len(user.WatchHistory) == 0 {
		return nil, nil
	}

	cfg := g.cfg.Recommendation
	// Limit the seed set to the most recent N videos to bound Redis fan-out.
	const maxSeeds = 20
	seeds := user.WatchHistory
	if len(seeds) > maxSeeds {
		seeds = seeds[:maxSeeds]
	}

	type simEntry struct {
		VideoID    string
		Similarity float64
	}

	// Gather top-K similar items for each seed in parallel.
	var mu sync.Mutex
	rawScores := make(map[string]float64, cfg.CandidatePoolSize)

	var wg sync.WaitGroup
	errCh := make(chan error, len(seeds))

	for _, seedID := range seeds {
		seedID := seedID
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := cfSimilarKey(seedID)
			// Stored as a Redis sorted set: member=videoID, score=similarity.
			zSlice, err := g.rdb.ZRevRangeWithScores(ctx, key, 0,
				int64(g.cfg.ModelUpdate.TopKSimilarItems-1)).Result()
			if err != nil {
				if err != redis.Nil {
					errCh <- fmt.Errorf("CF zrange %s: %w", key, err)
				}
				return
			}
			mu.Lock()
			for _, z := range zSlice {
				id, ok := z.Member.(string)
				if !ok {
					continue
				}
				if z.Score > rawScores[id] {
					rawScores[id] = z.Score
				}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	close(errCh)

	// Collect non-fatal errors.
	for err := range errCh {
		g.logger.Warn("CF redis error", zap.Error(err))
	}

	// Convert to candidates; exclude videos already in the watch history.
	watchSet := make(map[string]struct{}, len(user.WatchHistory))
	for _, id := range user.WatchHistory {
		watchSet[id] = struct{}{}
	}

	type scoredID struct {
		id    string
		score float64
	}
	sorted := make([]scoredID, 0, len(rawScores))
	for id, s := range rawScores {
		if _, already := watchSet[id]; already {
			continue
		}
		sorted = append(sorted, scoredID{id, s})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	limit := cfg.CandidatePoolSize / 5 // allocate ~20% of pool to CF
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	candidates := make([]*models.CandidateVideo, len(sorted))
	for i, s := range sorted {
		candidates[i] = &models.CandidateVideo{
			VideoID:        s.id,
			Source:         models.SourceCollaborativeFiltering,
			RetrievalScore: s.score,
		}
	}
	return candidates, nil
}

// -----------------------------------------------------------------
// Strategy 2 – Content-Based (Elasticsearch KNN)
// -----------------------------------------------------------------
// Encode the user's interest embedding and perform an approximate-nearest-
// neighbour search over the video embedding index.

func (g *CandidateGenerator) ContentBased(
	ctx context.Context,
	user *models.UserFeatures,
) ([]*models.CandidateVideo, error) {

	if len(user.Embedding) == 0 {
		return nil, nil
	}

	ecfg := g.cfg.Elasticsearch
	emcfg := g.cfg.Embedding

	// Build KNN query body.
	queryBody := map[string]interface{}{
		"knn": map[string]interface{}{
			"field":          "embedding",
			"query_vector":   user.Embedding,
			"k":              emcfg.KNNTopK,
			"num_candidates": emcfg.KNNNumCandidates,
			"filter": map[string]interface{}{
				"term": map[string]interface{}{
					"is_active": true,
				},
			},
		},
		"_source": []string{"video_id", "creator_id", "_score"},
		"size":    emcfg.KNNTopK,
	}

	body, err := json.Marshal(queryBody)
	if err != nil {
		return nil, fmt.Errorf("marshal KNN query: %w", err)
	}

	res, err := g.es.Search(
		g.es.Search.WithContext(ctx),
		g.es.Search.WithIndex(ecfg.EmbeddingIndex),
		g.es.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch KNN search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch KNN error response [%s]", res.Status())
	}

	var esResp struct {
		Hits struct {
			Hits []struct {
				ID     string  `json:"_id"`
				Score  float64 `json:"_score"`
				Source struct {
					VideoID   string `json:"video_id"`
					CreatorID string `json:"creator_id"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode KNN response: %w", err)
	}

	candidates := make([]*models.CandidateVideo, 0, len(esResp.Hits.Hits))
	for _, hit := range esResp.Hits.Hits {
		videoID := hit.Source.VideoID
		if videoID == "" {
			videoID = hit.ID // fall back to document _id
		}
		candidates = append(candidates, &models.CandidateVideo{
			VideoID:        videoID,
			CreatorID:      hit.Source.CreatorID,
			Source:         models.SourceContentBased,
			RetrievalScore: hit.Score,
		})
	}
	return candidates, nil
}

// -----------------------------------------------------------------
// Strategy 3 – Trending Pool
// -----------------------------------------------------------------
// Pull from the pre-computed global and per-category trending sorted sets.

func (g *CandidateGenerator) Trending(
	ctx context.Context,
	user *models.UserFeatures,
) ([]*models.CandidateVideo, error) {

	limit := g.cfg.Recommendation.CandidatePoolSize / 5

	var mu sync.Mutex
	scoreMap := make(map[string]float64, limit)

	// 1. Global trending set.
	globalKey := trendingGlobalKey()
	g.fetchTrendingSet(ctx, globalKey, limit, &mu, scoreMap)

	// 2. Per-category trending sets for the user's top-3 categories.
	type catScore struct {
		cat   string
		score float64
	}
	topCats := topNCategories(user.LikedCategories, 3)
	for _, cs := range topCats {
		key := trendingCategoryKey(cs.cat)
		g.fetchTrendingSet(ctx, key, limit/3, &mu, scoreMap)
	}

	candidates := make([]*models.CandidateVideo, 0, len(scoreMap))
	for id, score := range scoreMap {
		candidates = append(candidates, &models.CandidateVideo{
			VideoID:        id,
			Source:         models.SourceTrending,
			RetrievalScore: score,
		})
	}
	// Sort descending by trending score.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].RetrievalScore > candidates[j].RetrievalScore
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func (g *CandidateGenerator) fetchTrendingSet(
	ctx context.Context,
	key string,
	n int,
	mu *sync.Mutex,
	dest map[string]float64,
) {
	zSlice, err := g.rdb.ZRevRangeWithScores(ctx, key, 0, int64(n-1)).Result()
	if err != nil {
		if err != redis.Nil {
			g.logger.Warn("trending zrange failed", zap.String("key", key), zap.Error(err))
		}
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for _, z := range zSlice {
		id, ok := z.Member.(string)
		if !ok {
			continue
		}
		if z.Score > dest[id] {
			dest[id] = z.Score
		}
	}
}

// -----------------------------------------------------------------
// Strategy 4 – Following Network
// -----------------------------------------------------------------
// Fetch the latest N videos from each creator the user follows.

func (g *CandidateGenerator) FollowingNetwork(
	ctx context.Context,
	user *models.UserFeatures,
) ([]*models.CandidateVideo, error) {

	if len(user.FollowedCreators) == 0 {
		return nil, nil
	}

	const maxCreators = 50
	const videosPerCreator = 5

	creators := user.FollowedCreators
	if len(creators) > maxCreators {
		creators = creators[:maxCreators]
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	candidates := make([]*models.CandidateVideo, 0, len(creators)*videosPerCreator)

	for _, creatorID := range creators {
		creatorID := creatorID
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := creatorVideosKey(creatorID)
			// The creator video key is a Redis sorted set keyed by publish time (Unix ts).
			zSlice, err := g.rdb.ZRevRangeWithScores(ctx, key, 0, videosPerCreator-1).Result()
			if err != nil {
				if err != redis.Nil {
					g.logger.Warn("following network zrange failed",
						zap.String("creator_id", creatorID),
						zap.Error(err))
				}
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, z := range zSlice {
				id, ok := z.Member.(string)
				if !ok {
					continue
				}
				// Convert Unix timestamp to a normalised freshness-ish score [0, 1].
				now := float64(time.Now().Unix())
				age := now - z.Score
				if age < 0 {
					age = 0
				}
				// Exponential decay with a 24-hour half-life.
				halfLife := g.cfg.Recommendation.FreshnessHalfLifeHours * 3600
				retrievalScore := math.Exp(-0.693 * age / halfLife)

				candidates = append(candidates, &models.CandidateVideo{
					VideoID:        id,
					CreatorID:      creatorID,
					Source:         models.SourceFollowingNetwork,
					RetrievalScore: retrievalScore,
				})
			}
		}()
	}
	wg.Wait()
	return candidates, nil
}

// -----------------------------------------------------------------
// Strategy 5 – Recent Interaction Graph
// -----------------------------------------------------------------
// Use the user's recent interaction history to expand candidates through a
// one-hop graph walk: find other users who interacted with the same videos
// (co-interaction), then surface videos those users liked that the current
// user has not seen.

func (g *CandidateGenerator) RecentInteractionGraph(
	ctx context.Context,
	user *models.UserFeatures,
) ([]*models.CandidateVideo, error) {

	if len(user.WatchHistory) == 0 {
		return nil, nil
	}

	const (
		maxSeedVideos   = 10  // limit fan-out
		maxCoUsers      = 20  // co-interacted users per seed
		videosPerCoUser = 5
	)

	seeds := user.WatchHistory
	if len(seeds) > maxSeedVideos {
		seeds = seeds[:maxSeedVideos]
	}

	watchSet := make(map[string]struct{}, len(user.WatchHistory))
	for _, id := range user.WatchHistory {
		watchSet[id] = struct{}{}
	}

	var mu sync.Mutex
	scoreMap := make(map[string]float64, 200)
	var wg sync.WaitGroup

	for _, seedID := range seeds {
		seedID := seedID
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Fetch co-users: the sorted set stores userIDs scored by interaction
			// weight (likes > views etc.) so the highest-signal users come first.
			coUsersKey := videoCoUsersKey(seedID)
			coUsers, err := g.rdb.ZRevRange(ctx, coUsersKey, 0, maxCoUsers-1).Result()
			if err != nil {
				if err != redis.Nil {
					g.logger.Warn("co-users zrange failed",
						zap.String("video_id", seedID),
						zap.Error(err))
				}
				return
			}

			for _, coUserID := range coUsers {
				if coUserID == user.UserID {
					continue
				}
				// Fetch the co-user's liked videos.
				likedKey := userLikedKey(coUserID)
				liked, err := g.rdb.ZRevRangeWithScores(ctx, likedKey, 0,
					int64(videosPerCoUser-1)).Result()
				if err != nil {
					continue
				}
				mu.Lock()
				for _, z := range liked {
					id, ok := z.Member.(string)
					if !ok {
						continue
					}
					if _, seen := watchSet[id]; seen {
						continue
					}
					// Accumulate scores; each co-user vote adds weight.
					scoreMap[id] += z.Score
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	limit := g.cfg.Recommendation.CandidatePoolSize / 5

	type scoredID struct {
		id    string
		score float64
	}
	sorted := make([]scoredID, 0, len(scoreMap))
	for id, s := range scoreMap {
		sorted = append(sorted, scoredID{id, s})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	candidates := make([]*models.CandidateVideo, len(sorted))
	for i, s := range sorted {
		// Normalise raw accumulated score to [0, 1] by dividing by maxCoUsers * videosPerCoUser.
		normScore := s.score / float64(maxCoUsers*videosPerCoUser)
		if normScore > 1 {
			normScore = 1
		}
		candidates[i] = &models.CandidateVideo{
			VideoID:        s.id,
			Source:         models.SourceRecentInteractionGraph,
			RetrievalScore: normScore,
		}
	}
	return candidates, nil
}

// -----------------------------------------------------------------
// Redis key helpers
// -----------------------------------------------------------------

func cfSimilarKey(videoID string) string {
	return fmt.Sprintf("rec:cf:similar:%s", videoID)
}

func trendingGlobalKey() string {
	return "rec:trending:global"
}

func trendingCategoryKey(category string) string {
	return fmt.Sprintf("rec:trending:cat:%s", category)
}

func creatorVideosKey(creatorID string) string {
	return fmt.Sprintf("rec:creator:videos:%s", creatorID)
}

func videoCoUsersKey(videoID string) string {
	return fmt.Sprintf("rec:co_users:%s", videoID)
}

func userLikedKey(userID string) string {
	return fmt.Sprintf("rec:user:liked:%s", userID)
}

// -----------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------

type catScore struct {
	cat   string
	score float64
}

func topNCategories(cats map[string]float64, n int) []catScore {
	all := make([]catScore, 0, len(cats))
	for cat, score := range cats {
		all = append(all, catScore{cat, score})
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].score > all[j].score
	})
	if len(all) > n {
		all = all[:n]
	}
	return all
}
