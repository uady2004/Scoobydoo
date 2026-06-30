package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the wallet-service.
type Config struct {
	Server  ServerConfig
	DB      DatabaseConfig
	Redis   RedisConfig
	JWT     JWTConfig
	Payment PaymentServiceConfig
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

// DatabaseConfig holds PostgreSQL connection settings (pgx pool).
type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// RedisConfig holds Redis settings used for idempotency caching.
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// JWTConfig holds JWT validation settings (shared secret with auth-service).
type JWTConfig struct {
	Secret string
	Issuer string
}

// PaymentServiceConfig holds the gRPC / REST address of the payment-service.
type PaymentServiceConfig struct {
	BaseURL        string
	TimeoutSeconds int
}

// AppConfig holds application-level tuning knobs.
type AppConfig struct {
	Environment string
	LogLevel    string
	// DiamondsToUSDRate is the fixed conversion rate: 1 diamond = N USD cents.
	// E.g. 0.05 means 1 diamond = $0.0005 = 0.05 cents.
	DiamondsToUSDCents float64
	// CoinPackages maps package ID → {coins, price_cents}.
	// Loaded from env as JSON; default packages are hardcoded.
	CreatorRevenueSharePct float64 // e.g. 0.50 = 50 %
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
			Port:         getEnvInt("SERVER_PORT", 8090),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		DB: DatabaseConfig{
			URL:             getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/tiktok_wallet?sslmode=disable"),
			MaxConns:        int32(getEnvInt("DB_MAX_CONNS", 25)),
			MinConns:        int32(getEnvInt("DB_MIN_CONNS", 5)),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 1),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 20),
			DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "change-me-in-production"),
			Issuer: getEnv("JWT_ISSUER", "tiktok-clone-auth"),
		},
		Payment: PaymentServiceConfig{
			BaseURL:        getEnv("PAYMENT_SERVICE_URL", "http://localhost:8091"),
			TimeoutSeconds: getEnvInt("PAYMENT_SERVICE_TIMEOUT_SECONDS", 30),
		},
		App: AppConfig{
			Environment:            getEnv("APP_ENV", "development"),
			LogLevel:               getEnv("LOG_LEVEL", "info"),
			DiamondsToUSDCents:     getEnvFloat64("DIAMONDS_TO_USD_CENTS", 0.05),
			CreatorRevenueSharePct: getEnvFloat64("CREATOR_REVENUE_SHARE_PCT", 0.50),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("wallet-service config validation failed: %w", err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.DB.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.Payment.BaseURL == "" {
		return fmt.Errorf("PAYMENT_SERVICE_URL is required")
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

func getEnvFloat64(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
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
