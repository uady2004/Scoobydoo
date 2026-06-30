package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/user-service/internal/models"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

// ErrDuplicateUsername is returned when a username is already taken.
var ErrDuplicateUsername = errors.New("username already taken")

// ProfileRepository defines all database operations for user profiles.
type ProfileRepository interface {
	// Profile CRUD.
	CreateProfile(ctx context.Context, profile *models.UserProfile) error
	GetProfileByID(ctx context.Context, id uuid.UUID) (*models.UserProfile, error)
	GetProfileByUserID(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error)
	GetProfileByUsername(ctx context.Context, username string) (*models.UserProfile, error)
	UpdateProfile(ctx context.Context, profile *models.UserProfile) error
	SoftDeleteProfile(ctx context.Context, id uuid.UUID) error

	// Privacy settings.
	CreatePrivacySettings(ctx context.Context, settings *models.PrivacySettings) error
	GetPrivacySettings(ctx context.Context, profileID uuid.UUID) (*models.PrivacySettings, error)
	UpdatePrivacySettings(ctx context.Context, settings *models.PrivacySettings) error

	// Creator profile.
	CreateCreatorProfile(ctx context.Context, cp *models.CreatorProfile) error
	GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*models.CreatorProfile, error)
	UpdateCreatorProfile(ctx context.Context, cp *models.CreatorProfile) error

	// Verification badges.
	CreateVerificationBadge(ctx context.Context, badge *models.VerificationBadge) error
	GetActiveVerificationBadge(ctx context.Context, profileID uuid.UUID) (*models.VerificationBadge, error)
	RevokeVerificationBadge(ctx context.Context, badgeID, revokedByID uuid.UUID, reason string) error

	// Counter operations (atomic increments/decrements).
	IncrementFollowerCount(ctx context.Context, profileID uuid.UUID, delta int64) error
	IncrementFollowingCount(ctx context.Context, profileID uuid.UUID, delta int64) error
	IncrementVideoCount(ctx context.Context, profileID uuid.UUID, delta int64) error
	IncrementLikeCount(ctx context.Context, profileID uuid.UUID, delta int64) error
	GetFollowerCount(ctx context.Context, profileID uuid.UUID) (int64, error)
	GetFollowingCount(ctx context.Context, profileID uuid.UUID) (int64, error)

	// Block records.
	CreateBlockRecord(ctx context.Context, block *models.BlockRecord) error
	DeleteBlockRecord(ctx context.Context, blockerID, blockedID uuid.UUID) error
	IsBlocked(ctx context.Context, blockerID, blockedID uuid.UUID) (bool, error)
	GetBlockedUsers(ctx context.Context, blockerID uuid.UUID, limit, offset int) ([]*models.BlockRecord, error)

	// Analytics.
	GetAccountAnalytics(ctx context.Context, profileID uuid.UUID, window time.Duration) (*models.AccountAnalytics, error)
	UpsertCreatorAnalytics(ctx context.Context, analytics *models.AccountAnalytics) error

	// Search.
	SearchProfiles(ctx context.Context, query string, limit, offset int) ([]*models.UserProfile, int64, error)

	// Bulk fetch helpers.
	GetProfilesByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.UserProfile, error)
}

type pgProfileRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewProfileRepository returns a PostgreSQL-backed ProfileRepository.
func NewProfileRepository(pool *pgxpool.Pool, logger *zap.Logger) ProfileRepository {
	return &pgProfileRepository{pool: pool, logger: logger}
}

// ---------- Profile CRUD ----------

func (r *pgProfileRepository) CreateProfile(ctx context.Context, p *models.UserProfile) error {
	const q = `
		INSERT INTO user_profiles (
			id, user_id, username, display_name, bio, avatar_url, avatar_key,
			website_url, location, email, phone_number,
			account_status, is_creator, is_verified, verification_tier,
			follower_count, following_count, like_count, video_count,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
		)`

	_, err := r.pool.Exec(ctx, q,
		p.ID, p.UserID, p.Username, p.DisplayName, p.Bio, p.AvatarURL, p.AvatarKey,
		p.WebsiteURL, p.Location, p.Email, p.PhoneNumber,
		p.AccountStatus, p.IsCreator, p.IsVerified, p.VerificationTier,
		p.FollowerCount, p.FollowingCount, p.LikeCount, p.VideoCount,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if isDuplicateError(err) {
			return ErrDuplicateUsername
		}
		return fmt.Errorf("create profile: %w", err)
	}
	return nil
}

func (r *pgProfileRepository) GetProfileByID(ctx context.Context, id uuid.UUID) (*models.UserProfile, error) {
	const q = `
		SELECT id, user_id, username, display_name, bio, avatar_url, avatar_key,
		       website_url, location, email, phone_number,
		       account_status, is_creator, is_verified, verification_tier,
		       follower_count, following_count, like_count, video_count,
		       created_at, updated_at, deleted_at
		FROM user_profiles
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	return scanProfile(row)
}

func (r *pgProfileRepository) GetProfileByUserID(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error) {
	const q = `
		SELECT id, user_id, username, display_name, bio, avatar_url, avatar_key,
		       website_url, location, email, phone_number,
		       account_status, is_creator, is_verified, verification_tier,
		       follower_count, following_count, like_count, video_count,
		       created_at, updated_at, deleted_at
		FROM user_profiles
		WHERE user_id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, userID)
	return scanProfile(row)
}

func (r *pgProfileRepository) GetProfileByUsername(ctx context.Context, username string) (*models.UserProfile, error) {
	const q = `
		SELECT id, user_id, username, display_name, bio, avatar_url, avatar_key,
		       website_url, location, email, phone_number,
		       account_status, is_creator, is_verified, verification_tier,
		       follower_count, following_count, like_count, video_count,
		       created_at, updated_at, deleted_at
		FROM user_profiles
		WHERE username = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, username)
	return scanProfile(row)
}

func (r *pgProfileRepository) UpdateProfile(ctx context.Context, p *models.UserProfile) error {
	const q = `
		UPDATE user_profiles SET
			display_name      = $2,
			bio               = $3,
			avatar_url        = $4,
			avatar_key        = $5,
			website_url       = $6,
			location          = $7,
			account_status    = $8,
			is_creator        = $9,
			is_verified       = $10,
			verification_tier = $11,
			follower_count    = $12,
			following_count   = $13,
			like_count        = $14,
			video_count       = $15,
			updated_at        = $16
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q,
		p.ID, p.DisplayName, p.Bio, p.AvatarURL, p.AvatarKey,
		p.WebsiteURL, p.Location, p.AccountStatus,
		p.IsCreator, p.IsVerified, p.VerificationTier,
		p.FollowerCount, p.FollowingCount, p.LikeCount, p.VideoCount,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgProfileRepository) SoftDeleteProfile(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE user_profiles SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("soft delete profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- Privacy settings ----------

func (r *pgProfileRepository) CreatePrivacySettings(ctx context.Context, s *models.PrivacySettings) error {
	const q = `
		INSERT INTO privacy_settings (
			id, user_id, profile_id,
			profile_visibility, video_visibility, liked_videos_visible,
			followers_list_visible, following_list_visible,
			allow_comments, allow_duet, allow_stitch, allow_download,
			allow_direct_messages, allow_tagging,
			notify_on_follow, notify_on_like, notify_on_comment,
			notify_on_mention, notify_on_share,
			filter_spam, filter_offensive, restricted_mode,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24
		)`
	_, err := r.pool.Exec(ctx, q,
		s.ID, s.UserID, s.ProfileID,
		s.ProfileVisibility, s.VideoVisibility, s.LikedVideosVisible,
		s.FollowersListVisible, s.FollowingListVisible,
		s.AllowComments, s.AllowDuet, s.AllowStitch, s.AllowDownload,
		s.AllowDirectMessages, s.AllowTagging,
		s.NotifyOnFollow, s.NotifyOnLike, s.NotifyOnComment,
		s.NotifyOnMention, s.NotifyOnShare,
		s.FilterSpam, s.FilterOffensive, s.RestrictedMode,
		s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create privacy settings: %w", err)
	}
	return nil
}

func (r *pgProfileRepository) GetPrivacySettings(ctx context.Context, profileID uuid.UUID) (*models.PrivacySettings, error) {
	const q = `
		SELECT id, user_id, profile_id,
		       profile_visibility, video_visibility, liked_videos_visible,
		       followers_list_visible, following_list_visible,
		       allow_comments, allow_duet, allow_stitch, allow_download,
		       allow_direct_messages, allow_tagging,
		       notify_on_follow, notify_on_like, notify_on_comment,
		       notify_on_mention, notify_on_share,
		       filter_spam, filter_offensive, restricted_mode,
		       created_at, updated_at
		FROM privacy_settings
		WHERE profile_id = $1`

	row := r.pool.QueryRow(ctx, q, profileID)
	s := &models.PrivacySettings{}
	err := row.Scan(
		&s.ID, &s.UserID, &s.ProfileID,
		&s.ProfileVisibility, &s.VideoVisibility, &s.LikedVideosVisible,
		&s.FollowersListVisible, &s.FollowingListVisible,
		&s.AllowComments, &s.AllowDuet, &s.AllowStitch, &s.AllowDownload,
		&s.AllowDirectMessages, &s.AllowTagging,
		&s.NotifyOnFollow, &s.NotifyOnLike, &s.NotifyOnComment,
		&s.NotifyOnMention, &s.NotifyOnShare,
		&s.FilterSpam, &s.FilterOffensive, &s.RestrictedMode,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get privacy settings: %w", err)
	}
	return s, nil
}

func (r *pgProfileRepository) UpdatePrivacySettings(ctx context.Context, s *models.PrivacySettings) error {
	const q = `
		UPDATE privacy_settings SET
			profile_visibility     = $2,
			video_visibility       = $3,
			liked_videos_visible   = $4,
			followers_list_visible = $5,
			following_list_visible = $6,
			allow_comments         = $7,
			allow_duet             = $8,
			allow_stitch           = $9,
			allow_download         = $10,
			allow_direct_messages  = $11,
			allow_tagging          = $12,
			notify_on_follow       = $13,
			notify_on_like         = $14,
			notify_on_comment      = $15,
			notify_on_mention      = $16,
			notify_on_share        = $17,
			filter_spam            = $18,
			filter_offensive       = $19,
			restricted_mode        = $20,
			updated_at             = $21
		WHERE profile_id = $1`

	tag, err := r.pool.Exec(ctx, q,
		s.ProfileID,
		s.ProfileVisibility, s.VideoVisibility, s.LikedVideosVisible,
		s.FollowersListVisible, s.FollowingListVisible,
		s.AllowComments, s.AllowDuet, s.AllowStitch, s.AllowDownload,
		s.AllowDirectMessages, s.AllowTagging,
		s.NotifyOnFollow, s.NotifyOnLike, s.NotifyOnComment,
		s.NotifyOnMention, s.NotifyOnShare,
		s.FilterSpam, s.FilterOffensive, s.RestrictedMode,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("update privacy settings: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- Creator profile ----------

func (r *pgProfileRepository) CreateCreatorProfile(ctx context.Context, cp *models.CreatorProfile) error {
	const q = `
		INSERT INTO creator_profiles (
			id, user_id, profile_id, category, sub_categories,
			is_monetised, creator_fund_enabled, tip_enabled, minimum_tip_amount,
			business_name, business_contact,
			total_views, avg_views_per_video, engagement_rate,
			profile_view_count, share_count, comment_count,
			creator_since, verified_at, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
		)`
	_, err := r.pool.Exec(ctx, q,
		cp.ID, cp.UserID, cp.ProfileID, cp.Category, cp.SubCategories,
		cp.IsMonetised, cp.CreatorFundEnabled, cp.TipEnabled, cp.MinimumTipAmount,
		cp.BusinessName, cp.BusinessContact,
		cp.TotalViews, cp.AvgViewsPerVideo, cp.EngagementRate,
		cp.ProfileViewCount, cp.ShareCount, cp.CommentCount,
		cp.CreatorSince, cp.VerifiedAt, cp.CreatedAt, cp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create creator profile: %w", err)
	}
	return nil
}

func (r *pgProfileRepository) GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*models.CreatorProfile, error) {
	const q = `
		SELECT id, user_id, profile_id, category, sub_categories,
		       is_monetised, creator_fund_enabled, tip_enabled, minimum_tip_amount,
		       business_name, business_contact,
		       total_views, avg_views_per_video, engagement_rate,
		       profile_view_count, share_count, comment_count,
		       creator_since, verified_at, created_at, updated_at
		FROM creator_profiles
		WHERE user_id = $1`

	row := r.pool.QueryRow(ctx, q, userID)
	cp := &models.CreatorProfile{}
	err := row.Scan(
		&cp.ID, &cp.UserID, &cp.ProfileID, &cp.Category, &cp.SubCategories,
		&cp.IsMonetised, &cp.CreatorFundEnabled, &cp.TipEnabled, &cp.MinimumTipAmount,
		&cp.BusinessName, &cp.BusinessContact,
		&cp.TotalViews, &cp.AvgViewsPerVideo, &cp.EngagementRate,
		&cp.ProfileViewCount, &cp.ShareCount, &cp.CommentCount,
		&cp.CreatorSince, &cp.VerifiedAt, &cp.CreatedAt, &cp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get creator profile: %w", err)
	}
	return cp, nil
}

func (r *pgProfileRepository) UpdateCreatorProfile(ctx context.Context, cp *models.CreatorProfile) error {
	const q = `
		UPDATE creator_profiles SET
			category              = $2,
			sub_categories        = $3,
			is_monetised          = $4,
			creator_fund_enabled  = $5,
			tip_enabled           = $6,
			minimum_tip_amount    = $7,
			business_name         = $8,
			business_contact      = $9,
			total_views           = $10,
			avg_views_per_video   = $11,
			engagement_rate       = $12,
			profile_view_count    = $13,
			share_count           = $14,
			comment_count         = $15,
			updated_at            = $16
		WHERE user_id = $1`

	tag, err := r.pool.Exec(ctx, q,
		cp.UserID,
		cp.Category, cp.SubCategories,
		cp.IsMonetised, cp.CreatorFundEnabled, cp.TipEnabled, cp.MinimumTipAmount,
		cp.BusinessName, cp.BusinessContact,
		cp.TotalViews, cp.AvgViewsPerVideo, cp.EngagementRate,
		cp.ProfileViewCount, cp.ShareCount, cp.CommentCount,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("update creator profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- Verification ----------

func (r *pgProfileRepository) CreateVerificationBadge(ctx context.Context, b *models.VerificationBadge) error {
	const q = `
		INSERT INTO verification_badges (
			id, user_id, profile_id, tier, granted_by_id, reason, document_ref,
			granted_at, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.pool.Exec(ctx, q,
		b.ID, b.UserID, b.ProfileID, b.Tier, b.GrantedByID, b.Reason, b.DocumentRef,
		b.GrantedAt, b.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create verification badge: %w", err)
	}
	return nil
}

func (r *pgProfileRepository) GetActiveVerificationBadge(ctx context.Context, profileID uuid.UUID) (*models.VerificationBadge, error) {
	const q = `
		SELECT id, user_id, profile_id, tier, granted_by_id, reason, document_ref,
		       granted_at, expires_at, revoked_at, revoked_by_id, revoke_reason
		FROM verification_badges
		WHERE profile_id = $1
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY granted_at DESC
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, profileID)
	b := &models.VerificationBadge{}
	err := row.Scan(
		&b.ID, &b.UserID, &b.ProfileID, &b.Tier, &b.GrantedByID, &b.Reason, &b.DocumentRef,
		&b.GrantedAt, &b.ExpiresAt, &b.RevokedAt, &b.RevokedByID, &b.RevokeReason,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get active verification badge: %w", err)
	}
	return b, nil
}

func (r *pgProfileRepository) RevokeVerificationBadge(ctx context.Context, badgeID, revokedByID uuid.UUID, reason string) error {
	const q = `
		UPDATE verification_badges
		SET revoked_at = $3, revoked_by_id = $4, revoke_reason = $5
		WHERE id = $1 AND revoked_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, badgeID, badgeID, time.Now().UTC(), revokedByID, reason)
	if err != nil {
		return fmt.Errorf("revoke verification badge: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- Counter operations ----------

func (r *pgProfileRepository) IncrementFollowerCount(ctx context.Context, profileID uuid.UUID, delta int64) error {
	return r.incrementCounter(ctx, profileID, "follower_count", delta)
}

func (r *pgProfileRepository) IncrementFollowingCount(ctx context.Context, profileID uuid.UUID, delta int64) error {
	return r.incrementCounter(ctx, profileID, "following_count", delta)
}

func (r *pgProfileRepository) IncrementVideoCount(ctx context.Context, profileID uuid.UUID, delta int64) error {
	return r.incrementCounter(ctx, profileID, "video_count", delta)
}

func (r *pgProfileRepository) IncrementLikeCount(ctx context.Context, profileID uuid.UUID, delta int64) error {
	return r.incrementCounter(ctx, profileID, "like_count", delta)
}

// incrementCounter is a generic helper that atomically adjusts a counter column.
// delta may be negative to decrement.
func (r *pgProfileRepository) incrementCounter(ctx context.Context, profileID uuid.UUID, col string, delta int64) error {
	// Allowlist the column name to prevent SQL injection.
	allowed := map[string]bool{
		"follower_count":  true,
		"following_count": true,
		"video_count":     true,
		"like_count":      true,
	}
	if !allowed[col] {
		return fmt.Errorf("unknown counter column: %s", col)
	}
	q := fmt.Sprintf(
		`UPDATE user_profiles SET %s = GREATEST(0, %s + $2), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
		col, col,
	)
	tag, err := r.pool.Exec(ctx, q, profileID, delta)
	if err != nil {
		return fmt.Errorf("increment %s: %w", col, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgProfileRepository) GetFollowerCount(ctx context.Context, profileID uuid.UUID) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT follower_count FROM user_profiles WHERE id = $1 AND deleted_at IS NULL`,
		profileID,
	).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("get follower count: %w", err)
	}
	return count, nil
}

func (r *pgProfileRepository) GetFollowingCount(ctx context.Context, profileID uuid.UUID) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT following_count FROM user_profiles WHERE id = $1 AND deleted_at IS NULL`,
		profileID,
	).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("get following count: %w", err)
	}
	return count, nil
}

// ---------- Block records ----------

func (r *pgProfileRepository) CreateBlockRecord(ctx context.Context, b *models.BlockRecord) error {
	const q = `
		INSERT INTO block_records (id, blocker_id, blocked_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING`
	_, err := r.pool.Exec(ctx, q, b.ID, b.BlockerID, b.BlockedID, b.CreatedAt)
	if err != nil {
		return fmt.Errorf("create block record: %w", err)
	}
	return nil
}

func (r *pgProfileRepository) DeleteBlockRecord(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	const q = `DELETE FROM block_records WHERE blocker_id = $1 AND blocked_id = $2`
	tag, err := r.pool.Exec(ctx, q, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("delete block record: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgProfileRepository) IsBlocked(ctx context.Context, blockerID, blockedID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM block_records WHERE blocker_id = $1 AND blocked_id = $2)`,
		blockerID, blockedID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is blocked: %w", err)
	}
	return exists, nil
}

func (r *pgProfileRepository) GetBlockedUsers(ctx context.Context, blockerID uuid.UUID, limit, offset int) ([]*models.BlockRecord, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, blocker_id, blocked_id, created_at FROM block_records WHERE blocker_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		blockerID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get blocked users: %w", err)
	}
	defer rows.Close()

	var records []*models.BlockRecord
	for rows.Next() {
		b := &models.BlockRecord{}
		if err := rows.Scan(&b.ID, &b.BlockerID, &b.BlockedID, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan block record: %w", err)
		}
		records = append(records, b)
	}
	return records, rows.Err()
}

// ---------- Analytics ----------

func (r *pgProfileRepository) GetAccountAnalytics(ctx context.Context, profileID uuid.UUID, window time.Duration) (*models.AccountAnalytics, error) {
	windowStart := time.Now().UTC().Add(-window)

	// This query aggregates counters directly from the profile and creator tables.
	// In a real system, the detailed event data would live in a separate analytics
	// store (ClickHouse / Redshift); here we approximate from denormalised counters.
	const q = `
		SELECT
			up.user_id,
			up.id               AS profile_id,
			up.follower_count   AS total_followers,
			up.like_count       AS total_likes,
			up.video_count,
			COALESCE(cp.total_views, 0)         AS total_views,
			COALESCE(cp.profile_view_count, 0)  AS profile_views,
			COALESCE(cp.share_count, 0)         AS total_shares,
			COALESCE(cp.comment_count, 0)       AS total_comments,
			COALESCE(cp.avg_views_per_video, 0) AS avg_views_per_video,
			COALESCE(cp.engagement_rate, 0)     AS engagement_rate
		FROM user_profiles up
		LEFT JOIN creator_profiles cp ON cp.profile_id = up.id
		WHERE up.id = $1 AND up.deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, profileID)

	a := &models.AccountAnalytics{
		WindowStart: windowStart,
		WindowEnd:   time.Now().UTC(),
		ComputedAt:  time.Now().UTC(),
	}
	err := row.Scan(
		&a.UserID, &a.ProfileID,
		&a.TotalFollowers, &a.TotalLikes,
		&a.VideoCount, &a.TotalViews, &a.ProfileViews,
		&a.TotalShares, &a.TotalComments,
		&a.AvgViewsPerVideo, &a.EngagementRate,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get account analytics: %w", err)
	}

	// Recalculate engagement rate from raw counters to ensure consistency.
	if a.TotalViews > 0 {
		interactions := a.TotalLikes + a.TotalComments + a.TotalShares
		a.EngagementRate = float64(interactions) / float64(a.TotalViews) * 100
	}
	if a.VideoCount > 0 {
		a.AvgViewsPerVideo = float64(a.TotalViews) / float64(a.VideoCount)
	}

	return a, nil
}

func (r *pgProfileRepository) UpsertCreatorAnalytics(ctx context.Context, a *models.AccountAnalytics) error {
	const q = `
		UPDATE creator_profiles SET
			total_views         = $2,
			avg_views_per_video = $3,
			engagement_rate     = $4,
			profile_view_count  = $5,
			share_count         = $6,
			comment_count       = $7,
			updated_at          = NOW()
		WHERE profile_id = $1`

	_, err := r.pool.Exec(ctx, q,
		a.ProfileID,
		a.TotalViews, a.AvgViewsPerVideo, a.EngagementRate,
		a.ProfileViews, a.TotalShares, a.TotalComments,
	)
	if err != nil {
		return fmt.Errorf("upsert creator analytics: %w", err)
	}
	return nil
}

// ---------- Search ----------

func (r *pgProfileRepository) SearchProfiles(ctx context.Context, query string, limit, offset int) ([]*models.UserProfile, int64, error) {
	// Full-text search using PostgreSQL's tsvector / tsquery with a fallback ILIKE.
	const countQ = `
		SELECT COUNT(*)
		FROM user_profiles
		WHERE deleted_at IS NULL
		  AND account_status = 'active'
		  AND (
		        to_tsvector('english', username || ' ' || display_name) @@ plainto_tsquery('english', $1)
		     OR username     ILIKE '%' || $1 || '%'
		     OR display_name ILIKE '%' || $1 || '%'
		  )`

	const selectQ = `
		SELECT id, user_id, username, display_name, bio, avatar_url, avatar_key,
		       website_url, location, email, phone_number,
		       account_status, is_creator, is_verified, verification_tier,
		       follower_count, following_count, like_count, video_count,
		       created_at, updated_at, deleted_at
		FROM user_profiles
		WHERE deleted_at IS NULL
		  AND account_status = 'active'
		  AND (
		        to_tsvector('english', username || ' ' || display_name) @@ plainto_tsquery('english', $1)
		     OR username     ILIKE '%' || $1 || '%'
		     OR display_name ILIKE '%' || $1 || '%'
		  )
		ORDER BY follower_count DESC, is_verified DESC
		LIMIT $2 OFFSET $3`

	var total int64
	if err := r.pool.QueryRow(ctx, countQ, query).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search profiles count: %w", err)
	}

	rows, err := r.pool.Query(ctx, selectQ, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search profiles: %w", err)
	}
	defer rows.Close()

	var profiles []*models.UserProfile
	for rows.Next() {
		p, err := scanProfileRow(rows)
		if err != nil {
			return nil, 0, err
		}
		profiles = append(profiles, p)
	}
	return profiles, total, rows.Err()
}

// ---------- Bulk fetch ----------

func (r *pgProfileRepository) GetProfilesByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.UserProfile, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, username, display_name, bio, avatar_url, avatar_key,
		        website_url, location, email, phone_number,
		        account_status, is_creator, is_verified, verification_tier,
		        follower_count, following_count, like_count, video_count,
		        created_at, updated_at, deleted_at
		 FROM user_profiles
		 WHERE id = ANY($1) AND deleted_at IS NULL`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("get profiles by ids: %w", err)
	}
	defer rows.Close()

	var profiles []*models.UserProfile
	for rows.Next() {
		p, err := scanProfileRow(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// ---------- scan helpers ----------

type scanner interface {
	Scan(dest ...any) error
}

func scanProfile(row pgx.Row) (*models.UserProfile, error) {
	p := &models.UserProfile{}
	err := row.Scan(
		&p.ID, &p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL, &p.AvatarKey,
		&p.WebsiteURL, &p.Location, &p.Email, &p.PhoneNumber,
		&p.AccountStatus, &p.IsCreator, &p.IsVerified, &p.VerificationTier,
		&p.FollowerCount, &p.FollowingCount, &p.LikeCount, &p.VideoCount,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan profile: %w", err)
	}
	return p, nil
}

func scanProfileRow(rows pgx.Rows) (*models.UserProfile, error) {
	p := &models.UserProfile{}
	err := rows.Scan(
		&p.ID, &p.UserID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL, &p.AvatarKey,
		&p.WebsiteURL, &p.Location, &p.Email, &p.PhoneNumber,
		&p.AccountStatus, &p.IsCreator, &p.IsVerified, &p.VerificationTier,
		&p.FollowerCount, &p.FollowingCount, &p.LikeCount, &p.VideoCount,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan profile row: %w", err)
	}
	return p, nil
}

// isDuplicateError detects PostgreSQL unique-constraint violations.
func isDuplicateError(err error) bool {
	return err != nil && (containsStr(err.Error(), "23505") || containsStr(err.Error(), "unique"))
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
