package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/notification-service/internal/models"
)

// NotificationRepository defines the persistence interface used by the service layer.
type NotificationRepository interface {
	// Notifications
	CreateNotification(ctx context.Context, n *models.Notification) error
	GetNotifications(ctx context.Context, req models.ListNotificationsRequest) ([]*models.Notification, int64, error)
	GetNotificationByID(ctx context.Context, id string) (*models.Notification, error)
	MarkAsRead(ctx context.Context, notificationID, userID string) error
	MarkAllRead(ctx context.Context, userID string) error
	GetUnreadCount(ctx context.Context, userID string) (int64, error)
	// Aggregation: returns the group count for a given group key.
	GetGroupCount(ctx context.Context, groupKey, userID string) (int, error)
	IncrementGroupCount(ctx context.Context, groupKey, userID string, actorName string) error

	// Devices
	SaveDevice(ctx context.Context, device *models.PushDevice) error
	RemoveDevice(ctx context.Context, token, userID string) error
	GetDevicesByUserID(ctx context.Context, userID string) ([]*models.PushDevice, error)
	DeactivateDevice(ctx context.Context, token string) error

	// Preferences
	GetPreferences(ctx context.Context, userID string) (*models.NotificationPreference, error)
	UpdatePreferences(ctx context.Context, pref *models.NotificationPreference) error
	UpsertPreferences(ctx context.Context, pref *models.NotificationPreference) error
}

// pgxNotificationRepository is the PostgreSQL implementation backed by pgxpool.
type pgxNotificationRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewNotificationRepository creates a new pgxpool-backed repository.
func NewNotificationRepository(pool *pgxpool.Pool, logger *zap.Logger) NotificationRepository {
	return &pgxNotificationRepository{pool: pool, logger: logger}
}

// ---------------------------------------------------------------------------
// Notification CRUD
// ---------------------------------------------------------------------------

func (r *pgxNotificationRepository) CreateNotification(ctx context.Context, n *models.Notification) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	n.CreatedAt = now
	n.UpdatedAt = now

	metaJSON, err := json.Marshal(n.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO notifications (
			id, user_id, actor_id, actor_name, actor_avatar,
			type, title, body, image_url, deep_link,
			metadata, group_key, group_count,
			is_read, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13,
			FALSE, $14, $15
		)
		ON CONFLICT (id) DO NOTHING`

	_, err = r.pool.Exec(ctx, q,
		n.ID, n.UserID, n.ActorID, n.ActorName, n.ActorAvatar,
		n.Type, n.Title, n.Body, n.ImageURL, n.DeepLink,
		metaJSON, n.GroupKey, n.GroupCount,
		n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (r *pgxNotificationRepository) GetNotifications(
	ctx context.Context,
	req models.ListNotificationsRequest,
) ([]*models.Notification, int64, error) {

	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}

	whereClause := "WHERE user_id = $1"
	args := []interface{}{req.UserID}
	argIdx := 2

	if req.UnreadOnly {
		whereClause += fmt.Sprintf(" AND is_read = FALSE")
	}

	countQ := fmt.Sprintf("SELECT COUNT(*) FROM notifications %s", whereClause)
	var total int64
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	selectQ := fmt.Sprintf(`
		SELECT id, user_id, actor_id, actor_name, actor_avatar,
		       type, title, body, image_url, deep_link,
		       metadata, group_key, group_count,
		       is_read, read_at, created_at, updated_at
		FROM notifications
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, req.Limit, req.Offset)

	rows, err := r.pool.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*models.Notification
	for rows.Next() {
		n := &models.Notification{}
		var metaJSON []byte
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.ActorID, &n.ActorName, &n.ActorAvatar,
			&n.Type, &n.Title, &n.Body, &n.ImageURL, &n.DeepLink,
			&metaJSON, &n.GroupKey, &n.GroupCount,
			&n.IsRead, &n.ReadAt, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		if len(metaJSON) > 0 {
			if err := json.Unmarshal(metaJSON, &n.Metadata); err != nil {
				r.logger.Warn("unmarshal notification metadata", zap.String("id", n.ID), zap.Error(err))
			}
		}
		notifications = append(notifications, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}
	return notifications, total, nil
}

func (r *pgxNotificationRepository) GetNotificationByID(ctx context.Context, id string) (*models.Notification, error) {
	const q = `
		SELECT id, user_id, actor_id, actor_name, actor_avatar,
		       type, title, body, image_url, deep_link,
		       metadata, group_key, group_count,
		       is_read, read_at, created_at, updated_at
		FROM notifications
		WHERE id = $1`

	n := &models.Notification{}
	var metaJSON []byte
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&n.ID, &n.UserID, &n.ActorID, &n.ActorName, &n.ActorAvatar,
		&n.Type, &n.Title, &n.Body, &n.ImageURL, &n.DeepLink,
		&metaJSON, &n.GroupKey, &n.GroupCount,
		&n.IsRead, &n.ReadAt, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("notification not found")
		}
		return nil, fmt.Errorf("get notification by id: %w", err)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &n.Metadata)
	}
	return n, nil
}

func (r *pgxNotificationRepository) MarkAsRead(ctx context.Context, notificationID, userID string) error {
	now := time.Now().UTC()
	const q = `
		UPDATE notifications
		SET is_read = TRUE, read_at = $1, updated_at = $1
		WHERE id = $2 AND user_id = $3 AND is_read = FALSE`
	_, err := r.pool.Exec(ctx, q, now, notificationID, userID)
	if err != nil {
		return fmt.Errorf("mark as read: %w", err)
	}
	return nil
}

func (r *pgxNotificationRepository) MarkAllRead(ctx context.Context, userID string) error {
	now := time.Now().UTC()
	const q = `
		UPDATE notifications
		SET is_read = TRUE, read_at = $1, updated_at = $1
		WHERE user_id = $2 AND is_read = FALSE`
	_, err := r.pool.Exec(ctx, q, now, userID)
	if err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}

func (r *pgxNotificationRepository) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	const q = `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`
	var count int64
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("get unread count: %w", err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Aggregation helpers
// ---------------------------------------------------------------------------

func (r *pgxNotificationRepository) GetGroupCount(ctx context.Context, groupKey, userID string) (int, error) {
	const q = `
		SELECT COALESCE(MAX(group_count), 0)
		FROM notifications
		WHERE group_key = $1 AND user_id = $2`
	var count int
	if err := r.pool.QueryRow(ctx, q, groupKey, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("get group count: %w", err)
	}
	return count, nil
}

// IncrementGroupCount updates the group_count and actor_name of the most-recent
// grouped notification record and returns after bumping the count.
func (r *pgxNotificationRepository) IncrementGroupCount(
	ctx context.Context, groupKey, userID string, actorName string,
) error {
	now := time.Now().UTC()
	const q = `
		UPDATE notifications
		SET group_count = group_count + 1,
		    actor_name  = $1,
		    updated_at  = $2
		WHERE id = (
		    SELECT id FROM notifications
		    WHERE group_key = $3 AND user_id = $4
		    ORDER BY created_at DESC
		    LIMIT 1
		)`
	_, err := r.pool.Exec(ctx, q, actorName, now, groupKey, userID)
	if err != nil {
		return fmt.Errorf("increment group count: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Device management
// ---------------------------------------------------------------------------

func (r *pgxNotificationRepository) SaveDevice(ctx context.Context, device *models.PushDevice) error {
	if device.ID == "" {
		device.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	device.CreatedAt = now
	device.UpdatedAt = now
	device.IsActive = true

	const q = `
		INSERT INTO push_devices (id, user_id, token, platform, app_version, device_name, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (token) DO UPDATE
		SET user_id     = EXCLUDED.user_id,
		    platform    = EXCLUDED.platform,
		    app_version = EXCLUDED.app_version,
		    device_name = EXCLUDED.device_name,
		    is_active   = TRUE,
		    updated_at  = EXCLUDED.updated_at`

	_, err := r.pool.Exec(ctx, q,
		device.ID, device.UserID, device.Token, device.Platform,
		device.AppVersion, device.DeviceName, device.IsActive,
		device.CreatedAt, device.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save device: %w", err)
	}
	return nil
}

func (r *pgxNotificationRepository) RemoveDevice(ctx context.Context, token, userID string) error {
	const q = `DELETE FROM push_devices WHERE token = $1 AND user_id = $2`
	_, err := r.pool.Exec(ctx, q, token, userID)
	if err != nil {
		return fmt.Errorf("remove device: %w", err)
	}
	return nil
}

func (r *pgxNotificationRepository) GetDevicesByUserID(ctx context.Context, userID string) ([]*models.PushDevice, error) {
	const q = `
		SELECT id, user_id, token, platform, app_version, device_name, is_active, created_at, updated_at
		FROM push_devices
		WHERE user_id = $1 AND is_active = TRUE`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get devices: %w", err)
	}
	defer rows.Close()

	var devices []*models.PushDevice
	for rows.Next() {
		d := &models.PushDevice{}
		if err := rows.Scan(
			&d.ID, &d.UserID, &d.Token, &d.Platform,
			&d.AppVersion, &d.DeviceName, &d.IsActive,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return devices, nil
}

// DeactivateDevice marks a token as inactive (e.g. after FCM reports it as invalid).
func (r *pgxNotificationRepository) DeactivateDevice(ctx context.Context, token string) error {
	now := time.Now().UTC()
	const q = `UPDATE push_devices SET is_active = FALSE, updated_at = $1 WHERE token = $2`
	_, err := r.pool.Exec(ctx, q, now, token)
	if err != nil {
		return fmt.Errorf("deactivate device: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Preferences
// ---------------------------------------------------------------------------

func (r *pgxNotificationRepository) GetPreferences(ctx context.Context, userID string) (*models.NotificationPreference, error) {
	const q = `
		SELECT user_id,
		       push_enabled, email_enabled, sms_enabled, in_app_enabled,
		       likes_enabled, comments_enabled, follows_enabled, mentions_enabled,
		       gifts_enabled, orders_enabled, livestream_enabled, system_enabled,
		       quiet_hours_enabled, quiet_start, quiet_end, timezone,
		       digest_enabled, digest_frequency,
		       created_at, updated_at
		FROM notification_preferences
		WHERE user_id = $1`

	p := &models.NotificationPreference{}
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&p.UserID,
		&p.PushEnabled, &p.EmailEnabled, &p.SMSEnabled, &p.InAppEnabled,
		&p.LikesEnabled, &p.CommentsEnabled, &p.FollowsEnabled, &p.MentionsEnabled,
		&p.GiftsEnabled, &p.OrdersEnabled, &p.LiveStreamEnabled, &p.SystemEnabled,
		&p.QuietHoursEnabled, &p.QuietStart, &p.QuietEnd, &p.Timezone,
		&p.DigestEnabled, &p.DigestFrequency,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return defaults; the caller may upsert them.
			def := models.DefaultPreferences(userID)
			return &def, nil
		}
		return nil, fmt.Errorf("get preferences: %w", err)
	}
	return p, nil
}

func (r *pgxNotificationRepository) UpdatePreferences(ctx context.Context, pref *models.NotificationPreference) error {
	pref.UpdatedAt = time.Now().UTC()
	const q = `
		UPDATE notification_preferences
		SET push_enabled       = $1,
		    email_enabled      = $2,
		    sms_enabled        = $3,
		    in_app_enabled     = $4,
		    likes_enabled      = $5,
		    comments_enabled   = $6,
		    follows_enabled    = $7,
		    mentions_enabled   = $8,
		    gifts_enabled      = $9,
		    orders_enabled     = $10,
		    livestream_enabled = $11,
		    system_enabled     = $12,
		    quiet_hours_enabled = $13,
		    quiet_start        = $14,
		    quiet_end          = $15,
		    timezone           = $16,
		    digest_enabled     = $17,
		    digest_frequency   = $18,
		    updated_at         = $19
		WHERE user_id = $20`

	_, err := r.pool.Exec(ctx, q,
		pref.PushEnabled, pref.EmailEnabled, pref.SMSEnabled, pref.InAppEnabled,
		pref.LikesEnabled, pref.CommentsEnabled, pref.FollowsEnabled, pref.MentionsEnabled,
		pref.GiftsEnabled, pref.OrdersEnabled, pref.LiveStreamEnabled, pref.SystemEnabled,
		pref.QuietHoursEnabled, pref.QuietStart, pref.QuietEnd, pref.Timezone,
		pref.DigestEnabled, pref.DigestFrequency,
		pref.UpdatedAt, pref.UserID,
	)
	if err != nil {
		return fmt.Errorf("update preferences: %w", err)
	}
	return nil
}

func (r *pgxNotificationRepository) UpsertPreferences(ctx context.Context, pref *models.NotificationPreference) error {
	now := time.Now().UTC()
	if pref.CreatedAt.IsZero() {
		pref.CreatedAt = now
	}
	pref.UpdatedAt = now

	const q = `
		INSERT INTO notification_preferences (
			user_id,
			push_enabled, email_enabled, sms_enabled, in_app_enabled,
			likes_enabled, comments_enabled, follows_enabled, mentions_enabled,
			gifts_enabled, orders_enabled, livestream_enabled, system_enabled,
			quiet_hours_enabled, quiet_start, quiet_end, timezone,
			digest_enabled, digest_frequency,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17,
			$18, $19,
			$20, $21
		)
		ON CONFLICT (user_id) DO UPDATE
		SET push_enabled       = EXCLUDED.push_enabled,
		    email_enabled      = EXCLUDED.email_enabled,
		    sms_enabled        = EXCLUDED.sms_enabled,
		    in_app_enabled     = EXCLUDED.in_app_enabled,
		    likes_enabled      = EXCLUDED.likes_enabled,
		    comments_enabled   = EXCLUDED.comments_enabled,
		    follows_enabled    = EXCLUDED.follows_enabled,
		    mentions_enabled   = EXCLUDED.mentions_enabled,
		    gifts_enabled      = EXCLUDED.gifts_enabled,
		    orders_enabled     = EXCLUDED.orders_enabled,
		    livestream_enabled = EXCLUDED.livestream_enabled,
		    system_enabled     = EXCLUDED.system_enabled,
		    quiet_hours_enabled = EXCLUDED.quiet_hours_enabled,
		    quiet_start        = EXCLUDED.quiet_start,
		    quiet_end          = EXCLUDED.quiet_end,
		    timezone           = EXCLUDED.timezone,
		    digest_enabled     = EXCLUDED.digest_enabled,
		    digest_frequency   = EXCLUDED.digest_frequency,
		    updated_at         = EXCLUDED.updated_at`

	_, err := r.pool.Exec(ctx, q,
		pref.UserID,
		pref.PushEnabled, pref.EmailEnabled, pref.SMSEnabled, pref.InAppEnabled,
		pref.LikesEnabled, pref.CommentsEnabled, pref.FollowsEnabled, pref.MentionsEnabled,
		pref.GiftsEnabled, pref.OrdersEnabled, pref.LiveStreamEnabled, pref.SystemEnabled,
		pref.QuietHoursEnabled, pref.QuietStart, pref.QuietEnd, pref.Timezone,
		pref.DigestEnabled, pref.DigestFrequency,
		pref.CreatedAt, pref.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert preferences: %w", err)
	}
	return nil
}
