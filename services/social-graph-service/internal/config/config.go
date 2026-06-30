package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the social-graph-service.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	JWT      JWTConfig
	Service  ServiceConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxConns        int32         `mapstructure:"max_conns"`
	MinConns        int32         `mapstructure:"min_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `mapstructure:"max_conn_idle_time"`
}

// DSN returns a pgx-compatible connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d pool_min_conns=%d",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode, d.MaxConns, d.MinConns,
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	// TTLs for cached data.
	CounterTTL    time.Duration `mapstructure:"counter_ttl"`
	SuggestionTTL time.Duration `mapstructure:"suggestion_ttl"`
}

// KafkaConfig holds Kafka broker and topic settings.
type KafkaConfig struct {
	Brokers         []string      `mapstructure:"brokers"`
	ConsumerGroupID string        `mapstructure:"consumer_group_id"`
	Topics          KafkaTopics   `mapstructure:"topics"`
	RetryMax        int           `mapstructure:"retry_max"`
	RetryBackoff    time.Duration `mapstructure:"retry_backoff"`
}

// KafkaTopics enumerates all Kafka topics used by this service.
type KafkaTopics struct {
	UserFollowed   string `mapstructure:"user_followed"`
	UserUnfollowed string `mapstructure:"user_unfollowed"`
	FeedInvalidate string `mapstructure:"feed_invalidate"`
	NotifyFollow   string `mapstructure:"notify_follow"`
}

// JWTConfig holds JWT signing key material.
type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

// ServiceConfig holds service-level tuning knobs.
type ServiceConfig struct {
	// MaxSuggestions caps the number of friend suggestions returned.
	MaxSuggestions int `mapstructure:"max_suggestions"`
	// BFSDepth is how many hops from the user the BFS suggestion algorithm explores.
	// Depth 2 means: viewer -> following -> following-of-following (candidates).
	BFSDepth int `mapstructure:"bfs_depth"`
	// DefaultPageSize is used for paginated list endpoints.
	DefaultPageSize int `mapstructure:"default_page_size"`
	// MaxPageSize caps the page_size query parameter.
	MaxPageSize int `mapstructure:"max_page_size"`
	// NotificationServiceURL is the base URL for the notification micro-service.
	NotificationServiceURL string `mapstructure:"notification_service_url"`
}

// Load reads configuration from environment variables and optional config file.
// Environment variables take precedence and follow the pattern
// SOCIAL_GRAPH_<SECTION>_<KEY> (upper-cased, underscore-separated).
func Load() (*Config, error) {
	v := viper.New()

	// -------------------------------------------------------------------------
	// Defaults
	// -------------------------------------------------------------------------
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")

	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "social_graph")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_conns", 20)
	v.SetDefault("database.min_conns", 2)
	v.SetDefault("database.max_conn_lifetime", "1h")
	v.SetDefault("database.max_conn_idle_time", "30m")

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 20)
	v.SetDefault("redis.min_idle_conns", 5)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("redis.counter_ttl", "24h")
	v.SetDefault("redis.suggestion_ttl", "10m")

	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.consumer_group_id", "social-graph-service")
	v.SetDefault("kafka.topics.user_followed", "user.followed")
	v.SetDefault("kafka.topics.user_unfollowed", "user.unfollowed")
	v.SetDefault("kafka.topics.feed_invalidate", "feed.invalidate")
	v.SetDefault("kafka.topics.notify_follow", "notification.follow")
	v.SetDefault("kafka.retry_max", 3)
	v.SetDefault("kafka.retry_backoff", "250ms")

	v.SetDefault("service.max_suggestions", 20)
	v.SetDefault("service.bfs_depth", 2)
	v.SetDefault("service.default_page_size", 20)
	v.SetDefault("service.max_page_size", 100)
	v.SetDefault("service.notification_service_url", "http://notification-service:8080")

	// -------------------------------------------------------------------------
	// Config file (optional — env vars are sufficient for containers)
	// -------------------------------------------------------------------------
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/social-graph-service")
	_ = v.ReadInConfig() // Ignore not-found.

	// -------------------------------------------------------------------------
	// Environment variables override everything else.
	// E.g. SOCIAL_GRAPH_DATABASE_HOST overrides database.host.
	// -------------------------------------------------------------------------
	v.SetEnvPrefix("SOCIAL_GRAPH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config: validation: %w", err)
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be in range 1-65535, got %d", cfg.Server.Port)
	}
	if cfg.Database.Host == "" {
		return fmt.Errorf("database.host must not be empty")
	}
	if cfg.Database.DBName == "" {
		return fmt.Errorf("database.dbname must not be empty")
	}
	if len(cfg.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers must not be empty")
	}
	if cfg.Service.MaxSuggestions <= 0 {
		return fmt.Errorf("service.max_suggestions must be > 0")
	}
	if cfg.Service.BFSDepth <= 0 {
		return fmt.Errorf("service.bfs_depth must be > 0")
	}
	if cfg.Service.DefaultPageSize <= 0 {
		cfg.Service.DefaultPageSize = 20
	}
	if cfg.Service.MaxPageSize <= 0 {
		cfg.Service.MaxPageSize = 100
	}
	return nil
}
