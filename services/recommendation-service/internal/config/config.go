package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the recommendation service.
type Config struct {
	Server          ServerConfig
	Redis           RedisConfig
	Elasticsearch   ElasticsearchConfig
	Kafka           KafkaConfig
	Recommendation  RecommendationConfig
	ABTesting       ABTestingConfig
	FeatureStore    FeatureStoreConfig
	Embedding       EmbeddingConfig
	ModelUpdate     ModelUpdateConfig
	Observability   ObservabilityConfig
}

type ServerConfig struct {
	GRPCPort    int           `mapstructure:"grpc_port"`
	HTTPPort    int           `mapstructure:"http_port"`
	GracePeriod time.Duration `mapstructure:"grace_period"`
}

type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	// Cluster mode: when Addrs has more than one entry, a cluster client is used.
	Addrs []string `mapstructure:"addrs"`
}

type ElasticsearchConfig struct {
	Addresses        []string      `mapstructure:"addresses"`
	Username         string        `mapstructure:"username"`
	Password         string        `mapstructure:"password"`
	VideoIndex       string        `mapstructure:"video_index"`
	EmbeddingIndex   string        `mapstructure:"embedding_index"`
	RequestTimeout   time.Duration `mapstructure:"request_timeout"`
	MaxRetries       int           `mapstructure:"max_retries"`
}

type KafkaConfig struct {
	Brokers              []string      `mapstructure:"brokers"`
	EngagementTopic      string        `mapstructure:"engagement_topic"`
	RecommendationTopic  string        `mapstructure:"recommendation_topic"`
	ConsumerGroup        string        `mapstructure:"consumer_group"`
	SessionTimeout       time.Duration `mapstructure:"session_timeout"`
	HeartbeatInterval    time.Duration `mapstructure:"heartbeat_interval"`
}

type RecommendationConfig struct {
	// CandidatePoolSize is the maximum number of raw candidates fetched before ranking.
	CandidatePoolSize int `mapstructure:"candidate_pool_size"`
	// FinalFeedSize is the number of items returned per request.
	FinalFeedSize int `mapstructure:"final_feed_size"`
	// CoarseRankSize is the size of the set passed to fine-ranking.
	CoarseRankSize int `mapstructure:"coarse_rank_size"`
	// MaxConsecutiveSameCreator enforces creator diversity during injection.
	MaxConsecutiveSameCreator int `mapstructure:"max_consecutive_same_creator"`
	// SeenVideoTTL is how long a video stays in the user's seen set in Redis.
	SeenVideoTTL time.Duration `mapstructure:"seen_video_ttl"`
	// FreshnessHalfLifeHours controls how fast freshness decays.
	FreshnessHalfLifeHours float64 `mapstructure:"freshness_half_life_hours"`
	// Weights for the coarse ranking formula.
	CoarseWeightEngagement float64 `mapstructure:"coarse_weight_engagement"`
	CoarseWeightFreshness  float64 `mapstructure:"coarse_weight_freshness"`
	CoarseWeightRelevance  float64 `mapstructure:"coarse_weight_relevance"`
}

type ABTestingConfig struct {
	// ExperimentsKey is the Redis key from which active experiments are loaded.
	ExperimentsKey    string        `mapstructure:"experiments_key"`
	RefreshInterval   time.Duration `mapstructure:"refresh_interval"`
	TrackingEnabled   bool          `mapstructure:"tracking_enabled"`
}

type FeatureStoreConfig struct {
	// WatchHistorySize is how many videos are kept in the user watch-history feature.
	WatchHistorySize   int           `mapstructure:"watch_history_size"`
	UserFeatureTTL     time.Duration `mapstructure:"user_feature_ttl"`
	VideoFeatureTTL    time.Duration `mapstructure:"video_feature_ttl"`
}

type EmbeddingConfig struct {
	Dimensions      int     `mapstructure:"dimensions"`
	// KNNNumCandidates is the ef_search parameter for the HNSW index.
	KNNNumCandidates int    `mapstructure:"knn_num_candidates"`
	// KNNTopK is how many neighbors are returned from the KNN query.
	KNNTopK          int    `mapstructure:"knn_top_k"`
	SimilarityMetric string `mapstructure:"similarity_metric"` // "cosine" | "dot_product" | "l2_norm"
}

type ModelUpdateConfig struct {
	// MatrixUpdateInterval controls how frequently the collaborative filtering
	// item-item similarity matrix is rebuilt from the latest engagement events.
	MatrixUpdateInterval time.Duration `mapstructure:"matrix_update_interval"`
	// MinInteractionsForItem is the minimum number of engagement events an item
	// needs before it is included in the CF matrix.
	MinInteractionsForItem int `mapstructure:"min_interactions_for_item"`
	// TopKSimilarItems is how many similar items are stored per item in Redis.
	TopKSimilarItems       int `mapstructure:"top_k_similar_items"`
	// EngagementWindowDays is the lookback window for building the matrix.
	EngagementWindowDays   int `mapstructure:"engagement_window_days"`
}

type ObservabilityConfig struct {
	OTLPEndpoint    string `mapstructure:"otlp_endpoint"`
	ServiceName     string `mapstructure:"service_name"`
	LogLevel        string `mapstructure:"log_level"`
	MetricsEnabled  bool   `mapstructure:"metrics_enabled"`
}

// Load reads configuration from environment variables and config files.
// Environment variables override file values; the prefix "REC" is stripped,
// e.g. REC_SERVER_GRPC_PORT=9090.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults.
	setDefaults(v)

	// Config file (optional).
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/recommendation-service/")
	v.AddConfigPath("$HOME/.recommendation-service")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config file: %w", err)
		}
		// Config file is optional; continue with defaults and env vars.
	}

	// Environment variable binding.
	v.SetEnvPrefix("REC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.grpc_port", 50051)
	v.SetDefault("server.http_port", 8080)
	v.SetDefault("server.grace_period", 10*time.Second)

	// Redis
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 20)
	v.SetDefault("redis.read_timeout", 3*time.Second)
	v.SetDefault("redis.write_timeout", 3*time.Second)

	// Elasticsearch
	v.SetDefault("elasticsearch.addresses", []string{"http://localhost:9200"})
	v.SetDefault("elasticsearch.video_index", "videos")
	v.SetDefault("elasticsearch.embedding_index", "video_embeddings")
	v.SetDefault("elasticsearch.request_timeout", 5*time.Second)
	v.SetDefault("elasticsearch.max_retries", 3)

	// Kafka
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.engagement_topic", "engagement-events")
	v.SetDefault("kafka.recommendation_topic", "recommendation-events")
	v.SetDefault("kafka.consumer_group", "recommendation-service")
	v.SetDefault("kafka.session_timeout", 10*time.Second)
	v.SetDefault("kafka.heartbeat_interval", 3*time.Second)

	// Recommendation
	v.SetDefault("recommendation.candidate_pool_size", 500)
	v.SetDefault("recommendation.final_feed_size", 20)
	v.SetDefault("recommendation.coarse_rank_size", 100)
	v.SetDefault("recommendation.max_consecutive_same_creator", 3)
	v.SetDefault("recommendation.seen_video_ttl", 7*24*time.Hour)
	v.SetDefault("recommendation.freshness_half_life_hours", 24.0)
	v.SetDefault("recommendation.coarse_weight_engagement", 0.5)
	v.SetDefault("recommendation.coarse_weight_freshness", 0.3)
	v.SetDefault("recommendation.coarse_weight_relevance", 0.2)

	// A/B testing
	v.SetDefault("abtesting.experiments_key", "rec:experiments:active")
	v.SetDefault("abtesting.refresh_interval", 60*time.Second)
	v.SetDefault("abtesting.tracking_enabled", true)

	// Feature store
	v.SetDefault("featurestore.watch_history_size", 100)
	v.SetDefault("featurestore.user_feature_ttl", 1*time.Hour)
	v.SetDefault("featurestore.video_feature_ttl", 30*time.Minute)

	// Embedding
	v.SetDefault("embedding.dimensions", 128)
	v.SetDefault("embedding.knn_num_candidates", 200)
	v.SetDefault("embedding.knn_top_k", 50)
	v.SetDefault("embedding.similarity_metric", "cosine")

	// Model update
	v.SetDefault("modelupdate.matrix_update_interval", 15*time.Minute)
	v.SetDefault("modelupdate.min_interactions_for_item", 5)
	v.SetDefault("modelupdate.top_k_similar_items", 50)
	v.SetDefault("modelupdate.engagement_window_days", 30)

	// Observability
	v.SetDefault("observability.service_name", "recommendation-service")
	v.SetDefault("observability.log_level", "info")
	v.SetDefault("observability.metrics_enabled", true)
}

func validate(cfg *Config) error {
	if cfg.Server.GRPCPort <= 0 || cfg.Server.GRPCPort > 65535 {
		return fmt.Errorf("server.grpc_port must be in range [1, 65535]")
	}
	if cfg.Server.HTTPPort <= 0 || cfg.Server.HTTPPort > 65535 {
		return fmt.Errorf("server.http_port must be in range [1, 65535]")
	}
	if cfg.Recommendation.FinalFeedSize <= 0 {
		return fmt.Errorf("recommendation.final_feed_size must be > 0")
	}
	if cfg.Recommendation.CandidatePoolSize < cfg.Recommendation.FinalFeedSize {
		return fmt.Errorf("recommendation.candidate_pool_size must be >= final_feed_size")
	}
	total := cfg.Recommendation.CoarseWeightEngagement +
		cfg.Recommendation.CoarseWeightFreshness +
		cfg.Recommendation.CoarseWeightRelevance
	if total < 0.99 || total > 1.01 {
		return fmt.Errorf("coarse ranking weights must sum to 1.0 (got %.3f)", total)
	}
	if cfg.Embedding.Dimensions <= 0 {
		return fmt.Errorf("embedding.dimensions must be > 0")
	}
	return nil
}
