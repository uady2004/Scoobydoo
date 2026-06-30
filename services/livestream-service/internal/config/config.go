package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the livestream service.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	RTMP      RTMPConfig      `mapstructure:"rtmp"`
	FFmpeg    FFmpegConfig    `mapstructure:"ffmpeg"`
	HLS       HLSConfig       `mapstructure:"hls"`
	WebRTC    WebRTCConfig    `mapstructure:"webrtc"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	CDN       CDNConfig       `mapstructure:"cdn"`
	Moderation ModerationConfig `mapstructure:"moderation"`
}

type ServerConfig struct {
	HTTPPort    int           `mapstructure:"http_port"`
	GRPCPort    int           `mapstructure:"grpc_port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	Environment  string        `mapstructure:"environment"`
	LogLevel     string        `mapstructure:"log_level"`
}

type RTMPConfig struct {
	Port            int           `mapstructure:"port"`
	Host            string        `mapstructure:"host"`
	// AppName is the RTMP application name (the path segment after the host:port).
	AppName         string        `mapstructure:"app_name"`
	ChunkSize       int           `mapstructure:"chunk_size"`
	GopCacheEnabled bool          `mapstructure:"gop_cache_enabled"`
	PingTimeout     time.Duration `mapstructure:"ping_timeout"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	MaxConnections  int           `mapstructure:"max_connections"`
	// Stream key prefix used for validation
	StreamKeyPrefix string `mapstructure:"stream_key_prefix"`
	// TLS config (optional)
	TLSEnabled  bool   `mapstructure:"tls_enabled"`
	TLSCertFile string `mapstructure:"tls_cert_file"`
	TLSKeyFile  string `mapstructure:"tls_key_file"`
}

// Addr returns the listen address for the RTMP server (host:port).
func (r RTMPConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type FFmpegConfig struct {
	// Path to the ffmpeg binary
	BinaryPath string `mapstructure:"binary_path"`
	// Path to the ffprobe binary
	ProbePath string `mapstructure:"probe_path"`
	// Number of threads per transcoding job
	Threads int `mapstructure:"threads"`
	// Preset: ultrafast, superfast, veryfast, faster, fast, medium
	Preset string `mapstructure:"preset"`
	// CRF quality factor (0-51; lower = better quality)
	CRF int `mapstructure:"crf"`
	// Video codec: libx264, libx265, copy
	VideoCodec string `mapstructure:"video_codec"`
	// Audio codec: aac, copy
	AudioCodec string `mapstructure:"audio_codec"`
	// Audio bitrate e.g. "128k"
	AudioBitrate string `mapstructure:"audio_bitrate"`
	// Max concurrent transcoding sessions
	MaxSessions int `mapstructure:"max_sessions"`
	// LogLevel controls FFmpeg's -loglevel flag (e.g. "error", "warning", "info")
	LogLevel string `mapstructure:"log_level"`
	// HWAccel enables hardware acceleration (e.g. "nvenc", "vaapi", "" to disable)
	HWAccel string `mapstructure:"hw_accel"`
}

type HLSConfig struct {
	// Root path on the shared volume where HLS files are stored
	OutputPath string `mapstructure:"output_path"`
	// Segment duration in seconds (target 2s)
	SegmentDuration int `mapstructure:"segment_duration"`
	// Number of segments to keep in the playlist
	PlaylistLength int `mapstructure:"playlist_length"`
	// Base public URL for the HLS playlist (e.g. https://cdn.example.com/hls)
	BaseURL string `mapstructure:"base_url"`
	// Renditions to produce
	Renditions []HLSRendition `mapstructure:"renditions"`
	// Delete old segment files automatically
	DeleteOldSegments bool `mapstructure:"delete_old_segments"`
	// Encryption key rotation interval (0 = disabled)
	KeyRotationInterval int `mapstructure:"key_rotation_interval"`
}

// HLSRendition describes a single quality level.
type HLSRendition struct {
	Name       string `mapstructure:"name"`        // e.g. "360p"
	Width      int    `mapstructure:"width"`       // e.g. 640
	Height     int    `mapstructure:"height"`      // e.g. 360
	Bitrate    string `mapstructure:"bitrate"`     // e.g. "800k"
	MaxBitrate string `mapstructure:"max_bitrate"` // e.g. "856k"
	// Framerate is the target FPS (0 = use source frame rate).
	Framerate  int    `mapstructure:"framerate"`
}

// FramerateOrDefault returns the configured framerate, falling back to def when 0.
func (r HLSRendition) FramerateOrDefault(def int) int {
	if r.Framerate > 0 {
		return r.Framerate
	}
	return def
}

// DefaultRenditions returns the standard 360p and 720p renditions.
func DefaultRenditions() []HLSRendition {
	return []HLSRendition{
		{Name: "360p", Width: 640, Height: 360, Bitrate: "800k", MaxBitrate: "856k"},
		{Name: "720p", Width: 1280, Height: 720, Bitrate: "2800k", MaxBitrate: "2996k"},
	}
}

type WebRTCConfig struct {
	// STUN server URLs
	STUNServers []string `mapstructure:"stun_servers"`
	// TURN server URLs
	TURNServers []TURNServer `mapstructure:"turn_servers"`
	// ICE candidate gather timeout
	ICEGatherTimeout time.Duration `mapstructure:"ice_gather_timeout"`
	// Maximum bitrate for WebRTC publishing (bps)
	MaxBitrate int `mapstructure:"max_bitrate"`
	// Enable simulcast
	SimulcastEnabled bool `mapstructure:"simulcast_enabled"`
	// Enable DTLS fingerprint verification
	DTLSVerify bool `mapstructure:"dtls_verify"`
}

type TURNServer struct {
	URL        string `mapstructure:"url"`
	Username   string `mapstructure:"username"`
	Credential string `mapstructure:"credential"`
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
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// DSN builds the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d pool_min_conns=%d pool_max_conn_lifetime=%s pool_max_conn_idle_time=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
		d.MaxOpenConns, d.MaxIdleConns,
		d.ConnMaxLifetime.String(), d.ConnMaxIdleTime.String(),
	)
}

type RedisConfig struct {
	// Support both single-node and cluster modes
	Addrs        []string      `mapstructure:"addrs"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
	// Key TTLs
	ViewerCountTTL  time.Duration `mapstructure:"viewer_count_ttl"`
	StreamMetaTTL   time.Duration `mapstructure:"stream_meta_ttl"`
	BattleScoreTTL  time.Duration `mapstructure:"battle_score_ttl"`
	PKBattleTTL     time.Duration `mapstructure:"pk_battle_ttl"`
}

// RedisAddr returns the first address for single-node usage.
func (r RedisConfig) RedisAddr() string {
	if len(r.Addrs) > 0 {
		return r.Addrs[0]
	}
	return "localhost:6379"
}

type KafkaConfig struct {
	Brokers          []string      `mapstructure:"brokers"`
	GroupID          string        `mapstructure:"group_id"`
	ClientID         string        `mapstructure:"client_id"`
	DialTimeout      time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout"`
	RequiredAcks     int           `mapstructure:"required_acks"`
	MaxAttempts      int           `mapstructure:"max_attempts"`
	// Topic names
	Topics KafkaTopics `mapstructure:"topics"`
}

type KafkaTopics struct {
	LivestreamStarted  string `mapstructure:"livestream_started"`
	LivestreamEnded    string `mapstructure:"livestream_ended"`
	LivestreamViewer   string `mapstructure:"livestream_viewer"`
	GiftSent           string `mapstructure:"gift_sent"`
	ChatMessage        string `mapstructure:"chat_message"`
	PKBattleResult     string `mapstructure:"pk_battle_result"`
	PollCreated        string `mapstructure:"poll_created"`
	NotificationFanout string `mapstructure:"notification_fanout"`
}

type JWTConfig struct {
	Secret          string        `mapstructure:"secret"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	Issuer          string        `mapstructure:"issuer"`
}

type CDNConfig struct {
	// Base URL for the CDN serving HLS files
	BaseURL    string `mapstructure:"base_url"`
	// Signed URL secret for protected streams
	SigningKey  string `mapstructure:"signing_key"`
	// How long a signed URL is valid
	URLTTLSeconds int `mapstructure:"url_ttl_seconds"`
}

type ModerationConfig struct {
	// Enable automated profanity filter
	ProfanityFilterEnabled bool     `mapstructure:"profanity_filter_enabled"`
	// Max messages per minute per user
	RateLimitMsgPerMin int          `mapstructure:"rate_limit_msg_per_min"`
	// Banned keyword list (comma-separated in env)
	BannedKeywords     []string     `mapstructure:"banned_keywords"`
	// AutoBan after N violations
	AutoBanThreshold   int          `mapstructure:"auto_ban_threshold"`
}

// Load reads configuration from file and environment variables.
func Load() (*Config, error) {
	v := viper.New()

	// Default values
	setDefaults(v)

	// Config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/livestream-service/")
	v.AddConfigPath("$HOME/.livestream-service")
	v.AddConfigPath(".")

	// Environment variable overrides — all prefixed with LIVESTREAM_
	v.SetEnvPrefix("LIVESTREAM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// Config file is optional; fall through to env / defaults.
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Apply default renditions if none supplied.
	if len(cfg.HLS.Renditions) == 0 {
		cfg.HLS.Renditions = DefaultRenditions()
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}

// Validate performs basic sanity checks on the config.
func (c *Config) Validate() error {
	if c.RTMP.Port <= 0 || c.RTMP.Port > 65535 {
		return fmt.Errorf("invalid RTMP port: %d", c.RTMP.Port)
	}
	if c.Server.HTTPPort <= 0 || c.Server.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.Server.HTTPPort)
	}
	if c.FFmpeg.BinaryPath == "" {
		return fmt.Errorf("ffmpeg binary path must be set")
	}
	if c.HLS.OutputPath == "" {
		return fmt.Errorf("HLS output path must be set")
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database host must be set")
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("at least one Kafka broker must be configured")
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.http_port", 8085)
	v.SetDefault("server.grpc_port", 9095)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.environment", "development")
	v.SetDefault("server.log_level", "info")

	// RTMP
	v.SetDefault("rtmp.port", 1935)
	v.SetDefault("rtmp.host", "0.0.0.0")
	v.SetDefault("rtmp.app_name", "live")
	v.SetDefault("rtmp.chunk_size", 4096)
	v.SetDefault("rtmp.gop_cache_enabled", true)
	v.SetDefault("rtmp.ping_timeout", "60s")
	v.SetDefault("rtmp.read_timeout", "10s")
	v.SetDefault("rtmp.write_timeout", "10s")
	v.SetDefault("rtmp.max_connections", 1000)
	v.SetDefault("rtmp.stream_key_prefix", "live_")
	v.SetDefault("rtmp.tls_enabled", false)

	// FFmpeg
	v.SetDefault("ffmpeg.binary_path", "/usr/bin/ffmpeg")
	v.SetDefault("ffmpeg.probe_path", "/usr/bin/ffprobe")
	v.SetDefault("ffmpeg.threads", 2)
	v.SetDefault("ffmpeg.preset", "veryfast")
	v.SetDefault("ffmpeg.crf", 23)
	v.SetDefault("ffmpeg.video_codec", "libx264")
	v.SetDefault("ffmpeg.audio_codec", "aac")
	v.SetDefault("ffmpeg.audio_bitrate", "128k")
	v.SetDefault("ffmpeg.max_sessions", 100)
	v.SetDefault("ffmpeg.log_level", "error")
	v.SetDefault("ffmpeg.hw_accel", "")

	// HLS
	v.SetDefault("hls.output_path", "/var/hls")
	v.SetDefault("hls.segment_duration", 2)
	v.SetDefault("hls.playlist_length", 6)
	v.SetDefault("hls.base_url", "http://localhost:8085/hls")
	v.SetDefault("hls.delete_old_segments", true)
	v.SetDefault("hls.key_rotation_interval", 0)

	// WebRTC
	v.SetDefault("webrtc.stun_servers", []string{"stun:stun.l.google.com:19302"})
	v.SetDefault("webrtc.ice_gather_timeout", "5s")
	v.SetDefault("webrtc.max_bitrate", 3000000)
	v.SetDefault("webrtc.simulcast_enabled", true)
	v.SetDefault("webrtc.dtls_verify", true)

	// Database
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "tiktok_livestream")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "5m")
	v.SetDefault("database.conn_max_idle_time", "1m")

	// Redis
	v.SetDefault("redis.addrs", []string{"localhost:6379"})
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("redis.pool_size", 20)
	v.SetDefault("redis.viewer_count_ttl", "5m")
	v.SetDefault("redis.stream_meta_ttl", "24h")
	v.SetDefault("redis.battle_score_ttl", "30m")
	v.SetDefault("redis.pk_battle_ttl", "10m")

	// Kafka
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.group_id", "livestream-service")
	v.SetDefault("kafka.client_id", "livestream-service")
	v.SetDefault("kafka.dial_timeout", "10s")
	v.SetDefault("kafka.read_timeout", "10s")
	v.SetDefault("kafka.write_timeout", "10s")
	v.SetDefault("kafka.required_acks", -1)
	v.SetDefault("kafka.max_attempts", 3)
	v.SetDefault("kafka.topics.livestream_started", "livestream.started")
	v.SetDefault("kafka.topics.livestream_ended", "livestream.ended")
	v.SetDefault("kafka.topics.livestream_viewer", "livestream.viewer")
	v.SetDefault("kafka.topics.gift_sent", "livestream.gift_sent")
	v.SetDefault("kafka.topics.chat_message", "livestream.chat_message")
	v.SetDefault("kafka.topics.pk_battle_result", "livestream.pk_battle_result")
	v.SetDefault("kafka.topics.poll_created", "livestream.poll_created")
	v.SetDefault("kafka.topics.notification_fanout", "notification.fanout")

	// JWT
	v.SetDefault("jwt.secret", "change-me-in-production")
	v.SetDefault("jwt.access_token_ttl", "15m")
	v.SetDefault("jwt.refresh_token_ttl", "168h")
	v.SetDefault("jwt.issuer", "tiktok-clone")

	// CDN
	v.SetDefault("cdn.base_url", "http://localhost:8085/hls")
	v.SetDefault("cdn.url_ttl_seconds", 3600)

	// Moderation
	v.SetDefault("moderation.profanity_filter_enabled", true)
	v.SetDefault("moderation.rate_limit_msg_per_min", 30)
	v.SetDefault("moderation.auto_ban_threshold", 5)
}
