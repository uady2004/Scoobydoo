package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/user-service/internal/config"
	"github.com/tiktok-clone/user-service/internal/models"
	"github.com/tiktok-clone/user-service/internal/repositories"
)

// ErrUnauthorized is returned when the caller does not have permission.
var ErrUnauthorized = errors.New("unauthorized")

// ErrInvalidInput is returned when input fails validation.
var ErrInvalidInput = errors.New("invalid input")

// ErrAvatarTooLarge is returned when an uploaded avatar exceeds the size limit.
var ErrAvatarTooLarge = errors.New("avatar file too large")

// ErrUnsupportedAvatarType is returned when the MIME type is not allowed.
var ErrUnsupportedAvatarType = errors.New("unsupported avatar mime type")

// ProfileService is the application-layer interface for all profile operations.
type ProfileService interface {
	// Profile read/write.
	GetProfile(ctx context.Context, userID uuid.UUID) (*models.PublicUserProfile, error)
	GetProfileByUsername(ctx context.Context, username string) (*models.PublicUserProfile, error)
	UpdateProfile(ctx context.Context, actorID uuid.UUID, update *models.UpdateProfile) (*models.PublicUserProfile, error)

	// Avatar upload flow.
	// InitiateAvatarUpload returns a presigned PUT URL; the client uploads directly.
	InitiateAvatarUpload(ctx context.Context, userID uuid.UUID, contentType string) (*models.AvatarUploadRequest, error)
	// ConfirmAvatarUpload is called after the client finishes uploading to update the profile record.
	ConfirmAvatarUpload(ctx context.Context, userID uuid.UUID, objectKey string) error
	// UploadAvatar streams the file through the API server into MinIO (alternative flow).
	UploadAvatar(ctx context.Context, userID uuid.UUID, fh *multipart.FileHeader) (string, error)

	// Privacy.
	GetPrivacySettings(ctx context.Context, userID uuid.UUID) (*models.PrivacySettings, error)
	UpdatePrivacySettings(ctx context.Context, userID uuid.UUID, settings *models.PrivacySettings) error

	// Analytics.
	GetAccountAnalytics(ctx context.Context, userID uuid.UUID) (*models.AccountAnalytics, error)

	// Creator.
	VerifyCreator(ctx context.Context, adminID, targetUserID uuid.UUID, tier models.VerificationTier, reason string) error
	GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*models.CreatorProfile, error)
	UpdateCreatorProfile(ctx context.Context, actorID uuid.UUID, cp *models.CreatorProfile) (*models.CreatorProfile, error)

	// Social counters.
	GetFollowerCount(ctx context.Context, userID uuid.UUID) (int64, error)
	GetFollowingCount(ctx context.Context, userID uuid.UUID) (int64, error)

	// Discovery.
	SearchUsers(ctx context.Context, query string, page, pageSize int) (*models.SearchUsersResult, error)

	// Blocking.
	BlockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error
	UnblockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error
}

type profileService struct {
	repo    repositories.ProfileRepository
	minioC  *minio.Client
	rdb     *redis.Client
	cfg     *config.Config
	logger  *zap.Logger
}

// NewProfileService constructs a ProfileService backed by PostgreSQL, Redis, and MinIO.
func NewProfileService(
	repo repositories.ProfileRepository,
	rdb *redis.Client,
	cfg *config.Config,
	logger *zap.Logger,
) (ProfileService, error) {
	mc, err := minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey, ""),
		Secure: cfg.S3.UseSSL,
		Region: cfg.S3.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("init minio client: %w", err)
	}

	return &profileService{
		repo:   repo,
		minioC: mc,
		rdb:    rdb,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// ---------- GetProfile ----------

func (s *profileService) GetProfile(ctx context.Context, userID uuid.UUID) (*models.PublicUserProfile, error) {
	cacheKey := fmt.Sprintf("profile:user:%s", userID)
	if cached, err := s.getCachedProfile(ctx, cacheKey); err == nil {
		return cached, nil
	}

	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	pub := p.PublicProfile()
	_ = s.cacheProfile(ctx, cacheKey, pub, s.cfg.Redis.ProfileTTL)
	return pub, nil
}

func (s *profileService) GetProfileByUsername(ctx context.Context, username string) (*models.PublicUserProfile, error) {
	cacheKey := fmt.Sprintf("profile:username:%s", strings.ToLower(username))
	if cached, err := s.getCachedProfile(ctx, cacheKey); err == nil {
		return cached, nil
	}

	p, err := s.repo.GetProfileByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get profile by username: %w", err)
	}

	pub := p.PublicProfile()
	_ = s.cacheProfile(ctx, cacheKey, pub, s.cfg.Redis.ProfileTTL)
	return pub, nil
}

// ---------- UpdateProfile ----------

func (s *profileService) UpdateProfile(ctx context.Context, actorID uuid.UUID, update *models.UpdateProfile) (*models.PublicUserProfile, error) {
	p, err := s.repo.GetProfileByUserID(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("update profile fetch: %w", err)
	}

	if update.DisplayName != nil {
		p.DisplayName = *update.DisplayName
	}
	if update.Bio != nil {
		p.Bio = *update.Bio
	}
	if update.WebsiteURL != nil {
		if err := validateURL(*update.WebsiteURL); err != nil {
			return nil, fmt.Errorf("%w: website_url: %v", ErrInvalidInput, err)
		}
		p.WebsiteURL = *update.WebsiteURL
	}
	if update.Location != nil {
		p.Location = *update.Location
	}

	if err := s.repo.UpdateProfile(ctx, p); err != nil {
		return nil, fmt.Errorf("update profile save: %w", err)
	}

	s.invalidateProfileCaches(ctx, p.UserID, p.Username)
	return p.PublicProfile(), nil
}

// ---------- Avatar upload (presigned URL flow) ----------

func (s *profileService) InitiateAvatarUpload(ctx context.Context, userID uuid.UUID, contentType string) (*models.AvatarUploadRequest, error) {
	if !s.isAllowedAvatarMIME(contentType) {
		return nil, ErrUnsupportedAvatarType
	}

	ext := mimeToExt(contentType)
	objectKey := fmt.Sprintf("%s/%s/%s%s",
		s.cfg.S3.AvatarPathPrefix,
		userID.String(),
		uuid.New().String(),
		ext,
	)

	expiry := s.cfg.S3.PresignedURLExpiry
	presignedURL, err := s.minioC.PresignedPutObject(ctx, s.cfg.S3.BucketName, objectKey, expiry)
	if err != nil {
		return nil, fmt.Errorf("generate presigned upload url: %w", err)
	}

	return &models.AvatarUploadRequest{
		UploadURL: presignedURL.String(),
		ObjectKey: objectKey,
		ExpiresAt: time.Now().UTC().Add(expiry),
		Headers: map[string]string{
			"Content-Type": contentType,
		},
	}, nil
}

func (s *profileService) ConfirmAvatarUpload(ctx context.Context, userID uuid.UUID, objectKey string) error {
	// Validate the object actually exists in MinIO before updating the profile.
	_, err := s.minioC.StatObject(ctx, s.cfg.S3.BucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("avatar object not found in storage: %w", err)
	}

	avatarURL := s.buildPublicURL(objectKey)

	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("confirm avatar upload: fetch profile: %w", err)
	}

	oldKey := p.AvatarKey
	p.AvatarURL = avatarURL
	p.AvatarKey = objectKey

	if err := s.repo.UpdateProfile(ctx, p); err != nil {
		return fmt.Errorf("confirm avatar upload: save profile: %w", err)
	}

	// Asynchronously delete the old avatar object to avoid orphaned files.
	if oldKey != "" && oldKey != objectKey {
		go func() {
			bgCtx := context.Background()
			if rmErr := s.minioC.RemoveObject(bgCtx, s.cfg.S3.BucketName, oldKey, minio.RemoveObjectOptions{}); rmErr != nil {
				s.logger.Warn("failed to delete old avatar", zap.String("key", oldKey), zap.Error(rmErr))
			}
		}()
	}

	s.invalidateProfileCaches(ctx, p.UserID, p.Username)
	return nil
}

// UploadAvatar handles the server-proxied multipart upload. The file is streamed
// directly to MinIO; the profile record is updated on success.
func (s *profileService) UploadAvatar(ctx context.Context, userID uuid.UUID, fh *multipart.FileHeader) (string, error) {
	if fh.Size > s.cfg.S3.AvatarMaxSizeBytes {
		return "", ErrAvatarTooLarge
	}

	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !s.isAllowedAvatarMIME(contentType) {
		return "", ErrUnsupportedAvatarType
	}

	src, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("open avatar file: %w", err)
	}
	defer func() { _ = src.(io.Closer).Close() }()

	ext := mimeToExt(contentType)
	objectKey := fmt.Sprintf("%s/%s/%s%s",
		s.cfg.S3.AvatarPathPrefix,
		userID.String(),
		uuid.New().String(),
		ext,
	)

	_, err = s.minioC.PutObject(ctx, s.cfg.S3.BucketName, objectKey, src, fh.Size,
		minio.PutObjectOptions{
			ContentType:  contentType,
			UserMetadata: map[string]string{"owner": userID.String()},
		},
	)
	if err != nil {
		return "", fmt.Errorf("upload avatar to minio: %w", err)
	}

	if err := s.ConfirmAvatarUpload(ctx, userID, objectKey); err != nil {
		return "", err
	}

	return s.buildPublicURL(objectKey), nil
}

// ---------- Privacy settings ----------

func (s *profileService) GetPrivacySettings(ctx context.Context, userID uuid.UUID) (*models.PrivacySettings, error) {
	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get privacy settings: fetch profile: %w", err)
	}

	settings, err := s.repo.GetPrivacySettings(ctx, p.ID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			// Auto-provision default settings for accounts that predate this feature.
			defaults := models.DefaultPrivacySettings(userID, p.ID)
			if createErr := s.repo.CreatePrivacySettings(ctx, defaults); createErr != nil {
				return nil, fmt.Errorf("provision default privacy settings: %w", createErr)
			}
			return defaults, nil
		}
		return nil, fmt.Errorf("get privacy settings: %w", err)
	}
	return settings, nil
}

func (s *profileService) UpdatePrivacySettings(ctx context.Context, userID uuid.UUID, settings *models.PrivacySettings) error {
	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("update privacy settings: fetch profile: %w", err)
	}
	settings.ProfileID = p.ID
	settings.UserID = userID

	if err := s.repo.UpdatePrivacySettings(ctx, settings); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			if settings.ID == uuid.Nil {
				settings.ID = uuid.New()
			}
			now := time.Now().UTC()
			settings.CreatedAt = now
			settings.UpdatedAt = now
			return s.repo.CreatePrivacySettings(ctx, settings)
		}
		return fmt.Errorf("update privacy settings: %w", err)
	}
	return nil
}

// ---------- Analytics ----------

func (s *profileService) GetAccountAnalytics(ctx context.Context, userID uuid.UUID) (*models.AccountAnalytics, error) {
	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get analytics: fetch profile: %w", err)
	}

	analytics, err := s.repo.GetAccountAnalytics(ctx, p.ID, s.cfg.App.EngagementWindow)
	if err != nil {
		return nil, fmt.Errorf("get account analytics: %w", err)
	}

	// Recompute engagement rate: (likes + comments + shares) / total_views * 100.
	if analytics.TotalViews > 0 {
		interactions := analytics.TotalLikes + analytics.TotalComments + analytics.TotalShares
		analytics.EngagementRate = float64(interactions) / float64(analytics.TotalViews) * 100.0
	}
	if analytics.VideoCount > 0 {
		analytics.AvgViewsPerVideo = float64(analytics.TotalViews) / float64(analytics.VideoCount)
	}

	return analytics, nil
}

// ---------- Creator ----------

func (s *profileService) VerifyCreator(ctx context.Context, adminID, targetUserID uuid.UUID, tier models.VerificationTier, reason string) error {
	p, err := s.repo.GetProfileByUserID(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("verify creator: fetch profile: %w", err)
	}

	badge := &models.VerificationBadge{
		ID:          uuid.New(),
		UserID:      targetUserID,
		ProfileID:   p.ID,
		Tier:        tier,
		GrantedByID: adminID,
		Reason:      reason,
		GrantedAt:   time.Now().UTC(),
	}
	if err := s.repo.CreateVerificationBadge(ctx, badge); err != nil {
		return fmt.Errorf("create verification badge: %w", err)
	}

	p.IsVerified = true
	p.VerificationTier = tier
	if err := s.repo.UpdateProfile(ctx, p); err != nil {
		return fmt.Errorf("update profile verification: %w", err)
	}

	s.invalidateProfileCaches(ctx, p.UserID, p.Username)
	return nil
}

func (s *profileService) GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*models.CreatorProfile, error) {
	return s.repo.GetCreatorProfile(ctx, userID)
}

func (s *profileService) UpdateCreatorProfile(ctx context.Context, actorID uuid.UUID, cp *models.CreatorProfile) (*models.CreatorProfile, error) {
	existing, err := s.repo.GetCreatorProfile(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("update creator profile: fetch: %w", err)
	}

	// Only update mutable fields; analytics counters are maintained by background jobs.
	existing.Category = cp.Category
	existing.SubCategories = cp.SubCategories
	existing.IsMonetised = cp.IsMonetised
	existing.CreatorFundEnabled = cp.CreatorFundEnabled
	existing.TipEnabled = cp.TipEnabled
	existing.MinimumTipAmount = cp.MinimumTipAmount
	existing.BusinessName = cp.BusinessName
	existing.BusinessContact = cp.BusinessContact

	if err := s.repo.UpdateCreatorProfile(ctx, existing); err != nil {
		return nil, fmt.Errorf("update creator profile: save: %w", err)
	}
	return existing, nil
}

// ---------- Social counters ----------

func (s *profileService) GetFollowerCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("get follower count: %w", err)
	}
	return s.repo.GetFollowerCount(ctx, p.ID)
}

func (s *profileService) GetFollowingCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("get following count: %w", err)
	}
	return s.repo.GetFollowingCount(ctx, p.ID)
}

// ---------- Search ----------

func (s *profileService) SearchUsers(ctx context.Context, query string, page, pageSize int) (*models.SearchUsersResult, error) {
	if query == "" {
		return nil, fmt.Errorf("%w: search query must not be empty", ErrInvalidInput)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > s.cfg.App.MaxSearchResults {
		pageSize = s.cfg.App.MaxSearchResults
	}
	offset := (page - 1) * pageSize

	profiles, total, err := s.repo.SearchProfiles(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}

	pubProfiles := make([]*models.PublicUserProfile, 0, len(profiles))
	for _, p := range profiles {
		pubProfiles = append(pubProfiles, p.PublicProfile())
	}

	return &models.SearchUsersResult{
		Users:    pubProfiles,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(offset+pageSize) < total,
	}, nil
}

// ---------- Blocking ----------

func (s *profileService) BlockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	if blockerID == blockedID {
		return fmt.Errorf("%w: cannot block yourself", ErrInvalidInput)
	}
	if _, err := s.repo.GetProfileByUserID(ctx, blockerID); err != nil {
		return fmt.Errorf("block user: fetch blocker: %w", err)
	}
	if _, err := s.repo.GetProfileByUserID(ctx, blockedID); err != nil {
		return fmt.Errorf("block user: fetch blocked: %w", err)
	}

	record := &models.BlockRecord{
		ID:        uuid.New(),
		BlockerID: blockerID,
		BlockedID: blockedID,
		CreatedAt: time.Now().UTC(),
	}
	return s.repo.CreateBlockRecord(ctx, record)
}

func (s *profileService) UnblockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	if err := s.repo.DeleteBlockRecord(ctx, blockerID, blockedID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil // idempotent
		}
		return fmt.Errorf("unblock user: %w", err)
	}
	return nil
}

// ---------- Cache helpers ----------

func (s *profileService) getCachedProfile(ctx context.Context, key string) (*models.PublicUserProfile, error) {
	val, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var p models.PublicUserProfile
	if err := json.Unmarshal(val, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *profileService) cacheProfile(ctx context.Context, key string, p *models.PublicUserProfile, ttl time.Duration) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key, b, ttl).Err()
}

func (s *profileService) invalidateProfileCaches(ctx context.Context, userID uuid.UUID, username string) {
	keys := []string{
		fmt.Sprintf("profile:user:%s", userID),
		fmt.Sprintf("profile:username:%s", strings.ToLower(username)),
	}
	for _, k := range keys {
		if err := s.rdb.Del(ctx, k).Err(); err != nil {
			s.logger.Warn("cache invalidation failed", zap.String("key", k), zap.Error(err))
		}
	}
}

// ---------- MinIO helpers ----------

func (s *profileService) buildPublicURL(objectKey string) string {
	scheme := "http"
	if s.cfg.S3.UseSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.cfg.S3.Endpoint, s.cfg.S3.BucketName, objectKey)
}

func (s *profileService) isAllowedAvatarMIME(contentType string) bool {
	for _, allowed := range s.cfg.S3.AllowedAvatarMIMETypes {
		if strings.EqualFold(contentType, allowed) {
			return true
		}
	}
	return false
}

// ---------- Utility ----------

func validateURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https")
	}
	return nil
}

func mimeToExt(mime string) string {
	m := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
		"image/gif":  ".gif",
	}
	if ext, ok := m[strings.ToLower(mime)]; ok {
		return ext
	}
	return ".bin"
}

// pathBase is kept as a utility in case callers need the object key basename.
func pathBase(key string) string { return path.Base(key) }

// Ensure multipart.File satisfies io.ReadCloser at compile time.
var _ io.ReadCloser = (multipart.File)(nil)
