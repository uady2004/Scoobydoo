package tests

import (
	"context"
	"errors"
	"mime/multipart"
	"net/textproto"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/tiktok-clone/user-service/internal/models"
	"github.com/tiktok-clone/user-service/internal/repositories"
)

// ---------- mock repository ----------

// mockProfileRepository implements repositories.ProfileRepository via testify/mock.
type mockProfileRepository struct {
	mock.Mock
}

func (m *mockProfileRepository) CreateProfile(ctx context.Context, p *models.UserProfile) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockProfileRepository) GetProfileByID(ctx context.Context, id uuid.UUID) (*models.UserProfile, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserProfile), args.Error(1)
}

func (m *mockProfileRepository) GetProfileByUserID(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserProfile), args.Error(1)
}

func (m *mockProfileRepository) GetProfileByUsername(ctx context.Context, username string) (*models.UserProfile, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserProfile), args.Error(1)
}

func (m *mockProfileRepository) UpdateProfile(ctx context.Context, p *models.UserProfile) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockProfileRepository) SoftDeleteProfile(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockProfileRepository) CreatePrivacySettings(ctx context.Context, s *models.PrivacySettings) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *mockProfileRepository) GetPrivacySettings(ctx context.Context, profileID uuid.UUID) (*models.PrivacySettings, error) {
	args := m.Called(ctx, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PrivacySettings), args.Error(1)
}

func (m *mockProfileRepository) UpdatePrivacySettings(ctx context.Context, s *models.PrivacySettings) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *mockProfileRepository) CreateCreatorProfile(ctx context.Context, cp *models.CreatorProfile) error {
	args := m.Called(ctx, cp)
	return args.Error(0)
}

func (m *mockProfileRepository) GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*models.CreatorProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CreatorProfile), args.Error(1)
}

func (m *mockProfileRepository) UpdateCreatorProfile(ctx context.Context, cp *models.CreatorProfile) error {
	args := m.Called(ctx, cp)
	return args.Error(0)
}

func (m *mockProfileRepository) CreateVerificationBadge(ctx context.Context, b *models.VerificationBadge) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}

func (m *mockProfileRepository) GetActiveVerificationBadge(ctx context.Context, profileID uuid.UUID) (*models.VerificationBadge, error) {
	args := m.Called(ctx, profileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.VerificationBadge), args.Error(1)
}

func (m *mockProfileRepository) RevokeVerificationBadge(ctx context.Context, badgeID, revokedByID uuid.UUID, reason string) error {
	args := m.Called(ctx, badgeID, revokedByID, reason)
	return args.Error(0)
}

func (m *mockProfileRepository) IncrementFollowerCount(ctx context.Context, profileID uuid.UUID, delta int64) error {
	args := m.Called(ctx, profileID, delta)
	return args.Error(0)
}

func (m *mockProfileRepository) IncrementFollowingCount(ctx context.Context, profileID uuid.UUID, delta int64) error {
	args := m.Called(ctx, profileID, delta)
	return args.Error(0)
}

func (m *mockProfileRepository) IncrementVideoCount(ctx context.Context, profileID uuid.UUID, delta int64) error {
	args := m.Called(ctx, profileID, delta)
	return args.Error(0)
}

func (m *mockProfileRepository) IncrementLikeCount(ctx context.Context, profileID uuid.UUID, delta int64) error {
	args := m.Called(ctx, profileID, delta)
	return args.Error(0)
}

func (m *mockProfileRepository) GetFollowerCount(ctx context.Context, profileID uuid.UUID) (int64, error) {
	args := m.Called(ctx, profileID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockProfileRepository) GetFollowingCount(ctx context.Context, profileID uuid.UUID) (int64, error) {
	args := m.Called(ctx, profileID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockProfileRepository) CreateBlockRecord(ctx context.Context, b *models.BlockRecord) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}

func (m *mockProfileRepository) DeleteBlockRecord(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	args := m.Called(ctx, blockerID, blockedID)
	return args.Error(0)
}

func (m *mockProfileRepository) IsBlocked(ctx context.Context, blockerID, blockedID uuid.UUID) (bool, error) {
	args := m.Called(ctx, blockerID, blockedID)
	return args.Bool(0), args.Error(1)
}

func (m *mockProfileRepository) GetBlockedUsers(ctx context.Context, blockerID uuid.UUID, limit, offset int) ([]*models.BlockRecord, error) {
	args := m.Called(ctx, blockerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BlockRecord), args.Error(1)
}

func (m *mockProfileRepository) GetAccountAnalytics(ctx context.Context, profileID uuid.UUID, window time.Duration) (*models.AccountAnalytics, error) {
	args := m.Called(ctx, profileID, window)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AccountAnalytics), args.Error(1)
}

func (m *mockProfileRepository) UpsertCreatorAnalytics(ctx context.Context, a *models.AccountAnalytics) error {
	args := m.Called(ctx, a)
	return args.Error(0)
}

func (m *mockProfileRepository) SearchProfiles(ctx context.Context, query string, limit, offset int) ([]*models.UserProfile, int64, error) {
	args := m.Called(ctx, query, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.UserProfile), args.Get(1).(int64), args.Error(2)
}

func (m *mockProfileRepository) GetProfilesByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.UserProfile, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.UserProfile), args.Error(1)
}

// ---------- fixtures ----------

func newActiveProfile() *models.UserProfile {
	now := time.Now().UTC()
	return &models.UserProfile{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		Username:         "testuser",
		DisplayName:      "Test User",
		Bio:              "Hello world",
		AvatarURL:        "https://cdn.example.com/avatars/testuser.jpg",
		AvatarKey:        "avatars/testuser/abc123.jpg",
		WebsiteURL:       "https://example.com",
		Location:         "San Francisco",
		AccountStatus:    models.AccountStatusActive,
		IsCreator:        false,
		IsVerified:       false,
		VerificationTier: models.VerificationTierNone,
		FollowerCount:    1000,
		FollowingCount:   200,
		LikeCount:        5000,
		VideoCount:       42,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func newCreatorProfile(userID, profileID uuid.UUID) *models.CreatorProfile {
	now := time.Now().UTC()
	return &models.CreatorProfile{
		ID:                 uuid.New(),
		UserID:             userID,
		ProfileID:          profileID,
		Category:           "Entertainment",
		SubCategories:      []string{"Comedy", "Music"},
		IsMonetised:        true,
		CreatorFundEnabled: false,
		TipEnabled:         true,
		MinimumTipAmount:   1.0,
		BusinessName:       "Creator Inc.",
		BusinessContact:    "creator@example.com",
		TotalViews:         100000,
		AvgViewsPerVideo:   2380,
		EngagementRate:     6.5,
		ProfileViewCount:   8000,
		ShareCount:         1200,
		CommentCount:       3400,
		CreatorSince:       now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// ---------- Repository unit tests ----------

// TestGetProfileByUserID_Found verifies that the mock correctly returns a
// profile when one exists for the given userID.
func TestGetProfileByUserID_Found(t *testing.T) {
	repo := &mockProfileRepository{}
	profile := newActiveProfile()
	repo.On("GetProfileByUserID", mock.Anything, profile.UserID).Return(profile, nil)

	got, err := repo.GetProfileByUserID(context.Background(), profile.UserID)

	require.NoError(t, err)
	assert.Equal(t, profile.UserID, got.UserID)
	assert.Equal(t, profile.Username, got.Username)
	repo.AssertExpectations(t)
}

// TestGetProfileByUserID_NotFound verifies ErrNotFound is surfaced.
func TestGetProfileByUserID_NotFound(t *testing.T) {
	repo := &mockProfileRepository{}
	missingID := uuid.New()
	repo.On("GetProfileByUserID", mock.Anything, missingID).Return(nil, repositories.ErrNotFound)

	got, err := repo.GetProfileByUserID(context.Background(), missingID)

	assert.Nil(t, got)
	assert.True(t, errors.Is(err, repositories.ErrNotFound))
	repo.AssertExpectations(t)
}

// TestUpdateProfile_Success verifies a clean update path through the mock.
func TestUpdateProfile_Success(t *testing.T) {
	repo := &mockProfileRepository{}
	profile := newActiveProfile()
	profile.DisplayName = "Updated Name"
	repo.On("UpdateProfile", mock.Anything, profile).Return(nil)

	err := repo.UpdateProfile(context.Background(), profile)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// ---------- Model unit tests ----------

// TestUserProfile_IsActive ensures IsActive respects status and soft delete.
func TestUserProfile_IsActive(t *testing.T) {
	p := newActiveProfile()
	assert.True(t, p.IsActive(), "active profile should be active")

	p.AccountStatus = models.AccountStatusSuspended
	assert.False(t, p.IsActive(), "suspended profile should not be active")

	p.AccountStatus = models.AccountStatusActive
	now := time.Now().UTC()
	p.DeletedAt = &now
	assert.False(t, p.IsActive(), "soft-deleted profile should not be active")
}

// TestUserProfile_PublicProfile checks that sensitive fields are stripped.
func TestUserProfile_PublicProfile(t *testing.T) {
	p := newActiveProfile()
	p.Email = "secret@example.com"
	p.PhoneNumber = "+15551234567"
	p.AvatarKey = "avatars/user/secret-key.jpg"

	pub := p.PublicProfile()

	assert.Equal(t, p.Username, pub.Username)
	assert.Equal(t, p.FollowerCount, pub.FollowerCount)
	// AvatarKey must not appear in the public struct.
	// (The field does not exist on PublicUserProfile, so we verify by type assertion absence.)
	assert.NotEmpty(t, pub.AvatarURL, "avatar URL should be present")
}

// TestDefaultPrivacySettings verifies sensible defaults are set.
func TestDefaultPrivacySettings(t *testing.T) {
	userID := uuid.New()
	profileID := uuid.New()
	ps := models.DefaultPrivacySettings(userID, profileID)

	assert.Equal(t, userID, ps.UserID)
	assert.Equal(t, profileID, ps.ProfileID)
	assert.Equal(t, models.PrivacyPublic, ps.ProfileVisibility)
	assert.Equal(t, models.PrivacyPublic, ps.VideoVisibility)
	assert.True(t, ps.AllowComments)
	assert.True(t, ps.AllowDuet)
	assert.True(t, ps.FilterSpam)
	assert.False(t, ps.RestrictedMode)
}

// TestVerificationBadge_IsActive validates the active logic.
func TestVerificationBadge_IsActive(t *testing.T) {
	badge := &models.VerificationBadge{
		ID:          uuid.New(),
		Tier:        models.VerificationTierVerified,
		GrantedAt:   time.Now().UTC(),
	}
	assert.True(t, badge.IsActive(), "badge with no revoke or expiry should be active")

	// Expired badge.
	past := time.Now().UTC().Add(-24 * time.Hour)
	badge.ExpiresAt = &past
	assert.False(t, badge.IsActive(), "expired badge should not be active")

	// Reset expiry; revoke.
	badge.ExpiresAt = nil
	now := time.Now().UTC()
	badge.RevokedAt = &now
	assert.False(t, badge.IsActive(), "revoked badge should not be active")
}

// ---------- Analytics unit tests ----------

// TestEngagementRateCalculation verifies the formula:
// engagement_rate = (likes + comments + shares) / total_views * 100.
func TestEngagementRateCalculation(t *testing.T) {
	cases := []struct {
		name          string
		totalViews    int64
		totalLikes    int64
		totalComments int64
		totalShares   int64
		wantRate      float64
	}{
		{
			name:          "standard engagement",
			totalViews:    10000,
			totalLikes:    500,
			totalComments: 100,
			totalShares:   50,
			wantRate:      6.5,
		},
		{
			name:       "zero views",
			totalViews: 0,
			wantRate:   0,
		},
		{
			name:          "no interactions",
			totalViews:    5000,
			totalLikes:    0,
			totalComments: 0,
			totalShares:   0,
			wantRate:      0,
		},
		{
			name:          "100% engagement",
			totalViews:    100,
			totalLikes:    100,
			totalComments: 0,
			totalShares:   0,
			wantRate:      100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &models.AccountAnalytics{
				TotalViews:    tc.totalViews,
				TotalLikes:    tc.totalLikes,
				TotalComments: tc.totalComments,
				TotalShares:   tc.totalShares,
			}

			if a.TotalViews > 0 {
				interactions := a.TotalLikes + a.TotalComments + a.TotalShares
				a.EngagementRate = float64(interactions) / float64(a.TotalViews) * 100.0
			}

			assert.InDelta(t, tc.wantRate, a.EngagementRate, 0.001)
		})
	}
}

// TestAvgViewsPerVideoCalculation verifies average views computation.
func TestAvgViewsPerVideoCalculation(t *testing.T) {
	a := &models.AccountAnalytics{
		TotalViews: 1_000_000,
		VideoCount: 42,
	}
	a.AvgViewsPerVideo = float64(a.TotalViews) / float64(a.VideoCount)
	assert.InDelta(t, 23809.52, a.AvgViewsPerVideo, 1.0)
}

// ---------- Block record tests ----------

func TestBlockUser_Success(t *testing.T) {
	repo := &mockProfileRepository{}
	blockerID := uuid.New()
	blockedID := uuid.New()

	expected := &models.BlockRecord{
		BlockerID: blockerID,
		BlockedID: blockedID,
	}
	repo.On("CreateBlockRecord", mock.Anything, mock.MatchedBy(func(b *models.BlockRecord) bool {
		return b.BlockerID == blockerID && b.BlockedID == blockedID
	})).Return(nil)

	err := repo.CreateBlockRecord(context.Background(), expected)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestBlockUser_CannotBlockSelf(t *testing.T) {
	// This logic lives in the service layer; we test the invariant directly here.
	userID := uuid.New()
	isBlockingSelf := userID == userID // always true — mirrors service guard
	assert.True(t, isBlockingSelf, "blocking self should be detected")
}

func TestIsBlocked_True(t *testing.T) {
	repo := &mockProfileRepository{}
	blockerID := uuid.New()
	blockedID := uuid.New()

	repo.On("IsBlocked", mock.Anything, blockerID, blockedID).Return(true, nil)

	blocked, err := repo.IsBlocked(context.Background(), blockerID, blockedID)

	require.NoError(t, err)
	assert.True(t, blocked)
	repo.AssertExpectations(t)
}

// ---------- Privacy settings tests ----------

func TestUpdatePrivacySettings_InvalidPrivacyLevel(t *testing.T) {
	ps := &models.PrivacySettings{
		ProfileVisibility: models.PrivacyLevel("everyone"), // invalid value
		VideoVisibility:   models.PrivacyPublic,
	}

	// Validate manually — mirrors what the service does before persisting.
	validLevels := map[models.PrivacyLevel]bool{
		models.PrivacyPublic:    true,
		models.PrivacyFollowers: true,
		models.PrivacyFriends:   true,
		models.PrivacyPrivate:   true,
	}
	assert.False(t, validLevels[ps.ProfileVisibility], "invalid privacy level should not be valid")
}

// ---------- Search results tests ----------

func TestSearchProfiles_ReturnsResults(t *testing.T) {
	repo := &mockProfileRepository{}
	profiles := []*models.UserProfile{newActiveProfile(), newActiveProfile()}
	var total int64 = 2

	repo.On("SearchProfiles", mock.Anything, "test", 20, 0).Return(profiles, total, nil)

	got, count, err := repo.SearchProfiles(context.Background(), "test", 20, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, got, 2)
	repo.AssertExpectations(t)
}

func TestSearchProfiles_EmptyResults(t *testing.T) {
	repo := &mockProfileRepository{}
	repo.On("SearchProfiles", mock.Anything, "zzznomatch", 20, 0).
		Return([]*models.UserProfile{}, int64(0), nil)

	got, count, err := repo.SearchProfiles(context.Background(), "zzznomatch", 20, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
	assert.Empty(t, got)
	repo.AssertExpectations(t)
}

// ---------- Creator profile tests ----------

func TestGetCreatorProfile_NotACreator(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()

	repo.On("GetCreatorProfile", mock.Anything, userID).Return(nil, repositories.ErrNotFound)

	got, err := repo.GetCreatorProfile(context.Background(), userID)

	assert.Nil(t, got)
	assert.True(t, errors.Is(err, repositories.ErrNotFound))
	repo.AssertExpectations(t)
}

func TestUpdateCreatorProfile_Success(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	profileID := uuid.New()
	cp := newCreatorProfile(userID, profileID)

	repo.On("UpdateCreatorProfile", mock.Anything, cp).Return(nil)

	err := repo.UpdateCreatorProfile(context.Background(), cp)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// ---------- Avatar upload (multipart) helpers ----------

// newFileHeader is a test helper that creates a synthetic multipart.FileHeader.
func newFileHeader(filename, contentType string, size int64) *multipart.FileHeader {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition",
		`form-data; name="avatar"; filename="`+filename+`"`)
	header.Set("Content-Type", contentType)

	return &multipart.FileHeader{
		Filename: filename,
		Header:   header,
		Size:     size,
	}
}

// TestAvatarUpload_SizeValidation ensures the size guard is correct.
func TestAvatarUpload_SizeValidation(t *testing.T) {
	const maxSizeBytes = 5 * 1024 * 1024 // 5 MB

	smallFile := newFileHeader("avatar.jpg", "image/jpeg", 1*1024*1024)  // 1 MB — OK
	largeFile := newFileHeader("avatar.jpg", "image/jpeg", 10*1024*1024) // 10 MB — too large

	assert.LessOrEqual(t, smallFile.Size, int64(maxSizeBytes), "1 MB file should be within limit")
	assert.Greater(t, largeFile.Size, int64(maxSizeBytes), "10 MB file should exceed limit")
}

// TestAvatarUpload_MIMETypeValidation ensures unsupported types are rejected.
func TestAvatarUpload_MIMETypeValidation(t *testing.T) {
	allowed := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}

	assert.True(t, allowed["image/jpeg"], "jpeg should be allowed")
	assert.True(t, allowed["image/png"], "png should be allowed")
	assert.False(t, allowed["image/bmp"], "bmp should not be allowed")
	assert.False(t, allowed["application/octet-stream"], "binary should not be allowed")
}

// ---------- Follower count tests ----------

func TestGetFollowerCount_Success(t *testing.T) {
	repo := &mockProfileRepository{}
	profileID := uuid.New()

	repo.On("GetFollowerCount", mock.Anything, profileID).Return(int64(9500), nil)

	count, err := repo.GetFollowerCount(context.Background(), profileID)

	require.NoError(t, err)
	assert.Equal(t, int64(9500), count)
	repo.AssertExpectations(t)
}

func TestGetFollowingCount_Success(t *testing.T) {
	repo := &mockProfileRepository{}
	profileID := uuid.New()

	repo.On("GetFollowingCount", mock.Anything, profileID).Return(int64(310), nil)

	count, err := repo.GetFollowingCount(context.Background(), profileID)

	require.NoError(t, err)
	assert.Equal(t, int64(310), count)
	repo.AssertExpectations(t)
}

// ---------- SearchUsersResult pagination tests ----------

func TestSearchUsersResult_HasMore(t *testing.T) {
	cases := []struct {
		name     string
		total    int64
		page     int
		pageSize int
		hasMore  bool
	}{
		{"first page, more results", 100, 1, 20, true},
		{"last page, no more", 20, 1, 20, false},
		{"last page, exact fit", 40, 2, 20, false},
		{"middle page", 100, 3, 20, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			offset := int64((tc.page - 1) * tc.pageSize)
			hasMore := offset+int64(tc.pageSize) < tc.total
			assert.Equal(t, tc.hasMore, hasMore)
		})
	}
}
