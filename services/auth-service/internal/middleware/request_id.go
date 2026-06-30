package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// HeaderRequestID is the HTTP header name used to propagate the request ID.
	HeaderRequestID = "X-Request-ID"
	// ContextKeyRequestID is the key used to store the request ID in gin.Context.
	ContextKeyRequestID = "request_id"
)

// RequestID is Gin middleware that injects a unique request ID into every
// request.  It first checks for a pre-existing X-Request-ID header (set by
// an upstream load balancer or API gateway) and falls back to generating a
// new UUID v4.  The resolved ID is:
//
//   - stored in gin.Context under the key "request_id"
//   - placed into the standard context.Context via context.WithValue
//   - echoed back to the client in the X-Request-ID response header
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Make the ID available via gin's context helpers.
		c.Set(ContextKeyRequestID, requestID)

		// Propagate into the stdlib context so downstream code can retrieve it
		// without importing Gin.
		ctx := context.WithValue(c.Request.Context(), contextKey(ContextKeyRequestID), requestID)
		c.Request = c.Request.WithContext(ctx)

		// Echo the ID back to the caller.
		c.Header(HeaderRequestID, requestID)

		c.Next()
	}
}

// GetRequestID extracts the request ID from a standard context.Context.
// Returns an empty string if none has been set.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(contextKey(ContextKeyRequestID)).(string); ok {
		return v
	}
	return ""
}

// contextKey is a private type used to avoid collisions in context.WithValue.
type contextKey string
