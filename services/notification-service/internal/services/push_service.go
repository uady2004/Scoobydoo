package services

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/zap"
	"google.golang.org/api/option"

	"github.com/tiktok-clone/notification-service/internal/config"
	"github.com/tiktok-clone/notification-service/internal/models"
	"github.com/tiktok-clone/notification-service/internal/repositories"
)

// PushService sends Firebase Cloud Messaging (FCM v1) push notifications.
type PushService interface {
	// SendToDevice sends a push notification to a single FCM token.
	SendToDevice(ctx context.Context, token string, n *models.Notification) error
	// SendToUser fans out a notification to all active devices registered for a user.
	// Invalid tokens encountered during delivery are automatically deactivated.
	SendToUser(ctx context.Context, userID string, n *models.Notification) error
	// SendMulticast sends to up to 500 tokens in a single FCM API call.
	SendMulticast(ctx context.Context, tokens []string, n *models.Notification) error
}

type fcmPushService struct {
	client *messaging.Client
	repo   repositories.NotificationRepository
	logger *zap.Logger
}

// NewPushService initialises the Firebase Admin SDK and returns a PushService.
// It accepts either a credentials file path or a raw JSON blob from config.
func NewPushService(
	cfg config.FirebaseConfig,
	repo repositories.NotificationRepository,
	logger *zap.Logger,
) (PushService, error) {
	var opt option.ClientOption
	if cfg.CredentialsJSON != "" {
		opt = option.WithCredentialsJSON([]byte(cfg.CredentialsJSON))
	} else {
		opt = option.WithCredentialsFile(cfg.CredentialsFile)
	}

	app, err := firebase.NewApp(context.Background(), &firebase.Config{
		ProjectID: cfg.ProjectID,
	}, opt)
	if err != nil {
		return nil, fmt.Errorf("firebase.NewApp: %w", err)
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("firebase.Messaging: %w", err)
	}

	return &fcmPushService{client: client, repo: repo, logger: logger}, nil
}

// buildFCMMessage converts a Notification into a firebase messaging.Message.
func buildFCMMessage(token string, n *models.Notification) *messaging.Message {
	msg := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: n.Title,
			Body:  n.Body,
		},
		Data: map[string]string{
			"notification_id": n.ID,
			"type":            string(n.Type),
			"deep_link":       n.DeepLink,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Icon:        "ic_notification",
				Color:       "#FE2C55",
				ClickAction: "FLUTTER_NOTIFICATION_CLICK",
				ImageURL:    n.ImageURL,
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound:            "default",
					MutableContent:   true,
					ContentAvailable: true,
				},
			},
			Headers: map[string]string{
				"apns-priority": "10",
			},
		},
		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: n.Title,
				Body:  n.Body,
				Icon:  n.ImageURL,
			},
		},
	}

	if n.ImageURL != "" {
		msg.Notification.ImageURL = n.ImageURL
	}

	return msg
}

// buildFCMMulticastMessage creates a MulticastMessage for up to 500 tokens.
func buildFCMMulticastMessage(tokens []string, n *models.Notification) *messaging.MulticastMessage {
	return &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title:    n.Title,
			Body:     n.Body,
			ImageURL: n.ImageURL,
		},
		Data: map[string]string{
			"notification_id": n.ID,
			"type":            string(n.Type),
			"deep_link":       n.DeepLink,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Icon:        "ic_notification",
				Color:       "#FE2C55",
				ClickAction: "FLUTTER_NOTIFICATION_CLICK",
				ImageURL:    n.ImageURL,
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound:          "default",
					MutableContent: true,
				},
			},
		},
	}
}

func (s *fcmPushService) SendToDevice(ctx context.Context, token string, n *models.Notification) error {
	msg := buildFCMMessage(token, n)
	_, err := s.client.Send(ctx, msg)
	if err != nil {
		if isInvalidToken(err) {
			s.logger.Warn("deactivating invalid FCM token", zap.String("token", token))
			_ = s.repo.DeactivateDevice(ctx, token)
			return nil // token gone — not a hard failure for the caller
		}
		return fmt.Errorf("fcm send: %w", err)
	}
	return nil
}

func (s *fcmPushService) SendToUser(ctx context.Context, userID string, n *models.Notification) error {
	devices, err := s.repo.GetDevicesByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get devices for user %s: %w", userID, err)
	}
	if len(devices) == 0 {
		s.logger.Debug("no active devices for user", zap.String("user_id", userID))
		return nil
	}

	tokens := make([]string, 0, len(devices))
	for _, d := range devices {
		tokens = append(tokens, d.Token)
	}

	return s.SendMulticast(ctx, tokens, n)
}

func (s *fcmPushService) SendMulticast(ctx context.Context, tokens []string, n *models.Notification) error {
	if len(tokens) == 0 {
		return nil
	}

	// FCM multicast accepts at most 500 tokens per request.
	const batchSize = 500
	for i := 0; i < len(tokens); i += batchSize {
		end := i + batchSize
		if end > len(tokens) {
			end = len(tokens)
		}
		batch := tokens[i:end]

		msg := buildFCMMulticastMessage(batch, n)
		resp, err := s.client.SendEachForMulticast(ctx, msg)
		if err != nil {
			return fmt.Errorf("fcm multicast send: %w", err)
		}

		// Process per-token results.
		if resp.FailureCount > 0 {
			for j, res := range resp.Responses {
				if res.Error != nil && isInvalidToken(res.Error) {
					badToken := batch[j]
					s.logger.Warn("deactivating invalid FCM token", zap.String("token", badToken))
					_ = s.repo.DeactivateDevice(ctx, badToken)
				}
			}
		}

		s.logger.Info("fcm multicast sent",
			zap.Int("success", resp.SuccessCount),
			zap.Int("failure", resp.FailureCount),
		)
	}
	return nil
}

// isInvalidToken returns true when an FCM error indicates the registration
// token is no longer valid and should be removed from the database.
func isInvalidToken(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	invalidCodes := []string{
		"registration-token-not-registered",
		"invalid-registration-token",
		"invalid-argument",
	}
	for _, code := range invalidCodes {
		if contains(errStr, code) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
