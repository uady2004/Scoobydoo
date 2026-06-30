package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	"github.com/tiktok-clone/video-service/internal/config"
	"github.com/tiktok-clone/video-service/internal/models"
	"github.com/tiktok-clone/video-service/internal/repositories"
	"github.com/tiktok-clone/video-service/internal/services"
)

// TranscodingWorker is a Kafka consumer that processes VideoUploaded events
// and triggers the async transcoding pipeline for each video.
type TranscodingWorker struct {
	cfg          *config.Config
	transcodeSvc *services.TranscodingService
	repo         *repositories.VideoRepository
	logger       *zap.Logger

	consumerGroup sarama.ConsumerGroup
	ready         chan struct{} // closed once the consumer group is set up
}

// NewTranscodingWorker creates a new TranscodingWorker and initialises the
// Kafka consumer group.
func NewTranscodingWorker(
	cfg *config.Config,
	transcodeSvc *services.TranscodingService,
	repo *repositories.VideoRepository,
	logger *zap.Logger,
) (*TranscodingWorker, error) {
	saramaCfg, err := buildSaramaConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("NewTranscodingWorker build sarama config: %w", err)
	}

	group, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("NewTranscodingWorker create consumer group: %w", err)
	}

	return &TranscodingWorker{
		cfg:           cfg,
		transcodeSvc:  transcodeSvc,
		repo:          repo,
		logger:        logger,
		consumerGroup: group,
		ready:         make(chan struct{}),
	}, nil
}

// Start begins consuming messages from the VideoUploaded topic. It blocks until
// the provided context is cancelled or an OS signal (SIGINT/SIGTERM) is received.
func (w *TranscodingWorker) Start(ctx context.Context) error {
	topics := []string{w.cfg.Kafka.TopicUploaded}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := w.consumerGroup.Consume(ctx, topics, w); err != nil {
				if ctx.Err() != nil {
					return // context cancelled — normal shutdown
				}
				w.logger.Error("consumer group error; restarting", zap.Error(err))
				time.Sleep(2 * time.Second)
				continue
			}
			if ctx.Err() != nil {
				return
			}
			// Re-balance: reset ready flag and loop again.
			w.ready = make(chan struct{})
		}
	}()

	// Wait for the consumer group to be ready.
	select {
	case <-w.ready:
		w.logger.Info("transcoding worker is ready",
			zap.Strings("topics", topics),
			zap.String("group", w.cfg.Kafka.ConsumerGroup),
		)
	case <-ctx.Done():
		return ctx.Err()
	}

	<-ctx.Done()
	w.logger.Info("transcoding worker shutting down")
	wg.Wait()

	return w.consumerGroup.Close()
}

// Stop closes the consumer group gracefully.
func (w *TranscodingWorker) Stop() error {
	return w.consumerGroup.Close()
}

// ---- sarama.ConsumerGroupHandler implementation --------------------------------

// Setup is called at the beginning of a new session, before ConsumeClaim.
func (w *TranscodingWorker) Setup(_ sarama.ConsumerGroupSession) error {
	close(w.ready)
	return nil
}

// Cleanup is called at the end of a session, once all ConsumeClaim goroutines
// have exited.
func (w *TranscodingWorker) Cleanup(_ sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim processes messages from a single topic/partition claim.
// Each message represents one VideoUploaded event.
func (w *TranscodingWorker) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			w.handleMessage(session, msg)

		case <-session.Context().Done():
			return nil
		}
	}
}

// handleMessage deserialises a VideoUploaded event and starts the transcoding
// pipeline. On success it marks the Kafka offset as committed.
func (w *TranscodingWorker) handleMessage(session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	start := time.Now()

	var event models.VideoUploadedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		w.logger.Error("transcoding worker: failed to unmarshal event",
			zap.ByteString("raw", msg.Value),
			zap.Error(err),
		)
		// Commit offset anyway to avoid infinite re-delivery of malformed messages.
		session.MarkMessage(msg, "")
		return
	}

	w.logger.Info("processing VideoUploaded event",
		zap.String("video_id", event.VideoID),
		zap.String("user_id", event.UserID),
		zap.String("raw_s3_key", event.RawS3Key),
	)

	// Process in a separate goroutine so the consumer loop stays responsive,
	// but block until done before marking the offset.
	ctx := session.Context()
	if err := w.transcodeSvc.ProcessVideo(ctx, event.VideoID, event.RawS3Key); err != nil {
		w.logger.Error("transcoding failed",
			zap.String("video_id", event.VideoID),
			zap.Duration("elapsed", time.Since(start)),
			zap.Error(err),
		)

		// Mark the video as failed in the database.
		if dbErr := w.repo.UpdateStatus(ctx, event.VideoID, "failed"); dbErr != nil {
			w.logger.Warn("failed to update video status to failed",
				zap.String("video_id", event.VideoID),
				zap.Error(dbErr),
			)
		}

		// Still commit the offset; dead-letter queue handling can be added here.
		session.MarkMessage(msg, "")
		return
	}

	w.logger.Info("transcoding succeeded",
		zap.String("video_id", event.VideoID),
		zap.Duration("elapsed", time.Since(start)),
	)

	session.MarkMessage(msg, "")
}

// ---- Sarama configuration helper --------------------------------------------

// buildSaramaConfig builds a sarama.Config from the application config.
func buildSaramaConfig(cfg *config.Config) (*sarama.Config, error) {
	saramaCfg := sarama.NewConfig()

	// Parse Kafka version string.
	version, err := sarama.ParseKafkaVersion(cfg.Kafka.Version)
	if err != nil {
		// Fall back to a safe default.
		version = sarama.V3_0_0_0
	}
	saramaCfg.Version = version

	// Consumer settings.
	saramaCfg.Consumer.Group.Session.Timeout = cfg.Kafka.SessionTimeout
	saramaCfg.Consumer.Group.Heartbeat.Interval = cfg.Kafka.HeartbeatTimeout / 3
	saramaCfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaCfg.Consumer.Offsets.AutoCommit.Enable = false // we commit manually
	saramaCfg.Consumer.Return.Errors = true

	// Producer settings (used by the video service for outbound events).
	saramaCfg.Producer.Return.Successes = true
	saramaCfg.Producer.Return.Errors = true
	saramaCfg.Producer.RequiredAcks = sarama.WaitForAll
	saramaCfg.Producer.Retry.Max = 5

	return saramaCfg, nil
}

// NewSyncProducer creates a Sarama SyncProducer connected to the Kafka brokers
// in cfg. It is used by the VideoService to emit Kafka events.
func NewSyncProducer(cfg *config.Config) (sarama.SyncProducer, error) {
	saramaCfg, err := buildSaramaConfig(cfg)
	if err != nil {
		return nil, err
	}
	return sarama.NewSyncProducer(cfg.Kafka.Brokers, saramaCfg)
}

// Ensure the unused os import does not cause a build error in environments
// where the signal package pulls it in indirectly.
var _ = os.Stderr
