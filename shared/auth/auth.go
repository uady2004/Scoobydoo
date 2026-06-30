// Package auth provides framework-agnostic JWT helpers, context keys, and
// Bearer-token extraction for the TikTok-clone platform.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	jwtpkg "github.com/tiktok-clone/shared/pkg/jwt"
)

// ---- Typed context keys -------------------------------------------------------

type contextKey string

const (
	ContextKeyUserID    contextKey = "user_id"
	ContextKeyUsername  contextKey = "username"
	ContextKeyRoles     contextKey = "roles"
	ContextKeySessionID contextKey = "session_id"
)

// ---- Sentinel errors ----------------------------------------------------------

var (
	ErrMissingToken  = errors.New("auth: missing Authorization header")
	ErrInvalidScheme = errors.New("auth: Authorization header must use Bearer scheme")
	ErrInvalidToken  = errors.New("auth: invalid or expired token")
)

// ---- Token extraction ---------------------------------------------------------

// ExtractBearerToken reads the raw JWT string from the Authorization header.
func ExtractBearerToken(r *http.Request) (string, error) {
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return "", ErrMissingToken
	}
	parts := strings.SplitN(hdr, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidScheme
	}
	tok := strings.TrimSpace(parts[1])
	if tok == "" {
		return "", ErrMissingToken
	}
	return tok, nil
}

// ParseRequest validates the Bearer token in r and returns the parsed claims.
func ParseRequest(r *http.Request, mgr *jwtpkg.Manager) (*jwtpkg.Claims, error) {
	tok, err := ExtractBearerToken(r)
	if err != nil {
		return nil, err
	}
	claims, err := mgr.ParseAccessToken(tok)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// ---- Context helpers ----------------------------------------------------------

// WithClaims stores JWT claims fields into the context.
func WithClaims(ctx context.Context, claims *jwtpkg.Claims) context.Context {
	ctx = context.WithValue(ctx, ContextKeyUserID, claims.UserID)
	ctx = context.WithValue(ctx, ContextKeyUsername, claims.Username)
	ctx = context.WithValue(ctx, ContextKeyRoles, claims.Roles)
	ctx = context.WithValue(ctx, ContextKeySessionID, claims.SessionID)
	return ctx
}

// UserIDFromContext returns the user ID stored by WithClaims.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(string)
	return v, ok && v != ""
}

// UsernameFromContext returns the username stored by WithClaims.
func UsernameFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyUsername).(string)
	return v, ok && v != ""
}

// RolesFromContext returns the roles slice stored by WithClaims.
func RolesFromContext(ctx context.Context) []string {
	v, _ := ctx.Value(ContextKeyRoles).([]string)
	return v
}

// HasRole reports whether the context contains the given role.
func HasRole(ctx context.Context, role string) bool {
	for _, r := range RolesFromContext(ctx) {
		if r == role {
			return true
		}
	}
	return false
}

// SessionIDFromContext returns the session ID stored by WithClaims.
func SessionIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeySessionID).(string)
	return v, ok && v != ""
}

// ---- HTTP header helpers ------------------------------------------------------

const (
	HeaderAuthorization = "Authorization"
	HeaderXUserID       = "X-User-ID"
	HeaderXUsername     = "X-Username"
	HeaderXRequestID    = "X-Request-ID"
	HeaderXForwardedFor = "X-Forwarded-For"
	HeaderXRealIP       = "X-Real-IP"
)

// UserIDFromHeader returns the X-User-ID header value (set by the API gateway).
func UserIDFromHeader(r *http.Request) string {
	return r.Header.Get(HeaderXUserID)
}

// UsernameFromHeader returns the X-Username header value (set by the API gateway).
func UsernameFromHeader(r *http.Request) string {
	return r.Header.Get(HeaderXUsername)
}

// ClientIP returns the real client IP, preferring X-Real-IP then X-Forwarded-For.
func ClientIP(r *http.Request) string {
	if ip := r.Header.Get(HeaderXRealIP); ip != "" {
		return ip
	}
	if fwd := r.Header.Get(HeaderXForwardedFor); fwd != "" {
		return strings.SplitN(fwd, ",", 2)[0]
	}
	return r.RemoteAddr
}
