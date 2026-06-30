package services

import (
	"context"
	"fmt"

	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
	"go.uber.org/zap"

	"github.com/tiktok-clone/notification-service/internal/config"
)

// SMSService sends SMS messages via Twilio.
type SMSService interface {
	// SendOTP delivers a one-time password code to a phone number.
	SendOTP(ctx context.Context, toPhone, code string) error
	// SendAlert sends a plain-text important alert message.
	SendAlert(ctx context.Context, toPhone, message string) error
	// SendCustom sends an arbitrary SMS body to a phone number.
	SendCustom(ctx context.Context, toPhone, body string) error
}

type twilioSMSService struct {
	client *twilio.RestClient
	cfg    config.TwilioConfig
	logger *zap.Logger
}

// NewSMSService creates a Twilio-backed SMSService.
func NewSMSService(cfg config.TwilioConfig, logger *zap.Logger) SMSService {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: cfg.AccountSID,
		Password: cfg.AuthToken,
	})
	return &twilioSMSService{client: client, cfg: cfg, logger: logger}
}

func (s *twilioSMSService) send(ctx context.Context, toPhone, body string) error {
	params := &twilioApi.CreateMessageParams{}
	params.SetTo(toPhone)
	params.SetBody(body)

	// Prefer the messaging service SID when configured; otherwise use the from number.
	if s.cfg.MessagingServiceSID != "" {
		params.SetMessagingServiceSid(s.cfg.MessagingServiceSID)
	} else {
		params.SetFrom(s.cfg.FromNumber)
	}

	resp, err := s.client.Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("twilio send sms: %w", err)
	}

	sid := ""
	if resp.Sid != nil {
		sid = *resp.Sid
	}
	status := ""
	if resp.Status != nil {
		status = *resp.Status
	}

	s.logger.Info("sms sent",
		zap.String("to", toPhone),
		zap.String("sid", sid),
		zap.String("status", status),
	)
	return nil
}

// SendOTP formats an OTP delivery message and sends it.
// The OTP expiry is taken from configuration so the message reflects the
// actual validity window.
func (s *twilioSMSService) SendOTP(ctx context.Context, toPhone, code string) error {
	expiryMinutes := s.cfg.OTPExpirySeconds / 60
	if expiryMinutes < 1 {
		expiryMinutes = 5
	}
	body := fmt.Sprintf(
		"Your TikTok Clone verification code is: %s\nThis code expires in %d minutes. Do not share it with anyone.",
		code,
		expiryMinutes,
	)
	return s.send(ctx, toPhone, body)
}

// SendAlert sends a brief alert prefixed with a visual indicator.
func (s *twilioSMSService) SendAlert(ctx context.Context, toPhone, message string) error {
	body := fmt.Sprintf("[TikTok Clone Alert] %s", message)
	return s.send(ctx, toPhone, body)
}

// SendCustom sends an arbitrary message body without any prefix.
func (s *twilioSMSService) SendCustom(ctx context.Context, toPhone, body string) error {
	return s.send(ctx, toPhone, body)
}
