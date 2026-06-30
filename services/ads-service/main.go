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

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ads-service/internal/config"
	"github.com/tiktok-clone/ads-service/internal/handlers"
	"github.com/tiktok-clone/ads-service/internal/services"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	cfg := config.Load()

	// Redis client (shared across services).
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("redis connection failed", zap.Error(err))
	}
	defer rdb.Close()

	// NOTE: In production, pass a real database-backed repository here.
	// For now we wire up the services with a nil repo; replace with the
	// concrete PostgreSQL implementation from the repository package.
	var repo services.CampaignRepository // inject concrete impl at startup

	campaignSvc := services.NewCampaignService(cfg, repo, rdb, logger)
	targetingSvc := services.NewTargetingService(cfg, rdb, logger)
	auctionSvc := services.NewAuctionService(cfg, repo, rdb, targetingSvc, campaignSvc, logger)

	handler := handlers.NewAdsHandler(campaignSvc, auctionSvc, logger)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("ads-service listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("shutting down ads-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
	logger.Info("ads-service stopped")
}
