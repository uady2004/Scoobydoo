package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the payment-service.
type Config struct {
	Server  ServerConfig
	DB      DatabaseConfig
	Redis   RedisConfig
	Stripe  StripeConfig
	JWT     JWTConfig
	App     AppConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// Addr returns the "host:port" listen address.
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// RedisConfig holds Redis settings used for idempotency key caching.
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// IdempotencyTTL is how long idempotency keys are remembered.
	IdempotencyTTL time.Duration
}

// StripeConfig holds all Stripe API credentials and endpoint settings.
type StripeConfig struct {
	// SecretKey is the Stripe secret key (sk_live_... / sk_test_...).
	SecretKey string
	// PublishableKey is returned to clients to initialise Stripe.js.
	PublishableKey string
	// WebhookSecret is used to verify the Stripe-Signature header on webhooks.
	WebhookSecret string
	// ConnectClientID is the Stripe Connect OAuth client_id for creator payouts.
	ConnectClientID string
	// Currency is the default ISO 4217 currency code (e.g. "usd").
	Currency string
	// StatementDescriptor is shown on users' bank statements (max 22 chars).
	StatementDescriptor string
	// MaxNetworkRetries configures automatic retries by the Stripe SDK.
	MaxNetworkRetries int
}

// JWTConfig holds JWT validation settings shared with auth-service.
type JWTConfig struct {
	Secret string
	Issuer string
}

// AppConfig holds application-level settings.
type AppConfig struct {
	Environment string
	LogLevel    string
}

// IsProduction returns true when running in production.
func (a AppConfig) IsProduction() bool {
	return a.Environment == "production"
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvInt("SERVER_PORT", 8091),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		DB: DatabaseConfig{
			URL:             getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/tiktok_payments?sslmode=disable"),
			MaxConns:        int32(getEnvInt("DB_MAX_CONNS", 20)),
			MinConns:        int32(getEnvInt("DB_MIN_CONNS", 5)),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:           getEnv("REDIS_ADDR", "localhost:6379"),
			Password:       getEnv("REDIS_PASSWORD", ""),
			DB:             getEnvInt("REDIS_DB", 2),
			PoolSize:       getEnvInt("REDIS_POOL_SIZE", 20),
			DialTimeout:    getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:    getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout:   getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
			IdempotencyTTL: getEnvDuration("REDIS_IDEMPOTENCY_TTL", 24*time.Hour),
		},
		Stripe: StripeConfig{
			SecretKey:           getEnv("STRIPE_SECRET_KEY", ""),
			PublishableKey:      getEnv("STRIPE_PUBLISHABLE_KEY", ""),
			WebhookSecret:       getEnv("STRIPE_WEBHOOK_SECRET", ""),
			ConnectClientID:     getEnv("STRIPE_CONNECT_CLIENT_ID", ""),
			Currency:            getEnv("STRIPE_CURRENCY", "usd"),
			StatementDescriptor: getEnv("STRIPE_STATEMENT_DESCRIPTOR", "TIKTOKCLONE"),
			MaxNetworkRetries:   getEnvInt("STRIPE_MAX_NETWORK_RETRIES", 3),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "change-me-in-production"),
			Issuer: getEnv("JWT_ISSUER", "tiktok-clone-auth"),
		},
		App: AppConfig{
			Environment: getEnv("APP_ENV", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("payment-service config validation failed: %w", err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.DB.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Stripe.SecretKey == "" {
		return fmt.Errorf("STRIPE_SECRET_KEY is required")
	}
	if c.Stripe.WebhookSecret == "" {
		return fmt.Errorf("STRIPE_WEBHOOK_SECRET is required")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	return nil
}

// ---------- helpers ----------

func getEnv(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
