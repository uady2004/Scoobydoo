package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/social-graph-service/internal/models"
	"github.com/tiktok-clone/social-graph-service/internal/repositories"
)

// ---------------------------------------------------------------------------
// Redis key
// ---------------------------------------------------------------------------

// suggestionCacheKeyFmt is the Redis key template for per-user cached
// suggestions. The key expires after SuggestionTTL (default 10 minutes).
const suggestionCacheKeyFmt = "social:suggestions:%s"

// ---------------------------------------------------------------------------
// Internal scoring type
// ---------------------------------------------------------------------------

// candidateScore holds intermediate BFS scoring data for a single suggestion
// candidate before it is converted to a FriendSuggestion.
type candidateScore struct {
	userID string
	// mutualFollowerCount is the number of depth-1 neighbours (users the viewer
	// already follows) who also follow this candidate. This is the primary
	// ranking signal.
	mutualFollowerCount int
	// mutualFollowerIDs holds the user IDs of those depth-1 neighbours. Up to 3
	// are surfaced as social proof in the API response.
	mutualFollowerIDs []string
	// mlScore is an optional score from an ML model (0 if unavailable).
	// When present it is blended with the mutual-follower signal.
	mlScore float64
	// blendedScore is the final ranking score computed by scoreCandidates.
	blendedScore float64
}

// ---------------------------------------------------------------------------
// SuggestionService
// ---------------------------------------------------------------------------

// SuggestionService computes friend suggestions using a BFS walk over the
// follow graph combined with an optional ML score. Results are cached in Redis
// with a short TTL.
//
// Algorithm overview (see computeSuggestions for the implementation):
//  1. Resolve the viewer's current following set (depth-1 nodes).
//  2. Resolve the viewer's blocked/blocking set.
//  3. BFS from the viewer up to bfsDepth hops:
//     - Depth-1 nodes are the viewer's direct followees (already followed — skip).
//     - Depth-2 nodes are the followees of each depth-1 node. Any such node that
//       the viewer does not already follow and has not blocked is a candidate.
//  4. For each candidate, count how many depth-1 nodes also follow it.
//     That count is the mutual-follower score.
//  5. Normalise mutual counts to [0,1] with sqrt dampening, then blend with
//     ML scores: finalScore = 0.7 * normMutual + 0.3 * mlScore.
//     When no ML score is available the full weight goes to mutual count.
//  6. Sort by final score descending, break ties by raw mutual count descending.
//  7. Return the top maxSuggestions results.
type SuggestionService struct {
	repo           repositories.GraphRepository
	redis          *redis.Client
	logger         *zap.Logger
	maxSuggestions int
	bfsDepth       int
	suggestionTTL  time.Duration
}

// NewSuggestionService creates a new SuggestionService.
func NewSuggestionService(
	repo repositories.GraphRepository,
	redisClient *redis.Client,
	logger *zap.Logger,
	maxSuggestions int,
	bfsDepth int,
	suggestionTTL time.Duration,
) *SuggestionService {
	return &SuggestionService{
		repo:           repo,
		redis:          redisClient,
		logger:         logger,
		maxSuggestions: maxSuggestions,
		bfsDepth:       bfsDepth,
		suggestionTTL:  suggestionTTL,
	}
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// GetSuggestions returns friend suggestions for viewerID.
// Results are served from Redis when available; on a cache miss the BFS
// algorithm is invoked and the result is asynchronously cached.
func (s *SuggestionService) GetSuggestions(ctx context.Context, viewerID string) ([]models.FriendSuggestion, error) {
	cacheKey := fmt.Sprintf(suggestionCacheKeyFmt, viewerID)

	// Try the Redis cache first.
	if cached, err := s.loadFromCache(ctx, cacheKey); err == nil && len(cached) > 0 {
		s.logger.Debug("suggestions: cache hit", zap.String("viewer_id", viewerID))
		return cached, nil
	}

	// Cache miss: run the full BFS computation.
	suggestions, err := s.computeSuggestions(ctx, viewerID)
	if err != nil {
		return nil, err
	}

	// Cache asynchronously so the caller is not delayed by the Redis write.
	go func() {
		if cacheErr := s.saveToCache(context.Background(), cacheKey, suggestions); cacheErr != nil {
			s.logger.Warn("suggestions: failed to write cache",
				zap.String("viewer_id", viewerID),
				zap.Error(cacheErr),
			)
		}
	}()

	return suggestions, nil
}

// InvalidateSuggestionsCache removes the cached suggestions for userID.
// Called after any follow/unfollow/block event that changes the user's graph
// neighbourhood.
func (s *SuggestionService) InvalidateSuggestionsCache(ctx context.Context, userID string) error {
	key := fmt.Sprintf(suggestionCacheKeyFmt, userID)
	return s.redis.Del(ctx, key).Err()
}

// ---------------------------------------------------------------------------
// BFS computation
// ---------------------------------------------------------------------------

// computeSuggestions executes the BFS graph traversal without caching.
func (s *SuggestionService) computeSuggestions(ctx context.Context, viewerID string) ([]models.FriendSuggestion, error) {
	// -------------------------------------------------------------------------
	// Step 1: load the viewer's current following set.
	// -------------------------------------------------------------------------
	followingIDs, err := s.repo.GetFollowingIDs(ctx, viewerID)
	if err != nil {
		return nil, fmt.Errorf("suggestions: get_following_ids: %w", err)
	}

	// alreadyFollowing is used throughout the BFS to skip nodes the viewer
	// already follows.
	alreadyFollowing := make(map[string]struct{}, len(followingIDs))
	for _, id := range followingIDs {
		alreadyFollowing[id] = struct{}{}
	}

	// With no follows there can be no depth-2 candidates; return early.
	if len(followingIDs) == 0 {
		return []models.FriendSuggestion{}, nil
	}

	// -------------------------------------------------------------------------
	// Step 2: load the blocked/blocking set (symmetric).
	// -------------------------------------------------------------------------
	blockedIDs, err := s.repo.GetBlockedIDs(ctx, viewerID)
	if err != nil {
		return nil, fmt.Errorf("suggestions: get_blocked_ids: %w", err)
	}
	blocked := make(map[string]struct{}, len(blockedIDs))
	for _, id := range blockedIDs {
		blocked[id] = struct{}{}
	}

	// -------------------------------------------------------------------------
	// Step 3: BFS traversal.
	//
	// visited tracks nodes whose outgoing edges we have already expanded so
	// the BFS never processes the same node twice.
	//
	// We start with the viewer's depth-1 nodes in the queue. For each such
	// node we fetch its followees and collect any that pass the candidate
	// filters. If bfsDepth > 2 we would enqueue those candidates for further
	// expansion, but the default depth of 2 means we stop after the first
	// expansion round.
	// -------------------------------------------------------------------------

	// visited is pre-seeded with the viewer and all depth-1 nodes so we do not
	// re-process them.
	visited := make(map[string]struct{}, len(followingIDs)+1)
	visited[viewerID] = struct{}{}
	for _, id := range followingIDs {
		visited[id] = struct{}{}
	}

	// candidates maps candidateID -> candidateScore.
	candidates := make(map[string]*candidateScore)

	// qEntry is a BFS queue element.
	type qEntry struct {
		userID string
		depth  int
	}

	// Seed the queue with depth-1 nodes.
	queue := make([]qEntry, 0, len(followingIDs))
	for _, id := range followingIDs {
		queue = append(queue, qEntry{userID: id, depth: 1})
	}

	for len(queue) > 0 {
		// Dequeue from front (standard BFS order).
		entry := queue[0]
		queue = queue[1:]

		// Do not expand beyond the configured depth.
		if entry.depth >= s.bfsDepth {
			continue
		}

		// Fetch the followees of the current node.
		neighborIDs, fetchErr := s.repo.GetFollowingIDs(ctx, entry.userID)
		if fetchErr != nil {
			// Non-fatal: log and continue — a single failed node should not
			// abort the whole suggestion computation.
			s.logger.Warn("BFS: GetFollowingIDs failed",
				zap.String("user_id", entry.userID),
				zap.Error(fetchErr),
			)
			continue
		}

		for _, neighborID := range neighborIDs {
			// Exclude: the viewer themselves.
			if neighborID == viewerID {
				continue
			}
			// Exclude: users the viewer already follows.
			if _, alreadyFollows := alreadyFollowing[neighborID]; alreadyFollows {
				continue
			}
			// Exclude: blocked / blocking users.
			if _, isBlocked := blocked[neighborID]; isBlocked {
				continue
			}

			// neighborID is a valid candidate. The current entry.userID is a
			// depth-1 node (a user the viewer follows) that also follows
			// neighborID, so it counts as a mutual connection.
			c, exists := candidates[neighborID]
			if !exists {
				c = &candidateScore{userID: neighborID}
				candidates[neighborID] = c
			}
			c.mutualFollowerCount++
			c.mutualFollowerIDs = append(c.mutualFollowerIDs, entry.userID)

			// Enqueue for further expansion only if the node has not been
			// visited yet and depth allows deeper traversal.
			if _, seen := visited[neighborID]; !seen {
				visited[neighborID] = struct{}{}
				nextDepth := entry.depth + 1
				if nextDepth < s.bfsDepth {
					queue = append(queue, qEntry{userID: neighborID, depth: nextDepth})
				}
			}
		}
	}

	if len(candidates) == 0 {
		return []models.FriendSuggestion{}, nil
	}

	// -------------------------------------------------------------------------
	// Steps 4 & 5: normalise, blend, sort.
	// -------------------------------------------------------------------------
	scored := s.scoreCandidates(candidates)

	// -------------------------------------------------------------------------
	// Step 6: take top-N.
	// -------------------------------------------------------------------------
	if len(scored) > s.maxSuggestions {
		scored = scored[:s.maxSuggestions]
	}

	// -------------------------------------------------------------------------
	// Step 7: convert to API response type.
	// -------------------------------------------------------------------------
	suggestions := make([]models.FriendSuggestion, 0, len(scored))
	for _, c := range scored {
		suggestions = append(suggestions, s.buildSuggestion(c))
	}
	return suggestions, nil
}

// ---------------------------------------------------------------------------
// Scoring
// ---------------------------------------------------------------------------

// scoreCandidates normalises mutual-follower counts, optionally blends with ML
// scores, and returns candidates sorted by final score descending.
//
// Scoring formula:
//   - normMutual = sqrt(mutualCount / maxMutualCount)   (sub-linear growth)
//   - finalScore = 0.7 * normMutual + 0.3 * mlScore     (when ML available)
//   - finalScore = normMutual                             (when ML unavailable)
func (s *SuggestionService) scoreCandidates(candidates map[string]*candidateScore) []*candidateScore {
	// Find the maximum mutual count for normalisation.
	maxMutual := 0
	for _, c := range candidates {
		if c.mutualFollowerCount > maxMutual {
			maxMutual = c.mutualFollowerCount
		}
	}

	result := make([]*candidateScore, 0, len(candidates))
	for _, c := range candidates {
		// Normalise to [0, 1] then apply sqrt dampening so that a candidate with
		// 10 mutual followers is not disproportionately ranked over one with 5.
		normMutual := 0.0
		if maxMutual > 0 {
			normMutual = math.Sqrt(float64(c.mutualFollowerCount) / float64(maxMutual))
		}

		const (
			wMutual = 0.7
			wML     = 0.3
		)

		if c.mlScore > 0 {
			// Clamp ML score to [0, 1].
			mlScore := math.Max(0, math.Min(1, c.mlScore))
			c.blendedScore = wMutual*normMutual + wML*mlScore
		} else {
			// No ML signal: full weight goes to the mutual-follower signal.
			c.blendedScore = normMutual
		}

		result = append(result, c)
	}

	// Sort by blended score descending. Ties are broken by raw mutual count so
	// that higher-confidence candidates are preferred.
	sort.Slice(result, func(i, j int) bool {
		if result[i].blendedScore != result[j].blendedScore {
			return result[i].blendedScore > result[j].blendedScore
		}
		return result[i].mutualFollowerCount > result[j].mutualFollowerCount
	})

	return result
}

// ---------------------------------------------------------------------------
// Response builder
// ---------------------------------------------------------------------------

// buildSuggestion converts a scored candidateScore into a FriendSuggestion.
// In a production deployment the UserSummary fields (username, avatar, etc.)
// would be hydrated via a batched gRPC call to the user-service using the
// candidate userIDs. Here they are populated with the ID only.
func (s *SuggestionService) buildSuggestion(c *candidateScore) models.FriendSuggestion {
	// Surface up to 3 mutual followers as social proof.
	proofCount := 3
	if len(c.mutualFollowerIDs) < proofCount {
		proofCount = len(c.mutualFollowerIDs)
	}
	mutualFollowers := make([]models.UserSummary, proofCount)
	for i := 0; i < proofCount; i++ {
		mutualFollowers[i] = models.UserSummary{UserID: c.mutualFollowerIDs[i]}
	}

	// Build a human-readable reason string.
	reason := fmt.Sprintf("%d mutual follower", c.mutualFollowerCount)
	if c.mutualFollowerCount != 1 {
		reason += "s"
	}

	return models.FriendSuggestion{
		User:                models.UserSummary{UserID: c.userID},
		MutualFollowerCount: c.mutualFollowerCount,
		MutualFollowers:     mutualFollowers,
		Score:               c.blendedScore,
		MLScore:             c.mlScore, // 0 when not consulted
		Reason:              reason,
	}
}

// ---------------------------------------------------------------------------
// Cache helpers
// ---------------------------------------------------------------------------

// loadFromCache retrieves a JSON-encoded []FriendSuggestion from Redis.
// Returns a non-nil error (including redis.Nil) on any miss or decode failure.
func (s *SuggestionService) loadFromCache(ctx context.Context, key string) ([]models.FriendSuggestion, error) {
	data, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var suggestions []models.FriendSuggestion
	if err := unmarshalJSON(data, &suggestions); err != nil {
		return nil, fmt.Errorf("suggestions: decode cache: %w", err)
	}
	return suggestions, nil
}

// saveToCache serialises suggestions to JSON and stores them in Redis with the
// configured TTL.
func (s *SuggestionService) saveToCache(ctx context.Context, key string, suggestions []models.FriendSuggestion) error {
	data, err := marshalJSON(suggestions)
	if err != nil {
		return fmt.Errorf("suggestions: encode cache: %w", err)
	}
	return s.redis.Set(ctx, key, data, s.suggestionTTL).Err()
}
