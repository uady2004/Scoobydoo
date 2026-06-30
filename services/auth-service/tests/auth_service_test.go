package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/tiktok-clone/auth-service/internal/config"
	"github.com/tiktok-clone/auth-service/internal/models"
	"github.com/tiktok-clone/auth-service/internal/repositories"
	"github.com/tiktok-clone/auth-service/internal/services"
)

// ── Mock: UserRepository ──────────────────────────────────────────────────────

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) CreateUser(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) FindByPhone(ctx context.Context, phone string) (*models.User, error) {
	args := m.Called(ctx, phone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) FindByProviderID(ctx context.Context, provider models.AuthProvider, providerUserID string) (*models.User, error) {
	args := m.Called(ctx, provider, providerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) UpdateUser(ctx context.Context, user *models.User) error {
	return m.Called(ctx, user).Error(0)
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	return m.Called(ctx, userID, passwordHash).Error(0)
}

func (m *MockUserRepository) UpdateMFASecret(ctx context.Context, userID uuid.UUID, secret string, enabled bool) error {
	return m.Called(ctx, userID, secret, enabled).Error(0)
}

func (m *MockUserRepository) SetEmailVerified(ctx context.Context, userID uuid.UUID) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockUserRepository) SetPhoneVerified(ctx context.Context, userID uuid.UUID) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockUserRepository) CreateSession(ctx context.Context, session *models.Session) error {
	return m.Called(ctx, session).Error(0)
}

func (m *MockUserRepository) FindSession(ctx context.Context, sessionID uuid.UUID) (*models.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Session), args.Error(1)
}

func (m *MockUserRepository) FindSessionByToken(ctx context.Context, refreshTokenHash string) (*models.Session, error) {
	args := m.Called(ctx, refreshTokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Session), args.Error(1)
}

func (m *MockUserRepository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	return m.Called(ctx, sessionID).Error(0)
}

func (m *MockUserRepository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockUserRepository) UpdateSessionLastSeen(ctx context.Context, sessionID uuid.UUID) error {
	return m.Called(ctx, sessionID).Error(0)
}

func (m *MockUserRepository) CreateOTP(ctx context.Context, otp *models.OTPCode) error {
	return m.Called(ctx, otp).Error(0)
}

func (m *MockUserRepository) FindActiveOTP(ctx context.Context, userID uuid.UUID, otpType models.OTPType) (*models.OTPCode, error) {
	args := m.Called(ctx, userID, otpType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OTPCode), args.Error(1)
}

func (m *MockUserRepository) ValidateOTP(ctx context.Context, userID uuid.UUID, code string, otpType models.OTPType) (bool, error) {
	args := m.Called(ctx, userID, code, otpType)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) InvalidateOTPs(ctx context.Context, userID uuid.UUID, otpType models.OTPType) error {
	return m.Called(ctx, userID, otpType).Error(0)
}

func (m *MockUserRepository) IncrementOTPAttempts(ctx context.Context, otpID uuid.UUID) error {
	return m.Called(ctx, otpID).Error(0)
}

func (m *MockUserRepository) UpsertDeviceSession(ctx context.Context, ds *models.DeviceSession) error {
	return m.Called(ctx, ds).Error(0)
}

func (m *MockUserRepository) FindDeviceSession(ctx context.Context, userID uuid.UUID, deviceID string) (*models.DeviceSession, error) {
	args := m.Called(ctx, userID, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DeviceSession), args.Error(1)
}

// ── Mock: EventPublisher ──────────────────────────────────────────────────────

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, topic, key string, payload []byte) error {
	return m.Called(ctx, topic, key, payload).Error(0)
}

// ── Mock: EmailSender ─────────────────────────────────────────────────────────

type MockEmailSender struct {
	mock.Mock
}

func (m *MockEmailSender) SendEmailVerification(ctx context.Context, to, token string) error {
	return m.Called(ctx, to, token).Error(0)
}

func (m *MockEmailSender) SendPasswordReset(ctx context.Context, to, token string) error {
	return m.Called(ctx, to, token).Error(0)
}

// ── Mock: SMSSender ───────────────────────────────────────────────────────────

type MockSMSSender struct {
	mock.Mock
}

func (m *MockSMSSender) SendOTP(ctx context.Context, phone, code string) error {
	return m.Called(ctx, phone, code).Error(0)
}

// ── Test fixtures ─────────────────────────────────────────────────────────────

func testConfig() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			AccessSecret:  "test-access-secret-32-bytes-long!",
			RefreshSecret: "test-refresh-secret-32-bytes-lon!",
			AccessTTL:     15 * time.Minute,
			RefreshTTL:    720 * time.Hour,
			Issuer:        "tiktok-clone-auth",
		},
		MFA: config.MFAConfig{
			Issuer: "TikTokClone",
			Digits: 6,
			Period: 30,
		},
		Redis: config.RedisConfig{
			OTPExpiry:     5 * time.Minute,
			SessionExpiry: 720 * time.Hour,
		},
		Kafka: config.KafkaConfig{
			UserEventTopic: "user-events",
		},
	}
}

func testUser() *models.User {
	email := "alice@example.com"
	hash, _ := bcrypt.GenerateFromPassword([]byte("Password1"), bcrypt.MinCost)
	hashStr := string(hash)
	return &models.User{
		ID:           uuid.New(),
		Email:        &email,
		Username:     "alice",
		PasswordHash: &hashStr,
		Provider:     models.AuthProviderLocal,
		Status:       models.UserStatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// newMockRedis returns a minimal fake redis.Client.
// For real integration tests use miniredis instead.
// We pass nil here and guard in the service wherever redis is called.
// These unit tests use a real miniredis-free path by controlling mock expectations.

// ── Test suite ─────────────────────────────────────────────────────────────────

// newTestService builds an AuthService wired to mocks.
// redisClient is nil — tests that touch Redis-dependent paths (OTP rate-limit,
// MFA pending, blacklist) should be covered by integration tests.
func newTestService(repo repositories.UserRepository) services.AuthService {
	logger, _ := zap.NewDevelopment()
	pub := &MockEventPublisher{}
	pub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	return services.NewAuthService(repo, nil, testConfig(), pub, nil, nil, logger)
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	repo := &MockUserRepository{}
	email := "bob@example.com"

	// No existing user
	repo.On("FindByEmail", mock.Anything, email).Return(nil, repositories.ErrNotFound)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.Session")).Return(nil)

	svc := newTestService(repo)
	user, tokens, err := svc.Register(context.Background(), services.RegisterRequest{
		Email:    email,
		Username: "bob",
		Password: "Password1",
	})

	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, tokens)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	assert.Equal(t, "Bearer", tokens.TokenType)

	repo.AssertExpectations(t)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := &MockUserRepository{}
	existing := testUser()
	email := *existing.Email

	repo.On("FindByEmail", mock.Anything, email).Return(existing, nil)

	svc := newTestService(repo)
	_, _, err := svc.Register(context.Background(), services.RegisterRequest{
		Email:    email,
		Username: "dup",
		Password: "Password1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrUserAlreadyExists))
}

func TestRegister_BothEmailAndPhone(t *testing.T) {
	repo := &MockUserRepository{}
	email := "charlie@example.com"
	phone := "+12125550001"

	repo.On("FindByEmail", mock.Anything, email).Return(nil, repositories.ErrNotFound)
	repo.On("FindByPhone", mock.Anything, phone).Return(nil, repositories.ErrNotFound)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.Session")).Return(nil)

	svc := newTestService(repo)
	user, tokens, err := svc.Register(context.Background(), services.RegisterRequest{
		Email:    email,
		Phone:    phone,
		Username: "charlie",
		Password: "Password1",
	})

	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotNil(t, tokens)
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	repo := &MockUserRepository{}
	user := testUser()

	repo.On("FindByEmail", mock.Anything, *user.Email).Return(user, nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.Session")).Return(nil)

	svc := newTestService(repo)
	gotUser, tokens, err := svc.Login(context.Background(), services.LoginRequest{
		Email:    *user.Email,
		Password: "Password1",
	})

	require.NoError(t, err)
	assert.Equal(t, user.ID, gotUser.ID)
	assert.NotNil(t, tokens)
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := &MockUserRepository{}
	user := testUser()

	repo.On("FindByEmail", mock.Anything, *user.Email).Return(user, nil)

	svc := newTestService(repo)
	_, _, err := svc.Login(context.Background(), services.LoginRequest{
		Email:    *user.Email,
		Password: "WrongPassword1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidCredentials))
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("FindByEmail", mock.Anything, "ghost@example.com").Return(nil, repositories.ErrNotFound)

	svc := newTestService(repo)
	_, _, err := svc.Login(context.Background(), services.LoginRequest{
		Email:    "ghost@example.com",
		Password: "Password1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidCredentials))
}

func TestLogin_SuspendedAccount(t *testing.T) {
	repo := &MockUserRepository{}
	user := testUser()
	user.Status = models.UserStatusSuspended

	repo.On("FindByEmail", mock.Anything, *user.Email).Return(user, nil)

	svc := newTestService(repo)
	_, _, err := svc.Login(context.Background(), services.LoginRequest{
		Email:    *user.Email,
		Password: "Password1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrAccountSuspended))
}

func TestLogin_MFARequired(t *testing.T) {
	repo := &MockUserRepository{}
	user := testUser()
	user.MFAEnabled = true
	secret := "JBSWY3DPEHPK3PXP"
	user.MFASecret = &secret

	repo.On("FindByEmail", mock.Anything, *user.Email).Return(user, nil)

	// MFA path tries to set a Redis key; with nil redis client it logs a warn and continues.
	svc := newTestService(repo)
	_, tokens, err := svc.Login(context.Background(), services.LoginRequest{
		Email:    *user.Email,
		Password: "Password1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrMFARequired))
	assert.Nil(t, tokens, "tokens must be nil when MFA is required")
}

// ── ValidateAccessToken ───────────────────────────────────────────────────────

func TestValidateAccessToken_ValidToken(t *testing.T) {
	repo := &MockUserRepository{}
	user := testUser()

	repo.On("FindByEmail", mock.Anything, *user.Email).Return(user, nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.Session")).Return(nil)

	svc := newTestService(repo)
	_, tokens, err := svc.Login(context.Background(), services.LoginRequest{
		Email:    *user.Email,
		Password: "Password1",
	})
	require.NoError(t, err)

	claims, err := svc.ValidateAccessToken(context.Background(), tokens.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
}

func TestValidateAccessToken_InvalidToken(t *testing.T) {
	svc := newTestService(&MockUserRepository{})
	_, err := svc.ValidateAccessToken(context.Background(), "not.a.valid.jwt")
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidToken))
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_Success(t *testing.T) {
	repo := &MockUserRepository{}
	sessionID := uuid.New()
	userID := uuid.New()

	session := &models.Session{
		ID:           sessionID,
		UserID:       userID,
		RefreshToken: "hashed-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	// FindSessionByToken accepts ANY string (the raw token gets hashed internally).
	repo.On("FindSessionByToken", mock.Anything, mock.AnythingOfType("string")).Return(session, nil)
	repo.On("RevokeSession", mock.Anything, sessionID).Return(nil)

	svc := newTestService(repo)
	err := svc.Logout(context.Background(), "raw-refresh-token")
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestLogout_AlreadyGone(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("FindSessionByToken", mock.Anything, mock.AnythingOfType("string")).Return(nil, repositories.ErrNotFound)

	svc := newTestService(repo)
	err := svc.Logout(context.Background(), "expired-token")
	// Should be a no-op — not an error.
	require.NoError(t, err)
}

// ── ChangePassword ────────────────────────────────────────────────────────────

func TestChangePassword_Success(t *testing.T) {
	repo := &MockUserRepository{}
	user := testUser()

	repo.On("FindByID", mock.Anything, user.ID).Return(user, nil)
	repo.On("UpdatePassword", mock.Anything, user.ID, mock.AnythingOfType("string")).Return(nil)

	svc := newTestService(repo)
	err := svc.ChangePassword(context.Background(), services.ChangePasswordRequest{
		UserID:      user.ID,
		OldPassword: "Password1",
		NewPassword: "NewPassword2",
	})
	require.NoError(t, err)
}

func TestChangePassword_WrongOld(t *testing.T) {
	repo := &MockUserRepository{}
	user := testUser()

	repo.On("FindByID", mock.Anything, user.ID).Return(user, nil)

	svc := newTestService(repo)
	err := svc.ChangePassword(context.Background(), services.ChangePasswordRequest{
		UserID:      user.ID,
		OldPassword: "WrongOld1",
		NewPassword: "NewPassword2",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidCredentials))
}

// ── ResetPassword ─────────────────────────────────────────────────────────────

func TestResetPassword_InvalidToken(t *testing.T) {
	svc := newTestService(&MockUserRepository{})
	err := svc.ResetPassword(context.Background(), services.ResetPasswordRequest{
		Token:       "bad-token",
		NewPassword: "NewPassword1",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidToken))
}

// ── LogoutAll ─────────────────────────────────────────────────────────────────

func TestLogoutAll(t *testing.T) {
	repo := &MockUserRepository{}
	userID := uuid.New()
	repo.On("RevokeAllUserSessions", mock.Anything, userID).Return(nil)

	svc := newTestService(repo)
	err := svc.LogoutAll(context.Background(), userID)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// ── EnableMFA ─────────────────────────────────────────────────────────────────

func TestEnableMFA_Success(t *testing.T) {
	repo := &MockUserRepository{}
	user := testUser()

	repo.On("FindByID", mock.Anything, user.ID).Return(user, nil)
	repo.On("UpdateMFASecret", mock.Anything, user.ID, mock.AnythingOfType("string"), false).Return(nil)

	svc := newTestService(repo)
	resp, err := svc.EnableMFA(context.Background(), user.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Secret)
	assert.NotEmpty(t, resp.QRCodeURL)
}

func TestEnableMFA_AlreadyEnabled(t *testing.T) {
	repo := &MockUserRepository{}
	user := testUser()
	user.MFAEnabled = true

	repo.On("FindByID", mock.Anything, user.ID).Return(user, nil)

	svc := newTestService(repo)
	_, err := svc.EnableMFA(context.Background(), user.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrMFAAlreadyEnabled))
}

// ── SendOTP (limited — no Redis) ──────────────────────────────────────────────

func TestSendOTP_UserNotFound(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("FindByPhone", mock.Anything, "+12125559999").Return(nil, repositories.ErrNotFound)

	svc := newTestService(repo)
	err := svc.SendOTP(context.Background(), services.SendOTPRequest{
		Phone: "+12125559999",
		Type:  models.OTPTypePhoneVerification,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrUserNotFound))
}

// ── VerifyOTP ─────────────────────────────────────────────────────────────────

func TestVerifyOTP_InvalidCode(t *testing.T) {
	repo := &MockUserRepository{}
	userID := uuid.New()

	repo.On("ValidateOTP", mock.Anything, userID, "000000", models.OTPTypePhoneVerification).
		Return(false, nil)

	svc := newTestService(repo)
	_, err := svc.VerifyOTP(context.Background(), services.VerifyOTPRequest{
		UserID: userID,
		Code:   "000000",
		Type:   models.OTPTypePhoneVerification,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrOTPInvalid))
}

func TestVerifyOTP_NotFound(t *testing.T) {
	repo := &MockUserRepository{}
	userID := uuid.New()

	repo.On("ValidateOTP", mock.Anything, userID, "123456", models.OTPTypePhoneVerification).
		Return(false, repositories.ErrNotFound)

	svc := newTestService(repo)
	_, err := svc.VerifyOTP(context.Background(), services.VerifyOTPRequest{
		UserID: userID,
		Code:   "123456",
		Type:   models.OTPTypePhoneVerification,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrOTPInvalid))
}
