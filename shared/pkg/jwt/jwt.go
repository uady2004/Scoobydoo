// Package jwt provides RS256 JWT generation, validation, and refresh-token
// rotation for the TikTok-clone platform.
//
// Key design decisions:
//   - Access tokens are short-lived RS256 JWTs (stateless, verifiable with the
//     public key only).
//   - Refresh tokens are opaque random strings stored server-side; rotation
//     invalidates the previous token on each use.
//   - The Manager holds both private and public RSA keys; verifiers (e.g.,
//     API gateways) may be initialised with the public key only.
package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	// Aliased to avoid collision with this package's own name.
	jwtlib "github.com/golang-jwt/jwt/v5"

	apperrors "github.com/tiktok-clone/shared/pkg/errors"
)

// ---- Configuration ----------------------------------------------------------

// Config holds all Manager configuration.
type Config struct {
	// PrivateKeyPath is the path to the PEM-encoded RSA private key file.
	// Required for token issuance; optional for verification-only nodes.
	PrivateKeyPath string
	// PublicKeyPath is the path to the PEM-encoded RSA public key file.
	// Used for signature verification.
	PublicKeyPath string

	// PrivateKeyPEM may be set instead of PrivateKeyPath (takes precedence).
	PrivateKeyPEM []byte
	// PublicKeyPEM may be set instead of PublicKeyPath (takes precedence).
	PublicKeyPEM []byte

	// Issuer is placed in the "iss" claim.
	Issuer string
	// Audience is placed in the "aud" claim.
	Audience string

	// AccessTokenTTL is the lifetime of a new access token. Defaults to 15 min.
	AccessTokenTTL time.Duration
	// RefreshTokenTTL is the lifetime of a new refresh token. Defaults to 7 days.
	RefreshTokenTTL time.Duration

	// RSABits is used only when GenerateKeyPair is called. Defaults to 2048.
	RSABits int
}

func (c *Config) defaults() {
	if c.AccessTokenTTL == 0 {
		c.AccessTokenTTL = 15 * time.Minute
	}
	if c.RefreshTokenTTL == 0 {
		c.RefreshTokenTTL = 7 * 24 * time.Hour
	}
	if c.Issuer == "" {
		c.Issuer = "tiktok-clone"
	}
	if c.RSABits == 0 {
		c.RSABits = 2048
	}
}

// ---- Claims -----------------------------------------------------------------

// Claims is the JWT payload structure.
type Claims struct {
	jwtlib.RegisteredClaims
	// UserID is the subject's stable identifier.
	UserID string `json:"uid"`
	// Username is the human-readable handle.
	Username string `json:"username"`
	// Roles is a list of role strings (e.g. ["user", "creator"]).
	Roles []string `json:"roles,omitempty"`
	// SessionID ties this token to a server-side session record.
	SessionID string `json:"sid"`
}

// HasRole reports whether the claims include the given role.
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// ---- Token payloads ---------------------------------------------------------

// TokenPair bundles an access token with its companion refresh token.
type TokenPair struct {
	AccessToken        string    `json:"access_token"`
	RefreshToken       string    `json:"refresh_token"`
	AccessTokenExpiry  time.Time `json:"access_token_expiry"`
	RefreshTokenExpiry time.Time `json:"refresh_token_expiry"`
	TokenType          string    `json:"token_type"`
}

// RefreshTokenPayload is the server-side record for a refresh token.
type RefreshTokenPayload struct {
	Token     string
	UserID    string
	SessionID string
	ExpiresAt time.Time
	IssuedAt  time.Time
}

// ---- RefreshTokenStore ------------------------------------------------------

// RefreshTokenStore abstracts storage of refresh tokens so the Manager is
// independent of any particular database.
type RefreshTokenStore interface {
	// Store persists a new refresh token.
	Store(ctx context.Context, payload RefreshTokenPayload) error
	// Load retrieves a refresh token payload by its opaque token string.
	// Returns apperrors.ErrNotFound if the token does not exist or is expired.
	Load(ctx context.Context, token string) (*RefreshTokenPayload, error)
	// Revoke marks a token as used / invalid.
	Revoke(ctx context.Context, token string) error
	// RevokeAllForUser revokes all refresh tokens belonging to userID (logout
	// from all devices).
	RevokeAllForUser(ctx context.Context, userID string) error
}

// ---- Manager ----------------------------------------------------------------

// Manager issues and validates JWTs and manages refresh token rotation.
type Manager struct {
	cfg        Config
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	store      RefreshTokenStore
}

// New creates a Manager, loading keys from the Config.
// store may be nil if you only need access-token verification.
func New(cfg Config, store RefreshTokenStore) (*Manager, error) {
	cfg.defaults()

	m := &Manager{cfg: cfg, store: store}

	// Load private key (for signing).
	if len(cfg.PrivateKeyPEM) > 0 {
		if err := m.loadPrivateKeyPEM(cfg.PrivateKeyPEM); err != nil {
			return nil, err
		}
	} else if cfg.PrivateKeyPath != "" {
		raw, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("jwt: reading private key: %w", err)
		}
		if err := m.loadPrivateKeyPEM(raw); err != nil {
			return nil, err
		}
	}

	// Load public key (for verification).
	if len(cfg.PublicKeyPEM) > 0 {
		if err := m.loadPublicKeyPEM(cfg.PublicKeyPEM); err != nil {
			return nil, err
		}
	} else if cfg.PublicKeyPath != "" {
		raw, err := os.ReadFile(cfg.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("jwt: reading public key: %w", err)
		}
		if err := m.loadPublicKeyPEM(raw); err != nil {
			return nil, err
		}
	} else if m.privateKey != nil {
		// Derive public key from private key when no separate public key given.
		m.publicKey = &m.privateKey.PublicKey
	}

	if m.publicKey == nil {
		return nil, errors.New("jwt: at least one of public or private key must be provided")
	}

	return m, nil
}

// ---- Key generation ---------------------------------------------------------

// GenerateKeyPair generates a new RSA key pair and returns the PEM-encoded
// private and public keys. Useful for test setups and one-time initialisation.
func GenerateKeyPair(bits int) (privatePEM, publicPEM []byte, err error) {
	if bits == 0 {
		bits = 2048
	}
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("jwt: generating RSA key: %w", err)
	}

	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	privatePEM = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("jwt: marshalling public key: %w", err)
	}
	publicPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return privatePEM, publicPEM, nil
}

// NewWithGeneratedKeys creates a Manager with a freshly generated key pair.
// This is convenient for tests; do NOT use in production (keys are ephemeral).
func NewWithGeneratedKeys(cfg Config, store RefreshTokenStore) (*Manager, error) {
	cfg.defaults()
	privPEM, pubPEM, err := GenerateKeyPair(cfg.RSABits)
	if err != nil {
		return nil, err
	}
	cfg.PrivateKeyPEM = privPEM
	cfg.PublicKeyPEM = pubPEM
	return New(cfg, store)
}

// ---- Token issuance ---------------------------------------------------------

// IssueTokenPair generates a new access + refresh token pair for the user.
// sessionID is used to tie both tokens to a logical session.
func (m *Manager) IssueTokenPair(
	ctx context.Context,
	userID, username, sessionID string,
	roles []string,
) (*TokenPair, error) {
	if m.privateKey == nil {
		return nil, errors.New("jwt: private key not loaded; cannot sign tokens")
	}

	now := time.Now().UTC()
	accessExpiry := now.Add(m.cfg.AccessTokenTTL)
	refreshExpiry := now.Add(m.cfg.RefreshTokenTTL)

	claims := Claims{
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    m.cfg.Issuer,
			Subject:   userID,
			Audience:  jwtlib.ClaimStrings{m.cfg.Audience},
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(accessExpiry),
		},
		UserID:    userID,
		Username:  username,
		Roles:     roles,
		SessionID: sessionID,
	}

	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	accessToken, err := tok.SignedString(m.privateKey)
	if err != nil {
		return nil, fmt.Errorf("jwt: signing access token: %w", err)
	}

	refreshToken, err := generateOpaqueToken()
	if err != nil {
		return nil, err
	}

	if m.store != nil {
		if err := m.store.Store(ctx, RefreshTokenPayload{
			Token:     refreshToken,
			UserID:    userID,
			SessionID: sessionID,
			ExpiresAt: refreshExpiry,
			IssuedAt:  now,
		}); err != nil {
			return nil, fmt.Errorf("jwt: storing refresh token: %w", err)
		}
	}

	return &TokenPair{
		AccessToken:        accessToken,
		RefreshToken:       refreshToken,
		AccessTokenExpiry:  accessExpiry,
		RefreshTokenExpiry: refreshExpiry,
		TokenType:          "Bearer",
	}, nil
}

// ---- Token validation -------------------------------------------------------

// ParseAccessToken validates the access token signature and returns the claims.
// It does NOT check the RefreshTokenStore.
func (m *Manager) ParseAccessToken(tokenStr string) (*Claims, error) {
	tok, err := jwtlib.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(t *jwtlib.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwtlib.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("jwt: unexpected signing method %v", t.Header["alg"])
			}
			return m.publicKey, nil
		},
		jwtlib.WithIssuer(m.cfg.Issuer),
		jwtlib.WithExpirationRequired(),
		jwtlib.WithValidMethods([]string{"RS256"}),
	)
	if err != nil {
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return nil, apperrors.NewUnauthorized("access token has expired")
		}
		return nil, apperrors.NewUnauthorized("access token is invalid")
	}

	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, apperrors.NewUnauthorized("access token claims are invalid")
	}
	return claims, nil
}

// ---- Refresh token rotation -------------------------------------------------

// RotateRefreshToken validates the given refresh token, revokes it, issues a
// fresh token pair, and returns it. This implements single-use refresh tokens.
func (m *Manager) RotateRefreshToken(
	ctx context.Context,
	refreshToken string,
	roles []string,
) (*TokenPair, error) {
	if m.store == nil {
		return nil, errors.New("jwt: refresh token store not configured")
	}
	if m.privateKey == nil {
		return nil, errors.New("jwt: private key not loaded; cannot sign tokens")
	}

	payload, err := m.store.Load(ctx, refreshToken)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.NewUnauthorized("refresh token not found or expired")
		}
		return nil, fmt.Errorf("jwt: loading refresh token: %w", err)
	}

	if time.Now().UTC().After(payload.ExpiresAt) {
		_ = m.store.Revoke(ctx, refreshToken)
		return nil, apperrors.NewUnauthorized("refresh token has expired")
	}

	// Revoke the current token before issuing a new pair (rotation).
	if err := m.store.Revoke(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("jwt: revoking old refresh token: %w", err)
	}

	return m.IssueTokenPair(ctx, payload.UserID, "", payload.SessionID, roles)
}

// RevokeRefreshToken revokes a single refresh token (logout from one device).
func (m *Manager) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	if m.store == nil {
		return errors.New("jwt: refresh token store not configured")
	}
	return m.store.Revoke(ctx, refreshToken)
}

// RevokeAllRefreshTokens revokes all refresh tokens for a user (logout from
// all devices).
func (m *Manager) RevokeAllRefreshTokens(ctx context.Context, userID string) error {
	if m.store == nil {
		return errors.New("jwt: refresh token store not configured")
	}
	return m.store.RevokeAllForUser(ctx, userID)
}

// ---- Public key export -------------------------------------------------------

// PublicKeyPEM returns the PEM-encoded RSA public key, suitable for sharing
// with verification-only services (e.g. API gateway).
func (m *Manager) PublicKeyPEM() ([]byte, error) {
	if m.publicKey == nil {
		return nil, errors.New("jwt: no public key loaded")
	}
	b, err := x509.MarshalPKIXPublicKey(m.publicKey)
	if err != nil {
		return nil, fmt.Errorf("jwt: marshalling public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: b}), nil
}

// ---- internal helpers -------------------------------------------------------

func (m *Manager) loadPrivateKeyPEM(data []byte) error {
	block, _ := pem.Decode(data)
	if block == nil {
		return errors.New("jwt: failed to decode PEM block for private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Fallback: try PKCS8.
		parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return fmt.Errorf("jwt: parsing private key (PKCS1: %v; PKCS8: %v)", err, err2)
		}
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return errors.New("jwt: private key is not RSA")
		}
	}
	m.privateKey = key
	return nil
}

func (m *Manager) loadPublicKeyPEM(data []byte) error {
	block, _ := pem.Decode(data)
	if block == nil {
		return errors.New("jwt: failed to decode PEM block for public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("jwt: parsing public key: %w", err)
	}
	key, ok := pub.(*rsa.PublicKey)
	if !ok {
		return errors.New("jwt: public key is not RSA")
	}
	m.publicKey = key
	return nil
}

// generateOpaqueToken returns a 256-bit cryptographically random token
// encoded as a 64-character lowercase hex string.
func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("jwt: generating opaque token: %w", err)
	}
	const hextable = "0123456789abcdef"
	var buf [64]byte
	for i, v := range b {
		buf[i*2] = hextable[v>>4]
		buf[i*2+1] = hextable[v&0x0f]
	}
	return string(buf[:]), nil
}
