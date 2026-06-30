package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the ads service.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	Auction  AuctionConfig
	Targeting TargetingConfig
	Budget   BudgetConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	GRPCPort     string
}

type DatabaseConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	// Separate DB for frequency capping (high QPS).
	FreqCapDB int
}

type KafkaConfig struct {
	Brokers           []string
	ConsumerGroup     string
	ImpressionTopic   string
	ClickTopic        string
	ConversionTopic   string
}

// AuctionConfig controls second-price auction behaviour.
type AuctionConfig struct {
	// Minimum allowed bid in micro-USD (1_000_000 = $1.00).
	MinBidMicroUSD int64
	// Penny added to the second price to determine final charge.
	PricePremiumMicroUSD int64
	// Maximum ads to consider per auction request.
	MaxCandidates int
	// How long to cache predicted CTR values (seconds).
	CTRCacheTTLSec int
	// Fallback CTR when no historical data exists.
	DefaultCTR float64
}

// TargetingConfig controls how user segments are matched.
type TargetingConfig struct {
	// Maximum number of interest categories to store per user.
	MaxInterestCategories int
	// Minimum lookalike similarity score (0-1) to include a user in an audience.
	LookalikeMinSimilarity float64
	// TTL for cached user profile data (seconds).
	UserProfileCacheTTLSec int
	// How many past ad impressions to track per user for exclusion.
	SeenAdsWindowDays int
}

// BudgetConfig controls pacing and billing cycles.
type BudgetConfig struct {
	// How often the budget pacer recalculates spend rates (seconds).
	PacingIntervalSec int
	// Overspend buffer: allow spending up to this fraction over daily budget before hard pause.
	OverspendBuffer float64
	// Billing cycle in days.
	BillingCycleDays int
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8085"),
			GRPCPort:     getEnv("GRPC_PORT", "9085"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
		},
		Database: DatabaseConfig{
			DSN:             getEnv("DATABASE_DSN", "postgres://postgres:postgres@localhost:5432/ads?sslmode=disable"),
			MaxOpenConns:    getInt("DB_MAX_OPEN_CONNS", 50),
			MaxIdleConns:    getInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:      getEnv("REDIS_ADDR", "localhost:6379"),
			Password:  getEnv("REDIS_PASSWORD", ""),
			DB:        getInt("REDIS_DB", 2),
			FreqCapDB: getInt("REDIS_FREQ_CAP_DB", 3),
		},
		Kafka: KafkaConfig{
			Brokers:         splitEnv("KAFKA_BROKERS", []string{"localhost:9092"}),
			ConsumerGroup:   getEnv("KAFKA_CONSUMER_GROUP", "ads-service"),
			ImpressionTopic: getEnv("KAFKA_IMPRESSION_TOPIC", "ads.impressions"),
			ClickTopic:      getEnv("KAFKA_CLICK_TOPIC", "ads.clicks"),
			ConversionTopic: getEnv("KAFKA_CONVERSION_TOPIC", "ads.conversions"),
		},
		Auction: AuctionConfig{
			MinBidMicroUSD:       getInt64("AUCTION_MIN_BID_MICRO_USD", 10_000),   // $0.01
			PricePremiumMicroUSD: getInt64("AUCTION_PRICE_PREMIUM_MICRO_USD", 10_000), // $0.01
			MaxCandidates:        getInt("AUCTION_MAX_CANDIDATES", 100),
			CTRCacheTTLSec:       getInt("AUCTION_CTR_CACHE_TTL_SEC", 300),
			DefaultCTR:           getFloat("AUCTION_DEFAULT_CTR", 0.02),
		},
		Targeting: TargetingConfig{
			MaxInterestCategories:  getInt("TARGETING_MAX_INTEREST_CATEGORIES", 50),
			LookalikeMinSimilarity: getFloat("TARGETING_LOOKALIKE_MIN_SIMILARITY", 0.65),
			UserProfileCacheTTLSec: getInt("TARGETING_USER_PROFILE_CACHE_TTL_SEC", 600),
			SeenAdsWindowDays:      getInt("TARGETING_SEEN_ADS_WINDOW_DAYS", 30),
		},
		Budget: BudgetConfig{
			PacingIntervalSec: getInt("BUDGET_PACING_INTERVAL_SEC", 60),
			OverspendBuffer:   getFloat("BUDGET_OVERSPEND_BUFFER", 0.05),
			BillingCycleDays:  getInt("BUDGET_BILLING_CYCLE_DAYS", 30),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getInt64(key string, defaultVal int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}

func getFloat(key string, defaultVal float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

func getDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

func splitEnv(key string, defaultVal []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	var result []string
	start := 0
	for i := 0; i < len(v); i++ {
		if v[i] == ',' {
			if s := v[start:i]; s != "" {
				result = append(result, s)
			}
			start = i + 1
		}
	}
	if s := v[start:]; s != "" {
		result = append(result, s)
	}
	return result
}
