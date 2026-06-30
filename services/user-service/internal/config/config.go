package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the user-service.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	S3       S3Config
	JWT      JWTConfig
	App      AppConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d pool_max_conn_lifetime=%s pool_max_conn_idle_time=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
		d.MaxOpenConns,
		d.ConnMaxLifetime.String(),
		d.ConnMaxIdleTime.String(),
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// TTL durations for various cache keys.
	ProfileTTL  time.Duration
	AnalyticsTTL time.Duration
}

// S3Config holds MinIO / S3-compatible object storage settings.
type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	Region          string
	// PresignedURLExpiry is how long a presigned upload URL is valid.
	PresignedURLExpiry time.Duration
	// AvatarMaxSizeBytes is the maximum allowed avatar file size.
	AvatarMaxSizeBytes int64
	// AllowedAvatarMIMETypes lists acceptable content-types for avatars.
	AllowedAvatarMIMETypes []string
	// AvatarPathPrefix is the object key prefix used for avatar files inside the bucket.
	AvatarPathPrefix string
}

// JWTConfig holds JWT validation settings.
type JWTConfig struct {
	Secret          string
	Issuer          string
	Audience        string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// AppConfig holds application-level tuning knobs.
type AppConfig struct {
	Environment      string
	LogLevel         string
	MaxSearchResults int
	EngagementWindow time.Duration // rolling window for engagement rate
}

// Load reads configuration from environment variables and returns a populated Config.
// All fields have sensible defaults so the service can start in development without
// extra configuration.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvInt("SERVER_PORT", 8082),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "postgres"),
			DBName:          getEnv("DB_NAME", "tiktok_users"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 0),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 20),
			MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
			DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
			ProfileTTL:   getEnvDuration("REDIS_PROFILE_TTL", 10*time.Minute),
			AnalyticsTTL: getEnvDuration("REDIS_ANALYTICS_TTL", 5*time.Minute),
		},
		S3: S3Config{
			Endpoint:        getEnv("S3_ENDPOINT", "localhost:9000"),
			AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", "minioadmin"),
			SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", "minioadmin"),
			UseSSL:          getEnvBool("S3_USE_SSL", false),
			BucketName:      getEnv("S3_BUCKET_NAME", "tiktok-user-assets"),
			Region:          getEnv("S3_REGION", "us-east-1"),
			PresignedURLExpiry: getEnvDuration("S3_PRESIGNED_URL_EXPIRY", 15*time.Minute),
			AvatarMaxSizeBytes: getEnvInt64("S3_AVATAR_MAX_SIZE_BYTES", 5*1024*1024), // 5 MB
			AllowedAvatarMIMETypes: []string{
				"image/jpeg",
				"image/png",
				"image/webp",
				"image/gif",
			},
			AvatarPathPrefix: getEnv("S3_AVATAR_PATH_PREFIX", "avatars"),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "change-me-in-production"),
			Issuer:          getEnv("JWT_ISSUER", "tiktok-clone-auth"),
			Audience:        getEnv("JWT_AUDIENCE", "tiktok-clone-users"),
			AccessTokenTTL:  getEnvDuration("JWT_ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTokenTTL: getEnvDuration("JWT_REFRESH_TOKEN_TTL", 7*24*time.Hour),
		},
		App: AppConfig{
			Environment:      getEnv("APP_ENV", "development"),
			LogLevel:         getEnv("LOG_LEVEL", "info"),
			MaxSearchResults: getEnvInt("APP_MAX_SEARCH_RESULTS", 50),
			EngagementWindow: getEnvDuration("APP_ENGAGEMENT_WINDOW", 30*24*time.Hour),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET must not be empty")
	}
	if c.Database.Host == "" {
		return fmt.Errorf("DB_HOST must not be empty")
	}
	if c.S3.BucketName == "" {
		return fmt.Errorf("S3_BUCKET_NAME must not be empty")
	}
	return nil
}

// Address returns the host:port string for the HTTP server listener.
func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// IsProduction returns true when the service is running in a production environment.
func (a AppConfig) IsProduction() bool {
	return a.Environment == "production"
}

// ---------- helper functions ----------

func getEnv(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return i
}

func getEnvInt64(key string, defaultVal int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return i
}

func getEnvBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
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
