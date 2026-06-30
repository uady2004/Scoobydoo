// Package cache provides typed JSON helpers and a cache-aside pattern on top of
// shared/pkg/cache, reducing boilerplate in service layers.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pkgcache "github.com/tiktok-clone/shared/pkg/cache"
)

// ErrCacheMiss is returned when a key is not present in the cache.
var ErrCacheMiss = errors.New("cache: miss")

// Client wraps *pkgcache.Client with typed JSON helpers.
type Client struct {
	c *pkgcache.Client
}

// New creates a Client from a pkgcache.Client.
func New(c *pkgcache.Client) *Client {
	return &Client{c: c}
}

// Raw returns the underlying pkgcache.Client.
func (c *Client) Raw() *pkgcache.Client { return c.c }

// ---- JSON helpers -------------------------------------------------------------

// SetJSON marshals v to JSON and stores it under key with the given TTL.
func (c *Client) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("cache: marshalling value for %q: %w", key, err)
	}
	return c.c.Set(ctx, key, data, ttl)
}

// GetJSON fetches the value at key and unmarshals it into dst.
// Returns ErrCacheMiss if the key does not exist.
func (c *Client) GetJSON(ctx context.Context, key string, dst any) error {
	data, err := c.c.GetBytes(ctx, key)
	if err != nil {
		if errors.Is(err, pkgcache.ErrKeyNotFound) {
			return ErrCacheMiss
		}
		return fmt.Errorf("cache: getting %q: %w", key, err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("cache: unmarshalling value for %q: %w", key, err)
	}
	return nil
}

// Delete removes one or more keys. Returns the number of keys actually deleted.
func (c *Client) Delete(ctx context.Context, keys ...string) (int64, error) {
	return c.c.Delete(ctx, keys...)
}

// Exists reports whether key exists.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.c.Exists(ctx, key)
	return n > 0, err
}

// ---- Cache-aside pattern ------------------------------------------------------

// RememberJSON implements the cache-aside (read-through) pattern for JSON values.
// If key exists and is valid JSON for T, returns the cached value. Otherwise,
// calls fn to produce the value, stores it under key with ttl, and returns it.
func RememberJSON[T any](ctx context.Context, c *Client, key string, ttl time.Duration, fn func(ctx context.Context) (T, error)) (T, error) {
	var dst T
	if err := c.GetJSON(ctx, key, &dst); err == nil {
		return dst, nil
	}
	val, err := fn(ctx)
	if err != nil {
		return val, err
	}
	_ = c.SetJSON(ctx, key, val, ttl)
	return val, nil
}

// ---- Counter helpers ----------------------------------------------------------

// Incr atomically increments the integer counter at key and returns the new value.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.c.Incr(ctx, key)
}

// IncrBy atomically increments the integer counter at key by delta.
func (c *Client) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return c.c.IncrBy(ctx, key, delta)
}

// SetNX sets key to value only if the key does not exist. Returns true if set.
func (c *Client) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return c.c.SetNX(ctx, key, value, ttl)
}

// ---- Distributed lock helpers -------------------------------------------------

// WithLock acquires a distributed lock on key, runs fn, then releases the lock.
// token must be unique per caller (e.g. uuid.NewString()).
// Returns ErrLockNotAcquired if the lock is already held.
func (c *Client) WithLock(ctx context.Context, key, token string, ttl time.Duration, fn func() error) error {
	lock, err := c.c.AcquireLock(ctx, key, token, ttl)
	if err != nil {
		return err
	}
	defer lock.Release(ctx) //nolint:errcheck
	return fn()
}

// ---- Health ------------------------------------------------------------------

// Ping verifies Redis connectivity.
func (c *Client) Ping(ctx context.Context) error { return c.c.Ping(ctx) }
