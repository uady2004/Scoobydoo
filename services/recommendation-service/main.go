package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/tiktok-clone/recommendation-service/internal/config"
	"github.com/tiktok-clone/recommendation-service/internal/handlers"
	"github.com/tiktok-clone/recommendation-service/internal/services"
	"github.com/tiktok-clone/recommendation-service/internal/workers"
)

func main() {
	// ---- Configuration -------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// ---- Logger --------------------------------------------------------------
	logger := buildLogger(cfg.Observability.LogLevel)
	defer logger.Sync() //nolint:errcheck

	logger.Info("recommendation-service starting",
		zap.Int("grpc_port", cfg.Server.GRPCPort),
		zap.Int("http_port", cfg.Server.HTTPPort),
	)

	// ---- Redis ---------------------------------------------------------------
	rdb := buildRedisClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("redis ping failed", zap.Error(err))
	}
	cancel()
	logger.Info("redis connected")

	// ---- Elasticsearch -------------------------------------------------------
	esCfg := elasticsearch.Config{
		Addresses: cfg.Elasticsearch.Addresses,
		Username:  cfg.Elasticsearch.Username,
		Password:  cfg.Elasticsearch.Password,
		RetryOnStatus: []int{502, 503, 504},
		MaxRetries:    cfg.Elasticsearch.MaxRetries,
	}
	esClient, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		logger.Fatal("elasticsearch client creation failed", zap.Error(err))
	}
	logger.Info("elasticsearch client initialised",
		zap.Strings("addresses", cfg.Elasticsearch.Addresses))

	// ---- Service layer -------------------------------------------------------
	featureStore := services.NewFeatureStore(cfg, rdb, logger)
	embeddingSvc := services.NewEmbeddingService(cfg, esClient, rdb, logger)
	candidateGen := services.NewCandidateGenerator(cfg, esClient, rdb, logger)
	abTesting := services.NewABTestingService(&cfg.ABTesting, rdb, logger)
	rankingSvc := services.NewRankingService(cfg, featureStore, embeddingSvc, rdb, logger)

	// ---- Handler -------------------------------------------------------------
	handler := handlers.NewRecommendationHandler(
		cfg,
		candidateGen,
		rankingSvc,
		featureStore,
		embeddingSvc,
		abTesting,
		logger,
	)

	// ---- Model update worker -------------------------------------------------
	modelWorker, err := workers.NewModelUpdateWorker(cfg, rdb, featureStore, logger)
	if err != nil {
		logger.Fatal("model update worker creation failed", zap.Error(err))
	}

	// ---- HTTP server ---------------------------------------------------------
	if cfg.Observability.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ---- gRPC server ---------------------------------------------------------
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(16 * 1024 * 1024),
		grpc.MaxSendMsgSize(16 * 1024 * 1024),
	)
	reflection.Register(grpcServer)

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	if err != nil {
		logger.Fatal("grpc listen failed", zap.Error(err))
	}

	// ---- Start all components ------------------------------------------------
	runCtx, runCancel := context.WithCancel(context.Background())

	go func() {
		logger.Info("http server listening", zap.String("addr", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", zap.Error(err))
		}
	}()

	go func() {
		logger.Info("grpc server listening", zap.String("addr", grpcLis.Addr().String()))
		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Error("grpc server error", zap.Error(err))
		}
	}()

	go func() {
		if err := modelWorker.Start(runCtx); err != nil && err != context.Canceled {
			logger.Error("model update worker error", zap.Error(err))
		}
	}()

	// ---- Graceful shutdown ---------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received; draining")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.GracePeriod)
	defer shutdownCancel()

	// Stop HTTP.
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", zap.Error(err))
	}
	// Stop gRPC.
	grpcServer.GracefulStop()
	// Stop model worker.
	runCancel()
	modelWorker.Stop()
	// Stop A/B testing refresh loop.
	abTesting.Stop()
	// Close Redis.
	rdb.Close() //nolint:errcheck

	logger.Info("recommendation-service stopped")
}

// buildLogger creates a zap logger configured to the requested level.
func buildLogger(level string) *zap.Logger {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	logCfg := zap.Config{
		Level:             zap.NewAtomicLevelAt(zapLevel),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: true,
		Sampling:          nil,
		Encoding:          "json",
		EncoderConfig:     encoderCfg,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
	}
	logger, _ := logCfg.Build()
	return logger
}

// buildRedisClient creates a universal Redis client (single-node or cluster).
func buildRedisClient(cfg *config.Config) redis.UniversalClient {
	if len(cfg.Redis.Addrs) > 1 {
		return redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        cfg.Redis.Addrs,
			Password:     cfg.Redis.Password,
			PoolSize:     cfg.Redis.PoolSize,
			ReadTimeout:  cfg.Redis.ReadTimeout,
			WriteTimeout: cfg.Redis.WriteTimeout,
		})
	}
	addr := cfg.Redis.Addr
	if len(cfg.Redis.Addrs) == 1 {
		addr = cfg.Redis.Addrs[0]
	}
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
}
