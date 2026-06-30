package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/video-service/internal/config"
	"github.com/tiktok-clone/video-service/internal/handlers"
	"github.com/tiktok-clone/video-service/internal/repositories"
	"github.com/tiktok-clone/video-service/internal/services"
	"github.com/tiktok-clone/video-service/internal/workers"
)

func main() {
	// ---- logger ----------------------------------------------------------------
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialise logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	// ---- config ----------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// ---- database --------------------------------------------------------------
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		logger.Fatal("failed to parse DB DSN", zap.Error(err))
	}
	poolCfg.MaxConns = cfg.Database.MaxConns
	poolCfg.MinConns = cfg.Database.MinConns
	poolCfg.MaxConnLifetime = cfg.Database.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.Database.MaxConnIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		logger.Fatal("database ping failed", zap.Error(err))
	}
	logger.Info("database connected", zap.String("host", cfg.Database.Host))

	// ---- Redis -----------------------------------------------------------------
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("Redis ping failed", zap.Error(err))
	}
	defer rdb.Close()
	logger.Info("Redis connected", zap.String("addr", cfg.Redis.Addr))

	// ---- AWS S3 ----------------------------------------------------------------
	awsOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.S3.Region),
	}
	if cfg.S3.AccessKeyID != "" {
		awsOpts = append(awsOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey, ""),
		))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsOpts...)
	if err != nil {
		logger.Fatal("failed to load AWS config", zap.Error(err))
	}

	s3Opts := []func(*s3.Options){}
	if cfg.S3.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = &cfg.S3.Endpoint
			o.UsePathStyle = cfg.S3.UsePathStyle
		})
	}
	s3Client := s3.NewFromConfig(awsCfg, s3Opts...)
	logger.Info("S3 client initialised", zap.String("bucket", cfg.S3.Bucket))

	// ---- Kafka producer --------------------------------------------------------
	producer, err := workers.NewSyncProducer(cfg)
	if err != nil {
		logger.Fatal("failed to create Kafka producer", zap.Error(err))
	}
	defer producer.Close()
	logger.Info("Kafka producer connected", zap.Strings("brokers", cfg.Kafka.Brokers))

	// ---- repositories ----------------------------------------------------------
	videoRepo := repositories.NewVideoRepository(db)

	// ---- services --------------------------------------------------------------
	uploadSvc := services.NewUploadService(cfg, videoRepo, s3Client, rdb, logger)
	transcodeSvc := services.NewTranscodingService(cfg, videoRepo, s3Client, logger)
	videoSvc := services.NewVideoService(cfg, videoRepo, rdb, producer, logger)

	// ---- HTTP server -----------------------------------------------------------
	gin.SetMode(cfg.Server.Mode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger(logger))

	// Health check.
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "video-service"})
	})

	v1 := router.Group("/api/v1")
	handlers.NewUploadHandler(uploadSvc, logger).RegisterRoutes(v1)
	handlers.NewVideoHandler(videoSvc, logger).RegisterRoutes(v1)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// ---- background workers ----------------------------------------------------
	// Transcoding consumer.
	transcodingWorker, err := workers.NewTranscodingWorker(cfg, transcodeSvc, videoRepo, logger)
	if err != nil {
		logger.Fatal("failed to create transcoding worker", zap.Error(err))
	}

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go func() {
		if err := transcodingWorker.Start(workerCtx); err != nil && err != context.Canceled {
			logger.Error("transcoding worker exited with error", zap.Error(err))
		}
	}()

	// Scheduler.
	schedulerWorker := workers.NewSchedulerWorker(cfg, videoRepo, producer, logger, "")
	if err := schedulerWorker.Start(); err != nil {
		logger.Fatal("failed to start scheduler worker", zap.Error(err))
	}
	defer schedulerWorker.Stop()

	// ---- graceful shutdown -----------------------------------------------------
	go func() {
		logger.Info("video-service listening", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down video-service...")
	workerCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server forced shutdown", zap.Error(err))
	}
	logger.Info("video-service stopped")
}

// requestLogger returns a gin middleware that logs each request with zap.
func requestLogger(logger *zap.Logger) gin.HandlerFunc {
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
