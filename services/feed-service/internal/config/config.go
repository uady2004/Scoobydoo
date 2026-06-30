// Package config holds all configuration for the feed-service loaded from
// environment variables. Sensible defaults are applied where possible so that
// the service can start with minimal environment setup during local development.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the root configuration object for the feed-service.
type Config struct {
	Server           ServerConfig
	Database         DatabaseConfig
	Redis            RedisConfig
	RecommendService RecommendServiceConfig
	SocialGraph      SocialGraphConfig
	Feed             FeedConfig
	Kafka            KafkaConfig
	Observability    ObservabilityConfig
}

// ServerConfig controls HTTP listener settings.
type ServerConfig struct {
	// Host is the interface to bind to (default "0.0.0.0").
	Host string
	// Port is the HTTP port (default 8084).
	Port int
	// ReadTimeout is the maximum duration for reading the full request.
	ReadTimeout time.Duration
	// WriteTimeout is the maximum duration before timing out writes.
	WriteTimeout time.Duration
	// IdleTimeout is the maximum amount of time to wait for the next request.
	IdleTimeout time.Duration
	// ShutdownTimeout is how long we wait for in-flight requests on SIGTERM.
	ShutdownTimeout time.Duration
}

// DatabaseConfig holds PostgreSQL/PostGIS connection settings.
type DatabaseConfig struct {
	// DSN is a libpq-compatible connection string.
	DSN               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	// Addr is the host:port for standalone mode.
	Addr string
	// Addrs is used when connecting to a Redis cluster (comma-separated).
	Addrs    []string
	Password string
	DB       int
	PoolSize int
}

// RecommendServiceConfig holds gRPC endpoint settings for the recommendation
// service which provides personalised For-You feeds.
type RecommendServiceConfig struct {
	// GRPCAddr is the host:port of the recommendation gRPC server.
	GRPCAddr string
	// Timeout is the per-call deadline.
	Timeout time.Duration
}

// SocialGraphConfig holds gRPC endpoint settings for the social-graph service
// used to resolve follower/following relationships.
type SocialGraphConfig struct {
	GRPCAddr string
	Timeout  time.Duration
}

// FeedConfig holds feed-specific behavioural knobs.
type FeedConfig struct {
	// DefaultPageSize is the number of items returned when the caller omits the
	// size parameter (default 20).
	DefaultPageSize int
	// MaxPageSize caps a single page (default 50).
	MaxPageSize int
	// ForYouCacheTTL is how long a pre-computed For-You feed lives in Redis.
	ForYouCacheTTL time.Duration
	// FollowingCacheTTL is how long a following feed is cached.
	FollowingCacheTTL time.Duration
	// TrendingCacheTTL is how long a trending feed entry lives.
	TrendingCacheTTL time.Duration
	// NearbyCacheTTL is how long nearby feed results are cached.
	NearbyCacheTTL time.Duration
	// ExploreCacheTTL is how long an explore feed page is cached.
	ExploreCacheTTL time.Duration
	// DeduplicationTTL is how long the seen-video set lives per user session.
	DeduplicationTTL time.Duration
	// TrendingWindowHours is the look-back window used by the trending scorer.
	TrendingWindowHours int
	// PrecomputeBatchSize is the number of users processed per precompute tick.
	PrecomputeBatchSize int
	// PrecomputeInterval is how frequently the precompute worker runs.
	PrecomputeInterval time.Duration
	// TrendingUpdateInterval is how frequently trending scores are recalculated.
	TrendingUpdateInterval time.Duration
	// NearbyDefaultRadiusKm is the default search radius for nearby feeds.
	NearbyDefaultRadiusKm float64
	// MaxNearbyRadiusKm is the maximum allowable nearby radius.
	MaxNearbyRadiusKm float64
}

// KafkaConfig holds Kafka broker settings used for consuming video-event
// streams that feed the trending scorer.
type KafkaConfig struct {
	Brokers []string
	// VideoEventsTopic is the topic producing VideoViewed / VideoLiked events.
	VideoEventsTopic string
	// ConsumerGroup is the Kafka consumer group ID for this service instance.
	ConsumerGroup string
}

// ObservabilityConfig controls tracing and metrics export.
type ObservabilityConfig struct {
	// OTLPEndpoint is the host:port of the OTLP gRPC collector.
	OTLPEndpoint string
	// ServiceName is injected into every trace/metric.
	ServiceName string
	// ServiceVersion is injected into every trace/metric.
	ServiceVersion string
}

// Load reads configuration from environment variables, applying defaults for
// any variable that is unset or empty.
func Load() (*Config, error) {
	cfg := &Config{}

	// ---- Server ----------------------------------------------------------------
	cfg.Server.Host = envStr("SERVER_HOST", "0.0.0.0")
	port, err := envInt("SERVER_PORT", 8084)
	if err != nil {
		return nil, fmt.Errorf("config: SERVER_PORT: %w", err)
	}
	cfg.Server.Port = port
	cfg.Server.ReadTimeout = envDuration("SERVER_READ_TIMEOUT", 15*time.Second)
	cfg.Server.WriteTimeout = envDuration("SERVER_WRITE_TIMEOUT", 30*time.Second)
	cfg.Server.IdleTimeout = envDuration("SERVER_IDLE_TIMEOUT", 120*time.Second)
	cfg.Server.ShutdownTimeout = envDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second)

	// ---- Database --------------------------------------------------------------
	cfg.Database.DSN = envStr("DATABASE_DSN",
		"postgres://feed:feed@localhost:5432/feeddb?sslmode=disable")
	maxConns, err := envInt32("DB_MAX_CONNS", 25)
	if err != nil {
		return nil, fmt.Errorf("config: DB_MAX_CONNS: %w", err)
	}
	cfg.Database.MaxConns = maxConns
	minConns, err := envInt32("DB_MIN_CONNS", 2)
	if err != nil {
		return nil, fmt.Errorf("config: DB_MIN_CONNS: %w", err)
	}
	cfg.Database.MinConns = minConns
	cfg.Database.MaxConnLifetime = envDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute)
	cfg.Database.MaxConnIdleTime = envDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute)
	cfg.Database.HealthCheckPeriod = envDuration("DB_HEALTH_CHECK_PERIOD", 1*time.Minute)

	// ---- Redis -----------------------------------------------------------------
	cfg.Redis.Addr = envStr("REDIS_ADDR", "localhost:6379")
	rawAddrs := envStr("REDIS_ADDRS", "")
	if rawAddrs != "" {
		cfg.Redis.Addrs = strings.Split(rawAddrs, ",")
	}
	cfg.Redis.Password = envStr("REDIS_PASSWORD", "")
	redisDB, err := envInt("REDIS_DB", 0)
	if err != nil {
		return nil, fmt.Errorf("config: REDIS_DB: %w", err)
	}
	cfg.Redis.DB = redisDB
	redisPool, err := envInt("REDIS_POOL_SIZE", 20)
	if err != nil {
		return nil, fmt.Errorf("config: REDIS_POOL_SIZE: %w", err)
	}
	cfg.Redis.PoolSize = redisPool

	// ---- Recommendation service ------------------------------------------------
	cfg.RecommendService.GRPCAddr = envStr("RECOMMEND_SERVICE_GRPC_ADDR", "localhost:9093")
	cfg.RecommendService.Timeout = envDuration("RECOMMEND_SERVICE_TIMEOUT", 500*time.Millisecond)

	// ---- Social-graph service --------------------------------------------------
	cfg.SocialGraph.GRPCAddr = envStr("SOCIAL_GRAPH_GRPC_ADDR", "localhost:9095")
	cfg.SocialGraph.Timeout = envDuration("SOCIAL_GRAPH_TIMEOUT", 500*time.Millisecond)

	// ---- Feed knobs ------------------------------------------------------------
	defaultPageSize, err := envInt("FEED_DEFAULT_PAGE_SIZE", 20)
	if err != nil {
		return nil, fmt.Errorf("config: FEED_DEFAULT_PAGE_SIZE: %w", err)
	}
	cfg.Feed.DefaultPageSize = defaultPageSize
	maxPageSize, err := envInt("FEED_MAX_PAGE_SIZE", 50)
	if err != nil {
		return nil, fmt.Errorf("config: FEED_MAX_PAGE_SIZE: %w", err)
	}
	cfg.Feed.MaxPageSize = maxPageSize
	cfg.Feed.ForYouCacheTTL = envDuration("FEED_FORYOU_CACHE_TTL", 10*time.Minute)
	cfg.Feed.FollowingCacheTTL = envDuration("FEED_FOLLOWING_CACHE_TTL", 5*time.Minute)
	cfg.Feed.TrendingCacheTTL = envDuration("FEED_TRENDING_CACHE_TTL", 15*time.Minute)
	cfg.Feed.NearbyCacheTTL = envDuration("FEED_NEARBY_CACHE_TTL", 5*time.Minute)
	cfg.Feed.ExploreCacheTTL = envDuration("FEED_EXPLORE_CACHE_TTL", 10*time.Minute)
	cfg.Feed.DeduplicationTTL = envDuration("FEED_DEDUP_TTL", 24*time.Hour)
	trendingWindow, err := envInt("FEED_TRENDING_WINDOW_HOURS", 24)
	if err != nil {
		return nil, fmt.Errorf("config: FEED_TRENDING_WINDOW_HOURS: %w", err)
	}
	cfg.Feed.TrendingWindowHours = trendingWindow
	batchSize, err := envInt("FEED_PRECOMPUTE_BATCH_SIZE", 500)
	if err != nil {
		return nil, fmt.Errorf("config: FEED_PRECOMPUTE_BATCH_SIZE: %w", err)
	}
	cfg.Feed.PrecomputeBatchSize = batchSize
	cfg.Feed.PrecomputeInterval = envDuration("FEED_PRECOMPUTE_INTERVAL", 5*time.Minute)
	cfg.Feed.TrendingUpdateInterval = envDuration("FEED_TRENDING_UPDATE_INTERVAL", 1*time.Hour)
	nearbyDefault, err := envFloat("FEED_NEARBY_DEFAULT_RADIUS_KM", 10.0)
	if err != nil {
		return nil, fmt.Errorf("config: FEED_NEARBY_DEFAULT_RADIUS_KM: %w", err)
	}
	cfg.Feed.NearbyDefaultRadiusKm = nearbyDefault
	nearbyMax, err := envFloat("FEED_NEARBY_MAX_RADIUS_KM", 100.0)
	if err != nil {
		return nil, fmt.Errorf("config: FEED_NEARBY_MAX_RADIUS_KM: %w", err)
	}
	cfg.Feed.MaxNearbyRadiusKm = nearbyMax

	// ---- Kafka -----------------------------------------------------------------
	rawBrokers := envStr("KAFKA_BROKERS", "localhost:9092")
	cfg.Kafka.Brokers = strings.Split(rawBrokers, ",")
	cfg.Kafka.VideoEventsTopic = envStr("KAFKA_VIDEO_EVENTS_TOPIC", "video-events")
	cfg.Kafka.ConsumerGroup = envStr("KAFKA_CONSUMER_GROUP", "feed-service")

	// ---- Observability ---------------------------------------------------------
	cfg.Observability.OTLPEndpoint = envStr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	cfg.Observability.ServiceName = envStr("SERVICE_NAME", "feed-service")
	cfg.Observability.ServiceVersion = envStr("SERVICE_VERSION", "1.0.0")

	return cfg, nil
}

// Addr returns the full listen address for the HTTP server.
func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// ---- helpers ----------------------------------------------------------------

func envStr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envInt(key string, defaultVal int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("must be an integer, got %q", v)
	}
	return n, nil
}

func envInt32(key string, defaultVal int32) (int32, error) {
	n, err := envInt(key, int(defaultVal))
	return int32(n), err
}

func envFloat(key string, defaultVal float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("must be a float, got %q", v)
	}
	return f, nil
}

func envDuration(key string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultVal
	}
	return d
}
