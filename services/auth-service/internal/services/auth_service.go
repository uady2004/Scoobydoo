package services

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	googleOAuth "google.golang.org/api/idtoken"

	"github.com/tiktok-clone/auth-service/internal/config"
	"github.com/tiktok-clone/auth-service/internal/models"
	"github.com/tiktok-clone/auth-service/internal/repositories"
)

// ── Sentinel errors ────────────────────────────────────────────────────────────

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUserNotFound        = errors.New("user not found")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrInvalidToken        = errors.New("invalid token")
	ErrTokenExpired        = errors.New("token expired")
	ErrSessionRevoked      = errors.New("session revoked")
	ErrOTPInvalid          = errors.New("otp invalid or expired")
	ErrMFARequired         = errors.New("mfa verification required")
	ErrMFAAlreadyEnabled   = errors.New("mfa already enabled")
	ErrInvalidMFACode      = errors.New("invalid mfa code")
	ErrAccountSuspended    = errors.New("account suspended")
)

// EventPublisher emits domain events to a message broker.
type EventPublisher interface {
	Publish(ctx context.Context, topic string, key string, payload []byte) error
}

// EmailSender delivers transactional emails.
type EmailSender interface {
	SendEmailVerification(ctx context.Context, to, token string) error
	SendPasswordReset(ctx context.Context, to, token string) error
}

// SMSSender delivers OTP codes via SMS.
type SMSSender interface {
	SendOTP(ctx context.Context, phone, code string) error
}

// AuthService exposes all authentication use-cases.
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*models.User, *models.TokenPair, error)
	Login(ctx context.Context, req LoginRequest) (*models.User, *models.TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*models.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutAll(ctx context.Context, userID uuid.UUID) error

	GoogleOAuth(ctx context.Context, req GoogleOAuthRequest) (*models.User, *models.TokenPair, error)
	AppleOAuth(ctx context.Context, req AppleOAuthRequest) (*models.User, *models.TokenPair, error)

	SendOTP(ctx context.Context, req SendOTPRequest) error
	VerifyOTP(ctx context.Context, req VerifyOTPRequest) (*models.TokenPair, error)

	SendEmailVerification(ctx context.Context, userID uuid.UUID) error
	VerifyEmail(ctx context.Context, token string) error

	SendPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, req ResetPasswordRequest) error
	ChangePassword(ctx context.Context, req ChangePasswordRequest) error

	EnableMFA(ctx context.Context, userID uuid.UUID) (*MFASetupResponse, error)
	VerifyMFA(ctx context.Context, req VerifyMFARequest) (*models.TokenPair, error)
	DisableMFA(ctx context.Context, userID uuid.UUID, code string) error

	ValidateAccessToken(ctx context.Context, accessToken string) (*models.Claims, error)
}

// ── Request / Response DTOs ───────────────────────────────────────────────────

type RegisterRequest struct {
	Email     string
	Phone     string
	Username  string
	Password  string
	UserAgent string
	IPAddress string
	DeviceID  string
}

type LoginRequest struct {
	Email     string
	Phone     string
	Password  string
	UserAgent string
	IPAddress string
	DeviceID  string
}

type GoogleOAuthRequest struct {
	IDToken   string
	UserAgent string
	IPAddress string
}

type AppleOAuthRequest struct {
	IdentityToken string
	AuthCode      string
	GivenName     string
	FamilyName    string
	UserAgent     string
	IPAddress     string
}

type SendOTPRequest struct {
	Phone string
	Email string
	Type  models.OTPType
}

type VerifyOTPRequest struct {
	UserID uuid.UUID
	Code   string
	Type   models.OTPType
}

type ResetPasswordRequest struct {
	Token       string
	NewPassword string
}

type ChangePasswordRequest struct {
	UserID      uuid.UUID
	OldPassword string
	NewPassword string
}

type MFASetupResponse struct {
	Secret     string `json:"secret"`
	QRCodeURL  string `json:"qr_code_url"`
	BackupCode string `json:"backup_code"`
}

type VerifyMFARequest struct {
	UserID    uuid.UUID
	Code      string
	UserAgent string
	IPAddress string
}

// ── Implementation ────────────────────────────────────────────────────────────

type authService struct {
	repo      repositories.UserRepository
	redis     *redis.Client
	cfg       *config.Config
	publisher EventPublisher
	email     EmailSender
	sms       SMSSender
	log       *zap.Logger
}

// NewAuthService constructs a ready-to-use AuthService.
func NewAuthService(
	repo repositories.UserRepository,
	redisClient *redis.Client,
	cfg *config.Config,
	publisher EventPublisher,
	email EmailSender,
	sms SMSSender,
	log *zap.Logger,
) AuthService {
	return &authService{
		repo:      repo,
		redis:     redisClient,
		cfg:       cfg,
		publisher: publisher,
		email:     email,
		sms:       sms,
		log:       log,
	}
}

// ── Register ──────────────────────────────────────────────────────────────────

func (s *authService) Register(ctx context.Context, req RegisterRequest) (*models.User, *models.TokenPair, error) {
	// Ensure uniqueness before hashing the password (cheap check first).
	if req.Email != "" {
		if _, err := s.repo.FindByEmail(ctx, req.Email); !errors.Is(err, repositories.ErrNotFound) {
			if err == nil {
				return nil, nil, fmt.Errorf("register: email: %w", ErrUserAlreadyExists)
			}
			return nil, nil, fmt.Errorf("register: find by email: %w", err)
		}
	}
	if req.Phone != "" {
		if _, err := s.repo.FindByPhone(ctx, req.Phone); !errors.Is(err, repositories.ErrNotFound) {
			if err == nil {
				return nil, nil, fmt.Errorf("register: phone: %w", ErrUserAlreadyExists)
			}
			return nil, nil, fmt.Errorf("register: find by phone: %w", err)
		}
	}

	// Hash the password.
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("register: hash password: %w", err)
	}
	hashStr := string(hash)

	email := optionalStr(req.Email)
	phone := optionalStr(req.Phone)

	user := &models.User{
		Email:        email,
		Phone:        phone,
		Username:     req.Username,
		PasswordHash: &hashStr,
		Provider:     models.AuthProviderLocal,
		Status:       models.UserStatusActive,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		if errors.Is(err, repositories.ErrDuplicateKey) {
			return nil, nil, fmt.Errorf("register: %w", ErrUserAlreadyExists)
		}
		return nil, nil, fmt.Errorf("register: create user: %w", err)
	}

	// Emit domain event.
	s.emitUserRegistered(ctx, user)

	// Issue a token pair.
	tokens, err := s.issueTokenPair(ctx, user, req.UserAgent, req.IPAddress, req.DeviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("register: issue tokens: %w", err)
	}

	// Kick off async email verification (best-effort).
	if req.Email != "" {
		go func() {
			bgCtx := context.Background()
			if verifyErr := s.SendEmailVerification(bgCtx, user.ID); verifyErr != nil {
				s.log.Warn("register: send email verification", zap.Error(verifyErr))
			}
		}()
	}

	return user, tokens, nil
}

// ── Login ─────────────────────────────────────────────────────────────────────

func (s *authService) Login(ctx context.Context, req LoginRequest) (*models.User, *models.TokenPair, error) {
	var (
		user *models.User
		err  error
	)

	switch {
	case req.Email != "":
		user, err = s.repo.FindByEmail(ctx, req.Email)
	case req.Phone != "":
		user, err = s.repo.FindByPhone(ctx, req.Phone)
	default:
		return nil, nil, fmt.Errorf("login: email or phone required")
	}

	if errors.Is(err, repositories.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("login: find user: %w", err)
	}

	if user.Status == models.UserStatusSuspended {
		return nil, nil, ErrAccountSuspended
	}

	if user.PasswordHash == nil {
		return nil, nil, fmt.Errorf("login: account uses OAuth: %w", ErrInvalidCredentials)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	// If MFA is enabled return a partial state; the caller must complete VerifyMFA.
	if user.MFAEnabled {
		// Cache a short-lived pending MFA token in Redis.
		pendingKey := fmt.Sprintf("mfa:pending:%s", user.ID)
		if err := s.redis.Set(ctx, pendingKey, "1", 10*time.Minute).Err(); err != nil {
			s.log.Warn("login: set mfa pending", zap.Error(err))
		}
		return user, nil, ErrMFARequired
	}

	tokens, err := s.issueTokenPair(ctx, user, req.UserAgent, req.IPAddress, req.DeviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("login: issue tokens: %w", err)
	}
	return user, tokens, nil
}

// ── RefreshToken ──────────────────────────────────────────────────────────────

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*models.TokenPair, error) {
	// Verify the JWT signature first (cheap).
	claims, err := s.parseToken(refreshToken, s.cfg.JWT.RefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("refresh: %w: %w", ErrInvalidToken, err)
	}

	// Load the session from the database using the raw token (which is hashed inside FindSessionByToken).
	session, err := s.repo.FindSessionByToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("refresh: find session: %w", err)
	}

	if session.IsRevoked {
		return nil, ErrSessionRevoked
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Revoke the current session (refresh token rotation).
	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("refresh: revoke old session: %w", err)
	}

	user, err := s.repo.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("refresh: find user: %w", err)
	}

	tokens, err := s.issueTokenPair(ctx, user, session.UserAgent, session.IPAddress, "")
	if err != nil {
		return nil, fmt.Errorf("refresh: issue tokens: %w", err)
	}
	return tokens, nil
}

// ── Logout ────────────────────────────────────────────────────────────────────

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	session, err := s.repo.FindSessionByToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil // already gone
		}
		return fmt.Errorf("logout: find session: %w", err)
	}
	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		return fmt.Errorf("logout: revoke session: %w", err)
	}

	// Blacklist the access token for its remaining TTL via Redis.
	s.blacklistAccessToken(ctx, refreshToken)
	return nil
}

func (s *authService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	if err := s.repo.RevokeAllUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("logout all: %w", err)
	}
	return nil
}

// ── Google OAuth ──────────────────────────────────────────────────────────────

func (s *authService) GoogleOAuth(ctx context.Context, req GoogleOAuthRequest) (*models.User, *models.TokenPair, error) {
	payload, err := googleOAuth.Validate(ctx, req.IDToken, s.cfg.OAuth.Google.ClientID)
	if err != nil {
		return nil, nil, fmt.Errorf("google oauth: validate id token: %w", err)
	}

	providerID := payload.Subject
	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	avatar, _ := payload.Claims["picture"].(string)

	// Attempt to find an existing account linked to this Google identity.
	user, err := s.repo.FindByProviderID(ctx, models.AuthProviderGoogle, providerID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, nil, fmt.Errorf("google oauth: find by provider: %w", err)
	}

	if errors.Is(err, repositories.ErrNotFound) {
		// Check if the email already belongs to a local account; link it.
		if email != "" {
			user, err = s.repo.FindByEmail(ctx, email)
			if err != nil && !errors.Is(err, repositories.ErrNotFound) {
				return nil, nil, fmt.Errorf("google oauth: find by email: %w", err)
			}
		}

		if errors.Is(err, repositories.ErrNotFound) || user == nil {
			// Create a brand-new account.
			emailPtr := optionalStr(email)
			namePtr := optionalStr(name)
			avatarPtr := optionalStr(avatar)
			user = &models.User{
				Email:          emailPtr,
				Username:       generateUsername(email),
				Provider:       models.AuthProviderGoogle,
				ProviderUserID: &providerID,
				EmailVerified:  email != "",
				Status:         models.UserStatusActive,
				DisplayName:    namePtr,
				AvatarURL:      avatarPtr,
			}
			if createErr := s.repo.CreateUser(ctx, user); createErr != nil {
				return nil, nil, fmt.Errorf("google oauth: create user: %w", createErr)
			}
			s.emitUserRegistered(ctx, user)
		}
	}

	tokens, err := s.issueTokenPair(ctx, user, req.UserAgent, req.IPAddress, "")
	if err != nil {
		return nil, nil, fmt.Errorf("google oauth: issue tokens: %w", err)
	}
	return user, tokens, nil
}

// ── Apple OAuth ───────────────────────────────────────────────────────────────

// AppleOAuth validates a Sign-in-with-Apple identity token (JWT from Apple).
// Full JWKS verification is omitted here in favour of a minimal viable
// structure; production code should validate against Apple's public keys.
func (s *authService) AppleOAuth(ctx context.Context, req AppleOAuthRequest) (*models.User, *models.TokenPair, error) {
	// Parse without verification to extract sub / email claims.
	// In production, verify with Apple's JWKS endpoint.
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, _, err := parser.ParseUnverified(req.IdentityToken, jwt.MapClaims{})
	if err != nil {
		return nil, nil, fmt.Errorf("apple oauth: parse token: %w", err)
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, fmt.Errorf("apple oauth: bad claims type")
	}

	providerID, _ := mapClaims["sub"].(string)
	email, _ := mapClaims["email"].(string)
	if providerID == "" {
		return nil, nil, fmt.Errorf("apple oauth: missing sub claim")
	}

	user, err := s.repo.FindByProviderID(ctx, models.AuthProviderApple, providerID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, nil, fmt.Errorf("apple oauth: find by provider: %w", err)
	}

	if errors.Is(err, repositories.ErrNotFound) {
		emailPtr := optionalStr(email)
		displayName := strings.TrimSpace(req.GivenName + " " + req.FamilyName)
		displayNamePtr := optionalStr(displayName)
		user = &models.User{
			Email:          emailPtr,
			Username:       generateUsername(email),
			Provider:       models.AuthProviderApple,
			ProviderUserID: &providerID,
			EmailVerified:  email != "",
			Status:         models.UserStatusActive,
			DisplayName:    displayNamePtr,
		}
		if createErr := s.repo.CreateUser(ctx, user); createErr != nil {
			return nil, nil, fmt.Errorf("apple oauth: create user: %w", createErr)
		}
		s.emitUserRegistered(ctx, user)
	}

	tokens, err := s.issueTokenPair(ctx, user, req.UserAgent, req.IPAddress, "")
	if err != nil {
		return nil, nil, fmt.Errorf("apple oauth: issue tokens: %w", err)
	}
	return user, tokens, nil
}

// ── OTP ───────────────────────────────────────────────────────────────────────

func (s *authService) SendOTP(ctx context.Context, req SendOTPRequest) error {
	var (
		user   *models.User
		target string
		err    error
	)

	switch {
	case req.Phone != "":
		user, err = s.repo.FindByPhone(ctx, req.Phone)
		target = req.Phone
	case req.Email != "":
		user, err = s.repo.FindByEmail(ctx, req.Email)
		target = req.Email
	default:
		return fmt.Errorf("send otp: phone or email required")
	}

	if errors.Is(err, repositories.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("send otp: find user: %w", err)
	}

	// Rate-limit: one OTP per minute per user-type in Redis.
	rateLimitKey := fmt.Sprintf("otp:rl:%s:%s", user.ID, req.Type)
	if set, rlErr := s.redis.SetNX(ctx, rateLimitKey, "1", time.Minute).Result(); rlErr == nil && !set {
		return fmt.Errorf("send otp: rate limited")
	}

	// Invalidate previous codes of the same type.
	_ = s.repo.InvalidateOTPs(ctx, user.ID, req.Type)

	code, err := generateOTPCode(6)
	if err != nil {
		return fmt.Errorf("send otp: generate code: %w", err)
	}

	otpRecord := &models.OTPCode{
		UserID:    user.ID,
		Code:      code,
		Type:      req.Type,
		Target:    target,
		MaxTrials: 5,
		ExpiresAt: time.Now().Add(s.cfg.Redis.OTPExpiry),
	}
	if createErr := s.repo.CreateOTP(ctx, otpRecord); createErr != nil {
		return fmt.Errorf("send otp: create record: %w", createErr)
	}

	// Deliver the code.
	if req.Phone != "" && s.sms != nil {
		if smsErr := s.sms.SendOTP(ctx, req.Phone, code); smsErr != nil {
			s.log.Error("send otp: sms delivery", zap.Error(smsErr))
		}
	}
	return nil
}

func (s *authService) VerifyOTP(ctx context.Context, req VerifyOTPRequest) (*models.TokenPair, error) {
	valid, err := s.repo.ValidateOTP(ctx, req.UserID, req.Code, req.Type)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrOTPInvalid
		}
		return nil, fmt.Errorf("verify otp: %w", err)
	}
	if !valid {
		return nil, ErrOTPInvalid
	}

	user, err := s.repo.FindByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("verify otp: find user: %w", err)
	}

	// Auto-verify the channel used.
	switch req.Type {
	case models.OTPTypePhoneVerification:
		_ = s.repo.SetPhoneVerified(ctx, user.ID)
	case models.OTPTypeEmailVerification:
		_ = s.repo.SetEmailVerified(ctx, user.ID)
	}

	tokens, err := s.issueTokenPair(ctx, user, "", "", "")
	if err != nil {
		return nil, fmt.Errorf("verify otp: issue tokens: %w", err)
	}
	return tokens, nil
}

// ── Email verification ────────────────────────────────────────────────────────

func (s *authService) SendEmailVerification(ctx context.Context, userID uuid.UUID) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("send email verification: %w", err)
	}
	if user.Email == nil {
		return fmt.Errorf("send email verification: user has no email")
	}

	token, err := s.signEmailToken(userID, "email_verify", 24*time.Hour)
	if err != nil {
		return fmt.Errorf("send email verification: sign token: %w", err)
	}

	if s.email != nil {
		if sendErr := s.email.SendEmailVerification(ctx, *user.Email, token); sendErr != nil {
			return fmt.Errorf("send email verification: deliver: %w", sendErr)
		}
	}
	return nil
}

func (s *authService) VerifyEmail(ctx context.Context, token string) error {
	userID, err := s.validateEmailToken(token, "email_verify")
	if err != nil {
		return fmt.Errorf("verify email: %w", err)
	}
	if err := s.repo.SetEmailVerified(ctx, userID); err != nil {
		return fmt.Errorf("verify email: update: %w", err)
	}
	return nil
}

// ── Password management ───────────────────────────────────────────────────────

func (s *authService) SendPasswordReset(ctx context.Context, email string) error {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			// Return nil to prevent user enumeration.
			return nil
		}
		return fmt.Errorf("send password reset: find user: %w", err)
	}

	token, err := s.signEmailToken(user.ID, "password_reset", time.Hour)
	if err != nil {
		return fmt.Errorf("send password reset: sign token: %w", err)
	}

	if s.email != nil {
		if sendErr := s.email.SendPasswordReset(ctx, email, token); sendErr != nil {
			return fmt.Errorf("send password reset: deliver: %w", sendErr)
		}
	}
	return nil
}

func (s *authService) ResetPassword(ctx context.Context, req ResetPasswordRequest) error {
	userID, err := s.validateEmailToken(req.Token, "password_reset")
	if err != nil {
		return fmt.Errorf("reset password: %w", ErrInvalidToken)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("reset password: hash: %w", err)
	}

	if err := s.repo.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("reset password: update: %w", err)
	}

	// Revoke all sessions after a password reset.
	_ = s.repo.RevokeAllUserSessions(ctx, userID)
	return nil
}

func (s *authService) ChangePassword(ctx context.Context, req ChangePasswordRequest) error {
	user, err := s.repo.FindByID(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("change password: find user: %w", err)
	}
	if user.PasswordHash == nil {
		return fmt.Errorf("change password: OAuth account")
	}
	if bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.OldPassword)) != nil {
		return ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("change password: hash: %w", err)
	}
	if err := s.repo.UpdatePassword(ctx, req.UserID, string(hash)); err != nil {
		return fmt.Errorf("change password: update: %w", err)
	}
	return nil
}

// ── MFA (TOTP) ────────────────────────────────────────────────────────────────

func (s *authService) EnableMFA(ctx context.Context, userID uuid.UUID) (*MFASetupResponse, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("enable mfa: find user: %w", err)
	}
	if user.MFAEnabled {
		return nil, ErrMFAAlreadyEnabled
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.cfg.MFA.Issuer,
		AccountName: user.Username,
		SecretSize:  20,
		Digits:      otp.DigitsSix,
		Period:      s.cfg.MFA.Period,
	})
	if err != nil {
		return nil, fmt.Errorf("enable mfa: generate key: %w", err)
	}

	// Persist the TOTP secret (not yet "enabled" – caller must VerifyMFA first).
	if err := s.repo.UpdateMFASecret(ctx, userID, key.Secret(), false); err != nil {
		return nil, fmt.Errorf("enable mfa: save secret: %w", err)
	}

	return &MFASetupResponse{
		Secret:    key.Secret(),
		QRCodeURL: key.URL(),
	}, nil
}

func (s *authService) VerifyMFA(ctx context.Context, req VerifyMFARequest) (*models.TokenPair, error) {
	user, err := s.repo.FindByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("verify mfa: find user: %w", err)
	}
	if user.MFASecret == nil {
		return nil, fmt.Errorf("verify mfa: mfa not configured")
	}

	valid := totp.Validate(req.Code, *user.MFASecret)
	if !valid {
		return nil, ErrInvalidMFACode
	}

	// If MFA was being set up (not yet enabled), enable it now.
	if !user.MFAEnabled {
		if err := s.repo.UpdateMFASecret(ctx, user.ID, *user.MFASecret, true); err != nil {
			return nil, fmt.Errorf("verify mfa: enable: %w", err)
		}
	}

	// Clear the pending MFA marker.
	pendingKey := fmt.Sprintf("mfa:pending:%s", user.ID)
	_ = s.redis.Del(ctx, pendingKey)

	tokens, err := s.issueTokenPair(ctx, user, req.UserAgent, req.IPAddress, "")
	if err != nil {
		return nil, fmt.Errorf("verify mfa: issue tokens: %w", err)
	}
	return tokens, nil
}

func (s *authService) DisableMFA(ctx context.Context, userID uuid.UUID, code string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("disable mfa: find user: %w", err)
	}
	if !user.MFAEnabled || user.MFASecret == nil {
		return fmt.Errorf("disable mfa: mfa not enabled")
	}
	if !totp.Validate(code, *user.MFASecret) {
		return ErrInvalidMFACode
	}
	if err := s.repo.UpdateMFASecret(ctx, userID, "", false); err != nil {
		return fmt.Errorf("disable mfa: update: %w", err)
	}
	return nil
}

// ── Access token validation ───────────────────────────────────────────────────

func (s *authService) ValidateAccessToken(_ context.Context, accessToken string) (*models.Claims, error) {
	claims, err := s.parseToken(accessToken, s.cfg.JWT.AccessSecret)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// issueTokenPair mints a new access+refresh pair and persists a session record.
func (s *authService) issueTokenPair(ctx context.Context, user *models.User, userAgent, ipAddr, deviceID string) (*models.TokenPair, error) {
	now := time.Now().UTC()
	accessExp := now.Add(s.cfg.JWT.AccessTTL)
	refreshExp := now.Add(s.cfg.JWT.RefreshTTL)

	emailStr := ""
	if user.Email != nil {
		emailStr = *user.Email
	}
	phoneStr := ""
	if user.Phone != nil {
		phoneStr = *user.Phone
	}

	// Access token
	atClaims := jwt.MapClaims{
		"iss":      s.cfg.JWT.Issuer,
		"sub":      user.ID.String(),
		"uid":      user.ID.String(),
		"email":    emailStr,
		"phone":    phoneStr,
		"username": user.Username,
		"provider": string(user.Provider),
		"mfa_done": user.MFAEnabled, // considered done once account is loaded post-MFA
		"iat":      now.Unix(),
		"exp":      accessExp.Unix(),
	}
	atToken := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	accessTokenStr, err := atToken.SignedString([]byte(s.cfg.JWT.AccessSecret))
	if err != nil {
		return nil, fmt.Errorf("issue token pair: sign access: %w", err)
	}

	// Refresh token (contains minimal claims – just sub + session jti)
	sessionID := uuid.New()
	rtClaims := jwt.MapClaims{
		"iss": s.cfg.JWT.Issuer,
		"sub": user.ID.String(),
		"uid": user.ID.String(),
		"jti": sessionID.String(),
		"iat": now.Unix(),
		"exp": refreshExp.Unix(),
	}
	rtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	refreshTokenStr, err := rtToken.SignedString([]byte(s.cfg.JWT.RefreshSecret))
	if err != nil {
		return nil, fmt.Errorf("issue token pair: sign refresh: %w", err)
	}

	deviceIDPtr := optionalStr(deviceID)
	session := &models.Session{
		ID:           sessionID,
		UserID:       user.ID,
		RefreshToken: refreshTokenStr, // CreateSession will hash it
		UserAgent:    userAgent,
		IPAddress:    ipAddr,
		DeviceID:     deviceIDPtr,
		ExpiresAt:    refreshExp,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("issue token pair: create session: %w", err)
	}

	return &models.TokenPair{
		AccessToken:  accessTokenStr,
		RefreshToken: refreshTokenStr,
		ExpiresAt:    accessExp,
		TokenType:    "Bearer",
	}, nil
}

// parseToken validates a signed JWT and returns the extracted Claims.
func (s *authService) parseToken(tokenStr, secret string) (*models.Claims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithIssuedAt(), jwt.WithIssuer(s.cfg.JWT.Issuer))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	userIDStr, _ := mapClaims["uid"].(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("parse token: bad uid: %w", err)
	}

	claims := &models.Claims{
		UserID:   userID,
		Email:    stringClaim(mapClaims, "email"),
		Phone:    stringClaim(mapClaims, "phone"),
		Username: stringClaim(mapClaims, "username"),
		Provider: stringClaim(mapClaims, "provider"),
		MFADone:  boolClaim(mapClaims, "mfa_done"),
	}
	return claims, nil
}

// signEmailToken creates a short-lived, purpose-scoped JWT for email links.
func (s *authService) signEmailToken(userID uuid.UUID, purpose string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":     userID.String(),
		"purpose": purpose,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(ttl).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(s.cfg.JWT.AccessSecret))
}

// validateEmailToken parses and verifies a purpose-scoped email JWT.
func (s *authService) validateEmailToken(tokenStr, expectedPurpose string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWT.AccessSecret), nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("validate email token: %w", err)
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("validate email token: invalid")
	}

	purpose, _ := mapClaims["purpose"].(string)
	if purpose != expectedPurpose {
		return uuid.Nil, fmt.Errorf("validate email token: wrong purpose")
	}

	sub, _ := mapClaims["sub"].(string)
	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, fmt.Errorf("validate email token: bad sub")
	}
	return userID, nil
}

// blacklistAccessToken stores a token in Redis so middleware can reject it.
func (s *authService) blacklistAccessToken(ctx context.Context, token string) {
	key := fmt.Sprintf("auth:blacklist:%s", token)
	_ = s.redis.Set(ctx, key, "1", s.cfg.JWT.AccessTTL)
}

// emitUserRegistered publishes a UserRegistered domain event (best-effort).
func (s *authService) emitUserRegistered(ctx context.Context, user *models.User) {
	if s.publisher == nil {
		return
	}
	email := ""
	if user.Email != nil {
		email = *user.Email
	}
	payload := fmt.Sprintf(
		`{"user_id":%q,"email":%q,"username":%q,"provider":%q,"created_at":%q}`,
		user.ID, email, user.Username, user.Provider, user.CreatedAt.Format(time.RFC3339),
	)
	if err := s.publisher.Publish(ctx, s.cfg.Kafka.UserEventTopic, user.ID.String(), []byte(payload)); err != nil {
		s.log.Warn("emit user registered: publish", zap.Error(err))
	}
}

// ── Stand-alone utilities ─────────────────────────────────────────────────────

func optionalStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func generateOTPCode(length int) (string, error) {
	const digits = "0123456789"
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[n.Int64()]
	}
	return string(code), nil
}

// generateUsername builds a deterministic username from an email address
// or falls back to a random base32 string.
func generateUsername(email string) string {
	if email == "" {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		return "user_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
	}
	parts := strings.SplitN(email, "@", 2)
	return strings.ToLower(parts[0])
}

func stringClaim(m jwt.MapClaims, key string) string {
	v, _ := m[key].(string)
	return v
}

func boolClaim(m jwt.MapClaims, key string) bool {
	v, _ := m[key].(bool)
	return v
}
