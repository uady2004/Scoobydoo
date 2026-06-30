package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the analytics-service.
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	ClickHouse ClickHouseConfig
	Redis      RedisConfig
	Kafka      KafkaConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Mode         string
}

// DatabaseConfig holds PostgreSQL connection settings (used for metadata).
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DSN returns a pgx-compatible connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

// ClickHouseConfig holds ClickHouse connection settings.
type ClickHouseConfig struct {
	Addr     string // host:port
	Database string
	Username string
	Password string
	// Batch settings for event ingestion.
	BatchSize    int
	BatchTimeout time.Duration
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// KafkaConfig holds Kafka connection settings for the event consumer.
type KafkaConfig struct {
	Brokers       []string
	ConsumerGroup string
	Topics        KafkaTopics
}

// KafkaTopics enumerates all topics the analytics-service consumes.
type KafkaTopics struct {
	VideoViewed    string
	Engagement     string
	AdImpression   string
	LiveEvents     string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8086"),
			ReadTimeout:  parseDuration(getEnv("SERVER_READ_TIMEOUT", "30s")),
			WriteTimeout: parseDuration(getEnv("SERVER_WRITE_TIMEOUT", "30s")),
			Mode:         getEnv("GIN_MODE", "release"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "postgres"),
			Port:     parseInt(getEnv("DB_PORT", "5432")),
			User:     getEnv("DB_USER", "tiktok"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "tiktok_analytics"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		ClickHouse: ClickHouseConfig{
			Addr:         getEnv("CLICKHOUSE_ADDR", "clickhouse:9000"),
			Database:     getEnv("CLICKHOUSE_DATABASE", "tiktok"),
			Username:     getEnv("CLICKHOUSE_USERNAME", "default"),
			Password:     getEnv("CLICKHOUSE_PASSWORD", ""),
			BatchSize:    parseInt(getEnv("CLICKHOUSE_BATCH_SIZE", "1000")),
			BatchTimeout: parseDuration(getEnv("CLICKHOUSE_BATCH_TIMEOUT", "5s")),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "redis:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       parseInt(getEnv("REDIS_DB", "2")),
		},
		Kafka: KafkaConfig{
			Brokers:       strings.Split(getEnv("KAFKA_BROKERS", "kafka:9092"), ","),
			ConsumerGroup: getEnv("KAFKA_CONSUMER_GROUP", "analytics-service"),
			Topics: KafkaTopics{
				VideoViewed:  getEnv("KAFKA_TOPIC_VIDEO_VIEWED", "video.viewed"),
				Engagement:   getEnv("KAFKA_TOPIC_ENGAGEMENT", "video.engagement"),
				AdImpression: getEnv("KAFKA_TOPIC_AD_IMPRESSION", "ad.impression"),
				LiveEvents:   getEnv("KAFKA_TOPIC_LIVE_EVENTS", "live.events"),
			},
		},
	}
	return cfg, nil
}

// getEnv returns the value of an environment variable or the given default.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// parseInt parses s as a base-10 integer, returning 0 on error.
func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// parseDuration parses s as a time.Duration, returning 0 on error.
func parseDuration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	return d
}
