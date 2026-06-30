package models

import (
	"time"

	"github.com/google/uuid"
)

// AccountStatus represents the current state of a user account.
type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusBanned    AccountStatus = "banned"
	AccountStatusPending   AccountStatus = "pending"
)

// VerificationTier represents the level of creator verification.
type VerificationTier string

const (
	VerificationTierNone     VerificationTier = "none"
	VerificationTierVerified VerificationTier = "verified"   // blue tick
	VerificationTierOfficial VerificationTier = "official"   // government / brand
	VerificationTierCreator  VerificationTier = "creator"    // creator program
)

// PrivacyLevel controls who can see a piece of content or data.
type PrivacyLevel string

const (
	PrivacyPublic    PrivacyLevel = "public"
	PrivacyFollowers PrivacyLevel = "followers"
	PrivacyFriends   PrivacyLevel = "friends"
	PrivacyPrivate   PrivacyLevel = "private"
)

// ---------- UserProfile ----------

// UserProfile is the core profile record for every registered user.
type UserProfile struct {
	ID              uuid.UUID     `json:"id" db:"id"`
	UserID          uuid.UUID     `json:"user_id" db:"user_id"` // FK to auth-service users table
	Username        string        `json:"username" db:"username"`
	DisplayName     string        `json:"display_name" db:"display_name"`
	Bio             string        `json:"bio" db:"bio"`
	AvatarURL       string        `json:"avatar_url" db:"avatar_url"`
	AvatarKey       string        `json:"-" db:"avatar_key"` // MinIO object key – not exposed to clients
	WebsiteURL      string        `json:"website_url" db:"website_url"`
	Location        string        `json:"location" db:"location"`
	Email           string        `json:"email,omitempty" db:"email"`
	PhoneNumber     string        `json:"-" db:"phone_number"` // never serialised to JSON
	AccountStatus   AccountStatus `json:"account_status" db:"account_status"`
	IsCreator       bool          `json:"is_creator" db:"is_creator"`
	IsVerified      bool          `json:"is_verified" db:"is_verified"`
	VerificationTier VerificationTier `json:"verification_tier" db:"verification_tier"`

	// Counters – denormalised for fast reads; kept in sync by background jobs.
	FollowerCount  int64 `json:"follower_count" db:"follower_count"`
	FollowingCount int64 `json:"following_count" db:"following_count"`
	LikeCount      int64 `json:"like_count" db:"like_count"`
	VideoCount     int64 `json:"video_count" db:"video_count"`

	// Timestamps.
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// IsActive returns true when the profile has not been soft-deleted and is active.
func (p *UserProfile) IsActive() bool {
	return p.DeletedAt == nil && p.AccountStatus == AccountStatusActive
}

// PublicProfile returns a sanitised view of UserProfile safe for public consumption.
func (p *UserProfile) PublicProfile() *PublicUserProfile {
	return &PublicUserProfile{
		ID:               p.ID,
		UserID:           p.UserID,
		Username:         p.Username,
		DisplayName:      p.DisplayName,
		Bio:              p.Bio,
		AvatarURL:        p.AvatarURL,
		WebsiteURL:       p.WebsiteURL,
		Location:         p.Location,
		IsCreator:        p.IsCreator,
		IsVerified:       p.IsVerified,
		VerificationTier: p.VerificationTier,
		FollowerCount:    p.FollowerCount,
		FollowingCount:   p.FollowingCount,
		LikeCount:        p.LikeCount,
		VideoCount:       p.VideoCount,
		CreatedAt:        p.CreatedAt,
	}
}

// PublicUserProfile is the DTO returned to external callers.
type PublicUserProfile struct {
	ID               uuid.UUID        `json:"id"`
	UserID           uuid.UUID        `json:"user_id"`
	Username         string           `json:"username"`
	DisplayName      string           `json:"display_name"`
	Bio              string           `json:"bio"`
	AvatarURL        string           `json:"avatar_url"`
	WebsiteURL       string           `json:"website_url"`
	Location         string           `json:"location"`
	IsCreator        bool             `json:"is_creator"`
	IsVerified       bool             `json:"is_verified"`
	VerificationTier VerificationTier `json:"verification_tier"`
	FollowerCount    int64            `json:"follower_count"`
	FollowingCount   int64            `json:"following_count"`
	LikeCount        int64            `json:"like_count"`
	VideoCount       int64            `json:"video_count"`
	CreatedAt        time.Time        `json:"created_at"`
}

// ---------- CreatorProfile ----------

// CreatorProfile stores additional information for accounts enrolled in the
// TikTok creator programme.
type CreatorProfile struct {
	ID              uuid.UUID `json:"id" db:"id"`
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	ProfileID       uuid.UUID `json:"profile_id" db:"profile_id"` // FK -> user_profiles.id

	// Niche / content category.
	Category        string   `json:"category" db:"category"`
	SubCategories   []string `json:"sub_categories" db:"sub_categories"`

	// Monetisation.
	IsMonetised          bool    `json:"is_monetised" db:"is_monetised"`
	CreatorFundEnabled   bool    `json:"creator_fund_enabled" db:"creator_fund_enabled"`
	TipEnabled           bool    `json:"tip_enabled" db:"tip_enabled"`
	MinimumTipAmount     float64 `json:"minimum_tip_amount" db:"minimum_tip_amount"`

	// Business account details (optional).
	BusinessName    string `json:"business_name,omitempty" db:"business_name"`
	BusinessContact string `json:"business_contact,omitempty" db:"business_contact"`

	// Creator analytics snapshot – updated by a background job.
	TotalViews        int64   `json:"total_views" db:"total_views"`
	AvgViewsPerVideo  float64 `json:"avg_views_per_video" db:"avg_views_per_video"`
	EngagementRate    float64 `json:"engagement_rate" db:"engagement_rate"` // percentage
	ProfileViewCount  int64   `json:"profile_view_count" db:"profile_view_count"`
	ShareCount        int64   `json:"share_count" db:"share_count"`
	CommentCount      int64   `json:"comment_count" db:"comment_count"`

	// Joined / verified timestamps.
	CreatorSince     time.Time  `json:"creator_since" db:"creator_since"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty" db:"verified_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ---------- PrivacySettings ----------

// PrivacySettings controls the visibility of various profile sections.
type PrivacySettings struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	ProfileID uuid.UUID `json:"profile_id" db:"profile_id"`

	// Who can see the profile and its content.
	ProfileVisibility    PrivacyLevel `json:"profile_visibility" db:"profile_visibility"`
	VideoVisibility      PrivacyLevel `json:"video_visibility" db:"video_visibility"`
	LikedVideosVisible   bool         `json:"liked_videos_visible" db:"liked_videos_visible"`
	FollowersListVisible bool         `json:"followers_list_visible" db:"followers_list_visible"`
	FollowingListVisible bool         `json:"following_list_visible" db:"following_list_visible"`

	// Interaction settings.
	AllowComments       bool `json:"allow_comments" db:"allow_comments"`
	AllowDuet           bool `json:"allow_duet" db:"allow_duet"`
	AllowStitch         bool `json:"allow_stitch" db:"allow_stitch"`
	AllowDownload       bool `json:"allow_download" db:"allow_download"`
	AllowDirectMessages bool `json:"allow_direct_messages" db:"allow_direct_messages"`
	AllowTagging        bool `json:"allow_tagging" db:"allow_tagging"`

	// Notification preferences.
	NotifyOnFollow  bool `json:"notify_on_follow" db:"notify_on_follow"`
	NotifyOnLike    bool `json:"notify_on_like" db:"notify_on_like"`
	NotifyOnComment bool `json:"notify_on_comment" db:"notify_on_comment"`
	NotifyOnMention bool `json:"notify_on_mention" db:"notify_on_mention"`
	NotifyOnShare   bool `json:"notify_on_share" db:"notify_on_share"`

	// Safety.
	FilterSpam         bool `json:"filter_spam" db:"filter_spam"`
	FilterOffensive    bool `json:"filter_offensive" db:"filter_offensive"`
	RestrictedMode     bool `json:"restricted_mode" db:"restricted_mode"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// DefaultPrivacySettings returns a PrivacySettings with sensible defaults for
// a new account.
func DefaultPrivacySettings(userID, profileID uuid.UUID) *PrivacySettings {
	now := time.Now().UTC()
	return &PrivacySettings{
		ID:                   uuid.New(),
		UserID:               userID,
		ProfileID:            profileID,
		ProfileVisibility:    PrivacyPublic,
		VideoVisibility:      PrivacyPublic,
		LikedVideosVisible:   true,
		FollowersListVisible: true,
		FollowingListVisible: true,
		AllowComments:        true,
		AllowDuet:            true,
		AllowStitch:          true,
		AllowDownload:        true,
		AllowDirectMessages:  true,
		AllowTagging:         true,
		NotifyOnFollow:       true,
		NotifyOnLike:         true,
		NotifyOnComment:      true,
		NotifyOnMention:      true,
		NotifyOnShare:        false,
		FilterSpam:           true,
		FilterOffensive:      false,
		RestrictedMode:       false,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

// ---------- VerificationBadge ----------

// VerificationBadge records the formal verification of a user account.
type VerificationBadge struct {
	ID          uuid.UUID        `json:"id" db:"id"`
	UserID      uuid.UUID        `json:"user_id" db:"user_id"`
	ProfileID   uuid.UUID        `json:"profile_id" db:"profile_id"`
	Tier        VerificationTier `json:"tier" db:"tier"`
	GrantedByID uuid.UUID        `json:"granted_by_id" db:"granted_by_id"` // admin user ID
	Reason      string           `json:"reason" db:"reason"`
	DocumentRef string           `json:"-" db:"document_ref"` // internal reference only
	GrantedAt   time.Time        `json:"granted_at" db:"granted_at"`
	ExpiresAt   *time.Time       `json:"expires_at,omitempty" db:"expires_at"`
	RevokedAt   *time.Time       `json:"revoked_at,omitempty" db:"revoked_at"`
	RevokedByID *uuid.UUID       `json:"revoked_by_id,omitempty" db:"revoked_by_id"`
	RevokeReason string          `json:"revoke_reason,omitempty" db:"revoke_reason"`
}

// IsActive returns true when the badge has not been revoked and has not expired.
func (b *VerificationBadge) IsActive() bool {
	if b.RevokedAt != nil {
		return false
	}
	if b.ExpiresAt != nil && b.ExpiresAt.Before(time.Now().UTC()) {
		return false
	}
	return true
}

// ---------- BlockRecord ----------

// BlockRecord represents a user blocking another user.
type BlockRecord struct {
	ID          uuid.UUID `json:"id" db:"id"`
	BlockerID   uuid.UUID `json:"blocker_id" db:"blocker_id"`
	BlockedID   uuid.UUID `json:"blocked_id" db:"blocked_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// ---------- AccountAnalytics ----------

// AccountAnalytics is the aggregated analytics snapshot for a creator dashboard.
type AccountAnalytics struct {
	UserID           uuid.UUID `json:"user_id"`
	ProfileID        uuid.UUID `json:"profile_id"`

	// Reach & Impressions.
	TotalViews       int64   `json:"total_views"`
	ProfileViews     int64   `json:"profile_views"`
	UniqueViewers    int64   `json:"unique_viewers"`

	// Engagement.
	TotalLikes       int64   `json:"total_likes"`
	TotalComments    int64   `json:"total_comments"`
	TotalShares      int64   `json:"total_shares"`
	TotalFollowers   int64   `json:"total_followers"`
	NewFollowers     int64   `json:"new_followers"` // within the analytics window
	EngagementRate   float64 `json:"engagement_rate"` // (likes+comments+shares) / views * 100
	FollowerGrowthRate float64 `json:"follower_growth_rate"` // percentage change

	// Content.
	VideoCount       int64   `json:"video_count"`
	AvgViewsPerVideo float64 `json:"avg_views_per_video"`
	TopVideoID       *uuid.UUID `json:"top_video_id,omitempty"`

	// Window metadata.
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	ComputedAt  time.Time `json:"computed_at"`
}

// UpdateProfile is the payload accepted for partial profile updates.
type UpdateProfile struct {
	DisplayName *string `json:"display_name,omitempty" validate:"omitempty,min=1,max=50"`
	Bio         *string `json:"bio,omitempty" validate:"omitempty,max=160"`
	WebsiteURL  *string `json:"website_url,omitempty" validate:"omitempty,url,max=200"`
	Location    *string `json:"location,omitempty" validate:"omitempty,max=100"`
}

// AvatarUploadRequest is returned to the client so they can PUT the file
// directly to MinIO without routing through the API server.
type AvatarUploadRequest struct {
	UploadURL  string            `json:"upload_url"`
	ObjectKey  string            `json:"object_key"`
	ExpiresAt  time.Time         `json:"expires_at"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// SearchUsersResult wraps a paginated list of public profiles.
type SearchUsersResult struct {
	Users      []*PublicUserProfile `json:"users"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"page_size"`
	HasMore    bool                 `json:"has_more"`
}
