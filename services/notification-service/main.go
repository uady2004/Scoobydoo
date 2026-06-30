package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/notification-service/internal/config"
	"github.com/tiktok-clone/notification-service/internal/handlers"
	"github.com/tiktok-clone/notification-service/internal/repositories"
	"github.com/tiktok-clone/notification-service/internal/services"
	"github.com/tiktok-clone/notification-service/internal/workers"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		logger.Fatal("postgres connect", zap.Error(err))
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	repo := repositories.NewNotificationRepository(pool, logger)

	pushSvc, err := services.NewPushService(cfg.Firebase, repo, logger)
	if err != nil {
		logger.Warn("push service unavailable (Firebase credentials missing)", zap.Error(err))
		pushSvc = nil
	}
	emailSvc := services.NewEmailService(cfg.SendGrid, logger)
	smsSvc := services.NewSMSService(cfg.Twilio, logger)

	notifSvc := services.NewNotificationService(repo, pushSvc, emailSvc, smsSvc, logger)

	consumer, err := workers.NewEventConsumer(cfg.Kafka, notifSvc, emailSvc, logger)
	if err != nil {
		logger.Fatal("kafka consumer init", zap.Error(err))
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go consumer.Start(ctx, &wg)

	handler := handlers.NewNotificationHandler(notifSvc, logger)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("notification-service listening", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
	wg.Wait()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	}
	logger.Info("notification-service stopped")
}
