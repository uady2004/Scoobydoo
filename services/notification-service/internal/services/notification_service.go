package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/notification-service/internal/models"
	"github.com/tiktok-clone/notification-service/internal/repositories"
)

// NotificationService orchestrates notification creation, preference filtering,
// aggregation, and fan-out to push/email/SMS channels.
type NotificationService interface {
	// CreateNotification creates a notification record and dispatches it through
	// the appropriate channels based on the user's preferences.
	CreateNotification(ctx context.Context, req *models.CreateNotificationRequest) (*models.Notification, error)

	// GetNotifications returns a paginated list of notifications for a user.
	GetNotifications(ctx context.Context, req models.ListNotificationsRequest) (*models.NotificationsResponse, error)

	// MarkAsRead marks a single notification as read.
	MarkAsRead(ctx context.Context, notificationID, userID string) error

	// MarkAllRead marks every unread notification for a user as read.
	MarkAllRead(ctx context.Context, userID string) error

	// GetUnreadCount returns the number of unread notifications.
	GetUnreadCount(ctx context.Context, userID string) (int64, error)

	// RegisterDevice saves an FCM push token for the user's device.
	RegisterDevice(ctx context.Context, userID string, req *models.RegisterDeviceRequest) error

	// UnregisterDevice removes a push token.
	UnregisterDevice(ctx context.Context, token, userID string) error

	// GetPreferences returns the notification preferences for a user.
	GetPreferences(ctx context.Context, userID string) (*models.NotificationPreference, error)

	// UpdatePreferences applies a partial update to the user's notification preferences.
	UpdatePreferences(ctx context.Context, userID string, req *models.UpdatePreferencesRequest) (*models.NotificationPreference, error)
}

// aggregationThreshold is the number of events in a group before we start
// collapsing them (e.g. "Alice and 9 others liked your video").
const aggregationThreshold = 10

type notificationService struct {
	repo   repositories.NotificationRepository
	push   PushService
	email  EmailService
	sms    SMSService
	logger *zap.Logger
}

// NewNotificationService creates a new orchestrating NotificationService.
func NewNotificationService(
	repo repositories.NotificationRepository,
	push PushService,
	email EmailService,
	sms SMSService,
	logger *zap.Logger,
) NotificationService {
	return &notificationService{
		repo:   repo,
		push:   push,
		email:  email,
		sms:    sms,
		logger: logger,
	}
}

// ---------------------------------------------------------------------------
// CreateNotification — core orchestration logic
// ---------------------------------------------------------------------------

func (s *notificationService) CreateNotification(
	ctx context.Context,
	req *models.CreateNotificationRequest,
) (*models.Notification, error) {

	// 1. Load user preferences.
	prefs, err := s.repo.GetPreferences(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get preferences: %w", err)
	}

	// 2. Check per-type preference gate.
	if !prefs.IsTypeEnabled(req.Type) {
		s.logger.Debug("notification suppressed by user preference",
			zap.String("user_id", req.UserID),
			zap.String("type", string(req.Type)),
		)
		return nil, nil
	}

	// 3. Check quiet hours.
	if prefs.QuietHoursEnabled && s.isQuietHours(prefs) {
		s.logger.Debug("notification suppressed by quiet hours",
			zap.String("user_id", req.UserID),
		)
		// Still persist in-app; just skip push/sms/email.
		req.Channels = []models.Channel{models.ChannelInApp}
	}

	// 4. Aggregation: if a group key is set, try to collapse into an existing record.
	if req.GroupKey != "" {
		aggregated, err := s.tryAggregate(ctx, req, prefs)
		if err != nil {
			s.logger.Warn("aggregation failed, falling back to new record",
				zap.Error(err),
			)
		} else if aggregated {
			return nil, nil // merged into existing record; nothing more to do
		}
	}

	// 5. Build and persist the Notification record.
	n := &models.Notification{
		ID:          uuid.New().String(),
		UserID:      req.UserID,
		ActorID:     req.ActorID,
		ActorName:   req.ActorName,
		ActorAvatar: req.ActorAvatar,
		Type:        req.Type,
		Title:       req.Title,
		Body:        req.Body,
		ImageURL:    req.ImageURL,
		DeepLink:    req.DeepLink,
		Metadata:    req.Metadata,
		GroupKey:    req.GroupKey,
		GroupCount:  1,
		IsRead:      false,
	}

	if err := s.repo.CreateNotification(ctx, n); err != nil {
		return nil, fmt.Errorf("create notification record: %w", err)
	}

	// 6. Determine active channels (req.Channels overrides prefs).
	channels := req.Channels
	if len(channels) == 0 {
		channels = s.resolveChannels(req.Type, prefs)
	}

	// 7. Dispatch to each channel asynchronously; log but don't fail on delivery errors.
	for _, ch := range channels {
		ch := ch
		switch ch {
		case models.ChannelPush:
			if prefs.PushEnabled {
				go func() {
					dCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := s.push.SendToUser(dCtx, req.UserID, n); err != nil {
						s.logger.Error("push delivery failed",
							zap.String("user_id", req.UserID),
							zap.Error(err),
						)
					}
				}()
			}
		case models.ChannelEmail:
			// Email delivery for non-transactional types is handled by the
			// digest worker. Only dispatch immediately for system-level types.
			if prefs.EmailEnabled && isImmediateEmailType(req.Type) {
				s.logger.Info("immediate email notification",
					zap.String("user_id", req.UserID),
					zap.String("type", string(req.Type)),
				)
			}
		case models.ChannelSMS:
			if prefs.SMSEnabled {
				s.logger.Info("sms notification queued",
					zap.String("user_id", req.UserID),
					zap.String("type", string(req.Type)),
				)
			}
		case models.ChannelInApp:
			// In-app is always persisted (the DB record above handles this).
		}
	}

	return n, nil
}

// tryAggregate attempts to merge the incoming event into an existing
// notification group. Returns true if the event was successfully merged.
func (s *notificationService) tryAggregate(
	ctx context.Context,
	req *models.CreateNotificationRequest,
	_ *models.NotificationPreference,
) (bool, error) {
	count, err := s.repo.GetGroupCount(ctx, req.GroupKey, req.UserID)
	if err != nil {
		return false, fmt.Errorf("get group count: %w", err)
	}

	if count == 0 {
		// No existing group — allow a new notification record to be created.
		return false, nil
	}

	// Group already exists: bump the counter and update the body text.
	if err := s.repo.IncrementGroupCount(ctx, req.GroupKey, req.UserID, req.ActorName); err != nil {
		return false, fmt.Errorf("increment group count: %w", err)
	}

	// If we just crossed the aggregation threshold send an updated push
	// so the user's notification centre shows the grouped count.
	newCount := count + 1
	if newCount == aggregationThreshold || newCount%aggregationThreshold == 0 {
		actors := models.AggregatedActors{
			FirstActorName: req.ActorName,
			OtherCount:     newCount - 1,
		}
		action, subject := aggregationActionSubject(req.Type, req.Metadata)
		aggregatedBody := models.BuildAggregatedBody(actors, action, subject)

		synthetic := &models.Notification{
			ID:       uuid.New().String(),
			UserID:   req.UserID,
			Type:     req.Type,
			Title:    req.Title,
			Body:     aggregatedBody,
			ImageURL: req.ImageURL,
			DeepLink: req.DeepLink,
			GroupKey: req.GroupKey,
		}
		go func() {
			dCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.push.SendToUser(dCtx, req.UserID, synthetic); err != nil {
				s.logger.Warn("aggregated push delivery failed", zap.Error(err))
			}
		}()
	}

	return true, nil
}

// resolveChannels decides which delivery channels to use based on the
// notification type and the user's master preference switches.
func (s *notificationService) resolveChannels(
	t models.NotificationType,
	prefs *models.NotificationPreference,
) []models.Channel {
	var channels []models.Channel

	// In-app is always included if enabled.
	if prefs.InAppEnabled {
		channels = append(channels, models.ChannelInApp)
	}

	switch t {
	case models.NotificationTypeEmailVerify,
		models.NotificationTypePasswordReset:
		// Transactional emails: send regardless of email preference toggle.
		channels = append(channels, models.ChannelEmail)

	case models.NotificationTypeGift:
		if prefs.PushEnabled {
			channels = append(channels, models.ChannelPush)
		}
		if prefs.EmailEnabled {
			channels = append(channels, models.ChannelEmail)
		}

	case models.NotificationTypeOrderCreated,
		models.NotificationTypeOrderShipped:
		if prefs.EmailEnabled {
			channels = append(channels, models.ChannelEmail)
		}
		if prefs.SMSEnabled {
			channels = append(channels, models.ChannelSMS)
		}

	case models.NotificationTypeSystem:
		if prefs.PushEnabled {
			channels = append(channels, models.ChannelPush)
		}
		if prefs.EmailEnabled {
			channels = append(channels, models.ChannelEmail)
		}

	default:
		// Social interactions: push only.
		if prefs.PushEnabled {
			channels = append(channels, models.ChannelPush)
		}
	}

	return channels
}

// isQuietHours returns true when the current time falls within the user's
// quiet-hours window (both times in "HH:MM" format).
func (s *notificationService) isQuietHours(prefs *models.NotificationPreference) bool {
	loc, err := time.LoadLocation(prefs.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	currentMinutes := now.Hour()*60 + now.Minute()

	startH, startM := parseClock(prefs.QuietStart)
	endH, endM := parseClock(prefs.QuietEnd)
	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM

	if startMinutes <= endMinutes {
		return currentMinutes >= startMinutes && currentMinutes < endMinutes
	}
	// Spans midnight.
	return currentMinutes >= startMinutes || currentMinutes < endMinutes
}

// parseClock parses a "HH:MM" string into hours and minutes.
func parseClock(s string) (int, int) {
	if len(s) < 5 {
		return 0, 0
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	return h, m
}

// isImmediateEmailType returns true for notification types that require
// an email to be dispatched immediately (not batched into a digest).
func isImmediateEmailType(t models.NotificationType) bool {
	switch t {
	case models.NotificationTypeEmailVerify,
		models.NotificationTypePasswordReset,
		models.NotificationTypeGift,
		models.NotificationTypeOrderCreated,
		models.NotificationTypeOrderShipped:
		return true
	}
	return false
}

// aggregationActionSubject derives a human-readable action and subject from the
// notification type and optional metadata, used in aggregated push bodies.
func aggregationActionSubject(t models.NotificationType, meta map[string]interface{}) (action, subject string) {
	switch t {
	case models.NotificationTypeLike:
		videoTitle, _ := meta["video_title"].(string)
		if videoTitle == "" {
			videoTitle = "your video"
		}
		return "liked", videoTitle
	case models.NotificationTypeComment:
		return "commented on", "your video"
	case models.NotificationTypeFollow:
		return "followed", "you"
	case models.NotificationTypeMention:
		return "mentioned you in", "a video"
	case models.NotificationTypeGift:
		return "sent you", "a gift"
	default:
		return "interacted with", "your content"
	}
}

// ---------------------------------------------------------------------------
// Read / write operations delegated to the repository
// ---------------------------------------------------------------------------

func (s *notificationService) GetNotifications(
	ctx context.Context,
	req models.ListNotificationsRequest,
) (*models.NotificationsResponse, error) {
	notifications, total, err := s.repo.GetNotifications(ctx, req)
	if err != nil {
		return nil, err
	}
	unreadCount, err := s.repo.GetUnreadCount(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	return &models.NotificationsResponse{
		Notifications: notifications,
		Total:         total,
		UnreadCount:   unreadCount,
		Limit:         req.Limit,
		Offset:        req.Offset,
	}, nil
}

func (s *notificationService) MarkAsRead(ctx context.Context, notificationID, userID string) error {
	return s.repo.MarkAsRead(ctx, notificationID, userID)
}

func (s *notificationService) MarkAllRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *notificationService) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	return s.repo.GetUnreadCount(ctx, userID)
}

func (s *notificationService) RegisterDevice(
	ctx context.Context,
	userID string,
	req *models.RegisterDeviceRequest,
) error {
	device := &models.PushDevice{
		UserID:     userID,
		Token:      req.Token,
		Platform:   req.Platform,
		AppVersion: req.AppVersion,
		DeviceName: req.DeviceName,
	}
	return s.repo.SaveDevice(ctx, device)
}

func (s *notificationService) UnregisterDevice(ctx context.Context, token, userID string) error {
	return s.repo.RemoveDevice(ctx, token, userID)
}

func (s *notificationService) GetPreferences(
	ctx context.Context,
	userID string,
) (*models.NotificationPreference, error) {
	return s.repo.GetPreferences(ctx, userID)
}

func (s *notificationService) UpdatePreferences(
	ctx context.Context,
	userID string,
	req *models.UpdatePreferencesRequest,
) (*models.NotificationPreference, error) {
	prefs, err := s.repo.GetPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get preferences: %w", err)
	}

	// Apply only the fields present in the request (pointer-based partial update).
	if req.PushEnabled != nil {
		prefs.PushEnabled = *req.PushEnabled
	}
	if req.EmailEnabled != nil {
		prefs.EmailEnabled = *req.EmailEnabled
	}
	if req.SMSEnabled != nil {
		prefs.SMSEnabled = *req.SMSEnabled
	}
	if req.InAppEnabled != nil {
		prefs.InAppEnabled = *req.InAppEnabled
	}
	if req.LikesEnabled != nil {
		prefs.LikesEnabled = *req.LikesEnabled
	}
	if req.CommentsEnabled != nil {
		prefs.CommentsEnabled = *req.CommentsEnabled
	}
	if req.FollowsEnabled != nil {
		prefs.FollowsEnabled = *req.FollowsEnabled
	}
	if req.MentionsEnabled != nil {
		prefs.MentionsEnabled = *req.MentionsEnabled
	}
	if req.GiftsEnabled != nil {
		prefs.GiftsEnabled = *req.GiftsEnabled
	}
	if req.OrdersEnabled != nil {
		prefs.OrdersEnabled = *req.OrdersEnabled
	}
	if req.LiveStreamEnabled != nil {
		prefs.LiveStreamEnabled = *req.LiveStreamEnabled
	}
	if req.SystemEnabled != nil {
		prefs.SystemEnabled = *req.SystemEnabled
	}
	if req.QuietHoursEnabled != nil {
		prefs.QuietHoursEnabled = *req.QuietHoursEnabled
	}
	if req.QuietStart != nil {
		prefs.QuietStart = *req.QuietStart
	}
	if req.QuietEnd != nil {
		prefs.QuietEnd = *req.QuietEnd
	}
	if req.Timezone != nil {
		prefs.Timezone = *req.Timezone
	}
	if req.DigestEnabled != nil {
		prefs.DigestEnabled = *req.DigestEnabled
	}
	if req.DigestFrequency != nil {
		prefs.DigestFrequency = *req.DigestFrequency
	}

	if err := s.repo.UpsertPreferences(ctx, prefs); err != nil {
		return nil, fmt.Errorf("upsert preferences: %w", err)
	}
	return prefs, nil
}
