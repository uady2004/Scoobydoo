package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server         ServerConfig         `mapstructure:"server"`
	JWT            JWTConfig            `mapstructure:"jwt"`
	OAuth          OAuthConfig          `mapstructure:"oauth"`
	Redis          RedisConfig          `mapstructure:"redis"`
	Kafka          KafkaConfig          `mapstructure:"kafka"`
	Services       ServicesConfig       `mapstructure:"services"`
	WAF            WAFConfig            `mapstructure:"waf"`
	RateLimit      RateLimitConfig      `mapstructure:"rate_limit"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
}

type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	TrustedProxies  []string      `mapstructure:"trusted_proxies"`
	Mode            string        `mapstructure:"mode"` // debug | release | test
}

type JWTConfig struct {
	PublicKeyPath string        `mapstructure:"public_key_path"`
	PublicKeyPEM  string        `mapstructure:"public_key_pem"`
	Issuer        string        `mapstructure:"issuer"`
	Audience      string        `mapstructure:"audience"`
	TokenExpiry   time.Duration `mapstructure:"token_expiry"`
	RefreshExpiry time.Duration `mapstructure:"refresh_expiry"`
}

type OAuthConfig struct {
	Google GoogleOAuthConfig
	Apple  AppleOAuthConfig
}

type GoogleOAuthConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

type AppleOAuthConfig struct {
	ClientID   string `mapstructure:"client_id"`
	TeamID     string `mapstructure:"team_id"`
	KeyID      string `mapstructure:"key_id"`
	PrivateKey string `mapstructure:"private_key"`
}

type RedisConfig struct {
	Addresses    []string      `mapstructure:"addresses"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	MaxRetries   int           `mapstructure:"max_retries"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
}

type KafkaConfig struct {
	Brokers          []string `mapstructure:"brokers"`
	AuditTopic       string   `mapstructure:"audit_topic"`
	SecurityProtocol string   `mapstructure:"security_protocol"`
	SASLMechanism    string   `mapstructure:"sasl_mechanism"`
	SASLUsername     string   `mapstructure:"sasl_username"`
	SASLPassword     string   `mapstructure:"sasl_password"`
	TLSEnabled       bool     `mapstructure:"tls_enabled"`
}

type ServiceEndpoint struct {
	Addresses   []string      `mapstructure:"addresses"`
	Timeout     time.Duration `mapstructure:"timeout"`
	MaxIdleConn int           `mapstructure:"max_idle_conn"`
	HealthPath  string        `mapstructure:"health_path"`
}

type ServicesConfig struct {
	Auth         ServiceEndpoint `mapstructure:"auth"`
	User         ServiceEndpoint `mapstructure:"user"`
	Video        ServiceEndpoint `mapstructure:"video"`
	Feed         ServiceEndpoint `mapstructure:"feed"`
	Comment      ServiceEndpoint `mapstructure:"comment"`
	Like         ServiceEndpoint `mapstructure:"like"`
	Follow       ServiceEndpoint `mapstructure:"follow"`
	Search       ServiceEndpoint `mapstructure:"search"`
	Notification ServiceEndpoint `mapstructure:"notification"`
	Analytics    ServiceEndpoint `mapstructure:"analytics"`
	Live         ServiceEndpoint `mapstructure:"live"`
	Interaction  ServiceEndpoint `mapstructure:"interaction"`
	Reporting    ServiceEndpoint `mapstructure:"reporting"`
	Wallet       ServiceEndpoint `mapstructure:"wallet"`
	Messaging    ServiceEndpoint `mapstructure:"messaging"`
	Ecommerce    ServiceEndpoint `mapstructure:"ecommerce"`
}

type WAFConfig struct {
	Enabled            bool     `mapstructure:"enabled"`
	BlockSQLi          bool     `mapstructure:"block_sqli"`
	BlockXSS           bool     `mapstructure:"block_xss"`
	BlockPathTraversal bool     `mapstructure:"block_path_traversal"`
	MaxBodySize        int64    `mapstructure:"max_body_size"`
	AllowedHosts       []string `mapstructure:"allowed_hosts"`
	BlockedIPs         []string `mapstructure:"blocked_ips"`
}

type RateLimitConfig struct {
	UserRequestsPerMinute int           `mapstructure:"user_requests_per_minute"`
	IPRequestsPerMinute   int           `mapstructure:"ip_requests_per_minute"`
	WindowSize            time.Duration `mapstructure:"window_size"`
	BurstMultiplier       float64       `mapstructure:"burst_multiplier"`
}

type CircuitBreakerConfig struct {
	MaxRequests  uint32        `mapstructure:"max_requests"`
	Interval     time.Duration `mapstructure:"interval"`
	Timeout      time.Duration `mapstructure:"timeout"`
	FailureRatio float64       `mapstructure:"failure_ratio"`
	MinRequests  uint32        `mapstructure:"min_requests"`
}

// Load reads configuration from environment variables and config files.
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Environment variable configuration
	v.SetEnvPrefix("TIKTOK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Optionally read from config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/tiktok-gateway/")
	v.AddConfigPath("$HOME/.tiktok-gateway")
	v.AddConfigPath(".")

	// Read config file if it exists (not required)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.shutdown_timeout", 15*time.Second)
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.trusted_proxies", []string{"127.0.0.1"})

	// JWT
	v.SetDefault("jwt.issuer", "tiktok-clone")
	v.SetDefault("jwt.audience", "tiktok-api")
	v.SetDefault("jwt.token_expiry", 15*time.Minute)
	v.SetDefault("jwt.refresh_expiry", 7*24*time.Hour)

	// Redis
	v.SetDefault("redis.addresses", []string{"localhost:6379"})
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.max_retries", 3)
	v.SetDefault("redis.dial_timeout", 5*time.Second)
	v.SetDefault("redis.read_timeout", 3*time.Second)
	v.SetDefault("redis.write_timeout", 3*time.Second)
	v.SetDefault("redis.pool_size", 10)

	// Kafka
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.audit_topic", "audit-logs")
	v.SetDefault("kafka.security_protocol", "PLAINTEXT")

	// WAF
	v.SetDefault("waf.enabled", true)
	v.SetDefault("waf.block_sqli", true)
	v.SetDefault("waf.block_xss", true)
	v.SetDefault("waf.block_path_traversal", true)
	v.SetDefault("waf.max_body_size", int64(10*1024*1024)) // 10MB

	// Rate limiting
	v.SetDefault("rate_limit.user_requests_per_minute", 100)
	v.SetDefault("rate_limit.ip_requests_per_minute", 1000)
	v.SetDefault("rate_limit.window_size", time.Minute)
	v.SetDefault("rate_limit.burst_multiplier", 1.5)

	// Circuit breaker
	v.SetDefault("circuit_breaker.max_requests", uint32(5))
	v.SetDefault("circuit_breaker.interval", 60*time.Second)
	v.SetDefault("circuit_breaker.timeout", 30*time.Second)
	v.SetDefault("circuit_breaker.failure_ratio", 0.6)
	v.SetDefault("circuit_breaker.min_requests", uint32(10))

	// Service endpoints with defaults (ports match each service's Dockerfile EXPOSE).
	services := []string{"auth", "user", "video", "feed", "comment", "like", "follow", "search", "notification", "analytics", "live", "interaction", "reporting", "wallet", "messaging", "ecommerce"}
	ports := map[string]string{
		"auth":         "8081", // auth-service
		"user":         "8082", // user-service
		"video":        "8086", // video-service
		"feed":         "8084", // feed-service
		"comment":      "8083", // interaction-service (handles comments)
		"like":         "8083", // interaction-service (handles likes)
		"follow":       "8094", // social-graph-service
		"search":       "8087", // search-service
		"notification": "8088", // notification-service
		"analytics":    "8095", // analytics-service
		"live":         "8085", // livestream-service
		"interaction":  "8083", // interaction-service
		"reporting":    "8092", // reporting-service
		"wallet":       "8090", // wallet-service
		"messaging":    "8096", // messaging-service
		"ecommerce":    "8091", // ecommerce-service
	}
	for _, svc := range services {
		v.SetDefault(fmt.Sprintf("services.%s.addresses", svc), []string{fmt.Sprintf("http://localhost:%s", ports[svc])})
		v.SetDefault(fmt.Sprintf("services.%s.timeout", svc), 10*time.Second)
		v.SetDefault(fmt.Sprintf("services.%s.max_idle_conn", svc), 100)
		v.SetDefault(fmt.Sprintf("services.%s.health_path", svc), "/health")
	}
}

func validate(cfg *Config) error {
	if cfg.JWT.PublicKeyPath == "" && cfg.JWT.PublicKeyPEM == "" {
		// Allow empty in development; warn but don't fail
		fmt.Println("WARNING: No JWT public key configured. JWT validation will fail.")
	}

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.RateLimit.UserRequestsPerMinute < 1 {
		return fmt.Errorf("user rate limit must be positive")
	}

	if cfg.RateLimit.IPRequestsPerMinute < 1 {
		return fmt.Errorf("IP rate limit must be positive")
	}

	return nil
}
