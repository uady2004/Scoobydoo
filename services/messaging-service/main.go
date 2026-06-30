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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/tiktok-clone/messaging-service/internal/config"
	"github.com/tiktok-clone/messaging-service/internal/handlers"
	"github.com/tiktok-clone/messaging-service/internal/repositories"
	"github.com/tiktok-clone/messaging-service/internal/services"
	ws "github.com/tiktok-clone/messaging-service/internal/websocket"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// -------------------------------------------------------------------------
	// Configuration
	// -------------------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// -------------------------------------------------------------------------
	// Logger
	// -------------------------------------------------------------------------
	log, err := buildLogger(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	// -------------------------------------------------------------------------
	// Database (PostgreSQL via pgxpool)
	// -------------------------------------------------------------------------
	dbPool, err := pgxpool.New(context.Background(), cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer dbPool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = dbPool.Ping(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	log.Info("database connected")

	// -------------------------------------------------------------------------
	// Redis
	// -------------------------------------------------------------------------
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
		PoolSize:     cfg.Redis.PoolSize,
	})
	defer func() { _ = rdb.Close() }()

	rctx, rcancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer rcancel()
	if err = rdb.Ping(rctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	log.Info("redis connected")

	// -------------------------------------------------------------------------
	// Repositories, services, WebSocket infrastructure
	// -------------------------------------------------------------------------
	repo := repositories.NewMessageRepository(dbPool, log)

	hub := ws.NewHub(repo, log)
	presence := ws.NewPresenceService(rdb, log)

	svc, err := services.NewMessageService(repo, hub, cfg, log)
	if err != nil {
		return fmt.Errorf("build message service: %w", err)
	}

	// -------------------------------------------------------------------------
	// HTTP router
	// -------------------------------------------------------------------------
	if cfg.Log.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger(log))

	msgHandler := handlers.NewMessageHandler(svc, log)
	wsHandler := handlers.NewWSHandler(hub, presence, repo, cfg, log)

	// Health check (no auth required)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// All API routes require authentication.
	// In production a JWT middleware would be applied here:
	//   api := router.Group("/api/v1", jwtMiddleware(cfg.JWT.Secret))
	api := router.Group("/api/v1")

	// Conversations
	api.POST("/conversations", msgHandler.CreateConversation)
	api.GET("/conversations", msgHandler.GetConversations)
	api.GET("/conversations/unread/total", msgHandler.GetTotalUnreadCount)
	api.GET("/conversations/:id/unread", msgHandler.GetUnreadCount)

	// Messages
	api.POST("/messages", msgHandler.SendMessage)
	api.GET("/messages", msgHandler.GetMessages)
	api.POST("/messages/read", msgHandler.MarkRead)
	api.DELETE("/messages/:id", msgHandler.DeleteMessage)

	// Reactions
	api.POST("/messages/:id/reactions", msgHandler.AddReaction)
	api.DELETE("/messages/:id/reactions/:emoji", msgHandler.RemoveReaction)

	// Groups
	api.POST("/groups", msgHandler.CreateGroup)
	api.POST("/groups/:id/members", msgHandler.AddGroupMembers)
	api.DELETE("/groups/:id/members", msgHandler.RemoveGroupMember)

	// Media upload
	api.POST("/media/upload", msgHandler.ShareMedia)

	// WebSocket endpoint
	api.GET("/ws", wsHandler.ServeWS)

	// -------------------------------------------------------------------------
	// Start WebSocket hub event loop
	// -------------------------------------------------------------------------
	done := make(chan struct{})
	go hub.Run(done)

	// -------------------------------------------------------------------------
	// HTTP server with graceful shutdown
	// -------------------------------------------------------------------------
	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("messaging service listening", zap.String("addr", cfg.Addr()))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("received shutdown signal", zap.String("signal", sig.String()))
	case err := <-errCh:
		return fmt.Errorf("http server error: %w", err)
	}

	// Graceful HTTP shutdown (30-second window)
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err = srv.Shutdown(shutCtx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}

	// Stop WebSocket hub
	close(done)

	log.Info("messaging service stopped")
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildLogger(level, format string) (*zap.Logger, error) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var enc zapcore.Encoder
	if format == "console" {
		enc = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		enc = zapcore.NewJSONEncoder(encoderCfg)
	}

	core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), lvl)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

func requestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
