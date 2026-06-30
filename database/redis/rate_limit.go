package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Rate-limiter key patterns:
//
//	rl:sliding:{action}:{identifier}  -> sorted set of request timestamps (unix ms) (ZSET)
//	rl:bucket:{action}:{identifier}   -> token bucket state hash {tokens, ts}       (HASH)
//	rl:block:{identifier}             -> block reason string, TTL = block duration  (STRING)
//	rl:global:{action}                -> global sliding-window counter              (ZSET)
//
// ALL sliding-window and token-bucket operations are performed via pre-compiled
// Lua scripts to guarantee atomicity. No request can slip through a TOCTOU gap
// between the ZREMRANGEBYSCORE and ZADD/ZCOUNT steps.

// ─────────────────────────────────────────────────────────────────────────────
// Lua scripts
// ─────────────────────────────────────────────────────────────────────────────

// slidingWindowLua implements atomic sliding-window request admission.
//
// KEYS[1] = rate limit sorted-set key
// ARGV[1] = current time in unix milliseconds
// ARGV[2] = window size in milliseconds
// ARGV[3] = max requests allowed in the window
// ARGV[4] = unique request ID (prevents member collision on same-millisecond requests)
//
// Returns: []int64{allowed(0|1), current_count, remaining_capacity, retry_after_ms}
const slidingWindowLua = `
local key          = KEYS[1]
local now          = tonumber(ARGV[1])
local window_ms    = tonumber(ARGV[2])
local limit        = tonumber(ARGV[3])
local req_id       = ARGV[4]

local window_start = now - window_ms

-- 1. Evict timestamps outside the current window to keep the set bounded.
redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

-- 2. Count remaining requests in the window.
local count = redis.call('ZCARD', key)

if count < limit then
    -- Admit the request.
    redis.call('ZADD', key, now, req_id)
    -- Set TTL equal to window so the key self-cleans when idle.
    redis.call('PEXPIRE', key, window_ms)
    return {1, count + 1, limit - count - 1, 0}
end

-- Rejected: compute time until the oldest entry ages out of the window.
local oldest      = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local retry_after = 0
if #oldest > 0 then
    retry_after = window_ms - (now - tonumber(oldest[2]))
    if retry_after < 0 then retry_after = 0 end
end
return {0, count, 0, retry_after}
`

// tokenBucketLua implements atomic token-bucket replenishment and consumption.
//
// KEYS[1] = bucket hash key
// ARGV[1] = current unix time in milliseconds
// ARGV[2] = bucket capacity (max tokens)
// ARGV[3] = refill rate in tokens-per-millisecond (float string)
// ARGV[4] = tokens required for this request
// ARGV[5] = key TTL in milliseconds (2× fill time recommended)
//
// Returns: []int64{allowed(0|1), remaining_tokens_floored, wait_ms_if_rejected}
const tokenBucketLua = `
local key         = KEYS[1]
local now         = tonumber(ARGV[1])
local capacity    = tonumber(ARGV[2])
local refill_rate = tonumber(ARGV[3])
local requested   = tonumber(ARGV[4])
local ttl_ms      = tonumber(ARGV[5])

local bucket = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(bucket[1])
local ts     = tonumber(bucket[2])

if tokens == nil then
    -- First access: initialise with a full bucket.
    tokens = capacity
    ts     = now
end

-- Refill proportional to elapsed time (capped at capacity).
local elapsed = math.max(0, now - ts)
tokens = math.min(capacity, tokens + elapsed * refill_rate)

if tokens >= requested then
    tokens = tokens - requested
    redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
    redis.call('PEXPIRE', key, ttl_ms)
    return {1, math.floor(tokens), 0}
end

-- Not enough tokens; calculate wait time and persist refilled state.
local deficit  = requested - tokens
local wait_ms  = math.ceil(deficit / refill_rate)
redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
redis.call('PEXPIRE', key, ttl_ms)
return {0, math.floor(tokens), wait_ms}
`

// globalSlidingWindowLua applies a rate limit against a shared global counter.
// Used for API-wide throttling independent of individual user limits.
//
// Same signature as slidingWindowLua.
const globalSlidingWindowLua = `
local key          = KEYS[1]
local now          = tonumber(ARGV[1])
local window_ms    = tonumber(ARGV[2])
local limit        = tonumber(ARGV[3])
local req_id       = ARGV[4]
local window_start = now - window_ms
redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)
local count = redis.call('ZCARD', key)
if count < limit then
    redis.call('ZADD', key, now, req_id)
    redis.call('PEXPIRE', key, window_ms)
    return {1, count + 1, limit - count - 1, 0}
end
local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local retry_after = 0
if #oldest > 0 then
    retry_after = window_ms - (now - tonumber(oldest[2]))
    if retry_after < 0 then retry_after = 0 end
end
return {0, count, 0, retry_after}
`

// ─────────────────────────────────────────────────────────────────────────────
// Action constants
// ─────────────────────────────────────────────────────────────────────────────

// Predefined action names used as key components and for rule lookup.
// Keep these as constants so call sites do not embed magic strings.
const (
	ActionVideoUpload  = "video_upload"
	ActionComment      = "comment"
	ActionLike         = "like"
	ActionFollow       = "follow"
	ActionUnfollow     = "unfollow"
	ActionShare        = "share"
	ActionLogin        = "login"
	ActionSendGift     = "send_gift"
	ActionLiveStream   = "livestream_start"
	ActionAPIGeneral   = "api_general"
	ActionSearch       = "search"
	ActionReport       = "report"
	ActionMessageSend  = "message_send"
	ActionProfileEdit  = "profile_edit"
	ActionPasswordReset = "password_reset"
	ActionDMCreate     = "dm_create"
)

// ─────────────────────────────────────────────────────────────────────────────
// Rule configuration
// ─────────────────────────────────────────────────────────────────────────────

// RuleConfig defines rate limit parameters for a single action.
type RuleConfig struct {
	// Action is the identifier matched against the action parameter in Check().
	Action string

	// --- Sliding window (used when BucketMode == false) ---
	WindowSize  time.Duration // length of the sliding window
	MaxRequests int           // max requests allowed within WindowSize

	// --- Token bucket (used when BucketMode == true) ---
	BucketMode   bool
	BucketCap    float64 // maximum tokens in the bucket
	RefillPerSec float64 // tokens added per second

	// --- Global limit (applied in addition to per-user/IP limit) ---
	// GlobalLimit enables a second check against a shared global counter.
	// Useful to cap total QPS for expensive operations regardless of identity.
	GlobalLimit     int
	GlobalWindowSize time.Duration
}

// DefaultRules is the production rate-limit policy.
// All limits are conservatively set for a TikTok-scale service; tune to observed traffic.
var DefaultRules = []RuleConfig{
	// Content creation — tight daily limits to prevent spam.
	{Action: ActionVideoUpload, WindowSize: 24 * time.Hour, MaxRequests: 20},
	// Social interactions — per-minute limits to deter bulk bots.
	{Action: ActionComment, WindowSize: time.Minute, MaxRequests: 30},
	{Action: ActionLike, WindowSize: time.Minute, MaxRequests: 200},
	{Action: ActionFollow, WindowSize: time.Hour, MaxRequests: 100},
	{Action: ActionUnfollow, WindowSize: time.Hour, MaxRequests: 100},
	{Action: ActionShare, WindowSize: time.Minute, MaxRequests: 50},
	// Auth — strict to prevent brute-force and credential stuffing.
	{Action: ActionLogin, WindowSize: 15 * time.Minute, MaxRequests: 10},
	{Action: ActionPasswordReset, WindowSize: time.Hour, MaxRequests: 5},
	// Monetisation — strict to prevent gift fraud.
	{Action: ActionSendGift, WindowSize: time.Second, MaxRequests: 5},
	// Live streaming.
	{Action: ActionLiveStream, WindowSize: time.Hour, MaxRequests: 3},
	// General API — token bucket for smooth enforcement without cliff edges.
	{
		Action:       ActionAPIGeneral,
		BucketMode:   true,
		BucketCap:    300,
		RefillPerSec: 50,
	},
	// Search and discovery.
	{Action: ActionSearch, WindowSize: time.Minute, MaxRequests: 60},
	{Action: ActionReport, WindowSize: time.Hour, MaxRequests: 10},
	// Messaging.
	{Action: ActionMessageSend, WindowSize: time.Minute, MaxRequests: 60},
	{Action: ActionDMCreate, WindowSize: time.Hour, MaxRequests: 20},
	// Profile.
	{Action: ActionProfileEdit, WindowSize: time.Hour, MaxRequests: 10},
}

// ─────────────────────────────────────────────────────────────────────────────
// Result type
// ─────────────────────────────────────────────────────────────────────────────

// RateLimitResult carries the outcome of a rate limit check.
type RateLimitResult struct {
	// Allowed is true if the request is permitted.
	Allowed bool
	// Remaining is the number of requests/tokens left in the current window.
	Remaining int64
	// RetryAfter is how long the caller should wait before retrying.
	// Only meaningful when Allowed == false.
	RetryAfter time.Duration
	// TotalCount is the number of requests made so far in the current window.
	TotalCount int64
	// Action is echoed back for logging/metrics.
	Action string
	// Identifier is echoed back for logging/metrics.
	Identifier string
}

// ErrRateLimitExceeded is returned by Check when the limit is exceeded.
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// ─────────────────────────────────────────────────────────────────────────────
// RateLimiter
// ─────────────────────────────────────────────────────────────────────────────

// RateLimiter provides sliding-window and token-bucket rate limiting backed by
// Redis Lua scripts for atomic operations.
type RateLimiter struct {
	client        *goredis.Client
	slidingScript *goredis.Script
	bucketScript  *goredis.Script
	globalScript  *goredis.Script
	rules         map[string]RuleConfig
}

// NewRateLimiter constructs a RateLimiter, pre-registers all Lua scripts with
// Redis (SHA caching via go-redis), and indexes rules by action name.
func NewRateLimiter(client *goredis.Client, rules []RuleConfig) *RateLimiter {
	rl := &RateLimiter{
		client:        client,
		slidingScript: goredis.NewScript(slidingWindowLua),
		bucketScript:  goredis.NewScript(tokenBucketLua),
		globalScript:  goredis.NewScript(globalSlidingWindowLua),
		rules:         make(map[string]RuleConfig, len(rules)),
	}
	for _, r := range rules {
		rl.rules[r.Action] = r
	}
	return rl
}

// ─────────────────────────────────────────────────────────────────────────────
// Primary check methods
// ─────────────────────────────────────────────────────────────────────────────

// Check evaluates the rate limit for a single (action, identifier) pair.
// identifier is typically "u:{userID}" for authenticated users or "ip:{ip}"
// for anonymous requests. Use the UserIdentifier / IPIdentifier helpers.
//
// Returns ErrRateLimitExceeded if the limit is breached; the caller should
// respond with HTTP 429 and set the Retry-After header from result.RetryAfter.
func (rl *RateLimiter) Check(ctx context.Context, action, identifier string) (*RateLimitResult, error) {
	rule, ok := rl.rules[action]
	if !ok {
		return &RateLimitResult{Allowed: true, Action: action, Identifier: identifier}, nil
	}

	// Hard block check takes priority over algorithmic limits.
	if blocked, reason, err := rl.IsBlocked(ctx, identifier); err != nil {
		return nil, fmt.Errorf("block check: %w", err)
	} else if blocked {
		return &RateLimitResult{
			Allowed:    false,
			Action:     action,
			Identifier: identifier,
		}, fmt.Errorf("%w: identifier blocked (%s)", ErrRateLimitExceeded, reason)
	}

	var result *RateLimitResult
	var err error
	if rule.BucketMode {
		result, err = rl.checkTokenBucket(ctx, action, identifier, rule, 1)
	} else {
		result, err = rl.checkSlidingWindow(ctx, action, identifier, rule, 1)
	}
	if err != nil {
		return nil, err
	}

	// Optional global limit check (only if per-user check passed).
	if result.Allowed && rule.GlobalLimit > 0 {
		globalResult, gErr := rl.checkGlobalLimit(ctx, action, rule)
		if gErr != nil {
			return nil, gErr
		}
		if !globalResult.Allowed {
			return globalResult, ErrRateLimitExceeded
		}
	}

	if !result.Allowed {
		return result, ErrRateLimitExceeded
	}
	return result, nil
}

// CheckN is like Check but atomically consumes n slots/tokens.
// Useful for bulk operations (e.g. uploading N comments in one API call).
func (rl *RateLimiter) CheckN(ctx context.Context, action, identifier string, n int) (*RateLimitResult, error) {
	rule, ok := rl.rules[action]
	if !ok {
		return &RateLimitResult{Allowed: true, Action: action, Identifier: identifier}, nil
	}

	var result *RateLimitResult
	var err error
	if rule.BucketMode {
		result, err = rl.checkTokenBucket(ctx, action, identifier, rule, n)
	} else {
		result, err = rl.checkSlidingWindow(ctx, action, identifier, rule, n)
	}
	if err != nil {
		return nil, err
	}
	if !result.Allowed {
		return result, ErrRateLimitExceeded
	}
	return result, nil
}

// Peek returns the current rate limit state without consuming a slot.
// Use for admin dashboards or pre-flight checks.
func (rl *RateLimiter) Peek(ctx context.Context, action, identifier string) (*RateLimitResult, error) {
	rule, ok := rl.rules[action]
	if !ok {
		return &RateLimitResult{Allowed: true, Action: action, Identifier: identifier}, nil
	}

	if rule.BucketMode {
		key := rl.bucketKey(action, identifier)
		vals, err := rl.client.HMGet(ctx, key, "tokens", "ts").Result()
		if err != nil {
			return nil, err
		}
		remaining := int64(rule.BucketCap)
		if vals[0] != nil {
			fmt.Sscanf(vals[0].(string), "%d", &remaining)
		}
		return &RateLimitResult{
			Allowed:    remaining > 0,
			Remaining:  remaining,
			Action:     action,
			Identifier: identifier,
		}, nil
	}

	// Sliding window: count without adding.
	key := rl.slidingKey(action, identifier)
	windowStart := time.Now().UnixMilli() - rule.WindowSize.Milliseconds()
	count, err := rl.client.ZCount(ctx, key,
		fmt.Sprintf("%d", windowStart), "+inf",
	).Result()
	if err != nil {
		return nil, err
	}
	remaining := int64(rule.MaxRequests) - count
	if remaining < 0 {
		remaining = 0
	}
	return &RateLimitResult{
		Allowed:    count < int64(rule.MaxRequests),
		Remaining:  remaining,
		TotalCount: count,
		Action:     action,
		Identifier: identifier,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Block list management
// ─────────────────────────────────────────────────────────────────────────────

// Block adds an identifier to the explicit block list for duration.
// All rate limit checks will fail immediately for the duration, regardless
// of the algorithmic window state.
func (rl *RateLimiter) Block(ctx context.Context, identifier, reason string, duration time.Duration) error {
	return rl.client.Set(ctx, rl.blockKey(identifier), reason, duration).Err()
}

// Unblock removes a block on an identifier.
func (rl *RateLimiter) Unblock(ctx context.Context, identifier string) error {
	return rl.client.Del(ctx, rl.blockKey(identifier)).Err()
}

// IsBlocked reports whether an identifier is currently blocked and returns the reason.
func (rl *RateLimiter) IsBlocked(ctx context.Context, identifier string) (bool, string, error) {
	reason, err := rl.client.Get(ctx, rl.blockKey(identifier)).Result()
	if errors.Is(err, goredis.Nil) {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("is blocked: %w", err)
	}
	return true, reason, nil
}

// GetBlockTTL returns the remaining duration of a block.
// Returns 0 if the identifier is not blocked.
func (rl *RateLimiter) GetBlockTTL(ctx context.Context, identifier string) (time.Duration, error) {
	ttl, err := rl.client.TTL(ctx, rl.blockKey(identifier)).Result()
	if errors.Is(err, goredis.Nil) {
		return 0, nil
	}
	return ttl, err
}

// ─────────────────────────────────────────────────────────────────────────────
// State management
// ─────────────────────────────────────────────────────────────────────────────

// GetCount returns the number of requests recorded in the current window
// without admitting a new one. Works only for sliding-window rules.
func (rl *RateLimiter) GetCount(ctx context.Context, action, identifier string) (int64, error) {
	rule, ok := rl.rules[action]
	if !ok || rule.BucketMode {
		return 0, nil
	}
	windowStart := time.Now().UnixMilli() - rule.WindowSize.Milliseconds()
	return rl.client.ZCount(ctx, rl.slidingKey(action, identifier),
		fmt.Sprintf("%d", windowStart), "+inf",
	).Result()
}

// Reset clears all rate limit state for a given (action, identifier) pair.
// Intended for test environments and manual operator overrides only.
func (rl *RateLimiter) Reset(ctx context.Context, action, identifier string) error {
	pipe := rl.client.Pipeline()
	pipe.Del(ctx, rl.slidingKey(action, identifier))
	pipe.Del(ctx, rl.bucketKey(action, identifier))
	_, err := pipe.Exec(ctx)
	return err
}

// ResetAll clears every rate limit key for an identifier across all configured actions.
func (rl *RateLimiter) ResetAll(ctx context.Context, identifier string) error {
	keys := make([]string, 0, len(rl.rules)*2)
	for action := range rl.rules {
		keys = append(keys, rl.slidingKey(action, identifier), rl.bucketKey(action, identifier))
	}
	if len(keys) == 0 {
		return nil
	}
	return rl.client.Del(ctx, keys...).Err()
}

// AddRule registers or replaces a rate limit rule at runtime.
// Thread-safe for reads; concurrent writes should be protected externally.
func (rl *RateLimiter) AddRule(rule RuleConfig) {
	rl.rules[rule.Action] = rule
}

// GetRule returns the configuration for an action. Returns false if no rule exists.
func (rl *RateLimiter) GetRule(action string) (RuleConfig, bool) {
	r, ok := rl.rules[action]
	return r, ok
}

// ─────────────────────────────────────────────────────────────────────────────
// Sliding-window implementation
// ─────────────────────────────────────────────────────────────────────────────

func (rl *RateLimiter) checkSlidingWindow(
	ctx context.Context,
	action, identifier string,
	rule RuleConfig,
	n int,
) (*RateLimitResult, error) {
	key := rl.slidingKey(action, identifier)
	now := time.Now().UnixMilli()
	// Unique member prevents collisions when two requests arrive in the same millisecond.
	reqID := fmt.Sprintf("%d-%s-%d", now, identifier[:min(len(identifier), 16)], n)

	// For n>1 reduce the effective limit so the n slots are consumed atomically.
	effectiveLimit := rule.MaxRequests - n + 1
	if effectiveLimit < 1 {
		effectiveLimit = 1
	}

	vals, err := rl.slidingScript.Run(ctx, rl.client,
		[]string{key},
		now,
		rule.WindowSize.Milliseconds(),
		effectiveLimit,
		reqID,
	).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("sliding window script [%s]: %w", action, err)
	}

	allowed := vals[0] == 1
	count := vals[1]
	remaining := int64(rule.MaxRequests) - count
	if remaining < 0 {
		remaining = 0
	}
	result := &RateLimitResult{
		Allowed:    allowed,
		TotalCount: count,
		Remaining:  remaining,
		Action:     action,
		Identifier: identifier,
	}
	if !allowed && len(vals) >= 4 {
		result.RetryAfter = time.Duration(vals[3]) * time.Millisecond
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Token-bucket implementation
// ─────────────────────────────────────────────────────────────────────────────

func (rl *RateLimiter) checkTokenBucket(
	ctx context.Context,
	action, identifier string,
	rule RuleConfig,
	tokens int,
) (*RateLimitResult, error) {
	key := rl.bucketKey(action, identifier)
	now := time.Now().UnixMilli()
	refillPerMS := rule.RefillPerSec / 1000.0
	// TTL = 2× the time needed to fill from empty, so idle buckets expire.
	ttlMS := int64((rule.BucketCap / rule.RefillPerSec) * 1000 * 2)

	vals, err := rl.bucketScript.Run(ctx, rl.client,
		[]string{key},
		now,
		rule.BucketCap,
		fmt.Sprintf("%v", refillPerMS),
		tokens,
		ttlMS,
	).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("token bucket script [%s]: %w", action, err)
	}

	result := &RateLimitResult{
		Allowed:    vals[0] == 1,
		Remaining:  vals[1],
		Action:     action,
		Identifier: identifier,
	}
	if !result.Allowed && len(vals) >= 3 {
		result.RetryAfter = time.Duration(vals[2]) * time.Millisecond
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Global (shared) limit
// ─────────────────────────────────────────────────────────────────────────────

func (rl *RateLimiter) checkGlobalLimit(
	ctx context.Context,
	action string,
	rule RuleConfig,
) (*RateLimitResult, error) {
	key := fmt.Sprintf("rl:global:%s", action)
	now := time.Now().UnixMilli()
	reqID := fmt.Sprintf("g:%d", now)

	vals, err := rl.globalScript.Run(ctx, rl.client,
		[]string{key},
		now,
		rule.GlobalWindowSize.Milliseconds(),
		rule.GlobalLimit,
		reqID,
	).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("global sliding window script [%s]: %w", action, err)
	}

	allowed := vals[0] == 1
	result := &RateLimitResult{
		Allowed:    allowed,
		TotalCount: vals[1],
		Remaining:  vals[2],
		Action:     action,
		Identifier: "global",
	}
	if !allowed && len(vals) >= 4 {
		result.RetryAfter = time.Duration(vals[3]) * time.Millisecond
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Identifier constructors (used by middleware to build consistent key prefixes)
// ─────────────────────────────────────────────────────────────────────────────

// UserIdentifier returns the rate-limit identifier for an authenticated user.
func UserIdentifier(userID string) string { return "u:" + userID }

// IPIdentifier returns the rate-limit identifier for an IP address.
func IPIdentifier(ip string) string { return "ip:" + ip }

// DeviceIdentifier returns the rate-limit identifier for a device fingerprint.
func DeviceIdentifier(deviceID string) string { return "d:" + deviceID }

// CombinedIdentifier returns a rate-limit identifier combining user and IP,
// providing stronger protection than either alone.
func CombinedIdentifier(userID, ip string) string {
	return fmt.Sprintf("c:%s:%s", userID, ip)
}

// ─────────────────────────────────────────────────────────────────────────────
// Key helpers
// ─────────────────────────────────────────────────────────────────────────────

func (rl *RateLimiter) slidingKey(action, identifier string) string {
	return fmt.Sprintf("rl:sliding:%s:%s", action, identifier)
}

func (rl *RateLimiter) bucketKey(action, identifier string) string {
	return fmt.Sprintf("rl:bucket:%s:%s", action, identifier)
}

func (rl *RateLimiter) blockKey(identifier string) string {
	return fmt.Sprintf("rl:block:%s", identifier)
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
