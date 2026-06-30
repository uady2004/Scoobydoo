package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/user-service/internal/config"
	"github.com/tiktok-clone/user-service/internal/handlers"
	"github.com/tiktok-clone/user-service/internal/middleware"
	"github.com/tiktok-clone/user-service/internal/repositories"
	"github.com/tiktok-clone/user-service/internal/services"
	"github.com/tiktok-clone/user-service/internal/validators"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx := context.Background()

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

	repo := repositories.NewProfileRepository(pool, logger)

	profileSvc, err := services.NewProfileService(repo, rdb, cfg, logger)
	if err != nil {
		logger.Fatal("profile service init", zap.Error(err))
	}

	val := validators.New()

	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	authMW := middleware.AuthMiddleware(cfg, logger)
	optAuthMW := middleware.OptionalAuthMiddleware(cfg, logger)

	api := e.Group("/api/v1")
	api.Use(optAuthMW)

	authGroup := api.Group("", authMW)

	handlers.NewUserHandler(api.Group(""), profileSvc, val, logger)
	handlers.NewCreatorHandler(authGroup.Group(""), profileSvc, val, logger)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      e,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.Info("user-service listening", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	}
	logger.Info("user-service stopped")
}
