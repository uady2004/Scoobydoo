package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/tiktok-clone/livestream-service/internal/config"
	"github.com/tiktok-clone/livestream-service/internal/handlers"
	"github.com/tiktok-clone/livestream-service/internal/repositories"
	"github.com/tiktok-clone/livestream-service/internal/services"
)

func main() {
	// ── Logger ────────────────────────────────────────────────────────────────
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx := context.Background()

	// ── Database ──────────────────────────────────────────────────────────────
	pool, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		logger.Fatal("failed to create db pool", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal("database ping failed", zap.Error(err))
	}
	logger.Info("database connected")

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.RedisAddr(),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
		PoolSize:     cfg.Redis.PoolSize,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("redis ping failed", zap.Error(err))
	}
	defer rdb.Close()
	logger.Info("redis connected", zap.String("addr", cfg.Redis.RedisAddr()))

	// ── Kafka ─────────────────────────────────────────────────────────────────
	kafkaWriter := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Kafka.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequiredAcks(cfg.Kafka.RequiredAcks),
		MaxAttempts:  cfg.Kafka.MaxAttempts,
		WriteTimeout: cfg.Kafka.WriteTimeout,
	}
	defer kafkaWriter.Close()
	logger.Info("kafka writer ready", zap.Strings("brokers", cfg.Kafka.Brokers))

	// ── Wiring ────────────────────────────────────────────────────────────────
	repo := repositories.NewLivestreamRepository(pool)
	streamSvc := services.NewStreamService(repo, rdb, kafkaWriter, cfg, logger)

	// WebSocket hub — manages all stream rooms.
	hub := handlers.NewHub(streamSvc, logger)

	// HTTP handlers.
	streamHandler := handlers.NewStreamHandler(streamSvc, hub, logger)
	giftHandler := handlers.NewGiftHandler(kafkaWriter, logger)

	// ── Router ────────────────────────────────────────────────────────────────
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "livestream"})
	})

	// WebSocket endpoint: GET /ws/:streamId?token=<jwt>&user_id=<id>&username=<name>
	router.GET("/ws/:streamId", hub.ServeWS)

	// REST API
	api := router.Group("/api/v1/streams")
	streamHandler.RegisterRoutes(api)

	gifts := router.Group("/api/v1/gifts")
	giftHandler.RegisterRoutes(gifts)

	// ── HTTP server ───────────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.Info("livestream-service starting",
			zap.String("addr", addr),
			zap.String("env", cfg.Server.Environment),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down livestream-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced shutdown", zap.Error(err))
	}
	logger.Info("livestream-service stopped")
}
