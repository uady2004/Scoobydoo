package models

import "time"

// AdminRole defines the permission level of an admin user.
type AdminRole string

const (
	AdminRoleSuperAdmin  AdminRole = "superadmin"
	AdminRoleModerator   AdminRole = "moderator"
	AdminRoleSupport     AdminRole = "support"
	AdminRoleAnalyst     AdminRole = "analyst"
)

// AdminUser represents a member of the operations team.
type AdminUser struct {
	ID           string    `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	FullName     string    `json:"full_name" db:"full_name"`
	Role         AdminRole `json:"role" db:"role"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// UserBan represents a ban applied to a platform user.
type UserBan struct {
	ID          string     `json:"id" db:"id"`
	UserID      string     `json:"user_id" db:"user_id"`
	AdminID     string     `json:"admin_id" db:"admin_id"`
	Reason      string     `json:"reason" db:"reason"`
	BanType     string     `json:"ban_type" db:"ban_type"` // "temporary" | "permanent"
	ExpiresAt   *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// ContentModeration records a moderation action on a video/comment.
type ContentModeration struct {
	ID          string    `json:"id" db:"id"`
	ContentID   string    `json:"content_id" db:"content_id"`
	ContentType string    `json:"content_type" db:"content_type"` // "video" | "comment" | "livestream"
	AdminID     string    `json:"admin_id" db:"admin_id"`
	Action      string    `json:"action" db:"action"` // "approve" | "reject" | "remove" | "warn"
	Reason      string    `json:"reason" db:"reason"`
	Notes       string    `json:"notes,omitempty" db:"notes"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// AuditLog records every admin action for compliance.
type AuditLog struct {
	ID          string                 `json:"id" db:"id"`
	AdminID     string                 `json:"admin_id" db:"admin_id"`
	AdminEmail  string                 `json:"admin_email" db:"admin_email"`
	Action      string                 `json:"action" db:"action"`
	ResourceType string                `json:"resource_type" db:"resource_type"`
	ResourceID  string                 `json:"resource_id" db:"resource_id"`
	Details     map[string]interface{} `json:"details,omitempty" db:"details"`
	IPAddress   string                 `json:"ip_address" db:"ip_address"`
	UserAgent   string                 `json:"user_agent" db:"user_agent"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
}

// PlatformStats is a snapshot of key platform metrics.
type PlatformStats struct {
	TotalUsers        int64   `json:"total_users"`
	ActiveUsersToday  int64   `json:"active_users_today"`
	TotalVideos       int64   `json:"total_videos"`
	VideosUploadedToday int64 `json:"videos_uploaded_today"`
	ActiveLivestreams int64   `json:"active_livestreams"`
	TotalOrders       int64   `json:"total_orders"`
	RevenueToday      float64 `json:"revenue_today"`
	PendingReports    int64   `json:"pending_reports"`
}
