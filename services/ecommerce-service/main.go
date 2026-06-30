package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ecommerce-service/internal/config"
	"github.com/tiktok-clone/ecommerce-service/internal/handlers"
	"github.com/tiktok-clone/ecommerce-service/internal/repositories"
	"github.com/tiktok-clone/ecommerce-service/internal/services"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.Database.URL)
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

	productRepo := repositories.NewProductRepository(pool, logger)
	orderRepo := repositories.NewOrderRepository(pool, logger)

	productSvc, err := services.NewProductService(productRepo, cfg, logger)
	if err != nil {
		logger.Fatal("product service init", zap.Error(err))
	}
	orderSvc := services.NewOrderService(orderRepo, productRepo, cfg, logger)
	cartSvc := services.NewCartService(orderRepo, productRepo, orderSvc, logger)

	productHandler := handlers.NewProductHandler(productSvc, logger)
	orderHandler := handlers.NewOrderHandler(orderSvc, logger)
	cartHandler := handlers.NewCartHandler(cartSvc, logger)

	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := r.Group("/api/v1")
	// Products: public read (GET), auth-required write (POST/PUT/DELETE)
	productHandler.RegisterRoutes(api, api)
	orderHandler.RegisterRoutes(api)
	cartHandler.RegisterRoutes(api)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("ecommerce-service listening", zap.String("addr", addr))
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
	logger.Info("ecommerce-service stopped")
}
