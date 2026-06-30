package repositories

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tiktok-clone/auth-service/internal/models"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

// ErrDuplicateKey is returned when a unique constraint is violated.
var ErrDuplicateKey = errors.New("duplicate key")

// UserRepository defines the persistence contract for user and session data.
type UserRepository interface {
	// User operations
	CreateUser(ctx context.Context, user *models.User) error
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByPhone(ctx context.Context, phone string) (*models.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	FindByProviderID(ctx context.Context, provider models.AuthProvider, providerUserID string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	UpdateMFASecret(ctx context.Context, userID uuid.UUID, secret string, enabled bool) error
	SetEmailVerified(ctx context.Context, userID uuid.UUID) error
	SetPhoneVerified(ctx context.Context, userID uuid.UUID) error

	// Session operations
	CreateSession(ctx context.Context, session *models.Session) error
	FindSession(ctx context.Context, sessionID uuid.UUID) (*models.Session, error)
	FindSessionByToken(ctx context.Context, refreshTokenHash string) (*models.Session, error)
	RevokeSession(ctx context.Context, sessionID uuid.UUID) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error
	UpdateSessionLastSeen(ctx context.Context, sessionID uuid.UUID) error

	// OTP operations
	CreateOTP(ctx context.Context, otp *models.OTPCode) error
	FindActiveOTP(ctx context.Context, userID uuid.UUID, otpType models.OTPType) (*models.OTPCode, error)
	ValidateOTP(ctx context.Context, userID uuid.UUID, code string, otpType models.OTPType) (bool, error)
	InvalidateOTPs(ctx context.Context, userID uuid.UUID, otpType models.OTPType) error
	IncrementOTPAttempts(ctx context.Context, otpID uuid.UUID) error

	// Device session operations
	UpsertDeviceSession(ctx context.Context, ds *models.DeviceSession) error
	FindDeviceSession(ctx context.Context, userID uuid.UUID, deviceID string) (*models.DeviceSession, error)
}

// pgUserRepository is the pgx-backed implementation.
type pgUserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository backed by the given connection pool.
func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &pgUserRepository{pool: pool}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func isUniqueViolation(err error) bool {
	// pgx v5 wraps pg error codes in pgconn.PgError
	var pgErr interface{ Code() string }
	if errors.As(err, &pgErr) {
		return pgErr.Code() == "23505"
	}
	return false
}

// scanUser reads a complete User row from pgx.Rows / pgx.Row.
func scanUser(row pgx.Row) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(
		&u.ID, &u.Email, &u.Phone, &u.Username,
		&u.PasswordHash, &u.Provider, &u.ProviderUserID,
		&u.EmailVerified, &u.PhoneVerified,
		&u.MFAEnabled, &u.MFASecret,
		&u.Status, &u.DisplayName, &u.AvatarURL,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

const userColumns = `
	id, email, phone, username,
	password_hash, provider, provider_user_id,
	email_verified, phone_verified,
	mfa_enabled, mfa_secret,
	status, display_name, avatar_url,
	created_at, updated_at, deleted_at`

// ── User operations ───────────────────────────────────────────────────────────

func (r *pgUserRepository) CreateUser(ctx context.Context, u *models.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (
			id, email, phone, username,
			password_hash, provider, provider_user_id,
			email_verified, phone_verified,
			mfa_enabled, mfa_secret,
			status, display_name, avatar_url,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16
		)`,
		u.ID, u.Email, u.Phone, u.Username,
		u.PasswordHash, u.Provider, u.ProviderUserID,
		u.EmailVerified, u.PhoneVerified,
		u.MFAEnabled, u.MFASecret,
		u.Status, u.DisplayName, u.AvatarURL,
		u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("create user: %w", ErrDuplicateKey)
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *pgUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1 AND deleted_at IS NULL`, email)
	return scanUser(row)
}

func (r *pgUserRepository) FindByPhone(ctx context.Context, phone string) (*models.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE phone = $1 AND deleted_at IS NULL`, phone)
	return scanUser(row)
}

func (r *pgUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanUser(row)
}

func (r *pgUserRepository) FindByProviderID(ctx context.Context, provider models.AuthProvider, providerUserID string) (*models.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE provider = $1 AND provider_user_id = $2 AND deleted_at IS NULL`,
		provider, providerUserID)
	return scanUser(row)
}

func (r *pgUserRepository) UpdateUser(ctx context.Context, u *models.User) error {
	u.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET
			email = $2, phone = $3, username = $4,
			display_name = $5, avatar_url = $6,
			status = $7, updated_at = $8
		WHERE id = $1`,
		u.ID, u.Email, u.Phone, u.Username,
		u.DisplayName, u.AvatarURL,
		u.Status, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (r *pgUserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = $3 WHERE id = $1`,
		userID, passwordHash, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (r *pgUserRepository) UpdateMFASecret(ctx context.Context, userID uuid.UUID, secret string, enabled bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET mfa_secret = $2, mfa_enabled = $3, updated_at = $4 WHERE id = $1`,
		userID, secret, enabled, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update mfa secret: %w", err)
	}
	return nil
}

func (r *pgUserRepository) SetEmailVerified(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET email_verified = true, updated_at = $2 WHERE id = $1`,
		userID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("set email verified: %w", err)
	}
	return nil
}

func (r *pgUserRepository) SetPhoneVerified(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET phone_verified = true, updated_at = $2 WHERE id = $1`,
		userID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("set phone verified: %w", err)
	}
	return nil
}

// ── Session operations ────────────────────────────────────────────────────────

func scanSession(row pgx.Row) (*models.Session, error) {
	s := &models.Session{}
	err := row.Scan(
		&s.ID, &s.UserID, &s.RefreshToken,
		&s.UserAgent, &s.IPAddress, &s.DeviceID,
		&s.IsRevoked, &s.ExpiresAt,
		&s.CreatedAt, &s.LastSeenAt, &s.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	return s, nil
}

func (r *pgUserRepository) CreateSession(ctx context.Context, s *models.Session) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now().UTC()
	s.CreatedAt = now
	s.LastSeenAt = now

	// Hash the refresh token before persisting.
	tokenHash := hashToken(s.RefreshToken)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO sessions (
			id, user_id, refresh_token,
			user_agent, ip_address, device_id,
			is_revoked, expires_at, created_at, last_seen_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		s.ID, s.UserID, tokenHash,
		s.UserAgent, s.IPAddress, s.DeviceID,
		false, s.ExpiresAt, s.CreatedAt, s.LastSeenAt,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *pgUserRepository) FindSession(ctx context.Context, sessionID uuid.UUID) (*models.Session, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, refresh_token,
			user_agent, ip_address, device_id,
			is_revoked, expires_at, created_at, last_seen_at, revoked_at
		FROM sessions WHERE id = $1`, sessionID)
	return scanSession(row)
}

func (r *pgUserRepository) FindSessionByToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	tokenHash := hashToken(refreshToken)
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, refresh_token,
			user_agent, ip_address, device_id,
			is_revoked, expires_at, created_at, last_seen_at, revoked_at
		FROM sessions WHERE refresh_token = $1`, tokenHash)
	return scanSession(row)
}

func (r *pgUserRepository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`UPDATE sessions SET is_revoked = true, revoked_at = $2 WHERE id = $1`,
		sessionID, now)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (r *pgUserRepository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`UPDATE sessions SET is_revoked = true, revoked_at = $2
		WHERE user_id = $1 AND is_revoked = false`,
		userID, now)
	if err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}
	return nil
}

func (r *pgUserRepository) UpdateSessionLastSeen(ctx context.Context, sessionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sessions SET last_seen_at = $2 WHERE id = $1`,
		sessionID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update session last seen: %w", err)
	}
	return nil
}

// ── OTP operations ────────────────────────────────────────────────────────────

func (r *pgUserRepository) CreateOTP(ctx context.Context, otp *models.OTPCode) error {
	if otp.ID == uuid.Nil {
		otp.ID = uuid.New()
	}
	otp.CreatedAt = time.Now().UTC()
	if otp.MaxTrials == 0 {
		otp.MaxTrials = 5
	}

	// Hash the plaintext code before persisting.
	codeHash := hashToken(otp.Code)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO otp_codes (id, user_id, code, type, target, attempts, max_trials, is_used, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		otp.ID, otp.UserID, codeHash, otp.Type, otp.Target,
		0, otp.MaxTrials, false, otp.ExpiresAt, otp.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create otp: %w", err)
	}
	return nil
}

func (r *pgUserRepository) FindActiveOTP(ctx context.Context, userID uuid.UUID, otpType models.OTPType) (*models.OTPCode, error) {
	otp := &models.OTPCode{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, code, type, target, attempts, max_trials, is_used, expires_at, created_at
		FROM otp_codes
		WHERE user_id = $1 AND type = $2 AND is_used = false AND expires_at > now()
		ORDER BY created_at DESC
		LIMIT 1`,
		userID, otpType,
	).Scan(
		&otp.ID, &otp.UserID, &otp.Code, &otp.Type, &otp.Target,
		&otp.Attempts, &otp.MaxTrials, &otp.IsUsed, &otp.ExpiresAt, &otp.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find active otp: %w", err)
	}
	return otp, nil
}

// ValidateOTP checks the provided plaintext code against the stored hash,
// marks it as used on success, and increments attempts on failure.
func (r *pgUserRepository) ValidateOTP(ctx context.Context, userID uuid.UUID, code string, otpType models.OTPType) (bool, error) {
	otp, err := r.FindActiveOTP(ctx, userID, otpType)
	if err != nil {
		return false, err
	}

	if otp.Attempts >= otp.MaxTrials {
		return false, fmt.Errorf("otp: max attempts exceeded")
	}

	codeHash := hashToken(code)
	if otp.Code != codeHash {
		if incErr := r.IncrementOTPAttempts(ctx, otp.ID); incErr != nil {
			return false, fmt.Errorf("otp: increment attempts: %w", incErr)
		}
		return false, nil
	}

	// Mark as used.
	_, err = r.pool.Exec(ctx,
		`UPDATE otp_codes SET is_used = true WHERE id = $1`, otp.ID)
	if err != nil {
		return false, fmt.Errorf("otp: mark used: %w", err)
	}
	return true, nil
}

func (r *pgUserRepository) InvalidateOTPs(ctx context.Context, userID uuid.UUID, otpType models.OTPType) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE otp_codes SET is_used = true WHERE user_id = $1 AND type = $2 AND is_used = false`,
		userID, otpType)
	if err != nil {
		return fmt.Errorf("invalidate otps: %w", err)
	}
	return nil
}

func (r *pgUserRepository) IncrementOTPAttempts(ctx context.Context, otpID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1`, otpID)
	if err != nil {
		return fmt.Errorf("increment otp attempts: %w", err)
	}
	return nil
}

// ── Device session operations ─────────────────────────────────────────────────

func (r *pgUserRepository) UpsertDeviceSession(ctx context.Context, ds *models.DeviceSession) error {
	if ds.ID == uuid.Nil {
		ds.ID = uuid.New()
	}
	now := time.Now().UTC()
	ds.CreatedAt = now
	ds.LastActiveAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO device_sessions (id, user_id, device_id, device_name, platform, ip_address, user_agent, is_trusted, last_active_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			ip_address    = EXCLUDED.ip_address,
			user_agent    = EXCLUDED.user_agent,
			last_active_at = EXCLUDED.last_active_at`,
		ds.ID, ds.UserID, ds.DeviceID, ds.DeviceName, ds.Platform,
		ds.IPAddress, ds.UserAgent, ds.IsTrusted, ds.LastActiveAt, ds.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert device session: %w", err)
	}
	return nil
}

func (r *pgUserRepository) FindDeviceSession(ctx context.Context, userID uuid.UUID, deviceID string) (*models.DeviceSession, error) {
	ds := &models.DeviceSession{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, device_id, device_name, platform, ip_address, user_agent, is_trusted, last_active_at, created_at, revoked_at
		FROM device_sessions
		WHERE user_id = $1 AND device_id = $2 AND revoked_at IS NULL`,
		userID, deviceID,
	).Scan(
		&ds.ID, &ds.UserID, &ds.DeviceID, &ds.DeviceName, &ds.Platform,
		&ds.IPAddress, &ds.UserAgent, &ds.IsTrusted, &ds.LastActiveAt, &ds.CreatedAt, &ds.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find device session: %w", err)
	}
	return ds, nil
}
