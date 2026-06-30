package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/tiktok-clone/api-gateway/internal/config"
)

// Context keys for storing claims in request context.
type contextKey string

const (
	ContextKeyUserID   contextKey = "user_id"
	ContextKeyEmail    contextKey = "email"
	ContextKeyRole     contextKey = "role"
	ContextKeyDeviceID contextKey = "device_id"
	ContextKeyClaims   contextKey = "claims"

	// Gin context keys (string form for gin.Context.Get)
	GinKeyUserID   = "user_id"
	GinKeyEmail    = "email"
	GinKeyRole     = "role"
	GinKeyDeviceID = "device_id"
	GinKeyClaims   = "claims"
)

// TikTokClaims represents the JWT claims for this application.
type TikTokClaims struct {
	jwt.RegisteredClaims
	UserID    string   `json:"uid"`
	Email     string   `json:"email"`
	Role      string   `json:"role"`
	DeviceID  string   `json:"did,omitempty"`
	SessionID string   `json:"sid,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	TokenType string   `json:"token_type"` // "access" | "refresh"
}

// JWTValidator holds the validated configuration for JWT middleware.
type JWTValidator struct {
	publicKey *rsa.PublicKey
	cfg       *config.JWTConfig
	redis     *redis.Client
}

// NewJWTValidator creates a new JWT validator from configuration.
func NewJWTValidator(cfg *config.JWTConfig, rdb *redis.Client) (*JWTValidator, error) {
	var pubKey *rsa.PublicKey

	if cfg.PublicKeyPEM != "" {
		key, err := parseRSAPublicKeyFromPEM([]byte(cfg.PublicKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("parsing public key PEM from config: %w", err)
		}
		pubKey = key
	} else if cfg.PublicKeyPath != "" {
		data, err := os.ReadFile(cfg.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("reading public key file %q: %w", cfg.PublicKeyPath, err)
		}
		key, err := parseRSAPublicKeyFromPEM(data)
		if err != nil {
			return nil, fmt.Errorf("parsing public key from file: %w", err)
		}
		pubKey = key
	} else {
		// Dev mode: no key configured — return a passthrough validator that skips verification.
		fmt.Println("WARNING: No JWT public key configured — JWT validation disabled (dev mode).")
	}

	return &JWTValidator{
		publicKey: pubKey,
		cfg:       cfg,
		redis:     rdb,
	}, nil
}

// ValidateJWT returns a Gin middleware that enforces JWT authentication.
// It extracts the Bearer token from the Authorization header, validates it
// using RS256, checks revocation via Redis, and populates the Gin context
// with the parsed claims.
func (v *JWTValidator) ValidateJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		if v.publicKey == nil {
			c.Next()
			return
		}
		tokenStr, err := extractBearerToken(c.Request)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "missing_token", err.Error())
			return
		}

		claims, err := v.parseAndValidateClaims(c.Request.Context(), tokenStr)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "invalid_token", err.Error())
			return
		}

		// Populate Gin context with typed values for downstream handlers.
		c.Set(GinKeyUserID, claims.UserID)
		c.Set(GinKeyEmail, claims.Email)
		c.Set(GinKeyRole, claims.Role)
		c.Set(GinKeyDeviceID, claims.DeviceID)
		c.Set(GinKeyClaims, claims)

		// Also inject into standard context so non-gin downstream code can read them.
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, ContextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
		ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)
		ctx = context.WithValue(ctx, ContextKeyClaims, claims)
		c.Request = c.Request.WithContext(ctx)

		// Forward identity headers to downstream services.
		c.Request.Header.Set("X-User-ID", claims.UserID)
		c.Request.Header.Set("X-User-Role", claims.Role)
		c.Request.Header.Set("X-User-Email", claims.Email)
		if claims.DeviceID != "" {
			c.Request.Header.Set("X-Device-ID", claims.DeviceID)
		}

		c.Next()
	}
}

// OptionalJWT is like ValidateJWT but does not abort on missing/invalid tokens.
// Useful for endpoints that serve both anonymous and authenticated users.
func (v *JWTValidator) OptionalJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := extractBearerToken(c.Request)
		if err != nil {
			c.Next()
			return
		}

		claims, err := v.parseAndValidateClaims(c.Request.Context(), tokenStr)
		if err != nil {
			c.Next()
			return
		}

		c.Set(GinKeyUserID, claims.UserID)
		c.Set(GinKeyEmail, claims.Email)
		c.Set(GinKeyRole, claims.Role)
		c.Set(GinKeyDeviceID, claims.DeviceID)
		c.Set(GinKeyClaims, claims)

		c.Request.Header.Set("X-User-ID", claims.UserID)
		c.Request.Header.Set("X-User-Role", claims.Role)
		c.Request.Header.Set("X-User-Email", claims.Email)

		c.Next()
	}
}

func (v *JWTValidator) parseAndValidateClaims(ctx context.Context, tokenStr string) (*TikTokClaims, error) {
	claims := &TikTokClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// Enforce RS256 — reject symmetric algorithms to prevent alg confusion attacks.
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return v.publicKey, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.cfg.Issuer),
		jwt.WithAudience(v.cfg.Audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("token parse error: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("token is not valid")
	}

	if claims.TokenType != "access" {
		return nil, fmt.Errorf("expected access token, got %q", claims.TokenType)
	}

	if claims.UserID == "" {
		return nil, errors.New("token missing user ID claim")
	}

	// Check token revocation in Redis (token blacklist).
	if err := v.checkRevocation(ctx, claims); err != nil {
		return nil, err
	}

	return claims, nil
}

// checkRevocation checks whether the token's JTI or session has been revoked.
func (v *JWTValidator) checkRevocation(ctx context.Context, claims *TikTokClaims) error {
	if v.redis == nil {
		return nil
	}

	// Check JTI blacklist.
	if jti := claims.ID; jti != "" {
		key := fmt.Sprintf("token:revoked:%s", jti)
		exists, err := v.redis.Exists(ctx, key).Result()
		if err != nil {
			// Redis failure is non-fatal; log and continue.
			return nil
		}
		if exists > 0 {
			return errors.New("token has been revoked")
		}
	}

	// Check per-user token generation counter to invalidate all tokens on password change.
	if claims.UserID != "" {
		key := fmt.Sprintf("user:token_gen:%s", claims.UserID)
		genStr, err := v.redis.Get(ctx, key).Result()
		if err == nil && genStr != "" {
			// The token's IssuedAt must be after the generation reset timestamp.
			iat, err := claims.GetIssuedAt()
			if err == nil && iat != nil {
				var resetTS int64
				if _, err := fmt.Sscanf(genStr, "%d", &resetTS); err == nil {
					resetTime := time.Unix(resetTS, 0)
					if iat.Before(resetTime) {
						return errors.New("token issued before security reset; please login again")
					}
				}
			}
		}
	}

	return nil
}

// RevokeToken adds a token's JTI to the Redis blacklist with a TTL matching
// the token's remaining lifetime. Call this from the logout endpoint.
func (v *JWTValidator) RevokeToken(ctx context.Context, tokenStr string) error {
	claims := &TikTokClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(tokenStr, claims)
	if err != nil {
		return fmt.Errorf("parsing token for revocation: %w", err)
	}

	jti := claims.ID
	if jti == "" {
		return errors.New("token has no JTI; cannot revoke")
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return errors.New("token has no expiry; cannot set revocation TTL")
	}

	ttl := time.Until(exp.Time)
	if ttl <= 0 {
		return nil // Already expired, nothing to do.
	}

	key := fmt.Sprintf("token:revoked:%s", jti)
	return v.redis.Set(ctx, key, "1", ttl).Err()
}

// InvalidateUserTokens sets a generation reset timestamp for a user,
// invalidating all tokens issued before now. Use on password change or account compromise.
func (v *JWTValidator) InvalidateUserTokens(ctx context.Context, userID string) error {
	if v.redis == nil {
		return errors.New("redis not configured")
	}
	key := fmt.Sprintf("user:token_gen:%s", userID)
	// Store for 30 days (max refresh token lifetime)
	return v.redis.Set(ctx, key, fmt.Sprintf("%d", time.Now().Unix()), 30*24*time.Hour).Err()
}

// extractBearerToken pulls the JWT string from the Authorization header.
func extractBearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		// Fall back to query param for WebSocket upgrade requests.
		if tok := r.URL.Query().Get("token"); tok != "" {
			return tok, nil
		}
		return "", errors.New("authorization header is required")
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("authorization header must be 'Bearer <token>'")
	}

	if parts[1] == "" {
		return "", errors.New("bearer token is empty")
	}

	return parts[1], nil
}

func parseRSAPublicKeyFromPEM(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	switch block.Type {
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case "PUBLIC KEY":
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("key is not an RSA public key")
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}

func abortWithError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error":   code,
		"message": message,
	})
}

// GetUserIDFromContext retrieves the user ID from a standard context.
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(string)
	return v, ok && v != ""
}

// GetClaimsFromContext retrieves the full claims from a standard context.
func GetClaimsFromContext(ctx context.Context) (*TikTokClaims, bool) {
	v, ok := ctx.Value(ContextKeyClaims).(*TikTokClaims)
	return v, ok && v != nil
}
