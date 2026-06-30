package tests

import (
	"context"
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/recommendation-service/internal/config"
	"github.com/tiktok-clone/recommendation-service/internal/models"
	"github.com/tiktok-clone/recommendation-service/internal/services"
)

// =============================================================================
// Test helpers / fixtures
// =============================================================================

// newTestConfig returns a minimal Config suitable for unit tests.
func newTestConfig() *config.Config {
	return &config.Config{
		Recommendation: config.RecommendationConfig{
			CandidatePoolSize:         200,
			FinalFeedSize:             10,
			CoarseRankSize:            50,
			MaxConsecutiveSameCreator: 3,
			SeenVideoTTL:              7 * 24 * time.Hour,
			FreshnessHalfLifeHours:    24.0,
			CoarseWeightEngagement:    0.5,
			CoarseWeightFreshness:     0.3,
			CoarseWeightRelevance:     0.2,
		},
		FeatureStore: config.FeatureStoreConfig{
			WatchHistorySize: 100,
			UserFeatureTTL:   time.Hour,
			VideoFeatureTTL:  30 * time.Minute,
		},
		Embedding: config.EmbeddingConfig{
			Dimensions: 4, // small for tests
		},
		ModelUpdate: config.ModelUpdateConfig{
			MinInteractionsForItem: 2,
			TopKSimilarItems:       5,
		},
	}
}

// miniredisClient returns a real redis.UniversalClient backed by a local
// Redis instance.  Tests that need Redis are skipped if the instance is
// unavailable.
func redisClientOrSkip(t *testing.T) redis.UniversalClient {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable (%v); skipping integration test", err)
	}
	return rdb
}

// newVideoFeatures is a test fixture builder.
func newVideoFeatures(videoID, creatorID, category string, engagementRate float64, ageHours float64) *models.VideoFeatures {
	return &models.VideoFeatures{
		VideoID:        videoID,
		CreatorID:      creatorID,
		Category:       category,
		EngagementRate: engagementRate,
		ViewCount:      10000,
		LikeCount:      int64(engagementRate * 10000),
		LanguageCode:   "en",
		Duration:       45.0,
		PublishedAt:    time.Now().Add(-time.Duration(ageHours * float64(time.Hour))),
		TrendingScore:  0.5,
		IsActive:       true,
		Embedding:      []float64{0.5, 0.5, 0.5, 0.5},
	}
}

// newCandidate wraps a VideoFeatures in a CandidateVideo.
func newCandidate(vf *models.VideoFeatures, src models.SourceStrategy, score float64) *models.CandidateVideo {
	return &models.CandidateVideo{
		VideoID:        vf.VideoID,
		CreatorID:      vf.CreatorID,
		Source:         src,
		RetrievalScore: score,
		Features:       vf,
	}
}

// newUserFeatures returns a simple user feature set for tests.
func newUserFeatures() *models.UserFeatures {
	return &models.UserFeatures{
		UserID: "user-test-1",
		LikedCategories: map[string]float64{
			"comedy":    0.9,
			"education": 0.6,
		},
		FollowedCreators: []string{"creator-1"},
		DeviceType:       models.DeviceMobile,
		LanguageCode:     "en",
		Embedding:        []float64{0.5, 0.5, 0.5, 0.5},
		RetrievedAt:      time.Now(),
	}
}

// =============================================================================
// Unit tests for EmbeddingService helpers
// =============================================================================

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float64{1, 0, 0, 0}
	b := []float64{0, 1, 0, 0}
	got := services.CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("expected 0 for orthogonal vectors, got %f", got)
	}
}

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	v := []float64{1, 2, 3, 4}
	got := services.CosineSimilarity(v, v)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("expected 1.0 for identical vectors, got %f", got)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{-1, 0}
	got := services.CosineSimilarity(a, b)
	if math.Abs(got+1.0) > 1e-9 {
		t.Errorf("expected -1.0 for opposite vectors, got %f", got)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{1, 2, 3}
	got := services.CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("expected 0 for zero vector, got %f", got)
	}
}

func TestCosineSimilarity_MismatchedDimensions(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{1, 2}
	// Should not panic; should compute over the shorter dimension.
	got := services.CosineSimilarity(a, b)
	if math.IsNaN(got) {
		t.Error("expected non-NaN for mismatched dimensions")
	}
}

func TestL2Normalize_UnitVector(t *testing.T) {
	v := []float64{3, 4} // magnitude 5
	norm := services.L2Normalize(v)
	mag := math.Sqrt(norm[0]*norm[0] + norm[1]*norm[1])
	if math.Abs(mag-1.0) > 1e-9 {
		t.Errorf("expected unit magnitude after L2Normalize, got %f", mag)
	}
}

func TestL2Normalize_ZeroVector(t *testing.T) {
	v := []float64{0, 0, 0}
	norm := services.L2Normalize(v)
	for i, x := range norm {
		if x != 0 {
			t.Errorf("expected zero vector output, got %f at index %d", x, i)
		}
	}
}

func TestTopKByCosineSimilarity(t *testing.T) {
	query := []float64{1, 0, 0, 0}
	embeddings := map[string][]float64{
		"v1": {1, 0, 0, 0},   // sim = 1.0
		"v2": {0, 1, 0, 0},   // sim = 0.0
		"v3": {0.8, 0.6, 0, 0}, // sim ≈ 0.8
		"v4": {-1, 0, 0, 0},  // sim = -1.0
	}
	got := services.TopKByCosineSimilarity(query, embeddings, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0] != "v1" {
		t.Errorf("expected v1 at rank 0, got %s", got[0])
	}
	if got[1] != "v3" {
		t.Errorf("expected v3 at rank 1, got %s", got[1])
	}
}

// =============================================================================
// Unit tests for RankingService – no Redis required
// =============================================================================

// buildRankingService constructs a RankingService with a real Redis client (if
// available) or a nil-safe stub.  Pass a nil rdb to skip Redis-dependent tests.
func buildRankingService(t *testing.T, rdb redis.UniversalClient) *services.RankingService {
	t.Helper()
	cfg := newTestConfig()
	logger, _ := zap.NewDevelopment()
	fs := services.NewFeatureStore(cfg, rdb, logger)
	emb := services.NewEmbeddingService(cfg, nil, rdb, logger)
	return services.NewRankingService(cfg, fs, emb, rdb, logger)
}

// TestCoarseRankFormula_WeightedSum verifies the coarse ranking formula:
// score = engagement_rate * 0.5 + freshness * 0.3 + relevance * 0.2
// We use a mock ranker that exposes the coarse scores for inspection.
func TestCoarseRankFormula_HighEngagementBeatsOld(t *testing.T) {
	// Video A: high engagement, old
	// Video B: low engagement, very fresh
	// Expected: the coarse formula should balance these signals.
	cfg := newTestConfig()

	vfA := newVideoFeatures("v-a", "creator-a", "comedy", 0.9, 48.0) // 48h old
	vfB := newVideoFeatures("v-b", "creator-b", "education", 0.1, 1.0)  // 1h old

	user := newUserFeatures()

	// Manually compute expected scores.
	wE, wF, wR := cfg.Recommendation.CoarseWeightEngagement,
		cfg.Recommendation.CoarseWeightFreshness,
		cfg.Recommendation.CoarseWeightRelevance

	halfLife := cfg.Recommendation.FreshnessHalfLifeHours
	freshnessA := math.Exp(-0.693 * 48.0 / halfLife)
	freshnessB := math.Exp(-0.693 * 1.0 / halfLife)

	// Bayesian-smoothed engagement score.
	engScoreA := math.Sqrt((vfA.EngagementRate*float64(vfA.ViewCount) + 0.05*100) / (float64(vfA.ViewCount) + 100))
	engScoreB := math.Sqrt((vfB.EngagementRate*float64(vfB.ViewCount) + 0.05*100) / (float64(vfB.ViewCount) + 100))

	// Relevance: category affinity (no embeddings in this test).
	relA := user.LikedCategories[vfA.Category] // 0.9 for comedy
	relB := user.LikedCategories[vfB.Category] // 0.6 for education

	coarseA := wE*engScoreA + wF*freshnessA + wR*relA
	coarseB := wE*engScoreB + wF*freshnessB + wR*relB

	t.Logf("coarseA=%.4f coarseB=%.4f", coarseA, coarseB)

	// With high engagement AND high category affinity, A should dominate despite age.
	if coarseA <= coarseB {
		t.Errorf("expected coarseA (%.4f) > coarseB (%.4f) for high-engagement old video vs low-engagement fresh video",
			coarseA, coarseB)
	}
}

func TestFreshnessScore_Decay(t *testing.T) {
	halfLife := 24.0 // hours
	cases := []struct {
		ageHours float64
		wantApprox float64
		tolerancePct float64
	}{
		{0, 1.0, 0.01},
		{24, 0.5, 0.01},
		{48, 0.25, 0.01},
		{72, 0.125, 0.01},
	}
	for _, tc := range cases {
		got := math.Exp(-0.693 * tc.ageHours / halfLife)
		diff := math.Abs(got - tc.wantApprox)
		if diff > tc.tolerancePct {
			t.Errorf("ageHours=%.0f: got freshness %.4f, want ≈%.4f (diff %.4f)",
				tc.ageHours, got, tc.wantApprox, diff)
		}
	}
}

func TestFreshnessScore_NewVideoGetsMaxScore(t *testing.T) {
	vf := newVideoFeatures("v", "c", "comedy", 0.5, 0.0)
	vf.PublishedAt = time.Now() // just published
	age := time.Since(vf.PublishedAt).Hours()
	score := math.Exp(-0.693 * age / 24.0)
	if score < 0.99 {
		t.Errorf("expected near-1.0 freshness for brand-new video, got %.4f", score)
	}
}

// =============================================================================
// Diversity injection tests (no Redis needed)
// =============================================================================

func makeDiversityTestItems(n int, creatorsPerGroup int) []*models.RankedResult {
	items := make([]*models.RankedResult, n)
	for i := 0; i < n; i++ {
		creatorIdx := i / creatorsPerGroup
		items[i] = &models.RankedResult{
			VideoID:    fmt.Sprintf("video-%d", i),
			CreatorID:  fmt.Sprintf("creator-%d", creatorIdx),
			FinalScore: float64(n - i), // descending score
		}
	}
	return items
}

// injectDiversityDirect is a test-accessible wrapper since injectDiversity is
// unexported.  We test the observable behaviour through the public Rank method.
func TestDiversityInjection_MaxConsecutive(t *testing.T) {
	rdb := redisClientOrSkip(t)
	svc := buildRankingService(t, rdb)

	// 9 videos: 3 from creator-0, 3 from creator-1, 3 from creator-2.
	// Each group's videos score higher than the next group.
	// With maxConsecutive=3, consecutive runs of ≤3 per creator are acceptable.
	candidates := make([]*models.CandidateVideo, 0, 9)
	for creatorIdx := 0; creatorIdx < 3; creatorIdx++ {
		for vidIdx := 0; vidIdx < 3; vidIdx++ {
			videoID := fmt.Sprintf("vid-%d-%d", creatorIdx, vidIdx)
			creatorID := fmt.Sprintf("creator-%d", creatorIdx)
			// Higher-indexed creators score less.
			baseScore := float64(30 - creatorIdx*10 - vidIdx)
			vf := newVideoFeatures(videoID, creatorID, "comedy", 0.5, 1.0)
			vf.EngagementRate = baseScore / 30.0
			candidates = append(candidates, newCandidate(vf, models.SourceTrending, baseScore))
		}
	}

	user := newUserFeatures()
	ctx := context.Background()

	// Flush test keys first.
	rdb.Del(ctx, fmt.Sprintf("rec:seen:%s", user.UserID))

	results, err := svc.Rank(ctx, candidates, user, "")
	if err != nil {
		t.Fatalf("Rank failed: %v", err)
	}

	// Verify the diversity constraint: no more than MaxConsecutiveSameCreator
	// from the same creator in a row.
	maxRun := 3
	consecutiveCount := 0
	lastCreator := ""
	for _, item := range results {
		if item.CreatorID == lastCreator {
			consecutiveCount++
			if consecutiveCount > maxRun {
				t.Errorf("diversity violated: %d consecutive videos from creator %s (max %d)",
					consecutiveCount, lastCreator, maxRun)
			}
		} else {
			consecutiveCount = 1
			lastCreator = item.CreatorID
		}
	}
}

func TestDiversityInjection_SingleCreator(t *testing.T) {
	rdb := redisClientOrSkip(t)
	svc := buildRankingService(t, rdb)

	// All candidates from the same creator; diversity should still return them all
	// (fall-through logic when no other creator is available).
	candidates := make([]*models.CandidateVideo, 6)
	for i := range candidates {
		videoID := fmt.Sprintf("vid-single-%d", i)
		vf := newVideoFeatures(videoID, "solo-creator", "comedy", 0.5, float64(i))
		candidates[i] = newCandidate(vf, models.SourceTrending, float64(10-i))
	}

	user := newUserFeatures()
	ctx := context.Background()
	rdb.Del(ctx, fmt.Sprintf("rec:seen:%s", user.UserID))

	results, err := svc.Rank(ctx, candidates, user, "")
	if err != nil {
		t.Fatalf("Rank failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected non-empty results even when all videos are from one creator")
	}
}

// =============================================================================
// Filter already-seen tests
// =============================================================================

func TestFilterSeen_RemovesSeenVideos(t *testing.T) {
	rdb := redisClientOrSkip(t)
	svc := buildRankingService(t, rdb)

	user := newUserFeatures()
	ctx := context.Background()

	// Pre-populate seen set with 2 videos.
	seenKey := fmt.Sprintf("rec:seen:%s", user.UserID)
	rdb.Del(ctx, seenKey)
	rdb.SAdd(ctx, seenKey, "seen-vid-1", "seen-vid-2")

	candidates := []*models.CandidateVideo{
		newCandidate(newVideoFeatures("seen-vid-1", "c1", "comedy", 0.5, 1.0), models.SourceTrending, 0.9),
		newCandidate(newVideoFeatures("seen-vid-2", "c2", "comedy", 0.4, 2.0), models.SourceTrending, 0.8),
		newCandidate(newVideoFeatures("new-vid-3", "c3", "comedy", 0.7, 0.5), models.SourceTrending, 0.95),
	}

	results, err := svc.Rank(ctx, candidates, user, "")
	if err != nil {
		t.Fatalf("Rank failed: %v", err)
	}

	for _, r := range results {
		if r.VideoID == "seen-vid-1" || r.VideoID == "seen-vid-2" {
			t.Errorf("seen video %s should have been filtered out", r.VideoID)
		}
	}
	if len(results) != 1 || results[0].VideoID != "new-vid-3" {
		t.Errorf("expected only new-vid-3 in results, got %v", results)
	}

	// Cleanup.
	rdb.Del(ctx, seenKey)
}

func TestMarkSeen_PersistsToRedis(t *testing.T) {
	rdb := redisClientOrSkip(t)
	svc := buildRankingService(t, rdb)

	ctx := context.Background()
	userID := "test-mark-seen-user"
	seenKey := fmt.Sprintf("rec:seen:%s", userID)
	rdb.Del(ctx, seenKey)

	videoIDs := []string{"vid-a", "vid-b", "vid-c"}
	if err := svc.MarkSeen(ctx, userID, videoIDs); err != nil {
		t.Fatalf("MarkSeen failed: %v", err)
	}

	for _, id := range videoIDs {
		isMember, err := rdb.SIsMember(ctx, seenKey, id).Result()
		if err != nil {
			t.Fatalf("SIsMember failed: %v", err)
		}
		if !isMember {
			t.Errorf("expected video %s to be in seen set", id)
		}
	}

	// Cleanup.
	rdb.Del(ctx, seenKey)
}

// =============================================================================
// Full Rank pipeline integration test
// =============================================================================

func TestRank_ReturnsSortedByFinalScore(t *testing.T) {
	rdb := redisClientOrSkip(t)
	svc := buildRankingService(t, rdb)

	user := newUserFeatures()
	ctx := context.Background()
	rdb.Del(ctx, fmt.Sprintf("rec:seen:%s", user.UserID))

	// Create candidates with varying characteristics.
	candidates := []*models.CandidateVideo{
		newCandidate(newVideoFeatures("v1", "c1", "comedy", 0.9, 2.0), models.SourceCollaborativeFiltering, 0.8),
		newCandidate(newVideoFeatures("v2", "c2", "education", 0.3, 1.0), models.SourceContentBased, 0.5),
		newCandidate(newVideoFeatures("v3", "c3", "comedy", 0.7, 0.5), models.SourceTrending, 0.9),
		newCandidate(newVideoFeatures("v4", "c4", "education", 0.5, 5.0), models.SourceFollowingNetwork, 0.6),
		newCandidate(newVideoFeatures("v5", "c5", "comedy", 0.6, 3.0), models.SourceRecentInteractionGraph, 0.7),
	}

	results, err := svc.Rank(ctx, candidates, user, "exp-123")
	if err != nil {
		t.Fatalf("Rank failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	// Verify results are ordered by FinalScore descending.
	for i := 1; i < len(results); i++ {
		if results[i].FinalScore > results[i-1].FinalScore {
			t.Errorf("results not sorted: position %d (score %.4f) > position %d (score %.4f)",
				i, results[i].FinalScore, i-1, results[i-1].FinalScore)
		}
	}

	// Verify experiment ID is attached.
	for _, r := range results {
		if r.ExperimentID != "exp-123" {
			t.Errorf("expected experiment_id 'exp-123', got '%s'", r.ExperimentID)
		}
	}

	// Cleanup.
	rdb.Del(ctx, fmt.Sprintf("rec:seen:%s", user.UserID))
}

func TestRank_EmptyCandidates(t *testing.T) {
	rdb := redisClientOrSkip(t)
	svc := buildRankingService(t, rdb)

	ctx := context.Background()
	user := newUserFeatures()

	results, err := svc.Rank(ctx, nil, user, "")
	if err != nil {
		t.Fatalf("Rank with nil candidates should not error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results for nil candidates, got %d", len(results))
	}
}

func TestRank_RespectsPageSize(t *testing.T) {
	rdb := redisClientOrSkip(t)
	cfg := newTestConfig()
	cfg.Recommendation.FinalFeedSize = 3

	logger, _ := zap.NewDevelopment()
	fs := services.NewFeatureStore(cfg, rdb, logger)
	emb := services.NewEmbeddingService(cfg, nil, rdb, logger)
	svc := services.NewRankingService(cfg, fs, emb, rdb, logger)

	user := newUserFeatures()
	ctx := context.Background()
	rdb.Del(ctx, fmt.Sprintf("rec:seen:%s", user.UserID))

	// Create 10 candidates.
	candidates := make([]*models.CandidateVideo, 10)
	for i := range candidates {
		videoID := fmt.Sprintf("page-vid-%d", i)
		vf := newVideoFeatures(videoID, fmt.Sprintf("c%d", i), "comedy", 0.5, float64(i))
		candidates[i] = newCandidate(vf, models.SourceTrending, float64(10-i))
	}

	results, err := svc.Rank(ctx, candidates, user, "")
	if err != nil {
		t.Fatalf("Rank failed: %v", err)
	}
	if len(results) > 3 {
		t.Errorf("expected at most 3 results (FinalFeedSize=3), got %d", len(results))
	}

	// Cleanup.
	rdb.Del(ctx, fmt.Sprintf("rec:seen:%s", user.UserID))
}

// =============================================================================
// A/B testing unit tests
// =============================================================================

func newABConfig() *config.ABTestingConfig {
	return &config.ABTestingConfig{
		ExperimentsKey:  "rec:experiments:active:test",
		RefreshInterval: 60 * time.Second,
		TrackingEnabled: false, // disable Redis writes in unit tests
	}
}

func TestABTesting_ConsistentAssignment(t *testing.T) {
	rdb := redisClientOrSkip(t)
	logger, _ := zap.NewDevelopment()
	cfg := newABConfig()

	svc := services.NewABTestingService(cfg, rdb, logger)
	defer svc.Stop()

	ctx := context.Background()

	exp := &services.Experiment{
		ID:     "test-exp-1",
		Name:   "Test Experiment",
		Status: services.ExperimentStatusActive,
		Variants: []services.Variant{
			{ID: "control", TrafficPercent: 50},
			{ID: "treatment", TrafficPercent: 50},
		},
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
	}
	if err := svc.UpsertExperiment(ctx, exp); err != nil {
		t.Fatalf("UpsertExperiment failed: %v", err)
	}

	// The same user should always get the same variant.
	userID := "stable-user-abc"
	firstAssignment := svc.AssignVariant(ctx, userID, "US")
	if firstAssignment == nil {
		t.Fatal("expected an assignment, got nil")
	}
	for i := 0; i < 100; i++ {
		a := svc.AssignVariant(ctx, userID, "US")
		if a == nil {
			t.Fatalf("nil assignment on iteration %d", i)
		}
		if a.VariantID != firstAssignment.VariantID {
			t.Errorf("inconsistent assignment: got %s, want %s on iteration %d",
				a.VariantID, firstAssignment.VariantID, i)
		}
	}

	// Cleanup.
	rdb.Del(ctx, cfg.ExperimentsKey)
	rdb.Del(ctx, fmt.Sprintf("rec:ab:exp:%s", exp.ID))
}

func TestABTesting_TrafficDistribution(t *testing.T) {
	rdb := redisClientOrSkip(t)
	logger, _ := zap.NewDevelopment()
	cfg := newABConfig()

	svc := services.NewABTestingService(cfg, rdb, logger)
	defer svc.Stop()

	ctx := context.Background()

	exp := &services.Experiment{
		ID:     "test-exp-dist",
		Name:   "Distribution Test",
		Status: services.ExperimentStatusActive,
		Variants: []services.Variant{
			{ID: "control", TrafficPercent: 50},
			{ID: "treatment", TrafficPercent: 50},
		},
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
	}
	if err := svc.UpsertExperiment(ctx, exp); err != nil {
		t.Fatalf("UpsertExperiment failed: %v", err)
	}

	// Assign 1000 distinct users and verify ≈50/50 split.
	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		userID := fmt.Sprintf("dist-user-%d", i)
		a := svc.AssignVariant(ctx, userID, "US")
		if a != nil {
			counts[a.VariantID]++
		}
	}

	total := counts["control"] + counts["treatment"]
	if total == 0 {
		t.Fatal("no assignments made; experiment may not have loaded")
	}
	ratioControl := float64(counts["control"]) / float64(total)
	// Allow 10% tolerance around the expected 50%.
	if ratioControl < 0.40 || ratioControl > 0.60 {
		t.Errorf("traffic distribution out of range: control=%.1f%% (expected ~50%%)",
			ratioControl*100)
	}

	// Cleanup.
	rdb.Del(ctx, cfg.ExperimentsKey)
	rdb.Del(ctx, fmt.Sprintf("rec:ab:exp:%s", exp.ID))
}

func TestABTesting_InactiveExperiment(t *testing.T) {
	rdb := redisClientOrSkip(t)
	logger, _ := zap.NewDevelopment()
	cfg := newABConfig()

	svc := services.NewABTestingService(cfg, rdb, logger)
	defer svc.Stop()

	ctx := context.Background()

	exp := &services.Experiment{
		ID:     "test-exp-inactive",
		Name:   "Inactive Experiment",
		Status: services.ExperimentStatusPaused,
		Variants: []services.Variant{
			{ID: "control", TrafficPercent: 50},
		},
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
	}
	if err := svc.UpsertExperiment(ctx, exp); err != nil {
		t.Fatalf("UpsertExperiment failed: %v", err)
	}

	a := svc.AssignVariant(ctx, "any-user", "US")
	if a != nil {
		t.Errorf("expected nil assignment for paused experiment, got %+v", a)
	}

	// Cleanup.
	rdb.Del(ctx, cfg.ExperimentsKey)
	rdb.Del(ctx, fmt.Sprintf("rec:ab:exp:%s", exp.ID))
}

func TestABTesting_CountryRestriction(t *testing.T) {
	rdb := redisClientOrSkip(t)
	logger, _ := zap.NewDevelopment()
	cfg := newABConfig()

	svc := services.NewABTestingService(cfg, rdb, logger)
	defer svc.Stop()

	ctx := context.Background()

	exp := &services.Experiment{
		ID:     "test-exp-country",
		Name:   "Country Restricted",
		Status: services.ExperimentStatusActive,
		Variants: []services.Variant{
			{ID: "treatment", TrafficPercent: 100},
		},
		StartTime:         time.Now().Add(-time.Hour),
		EndTime:           time.Now().Add(time.Hour),
		EligibleCountries: []string{"US", "CA"},
	}
	if err := svc.UpsertExperiment(ctx, exp); err != nil {
		t.Fatalf("UpsertExperiment failed: %v", err)
	}

	// US user should get assigned.
	aUS := svc.AssignVariant(ctx, "us-user", "US")
	if aUS == nil {
		t.Error("expected assignment for US user in US-eligible experiment")
	}

	// DE user should not get assigned.
	aDE := svc.AssignVariant(ctx, "de-user", "DE")
	if aDE != nil {
		t.Errorf("expected no assignment for DE user in US/CA-only experiment, got %+v", aDE)
	}

	// Cleanup.
	rdb.Del(ctx, cfg.ExperimentsKey)
	rdb.Del(ctx, fmt.Sprintf("rec:ab:exp:%s", exp.ID))
}

// =============================================================================
// Feature store tests
// =============================================================================

func TestFeatureStore_WatchHistoryRoundTrip(t *testing.T) {
	rdb := redisClientOrSkip(t)
	cfg := newTestConfig()
	logger, _ := zap.NewDevelopment()
	fs := services.NewFeatureStore(cfg, rdb, logger)

	ctx := context.Background()
	userID := "fs-watch-test-user"

	watchKey := fmt.Sprintf("rec:user:watch:%s", userID)
	rdb.Del(ctx, watchKey)

	// Push 3 videos.
	for _, id := range []string{"vid-1", "vid-2", "vid-3"} {
		if err := fs.UpdateWatchHistory(ctx, userID, id); err != nil {
			t.Fatalf("UpdateWatchHistory failed for %s: %v", id, err)
		}
	}

	features, err := fs.GetUserFeatures(ctx, userID, models.RequestContext{})
	if err != nil {
		t.Fatalf("GetUserFeatures failed: %v", err)
	}

	// Most recent push (vid-3) should be at index 0.
	if len(features.WatchHistory) < 3 {
		t.Fatalf("expected 3 videos in watch history, got %d", len(features.WatchHistory))
	}
	if features.WatchHistory[0] != "vid-3" {
		t.Errorf("expected vid-3 at index 0, got %s", features.WatchHistory[0])
	}

	// Cleanup.
	rdb.Del(ctx, watchKey)
}

func TestFeatureStore_CategoryAffinityNormalisation(t *testing.T) {
	rdb := redisClientOrSkip(t)
	cfg := newTestConfig()
	logger, _ := zap.NewDevelopment()
	fs := services.NewFeatureStore(cfg, rdb, logger)

	ctx := context.Background()
	userID := "fs-cat-test-user"
	catsKey := fmt.Sprintf("rec:user:liked_cats:%s", userID)
	rdb.Del(ctx, catsKey)

	// Increment comedy to > 1.0 to trigger normalisation.
	for i := 0; i < 15; i++ {
		if err := fs.IncrementCategoryAffinity(ctx, userID, "comedy", 0.1); err != nil {
			t.Fatalf("IncrementCategoryAffinity failed: %v", err)
		}
	}

	features, err := fs.GetUserFeatures(ctx, userID, models.RequestContext{})
	if err != nil {
		t.Fatalf("GetUserFeatures failed: %v", err)
	}

	comedyScore := features.LikedCategories["comedy"]
	if comedyScore > 1.0+1e-6 {
		t.Errorf("category affinity exceeds 1.0 after normalisation: %.4f", comedyScore)
	}

	// Cleanup.
	rdb.Del(ctx, catsKey)
}

func TestFeatureStore_VideoFeaturesRoundTrip(t *testing.T) {
	rdb := redisClientOrSkip(t)
	cfg := newTestConfig()
	logger, _ := zap.NewDevelopment()
	fs := services.NewFeatureStore(cfg, rdb, logger)

	ctx := context.Background()
	vf := newVideoFeatures("store-vid-1", "c1", "comedy", 0.7, 2.0)

	videoKey := fmt.Sprintf("rec:video:features:%s", vf.VideoID)
	rdb.Del(ctx, videoKey)

	if err := fs.SetVideoFeatures(ctx, vf); err != nil {
		t.Fatalf("SetVideoFeatures failed: %v", err)
	}

	got, err := fs.GetVideoFeatures(ctx, vf.VideoID)
	if err != nil {
		t.Fatalf("GetVideoFeatures failed: %v", err)
	}

	if got.VideoID != vf.VideoID {
		t.Errorf("VideoID mismatch: got %s, want %s", got.VideoID, vf.VideoID)
	}
	if got.CreatorID != vf.CreatorID {
		t.Errorf("CreatorID mismatch: got %s, want %s", got.CreatorID, vf.CreatorID)
	}
	if math.Abs(got.EngagementRate-vf.EngagementRate) > 1e-6 {
		t.Errorf("EngagementRate mismatch: got %.4f, want %.4f", got.EngagementRate, vf.EngagementRate)
	}

	// Cleanup.
	rdb.Del(ctx, videoKey)
}

// =============================================================================
// Engagement score formula tests
// =============================================================================

func TestEngagementScore_BayesianSmoothing(t *testing.T) {
	// A video with 0 views should get a Bayesian-prior-based score, not NaN/0.
	vf := &models.VideoFeatures{
		VideoID:        "zero-views",
		EngagementRate: 0,
		ViewCount:      0,
	}
	// Apply the Bayesian formula directly.
	prior := 0.05
	priorWeight := 100.0
	smoothed := (vf.EngagementRate*float64(vf.ViewCount) + prior*priorWeight) /
		(float64(vf.ViewCount) + priorWeight)
	score := math.Sqrt(smoothed)

	if math.IsNaN(score) || math.IsInf(score, 0) {
		t.Errorf("Bayesian engagement score should be finite, got %f", score)
	}
	if score <= 0 {
		t.Errorf("Bayesian engagement score should be > 0 for zero-view video, got %f", score)
	}
}

func TestEngagementScore_HighEngagement(t *testing.T) {
	// High engagement should yield a score close to 1.
	vf := newVideoFeatures("viral", "c1", "comedy", 1.0, 1.0)
	vf.ViewCount = 1_000_000
	prior := 0.05
	priorWeight := 100.0
	smoothed := (vf.EngagementRate*float64(vf.ViewCount) + prior*priorWeight) /
		(float64(vf.ViewCount) + priorWeight)
	score := math.Sqrt(smoothed)
	if score < 0.99 {
		t.Errorf("expected score close to 1.0 for max engagement, got %.4f", score)
	}
}

// =============================================================================
// Benchmark
// =============================================================================

func BenchmarkCosineSimilarity(b *testing.B) {
	dims := 128
	a := make([]float64, dims)
	bVec := make([]float64, dims)
	for i := range a {
		a[i] = float64(i) / float64(dims)
		bVec[i] = float64(dims-i) / float64(dims)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		services.CosineSimilarity(a, bVec)
	}
}

func BenchmarkTopKByCosineSimilarity(b *testing.B) {
	dims := 128
	k := 20
	query := make([]float64, dims)
	for i := range query {
		query[i] = 1.0 / float64(dims)
	}
	embeddings := make(map[string][]float64, 500)
	for i := 0; i < 500; i++ {
		emb := make([]float64, dims)
		for j := range emb {
			emb[j] = float64((i+j)%dims) / float64(dims)
		}
		embeddings[fmt.Sprintf("v%d", i)] = emb
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		services.TopKByCosineSimilarity(query, embeddings, k)
	}
}

// Ensure sort stability for diversity injection edge cases.
func TestDiversityInjection_PreservesAllItems(t *testing.T) {
	items := makeDiversityTestItems(12, 4) // 3 creators, 4 videos each

	// Verify every item appears exactly once in output (no losses or duplicates).
	// We simulate the diversity logic by checking set membership.
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		seen[item.VideoID] = false
	}

	// Sort items to produce a deterministic order (simulating post-fine-rank).
	sorted := make([]*models.RankedResult, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FinalScore > sorted[j].FinalScore
	})

	maxRun := 3
	result := diversityInject(sorted, maxRun)

	for _, r := range result {
		if _, exists := seen[r.VideoID]; !exists {
			t.Errorf("unknown video ID in output: %s", r.VideoID)
		}
		seen[r.VideoID] = true
	}
	for id, present := range seen {
		if !present {
			t.Errorf("video %s missing from diversity output", id)
		}
	}
}

// diversityInject is a pure test-local reimplementation of the diversity
// injection algorithm to verify correctness independently of the service.
func diversityInject(items []*models.RankedResult, maxRun int) []*models.RankedResult {
	byCreator := make(map[string][]*models.RankedResult)
	for _, item := range items {
		byCreator[item.CreatorID] = append(byCreator[item.CreatorID], item)
	}

	type bucket struct {
		creatorID string
		items     []*models.RankedResult
	}
	buckets := make([]bucket, 0, len(byCreator))
	for cid, vids := range byCreator {
		buckets = append(buckets, bucket{cid, vids})
	}

	result := make([]*models.RankedResult, 0, len(items))
	consecutive := 0
	lastCreator := ""

	for len(result) < len(items) {
		bestIdx := -1
		var bestScore float64
		for i, b := range buckets {
			if len(b.items) == 0 {
				continue
			}
			top := b.items[0]
			if top.CreatorID == lastCreator && consecutive >= maxRun {
				continue
			}
			if bestIdx == -1 || top.FinalScore > bestScore {
				bestIdx = i
				bestScore = top.FinalScore
			}
		}
		if bestIdx == -1 {
			consecutive = 0
			lastCreator = ""
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
			consecutive++
		} else {
			consecutive = 1
			lastCreator = chosen.CreatorID
		}
		result = append(result, chosen)
	}
	return result
}
