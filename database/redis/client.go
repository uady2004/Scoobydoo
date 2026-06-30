// Package redis provides all Redis-backed caching and data structures for the
// TikTok clone. It wraps go-redis/v9 and exposes domain-specific cache types:
//
//   - SessionCache     — JWT session management, token revocation
//   - FeedCache        — Per-user pre-computed feed sorted sets
//   - TrendingCache    — Time-windowed trending videos, hashtags, sounds
//   - LeaderboardCache — Creator, gifter, and live-stream leaderboards
//   - RateLimiter      — Sliding-window and token-bucket rate limiting
//   - PresenceTracker  — Online status, room presence, typing indicators
package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Config holds all Redis connection parameters. Populate from environment
// variables or a secrets manager (never hard-code credentials).
type Config struct {
	// Sentinel-based HA (preferred for production).
	SentinelAddrs    []string // e.g. ["10.0.1.11:26379","10.0.1.12:26379","10.0.1.13:26379"]
	SentinelPassword string
	MasterName       string // Sentinel master name, e.g. "tiktok-primary"

	// Standalone / replica fallback (used when SentinelAddrs is empty).
	Addr     string // host:port
	Password string
	DB       int

	// Connection pool tuning.
	PoolSize        int           // default: 10 × GOMAXPROCS
	MinIdleConns    int           // keep this many connections warm
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration // recycle idle connections after this
	ConnMaxLifetime time.Duration // recycle all connections after this
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration

	// TLS — set to non-nil to enable encrypted connections.
	TLSConfig *tls.Config

	// Health-check interval (0 = disabled).
	HealthCheckInterval time.Duration
}

// DefaultConfig returns a Config suitable for local development.
func DefaultConfig() Config {
	return Config{
		Addr:                "127.0.0.1:6379",
		DB:                  0,
		PoolSize:            50,
		MinIdleConns:        5,
		MaxIdleConns:        20,
		ConnMaxIdleTime:     5 * time.Minute,
		ConnMaxLifetime:     30 * time.Minute,
		DialTimeout:         5 * time.Second,
		ReadTimeout:         3 * time.Second,
		WriteTimeout:        3 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}
}

// ProductionSentinelConfig returns a Config template for the 3-node Sentinel
// cluster described in config/sentinel.conf.
func ProductionSentinelConfig(sentinelAddrs []string, masterName, redisPassword, sentinelPassword string) Config {
	return Config{
		SentinelAddrs:       sentinelAddrs,
		SentinelPassword:    sentinelPassword,
		MasterName:          masterName,
		Password:            redisPassword,
		DB:                  0,
		PoolSize:            200,
		MinIdleConns:        20,
		MaxIdleConns:        100,
		ConnMaxIdleTime:     5 * time.Minute,
		ConnMaxLifetime:     30 * time.Minute,
		DialTimeout:         5 * time.Second,
		ReadTimeout:         500 * time.Millisecond,
		WriteTimeout:        500 * time.Millisecond,
		HealthCheckInterval: 15 * time.Second,
	}
}

// Client is the central Redis client container. All domain caches are
// initialized from a single underlying go-redis client so they share
// the connection pool.
type Client struct {
	rdb *goredis.Client

	Session     *SessionCache
	Feed        *FeedCache
	Trending    *TrendingCache
	Leaderboard *LeaderboardCache
	RateLimit   *RateLimiter
	Presence    *PresenceTracker
}

// NewClient constructs a Client, pings Redis to verify connectivity, and
// initialises all domain caches. Call Close() when the application exits.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	var rdb *goredis.Client

	if len(cfg.SentinelAddrs) > 0 {
		// Sentinel-backed failover client.
		rdb = goredis.NewFailoverClient(&goredis.FailoverOptions{
			MasterName:       cfg.MasterName,
			SentinelAddrs:    cfg.SentinelAddrs,
			SentinelPassword: cfg.SentinelPassword,
			Password:         cfg.Password,
			DB:               cfg.DB,

			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			MaxIdleConns:    cfg.MaxIdleConns,
			ConnMaxIdleTime: cfg.ConnMaxIdleTime,
			ConnMaxLifetime: cfg.ConnMaxLifetime,
			DialTimeout:     cfg.DialTimeout,
			ReadTimeout:     cfg.ReadTimeout,
			WriteTimeout:    cfg.WriteTimeout,

			TLSConfig: cfg.TLSConfig,
		})
	} else {
		// Standalone client (development / single-node).
		rdb = goredis.NewClient(&goredis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,

			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			MaxIdleConns:    cfg.MaxIdleConns,
			ConnMaxIdleTime: cfg.ConnMaxIdleTime,
			ConnMaxLifetime: cfg.ConnMaxLifetime,
			DialTimeout:     cfg.DialTimeout,
			ReadTimeout:     cfg.ReadTimeout,
			WriteTimeout:    cfg.WriteTimeout,

			TLSConfig: cfg.TLSConfig,
		})
	}

	// Verify the connection.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	c := &Client{
		rdb:         rdb,
		Session:     NewSessionCache(rdb),
		Feed:        NewFeedCache(rdb),
		Trending:    NewTrendingCache(rdb),
		Leaderboard: NewLeaderboardCache(rdb),
		RateLimit:   NewRateLimiter(rdb, DefaultRules),
		Presence:    NewPresenceTracker(rdb),
	}

	return c, nil
}

// Close shuts down the underlying connection pool gracefully.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Raw returns the underlying go-redis client for operations not covered
// by the domain caches. Use sparingly.
func (c *Client) Raw() *goredis.Client {
	return c.rdb
}

// HealthCheck performs a PING and returns an error if Redis is unreachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// FlushDB removes all keys from the current database.
// ONLY call this in test environments; it is a no-op behind a build tag in production.
func (c *Client) FlushDB(ctx context.Context) error {
	return c.rdb.FlushDB(ctx).Err()
}

// Pipeline exposes the underlying pipeline for callers that need to batch
// multiple cross-domain operations.
func (c *Client) Pipeline() goredis.Pipeliner {
	return c.rdb.Pipeline()
}

// TxPipeline exposes the underlying transaction pipeline.
func (c *Client) TxPipeline() goredis.Pipeliner {
	return c.rdb.TxPipeline()
}

// Stats returns connection pool statistics for metrics/monitoring.
func (c *Client) Stats() *goredis.PoolStats {
	return c.rdb.PoolStats()
}
