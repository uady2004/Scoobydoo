package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"go.uber.org/zap"

	"github.com/tiktok-clone/notification-service/internal/config"
)

// EmailService sends transactional emails via SendGrid dynamic templates.
type EmailService interface {
	// SendEmailVerification sends an email verification link to the given address.
	SendEmailVerification(ctx context.Context, toEmail, toName, verificationURL string) error
	// SendPasswordReset sends a password-reset link email.
	SendPasswordReset(ctx context.Context, toEmail, toName, resetURL string) error
	// SendWeeklyDigest sends a weekly activity digest email.
	SendWeeklyDigest(ctx context.Context, toEmail, toName string, data WeeklyDigestData) error
	// SendGiftReceived notifies a creator that they received a gift.
	SendGiftReceived(ctx context.Context, toEmail, toName string, data GiftEmailData) error
	// SendOrderConfirmation sends an order-created confirmation email.
	SendOrderConfirmation(ctx context.Context, toEmail, toName string, data OrderEmailData) error
}

// WeeklyDigestData contains the dynamic template variables for the weekly digest.
type WeeklyDigestData struct {
	NewFollowers    int    `json:"new_followers"`
	TotalLikes      int    `json:"total_likes"`
	TotalViews      int    `json:"total_views"`
	TotalComments   int    `json:"total_comments"`
	TopVideoTitle   string `json:"top_video_title"`
	TopVideoViews   int    `json:"top_video_views"`
	ProfileURL      string `json:"profile_url"`
	UnsubscribeURL  string `json:"unsubscribe_url"`
}

// GiftEmailData holds the variables for a gift-received email.
type GiftEmailData struct {
	SenderName  string  `json:"sender_name"`
	GiftName    string  `json:"gift_name"`
	GiftValue   float64 `json:"gift_value"`
	Currency    string  `json:"currency"`
	VideoTitle  string  `json:"video_title"`
	DashboardURL string `json:"dashboard_url"`
}

// OrderEmailData holds the variables for an order confirmation email.
type OrderEmailData struct {
	OrderID      string  `json:"order_id"`
	ProductName  string  `json:"product_name"`
	Quantity     int     `json:"quantity"`
	TotalAmount  float64 `json:"total_amount"`
	Currency     string  `json:"currency"`
	TrackingURL  string  `json:"tracking_url,omitempty"`
	SupportEmail string  `json:"support_email"`
}

type sendGridEmailService struct {
	client    *sendgrid.Client
	cfg       config.SendGridConfig
	logger    *zap.Logger
}

// NewEmailService creates a SendGrid-backed EmailService.
func NewEmailService(cfg config.SendGridConfig, logger *zap.Logger) EmailService {
	client := sendgrid.NewSendClient(cfg.APIKey)
	return &sendGridEmailService{client: client, cfg: cfg, logger: logger}
}

// newDynamicMail builds a SendGrid v3 mail with a dynamic template.
func (s *sendGridEmailService) newDynamicMail(
	toEmail, toName, templateID string,
	dynamicData map[string]interface{},
) *mail.SGMailV3 {
	from := mail.NewEmail(s.cfg.FromName, s.cfg.FromEmail)
	to := mail.NewEmail(toName, toEmail)

	m := mail.NewV3Mail()
	m.SetFrom(from)
	m.SetTemplateID(templateID)

	p := mail.NewPersonalization()
	p.AddTos(to)
	for k, v := range dynamicData {
		p.SetDynamicTemplateData(k, v)
	}
	m.AddPersonalizations(p)
	return m
}

func (s *sendGridEmailService) send(ctx context.Context, m *mail.SGMailV3) error {
	resp, err := s.client.SendWithContext(ctx, m)
	if err != nil {
		return fmt.Errorf("sendgrid send: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		s.logger.Error("sendgrid error response",
			zap.Int("status", resp.StatusCode),
			zap.String("body", resp.Body),
		)
		return fmt.Errorf("sendgrid response %d: %s", resp.StatusCode, resp.Body)
	}
	s.logger.Info("email sent",
		zap.String("to", m.Personalizations[0].To[0].Address),
		zap.String("template_id", m.TemplateID),
		zap.Int("status", resp.StatusCode),
	)
	return nil
}

func (s *sendGridEmailService) SendEmailVerification(
	ctx context.Context, toEmail, toName, verificationURL string,
) error {
	data := map[string]interface{}{
		"name":             toName,
		"verification_url": verificationURL,
	}
	m := s.newDynamicMail(toEmail, toName, s.cfg.Templates.EmailVerification, data)
	return s.send(ctx, m)
}

func (s *sendGridEmailService) SendPasswordReset(
	ctx context.Context, toEmail, toName, resetURL string,
) error {
	data := map[string]interface{}{
		"name":      toName,
		"reset_url": resetURL,
	}
	m := s.newDynamicMail(toEmail, toName, s.cfg.Templates.PasswordReset, data)
	return s.send(ctx, m)
}

func (s *sendGridEmailService) SendWeeklyDigest(
	ctx context.Context, toEmail, toName string, digestData WeeklyDigestData,
) error {
	data := map[string]interface{}{
		"name":            toName,
		"new_followers":   digestData.NewFollowers,
		"total_likes":     digestData.TotalLikes,
		"total_views":     digestData.TotalViews,
		"total_comments":  digestData.TotalComments,
		"top_video_title": digestData.TopVideoTitle,
		"top_video_views": digestData.TopVideoViews,
		"profile_url":     digestData.ProfileURL,
		"unsubscribe_url": digestData.UnsubscribeURL,
	}
	m := s.newDynamicMail(toEmail, toName, s.cfg.Templates.WeeklyDigest, data)
	return s.send(ctx, m)
}

func (s *sendGridEmailService) SendGiftReceived(
	ctx context.Context, toEmail, toName string, giftData GiftEmailData,
) error {
	data := map[string]interface{}{
		"recipient_name": toName,
		"sender_name":    giftData.SenderName,
		"gift_name":      giftData.GiftName,
		"gift_value":     giftData.GiftValue,
		"currency":       giftData.Currency,
		"video_title":    giftData.VideoTitle,
		"dashboard_url":  giftData.DashboardURL,
	}
	m := s.newDynamicMail(toEmail, toName, s.cfg.Templates.GiftReceived, data)
	return s.send(ctx, m)
}

func (s *sendGridEmailService) SendOrderConfirmation(
	ctx context.Context, toEmail, toName string, orderData OrderEmailData,
) error {
	data := map[string]interface{}{
		"name":          toName,
		"order_id":      orderData.OrderID,
		"product_name":  orderData.ProductName,
		"quantity":      orderData.Quantity,
		"total_amount":  orderData.TotalAmount,
		"currency":      orderData.Currency,
		"tracking_url":  orderData.TrackingURL,
		"support_email": orderData.SupportEmail,
	}
	m := s.newDynamicMail(toEmail, toName, s.cfg.Templates.OrderConfirmation, data)
	return s.send(ctx, m)
}
