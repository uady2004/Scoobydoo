package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the notification service.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	Firebase FirebaseConfig
	SendGrid SendGridConfig
	Twilio   TwilioConfig
	JWT      JWTConfig
}

type ServerConfig struct {
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
	MaxConns        int32         `mapstructure:"max_conns"`
	MinConns        int32         `mapstructure:"min_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `mapstructure:"max_conn_idle_time"`
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d pool_min_conns=%d",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode, d.MaxConns, d.MinConns,
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

type KafkaConfig struct {
	Brokers         []string `mapstructure:"brokers"`
	GroupID         string   `mapstructure:"group_id"`
	AutoOffsetReset string   `mapstructure:"auto_offset_reset"`
	// Topics to subscribe to.
	Topics KafkaTopics `mapstructure:"topics"`
}

type KafkaTopics struct {
	VideoLiked      string `mapstructure:"video_liked"`
	UserFollowed    string `mapstructure:"user_followed"`
	CommentCreated  string `mapstructure:"comment_created"`
	GiftSent        string `mapstructure:"gift_sent"`
	OrderCreated    string `mapstructure:"order_created"`
	VideoMentioned  string `mapstructure:"video_mentioned"`
	LiveStreamStart string `mapstructure:"livestream_start"`
}

// AllTopics returns a slice of all configured Kafka topic names.
func (t KafkaTopics) AllTopics() []string {
	return []string{
		t.VideoLiked,
		t.UserFollowed,
		t.CommentCreated,
		t.GiftSent,
		t.OrderCreated,
		t.VideoMentioned,
		t.LiveStreamStart,
	}
}

// FirebaseConfig holds credentials for Firebase Admin SDK (FCM v1).
type FirebaseConfig struct {
	// CredentialsFile is the path to the service-account JSON file.
	// Mutually exclusive with CredentialsJSON.
	CredentialsFile string `mapstructure:"credentials_file"`
	// CredentialsJSON is the raw JSON string of the service account.
	// Takes precedence over CredentialsFile when non-empty.
	CredentialsJSON string `mapstructure:"credentials_json"`
	// ProjectID is the Firebase/GCP project identifier.
	ProjectID string `mapstructure:"project_id"`
}

// SendGridConfig holds SendGrid API credentials and template IDs.
type SendGridConfig struct {
	APIKey string `mapstructure:"api_key"`
	// FromEmail is the verified sender address.
	FromEmail string `mapstructure:"from_email"`
	FromName  string `mapstructure:"from_name"`
	// Dynamic template IDs for each notification type.
	Templates SendGridTemplates `mapstructure:"templates"`
}

type SendGridTemplates struct {
	EmailVerification string `mapstructure:"email_verification"`
	PasswordReset     string `mapstructure:"password_reset"`
	WeeklyDigest      string `mapstructure:"weekly_digest"`
	GiftReceived      string `mapstructure:"gift_received"`
	OrderConfirmation string `mapstructure:"order_confirmation"`
	OrderShipped      string `mapstructure:"order_shipped"`
}

// TwilioConfig holds Twilio API credentials.
type TwilioConfig struct {
	AccountSID  string `mapstructure:"account_sid"`
	AuthToken   string `mapstructure:"auth_token"`
	FromNumber  string `mapstructure:"from_number"`
	// MessagingServiceSID is optional; when set it overrides FromNumber.
	MessagingServiceSID string `mapstructure:"messaging_service_sid"`
	// OTPExpirySeconds controls how long an OTP code stays valid.
	OTPExpirySeconds int `mapstructure:"otp_expiry_seconds"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

// Load reads configuration from environment variables and optional config file.
// Environment variable names are uppercased with underscores, prefixed with NOTIFICATION_.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.idle_timeout", "60s")

	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_conns", 20)
	v.SetDefault("database.min_conns", 2)
	v.SetDefault("database.max_conn_lifetime", "1h")
	v.SetDefault("database.max_conn_idle_time", "30m")

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")

	v.SetDefault("kafka.group_id", "notification-service")
	v.SetDefault("kafka.auto_offset_reset", "earliest")
	v.SetDefault("kafka.topics.video_liked", "video.liked")
	v.SetDefault("kafka.topics.user_followed", "user.followed")
	v.SetDefault("kafka.topics.comment_created", "comment.created")
	v.SetDefault("kafka.topics.gift_sent", "gift.sent")
	v.SetDefault("kafka.topics.order_created", "order.created")
	v.SetDefault("kafka.topics.video_mentioned", "video.mentioned")
	v.SetDefault("kafka.topics.livestream_start", "livestream.started")

	v.SetDefault("twilio.otp_expiry_seconds", 300)

	v.SetDefault("sendgrid.from_name", "TikTok Clone")

	// Config file (optional)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/notification-service/")
	v.AddConfigPath(".")
	_ = v.ReadInConfig() // Ignore error; env vars take precedence

	// Environment variables
	v.SetEnvPrefix("NOTIFICATION")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit bindings for nested keys (AutomaticEnv alone is unreliable for nested structs)
	_ = v.BindEnv("database.host", "NOTIFICATION_DATABASE_HOST")
	_ = v.BindEnv("database.port", "NOTIFICATION_DATABASE_PORT")
	_ = v.BindEnv("database.user", "NOTIFICATION_DATABASE_USER")
	_ = v.BindEnv("database.password", "NOTIFICATION_DATABASE_PASSWORD")
	_ = v.BindEnv("database.dbname", "NOTIFICATION_DATABASE_DBNAME")
	_ = v.BindEnv("database.sslmode", "NOTIFICATION_DATABASE_SSLMODE")
	_ = v.BindEnv("redis.addr", "NOTIFICATION_REDIS_ADDR")
	_ = v.BindEnv("kafka.brokers", "NOTIFICATION_KAFKA_BROKERS")
	_ = v.BindEnv("server.port", "NOTIFICATION_SERVER_PORT")
	_ = v.BindEnv("firebase.project_id", "NOTIFICATION_FIREBASE_PROJECT_ID")
	_ = v.BindEnv("firebase.credentials_json", "NOTIFICATION_FIREBASE_CREDENTIALS_JSON")
	_ = v.BindEnv("sendgrid.api_key", "NOTIFICATION_SENDGRID_API_KEY")
	_ = v.BindEnv("sendgrid.from_email", "NOTIFICATION_SENDGRID_FROM_EMAIL")
	_ = v.BindEnv("twilio.account_sid", "NOTIFICATION_TWILIO_ACCOUNT_SID")
	_ = v.BindEnv("twilio.auth_token", "NOTIFICATION_TWILIO_AUTH_TOKEN")
	_ = v.BindEnv("twilio.from_number", "NOTIFICATION_TWILIO_FROM_NUMBER")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if cfg.Database.Password == "" {
		return fmt.Errorf("database.password is required")
	}
	if cfg.Database.DBName == "" {
		return fmt.Errorf("database.dbname is required")
	}
	if len(cfg.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers is required")
	}
	// Firebase, SendGrid, Twilio are optional in dev mode; missing credentials disable those channels.
	return nil
}
