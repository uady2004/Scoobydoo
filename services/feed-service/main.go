// main is the entry point for the feed-service. It wires all dependencies,
// starts background workers, and runs the HTTP server until SIGTERM/SIGINT.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/feed-service/internal/config"
	"github.com/tiktok-clone/feed-service/internal/handlers"
	"github.com/tiktok-clone/feed-service/internal/repositories"
	"github.com/tiktok-clone/feed-service/internal/services"
	"github.com/tiktok-clone/feed-service/internal/workers"
)

func main() {
	// ---- Logger ----------------------------------------------------------------
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	// ---- Config ----------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// ---- Context (cancelled on SIGTERM / SIGINT) --------------------------------
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// ---- PostgreSQL ------------------------------------------------------------
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN)
	if err != nil {
		logger.Fatal("invalid database DSN", zap.Error(err))
	}
	poolCfg.MaxConns = cfg.Database.MaxConns
	poolCfg.MinConns = cfg.Database.MinConns
	poolCfg.MaxConnLifetime = cfg.Database.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.Database.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.Database.HealthCheckPeriod

	db, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		logger.Fatal("database ping failed", zap.Error(err))
	}
	logger.Info("database connected")

	// ---- Redis -----------------------------------------------------------------
	var rdb redis.UniversalClient
	if len(cfg.Redis.Addrs) > 1 {
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    cfg.Redis.Addrs,
			Password: cfg.Redis.Password,
			PoolSize: cfg.Redis.PoolSize,
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			PoolSize: cfg.Redis.PoolSize,
		})
	}
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("redis ping failed", zap.Error(err))
	}
	logger.Info("redis connected")

	// ---- Repository ------------------------------------------------------------
	repo := repositories.NewFeedRepository(rdb, db, logger)

	// ---- Stub gRPC clients (replace with real gRPC in production) --------------
	recommendClient := &noopRecommendationClient{}
	socialGraphClient := &noopSocialGraphClient{}

	// ---- Feed service ----------------------------------------------------------
	feedSvc := services.NewFeedService(
		repo,
		recommendClient,
		socialGraphClient,
		services.FeedServiceConfig{
			DefaultLimit:    cfg.Feed.DefaultPageSize,
			MaxLimit:        cfg.Feed.MaxPageSize,
			ForYouTTL:       cfg.Feed.ForYouCacheTTL,
			FollowingTTL:    cfg.Feed.FollowingCacheTTL,
			TrendingTTL:     cfg.Feed.TrendingCacheTTL,
			NearbyTTL:       cfg.Feed.NearbyCacheTTL,
			ExploreTTL:      cfg.Feed.ExploreCacheTTL,
			DedupTTL:        cfg.Feed.DeduplicationTTL,
			NearbyDefaultKm: cfg.Feed.NearbyDefaultRadiusKm,
			NearbyMaxKm:     cfg.Feed.MaxNearbyRadiusKm,
		},
		logger,
	)

	// ---- Trending service ------------------------------------------------------
	trendingSvc := services.NewTrendingService(
		repo,
		cfg.Feed.TrendingWindowHours,
		logger,
	)

	// ---- Background workers ----------------------------------------------------
	precomputeWorker := workers.NewFeedPrecomputeWorker(
		feedSvc,
		repo,
		workers.FeedPrecomputeConfig{
			Interval:    cfg.Feed.PrecomputeInterval,
			BatchSize:   cfg.Feed.PrecomputeBatchSize,
			Concurrency: 10,
		},
		logger,
	)

	trendingUpdater := workers.NewTrendingUpdater(
		trendingSvc,
		workers.TrendingUpdaterConfig{
			Interval:         cfg.Feed.TrendingUpdateInterval,
			KafkaBrokers:     cfg.Kafka.Brokers,
			VideoEventsTopic: cfg.Kafka.VideoEventsTopic,
			ConsumerGroup:    cfg.Kafka.ConsumerGroup,
		},
		logger,
	)

	go precomputeWorker.Run(ctx)
	go trendingUpdater.Run(ctx)

	// ---- HTTP router -----------------------------------------------------------
	feedHandler := handlers.NewFeedHandler(feedSvc, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(cfg.Server.WriteTimeout))
	r.Use(handlers.AuthMiddleware)

	r.Get("/health", feedHandler.HandleHealth)
	r.Get("/ready", feedHandler.HandleReady)

	// Legacy flat routes (keep for direct service calls)
	r.Route("/feed", func(r chi.Router) {
		r.Get("/foryou", feedHandler.HandleForYou)
		r.Get("/following", feedHandler.HandleFollowing)
		r.Get("/trending", feedHandler.HandleTrending)
		r.Get("/nearby", feedHandler.HandleNearby)
		r.Get("/explore", feedHandler.HandleExplore)
	})

	// Gateway-forwarded routes: gateway sends full /api/v1/... paths
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/feed", feedHandler.HandleForYou)
		r.Get("/feed/for-you", feedHandler.HandleForYou)
		r.Get("/feed/following", feedHandler.HandleFollowing)
		r.Get("/feed/trending", feedHandler.HandleTrending)
		r.Get("/feed/nearby", feedHandler.HandleNearby)
		r.Get("/feed/explore", feedHandler.HandleExplore)
		r.Post("/feed/view", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		r.Get("/videos/trending", feedHandler.HandleTrending)
	})

	// ---- HTTP server -----------------------------------------------------------
	srv := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("feed-service listening", zap.String("addr", srv.Addr))
		serverErr <- srv.ListenAndServe()
	}()

	// ---- Graceful shutdown -----------------------------------------------------
	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(), cfg.Server.ShutdownTimeout,
	)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	}
	logger.Info("feed-service stopped")
}

// ---- Stub gRPC client implementations (replace in production) ---------------

type noopRecommendationClient struct{}

func (n *noopRecommendationClient) GetRecommendations(
	_ context.Context, _ string, _ int,
) ([]services.RecommendedVideo, error) {
	// Returns empty slice so the service falls back to trending.
	return nil, nil
}

type noopSocialGraphClient struct{}

func (n *noopSocialGraphClient) GetFollowing(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// noopSocialGraphClient satisfies services.SocialGraphClient.
var _ services.SocialGraphClient = (*noopSocialGraphClient)(nil)

// noopRecommendationClient satisfies services.RecommendationClient.
var _ services.RecommendationClient = (*noopRecommendationClient)(nil)

// Silence unused import warning for time — it is used by the pool config.
var _ = time.Second
