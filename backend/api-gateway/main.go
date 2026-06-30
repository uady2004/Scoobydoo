package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tiktok-clone/api-gateway/internal/config"
	"github.com/tiktok-clone/api-gateway/internal/handlers"
	"github.com/tiktok-clone/api-gateway/internal/middleware"
)

func main() {
	// ── Configuration ────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[main] failed to load config: %v", err)
	}

	// ── Gin mode ─────────────────────────────────────────────────────────────
	gin.SetMode(cfg.Server.Mode)

	// ── Redis client ─────────────────────────────────────────────────────────
	rdb := newRedisClient(&cfg.Redis)
	if err := pingRedis(rdb); err != nil {
		log.Printf("[main] WARNING: Redis ping failed: %v (continuing with degraded rate limiting)", err)
	}

	// ── Kafka audit logger ───────────────────────────────────────────────────
	auditLogger, err := middleware.NewAuditLogger(&cfg.Kafka)
	if err != nil {
		log.Fatalf("[main] failed to create audit logger: %v", err)
	}
	defer func() {
		if err := auditLogger.Close(); err != nil {
			log.Printf("[main] error closing audit logger: %v", err)
		}
	}()

	// ── JWT validator ────────────────────────────────────────────────────────
	jwtValidator, err := middleware.NewJWTValidator(&cfg.JWT, rdb)
	if err != nil {
		// JWT is required in production; treat it as fatal.
		log.Fatalf("[main] failed to initialize JWT validator: %v", err)
	}

	// ── OAuth validator ───────────────────────────────────────────────────────
	oauthValidator := middleware.NewOAuthValidator(&cfg.OAuth)

	// ── RBAC ─────────────────────────────────────────────────────────────────
	rbac := middleware.NewRBACMiddleware()

	// ── Rate limiter ──────────────────────────────────────────────────────────
	rateLimiter := middleware.NewRateLimiter(rdb, &cfg.RateLimit)

	// ── WAF ───────────────────────────────────────────────────────────────────
	wafMiddleware := middleware.NewWAFMiddleware(&cfg.WAF)

	// ── Circuit breaker manager ───────────────────────────────────────────────
	cbManager := middleware.NewCircuitBreakerManager(&cfg.CircuitBreaker)

	// ── Proxy router ─────────────────────────────────────────────────────────
	proxyRouter, err := handlers.NewProxyRouter(&cfg.Services, cbManager)
	if err != nil {
		log.Fatalf("[main] failed to create proxy router: %v", err)
	}

	// ── Gin engine ───────────────────────────────────────────────────────────
	engine := gin.New()

	// Trusted proxies (for correct ClientIP behind load balancer).
	if err := engine.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		log.Fatalf("[main] setting trusted proxies: %v", err)
	}

	// Global recovery middleware — catch panics and return 500.
	engine.Use(gin.RecoveryWithWriter(os.Stderr, func(c *gin.Context, err interface{}) {
		log.Printf("[main] PANIC recovered: %v", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "an unexpected error occurred",
		})
	}))

	// Request logger (structured, skips /health and /ready probes).
	engine.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: structuredLogFormatter,
		SkipPaths: []string{"/health", "/ready"},
	}))

	// CORS — configure allowed origins from environment.
	engine.Use(cors.New(buildCORSConfig()))

	// Security headers applied to every response.
	engine.Use(securityHeadersMiddleware())

	// Request-ID injection.
	engine.Use(requestIDMiddleware())

	// Register all routes (WAF + audit logger applied inside RegisterRoutes).
	proxyRouter.RegisterRoutes(
		engine,
		jwtValidator,
		rbac,
		rateLimiter,
		wafMiddleware,
		auditLogger,
	)

	// OAuth-specific registration (separate handler group, not proxied).
	registerOAuthRoutes(engine, oauthValidator)

	// 404 / 405 fallback handlers.
	engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": fmt.Sprintf("route %s %s not found", c.Request.Method, c.Request.URL.Path),
		})
	})
	engine.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error":   "method_not_allowed",
			"message": fmt.Sprintf("method %s not allowed on %s", c.Request.Method, c.Request.URL.Path),
		})
	})

	// ── HTTP server ───────────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
		// Limit request header size.
		MaxHeaderBytes: 1 << 20, // 1 MiB
	}

	// ── Start server ──────────────────────────────────────────────────────────
	go func() {
		log.Printf("[main] API Gateway listening on %s (mode=%s)", addr, cfg.Server.Mode)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[main] server error: %v", err)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[main] received signal %s; shutting down...", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[main] forced shutdown: %v", err)
	} else {
		log.Println("[main] server exited cleanly")
	}
}

// ── Redis ─────────────────────────────────────────────────────────────────────

func newRedisClient(cfg *config.RedisConfig) *redis.Client {
	opts := &redis.Options{
		Addr:         cfg.Addresses[0], // primary address
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
	}
	return redis.NewClient(opts)
}

func pingRedis(rdb *redis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return rdb.Ping(ctx).Err()
}

// ── OAuth route registration ─────────────────────────────────────────────────

// registerOAuthRoutes attaches OAuth introspection / status routes.
// Token exchange happens inside the user-service; these routes expose
// provider validation as a dedicated middleware check.
func registerOAuthRoutes(r *gin.Engine, v *middleware.OAuthValidator) {
	oauth := r.Group("/api/v1/auth/oauth")
	{
		// Validates a Google ID token and returns normalized claims (used by
		// mobile clients before calling the full register/login flow).
		oauth.POST("/google/validate", v.GoogleIDToken(), func(c *gin.Context) {
			claims, _ := c.Get("oauth_claims")
			c.JSON(http.StatusOK, gin.H{
				"valid":  true,
				"claims": claims,
			})
		})

		// Validates an Apple ID token.
		oauth.POST("/apple/validate", v.AppleIDToken(), func(c *gin.Context) {
			claims, _ := c.Get("oauth_claims")
			c.JSON(http.StatusOK, gin.H{
				"valid":  true,
				"claims": claims,
			})
		})

		// Multi-provider endpoint driven by X-OAuth-Provider header.
		oauth.POST("/validate", v.MultiProviderOAuth(), func(c *gin.Context) {
			claims, _ := c.Get("oauth_claims")
			c.JSON(http.StatusOK, gin.H{
				"valid":    true,
				"claims":   claims,
				"provider": c.GetHeader("X-OAuth-Provider"),
			})
		})
	}
}

// ── Middleware helpers ────────────────────────────────────────────────────────

// securityHeadersMiddleware sets standard security response headers.
func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'none'")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	}
}

// requestIDMiddleware injects or propagates an X-Request-ID header.
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		c.Set("request_id", reqID)
		c.Header("X-Request-ID", reqID)
		c.Next()
	}
}

// buildCORSConfig returns a production-ready CORS configuration.
// Allowed origins should be restricted further via CORS_ALLOWED_ORIGINS env var.
func buildCORSConfig() cors.Config {
	extra := os.Getenv("CORS_ALLOWED_ORIGINS")
	if extra == "*" {
		return cors.Config{
			AllowAllOrigins:  true,
			AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID", "X-OAuth-Provider", "X-Device-ID", "Accept", "Accept-Language"},
			ExposeHeaders:    []string{"X-Request-ID"},
			AllowCredentials: false,
			MaxAge:           12 * time.Hour,
		}
	}

	allowedOrigins := []string{
		"https://tiktok-clone.example.com",
		"https://www.tiktok-clone.example.com",
	}
	if extra != "" {
		allowedOrigins = append(allowedOrigins, extra)
	}

	return cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodHead,
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
			"X-Request-ID",
			"X-OAuth-Provider",
			"X-Device-ID",
			"Accept",
			"Accept-Language",
		},
		ExposeHeaders: []string{
			"X-Request-ID",
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-RateLimit-Reset",
			"Retry-After",
		},
		AllowCredentials: false, // Do not allow credentials to avoid CSRF
		MaxAge:           12 * time.Hour,
	}
}

// structuredLogFormatter formats Gin access logs as a one-liner with key fields.
func structuredLogFormatter(params gin.LogFormatterParams) string {
	statusColor := params.StatusCodeColor()
	resetColor := params.ResetColor()
	return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s | %-7s %s\n",
		params.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, params.StatusCode, resetColor,
		params.Latency,
		params.ClientIP,
		params.Method,
		params.Path,
	)
}
