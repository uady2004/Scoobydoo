// Package database provides a pgxpool connection pool with health checks and
// database migration support via embedded SQL files.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---- Configuration ----------------------------------------------------------

// Config holds PostgreSQL connection pool settings.
type Config struct {
	// DSN is a libpq-style connection string, e.g.:
	//   postgres://user:pass@host:5432/dbname?sslmode=require
	DSN string

	// Pool tuning.
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration

	// ConnectTimeout is the timeout for a single connection attempt.
	ConnectTimeout time.Duration

	// ConnectRetries is the number of times to retry the initial pool creation
	// if the database is not yet reachable. Useful in docker-compose startups.
	ConnectRetries int
	// ConnectRetryDelay is the wait between retries.
	ConnectRetryDelay time.Duration
}

func (c *Config) defaults() {
	if c.MaxConns == 0 {
		c.MaxConns = 25
	}
	if c.MinConns == 0 {
		c.MinConns = 2
	}
	if c.MaxConnLifetime == 0 {
		c.MaxConnLifetime = 30 * time.Minute
	}
	if c.MaxConnIdleTime == 0 {
		c.MaxConnIdleTime = 5 * time.Minute
	}
	if c.HealthCheckPeriod == 0 {
		c.HealthCheckPeriod = 1 * time.Minute
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 10 * time.Second
	}
	if c.ConnectRetries == 0 {
		c.ConnectRetries = 5
	}
	if c.ConnectRetryDelay == 0 {
		c.ConnectRetryDelay = 2 * time.Second
	}
}

// ---- DB wrapper -------------------------------------------------------------

// DB wraps *pgxpool.Pool and exposes convenience helpers.
type DB struct {
	pool *pgxpool.Pool
	cfg  Config
}

// New creates a DB, retrying the connection up to cfg.ConnectRetries times.
func New(ctx context.Context, cfg Config) (*DB, error) {
	cfg.defaults()

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("database: parsing DSN: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	// Trace hook: log slow queries (> 200 ms) to stderr.
	poolCfg.ConnConfig.Tracer = &slowQueryTracer{threshold: 200 * time.Millisecond}

	var pool *pgxpool.Pool
	for attempt := 1; attempt <= cfg.ConnectRetries; attempt++ {
		connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout*2)
		pool, err = pgxpool.NewWithConfig(connectCtx, poolCfg)
		cancel()
		if err != nil {
			if attempt < cfg.ConnectRetries {
				time.Sleep(cfg.ConnectRetryDelay)
				continue
			}
			return nil, fmt.Errorf("database: creating pool after %d attempts: %w", attempt, err)
		}

		// Verify we can actually talk to the server.
		pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
		pingErr := pool.Ping(pingCtx)
		cancel()
		if pingErr == nil {
			break
		}
		pool.Close()
		pool = nil
		err = pingErr
		if attempt < cfg.ConnectRetries {
			time.Sleep(cfg.ConnectRetryDelay)
		}
	}
	if pool == nil {
		return nil, fmt.Errorf("database: ping failed after %d attempts: %w", cfg.ConnectRetries, err)
	}

	return &DB{pool: pool, cfg: cfg}, nil
}

// Pool returns the underlying *pgxpool.Pool for direct use.
func (db *DB) Pool() *pgxpool.Pool { return db.pool }

// Close shuts down all idle connections in the pool.
func (db *DB) Close() { db.pool.Close() }

// Ping verifies that at least one connection is alive.
func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// ---- Query helpers ----------------------------------------------------------

// QueryRow executes a query expected to return at most one row.
func (db *DB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

// Query executes a query returning multiple rows.
func (db *DB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

// Exec executes a statement that does not return rows.
func (db *DB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return db.pool.Exec(ctx, sql, args...)
}

// ---- Transaction helpers ----------------------------------------------------

// TxFunc is a function executed inside a transaction.
type TxFunc func(ctx context.Context, tx pgx.Tx) error

// WithTx acquires a connection from the pool, begins a transaction, calls fn,
// and commits or rolls back depending on whether fn returns an error.
func (db *DB) WithTx(ctx context.Context, fn TxFunc) error {
	return db.WithTxOptions(ctx, pgx.TxOptions{}, fn)
}

// WithTxOptions is like WithTx but allows specifying transaction isolation level
// and access mode.
func (db *DB) WithTxOptions(ctx context.Context, opts pgx.TxOptions, fn TxFunc) error {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("database: acquiring connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("database: beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return fmt.Errorf("database: rollback failed (%v) after error: %w", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("database: committing transaction: %w", err)
	}
	return nil
}

// ---- Migration --------------------------------------------------------------

// MigrationSource provides ordered SQL migration scripts.
type MigrationSource interface {
	// Steps returns a slice of (id, upSQL) pairs in ascending order.
	Steps() []MigrationStep
}

// MigrationStep is a single migration.
type MigrationStep struct {
	ID   int    // monotonically increasing integer
	Name string // human-readable label
	Up   string // SQL to apply
	Down string // SQL to reverse (optional)
}

// Migrate applies all pending migrations from source.
// It creates the schema_migrations table if it does not exist.
func (db *DB) Migrate(ctx context.Context, source MigrationSource) error {
	// Ensure the migrations table exists.
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id         INTEGER PRIMARY KEY,
			name       TEXT    NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return fmt.Errorf("database: creating schema_migrations table: %w", err)
	}

	for _, step := range source.Steps() {
		var exists bool
		err := db.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE id = $1)`, step.ID,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("database: checking migration %d: %w", step.ID, err)
		}
		if exists {
			continue
		}

		if err := db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, step.Up); err != nil {
				return fmt.Errorf("applying migration %d (%s): %w", step.ID, step.Name, err)
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO schema_migrations (id, name) VALUES ($1, $2)`,
				step.ID, step.Name,
			); err != nil {
				return fmt.Errorf("recording migration %d: %w", step.ID, err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("database: migration %d: %w", step.ID, err)
		}
	}
	return nil
}

// ---- Pool stats -------------------------------------------------------------

// Stats returns current pool statistics.
func (db *DB) Stats() *pgxpool.Stat {
	return db.pool.Stat()
}

// ---- Slow query tracer ------------------------------------------------------

// slowQueryTracer logs queries that exceed the threshold to stderr via the
// standard library logger. Replace with your structured logger in production.
type slowQueryTracer struct {
	threshold time.Duration
}

type traceStartKey struct{}

type traceStartVal struct {
	start time.Time
	sql   string
}

func (t *slowQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, traceStartKey{}, traceStartVal{start: time.Now(), sql: data.SQL})
}

func (t *slowQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	val, ok := ctx.Value(traceStartKey{}).(traceStartVal)
	if !ok {
		return
	}
	elapsed := time.Since(val.start)
	if elapsed >= t.threshold {
		fmt.Printf("[SLOW QUERY] %s (%s)\n", val.sql, elapsed)
	}
}
