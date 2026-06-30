package tests

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/social-graph-service/internal/models"
	"github.com/tiktok-clone/social-graph-service/internal/repositories"
	"github.com/tiktok-clone/social-graph-service/internal/services"
)

// ---------------------------------------------------------------------------
// Fake repository
// ---------------------------------------------------------------------------

// fakeGraphRepo is an in-memory implementation of repositories.GraphRepository
// used exclusively in tests. It is intentionally kept simple: thread-safety
// is not required because all tests run sequentially.
type fakeGraphRepo struct {
	follows  map[string]map[string]*models.Follow // followerID -> followeeID -> Follow
	blocks   map[string]map[string]*models.Block  // blockerID -> blockedID -> Block
	nextID   int64
}

func newFakeGraphRepo() *fakeGraphRepo {
	return &fakeGraphRepo{
		follows: make(map[string]map[string]*models.Follow),
		blocks:  make(map[string]map[string]*models.Block),
		nextID:  1,
	}
}

func (r *fakeGraphRepo) nextAutoID() int64 {
	id := r.nextID
	r.nextID++
	return id
}

func (r *fakeGraphRepo) Follow(ctx context.Context, followerID, followeeID string) (*models.Follow, error) {
	if followerID == followeeID {
		return nil, repositories.ErrSelfFollow
	}
	if r.isBlocked(followerID, followeeID) || r.isBlocked(followeeID, followerID) {
		return nil, repositories.ErrUserBlocked
	}
	if _, ok := r.follows[followerID]; !ok {
		r.follows[followerID] = make(map[string]*models.Follow)
	}
	if _, alreadyExists := r.follows[followerID][followeeID]; alreadyExists {
		return nil, repositories.ErrAlreadyFollowing
	}
	f := &models.Follow{
		ID:         r.nextAutoID(),
		FollowerID: followerID,
		FolloweeID: followeeID,
		CreatedAt:  time.Now().UTC(),
	}
	r.follows[followerID][followeeID] = f
	return f, nil
}

func (r *fakeGraphRepo) Unfollow(ctx context.Context, followerID, followeeID string) error {
	if _, ok := r.follows[followerID]; !ok {
		return repositories.ErrNotFollowing
	}
	if _, ok := r.follows[followerID][followeeID]; !ok {
		return repositories.ErrNotFollowing
	}
	delete(r.follows[followerID], followeeID)
	return nil
}

func (r *fakeGraphRepo) GetFollowers(ctx context.Context, userID string, limit, offset int) ([]models.Follow, int64, error) {
	var out []models.Follow
	for _, byFollowee := range r.follows {
		if f, ok := byFollowee[userID]; ok {
			out = append(out, *f)
		}
	}
	total := int64(len(out))
	if offset >= len(out) {
		return []models.Follow{}, total, nil
	}
	out = out[offset:]
	if limit < len(out) {
		out = out[:limit]
	}
	return out, total, nil
}

func (r *fakeGraphRepo) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]models.Follow, int64, error) {
	var out []models.Follow
	if m, ok := r.follows[userID]; ok {
		for _, f := range m {
			out = append(out, *f)
		}
	}
	total := int64(len(out))
	if offset >= len(out) {
		return []models.Follow{}, total, nil
	}
	out = out[offset:]
	if limit < len(out) {
		out = out[:limit]
	}
	return out, total, nil
}

func (r *fakeGraphRepo) GetMutualFollowers(ctx context.Context, userID, targetID string, limit, offset int) ([]models.Follow, int64, error) {
	// mutual followers: someone who follows both userID and targetID
	var out []models.Follow
	for followerID, byFollowee := range r.follows {
		_, followsUser := byFollowee[userID]
		_, followsTarget := byFollowee[targetID]
		if followsUser && followsTarget && followerID != userID && followerID != targetID {
			out = append(out, *byFollowee[userID])
		}
	}
	total := int64(len(out))
	if offset >= len(out) {
		return []models.Follow{}, total, nil
	}
	out = out[offset:]
	if limit < len(out) {
		out = out[:limit]
	}
	return out, total, nil
}

func (r *fakeGraphRepo) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	if m, ok := r.follows[followerID]; ok {
		_, exists := m[followeeID]
		return exists, nil
	}
	return false, nil
}

func (r *fakeGraphRepo) GetFollowerCount(ctx context.Context, userID string) (int64, error) {
	var count int64
	for _, byFollowee := range r.follows {
		if _, ok := byFollowee[userID]; ok {
			count++
		}
	}
	return count, nil
}

func (r *fakeGraphRepo) GetFollowingCount(ctx context.Context, userID string) (int64, error) {
	if m, ok := r.follows[userID]; ok {
		return int64(len(m)), nil
	}
	return 0, nil
}

func (r *fakeGraphRepo) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	if _, ok := r.blocks[blockerID]; !ok {
		r.blocks[blockerID] = make(map[string]*models.Block)
	}
	r.blocks[blockerID][blockedID] = &models.Block{
		ID:        r.nextAutoID(),
		BlockerID: blockerID,
		BlockedID: blockedID,
		CreatedAt: time.Now().UTC(),
	}
	// Remove follow edges in both directions.
	if m, ok := r.follows[blockerID]; ok {
		delete(m, blockedID)
	}
	if m, ok := r.follows[blockedID]; ok {
		delete(m, blockerID)
	}
	return nil
}

func (r *fakeGraphRepo) UnblockUser(ctx context.Context, blockerID, blockedID string) error {
	if m, ok := r.blocks[blockerID]; ok {
		delete(m, blockedID)
	}
	return nil
}

func (r *fakeGraphRepo) GetBlockList(ctx context.Context, userID string, limit, offset int) ([]models.Block, int64, error) {
	var out []models.Block
	if m, ok := r.blocks[userID]; ok {
		for _, b := range m {
			out = append(out, *b)
		}
	}
	total := int64(len(out))
	if offset >= len(out) {
		return []models.Block{}, total, nil
	}
	out = out[offset:]
	if limit < len(out) {
		out = out[:limit]
	}
	return out, total, nil
}

func (r *fakeGraphRepo) IsBlocked(ctx context.Context, blockerID, blockedID string) (bool, error) {
	return r.isBlocked(blockerID, blockedID), nil
}

func (r *fakeGraphRepo) isBlocked(blockerID, blockedID string) bool {
	if m, ok := r.blocks[blockerID]; ok {
		_, exists := m[blockedID]
		return exists
	}
	return false
}

func (r *fakeGraphRepo) GetFollowingIDs(ctx context.Context, userID string) ([]string, error) {
	var out []string
	if m, ok := r.follows[userID]; ok {
		for id := range m {
			out = append(out, id)
		}
	}
	return out, nil
}

func (r *fakeGraphRepo) GetBlockedIDs(ctx context.Context, userID string) ([]string, error) {
	set := make(map[string]struct{})
	if m, ok := r.blocks[userID]; ok {
		for id := range m {
			set[id] = struct{}{}
		}
	}
	for blockerID, m := range r.blocks {
		if _, ok := m[userID]; ok {
			set[blockerID] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestRedis returns a miniredis-compatible real Redis client pointed at
// a local Redis if one is available, otherwise returns a no-op client.
// For unit tests we use a real Redis client with a dedicated test DB (index 15)
// to avoid polluting production data. If Redis is unreachable the tests that
// exercise Redis paths are skipped.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // dedicated test database
	})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping: " + err.Error())
	}
	// Flush the test DB before each test to ensure isolation.
	_ = c.FlushDB(ctx).Err()
	t.Cleanup(func() {
		_ = c.FlushDB(context.Background()).Err()
		_ = c.Close()
	})
	return c
}

// newTestSocialService wires up a SocialService backed by the fake repo and
// (optionally) a real Redis. The Kafka producer is a sarama mock that
// auto-succeeds all messages.
func newTestSocialService(t *testing.T, repo repositories.GraphRepository, redisClient *redis.Client) *services.SocialService {
	t.Helper()

	logger, _ := zap.NewDevelopment()
	cfg := mocks.NewSyncProducer(t, nil)
	// Allow any number of messages on any topic.
	cfg.ExpectSendMessageAndSucceed()
	cfg.ExpectSendMessageAndSucceed()
	cfg.ExpectSendMessageAndSucceed()
	cfg.ExpectSendMessageAndSucceed()
	cfg.ExpectSendMessageAndSucceed()
	cfg.ExpectSendMessageAndSucceed()
	cfg.ExpectSendMessageAndSucceed()
	cfg.ExpectSendMessageAndSucceed()

	suggestionSvc := services.NewSuggestionService(
		repo,
		redisClient,
		logger,
		20,
		2,
		10*time.Minute,
	)

	topics := services.TopicConfig{
		UserFollowed:   "user.followed",
		UserUnfollowed: "user.unfollowed",
		FeedInvalidate: "feed.invalidate",
		NotifyFollow:   "notification.follow",
	}

	return services.NewSocialService(
		repo,
		redisClient,
		cfg,
		suggestionSvc,
		logger,
		topics,
		"http://notification-service:8080",
		20,
		100,
		24*time.Hour,
	)
}

// permissiveMockProducer returns a sarama mock producer that accepts any
// number of messages without pre-registering expectations. It fulfils the
// sarama.SyncProducer interface.
type permissiveMockProducer struct{}

func (p *permissiveMockProducer) SendMessage(*sarama.ProducerMessage) (int32, int64, error) {
	return 0, 0, nil
}
func (p *permissiveMockProducer) SendMessages([]*sarama.ProducerMessage) error { return nil }
func (p *permissiveMockProducer) Close() error                                  { return nil }
func (p *permissiveMockProducer) IsTransactional() bool                         { return false }
func (p *permissiveMockProducer) BeginTxn() error                               { return nil }
func (p *permissiveMockProducer) CommitTxn() error                              { return nil }
func (p *permissiveMockProducer) AbortTxn() error                               { return nil }
func (p *permissiveMockProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}
func (p *permissiveMockProducer) AddMessageToTxnWithGroupMetadata(*sarama.ConsumerMessage, *sarama.ConsumerGroupMetadata, *string) error {
	return nil
}
func (p *permissiveMockProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}
func (p *permissiveMockProducer) AddOffsetsToTxnWithGroupMetadata(map[string][]*sarama.PartitionOffsetMetadata, *sarama.ConsumerGroupMetadata) error {
	return nil
}
func (p *permissiveMockProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return 0 }

// newPermissiveSocialService is like newTestSocialService but uses the
// permissive mock producer, avoiding the need to pre-register message
// expectations.
func newPermissiveSocialService(t *testing.T, repo repositories.GraphRepository, redisClient *redis.Client) *services.SocialService {
	t.Helper()
	logger, _ := zap.NewDevelopment()

	suggestionSvc := services.NewSuggestionService(
		repo,
		redisClient,
		logger,
		20,
		2,
		10*time.Minute,
	)

	topics := services.TopicConfig{
		UserFollowed:   "user.followed",
		UserUnfollowed: "user.unfollowed",
		FeedInvalidate: "feed.invalidate",
		NotifyFollow:   "notification.follow",
	}

	return services.NewSocialService(
		repo,
		redisClient,
		&permissiveMockProducer{},
		suggestionSvc,
		logger,
		topics,
		"http://notification-service:8080",
		20,
		100,
		24*time.Hour,
	)
}

// ---------------------------------------------------------------------------
// Tests: Follow
// ---------------------------------------------------------------------------

func TestFollow_Success(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	follow, err := svc.Follow(ctx, "user-A", "user-B")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if follow == nil {
		t.Fatal("expected non-nil Follow")
	}
	if follow.FollowerID != "user-A" || follow.FolloweeID != "user-B" {
		t.Errorf("unexpected follow: %+v", follow)
	}
}

func TestFollow_SelfFollow(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	_, err := svc.Follow(context.Background(), "user-A", "user-A")
	if !errors.Is(err, repositories.ErrSelfFollow) {
		t.Fatalf("expected ErrSelfFollow, got %v", err)
	}
}

func TestFollow_AlreadyFollowing(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	if _, err := svc.Follow(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("first follow failed: %v", err)
	}
	_, err := svc.Follow(ctx, "user-A", "user-B")
	if !errors.Is(err, repositories.ErrAlreadyFollowing) {
		t.Fatalf("expected ErrAlreadyFollowing, got %v", err)
	}
}

func TestFollow_BlockedUser(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	// Block user-B from user-A's side.
	if err := svc.BlockUser(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("block failed: %v", err)
	}
	_, err := svc.Follow(ctx, "user-A", "user-B")
	if !errors.Is(err, repositories.ErrUserBlocked) {
		t.Fatalf("expected ErrUserBlocked, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: Unfollow
// ---------------------------------------------------------------------------

func TestUnfollow_Success(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	if _, err := svc.Follow(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("setup follow: %v", err)
	}
	if err := svc.Unfollow(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("unfollow failed: %v", err)
	}

	isFollowing, err := repo.IsFollowing(ctx, "user-A", "user-B")
	if err != nil {
		t.Fatalf("is_following: %v", err)
	}
	if isFollowing {
		t.Error("expected user-A to not follow user-B after unfollow")
	}
}

func TestUnfollow_NotFollowing(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	err := svc.Unfollow(context.Background(), "user-A", "user-B")
	if !errors.Is(err, repositories.ErrNotFollowing) {
		t.Fatalf("expected ErrNotFollowing, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: GetFollowers / GetFollowing
// ---------------------------------------------------------------------------

func TestGetFollowers_Pagination(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	// user-B is followed by user-A, user-C, user-D.
	for _, follower := range []string{"user-A", "user-C", "user-D"} {
		if _, err := svc.Follow(ctx, follower, "user-B"); err != nil {
			t.Fatalf("setup follow (%s -> user-B): %v", follower, err)
		}
	}

	// First page: limit=2.
	resp, err := svc.GetFollowers(ctx, "user-B", 2, 0)
	if err != nil {
		t.Fatalf("get_followers: %v", err)
	}
	if int64(resp.Pagination.Total) != 3 {
		t.Errorf("expected total=3, got %d", resp.Pagination.Total)
	}
	if len(resp.Users) != 2 {
		t.Errorf("expected 2 users on first page, got %d", len(resp.Users))
	}
	if !resp.Pagination.HasMore {
		t.Error("expected HasMore=true")
	}

	// Second page: limit=2, offset=2.
	resp2, err := svc.GetFollowers(ctx, "user-B", 2, 2)
	if err != nil {
		t.Fatalf("get_followers page 2: %v", err)
	}
	if len(resp2.Users) != 1 {
		t.Errorf("expected 1 user on second page, got %d", len(resp2.Users))
	}
	if resp2.Pagination.HasMore {
		t.Error("expected HasMore=false on last page")
	}
}

func TestGetFollowing(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	if _, err := svc.Follow(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := svc.Follow(ctx, "user-A", "user-C"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	resp, err := svc.GetFollowing(ctx, "user-A", 20, 0)
	if err != nil {
		t.Fatalf("get_following: %v", err)
	}
	if int64(resp.Pagination.Total) != 2 {
		t.Errorf("expected total=2, got %d", resp.Pagination.Total)
	}
}

// ---------------------------------------------------------------------------
// Tests: GetMutualFollowers
// ---------------------------------------------------------------------------

func TestGetMutualFollowers(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	// user-C and user-D both follow user-A and user-B.
	for _, follower := range []string{"user-C", "user-D"} {
		for _, followee := range []string{"user-A", "user-B"} {
			if _, err := svc.Follow(ctx, follower, followee); err != nil {
				t.Fatalf("setup follow (%s -> %s): %v", follower, followee, err)
			}
		}
	}
	// user-E only follows user-A.
	if _, err := svc.Follow(ctx, "user-E", "user-A"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	resp, err := svc.GetMutualFollowers(ctx, "user-A", "user-B", 20, 0)
	if err != nil {
		t.Fatalf("get_mutual_followers: %v", err)
	}
	if int64(resp.Pagination.Total) != 2 {
		t.Errorf("expected 2 mutual followers, got %d", resp.Pagination.Total)
	}
}

// ---------------------------------------------------------------------------
// Tests: CheckRelationship
// ---------------------------------------------------------------------------

func TestCheckRelationship_Mutual(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	if _, err := svc.Follow(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := svc.Follow(ctx, "user-B", "user-A"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	rel, err := svc.CheckRelationship(ctx, "user-A", "user-B")
	if err != nil {
		t.Fatalf("check_relationship: %v", err)
	}
	if rel.Status != models.RelationshipMutual {
		t.Errorf("expected RelationshipMutual, got %s", rel.Status)
	}
	if !rel.IsFollowing {
		t.Error("expected IsFollowing=true")
	}
	if !rel.IsFollowedBy {
		t.Error("expected IsFollowedBy=true")
	}
}

func TestCheckRelationship_Blocked(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	if err := svc.BlockUser(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("block: %v", err)
	}

	rel, err := svc.CheckRelationship(ctx, "user-A", "user-B")
	if err != nil {
		t.Fatalf("check_relationship: %v", err)
	}
	if rel.Status != models.RelationshipBlocked {
		t.Errorf("expected RelationshipBlocked, got %s", rel.Status)
	}
}

func TestCheckRelationship_Self(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	rel, err := svc.CheckRelationship(context.Background(), "user-A", "user-A")
	if err != nil {
		t.Fatalf("check_relationship: %v", err)
	}
	if rel.Status != models.RelationshipNone {
		t.Errorf("expected RelationshipNone for self, got %s", rel.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: BlockUser / GetBlockList
// ---------------------------------------------------------------------------

func TestBlockUser_RemovesFollows(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	if _, err := svc.Follow(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("setup follow A->B: %v", err)
	}
	if _, err := svc.Follow(ctx, "user-B", "user-A"); err != nil {
		t.Fatalf("setup follow B->A: %v", err)
	}

	if err := svc.BlockUser(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("block failed: %v", err)
	}

	isFollowing, err := repo.IsFollowing(ctx, "user-A", "user-B")
	if err != nil {
		t.Fatalf("is_following: %v", err)
	}
	if isFollowing {
		t.Error("A->B follow should have been removed by block")
	}

	isFollowing, err = repo.IsFollowing(ctx, "user-B", "user-A")
	if err != nil {
		t.Fatalf("is_following: %v", err)
	}
	if isFollowing {
		t.Error("B->A follow should have been removed by block")
	}
}

func TestGetBlockList(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()
	svc := newPermissiveSocialService(t, repo, redisClient)

	ctx := context.Background()
	if err := svc.BlockUser(ctx, "user-A", "user-B"); err != nil {
		t.Fatalf("block: %v", err)
	}
	if err := svc.BlockUser(ctx, "user-A", "user-C"); err != nil {
		t.Fatalf("block: %v", err)
	}

	blocks, total, err := svc.GetBlockList(ctx, "user-A", 20, 0)
	if err != nil {
		t.Fatalf("get_block_list: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 blocks, got %d", total)
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 block records, got %d", len(blocks))
	}
}

// ---------------------------------------------------------------------------
// Tests: GetFriendSuggestions (BFS)
// ---------------------------------------------------------------------------

// buildSuggestionGraph creates a follow graph for BFS testing:
//
//	viewer -> A, B          (depth 1: already following)
//	A -> C, D               (depth 2: candidates)
//	B -> C, E               (depth 2: C is suggested by both A and B)
//
// Expected suggestions: C (mutual count 2), D (mutual count 1), E (mutual count 1).
func buildSuggestionGraph(t *testing.T, repo *fakeGraphRepo) {
	t.Helper()
	ctx := context.Background()
	edges := [][2]string{
		{"viewer", "user-A"},
		{"viewer", "user-B"},
		{"user-A", "user-C"},
		{"user-A", "user-D"},
		{"user-B", "user-C"},
		{"user-B", "user-E"},
	}
	for _, e := range edges {
		if _, err := repo.Follow(ctx, e[0], e[1]); err != nil {
			t.Fatalf("setup edge %s->%s: %v", e[0], e[1], err)
		}
	}
}

func TestGetFriendSuggestions_BFS(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()

	buildSuggestionGraph(t, repo)

	logger, _ := zap.NewDevelopment()
	suggestionSvc := services.NewSuggestionService(repo, redisClient, logger, 20, 2, 10*time.Minute)
	suggestions, err := suggestionSvc.GetSuggestions(context.Background(), "viewer")
	if err != nil {
		t.Fatalf("get_suggestions: %v", err)
	}

	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// user-C should be the top suggestion (mutual count 2).
	top := suggestions[0]
	if top.User.UserID != "user-C" {
		t.Errorf("expected top suggestion to be user-C (mutual count 2), got %s (mutual count %d)",
			top.User.UserID, top.MutualFollowerCount)
	}
	if top.MutualFollowerCount != 2 {
		t.Errorf("expected MutualFollowerCount=2 for user-C, got %d", top.MutualFollowerCount)
	}

	// viewer, user-A, user-B should NOT appear in suggestions.
	excluded := map[string]bool{"viewer": true, "user-A": true, "user-B": true}
	for _, s := range suggestions {
		if excluded[s.User.UserID] {
			t.Errorf("suggestion list should not include %s", s.User.UserID)
		}
	}
}

func TestGetFriendSuggestions_ExcludesBlocked(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()

	buildSuggestionGraph(t, repo)

	// viewer blocks user-C so it should not appear in suggestions.
	ctx := context.Background()
	if err := repo.BlockUser(ctx, "viewer", "user-C"); err != nil {
		t.Fatalf("block: %v", err)
	}

	logger, _ := zap.NewDevelopment()
	suggestionSvc := services.NewSuggestionService(repo, redisClient, logger, 20, 2, 10*time.Minute)
	suggestions, err := suggestionSvc.GetSuggestions(ctx, "viewer")
	if err != nil {
		t.Fatalf("get_suggestions: %v", err)
	}

	for _, s := range suggestions {
		if s.User.UserID == "user-C" {
			t.Error("blocked user user-C should not appear in suggestions")
		}
	}
}

func TestGetFriendSuggestions_EmptyGraph(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()

	logger, _ := zap.NewDevelopment()
	suggestionSvc := services.NewSuggestionService(repo, redisClient, logger, 20, 2, 10*time.Minute)
	suggestions, err := suggestionSvc.GetSuggestions(context.Background(), "viewer")
	if err != nil {
		t.Fatalf("get_suggestions: %v", err)
	}
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for isolated user, got %d", len(suggestions))
	}
}

func TestGetFriendSuggestions_Caching(t *testing.T) {
	redisClient := newTestRedis(t)
	repo := newFakeGraphRepo()

	buildSuggestionGraph(t, repo)

	logger, _ := zap.NewDevelopment()
	suggestionSvc := services.NewSuggestionService(repo, redisClient, logger, 20, 2, 10*time.Minute)
	ctx := context.Background()

	// First call — computes and caches.
	first, err := suggestionSvc.GetSuggestions(ctx, "viewer")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Give the goroutine that saves to cache a moment to complete.
	time.Sleep(50 * time.Millisecond)

	// Second call — should be served from cache. We verify by inspecting the
	// Redis key directly.
	cacheKey := "social:suggestions:viewer"
	data, err := redisClient.Get(ctx, cacheKey).Bytes()
	if err != nil {
		t.Fatalf("cache key not found after first call: %v", err)
	}

	var cached []models.FriendSuggestion
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatalf("failed to unmarshal cached suggestions: %v", err)
	}
	if len(cached) != len(first) {
		t.Errorf("cached length %d != computed length %d", len(cached), len(first))
	}

	// Invalidation: call InvalidateSuggestionsCache and verify the key is gone.
	if err := suggestionSvc.InvalidateSuggestionsCache(ctx, "viewer"); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	exists, _ := redisClient.Exists(ctx, cacheKey).Result()
	if exists != 0 {
		t.Error("cache key should not exist after invalidation")
	}
}

// ---------------------------------------------------------------------------
// Tests: FollowEvent model
// ---------------------------------------------------------------------------

func TestFollowEvent_JSONRoundTrip(t *testing.T) {
	event := models.FollowEvent{
		EventType:  "followed",
		FollowerID: "user-A",
		FolloweeID: "user-B",
		OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded models.FollowEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.EventType != event.EventType {
		t.Errorf("EventType mismatch: %s != %s", decoded.EventType, event.EventType)
	}
	if decoded.FollowerID != event.FollowerID {
		t.Errorf("FollowerID mismatch")
	}
	if decoded.FolloweeID != event.FolloweeID {
		t.Errorf("FolloweeID mismatch")
	}
	if !decoded.OccurredAt.Equal(event.OccurredAt) {
		t.Errorf("OccurredAt mismatch: %v != %v", decoded.OccurredAt, event.OccurredAt)
	}
}

// ---------------------------------------------------------------------------
// Tests: Relationship model
// ---------------------------------------------------------------------------

func TestRelationship_DeriveStatus(t *testing.T) {
	cases := []struct {
		name     string
		rel      models.Relationship
		expected models.RelationshipStatus
	}{
		{
			name:     "blocking",
			rel:      models.Relationship{IsBlocking: true},
			expected: models.RelationshipBlocked,
		},
		{
			name:     "blocked by",
			rel:      models.Relationship{IsBlockedBy: true},
			expected: models.RelationshipBlockedBy,
		},
		{
			name:     "mutual",
			rel:      models.Relationship{IsFollowing: true, IsFollowedBy: true},
			expected: models.RelationshipMutual,
		},
		{
			name:     "following only",
			rel:      models.Relationship{IsFollowing: true},
			expected: models.RelationshipFollowing,
		},
		{
			name:     "followed by only",
			rel:      models.Relationship{IsFollowedBy: true},
			expected: models.RelationshipFollowedBy,
		},
		{
			name:     "none",
			rel:      models.Relationship{},
			expected: models.RelationshipNone,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.rel.DeriveStatus()
			if tc.rel.Status != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, tc.rel.Status)
			}
		})
	}
}
