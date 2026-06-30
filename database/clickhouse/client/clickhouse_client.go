// Package clickhouseclient provides a production-ready ClickHouse client for the
// TikTok clone analytics platform. It wraps clickhouse-go/v2 with:
//   - Connection pooling and health checks
//   - Type-safe batch insertion helpers for every analytics table
//   - Retry logic with exponential back-off
//   - Structured logging via slog
//   - Context-aware cancellation propagation
package clickhouseclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// Config holds all runtime options for the ClickHouse client.
type Config struct {
	// Addresses is a list of "host:port" strings (native protocol, default 9000).
	Addresses []string

	Database string
	Username string
	Password string

	// DialTimeout is the maximum time to wait for a TCP connection.
	DialTimeout time.Duration

	// MaxOpenConns is the maximum number of connections in the pool.
	MaxOpenConns int

	// MaxIdleConns is the number of idle connections kept open.
	MaxIdleConns int

	// ConnMaxLifetime is how long a pooled connection may be reused.
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is how long an idle connection stays open before being closed.
	ConnMaxIdleTime time.Duration

	// BatchFlushInterval is the maximum time before a partial batch is flushed.
	BatchFlushInterval time.Duration

	// BatchSize is the maximum rows to accumulate before sending to ClickHouse.
	BatchSize int

	// TLSEnabled activates TLS for the native TCP connection.
	TLSEnabled bool

	// Debug enables query-level debug logging from the driver.
	Debug bool

	// MaxRetries controls how many times a failed write is retried.
	MaxRetries int

	// RetryBaseDelay is the initial delay for exponential back-off.
	RetryBaseDelay time.Duration
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		Addresses:          []string{"localhost:9000"},
		Database:           "tiktok",
		Username:           "default",
		Password:           "",
		DialTimeout:        10 * time.Second,
		MaxOpenConns:       20,
		MaxIdleConns:       5,
		ConnMaxLifetime:    10 * time.Minute,
		ConnMaxIdleTime:    5 * time.Minute,
		BatchFlushInterval: 5 * time.Second,
		BatchSize:          10_000,
		MaxRetries:         3,
		RetryBaseDelay:     200 * time.Millisecond,
	}
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client is the primary handle for all ClickHouse interactions.
type Client struct {
	conn   driver.Conn
	cfg    Config
	logger *slog.Logger
}

// New creates and validates a new ClickHouse Client.
func New(cfg Config, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	opts := &clickhouse.Options{
		Addr: cfg.Addresses,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout:      cfg.DialTimeout,
		MaxOpenConns:     cfg.MaxOpenConns,
		MaxIdleConns:     cfg.MaxIdleConns,
		ConnMaxLifetime:  cfg.ConnMaxLifetime,
		ConnMaxIdleTime:  cfg.ConnMaxIdleTime,
		Debug:            cfg.Debug,
		Debugf:           func(format string, v ...any) { logger.Debug(fmt.Sprintf(format, v...)) },
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Settings: clickhouse.Settings{
			// Async inserts: ClickHouse batches on the server side (optional, disable
			// if you prefer client-side batching only).
			"async_insert":          0,
			"wait_for_async_insert": 1,
		},
	}

	if cfg.TLSEnabled {
		opts.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse.Open: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	logger.Info("ClickHouse connection established",
		slog.Any("addresses", cfg.Addresses),
		slog.String("database", cfg.Database),
	)

	return &Client{conn: conn, cfg: cfg, logger: logger}, nil
}

// Close releases all connections in the pool.
func (c *Client) Close() error { return c.conn.Close() }

// Ping checks that the server is reachable.
func (c *Client) Ping(ctx context.Context) error { return c.conn.Ping(ctx) }

// ---------------------------------------------------------------------------
// Retry helper
// ---------------------------------------------------------------------------

// withRetry executes fn up to cfg.MaxRetries times with exponential back-off.
// It does NOT retry context cancellations or deadline exceeded errors.
func (c *Client) withRetry(ctx context.Context, op string, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential back-off with ±20% jitter
			delay := float64(c.cfg.RetryBaseDelay) * math.Pow(2, float64(attempt-1))
			jitter := delay * 0.2 * (rand.Float64()*2 - 1) //nolint:gosec // jitter only
			sleepFor := time.Duration(delay + jitter)
			c.logger.Warn("retrying ClickHouse operation",
				slog.String("op", op),
				slog.Int("attempt", attempt),
				slog.Duration("delay", sleepFor),
				slog.String("error", lastErr.Error()),
			)
			select {
			case <-ctx.Done():
				return fmt.Errorf("%s: context cancelled during retry: %w", op, ctx.Err())
			case <-time.After(sleepFor):
			}
		}

		if err := fn(); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("%s: %w", op, err)
			}
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("%s: all %d retries exhausted: %w", op, c.cfg.MaxRetries, lastErr)
}

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// VideoView represents a single video view event.
type VideoView struct {
	UserID          uint64
	VideoID         uint64
	CreatorID       uint64
	WatchDurationMs uint32
	WatchPercentage float32
	Source          string // fyp | following | search | profile | share | hashtag | sound
	DeviceType      string // ios | android | web | tablet
	Country         string // ISO-3166-1 alpha-2
	Region          string
	NetworkType     string
	SessionID       uuid.UUID
	IsAutoplay      uint8
	IsMuted         uint8
	Replays         uint8
	Timestamp       time.Time
}

// EngagementEvent represents a user action (like, comment, share, etc.).
type EngagementEvent struct {
	EventID       uuid.UUID
	UserID        uint64
	EventType     string // like | comment | share | follow | bookmark | duet | stitch | report
	IsUndo        uint8
	VideoID       uint64
	CommentID     uint64
	SoundID       uint64
	UserIDTarget  uint64
	HashtagID     uint64
	CreatorID     uint64
	SharePlatform string
	CommentText   string
	CommentLang   string
	DeviceType    string
	Country       string
	Region        string
	SessionID     uuid.UUID
	Timestamp     time.Time
}

// LiveSession represents a completed or in-progress live stream.
type LiveSession struct {
	RoomID              uint64
	CreatorID           uint64
	SessionID           uuid.UUID
	Status              string
	ViewerCount         uint64
	PeakViewers         uint64
	TotalUniqueViewers  uint64
	AvgConcurrentViewers float32
	GiftsReceived       uint64
	CoinsEarned         uint64
	USDEarned           float64
	TopGifterUserID     uint64
	TopGifterCoins      uint64
	Title               string
	Category            string
	Tags                []string
	StartedAt           time.Time
	EndedAt             time.Time
	Country             string
	DeviceType          string
	WarningCount        uint8
	IsAgeRestricted     uint8
}

// RevenueTransaction represents a single monetary event.
type RevenueTransaction struct {
	TransactionID   uuid.UUID
	IdempotencyKey  string
	CreatorID       uint64
	UserID          uint64
	TransactionType string
	Status          string
	Amount          float64
	Currency        string
	AmountUSD       float64
	ExchangeRate    float64
	PlatformFeePct  float64
	PlatformFee     float64
	NetAmount       float64
	NetAmountUSD    float64
	TaxAmount       float64
	TaxRegion       string
	Processor       string
	ProcessorTxnID  string
	SourceVideoID   uint64
	SourceRoomID    uint64
	SourceGiftID    uint32
	SourceAdID      uint64
	Country         string
	Timestamp       time.Time
	SettledAt       time.Time
}

// AdImpression represents a delivered ad impression event.
type AdImpression struct {
	ImpressionID     uuid.UUID
	AdID             uint64
	AdSetID          uint64
	CampaignID       uint64
	AdvertiserID     uint64
	UserID           uint64
	DeviceID         string
	AdFormat         string
	Placement        string
	BidType          string
	BidAmountUSD     float64
	ClearingPriceUSD float64
	IsWon            uint8
	ViewDurationMs   uint32
	IsViewable       uint8
	IsSkipped        uint8
	SkippedAtMs      uint32
	Country          string
	Region           string
	DeviceType       string
	OS               string
	AudienceSegment  string
	IsRetargeting    uint8
	Timestamp        time.Time
}

// ---------------------------------------------------------------------------
// Batch inserters
// ---------------------------------------------------------------------------

// InsertVideoViews writes a batch of VideoView events using a single prepared
// batch to minimise round-trips and serialisation overhead.
func (c *Client) InsertVideoViews(ctx context.Context, rows []VideoView) error {
	if len(rows) == 0 {
		return nil
	}
	return c.withRetry(ctx, "InsertVideoViews", func() error {
		batch, err := c.conn.PrepareBatch(ctx,
			"INSERT INTO tiktok.video_views "+
				"(user_id, video_id, creator_id, watch_duration_ms, watch_percentage, "+
				" source, device_type, country, region, network_type, session_id, "+
				" is_autoplay, is_muted, replays, timestamp)",
		)
		if err != nil {
			return fmt.Errorf("PrepareBatch: %w", err)
		}

		for i := range rows {
			r := &rows[i]
			sid := r.SessionID
			if sid == uuid.Nil {
				sid = uuid.New()
			}
			if err := batch.Append(
				r.UserID,
				r.VideoID,
				r.CreatorID,
				r.WatchDurationMs,
				r.WatchPercentage,
				r.Source,
				r.DeviceType,
				r.Country,
				r.Region,
				r.NetworkType,
				sid,
				r.IsAutoplay,
				r.IsMuted,
				r.Replays,
				r.Timestamp,
			); err != nil {
				return fmt.Errorf("batch.Append row %d: %w", i, err)
			}
		}

		if err := batch.Send(); err != nil {
			return fmt.Errorf("batch.Send: %w", err)
		}
		c.logger.Debug("InsertVideoViews: batch sent",
			slog.Int("rows", len(rows)),
		)
		return nil
	})
}

// InsertEngagementEvents writes a batch of EngagementEvent rows.
func (c *Client) InsertEngagementEvents(ctx context.Context, rows []EngagementEvent) error {
	if len(rows) == 0 {
		return nil
	}
	return c.withRetry(ctx, "InsertEngagementEvents", func() error {
		batch, err := c.conn.PrepareBatch(ctx,
			"INSERT INTO tiktok.engagement_events "+
				"(event_id, user_id, event_type, is_undo, video_id, comment_id, sound_id, "+
				" user_id_target, hashtag_id, creator_id, share_platform, comment_text, "+
				" comment_lang, device_type, country, region, session_id, timestamp)",
		)
		if err != nil {
			return fmt.Errorf("PrepareBatch: %w", err)
		}

		for i := range rows {
			r := &rows[i]
			eid := r.EventID
			if eid == uuid.Nil {
				eid = uuid.New()
			}
			sid := r.SessionID
			if sid == uuid.Nil {
				sid = uuid.New()
			}
			// Truncate comment text to 500 chars
			commentText := r.CommentText
			if len(commentText) > 500 {
				commentText = commentText[:500]
			}
			if err := batch.Append(
				eid,
				r.UserID,
				r.EventType,
				r.IsUndo,
				r.VideoID,
				r.CommentID,
				r.SoundID,
				r.UserIDTarget,
				r.HashtagID,
				r.CreatorID,
				r.SharePlatform,
				commentText,
				r.CommentLang,
				r.DeviceType,
				r.Country,
				r.Region,
				sid,
				r.Timestamp,
			); err != nil {
				return fmt.Errorf("batch.Append row %d: %w", i, err)
			}
		}

		return batch.Send()
	})
}

// UpsertLiveSession writes (or replaces) a live session row.
// live_sessions uses ReplacingMergeTree so the latest write per room_id wins
// after background merges; use FINAL in reads to get the current state.
func (c *Client) UpsertLiveSession(ctx context.Context, s LiveSession) error {
	return c.withRetry(ctx, "UpsertLiveSession", func() error {
		batch, err := c.conn.PrepareBatch(ctx,
			"INSERT INTO tiktok.live_sessions "+
				"(room_id, creator_id, session_id, status, viewer_count, peak_viewers, "+
				" total_unique_viewers, avg_concurrent_viewers, gifts_received, coins_earned, "+
				" usd_earned, top_gifter_user_id, top_gifter_coins, title, category, tags, "+
				" started_at, ended_at, country, device_type, warning_count, is_age_restricted)",
		)
		if err != nil {
			return fmt.Errorf("PrepareBatch: %w", err)
		}
		sid := s.SessionID
		if sid == uuid.Nil {
			sid = uuid.New()
		}
		if s.Tags == nil {
			s.Tags = []string{}
		}
		if err := batch.Append(
			s.RoomID,
			s.CreatorID,
			sid,
			s.Status,
			s.ViewerCount,
			s.PeakViewers,
			s.TotalUniqueViewers,
			s.AvgConcurrentViewers,
			s.GiftsReceived,
			s.CoinsEarned,
			s.USDEarned,
			s.TopGifterUserID,
			s.TopGifterCoins,
			s.Title,
			s.Category,
			s.Tags,
			s.StartedAt,
			s.EndedAt,
			s.Country,
			s.DeviceType,
			s.WarningCount,
			s.IsAgeRestricted,
		); err != nil {
			return fmt.Errorf("batch.Append: %w", err)
		}
		return batch.Send()
	})
}

// InsertRevenueTransactions writes a batch of RevenueTransaction rows.
func (c *Client) InsertRevenueTransactions(ctx context.Context, rows []RevenueTransaction) error {
	if len(rows) == 0 {
		return nil
	}
	return c.withRetry(ctx, "InsertRevenueTransactions", func() error {
		batch, err := c.conn.PrepareBatch(ctx,
			"INSERT INTO tiktok.revenue_transactions "+
				"(transaction_id, idempotency_key, creator_id, user_id, transaction_type, status, "+
				" amount, currency, amount_usd, exchange_rate, platform_fee_pct, platform_fee, "+
				" net_amount, net_amount_usd, tax_amount, tax_region, processor, processor_txn_id, "+
				" source_video_id, source_room_id, source_gift_id, source_ad_id, country, timestamp, settled_at)",
		)
		if err != nil {
			return fmt.Errorf("PrepareBatch: %w", err)
		}

		zeroTime := time.Unix(0, 0).UTC()
		for i := range rows {
			r := &rows[i]
			tid := r.TransactionID
			if tid == uuid.Nil {
				tid = uuid.New()
			}
			settledAt := r.SettledAt
			if settledAt.IsZero() {
				settledAt = zeroTime
			}
			if err := batch.Append(
				tid,
				r.IdempotencyKey,
				r.CreatorID,
				r.UserID,
				r.TransactionType,
				r.Status,
				r.Amount,
				r.Currency,
				r.AmountUSD,
				r.ExchangeRate,
				r.PlatformFeePct,
				r.PlatformFee,
				r.NetAmount,
				r.NetAmountUSD,
				r.TaxAmount,
				r.TaxRegion,
				r.Processor,
				r.ProcessorTxnID,
				r.SourceVideoID,
				r.SourceRoomID,
				r.SourceGiftID,
				r.SourceAdID,
				r.Country,
				r.Timestamp,
				settledAt,
			); err != nil {
				return fmt.Errorf("batch.Append row %d: %w", i, err)
			}
		}
		return batch.Send()
	})
}

// InsertAdImpressions writes a batch of AdImpression events.
func (c *Client) InsertAdImpressions(ctx context.Context, rows []AdImpression) error {
	if len(rows) == 0 {
		return nil
	}
	return c.withRetry(ctx, "InsertAdImpressions", func() error {
		batch, err := c.conn.PrepareBatch(ctx,
			"INSERT INTO tiktok.ad_impressions "+
				"(impression_id, ad_id, ad_set_id, campaign_id, advertiser_id, user_id, device_id, "+
				" ad_format, placement, bid_type, bid_amount_usd, clearing_price_usd, is_won, "+
				" view_duration_ms, is_viewable, is_skipped, skipped_at_ms, country, region, "+
				" device_type, os, audience_segment, is_retargeting, timestamp)",
		)
		if err != nil {
			return fmt.Errorf("PrepareBatch: %w", err)
		}

		for i := range rows {
			r := &rows[i]
			iid := r.ImpressionID
			if iid == uuid.Nil {
				iid = uuid.New()
			}
			if err := batch.Append(
				iid,
				r.AdID,
				r.AdSetID,
				r.CampaignID,
				r.AdvertiserID,
				r.UserID,
				r.DeviceID,
				r.AdFormat,
				r.Placement,
				r.BidType,
				r.BidAmountUSD,
				r.ClearingPriceUSD,
				r.IsWon,
				r.ViewDurationMs,
				r.IsViewable,
				r.IsSkipped,
				r.SkippedAtMs,
				r.Country,
				r.Region,
				r.DeviceType,
				r.OS,
				r.AudienceSegment,
				r.IsRetargeting,
				r.Timestamp,
			); err != nil {
				return fmt.Errorf("batch.Append row %d: %w", i, err)
			}
		}
		return batch.Send()
	})
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// QueryRow executes a SELECT that returns exactly one row and scans it into
// dest. Use for scalar / single-row dashboard queries.
func (c *Client) QueryRow(ctx context.Context, query string, args []any, dest ...any) error {
	row := c.conn.QueryRow(ctx, query, args...)
	if err := row.Scan(dest...); err != nil {
		return fmt.Errorf("QueryRow scan: %w", err)
	}
	return nil
}

// Query executes a SELECT and invokes fn for each result row.
// fn receives a driver.Rows; call rows.Scan inside fn.
func (c *Client) Query(ctx context.Context, query string, args []any, fn func(driver.Rows) error) error {
	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("conn.Query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Exec runs a DDL or non-SELECT statement.
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	return c.conn.Exec(ctx, query, args...)
}

// ---------------------------------------------------------------------------
// Async batch collector
// ---------------------------------------------------------------------------

// ViewBatcher accumulates VideoView events and flushes them in configurable
// batches. Safe for concurrent use. Start it with Run and stop with cancel.
type ViewBatcher struct {
	client *Client
	ch     chan VideoView
	cfg    Config
	logger *slog.Logger
}

// NewViewBatcher creates a ViewBatcher with an internal channel sized to
// 2x BatchSize for burst tolerance.
func NewViewBatcher(client *Client) *ViewBatcher {
	return &ViewBatcher{
		client: client,
		ch:     make(chan VideoView, client.cfg.BatchSize*2),
		cfg:    client.cfg,
		logger: client.logger,
	}
}

// Enqueue adds a single VideoView to the internal buffer.
// Returns an error if the channel is full (backpressure signal).
func (b *ViewBatcher) Enqueue(v VideoView) error {
	select {
	case b.ch <- v:
		return nil
	default:
		return errors.New("ViewBatcher: channel full — shedding load")
	}
}

// Run drains the channel, flushing when BatchSize is reached or
// BatchFlushInterval elapses. Call in a dedicated goroutine; stop by
// cancelling ctx.
func (b *ViewBatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(b.cfg.BatchFlushInterval)
	defer ticker.Stop()

	buf := make([]VideoView, 0, b.cfg.BatchSize)

	flush := func() {
		if len(buf) == 0 {
			return
		}
		if err := b.client.InsertVideoViews(ctx, buf); err != nil {
			b.logger.Error("ViewBatcher flush failed", slog.String("error", err.Error()))
		}
		buf = buf[:0]
	}

	for {
		select {
		case v, ok := <-b.ch:
			if !ok {
				flush()
				return
			}
			buf = append(buf, v)
			if len(buf) >= b.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			// Drain remaining items
			for {
				select {
				case v := <-b.ch:
					buf = append(buf, v)
				default:
					flush()
					return
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Health check
// ---------------------------------------------------------------------------

// HealthCheck returns a map of component → status string for readiness probes.
func (c *Client) HealthCheck(ctx context.Context) map[string]string {
	result := map[string]string{"clickhouse": "ok"}
	if err := c.conn.Ping(ctx); err != nil {
		result["clickhouse"] = fmt.Sprintf("error: %v", err)
	}
	return result
}
