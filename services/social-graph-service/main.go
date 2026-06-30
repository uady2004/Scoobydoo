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

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/social-graph-service/internal/config"
	"github.com/tiktok-clone/social-graph-service/internal/handlers"
	"github.com/tiktok-clone/social-graph-service/internal/repositories"
	"github.com/tiktok-clone/social-graph-service/internal/services"
	"github.com/tiktok-clone/social-graph-service/internal/workers"
)

func main() {
	// -------------------------------------------------------------------------
	// Logger
	// -------------------------------------------------------------------------
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	// -------------------------------------------------------------------------
	// Configuration
	// -------------------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}
	logger.Info("configuration loaded",
		zap.Int("server_port", cfg.Server.Port),
		zap.String("db_host", cfg.Database.Host),
		zap.String("redis_addr", cfg.Redis.Addr),
	)

	// -------------------------------------------------------------------------
	// PostgreSQL connection pool
	// -------------------------------------------------------------------------
	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		logger.Fatal("failed to parse database DSN", zap.Error(err))
	}
	poolCfg.MaxConns = cfg.Database.MaxConns
	poolCfg.MinConns = cfg.Database.MinConns
	poolCfg.MaxConnLifetime = cfg.Database.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.Database.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal("database ping failed", zap.Error(err))
	}
	logger.Info("database connection established")

	// -------------------------------------------------------------------------
	// Redis client
	// -------------------------------------------------------------------------
	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("redis ping failed", zap.Error(err))
	}
	defer redisClient.Close()
	logger.Info("redis connection established")

	// -------------------------------------------------------------------------
	// Kafka sync producer
	// -------------------------------------------------------------------------
	producerCfg := sarama.NewConfig()
	producerCfg.Version = sarama.V2_6_0_0
	producerCfg.Producer.Return.Successes = true
	producerCfg.Producer.Return.Errors = true
	producerCfg.Producer.RequiredAcks = sarama.WaitForAll
	producerCfg.Producer.Retry.Max = cfg.Kafka.RetryMax
	producerCfg.Producer.Retry.Backoff = cfg.Kafka.RetryBackoff

	producer, err := sarama.NewSyncProducer(cfg.Kafka.Brokers, producerCfg)
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()
	logger.Info("kafka producer ready", zap.Strings("brokers", cfg.Kafka.Brokers))

	// -------------------------------------------------------------------------
	// Kafka consumer group (for event_processor)
	// -------------------------------------------------------------------------
	consumerGroup, err := workers.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroupID)
	if err != nil {
		logger.Fatal("failed to create kafka consumer group", zap.Error(err))
	}

	eventTopics := []string{
		cfg.Kafka.Topics.UserFollowed,
		cfg.Kafka.Topics.UserUnfollowed,
	}
	eventProcessor := workers.NewEventProcessor(
		consumerGroup,
		redisClient,
		logger,
		eventTopics,
		cfg.Redis.CounterTTL,
	)

	// -------------------------------------------------------------------------
	// Repository / Service / Handler wiring
	// -------------------------------------------------------------------------
	graphRepo := repositories.NewGraphRepository(pool, logger)

	suggestionSvc := services.NewSuggestionService(
		graphRepo,
		redisClient,
		logger,
		cfg.Service.MaxSuggestions,
		cfg.Service.BFSDepth,
		cfg.Redis.SuggestionTTL,
	)

	topics := services.TopicConfig{
		UserFollowed:   cfg.Kafka.Topics.UserFollowed,
		UserUnfollowed: cfg.Kafka.Topics.UserUnfollowed,
		FeedInvalidate: cfg.Kafka.Topics.FeedInvalidate,
		NotifyFollow:   cfg.Kafka.Topics.NotifyFollow,
	}

	socialSvc := services.NewSocialService(
		graphRepo,
		redisClient,
		producer,
		suggestionSvc,
		logger,
		topics,
		cfg.Service.NotificationServiceURL,
		cfg.Service.DefaultPageSize,
		cfg.Service.MaxPageSize,
		cfg.Redis.CounterTTL,
	)

	socialHandler := handlers.NewSocialHandler(socialSvc, logger)

	// -------------------------------------------------------------------------
	// HTTP server
	// -------------------------------------------------------------------------
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginZapLogger(logger))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// All social-graph routes are grouped under /api/v1.
	// In production a JWT middleware would be inserted here.
	v1 := router.Group("/api/v1")
	socialHandler.RegisterRoutes(v1)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// -------------------------------------------------------------------------
	// Start background workers
	// -------------------------------------------------------------------------
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go func() {
		logger.Info("event_processor: starting")
		eventProcessor.Run(workerCtx)
		logger.Info("event_processor: stopped")
	}()

	// -------------------------------------------------------------------------
	// Start HTTP server in background
	// -------------------------------------------------------------------------
	go func() {
		logger.Info("http server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("http server error", zap.Error(err))
		}
	}()

	// -------------------------------------------------------------------------
	// Graceful shutdown
	// -------------------------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutdown signal received", zap.String("signal", sig.String()))

	// Stop accepting new HTTP connections.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", zap.Error(err))
	}

	// Stop the event processor.
	workerCancel()
	if err := eventProcessor.Close(); err != nil {
		logger.Error("event_processor close error", zap.Error(err))
	}

	logger.Info("social-graph-service stopped cleanly")
}

// ginZapLogger returns a gin middleware that logs requests with zap.
func ginZapLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
