package models

import (
	"time"
)

// NotificationType enumerates the supported notification kinds.
type NotificationType string

const (
	NotificationTypeLike          NotificationType = "like"
	NotificationTypeComment       NotificationType = "comment"
	NotificationTypeFollow        NotificationType = "follow"
	NotificationTypeMention       NotificationType = "mention"
	NotificationTypeGift          NotificationType = "gift"
	NotificationTypeOrderCreated  NotificationType = "order_created"
	NotificationTypeOrderShipped  NotificationType = "order_shipped"
	NotificationTypeLiveStream    NotificationType = "livestream"
	NotificationTypeSystem        NotificationType = "system"
	NotificationTypeEmailVerify   NotificationType = "email_verification"
	NotificationTypePasswordReset NotificationType = "password_reset"
	NotificationTypeWeeklyDigest  NotificationType = "weekly_digest"
)

// Channel describes how a notification is delivered.
type Channel string

const (
	ChannelPush  Channel = "push"
	ChannelEmail Channel = "email"
	ChannelSMS   Channel = "sms"
	ChannelInApp Channel = "in_app"
)

// DevicePlatform identifies the push-token platform.
type DevicePlatform string

const (
	PlatformIOS     DevicePlatform = "ios"
	PlatformAndroid DevicePlatform = "android"
	PlatformWeb     DevicePlatform = "web"
)

// Notification is the core notification record stored in PostgreSQL.
type Notification struct {
	ID         string           `json:"id" db:"id"`
	UserID     string           `json:"user_id" db:"user_id"`
	ActorID    string           `json:"actor_id,omitempty" db:"actor_id"`
	ActorName  string           `json:"actor_name,omitempty" db:"actor_name"`
	ActorAvatar string          `json:"actor_avatar,omitempty" db:"actor_avatar"`
	Type       NotificationType `json:"type" db:"type"`
	// Title and Body are the display strings shown in the UI / push payload.
	Title   string `json:"title" db:"title"`
	Body    string `json:"body" db:"body"`
	// ImageURL is an optional thumbnail attached to the push notification.
	ImageURL string `json:"image_url,omitempty" db:"image_url"`
	// DeepLink is the in-app route the notification should open.
	DeepLink string `json:"deep_link,omitempty" db:"deep_link"`
	// Metadata stores type-specific extra fields as raw JSON.
	Metadata map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	// GroupKey is used to aggregate related notifications (e.g. all likes on a video).
	GroupKey string `json:"group_key,omitempty" db:"group_key"`
	// GroupCount is the total number of events collapsed into this record.
	GroupCount int `json:"group_count" db:"group_count"`
	IsRead     bool      `json:"is_read" db:"is_read"`
	ReadAt     *time.Time `json:"read_at,omitempty" db:"read_at"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// PushDevice stores an FCM/APNs token for a user's device.
type PushDevice struct {
	ID        string         `json:"id" db:"id"`
	UserID    string         `json:"user_id" db:"user_id"`
	Token     string         `json:"token" db:"token"`
	Platform  DevicePlatform `json:"platform" db:"platform"`
	// AppVersion is stored for debugging purposes.
	AppVersion string    `json:"app_version,omitempty" db:"app_version"`
	DeviceName string    `json:"device_name,omitempty" db:"device_name"`
	IsActive   bool      `json:"is_active" db:"is_active"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// NotificationPreference controls which channels and types a user has enabled.
type NotificationPreference struct {
	UserID string `json:"user_id" db:"user_id"`

	// Channel-level master switches.
	PushEnabled  bool `json:"push_enabled" db:"push_enabled"`
	EmailEnabled bool `json:"email_enabled" db:"email_enabled"`
	SMSEnabled   bool `json:"sms_enabled" db:"sms_enabled"`
	InAppEnabled bool `json:"in_app_enabled" db:"in_app_enabled"`

	// Type-level granular switches.
	LikesEnabled      bool `json:"likes_enabled" db:"likes_enabled"`
	CommentsEnabled   bool `json:"comments_enabled" db:"comments_enabled"`
	FollowsEnabled    bool `json:"follows_enabled" db:"follows_enabled"`
	MentionsEnabled   bool `json:"mentions_enabled" db:"mentions_enabled"`
	GiftsEnabled      bool `json:"gifts_enabled" db:"gifts_enabled"`
	OrdersEnabled     bool `json:"orders_enabled" db:"orders_enabled"`
	LiveStreamEnabled bool `json:"livestream_enabled" db:"livestream_enabled"`
	SystemEnabled     bool `json:"system_enabled" db:"system_enabled"`

	// Quiet hours: notifications are suppressed between QuietStart and QuietEnd (24-h "HH:MM").
	QuietHoursEnabled bool   `json:"quiet_hours_enabled" db:"quiet_hours_enabled"`
	QuietStart        string `json:"quiet_start,omitempty" db:"quiet_start"`
	QuietEnd          string `json:"quiet_end,omitempty" db:"quiet_end"`
	Timezone          string `json:"timezone,omitempty" db:"timezone"`

	// DigestEnabled collapses non-urgent notifications into a daily or weekly digest email.
	DigestEnabled   bool   `json:"digest_enabled" db:"digest_enabled"`
	DigestFrequency string `json:"digest_frequency,omitempty" db:"digest_frequency"` // "daily" | "weekly"

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// DefaultPreferences returns a NotificationPreference with all channels enabled.
func DefaultPreferences(userID string) NotificationPreference {
	now := time.Now().UTC()
	return NotificationPreference{
		UserID:            userID,
		PushEnabled:       true,
		EmailEnabled:      true,
		SMSEnabled:        false,
		InAppEnabled:      true,
		LikesEnabled:      true,
		CommentsEnabled:   true,
		FollowsEnabled:    true,
		MentionsEnabled:   true,
		GiftsEnabled:      true,
		OrdersEnabled:     true,
		LiveStreamEnabled: true,
		SystemEnabled:     true,
		QuietHoursEnabled: false,
		DigestEnabled:     false,
		DigestFrequency:   "weekly",
		Timezone:          "UTC",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// IsTypeEnabled reports whether the given notification type is allowed
// by the user's per-type preference flags.
func (p *NotificationPreference) IsTypeEnabled(t NotificationType) bool {
	switch t {
	case NotificationTypeLike:
		return p.LikesEnabled
	case NotificationTypeComment:
		return p.CommentsEnabled
	case NotificationTypeFollow:
		return p.FollowsEnabled
	case NotificationTypeMention:
		return p.MentionsEnabled
	case NotificationTypeGift:
		return p.GiftsEnabled
	case NotificationTypeOrderCreated, NotificationTypeOrderShipped:
		return p.OrdersEnabled
	case NotificationTypeLiveStream:
		return p.LiveStreamEnabled
	case NotificationTypeSystem,
		NotificationTypeEmailVerify,
		NotificationTypePasswordReset,
		NotificationTypeWeeklyDigest:
		return p.SystemEnabled
	default:
		return true
	}
}

// ---------------------------------------------------------------------------
// Request / response DTOs
// ---------------------------------------------------------------------------

// CreateNotificationRequest is the internal input for creating a notification.
type CreateNotificationRequest struct {
	UserID      string                 `json:"user_id"`
	ActorID     string                 `json:"actor_id,omitempty"`
	ActorName   string                 `json:"actor_name,omitempty"`
	ActorAvatar string                 `json:"actor_avatar,omitempty"`
	Type        NotificationType       `json:"type"`
	Title       string                 `json:"title"`
	Body        string                 `json:"body"`
	ImageURL    string                 `json:"image_url,omitempty"`
	DeepLink    string                 `json:"deep_link,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	GroupKey    string                 `json:"group_key,omitempty"`
	// Channels overrides the user's preferences for this specific notification.
	// Leave nil to let the preference engine decide.
	Channels []Channel `json:"channels,omitempty"`
}

// ListNotificationsRequest holds pagination parameters.
type ListNotificationsRequest struct {
	UserID   string `form:"user_id"`
	Limit    int    `form:"limit"`
	Offset   int    `form:"offset"`
	UnreadOnly bool `form:"unread_only"`
}

// NotificationsResponse is the paginated list returned by the REST handler.
type NotificationsResponse struct {
	Notifications []*Notification `json:"notifications"`
	Total         int64           `json:"total"`
	UnreadCount   int64           `json:"unread_count"`
	Limit         int             `json:"limit"`
	Offset        int             `json:"offset"`
}

// RegisterDeviceRequest is the body for POST /devices.
type RegisterDeviceRequest struct {
	Token      string         `json:"token" binding:"required"`
	Platform   DevicePlatform `json:"platform" binding:"required,oneof=ios android web"`
	AppVersion string         `json:"app_version,omitempty"`
	DeviceName string         `json:"device_name,omitempty"`
}

// UpdatePreferencesRequest is the body for PUT /preferences.
type UpdatePreferencesRequest struct {
	PushEnabled       *bool   `json:"push_enabled,omitempty"`
	EmailEnabled      *bool   `json:"email_enabled,omitempty"`
	SMSEnabled        *bool   `json:"sms_enabled,omitempty"`
	InAppEnabled      *bool   `json:"in_app_enabled,omitempty"`
	LikesEnabled      *bool   `json:"likes_enabled,omitempty"`
	CommentsEnabled   *bool   `json:"comments_enabled,omitempty"`
	FollowsEnabled    *bool   `json:"follows_enabled,omitempty"`
	MentionsEnabled   *bool   `json:"mentions_enabled,omitempty"`
	GiftsEnabled      *bool   `json:"gifts_enabled,omitempty"`
	OrdersEnabled     *bool   `json:"orders_enabled,omitempty"`
	LiveStreamEnabled *bool   `json:"livestream_enabled,omitempty"`
	SystemEnabled     *bool   `json:"system_enabled,omitempty"`
	QuietHoursEnabled *bool   `json:"quiet_hours_enabled,omitempty"`
	QuietStart        *string `json:"quiet_start,omitempty"`
	QuietEnd          *string `json:"quiet_end,omitempty"`
	Timezone          *string `json:"timezone,omitempty"`
	DigestEnabled     *bool   `json:"digest_enabled,omitempty"`
	DigestFrequency   *string `json:"digest_frequency,omitempty"`
}

// KafkaEvent is the generic event envelope published by other services.
type KafkaEvent struct {
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"`
	OccurredAt time.Time             `json:"occurred_at"`
	Payload   map[string]interface{} `json:"payload"`
}

// ---------------------------------------------------------------------------
// Aggregation helpers
// ---------------------------------------------------------------------------

// AggregatedActors is used to build messages like "Alice and 9 others liked your video".
type AggregatedActors struct {
	FirstActorName string
	OtherCount     int
}

// BuildAggregatedBody returns a human-readable sentence for aggregated events.
func BuildAggregatedBody(actors AggregatedActors, action, subject string) string {
	if actors.OtherCount == 0 {
		return actors.FirstActorName + " " + action + " " + subject
	}
	return actors.FirstActorName + " and " +
		formatCount(actors.OtherCount) + " others " +
		action + " " + subject
}

func formatCount(n int) string {
	// Simple int-to-string without importing strconv at model level.
	if n < 0 {
		n = 0
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if len(digits) == 0 {
		return "0"
	}
	return string(digits)
}
