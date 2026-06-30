package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/tiktok-clone/payment-service/internal/config"
	"github.com/tiktok-clone/payment-service/internal/handlers"
	"github.com/tiktok-clone/payment-service/internal/repositories"
	"github.com/tiktok-clone/payment-service/internal/services"
)

func main() {
	// ---------- Logger ----------
	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.TimeKey = "ts"
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := logCfg.Build()
	if err != nil {
		panic("failed to initialise logger: " + err.Error())
	}
	defer logger.Sync() //nolint:errcheck

	// ---------- Config ----------
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}
	logger.Info("payment-service starting",
		zap.String("env", cfg.App.Environment),
		zap.String("addr", cfg.Server.Addr()),
	)

	// ---------- Database ----------
	poolCfg, err := pgxpool.ParseConfig(cfg.DB.URL)
	if err != nil {
		logger.Fatal("failed to parse database URL", zap.Error(err))
	}
	poolCfg.MaxConns = cfg.DB.MaxConns
	poolCfg.MinConns = cfg.DB.MinConns
	poolCfg.MaxConnLifetime = cfg.DB.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = cfg.DB.ConnMaxIdleTime

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err = pool.Ping(context.Background()); err != nil {
		logger.Fatal("database ping failed", zap.Error(err))
	}
	logger.Info("database connected")

	// ---------- Repository ----------
	repo := repositories.NewPaymentRepository(pool, logger)

	// ---------- Services ----------
	stripeSvc := services.NewStripeService(cfg, logger)
	paymentSvc := services.NewPaymentService(repo, stripeSvc, cfg, logger)

	// ---------- HTTP server (Echo) ----------
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = &echoValidator{v: validator.New()}

	// Global middleware.
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `{"time":"${time_rfc3339}","id":"${id}","method":"${method}","uri":"${uri}","status":${status},"latency_ms":${latency_ms}}` + "\n",
	}))
	if cfg.App.IsProduction() {
		e.Use(middleware.Secure())
	}

	// IMPORTANT: The Stripe webhook endpoint must receive the raw request body
	// before Echo parses it. Echo's binder reads Body on first Bind() call only,
	// so reading it explicitly in StripeWebhook (before any Bind) works correctly.

	// Register routes.
	h := handlers.NewPaymentHandler(paymentSvc, stripeSvc, cfg, logger)
	h.RegisterRoutes(e)

	// ---------- Graceful shutdown ----------
	srv := &http.Server{
		Addr:         cfg.Server.Addr(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("payment-service listening", zap.String("addr", cfg.Server.Addr()))
		if err := e.StartServer(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("shutting down payment-service…")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
	logger.Info("payment-service stopped")
}

// echoValidator adapts go-playground/validator to Echo's Validator interface.
type echoValidator struct {
	v *validator.Validate
}

func (ev *echoValidator) Validate(i interface{}) error {
	return ev.v.Struct(i)
}
