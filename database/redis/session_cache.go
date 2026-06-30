package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// Session key patterns:
//
//	session:{sessionID}                -> JSON SessionData blob (STRING, TTL = SessionTTL)
//	user_sessions:{userID}             -> set of active sessionIDs (SET, TTL = RefreshTTL)
//	device_session:{userID}:{deviceID} -> sessionID currently bound to this device (STRING)
//	revoked_tokens                     -> sorted set: JTI -> expiry unix ts (ZSET, no global TTL)
//	session_stats:{userID}             -> hash of per-user session statistics (HASH)
//	session_ip_index:{ipHash}          -> set of sessionIDs originating from this IP (SET, TTL = 7d)
const (
	// SessionTTL is the rolling lifetime of an active session.
	// Every successful GetSession call resets this timer (sliding expiry).
	SessionTTL = 7 * 24 * time.Hour

	// RefreshTTL is the lifetime of the user-sessions set.
	// Longer than SessionTTL to survive a brief session gap.
	RefreshTTL = 30 * 24 * time.Hour

	// MaxSessionsPerUser is the maximum number of concurrent device sessions.
	// Exceeding this evicts the oldest active session.
	MaxSessionsPerUser = 10

	// RevokedTokenPruneBatchSize controls how many expired JTIs are removed per prune call.
	RevokedTokenPruneBatchSize = 10000

	keySession        = "session:%s"
	keyUserSessions   = "user_sessions:%s"
	keyRevokedTokens  = "revoked_tokens"
	keyDeviceSession  = "device_session:%s:%s"
	keySessionStats   = "session_stats:%s"
	keySessionIPIndex = "session_ip_index:%s"
)

// SessionData holds everything stored per session in Redis.
// All fields are serialised as JSON to a single Redis STRING key.
type SessionData struct {
	SessionID    string    `json:"session_id"`
	UserID       string    `json:"user_id"`
	DeviceID     string    `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	JTI          string    `json:"jti"`           // JWT ID for the current access token
	RefreshToken string    `json:"refresh_token"` // bcrypt hash of the refresh token
	CreatedAt    time.Time `json:"created_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       []string  `json:"scopes"`
	IsActive     bool      `json:"is_active"`
	// Geo and security fields
	Country    string `json:"country,omitempty"`
	City       string `json:"city,omitempty"`
	DeviceType string `json:"device_type,omitempty"` // mobile | tablet | desktop | tv
	AppVersion string `json:"app_version,omitempty"`
}

// SessionStats tracks aggregate session metrics per user stored in a Redis hash.
type SessionStats struct {
	TotalCreated  int64     `json:"total_created"`
	TotalRevoked  int64     `json:"total_revoked"`
	LastLoginAt   time.Time `json:"last_login_at"`
	LastLoginIP   string    `json:"last_login_ip"`
	FailedLogins  int64     `json:"failed_logins"`
}

// SessionCache manages JWT sessions in Redis.
type SessionCache struct {
	client *goredis.Client

	// luaReleaseLock is a pre-compiled Lua script for atomic lock release.
	luaReleaseLock *goredis.Script
	// luaRevokeSession atomically reads session JSON, adds JTI to revocation
	// ZSET, and deletes the session key — all in a single round-trip.
	luaRevokeSession *goredis.Script
}

// NewSessionCache constructs a SessionCache backed by the given client.
// Pre-compiles all Lua scripts so SHA caching happens once at startup.
func NewSessionCache(client *goredis.Client) *SessionCache {
	return &SessionCache{
		client: client,
		luaReleaseLock: goredis.NewScript(`
			if redis.call('GET', KEYS[1]) == ARGV[1] then
				return redis.call('DEL', KEYS[1])
			end
			return 0
		`),
		luaRevokeSession: goredis.NewScript(`
			-- KEYS[1] = session key
			-- KEYS[2] = revoked_tokens ZSET
			-- ARGV[1] = JTI
			-- ARGV[2] = expiry unix timestamp (score)
			local raw = redis.call('GET', KEYS[1])
			if not raw then return 0 end
			redis.call('ZADD', KEYS[2], ARGV[2], ARGV[1])
			redis.call('DEL', KEYS[1])
			return 1
		`),
	}
}

// ----------------------------------------------------------------------------
// Session lifecycle
// ----------------------------------------------------------------------------

// CreateSession persists a new session, binds it to the device, and enforces
// the per-user concurrent session limit.
//
// If the same device already has a session it is atomically replaced.
// If the user exceeds MaxSessionsPerUser, the oldest session is evicted.
func (sc *SessionCache) CreateSession(ctx context.Context, data *SessionData) error {
	if data.SessionID == "" {
		data.SessionID = uuid.New().String()
	}
	if data.JTI == "" {
		data.JTI = uuid.New().String()
	}
	now := time.Now().UTC()
	data.CreatedAt = now
	data.LastSeenAt = now
	data.ExpiresAt = now.Add(SessionTTL)
	data.IsActive = true

	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("session marshal: %w", err)
	}

	sessionKey := fmt.Sprintf(keySession, data.SessionID)
	userSessionsKey := fmt.Sprintf(keyUserSessions, data.UserID)
	deviceSessionKey := fmt.Sprintf(keyDeviceSession, data.UserID, data.DeviceID)

	// If a session already exists for this device, revoke it first so we do not
	// leak phantom sessions in the user_sessions set.
	existingSessionID, lookupErr := sc.client.Get(ctx, deviceSessionKey).Result()
	if lookupErr == nil && existingSessionID != "" {
		// Best-effort silent revocation of the previous device session.
		_ = sc.revokeSessionByID(ctx, existingSessionID)
	}

	pipe := sc.client.TxPipeline()
	pipe.Set(ctx, sessionKey, raw, SessionTTL)
	pipe.SAdd(ctx, userSessionsKey, data.SessionID)
	pipe.Expire(ctx, userSessionsKey, RefreshTTL)
	pipe.Set(ctx, deviceSessionKey, data.SessionID, SessionTTL)

	// Update session statistics.
	statsKey := fmt.Sprintf(keySessionStats, data.UserID)
	pipe.HIncrBy(ctx, statsKey, "total_created", 1)
	pipe.HSet(ctx, statsKey, "last_login_at", now.Unix(), "last_login_ip", data.IPAddress)
	pipe.Expire(ctx, statsKey, RefreshTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("session create pipeline: %w", err)
	}

	return sc.enforceSessionLimit(ctx, data.UserID)
}

// GetSession retrieves a session by ID and refreshes its sliding TTL.
// Returns ErrSessionNotFound if the key does not exist, ErrSessionExpired
// if the session has been marked inactive or its wall-clock expiry has passed.
func (sc *SessionCache) GetSession(ctx context.Context, sessionID string) (*SessionData, error) {
	key := fmt.Sprintf(keySession, sessionID)
	raw, err := sc.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("session get: %w", err)
	}

	var data SessionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("session unmarshal: %w", err)
	}

	if !data.IsActive {
		return nil, ErrSessionExpired
	}
	if time.Now().UTC().After(data.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	// Sliding TTL: refresh expiry on each access.
	data.LastSeenAt = time.Now().UTC()
	data.ExpiresAt = data.LastSeenAt.Add(SessionTTL)
	updated, _ := json.Marshal(data)
	// Best-effort; failure just means TTL won't slide — not fatal.
	_ = sc.client.Set(ctx, key, updated, SessionTTL).Err()

	return &data, nil
}

// TouchSession updates the last-seen timestamp and slides the TTL without
// returning the full session body. Used by lightweight keep-alive endpoints.
func (sc *SessionCache) TouchSession(ctx context.Context, sessionID string) error {
	_, err := sc.GetSession(ctx, sessionID)
	return err
}

// RefreshSession rotates the JTI (used when the client exchanges a refresh
// token for a new access token) and persists the updated session atomically.
// The old JTI is added to the revocation list so in-flight tokens are rejected.
func (sc *SessionCache) RefreshSession(ctx context.Context, sessionID, newJTI, newRefreshToken string) error {
	sess, err := sc.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("refresh get session: %w", err)
	}

	oldJTI := sess.JTI

	sess.JTI = newJTI
	sess.RefreshToken = newRefreshToken
	sess.LastSeenAt = time.Now().UTC()
	sess.ExpiresAt = sess.LastSeenAt.Add(SessionTTL)

	raw, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("refresh marshal: %w", err)
	}

	key := fmt.Sprintf(keySession, sessionID)
	pipe := sc.client.TxPipeline()
	pipe.Set(ctx, key, raw, SessionTTL)
	// Revoke the old JTI — expiry score = session expiry so it auto-ages out of the ZSET.
	pipe.ZAdd(ctx, keyRevokedTokens, goredis.Z{
		Score:  float64(sess.ExpiresAt.Unix()),
		Member: oldJTI,
	})
	_, err = pipe.Exec(ctx)
	return err
}

// RevokeSession invalidates a single session by session ID.
// The current JTI is added to the revocation sorted set so any in-flight
// access token signed with that JTI will be rejected on the next auth check.
func (sc *SessionCache) RevokeSession(ctx context.Context, sessionID string) error {
	return sc.revokeSessionByID(ctx, sessionID)
}

// revokeSessionByID is the internal implementation for session revocation.
func (sc *SessionCache) revokeSessionByID(ctx context.Context, sessionID string) error {
	sess, err := sc.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionExpired) {
			// Already gone or expired — clean up stale references silently.
			sc.client.Del(ctx, fmt.Sprintf(keySession, sessionID)) //nolint:errcheck
			return nil
		}
		return err
	}

	pipe := sc.client.TxPipeline()
	pipe.Del(ctx, fmt.Sprintf(keySession, sessionID))
	pipe.SRem(ctx, fmt.Sprintf(keyUserSessions, sess.UserID), sessionID)
	pipe.Del(ctx, fmt.Sprintf(keyDeviceSession, sess.UserID, sess.DeviceID))
	// Revocation list: score is the expiry time so old JTIs self-age out.
	pipe.ZAdd(ctx, keyRevokedTokens, goredis.Z{
		Score:  float64(sess.ExpiresAt.Unix()),
		Member: sess.JTI,
	})
	// Bump revoked counter.
	statsKey := fmt.Sprintf(keySessionStats, sess.UserID)
	pipe.HIncrBy(ctx, statsKey, "total_revoked", 1)

	_, err = pipe.Exec(ctx)
	return err
}

// RevokeAllSessions invalidates every session for a user.
// Used on password change, account compromise, or explicit "sign out everywhere".
func (sc *SessionCache) RevokeAllSessions(ctx context.Context, userID string) error {
	userSessionsKey := fmt.Sprintf(keyUserSessions, userID)
	sessionIDs, err := sc.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return fmt.Errorf("smembers user sessions: %w", err)
	}

	if len(sessionIDs) == 0 {
		return nil
	}

	// Collect JTIs and device session keys for cleanup.
	sessionKeys := make([]string, 0, len(sessionIDs))
	var jtiEntries []goredis.Z

	for _, sid := range sessionIDs {
		key := fmt.Sprintf(keySession, sid)
		raw, getErr := sc.client.Get(ctx, key).Bytes()
		if getErr != nil {
			continue
		}
		var sd SessionData
		if jsonErr := json.Unmarshal(raw, &sd); jsonErr == nil {
			sessionKeys = append(sessionKeys, key)
			jtiEntries = append(jtiEntries, goredis.Z{
				Score:  float64(sd.ExpiresAt.Unix()),
				Member: sd.JTI,
			})
			// Remove the device binding.
			sc.client.Del(ctx, fmt.Sprintf(keyDeviceSession, userID, sd.DeviceID)) //nolint:errcheck
		}
	}

	pipe := sc.client.TxPipeline()
	if len(sessionKeys) > 0 {
		pipe.Del(ctx, sessionKeys...)
	}
	pipe.Del(ctx, userSessionsKey)
	if len(jtiEntries) > 0 {
		pipe.ZAdd(ctx, keyRevokedTokens, jtiEntries...)
	}
	statsKey := fmt.Sprintf(keySessionStats, userID)
	pipe.HIncrBy(ctx, statsKey, "total_revoked", int64(len(sessionIDs)))

	_, err = pipe.Exec(ctx)
	return err
}

// RevokeSessionsByIP revokes all sessions that originated from a specific
// IP address (used for security incidents). ipHash is a hashed representation
// of the IP suitable for use as a Redis key component.
func (sc *SessionCache) RevokeSessionsByIP(ctx context.Context, userID, ipAddress string) (int, error) {
	sessions, err := sc.ListUserSessions(ctx, userID)
	if err != nil {
		return 0, err
	}
	revoked := 0
	for _, s := range sessions {
		if s.IPAddress == ipAddress {
			if rErr := sc.RevokeSession(ctx, s.SessionID); rErr == nil {
				revoked++
			}
		}
	}
	return revoked, nil
}

// ----------------------------------------------------------------------------
// Token revocation list
// ----------------------------------------------------------------------------

// RevokeToken adds a JWT ID directly to the revocation sorted set.
// Use this when you need to revoke an access token without invalidating
// the full session (e.g. on logout from a single tab but keeping other tabs active).
func (sc *SessionCache) RevokeToken(ctx context.Context, jti string, expiry time.Time) error {
	return sc.client.ZAdd(ctx, keyRevokedTokens, goredis.Z{
		Score:  float64(expiry.Unix()),
		Member: jti,
	}).Err()
}

// IsTokenRevoked reports whether a JTI appears in the revocation list and
// its associated expiry has not yet passed.
//
// A JTI is considered NOT revoked if:
//   - It is absent from the sorted set, OR
//   - Its stored score (expiry) is in the past (token is expired anyway).
func (sc *SessionCache) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	score, err := sc.client.ZScore(ctx, keyRevokedTokens, jti).Result()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("zscore revoked tokens: %w", err)
	}
	// If the associated expiry has already passed the token would be invalid
	// regardless — treat it as not-in-revocation-list to avoid false positives.
	if time.Unix(int64(score), 0).Before(time.Now().UTC()) {
		return false, nil
	}
	return true, nil
}

// PruneRevokedTokens removes JTIs whose expiry has passed from the revocation
// sorted set. Call this from a scheduled job (e.g. hourly cron) to prevent
// unbounded growth.
//
// Returns the number of JTIs removed.
func (sc *SessionCache) PruneRevokedTokens(ctx context.Context) (int64, error) {
	maxScore := fmt.Sprintf("%d", time.Now().UTC().Unix())
	removed, err := sc.client.ZRemRangeByScore(ctx, keyRevokedTokens, "-inf", maxScore).Result()
	if err != nil {
		return 0, fmt.Errorf("prune revoked tokens: %w", err)
	}
	return removed, nil
}

// RevokedTokenCount returns the number of JTIs currently in the revocation list.
func (sc *SessionCache) RevokedTokenCount(ctx context.Context) (int64, error) {
	return sc.client.ZCard(ctx, keyRevokedTokens).Result()
}

// ----------------------------------------------------------------------------
// Session querying
// ----------------------------------------------------------------------------

// ListUserSessions returns all active, non-expired sessions for a user.
// Used by the "manage devices" UI to show where the account is logged in.
func (sc *SessionCache) ListUserSessions(ctx context.Context, userID string) ([]*SessionData, error) {
	userSessionsKey := fmt.Sprintf(keyUserSessions, userID)
	sessionIDs, err := sc.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("smembers user sessions: %w", err)
	}
	if len(sessionIDs) == 0 {
		return nil, nil
	}

	keys := make([]string, len(sessionIDs))
	for i, sid := range sessionIDs {
		keys[i] = fmt.Sprintf(keySession, sid)
	}

	rawValues, err := sc.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget sessions: %w", err)
	}

	now := time.Now().UTC()
	sessions := make([]*SessionData, 0, len(rawValues))
	staleIDs := make([]string, 0)

	for i, v := range rawValues {
		if v == nil {
			// Session key has expired but set membership remains — mark for cleanup.
			staleIDs = append(staleIDs, sessionIDs[i])
			continue
		}
		var sd SessionData
		if err := json.Unmarshal([]byte(v.(string)), &sd); err != nil {
			continue
		}
		if !sd.IsActive || now.After(sd.ExpiresAt) {
			staleIDs = append(staleIDs, sd.SessionID)
			continue
		}
		sessions = append(sessions, &sd)
	}

	// Asynchronously clean stale set members.
	if len(staleIDs) > 0 {
		go func() {
			purgeCtx := context.Background()
			args := make([]interface{}, len(staleIDs))
			for i, sid := range staleIDs {
				args[i] = sid
			}
			sc.client.SRem(purgeCtx, userSessionsKey, args...) //nolint:errcheck
		}()
	}

	return sessions, nil
}

// GetSessionByDevice looks up the session currently bound to a device.
func (sc *SessionCache) GetSessionByDevice(ctx context.Context, userID, deviceID string) (*SessionData, error) {
	deviceSessionKey := fmt.Sprintf(keyDeviceSession, userID, deviceID)
	sessionID, err := sc.client.Get(ctx, deviceSessionKey).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return sc.GetSession(ctx, sessionID)
}

// ActiveSessionCount returns how many concurrent active sessions a user has.
func (sc *SessionCache) ActiveSessionCount(ctx context.Context, userID string) (int, error) {
	sessions, err := sc.ListUserSessions(ctx, userID)
	return len(sessions), err
}

// GetSessionStats returns aggregate session statistics for a user.
func (sc *SessionCache) GetSessionStats(ctx context.Context, userID string) (*SessionStats, error) {
	statsKey := fmt.Sprintf(keySessionStats, userID)
	vals, err := sc.client.HGetAll(ctx, statsKey).Result()
	if err != nil {
		return nil, err
	}
	var stats SessionStats
	if v, ok := vals["total_created"]; ok {
		fmt.Sscanf(v, "%d", &stats.TotalCreated)
	}
	if v, ok := vals["total_revoked"]; ok {
		fmt.Sscanf(v, "%d", &stats.TotalRevoked)
	}
	if v, ok := vals["last_login_ip"]; ok {
		stats.LastLoginIP = v
	}
	if v, ok := vals["last_login_at"]; ok {
		var ts int64
		fmt.Sscanf(v, "%d", &ts)
		if ts > 0 {
			stats.LastLoginAt = time.Unix(ts, 0).UTC()
		}
	}
	return &stats, nil
}

// RecordFailedLogin increments the failed login counter for security monitoring.
func (sc *SessionCache) RecordFailedLogin(ctx context.Context, userID string) error {
	statsKey := fmt.Sprintf(keySessionStats, userID)
	pipe := sc.client.Pipeline()
	pipe.HIncrBy(ctx, statsKey, "failed_logins", 1)
	pipe.Expire(ctx, statsKey, RefreshTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// ResetFailedLogins clears the failed login counter after a successful login.
func (sc *SessionCache) ResetFailedLogins(ctx context.Context, userID string) error {
	return sc.client.HSet(ctx, fmt.Sprintf(keySessionStats, userID), "failed_logins", 0).Err()
}

// ----------------------------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------------------------

// enforceSessionLimit evicts the oldest session if the user exceeds MaxSessionsPerUser.
// Oldest is determined by CreatedAt timestamp.
func (sc *SessionCache) enforceSessionLimit(ctx context.Context, userID string) error {
	sessions, err := sc.ListUserSessions(ctx, userID)
	if err != nil {
		return err
	}
	if len(sessions) <= MaxSessionsPerUser {
		return nil
	}

	// Sort ascending by creation time; evict from the front.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
	})

	evictCount := len(sessions) - MaxSessionsPerUser
	for i := 0; i < evictCount; i++ {
		if err := sc.revokeSessionByID(ctx, sessions[i].SessionID); err != nil {
			return fmt.Errorf("evict oldest session: %w", err)
		}
	}
	return nil
}

// Sentinel errors returned by SessionCache methods.
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)
