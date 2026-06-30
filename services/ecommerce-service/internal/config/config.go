package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the ecommerce service.
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Redis         RedisConfig
	Kafka         KafkaConfig
	AWS           AWSConfig
	Elasticsearch ElasticsearchConfig
	Payment       PaymentServiceConfig
	JWT           JWTConfig
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

// RedisConfig holds Redis connection settings for cart caching and distributed locks.
type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
	// CartTTL is how long an idle cart lives in Redis.
	CartTTL time.Duration `mapstructure:"cart_ttl"`
	// LockTTL is the maximum lifetime of a distributed inventory lock.
	LockTTL time.Duration `mapstructure:"lock_ttl"`
}

// KafkaConfig holds Kafka broker settings for event emission.
type KafkaConfig struct {
	Brokers             []string `mapstructure:"brokers"`
	OrderEventsTopic    string   `mapstructure:"order_events_topic"`
	ProductEventsTopic  string   `mapstructure:"product_events_topic"`
	PaymentEventsTopic  string   `mapstructure:"payment_events_topic"`
	InventoryEventTopic string   `mapstructure:"inventory_event_topic"`
	GroupID             string   `mapstructure:"group_id"`
}

// AWSConfig holds AWS credentials and S3 settings for product image storage.
type AWSConfig struct {
	Region          string `mapstructure:"region"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	S3Bucket        string `mapstructure:"s3_bucket"`
	S3BaseURL       string `mapstructure:"s3_base_url"`
	// MaxImageSizeMB is the maximum allowed upload size in megabytes.
	MaxImageSizeMB int64 `mapstructure:"max_image_size_mb"`
}

// ElasticsearchConfig holds settings for the product search index.
type ElasticsearchConfig struct {
	Addresses    []string `mapstructure:"addresses"`
	Username     string   `mapstructure:"username"`
	Password     string   `mapstructure:"password"`
	ProductIndex string   `mapstructure:"product_index"`
}

// PaymentServiceConfig holds the internal gRPC/HTTP address of the payment service.
type PaymentServiceConfig struct {
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// JWTConfig holds the public settings required to validate inbound JWTs issued by
// the auth-service (the ecommerce service only verifies, never signs).
type JWTConfig struct {
	AccessSecret string `mapstructure:"access_secret"`
	Issuer       string `mapstructure:"issuer"`
}

// Load reads configuration from environment variables and an optional config file.
// Environment variables take precedence; keys are mapped by uppercasing and
// replacing "." with "_" (e.g. DATABASE_URL, AWS_S3_BUCKET).
func Load() (*Config, error) {
	v := viper.New()

	// ── Server defaults ──────────────────────────────────────────────────────
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8091)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.environment", "development")

	// ── Database defaults ────────────────────────────────────────────────────
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "5m")
	v.SetDefault("database.conn_max_idle_time", "1m")

	// ── Redis defaults ───────────────────────────────────────────────────────
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.cart_ttl", "72h")
	v.SetDefault("redis.lock_ttl", "10s")

	// ── Kafka defaults ───────────────────────────────────────────────────────
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.order_events_topic", "order-events")
	v.SetDefault("kafka.product_events_topic", "product-events")
	v.SetDefault("kafka.payment_events_topic", "payment-events")
	v.SetDefault("kafka.inventory_event_topic", "inventory-events")
	v.SetDefault("kafka.group_id", "ecommerce-service")

	// ── AWS defaults ─────────────────────────────────────────────────────────
	v.SetDefault("aws.region", "ap-southeast-1")
	v.SetDefault("aws.s3_bucket", "tiktok-clone-products")
	v.SetDefault("aws.max_image_size_mb", 10)

	// ── Elasticsearch defaults ───────────────────────────────────────────────
	v.SetDefault("elasticsearch.addresses", []string{"http://localhost:9200"})
	v.SetDefault("elasticsearch.product_index", "products")

	// ── Payment service defaults ─────────────────────────────────────────────
	v.SetDefault("payment.base_url", "http://payment-service:8085")
	v.SetDefault("payment.timeout", "30s")

	// ── JWT defaults ─────────────────────────────────────────────────────────
	v.SetDefault("jwt.issuer", "tiktok-clone-auth")

	// ── Config file (optional) ───────────────────────────────────────────────
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/ecommerce-service")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config: read file: %w", err)
		}
	}

	// ── Environment variable bindings ────────────────────────────────────────
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("database.url", "DATABASE_URL")
	_ = v.BindEnv("redis.addr", "REDIS_ADDR")
	_ = v.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = v.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	_ = v.BindEnv("aws.region", "AWS_REGION")
	_ = v.BindEnv("aws.access_key_id", "AWS_ACCESS_KEY_ID")
	_ = v.BindEnv("aws.secret_access_key", "AWS_SECRET_ACCESS_KEY")
	_ = v.BindEnv("aws.s3_bucket", "AWS_S3_BUCKET")
	_ = v.BindEnv("aws.s3_base_url", "AWS_S3_BASE_URL")
	_ = v.BindEnv("elasticsearch.addresses", "ELASTICSEARCH_ADDRESSES")
	_ = v.BindEnv("elasticsearch.username", "ELASTICSEARCH_USERNAME")
	_ = v.BindEnv("elasticsearch.password", "ELASTICSEARCH_PASSWORD")
	_ = v.BindEnv("payment.base_url", "PAYMENT_SERVICE_URL")
	_ = v.BindEnv("jwt.access_secret", "JWT_ACCESS_SECRET")

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
	return nil
}

// Addr returns the formatted "host:port" listen address.
func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
