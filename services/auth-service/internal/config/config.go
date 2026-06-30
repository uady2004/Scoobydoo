package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the auth service.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	OAuth    OAuthConfig
	MFA      MFAConfig
	Email    EmailConfig
	SMS      SMSConfig
	Kafka    KafkaConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	Environment  string        `mapstructure:"environment"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
	// OTP TTL settings
	OTPExpiry     time.Duration `mapstructure:"otp_expiry"`
	SessionExpiry time.Duration `mapstructure:"session_expiry"`
}

// JWTConfig holds JSON Web Token signing settings.
type JWTConfig struct {
	// AccessSecret is the HMAC-SHA256 secret for access tokens.
	AccessSecret string `mapstructure:"access_secret"`
	// RefreshSecret is the HMAC-SHA256 secret for refresh tokens.
	RefreshSecret string `mapstructure:"refresh_secret"`
	// AccessTTL is how long access tokens remain valid.
	AccessTTL time.Duration `mapstructure:"access_ttl"`
	// RefreshTTL is how long refresh tokens remain valid.
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
	// Issuer is embedded in every token's "iss" claim.
	Issuer string `mapstructure:"issuer"`
}

// OAuthConfig holds credentials for third-party OAuth providers.
type OAuthConfig struct {
	Google GoogleOAuthConfig `mapstructure:"google"`
	Apple  AppleOAuthConfig  `mapstructure:"apple"`
}

// GoogleOAuthConfig holds Google OAuth 2.0 credentials.
type GoogleOAuthConfig struct {
	ClientID     string   `mapstructure:"client_id"`
	ClientSecret string   `mapstructure:"client_secret"`
	RedirectURL  string   `mapstructure:"redirect_url"`
	Scopes       []string `mapstructure:"scopes"`
}

// AppleOAuthConfig holds Sign in with Apple credentials.
type AppleOAuthConfig struct {
	ClientID   string `mapstructure:"client_id"`
	TeamID     string `mapstructure:"team_id"`
	KeyID      string `mapstructure:"key_id"`
	PrivateKey string `mapstructure:"private_key"`
}

// MFAConfig holds TOTP / multi-factor authentication settings.
type MFAConfig struct {
	Issuer string `mapstructure:"issuer"`
	Digits int    `mapstructure:"digits"`
	Period uint   `mapstructure:"period"`
}

// EmailConfig holds SMTP / email delivery settings.
type EmailConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	FromName string `mapstructure:"from_name"`
}

// SMSConfig holds SMS gateway settings for OTP delivery.
type SMSConfig struct {
	Provider  string `mapstructure:"provider"`
	AccountID string `mapstructure:"account_id"`
	AuthToken string `mapstructure:"auth_token"`
	FromPhone string `mapstructure:"from_phone"`
}

// KafkaConfig holds Kafka broker settings for event emission.
type KafkaConfig struct {
	Brokers        []string `mapstructure:"brokers"`
	UserEventTopic string   `mapstructure:"user_event_topic"`
}

// Load reads configuration from environment variables and optional config file.
// Environment variables take precedence and are mapped by replacing "." with "_"
// and uppercasing (e.g. SERVER_PORT, JWT_ACCESS_SECRET).
func Load() (*Config, error) {
	v := viper.New()

	// ── Defaults ────────────────────────────────────────────────────────────
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.environment", "development")

	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "5m")
	v.SetDefault("database.conn_max_idle_time", "1m")

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.otp_expiry", "5m")
	v.SetDefault("redis.session_expiry", "720h") // 30 days

	v.SetDefault("jwt.access_ttl", "15m")
	v.SetDefault("jwt.refresh_ttl", "720h")
	v.SetDefault("jwt.issuer", "tiktok-clone-auth")

	v.SetDefault("oauth.google.scopes", []string{"email", "profile"})

	v.SetDefault("mfa.issuer", "TikTokClone")
	v.SetDefault("mfa.digits", 6)
	v.SetDefault("mfa.period", 30)

	v.SetDefault("kafka.user_event_topic", "user-events")
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})

	// ── Config file (optional) ───────────────────────────────────────────────
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/auth-service")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		// A missing config file is acceptable; env vars cover the gap.
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config: read file: %w", err)
		}
	}

	// ── Environment variables ────────────────────────────────────────────────
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit bindings for env vars that commonly differ in naming.
	_ = v.BindEnv("database.url", "DATABASE_URL")
	_ = v.BindEnv("redis.addr", "REDIS_ADDR")
	_ = v.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = v.BindEnv("jwt.access_secret", "JWT_ACCESS_SECRET")
	_ = v.BindEnv("jwt.refresh_secret", "JWT_REFRESH_SECRET")
	_ = v.BindEnv("oauth.google.client_id", "GOOGLE_CLIENT_ID")
	_ = v.BindEnv("oauth.google.client_secret", "GOOGLE_CLIENT_SECRET")
	_ = v.BindEnv("oauth.apple.client_id", "APPLE_CLIENT_ID")
	_ = v.BindEnv("oauth.apple.team_id", "APPLE_TEAM_ID")
	_ = v.BindEnv("oauth.apple.key_id", "APPLE_KEY_ID")
	_ = v.BindEnv("oauth.apple.private_key", "APPLE_PRIVATE_KEY")

	// ── Unmarshal ────────────────────────────────────────────────────────────
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate performs minimal sanity checks on required fields.
func (c *Config) validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("config: DATABASE_URL is required")
	}
	if c.JWT.AccessSecret == "" {
		return fmt.Errorf("config: JWT_ACCESS_SECRET is required")
	}
	if c.JWT.RefreshSecret == "" {
		return fmt.Errorf("config: JWT_REFRESH_SECRET is required")
	}
	return nil
}

// Addr returns the formatted "host:port" listen address.
func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
