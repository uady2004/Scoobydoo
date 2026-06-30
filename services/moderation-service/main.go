package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/moderation-service/internal/config"
	"github.com/tiktok-clone/moderation-service/internal/handlers"
	"github.com/tiktok-clone/moderation-service/internal/models"
	"github.com/tiktok-clone/moderation-service/internal/services"
	"github.com/tiktok-clone/moderation-service/internal/workers"
)

// ── stub Repository (no-op placeholder until a DB layer is added) ──────────

type noopRepo struct{}

func (r *noopRepo) CreateRequest(_ context.Context, _ *models.ModerationRequest) error { return nil }
func (r *noopRepo) GetRequest(_ context.Context, _ string) (*models.ModerationRequest, error) {
	return nil, services.ErrRequestNotFound
}
func (r *noopRepo) UpdateRequestStatus(_ context.Context, _ string, _ models.ModerationStatus) error {
	return nil
}
func (r *noopRepo) CreateResult(_ context.Context, _ *models.ModerationResult) error { return nil }
func (r *noopRepo) GetResult(_ context.Context, _ string) (*models.ModerationResult, error) {
	return nil, services.ErrResultNotFound
}
func (r *noopRepo) GetResultByContentID(_ context.Context, _ string) (*models.ModerationResult, error) {
	return nil, services.ErrResultNotFound
}
func (r *noopRepo) UpdateResult(_ context.Context, _ *models.ModerationResult) error { return nil }
func (r *noopRepo) CreateQueueItem(_ context.Context, _ *models.ModeratorQueueItem) error {
	return nil
}
func (r *noopRepo) GetQueueItem(_ context.Context, _ string) (*models.ModeratorQueueItem, error) {
	return nil, services.ErrQueueItemNotFound
}
func (r *noopRepo) UpdateQueueItem(_ context.Context, _ *models.ModeratorQueueItem) error {
	return nil
}
func (r *noopRepo) ListQueueItems(_ context.Context, _ services.QueueFilter) ([]*models.ModeratorQueueItem, int64, error) {
	return nil, 0, nil
}
func (r *noopRepo) EscalateStaleItems(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}
func (r *noopRepo) CreateAppeal(_ context.Context, _ *models.Appeal) error    { return nil }
func (r *noopRepo) GetAppeal(_ context.Context, _ string) (*models.Appeal, error) {
	return nil, services.ErrAppealNotFound
}
func (r *noopRepo) GetAppealByResultID(_ context.Context, _ string) (*models.Appeal, error) {
	return nil, services.ErrAppealNotFound
}
func (r *noopRepo) UpdateAppeal(_ context.Context, _ *models.Appeal) error        { return nil }
func (r *noopRepo) CountActiveAppeals(_ context.Context, _ string) (int64, error) { return 0, nil }
func (r *noopRepo) GetStats(_ context.Context, _, _ time.Time) (*models.ModerationStats, error) {
	return &models.ModerationStats{}, nil
}

// ── Kafka EventPublisher ───────────────────────────────────────────────────

type kafkaPublisher struct {
	producer sarama.SyncProducer
	topic    string
}

func (p *kafkaPublisher) PublishModerationResult(_ context.Context, result *models.ModerationResult) error {
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(result.ID),
	}
	_, _, err := p.producer.SendMessage(msg)
	return err
}

// ── main ──────────────────────────────────────────────────────────────────

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	saramaCfg := sarama.NewConfig()
	saramaCfg.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(cfg.Kafka.Brokers, saramaCfg)
	if err != nil {
		logger.Fatal("kafka producer init", zap.Error(err))
	}
	defer producer.Close()

	publisher := &kafkaPublisher{producer: producer, topic: cfg.Kafka.ModerationTopic}
	repo := &noopRepo{}

	svc := services.NewModerationService(cfg, repo, publisher, rdb, logger)
	handler := handlers.NewModerationHandler(svc, logger)

	worker, err := workers.NewVideoModerationWorker(cfg, svc, logger)
	if err != nil {
		logger.Fatal("worker init", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := worker.Run(ctx); err != nil {
			logger.Error("worker exited", zap.Error(err))
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("moderation-service listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	}
	logger.Info("moderation-service stopped")
}
