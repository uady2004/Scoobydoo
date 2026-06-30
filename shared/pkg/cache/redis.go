// Package cache provides a Redis client wrapper with connection pooling,
// distributed locks, and pub/sub support for the TikTok-clone platform.
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ---- Configuration ----------------------------------------------------------

// Config holds Redis connection settings.
type Config struct {
	// Addr is host:port of the Redis server (standalone mode).
	Addr string
	// Addrs is used for cluster mode when len > 1.
	Addrs []string
	// Password for AUTH.
	Password string
	// DB index (ignored in cluster mode).
	DB int
	// PoolSize is the maximum number of socket connections per CPU.
	// Defaults to 10.
	PoolSize int
	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int
	// DialTimeout for establishing new connections.
	DialTimeout time.Duration
	// ReadTimeout for socket reads.
	ReadTimeout time.Duration
	// WriteTimeout for socket writes.
	WriteTimeout time.Duration
	// MaxRetries on network errors (-1 disables retries).
	MaxRetries int
	// ConnMaxIdleTime closes connections idle longer than this.
	ConnMaxIdleTime time.Duration
}

func (c *Config) defaults() {
	if c.Addr == "" && len(c.Addrs) == 0 {
		c.Addr = "localhost:6379"
	}
	if c.PoolSize == 0 {
		c.PoolSize = 10
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 5 * time.Minute
	}
}

// ---- Client -----------------------------------------------------------------

// Client wraps a redis.UniversalClient (works transparently with standalone,
// sentinel, and cluster topologies).
type Client struct {
	rdb redis.UniversalClient
	cfg Config
}

// New creates a Client and verifies connectivity with a PING.
func New(cfg Config) (*Client, error) {
	cfg.defaults()

	opts := &redis.UniversalOptions{
		Addrs:           addrs(cfg),
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		MaxRetries:      cfg.MaxRetries,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	}

	rdb := redis.NewUniversalClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("cache: redis ping failed: %w", err)
	}

	return &Client{rdb: rdb, cfg: cfg}, nil
}

// Close closes the underlying connection pool.
func (c *Client) Close() error { return c.rdb.Close() }

// Raw returns the underlying redis.UniversalClient for advanced usage.
func (c *Client) Raw() redis.UniversalClient { return c.rdb }

// ---- Basic key/value --------------------------------------------------------

// Set stores value at key with the given TTL (0 = no expiry).
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Get retrieves the string value at key.
// Returns ErrKeyNotFound if the key does not exist.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrKeyNotFound
	}
	return val, err
}

// GetBytes retrieves the raw bytes at key.
func (c *Client) GetBytes(ctx context.Context, key string) ([]byte, error) {
	val, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrKeyNotFound
	}
	return val, err
}

// Delete removes one or more keys.
func (c *Client) Delete(ctx context.Context, keys ...string) (int64, error) {
	return c.rdb.Del(ctx, keys...).Result()
}

// Exists reports how many of the supplied keys exist.
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.rdb.Exists(ctx, keys...).Result()
}

// Expire refreshes the TTL on a key.
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.rdb.Expire(ctx, key, ttl).Result()
}

// TTL returns the remaining TTL on a key. Returns -1 if no TTL is set,
// -2 if the key does not exist.
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.rdb.TTL(ctx, key).Result()
}

// Incr atomically increments an integer key.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

// IncrBy atomically increments an integer key by delta.
func (c *Client) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return c.rdb.IncrBy(ctx, key, delta).Result()
}

// SetNX sets key to value only if the key does not exist.
// Returns true if the key was set.
func (c *Client) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, value, ttl).Result()
}

// MGet fetches multiple keys in a single round-trip.
// Nil entries indicate missing keys.
func (c *Client) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	return c.rdb.MGet(ctx, keys...).Result()
}

// ---- Hash operations --------------------------------------------------------

// HSet sets field(s) on a hash key. values must be key-value pairs.
func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) error {
	return c.rdb.HSet(ctx, key, values...).Err()
}

// HGet retrieves a field from a hash.
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := c.rdb.HGet(ctx, key, field).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrKeyNotFound
	}
	return val, err
}

// HGetAll retrieves all fields and values from a hash.
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.rdb.HGetAll(ctx, key).Result()
}

// HDel removes fields from a hash.
func (c *Client) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	return c.rdb.HDel(ctx, key, fields...).Result()
}

// ---- Sorted set operations --------------------------------------------------

// ZAdd adds members with scores to a sorted set.
func (c *Client) ZAdd(ctx context.Context, key string, members ...redis.Z) (int64, error) {
	return c.rdb.ZAdd(ctx, key, members...).Result()
}

// ZRange returns members in ascending score order.
func (c *Client) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.rdb.ZRange(ctx, key, start, stop).Result()
}

// ZRevRange returns members in descending score order.
func (c *Client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.rdb.ZRevRange(ctx, key, start, stop).Result()
}

// ZScore returns the score of a member.
func (c *Client) ZScore(ctx context.Context, key, member string) (float64, error) {
	score, err := c.rdb.ZScore(ctx, key, member).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrKeyNotFound
	}
	return score, err
}

// ZRem removes members from a sorted set.
func (c *Client) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return c.rdb.ZRem(ctx, key, members...).Result()
}

// ---- List operations --------------------------------------------------------

// LPush prepends values to a list.
func (c *Client) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	return c.rdb.LPush(ctx, key, values...).Result()
}

// RPush appends values to a list.
func (c *Client) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	return c.rdb.RPush(ctx, key, values...).Result()
}

// LRange returns elements from a list.
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.rdb.LRange(ctx, key, start, stop).Result()
}

// LLen returns the length of a list.
func (c *Client) LLen(ctx context.Context, key string) (int64, error) {
	return c.rdb.LLen(ctx, key).Result()
}

// ---- Distributed lock -------------------------------------------------------

const (
	// DefaultLockTTL is the default lease time for a distributed lock.
	DefaultLockTTL = 30 * time.Second
	// lockScript is a Lua script that atomically extends or releases a lock
	// only if we are still the owner.
	lockReleaseScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end`
	lockExtendScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("pexpire", KEYS[1], ARGV[2])
else
    return 0
end`
)

// Lock represents a held distributed lock.
type Lock struct {
	client *Client
	key    string
	token  string
	ttl    time.Duration
}

// AcquireLock tries to acquire a distributed lock on key.
// token must be unique per lock holder (e.g. a UUID). Returns ErrLockNotAcquired
// if the lock is already held.
func (c *Client) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (*Lock, error) {
	if ttl <= 0 {
		ttl = DefaultLockTTL
	}
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("cache: acquiring lock %q: %w", key, err)
	}
	if !ok {
		return nil, ErrLockNotAcquired
	}
	return &Lock{client: c, key: key, token: token, ttl: ttl}, nil
}

// Release releases the lock. It is safe to call multiple times.
func (l *Lock) Release(ctx context.Context) error {
	res, err := l.client.rdb.Eval(ctx, lockReleaseScript, []string{l.key}, l.token).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("cache: releasing lock %q: %w", l.key, err)
	}
	if res == 0 {
		return ErrLockNotOwned
	}
	return nil
}

// Extend prolongs the lock TTL by another ttl duration, but only if this
// process still owns it.
func (l *Lock) Extend(ctx context.Context) error {
	ms := l.ttl.Milliseconds()
	res, err := l.client.rdb.Eval(ctx, lockExtendScript, []string{l.key}, l.token, ms).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("cache: extending lock %q: %w", l.key, err)
	}
	if res == 0 {
		return ErrLockNotOwned
	}
	return nil
}

// ---- Pub/Sub ----------------------------------------------------------------

// Subscribe returns a *redis.PubSub subscription to the named channels.
// The caller must close the subscription when done.
func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.rdb.Subscribe(ctx, channels...)
}

// PSubscribe returns a *redis.PubSub subscription using pattern matching.
func (c *Client) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	return c.rdb.PSubscribe(ctx, patterns...)
}

// Publish sends a message to a channel. Returns the number of subscribers
// that received the message.
func (c *Client) Publish(ctx context.Context, channel string, message interface{}) (int64, error) {
	return c.rdb.Publish(ctx, channel, message).Result()
}

// ---- Pipeline / transaction -------------------------------------------------

// Pipeline returns a redis.Pipeliner for batching commands.
func (c *Client) Pipeline() redis.Pipeliner { return c.rdb.Pipeline() }

// TxPipelined executes fn inside a MULTI/EXEC transaction.
func (c *Client) TxPipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return c.rdb.TxPipelined(ctx, fn)
}

// ---- Health -----------------------------------------------------------------

// Ping sends a PING to Redis and returns an error if unreachable.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// ---- Sentinel errors --------------------------------------------------------

var (
	// ErrKeyNotFound is returned when a Redis key does not exist.
	ErrKeyNotFound = errors.New("cache: key not found")
	// ErrLockNotAcquired is returned when AcquireLock fails because the lock
	// is already held by another process.
	ErrLockNotAcquired = errors.New("cache: lock not acquired")
	// ErrLockNotOwned is returned when Release or Extend is called by a process
	// that no longer owns the lock.
	ErrLockNotOwned = errors.New("cache: lock not owned")
)

// ---- helpers ----------------------------------------------------------------

func addrs(cfg Config) []string {
	if len(cfg.Addrs) > 0 {
		return cfg.Addrs
	}
	return []string{cfg.Addr}
}
