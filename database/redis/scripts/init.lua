-- =============================================================================
-- TikTok Clone — Redis Lua Scripts
-- File: database/redis/scripts/init.lua
--
-- PURPOSE
-- -------
-- This file is the canonical source for every Lua script used by the Redis
-- layer.  Each script is:
--   1. Defined here with full documentation.
--   2. Embedded verbatim as a Go string constant in the corresponding
--      domain file (rate_limit.go, feed_cache.go, etc.) and registered via
--      goredis.NewScript() for automatic EVALSHA caching.
--   3. Loadable standalone for debugging:
--        redis-cli -a $REDIS_PASSWORD SCRIPT LOAD "$(cat init.lua)"
--
-- ATOMICITY GUARANTEE
-- -------------------
-- All scripts execute as a single Redis command — no other client can
-- interleave commands between the Lua statements.  This eliminates TOCTOU
-- races that would exist if the same logic were split across multiple
-- round-trips from Go.
--
-- NAMING CONVENTION
-- -----------------
--   <domain>_<operation>  e.g. rl_sliding_window, feed_fan_out, session_revoke
--
-- LOADING AT STARTUP
-- ------------------
-- From Go (recommended — go-redis caches the SHA automatically):
--   script := goredis.NewScript(scriptSource)
--   err    := script.Load(ctx, redisClient).Err()
--
-- From the CLI (useful for manual debugging):
--   redis-cli -a $REDIS_PASSWORD SCRIPT LOAD "$(cat init.lua)"
--
-- List loaded scripts by SHA:
--   redis-cli -a $REDIS_PASSWORD SCRIPT EXISTS <sha1> <sha2> ...
--
-- Flush all scripts after a deploy (if script source changed):
--   redis-cli -a $REDIS_PASSWORD SCRIPT FLUSH ASYNC
-- =============================================================================


-- ---------------------------------------------------------------------------
-- SCRIPT: rl_sliding_window
-- Domain: Rate limiting  (rate_limit.go — slidingWindowLua)
-- ---------------------------------------------------------------------------
-- Atomic sliding-window request admission.
-- Evicts expired timestamps, counts the window, admits or rejects the request,
-- and returns machine-readable retry information — all in one round-trip.
--
-- KEYS[1]  = rate-limit sorted-set key
--            e.g. "rl:sliding:comment:u:abc123"
--
-- ARGV[1]  = current time in Unix milliseconds  (string)
-- ARGV[2]  = window size in milliseconds         (string)
-- ARGV[3]  = maximum requests allowed in window  (string)
-- ARGV[4]  = unique request ID — prevents member collision when two requests
--            arrive within the same millisecond
--            e.g. "<now_ms>-<identifier>-<n>"
--
-- Returns: array of integers
--   [1] = 1 (admitted) | 0 (rejected)
--   [2] = request count AFTER this call (or current count when rejected)
--   [3] = remaining capacity (0 when rejected)
--   [4] = retry-after in ms (only meaningful when rejected; 0 otherwise)
-- ---------------------------------------------------------------------------
local function rl_sliding_window()
    local key         = KEYS[1]
    local now         = tonumber(ARGV[1])
    local window_ms   = tonumber(ARGV[2])
    local limit       = tonumber(ARGV[3])
    local req_id      = ARGV[4]

    local window_start = now - window_ms

    -- 1. Evict timestamps that have aged out of the window.
    --    This bounds the sorted-set cardinality to at most `limit` members.
    redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

    -- 2. Count remaining requests in the current window.
    local count = redis.call('ZCARD', key)

    if count < limit then
        -- Admit: record this request with the current timestamp as its score.
        redis.call('ZADD', key, now, req_id)
        -- Set key TTL to window_ms so it self-cleans when traffic stops.
        redis.call('PEXPIRE', key, window_ms)
        return {1, count + 1, limit - count - 1, 0}
    end

    -- Rejected: calculate how long until the oldest entry ages out.
    local oldest      = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retry_after = 0
    if #oldest > 0 then
        retry_after = window_ms - (now - tonumber(oldest[2]))
        if retry_after < 0 then retry_after = 0 end
    end
    return {0, count, 0, retry_after}
end

-- Entry point when loaded as a standalone script (not used by go-redis).
return rl_sliding_window()


-- ---------------------------------------------------------------------------
-- SCRIPT: rl_token_bucket
-- Domain: Rate limiting  (rate_limit.go — tokenBucketLua)
-- ---------------------------------------------------------------------------
-- Atomic token-bucket with continuous refill.
-- Reads the current bucket state, refills based on elapsed time, consumes
-- the requested tokens, and persists the new state — all atomically.
--
-- KEYS[1]  = bucket hash key
--            e.g. "rl:bucket:api_general:u:abc123"
--
-- ARGV[1]  = current Unix time in milliseconds
-- ARGV[2]  = bucket capacity (max tokens, integer treated as float)
-- ARGV[3]  = refill rate in tokens-per-millisecond (float string, e.g. "0.05")
-- ARGV[4]  = tokens required for this request (integer)
-- ARGV[5]  = key TTL in milliseconds (recommend 2× fill-from-empty time)
--
-- Returns: array of integers
--   [1] = 1 (admitted) | 0 (rejected)
--   [2] = remaining tokens after this call (floored to integer)
--   [3] = estimated wait ms until requested tokens are available (only when rejected)
-- ---------------------------------------------------------------------------
--[[  [EMBEDDED IN rate_limit.go — tokenBucketLua]
local function rl_token_bucket()
    local key         = KEYS[1]
    local now         = tonumber(ARGV[1])
    local capacity    = tonumber(ARGV[2])
    local refill_rate = tonumber(ARGV[3])   -- tokens / ms
    local requested   = tonumber(ARGV[4])
    local ttl_ms      = tonumber(ARGV[5])

    local bucket = redis.call('HMGET', key, 'tokens', 'ts')
    local tokens = tonumber(bucket[1])
    local ts     = tonumber(bucket[2])

    if tokens == nil then
        -- First access: start with a full bucket.
        tokens = capacity
        ts     = now
    end

    -- Refill proportional to elapsed time, capped at capacity.
    local elapsed = math.max(0, now - ts)
    tokens = math.min(capacity, tokens + elapsed * refill_rate)

    if tokens >= requested then
        tokens = tokens - requested
        redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
        redis.call('PEXPIRE', key, ttl_ms)
        return {1, math.floor(tokens), 0}
    end

    -- Not enough tokens — persist the refilled state and return wait time.
    local deficit  = requested - tokens
    local wait_ms  = math.ceil(deficit / refill_rate)
    redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
    redis.call('PEXPIRE', key, ttl_ms)
    return {0, math.floor(tokens), wait_ms}
end
--]]


-- ---------------------------------------------------------------------------
-- SCRIPT: session_revoke
-- Domain: Session management  (session_cache.go — luaRevokeSession)
-- ---------------------------------------------------------------------------
-- Atomically reads a session key, adds its JTI to the revocation sorted set,
-- and deletes the session — in a single execution with no TOCTOU gap between
-- reading and deleting.
--
-- KEYS[1]  = session key          e.g. "session:{sessionID}"
-- KEYS[2]  = revoked_tokens ZSET  (global revocation list)
--
-- ARGV[1]  = JTI to revoke        (sorted-set member)
-- ARGV[2]  = expiry unix timestamp (score — allows age-based pruning)
--
-- Returns: 1 if the session existed and was revoked; 0 if it did not exist.
-- ---------------------------------------------------------------------------
--[[  [EMBEDDED IN session_cache.go — luaRevokeSession]
local function session_revoke()
    local session_key  = KEYS[1]
    local revoked_key  = KEYS[2]
    local jti          = ARGV[1]
    local expiry_score = tonumber(ARGV[2])

    local raw = redis.call('GET', session_key)
    if not raw then
        return 0
    end

    -- Add the JTI to the revocation list. Score = expiry timestamp so
    -- expired JTIs can be pruned with ZREMRANGEBYSCORE in one call.
    redis.call('ZADD', revoked_key, expiry_score, jti)

    -- Delete the session immediately.
    redis.call('DEL', session_key)

    return 1
end
--]]


-- ---------------------------------------------------------------------------
-- SCRIPT: feed_fan_out
-- Domain: Feed management  (feed_cache.go — luaFanOut)
-- ---------------------------------------------------------------------------
-- Pushes a single video into a batch of following-feed sorted sets and trims
-- each set to the configured window size in one Lua execution.  Batching into
-- a single script reduces RTT overhead when a creator with 100 k followers
-- uploads a video.
--
-- KEYS[1..N]  = following_feed:{userID} sorted-set keys (one per follower batch)
--
-- ARGV[1]     = video score (float string: unix nanoseconds / 1e9)
-- ARGV[2]     = videoID (sorted-set member)
-- ARGV[3]     = window size  — max entries to keep per feed (integer)
-- ARGV[4]     = feed TTL in seconds (integer)
--
-- Returns: number of feeds updated.
-- ---------------------------------------------------------------------------
--[[  [EMBEDDED IN feed_cache.go — luaFanOut]
local function feed_fan_out()
    local score       = ARGV[1]
    local video_id    = ARGV[2]
    local window_size = tonumber(ARGV[3])
    local ttl_secs    = tonumber(ARGV[4])
    local updated     = 0

    for i = 1, #KEYS do
        local key = KEYS[i]
        redis.call('ZADD', key, score, video_id)
        -- Remove entries ranked below the window (lowest scores evicted first).
        redis.call('ZREMRANGEBYRANK', key, 0, -(window_size + 1))
        redis.call('EXPIRE', key, ttl_secs)
        updated = updated + 1
    end
    return updated
end
--]]


-- ---------------------------------------------------------------------------
-- SCRIPT: distributed_lock_release
-- Domain: General distributed locking
--         (feed_cache.go, trending_cache.go — luaReleaseLock)
-- ---------------------------------------------------------------------------
-- Releases a Redis-backed distributed lock only if the caller is still the
-- current owner.  Prevents a slow process from releasing a lock acquired by
-- a different process after the original TTL expired.
--
-- KEYS[1]  = lock key  e.g. "feed_lock:{userID}"
-- ARGV[1]  = lock value (random token set at acquisition time)
--
-- Returns: 1 if the lock was released; 0 if not owned by this caller.
-- ---------------------------------------------------------------------------
--[[  [EMBEDDED IN feed_cache.go, trending_cache.go — luaReleaseLock]
local function distributed_lock_release()
    if redis.call('GET', KEYS[1]) == ARGV[1] then
        return redis.call('DEL', KEYS[1])
    end
    return 0
end
--]]


-- ---------------------------------------------------------------------------
-- SCRIPT: presence_heartbeat
-- Domain: Presence tracking  (presence.go — luaHeartbeat)
-- ---------------------------------------------------------------------------
-- Atomically updates the per-user presence STRING key and the global online
-- sorted set in one round-trip.  Avoids a race where the ZSET score is
-- updated but the STRING expires before the next read.
--
-- KEYS[1]  = presence:{userID}  STRING key
-- KEYS[2]  = presence:online    sorted set (global)
--
-- ARGV[1]  = serialised PresenceInfo JSON blob
-- ARGV[2]  = unix timestamp seconds (ZSET score)
-- ARGV[3]  = presence STRING TTL in seconds
-- ARGV[4]  = userID (ZSET member)
--
-- Returns: 1 always.
-- ---------------------------------------------------------------------------
--[[  [EMBEDDED IN presence.go — luaHeartbeat]
local function presence_heartbeat()
    local presence_key = KEYS[1]
    local online_key   = KEYS[2]
    local json_blob    = ARGV[1]
    local now_ts       = tonumber(ARGV[2])
    local ttl_secs     = tonumber(ARGV[3])
    local user_id      = ARGV[4]

    redis.call('SET', presence_key, json_blob, 'EX', ttl_secs)
    redis.call('ZADD', online_key, now_ts, user_id)
    return 1
end
--]]


-- ---------------------------------------------------------------------------
-- SCRIPT: leaderboard_record_gift
-- Domain: Leaderboards  (leaderboard_cache.go — luaRecordGift)
-- ---------------------------------------------------------------------------
-- Atomically increments a gifter's score across all four period leaderboards
-- (daily, weekly, monthly, alltime) and trims each to the top-N in one
-- execution.  Avoids four separate ZINCRBY + ZREMRANGEBYRANK round-trips.
--
-- KEYS[1..4]  = lb:gifters:{daily|weekly|monthly|alltime}
--
-- ARGV[1]     = userID (sorted-set member)
-- ARGV[2]     = diamond value to add (integer string)
-- ARGV[3]     = top-N limit per leaderboard (integer string)
-- ARGV[4..7]  = TTL in seconds for each period (0 = persist forever)
--
-- Returns: array of new float scores per period (one element per KEYS entry).
-- ---------------------------------------------------------------------------
--[[  [EMBEDDED IN leaderboard_cache.go — luaRecordGift]
local function leaderboard_record_gift()
    local user_id  = ARGV[1]
    local diamonds = tonumber(ARGV[2])
    local top_n    = tonumber(ARGV[3])
    local scores   = {}

    for i = 1, #KEYS do
        local key       = KEYS[i]
        local new_score = redis.call('ZINCRBY', key, diamonds, user_id)

        -- Trim: remove entries ranked below top_n (0-indexed from bottom).
        redis.call('ZREMRANGEBYRANK', key, 0, -(top_n + 1))

        local ttl = tonumber(ARGV[3 + i])   -- ARGV[4], [5], [6], [7]
        if ttl and ttl > 0 then
            redis.call('EXPIRE', key, ttl)
        end
        scores[i] = new_score
    end
    return scores
end
--]]


-- ---------------------------------------------------------------------------
-- SCRIPT: trending_bulk_upsert
-- Domain: Trending  (trending_cache.go — luaBulkUpsert)
-- ---------------------------------------------------------------------------
-- Updates a single item's score across three time-window sorted sets and
-- trims each to top-N in one atomic execution.  Called per video/hashtag/sound
-- on every engagement event for consistent cross-window updates.
--
-- KEYS[1..3]  = trending:{videos|hashtags|sounds}:{1h|24h|7d}
--
-- ARGV[1]     = top-N limit (integer)
-- ARGV[2]     = TTL seconds for 1h window  (integer)
-- ARGV[3]     = TTL seconds for 24h window (integer)
-- ARGV[4]     = TTL seconds for 7d window  (integer)
-- ARGV[5]     = score (float string)
-- ARGV[6]     = member (videoID / hashtag / soundID)
--
-- Returns: 1 on success.
-- ---------------------------------------------------------------------------
--[[  [EMBEDDED IN trending_cache.go — luaBulkUpsert]
local function trending_bulk_upsert()
    local top_n  = tonumber(ARGV[1])
    local ttls   = {tonumber(ARGV[2]), tonumber(ARGV[3]), tonumber(ARGV[4])}
    local score  = ARGV[5]
    local member = ARGV[6]

    for i = 1, #KEYS do
        redis.call('ZADD', KEYS[i], score, member)
        redis.call('ZREMRANGEBYRANK', KEYS[i], 0, -(top_n + 1))
        if ttls[i] and ttls[i] > 0 then
            redis.call('EXPIRE', KEYS[i], ttls[i])
        end
    end
    return 1
end
--]]


-- ---------------------------------------------------------------------------
-- SCRIPT: feed_seen_filter
-- Domain: Feed deduplication  (feed_cache.go — not yet embedded, future use)
-- ---------------------------------------------------------------------------
-- Atomically adds a batch of videoIDs to the user's seen-set and returns the
-- subset of videoIDs that were NOT already in the set (i.e. unseen videos).
-- This lets the feed service filter duplicates without a separate SMISMEMBER
-- round-trip followed by SADD.
--
-- KEYS[1]  = feed_seen:{userID}
--
-- ARGV[1]  = TTL in seconds for the seen set
-- ARGV[2..N] = videoIDs to check and mark
--
-- Returns: array of videoIDs that were NOT previously seen (unseen subset).
-- ---------------------------------------------------------------------------
--[[  [CANDIDATE FOR feed_cache.go]
local function feed_seen_filter()
    local key      = KEYS[1]
    local ttl_secs = tonumber(ARGV[1])
    local unseen   = {}

    for i = 2, #ARGV do
        local vid = ARGV[i]
        -- SADD returns 1 if the element was added (i.e. it was not already present).
        local added = redis.call('SADD', key, vid)
        if added == 1 then
            unseen[#unseen + 1] = vid
        end
    end

    redis.call('EXPIRE', key, ttl_secs)
    return unseen
end
--]]


-- ---------------------------------------------------------------------------
-- SCRIPT: session_touch
-- Domain: Session management  (session_cache.go — sliding TTL refresh)
-- ---------------------------------------------------------------------------
-- Reads the session JSON, updates last_seen, resets the expiry, and writes
-- back — atomically.  Prevents the session from expiring mid-request when the
-- GET and SET are split across two commands.
--
-- KEYS[1]  = session:{sessionID}
--
-- ARGV[1]  = new last_seen unix timestamp (seconds)
-- ARGV[2]  = new expires_at unix timestamp (seconds)
-- ARGV[3]  = TTL in seconds
--
-- Returns: 1 if the session was found and refreshed; 0 if not found.
-- ---------------------------------------------------------------------------
--[[  [CANDIDATE FOR session_cache.go]
local function session_touch()
    local key          = KEYS[1]
    local last_seen_ts = ARGV[1]
    local expires_ts   = ARGV[2]
    local ttl_secs     = tonumber(ARGV[3])

    local raw = redis.call('GET', key)
    if not raw then
        return 0
    end

    -- Lua's cjson library is available in Redis 2.6+.
    local ok, data = pcall(cjson.decode, raw)
    if not ok then
        return 0
    end

    data['last_seen_at'] = last_seen_ts
    data['expires_at']   = expires_ts

    local updated = cjson.encode(data)
    redis.call('SET', key, updated, 'EX', ttl_secs)
    return 1
end
--]]


-- =============================================================================
-- END OF SCRIPT CATALOGUE
-- =============================================================================
--
-- When adding a new script:
--   1. Define and document it here (active or commented-out).
--   2. Embed the Lua source as a Go string constant in the relevant domain file.
--   3. Register with goredis.NewScript(source) and call .Load(ctx, client) at startup.
--   4. Call the script with script.Run(ctx, client, keys, args...) — go-redis
--      will automatically use EVALSHA and fall back to EVAL on cache miss.
-- =============================================================================
