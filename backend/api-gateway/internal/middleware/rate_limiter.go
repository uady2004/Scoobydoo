package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tiktok-clone/api-gateway/internal/config"
)

// slidingWindowScript is a Lua script that implements an atomic sliding window
// rate limiter in Redis. It returns the current request count after incrementing.
//
// KEYS[1] = rate limit key
// ARGV[1] = current timestamp in milliseconds
// ARGV[2] = window size in milliseconds
// ARGV[3] = max requests in window
// ARGV[4] = TTL for the key in milliseconds
//
// Returns: {current_count, is_allowed}  (1 = allowed, 0 = denied)
const slidingWindowScript = `
local key        = KEYS[1]
local now        = tonumber(ARGV[1])
local window_ms  = tonumber(ARGV[2])
local limit      = tonumber(ARGV[3])
local ttl_ms     = tonumber(ARGV[4])
local window_start = now - window_ms

-- Remove timestamps older than the window.
redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

-- Count remaining members in the window.
local count = redis.call('ZCARD', key)

if count < limit then
    -- Add current timestamp as both score and member (unique by appending count).
    redis.call('ZADD', key, now, now .. '-' .. count)
    redis.call('PEXPIRE', key, ttl_ms)
    return {count + 1, 1}
else
    return {count, 0}
end
`

// RateLimiter implements per-user and per-IP sliding window rate limiting
// backed by Redis.
type RateLimiter struct {
	rdb    redis.Cmdable
	cfg    *config.RateLimitConfig
	script *redis.Script
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter(rdb redis.Cmdable, cfg *config.RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		rdb:    rdb,
		cfg:    cfg,
		script: redis.NewScript(slidingWindowScript),
	}
}

// UserRateLimit applies the per-authenticated-user rate limit.
// Falls back to IP-based limiting when no user is authenticated.
func (rl *RateLimiter) UserRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		var key string
		var limit int

		userID, exists := c.Get(GinKeyUserID)
		if exists && userID.(string) != "" {
			key = fmt.Sprintf("rl:user:%s", userID.(string))
			limit = rl.cfg.UserRequestsPerMinute
		} else {
			key = fmt.Sprintf("rl:ip:%s", clientIP(c))
			limit = rl.cfg.IPRequestsPerMinute
		}

		allowed, count, resetIn, err := rl.check(c.Request.Context(), key, limit)
		if err != nil {
			// On Redis failure, allow the request but log the error.
			// Fail open to avoid service disruption.
			c.Header("X-RateLimit-Error", "backend unavailable")
			c.Next()
			return
		}

		remaining := limit - count
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(resetIn).Unix(), 10))
		c.Header("X-RateLimit-Window", rl.cfg.WindowSize.String())

		if !allowed {
			c.Header("Retry-After", strconv.FormatInt(int64(resetIn.Seconds())+1, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "too many requests; please slow down",
				"retry_after": int64(resetIn.Seconds()) + 1,
			})
			return
		}

		c.Next()
	}
}

// IPRateLimit applies a strict IP-only rate limit, regardless of authentication.
// Use this on unauthenticated endpoints like login and registration.
func (rl *RateLimiter) IPRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rl:ip:%s", clientIP(c))
		limit := rl.cfg.IPRequestsPerMinute

		allowed, count, resetIn, err := rl.check(c.Request.Context(), key, limit)
		if err != nil {
			c.Next()
			return
		}

		remaining := limit - count
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(resetIn).Unix(), 10))

		if !allowed {
			c.Header("Retry-After", strconv.FormatInt(int64(resetIn.Seconds())+1, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "too many requests from this IP",
				"retry_after": int64(resetIn.Seconds()) + 1,
			})
			return
		}

		c.Next()
	}
}

// EndpointRateLimit returns a middleware with a custom per-endpoint rate limit.
// The key prefix distinguishes this endpoint's bucket from others.
func (rl *RateLimiter) EndpointRateLimit(prefix string, requestsPerWindow int) gin.HandlerFunc {
	return func(c *gin.Context) {
		var identifier string
		userID, exists := c.Get(GinKeyUserID)
		if exists && userID.(string) != "" {
			identifier = userID.(string)
		} else {
			identifier = clientIP(c)
		}

		key := fmt.Sprintf("rl:endpoint:%s:%s", prefix, identifier)

		allowed, count, resetIn, err := rl.check(c.Request.Context(), key, requestsPerWindow)
		if err != nil {
			c.Next()
			return
		}

		remaining := requestsPerWindow - count
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(requestsPerWindow))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(resetIn).Unix(), 10))

		if !allowed {
			c.Header("Retry-After", strconv.FormatInt(int64(resetIn.Seconds())+1, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "endpoint rate limit exceeded",
				"retry_after": int64(resetIn.Seconds()) + 1,
			})
			return
		}

		c.Next()
	}
}

// check executes the sliding window Lua script.
// Returns: (allowed bool, currentCount int, windowResetDuration, error)
func (rl *RateLimiter) check(ctx context.Context, key string, limit int) (bool, int, time.Duration, error) {
	windowMS := rl.cfg.WindowSize.Milliseconds()
	nowMS := time.Now().UnixMilli()
	// TTL is 2x window to handle edge cases where cleanup is delayed.
	ttlMS := windowMS * 2

	result, err := rl.script.Run(ctx, rl.rdb, []string{key},
		nowMS,
		windowMS,
		limit,
		ttlMS,
	).Slice()
	if err != nil {
		return false, 0, rl.cfg.WindowSize, fmt.Errorf("rate limit script: %w", err)
	}

	if len(result) < 2 {
		return false, 0, rl.cfg.WindowSize, fmt.Errorf("unexpected script result length: %d", len(result))
	}

	count := int(result[0].(int64))
	isAllowed := result[1].(int64) == 1

	return isAllowed, count, rl.cfg.WindowSize, nil
}

// clientIP extracts the real client IP, respecting common proxy headers.
func clientIP(c *gin.Context) string {
	// Gin's ClientIP respects X-Forwarded-For when TrustedProxies is configured.
	return c.ClientIP()
}

// RateLimiterStats returns the current request count for a key (useful for monitoring).
func (rl *RateLimiter) RateLimiterStats(ctx context.Context, keyType, identifier string) (int64, error) {
	windowMS := rl.cfg.WindowSize.Milliseconds()
	nowMS := time.Now().UnixMilli()
	windowStart := nowMS - windowMS

	key := fmt.Sprintf("rl:%s:%s", keyType, identifier)
	count, err := rl.rdb.ZCount(ctx, key,
		strconv.FormatInt(windowStart, 10),
		strconv.FormatInt(nowMS, 10),
	).Result()
	if err != nil {
		return 0, err
	}
	return count, nil
}
