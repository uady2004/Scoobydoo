package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/tiktok-clone/user-service/internal/config"
)

// ContextKey is the type used for values stored in Echo's request context.
type ContextKey string

const (
	// ContextKeyUserID is the key under which the authenticated user's UUID is stored.
	ContextKeyUserID ContextKey = "user_id"
	// ContextKeyUsername is the key under which the authenticated user's username is stored.
	ContextKeyUsername ContextKey = "username"
	// ContextKeyRole is the key under which the authenticated user's role is stored.
	ContextKeyRole ContextKey = "role"
	// ContextKeyClaims is the key under which the full JWT claims struct is stored.
	ContextKeyClaims ContextKey = "claims"
)

// UserRole enumerates the possible roles embedded in a JWT.
type UserRole string

const (
	RoleUser      UserRole = "user"
	RoleCreator   UserRole = "creator"
	RoleAdmin     UserRole = "admin"
	RoleModerator UserRole = "moderator"
)

// Claims extends the standard JWT registered claims with application-specific fields.
type Claims struct {
	jwt.RegisteredClaims
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Role     UserRole `json:"role"`
}

// Validate performs semantic validation beyond signature verification.
func (c *Claims) Validate() error {
	if c.UserID == "" {
		return errors.New("user_id claim is missing")
	}
	if _, err := uuid.Parse(c.UserID); err != nil {
		return errors.New("user_id claim is not a valid UUID")
	}
	if c.ExpiresAt != nil && c.ExpiresAt.Before(time.Now()) {
		return errors.New("token has expired")
	}
	return nil
}

// AuthMiddleware validates the Bearer JWT in the Authorization header and
// populates the Echo context with the authenticated user's details.
func AuthMiddleware(cfg *config.Config, logger *zap.Logger) echo.MiddlewareFunc {
	jwtSecret := []byte(cfg.JWT.Secret)
	expectedIssuer := cfg.JWT.Issuer

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenStr, err := extractBearerToken(c.Request())
			if err != nil {
				logger.Debug("missing or malformed authorization header",
					zap.String("path", c.Request().URL.Path),
					zap.Error(err),
				)
				return echo.NewHTTPError(http.StatusUnauthorized, "missing or malformed authorization header")
			}

			claims, err := parseAndValidateClaims(tokenStr, jwtSecret, expectedIssuer)
			if err != nil {
				logger.Warn("jwt validation failed",
					zap.String("path", c.Request().URL.Path),
					zap.Error(err),
				)
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid user_id in token")
			}

			c.Set(string(ContextKeyUserID), userID)
			c.Set(string(ContextKeyUsername), claims.Username)
			c.Set(string(ContextKeyRole), claims.Role)
			c.Set(string(ContextKeyClaims), claims)

			return next(c)
		}
	}
}

// OptionalAuthMiddleware is like AuthMiddleware but does not reject the request
// when no token is present. Handlers must check whether ContextKeyUserID is set.
func OptionalAuthMiddleware(cfg *config.Config, logger *zap.Logger) echo.MiddlewareFunc {
	jwtSecret := []byte(cfg.JWT.Secret)
	expectedIssuer := cfg.JWT.Issuer

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenStr, err := extractBearerToken(c.Request())
			if err != nil {
				// No token — continue unauthenticated.
				return next(c)
			}

			claims, err := parseAndValidateClaims(tokenStr, jwtSecret, expectedIssuer)
			if err != nil {
				logger.Debug("optional auth: invalid token ignored",
					zap.String("path", c.Request().URL.Path),
					zap.Error(err),
				)
				return next(c)
			}

			userID, err := uuid.Parse(claims.UserID)
			if err == nil {
				c.Set(string(ContextKeyUserID), userID)
				c.Set(string(ContextKeyUsername), claims.Username)
				c.Set(string(ContextKeyRole), claims.Role)
				c.Set(string(ContextKeyClaims), claims)
			}

			return next(c)
		}
	}
}

// AdminOnly is a middleware that permits only requests carrying an admin role.
// It must be placed after AuthMiddleware in the middleware chain.
func AdminOnly() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, ok := c.Get(string(ContextKeyRole)).(UserRole)
			if !ok || role != RoleAdmin {
				return echo.NewHTTPError(http.StatusForbidden, "admin role required")
			}
			return next(c)
		}
	}
}

// CreatorOrAdmin permits creators and admins.
func CreatorOrAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, ok := c.Get(string(ContextKeyRole)).(UserRole)
			if !ok || (role != RoleCreator && role != RoleAdmin) {
				return echo.NewHTTPError(http.StatusForbidden, "creator or admin role required")
			}
			return next(c)
		}
	}
}

// MustGetUserID extracts the authenticated user's UUID from the Echo context.
// It panics if the middleware was not applied (programming error).
func MustGetUserID(c echo.Context) uuid.UUID {
	id, ok := c.Get(string(ContextKeyUserID)).(uuid.UUID)
	if !ok {
		panic("auth middleware not applied: ContextKeyUserID not set")
	}
	return id
}

// GetUserID extracts the authenticated user's UUID, returning false if not set.
func GetUserID(c echo.Context) (uuid.UUID, bool) {
	id, ok := c.Get(string(ContextKeyUserID)).(uuid.UUID)
	return id, ok
}

// GetRole extracts the user role from context, returning RoleUser as default.
func GetRole(c echo.Context) UserRole {
	role, ok := c.Get(string(ContextKeyRole)).(UserRole)
	if !ok {
		return RoleUser
	}
	return role
}

// ---------- package-private helpers ----------

func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header absent")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", errors.New("authorization header must use Bearer scheme")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
	if token == "" {
		return "", errors.New("bearer token is empty")
	}
	return token, nil
}

func parseAndValidateClaims(tokenStr string, secret []byte, expectedIssuer string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	}, jwt.WithIssuer(expectedIssuer), jwt.WithExpirationRequired())

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	if err := claims.Validate(); err != nil {
		return nil, err
	}

	return claims, nil
}
