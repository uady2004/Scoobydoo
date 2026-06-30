package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the video-service.
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	S3        S3Config
	Kafka     KafkaConfig
	FFmpeg    FFmpegConfig
	Whisper   WhisperConfig
	Upload    UploadConfig
	Transcode TranscodeConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Mode         string // gin mode: debug | release | test
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// DSN returns a pgx-compatible connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
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
}

// S3Config holds AWS S3 / compatible object-storage settings.
type S3Config struct {
	Region          string
	Bucket          string
	TempBucket      string // bucket for in-progress chunk uploads
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string // optional: for MinIO or other S3-compatible stores
	UsePathStyle    bool   // required for MinIO
	CDNBaseURL      string // public CDN prefix for serving media
}

// KafkaConfig holds Apache Kafka connection settings.
type KafkaConfig struct {
	Brokers          []string
	ConsumerGroup    string
	TopicUploaded    string // produced/consumed when a raw upload completes
	TopicTranscoded  string // produced when transcoding finishes
	TopicScheduled   string // produced when a video is scheduled
	Version          string // Kafka protocol version e.g. "3.6.0"
	SessionTimeout   time.Duration
	HeartbeatTimeout time.Duration
}

// FFmpegConfig holds FFmpeg binary and working-directory settings.
type FFmpegConfig struct {
	BinaryPath  string // absolute path to the ffmpeg executable
	ProbePath   string // absolute path to ffprobe executable
	TempDir     string // scratch space for transcoding artefacts
	Concurrency int    // max simultaneous transcode jobs
}

// WhisperConfig holds settings for the OpenAI Whisper subtitle API.
type WhisperConfig struct {
	APIKey   string
	Endpoint string
	Model    string
	Language string // ISO-639-1 default language hint, e.g. "en"
	Timeout  time.Duration
}

// UploadConfig holds chunked-upload behaviour settings.
type UploadConfig struct {
	ChunkSize      int64         // bytes per chunk (default 5 MiB)
	MaxFileSize    int64         // maximum allowed raw video size (default 2 GiB)
	TempDir        string        // local temp dir for assembling chunks
	ExpireAfter    time.Duration // how long an incomplete upload lives in Redis
	AllowedMIMEs   []string      // accepted MIME types
	MaxConcurrent  int           // max parallel uploads per user
}

// TranscodeConfig holds per-quality transcoding profiles.
type TranscodeConfig struct {
	Profiles      []TranscodeProfile
	HLSSegmentLen int    // HLS segment length in seconds (default 6)
	OutputDir     string // S3 key prefix for HLS output
}

// TranscodeProfile describes one output quality level.
type TranscodeProfile struct {
	Name       string // e.g. "360p"
	Width      int
	Height     int
	VideoBitrate string // e.g. "800k"
	AudioBitrate string // e.g. "128k"
	CRF        int    // constant rate factor (0-51; lower = better quality)
}

// DefaultTranscodeProfiles returns the standard 360p / 720p / 1080p profiles.
func DefaultTranscodeProfiles() []TranscodeProfile {
	return []TranscodeProfile{
		{Name: "360p",  Width: 640,  Height: 360,  VideoBitrate: "800k",  AudioBitrate: "96k",  CRF: 28},
		{Name: "720p",  Width: 1280, Height: 720,  VideoBitrate: "2500k", AudioBitrate: "128k", CRF: 23},
		{Name: "1080p", Width: 1920, Height: 1080, VideoBitrate: "5000k", AudioBitrate: "192k", CRF: 20},
	}
}

// Load reads configuration from environment variables, applying sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8082"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 60*time.Second),
			Mode:         getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", ""),
			DBName:          getEnv("DB_NAME", "tiktok_videos"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxConns:        int32(getEnvInt("DB_MAX_CONNS", 20)),
			MinConns:        int32(getEnvInt("DB_MIN_CONNS", 5)),
			MaxConnLifetime: getDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime: getDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 1),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 20),
			MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
			DialTimeout:  getDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		},
		S3: S3Config{
			Region:          getEnv("AWS_REGION", "us-east-1"),
			Bucket:          getEnv("S3_BUCKET", "tiktok-videos"),
			TempBucket:      getEnv("S3_TEMP_BUCKET", "tiktok-video-uploads-tmp"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
			Endpoint:        getEnv("S3_ENDPOINT", ""),
			UsePathStyle:    getEnv("S3_USE_PATH_STYLE", "false") == "true",
			CDNBaseURL:      getEnv("CDN_BASE_URL", ""),
		},
		Kafka: KafkaConfig{
			Brokers:          strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
			ConsumerGroup:    getEnv("KAFKA_CONSUMER_GROUP", "video-service"),
			TopicUploaded:    getEnv("KAFKA_TOPIC_UPLOADED", "video.uploaded"),
			TopicTranscoded:  getEnv("KAFKA_TOPIC_TRANSCODED", "video.transcoded"),
			TopicScheduled:   getEnv("KAFKA_TOPIC_SCHEDULED", "video.scheduled"),
			Version:          getEnv("KAFKA_VERSION", "3.6.0"),
			SessionTimeout:   getDuration("KAFKA_SESSION_TIMEOUT", 30*time.Second),
			HeartbeatTimeout: getDuration("KAFKA_HEARTBEAT_TIMEOUT", 10*time.Second),
		},
		FFmpeg: FFmpegConfig{
			BinaryPath:  getEnv("FFMPEG_PATH", "/usr/bin/ffmpeg"),
			ProbePath:   getEnv("FFPROBE_PATH", "/usr/bin/ffprobe"),
			TempDir:     getEnv("FFMPEG_TEMP_DIR", "/tmp/ffmpeg"),
			Concurrency: getEnvInt("FFMPEG_CONCURRENCY", 4),
		},
		Whisper: WhisperConfig{
			APIKey:   getEnv("WHISPER_API_KEY", ""),
			Endpoint: getEnv("WHISPER_ENDPOINT", "https://api.openai.com/v1/audio/transcriptions"),
			Model:    getEnv("WHISPER_MODEL", "whisper-1"),
			Language: getEnv("WHISPER_LANGUAGE", "en"),
			Timeout:  getDuration("WHISPER_TIMEOUT", 120*time.Second),
		},
		Upload: UploadConfig{
			ChunkSize:   getEnvInt64("UPLOAD_CHUNK_SIZE", 5*1024*1024),
			MaxFileSize: getEnvInt64("UPLOAD_MAX_FILE_SIZE", 2*1024*1024*1024),
			TempDir:     getEnv("UPLOAD_TEMP_DIR", "/tmp/uploads"),
			ExpireAfter: getDuration("UPLOAD_EXPIRE_AFTER", 24*time.Hour),
			AllowedMIMEs: strings.Split(
				getEnv("UPLOAD_ALLOWED_MIMES", "video/mp4,video/quicktime,video/x-msvideo,video/webm"),
				",",
			),
			MaxConcurrent: getEnvInt("UPLOAD_MAX_CONCURRENT", 3),
		},
		Transcode: TranscodeConfig{
			Profiles:      DefaultTranscodeProfiles(),
			HLSSegmentLen: getEnvInt("HLS_SEGMENT_LEN", 6),
			OutputDir:     getEnv("TRANSCODE_OUTPUT_DIR", "transcoded"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate performs basic sanity checks on the loaded configuration.
func (c *Config) validate() error {
	if c.Database.Password == "" {
		// Allow empty password for local dev; log a warning in production via caller.
	}
	if c.S3.Bucket == "" {
		return fmt.Errorf("config: S3_BUCKET must not be empty")
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("config: KAFKA_BROKERS must not be empty")
	}
	return nil
}

// ---- helpers ----------------------------------------------------------------

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
