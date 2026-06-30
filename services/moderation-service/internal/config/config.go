package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the moderation service.
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Kafka     KafkaConfig
	AI        AIConfig
	Thresholds ThresholdConfig
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
}

type KafkaConfig struct {
	Brokers         []string
	ConsumerGroup   string
	VideoUploadTopic string
	ModerationTopic  string
}

// AIConfig contains endpoints and keys for all AI moderation services.
type AIConfig struct {
	// NSFW detection service (e.g., NudeNet, Sightengine, or self-hosted)
	NSFWEndpoint   string
	NSFWAPIKey     string
	NSFWTimeout    time.Duration

	// Violence detection service (e.g., Google Video Intelligence, AWS Rekognition)
	ViolenceEndpoint string
	ViolenceAPIKey   string
	ViolenceTimeout  time.Duration

	// General content classification (fallback/secondary model)
	ContentClassifierEndpoint string
	ContentClassifierAPIKey   string
	ContentClassifierTimeout  time.Duration

	// Frame extraction: how many frames per second to sample for video
	VideoFrameSampleRate int
}

// ThresholdConfig defines auto-reject and auto-approve score boundaries.
// Scores are in [0.0, 1.0]. Above AutoRejectThreshold => automatic rejection.
// Below AutoApproveThreshold => automatic approval. In-between => human review queue.
type ThresholdConfig struct {
	// NSFW
	NSFWAutoReject  float64
	NSFWAutoApprove float64

	// Violence
	ViolenceAutoReject  float64
	ViolenceAutoApprove float64

	// Spam
	SpamAutoReject  float64
	SpamAutoApprove float64

	// Combined weighted score for final decision
	CombinedAutoReject  float64
	CombinedAutoApprove float64

	// Weights for combining individual scores
	NSFWWeight     float64
	ViolenceWeight float64
	SpamWeight     float64

	// Human review queue: max age before escalation (hours)
	ReviewQueueMaxAgeHours int

	// Maximum number of pending appeals per user
	MaxAppealsPerUser int
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8084"),
			GRPCPort:     getEnv("GRPC_PORT", "9084"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			DSN:             getEnv("DATABASE_DSN", "postgres://postgres:postgres@localhost:5432/moderation?sslmode=disable"),
			MaxOpenConns:    getInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 1),
		},
		Kafka: KafkaConfig{
			Brokers:          splitEnv("KAFKA_BROKERS", []string{"localhost:9092"}),
			ConsumerGroup:    getEnv("KAFKA_CONSUMER_GROUP", "moderation-service"),
			VideoUploadTopic: getEnv("KAFKA_VIDEO_UPLOAD_TOPIC", "video.uploaded"),
			ModerationTopic:  getEnv("KAFKA_MODERATION_TOPIC", "moderation.results"),
		},
		AI: AIConfig{
			NSFWEndpoint:              getEnv("NSFW_ENDPOINT", "http://nsfw-detector:8080/api/v1/detect"),
			NSFWAPIKey:                getEnv("NSFW_API_KEY", ""),
			NSFWTimeout:               getDuration("NSFW_TIMEOUT", 10*time.Second),
			ViolenceEndpoint:          getEnv("VIOLENCE_ENDPOINT", "http://violence-detector:8081/api/v1/detect"),
			ViolenceAPIKey:            getEnv("VIOLENCE_API_KEY", ""),
			ViolenceTimeout:           getDuration("VIOLENCE_TIMEOUT", 10*time.Second),
			ContentClassifierEndpoint: getEnv("CLASSIFIER_ENDPOINT", "http://content-classifier:8082/api/v1/classify"),
			ContentClassifierAPIKey:   getEnv("CLASSIFIER_API_KEY", ""),
			ContentClassifierTimeout:  getDuration("CLASSIFIER_TIMEOUT", 15*time.Second),
			VideoFrameSampleRate:      getInt("VIDEO_FRAME_SAMPLE_RATE", 2),
		},
		Thresholds: ThresholdConfig{
			NSFWAutoReject:          getFloat("NSFW_AUTO_REJECT_THRESHOLD", 0.85),
			NSFWAutoApprove:         getFloat("NSFW_AUTO_APPROVE_THRESHOLD", 0.20),
			ViolenceAutoReject:      getFloat("VIOLENCE_AUTO_REJECT_THRESHOLD", 0.85),
			ViolenceAutoApprove:     getFloat("VIOLENCE_AUTO_APPROVE_THRESHOLD", 0.20),
			SpamAutoReject:          getFloat("SPAM_AUTO_REJECT_THRESHOLD", 0.85),
			SpamAutoApprove:         getFloat("SPAM_AUTO_APPROVE_THRESHOLD", 0.25),
			CombinedAutoReject:      getFloat("COMBINED_AUTO_REJECT_THRESHOLD", 0.80),
			CombinedAutoApprove:     getFloat("COMBINED_AUTO_APPROVE_THRESHOLD", 0.15),
			NSFWWeight:              getFloat("NSFW_WEIGHT", 0.45),
			ViolenceWeight:          getFloat("VIOLENCE_WEIGHT", 0.35),
			SpamWeight:              getFloat("SPAM_WEIGHT", 0.20),
			ReviewQueueMaxAgeHours:  getInt("REVIEW_QUEUE_MAX_AGE_HOURS", 24),
			MaxAppealsPerUser:       getInt("MAX_APPEALS_PER_USER", 3),
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
