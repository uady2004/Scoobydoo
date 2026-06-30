package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/recommendation-service/internal/config"
	"github.com/tiktok-clone/recommendation-service/internal/models"
)

// EmbeddingService provides three capabilities:
//
//  1. Video embedding lookup from Elasticsearch.
//  2. User embedding computation from interaction history.
//  3. Cosine similarity computation between two vectors.
type EmbeddingService struct {
	cfg    *config.Config
	es     *elasticsearch.Client
	rdb    redis.UniversalClient
	logger *zap.Logger
}

// NewEmbeddingService constructs an EmbeddingService.
func NewEmbeddingService(
	cfg *config.Config,
	es *elasticsearch.Client,
	rdb redis.UniversalClient,
	logger *zap.Logger,
) *EmbeddingService {
	return &EmbeddingService{
		cfg:    cfg,
		es:     es,
		rdb:    rdb,
		logger: logger,
	}
}

// -----------------------------------------------------------------
// Video embedding lookup
// -----------------------------------------------------------------

// GetVideoEmbedding retrieves the embedding vector for a single video from
// Elasticsearch.  The result is cached in Redis for VideoFeatureTTL to avoid
// repeated ES round-trips.
func (svc *EmbeddingService) GetVideoEmbedding(
	ctx context.Context,
	videoID string,
) ([]float64, error) {
	// 1. Check Redis cache.
	cacheKey := videoEmbeddingKey(videoID)
	if cached, err := svc.rdb.Get(ctx, cacheKey).Result(); err == nil {
		var emb []float64
		if jsonErr := json.Unmarshal([]byte(cached), &emb); jsonErr == nil {
			return emb, nil
		}
	}

	// 2. Fetch from Elasticsearch.
	res, err := svc.es.Get(
		svc.cfg.Elasticsearch.EmbeddingIndex,
		videoID,
		svc.es.Get.WithContext(ctx),
		svc.es.Get.WithSourceIncludes("embedding"),
	)
	if err != nil {
		return nil, fmt.Errorf("es get embedding for video %s: %w", videoID, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == 404 {
			return nil, fmt.Errorf("embedding not found for video %s", videoID)
		}
		return nil, fmt.Errorf("es error [%s] for video %s", res.Status(), videoID)
	}

	var doc struct {
		Source struct {
			Embedding []float64 `json:"embedding"`
		} `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode embedding response for video %s: %w", videoID, err)
	}
	emb := doc.Source.Embedding

	// 3. Write to Redis cache.
	if data, err := json.Marshal(emb); err == nil {
		svc.rdb.Set(ctx, cacheKey, data, svc.cfg.FeatureStore.VideoFeatureTTL) //nolint:errcheck
	}

	return emb, nil
}

// GetVideoEmbeddingsBatch fetches embeddings for multiple videos using an
// Elasticsearch mget request, falling back to individual Gets for cache hits.
func (svc *EmbeddingService) GetVideoEmbeddingsBatch(
	ctx context.Context,
	videoIDs []string,
) (map[string][]float64, error) {
	if len(videoIDs) == 0 {
		return map[string][]float64{}, nil
	}

	result := make(map[string][]float64, len(videoIDs))
	var mu sync.Mutex

	// 1. Check Redis cache for all IDs.
	pipe := svc.rdb.Pipeline()
	cacheCmds := make(map[string]*redis.StringCmd, len(videoIDs))
	for _, id := range videoIDs {
		cacheCmds[id] = pipe.Get(ctx, videoEmbeddingKey(id))
	}
	pipe.Exec(ctx) //nolint:errcheck

	uncached := make([]string, 0, len(videoIDs))
	for id, cmd := range cacheCmds {
		data, err := cmd.Result()
		if err != nil {
			uncached = append(uncached, id)
			continue
		}
		var emb []float64
		if json.Unmarshal([]byte(data), &emb) == nil {
			mu.Lock()
			result[id] = emb
			mu.Unlock()
		} else {
			uncached = append(uncached, id)
		}
	}

	if len(uncached) == 0 {
		return result, nil
	}

	// 2. Bulk-fetch uncached embeddings from Elasticsearch using mget.
	docs := make([]map[string]interface{}, len(uncached))
	for i, id := range uncached {
		docs[i] = map[string]interface{}{
			"_id": id,
		}
	}
	mgetBody := map[string]interface{}{
		"docs": docs,
	}
	bodyBytes, err := json.Marshal(mgetBody)
	if err != nil {
		return result, fmt.Errorf("marshal mget body: %w", err)
	}

	res, err := svc.es.Mget(
		bytes.NewReader(bodyBytes),
		svc.es.Mget.WithContext(ctx),
		svc.es.Mget.WithIndex(svc.cfg.Elasticsearch.EmbeddingIndex),
		svc.es.Mget.WithSourceIncludes("embedding"),
	)
	if err != nil {
		return result, fmt.Errorf("es mget embeddings: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return result, fmt.Errorf("es mget error [%s]", res.Status())
	}

	var mgetResp struct {
		Docs []struct {
			ID    string `json:"_id"`
			Found bool   `json:"found"`
			Source struct {
				Embedding []float64 `json:"embedding"`
			} `json:"_source"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(res.Body).Decode(&mgetResp); err != nil {
		return result, fmt.Errorf("decode mget response: %w", err)
	}

	// 3. Store results and back-fill Redis cache.
	writePipe := svc.rdb.Pipeline()
	for _, doc := range mgetResp.Docs {
		if !doc.Found || len(doc.Source.Embedding) == 0 {
			continue
		}
		emb := doc.Source.Embedding
		mu.Lock()
		result[doc.ID] = emb
		mu.Unlock()

		if data, err := json.Marshal(emb); err == nil {
			writePipe.Set(ctx, videoEmbeddingKey(doc.ID), data,
				svc.cfg.FeatureStore.VideoFeatureTTL)
		}
	}
	writePipe.Exec(ctx) //nolint:errcheck

	return result, nil
}

// -----------------------------------------------------------------
// User embedding computation
// -----------------------------------------------------------------

// ComputeUserEmbedding derives a user embedding by averaging the embeddings of
// the user's recently watched videos.  A recency-weighted average is used so
// that the most recently watched videos contribute more to the result.
//
// If the user has no watch history with available embeddings, a zero vector is
// returned.
func (svc *EmbeddingService) ComputeUserEmbedding(
	ctx context.Context,
	user *models.UserFeatures,
) ([]float64, error) {
	if len(user.WatchHistory) == 0 {
		return make([]float64, svc.cfg.Embedding.Dimensions), nil
	}

	// Limit computation to the most recent 50 videos.
	const maxHistory = 50
	history := user.WatchHistory
	if len(history) > maxHistory {
		history = history[:maxHistory]
	}

	embeddings, err := svc.GetVideoEmbeddingsBatch(ctx, history)
	if err != nil {
		return nil, fmt.Errorf("fetch video embeddings for user %s: %w", user.UserID, err)
	}
	if len(embeddings) == 0 {
		return make([]float64, svc.cfg.Embedding.Dimensions), nil
	}

	dims := svc.cfg.Embedding.Dimensions
	userEmb := make([]float64, dims)
	totalWeight := 0.0

	// Assign exponentially decaying weights so position 0 (most recent) has
	// weight 1, position 1 has weight e^{-0.1}, etc.
	for i, videoID := range history {
		emb, ok := embeddings[videoID]
		if !ok || len(emb) == 0 {
			continue
		}
		weight := math.Exp(-0.1 * float64(i))
		totalWeight += weight
		for j := 0; j < dims && j < len(emb); j++ {
			userEmb[j] += emb[j] * weight
		}
	}

	if totalWeight == 0 {
		return userEmb, nil
	}

	// Normalise by total weight.
	for j := range userEmb {
		userEmb[j] /= totalWeight
	}

	// L2-normalise so cosine similarity equals dot product.
	return L2Normalize(userEmb), nil
}

// RefreshUserEmbedding recomputes and stores the user embedding.  It is called
// by the feature-store writer after a significant interaction event.
func (svc *EmbeddingService) RefreshUserEmbedding(
	ctx context.Context,
	user *models.UserFeatures,
	featureStore *FeatureStore,
) error {
	emb, err := svc.ComputeUserEmbedding(ctx, user)
	if err != nil {
		return err
	}
	return featureStore.UpdateUserEmbedding(ctx, user.UserID, emb)
}

// -----------------------------------------------------------------
// Cosine similarity
// -----------------------------------------------------------------

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if either vector is all zeros or has mismatched dimensions.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	dims := len(a)
	if len(b) < dims {
		dims = len(b)
	}
	dot, normA, normB := 0.0, 0.0, 0.0
	for i := 0; i < dims; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// DotProduct computes the inner product of two equal-length vectors.
func DotProduct(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	dims := len(a)
	if len(b) < dims {
		dims = len(b)
	}
	dot := 0.0
	for i := 0; i < dims; i++ {
		dot += a[i] * b[i]
	}
	return dot
}

// L2Normalize returns a new unit-length vector.  Returns a zero vector if the
// input has zero magnitude.
func L2Normalize(v []float64) []float64 {
	norm := 0.0
	for _, x := range v {
		norm += x * x
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return v
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / norm
	}
	return out
}

// TopKByCosineSimilarity returns the top-K video IDs from the provided
// embedding map sorted by cosine similarity to the query vector.
func TopKByCosineSimilarity(
	query []float64,
	embeddings map[string][]float64,
	k int,
) []string {
	type scored struct {
		id    string
		score float64
	}
	scored_ := make([]scored, 0, len(embeddings))
	for id, emb := range embeddings {
		scored_ = append(scored_, scored{id, CosineSimilarity(query, emb)})
	}
	sort.Slice(scored_, func(i, j int) bool {
		return scored_[i].score > scored_[j].score
	})
	if len(scored_) > k {
		scored_ = scored_[:k]
	}
	result := make([]string, len(scored_))
	for i, s := range scored_ {
		result[i] = s.id
	}
	return result
}

// -----------------------------------------------------------------
// Redis key helpers
// -----------------------------------------------------------------

func videoEmbeddingKey(videoID string) string {
	return fmt.Sprintf("rec:video:emb:%s", videoID)
}
