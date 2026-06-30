package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the messaging service.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	S3       S3Config       `mapstructure:"s3"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Crypto   CryptoConfig   `mapstructure:"crypto"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
	Log      LogConfig      `mapstructure:"log"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d pool_min_conns=%d pool_max_conn_lifetime=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode, d.MaxOpenConns, d.MaxIdleConns, d.ConnMaxLifetime,
	)
}

type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
}

type S3Config struct {
	Endpoint        string `mapstructure:"endpoint"`
	Region          string `mapstructure:"region"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	UsePathStyle    bool   `mapstructure:"use_path_style"`
}

type JWTConfig struct {
	Secret          string        `mapstructure:"secret"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
}

// CryptoConfig holds AES-256-GCM key material. Key must be exactly 32 bytes
// (256 bits) encoded as a 64-character hex string.
type CryptoConfig struct {
	EncryptionKeyHex string `mapstructure:"encryption_key_hex"`
}

type WebSocketConfig struct {
	// MaxMessageSize is the maximum allowed message size in bytes for a WebSocket frame.
	MaxMessageSize int64         `mapstructure:"max_message_size"`
	// PongWait is how long the server waits for a pong from the client after a ping.
	PongWait       time.Duration `mapstructure:"pong_wait"`
	// PingInterval must be less than PongWait.
	PingInterval   time.Duration `mapstructure:"ping_interval"`
	// WriteWait is the time allowed to write a message to the peer.
	WriteWait      time.Duration `mapstructure:"write_wait"`
	// SendBufferSize is the number of outbound messages buffered per client.
	SendBufferSize int           `mapstructure:"send_buffer_size"`
	// AllowedOrigins is a comma-separated list of allowed origins (supports "*").
	AllowedOrigins string        `mapstructure:"allowed_origins"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // "json" or "console"
}

// Load reads configuration from environment variables and optional config files.
// Environment variable keys are upper-cased and dot-separated keys use underscores,
// e.g. SERVER_PORT, DATABASE_HOST.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8085)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.idle_timeout", "60s")

	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.dbname", "messaging")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "15m")

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("redis.pool_size", 20)

	v.SetDefault("s3.endpoint", "http://localhost:9000")
	v.SetDefault("s3.region", "us-east-1")
	v.SetDefault("s3.bucket", "messaging-media")
	v.SetDefault("s3.use_path_style", true)

	v.SetDefault("jwt.access_token_ttl", "15m")
	v.SetDefault("jwt.refresh_token_ttl", "168h")

	v.SetDefault("websocket.max_message_size", 65536)
	v.SetDefault("websocket.pong_wait", "60s")
	v.SetDefault("websocket.ping_interval", "54s")
	v.SetDefault("websocket.write_wait", "10s")
	v.SetDefault("websocket.send_buffer_size", 256)
	v.SetDefault("websocket.allowed_origins", "*")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	// Environment variable binding
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit bindings for nested keys (AutomaticEnv alone is unreliable for nested structs)
	_ = v.BindEnv("crypto.encryption_key_hex", "CRYPTO_ENCRYPTION_KEY_HEX")
	_ = v.BindEnv("jwt.secret", "JWT_SECRET")
	_ = v.BindEnv("database.host", "DATABASE_HOST")
	_ = v.BindEnv("database.port", "DATABASE_PORT")
	_ = v.BindEnv("database.user", "DATABASE_USER")
	_ = v.BindEnv("database.password", "DATABASE_PASSWORD")
	_ = v.BindEnv("database.dbname", "DATABASE_DBNAME")
	_ = v.BindEnv("database.sslmode", "DATABASE_SSLMODE")
	_ = v.BindEnv("redis.addr", "REDIS_ADDR")

	// Optional config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/messaging-service/")
	v.AddConfigPath(".")
	_ = v.ReadInConfig() // Not fatal if absent

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal failed: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Crypto.EncryptionKeyHex == "" {
		return fmt.Errorf("CRYPTO_ENCRYPTION_KEY_HEX must be set (64-char hex string representing 32 bytes)")
	}
	if len(c.Crypto.EncryptionKeyHex) != 64 {
		return fmt.Errorf("CRYPTO_ENCRYPTION_KEY_HEX must be exactly 64 hex characters, got %d", len(c.Crypto.EncryptionKeyHex))
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET must be set")
	}
	return nil
}

// Addr returns the full server listen address.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
