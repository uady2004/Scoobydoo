package models

import (
	"time"

	"github.com/google/uuid"
)

// AuthProvider enumerates the identity providers supported by the service.
type AuthProvider string

const (
	AuthProviderLocal  AuthProvider = "local"
	AuthProviderGoogle AuthProvider = "google"
	AuthProviderApple  AuthProvider = "apple"
)

// UserStatus represents the lifecycle state of a user account.
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDeleted   UserStatus = "deleted"
)

// OTPType distinguishes the purpose for which a one-time password was issued.
type OTPType string

const (
	OTPTypePhoneVerification OTPType = "phone_verification"
	OTPTypeEmailVerification OTPType = "email_verification"
	OTPTypePasswordReset     OTPType = "password_reset"
	OTPTypeLogin             OTPType = "login"
)

// User is the core identity record stored in the users table.
type User struct {
	ID uuid.UUID `db:"id" json:"id"`

	// Credentials / identifiers
	Email    *string `db:"email"     json:"email,omitempty"`
	Phone    *string `db:"phone"     json:"phone,omitempty"`
	Username string  `db:"username"  json:"username"`

	// Password (nil for OAuth-only accounts)
	PasswordHash *string `db:"password_hash" json:"-"`

	// OAuth
	Provider       AuthProvider `db:"provider"        json:"provider"`
	ProviderUserID *string      `db:"provider_user_id" json:"provider_user_id,omitempty"`

	// Verification flags
	EmailVerified bool `db:"email_verified" json:"email_verified"`
	PhoneVerified bool `db:"phone_verified" json:"phone_verified"`

	// MFA
	MFAEnabled bool    `db:"mfa_enabled"  json:"mfa_enabled"`
	MFASecret  *string `db:"mfa_secret"   json:"-"` // TOTP shared secret

	// Account status
	Status UserStatus `db:"status" json:"status"`

	// Profile hints (populated by user-service later)
	DisplayName *string `db:"display_name" json:"display_name,omitempty"`
	AvatarURL   *string `db:"avatar_url"   json:"avatar_url,omitempty"`

	// Timestamps
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// Session represents a device session tied to a user.
// The refresh_token stored here is the SHA-256 hash of the actual token.
type Session struct {
	ID           uuid.UUID  `db:"id"            json:"id"`
	UserID       uuid.UUID  `db:"user_id"       json:"user_id"`
	RefreshToken string     `db:"refresh_token" json:"-"` // hashed
	UserAgent    string     `db:"user_agent"    json:"user_agent"`
	IPAddress    string     `db:"ip_address"    json:"ip_address"`
	DeviceID     *string    `db:"device_id"     json:"device_id,omitempty"`
	IsRevoked    bool       `db:"is_revoked"    json:"is_revoked"`
	ExpiresAt    time.Time  `db:"expires_at"    json:"expires_at"`
	CreatedAt    time.Time  `db:"created_at"    json:"created_at"`
	LastSeenAt   time.Time  `db:"last_seen_at"  json:"last_seen_at"`
	RevokedAt    *time.Time `db:"revoked_at"    json:"revoked_at,omitempty"`
}

// OTPCode is a short-lived code sent via SMS or email.
type OTPCode struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	UserID    uuid.UUID `db:"user_id"    json:"user_id"`
	Code      string    `db:"code"       json:"-"` // hashed before storage
	Type      OTPType   `db:"type"       json:"type"`
	Target    string    `db:"target"     json:"target"` // phone or email
	Attempts  int       `db:"attempts"   json:"attempts"`
	MaxTrials int       `db:"max_trials" json:"max_trials"`
	IsUsed    bool      `db:"is_used"    json:"is_used"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// DeviceSession associates a device fingerprint to a user for suspicious-login detection.
type DeviceSession struct {
	ID           uuid.UUID  `db:"id"            json:"id"`
	UserID       uuid.UUID  `db:"user_id"       json:"user_id"`
	DeviceID     string     `db:"device_id"     json:"device_id"`
	DeviceName   string     `db:"device_name"   json:"device_name"`
	Platform     string     `db:"platform"      json:"platform"`
	IPAddress    string     `db:"ip_address"    json:"ip_address"`
	UserAgent    string     `db:"user_agent"    json:"user_agent"`
	IsTrusted    bool       `db:"is_trusted"    json:"is_trusted"`
	LastActiveAt time.Time  `db:"last_active_at" json:"last_active_at"`
	CreatedAt    time.Time  `db:"created_at"    json:"created_at"`
	RevokedAt    *time.Time `db:"revoked_at"    json:"revoked_at,omitempty"`
}

// TokenPair groups the access and refresh tokens returned to callers after a
// successful authentication event.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// Claims extends the standard JWT claims with auth-service–specific fields.
type Claims struct {
	UserID   uuid.UUID  `json:"uid"`
	Email    string     `json:"email,omitempty"`
	Phone    string     `json:"phone,omitempty"`
	Username string     `json:"username"`
	Provider string     `json:"provider"`
	MFADone  bool       `json:"mfa_done"`
}
