package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	"github.com/tiktok-clone/moderation-service/internal/config"
	"github.com/tiktok-clone/moderation-service/internal/models"
	"github.com/tiktok-clone/moderation-service/internal/services"
)

// VideoUploadedEvent matches the event schema published by the video-service.
type VideoUploadedEvent struct {
	EventID     string    `json:"event_id"`
	VideoID     string    `json:"video_id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	VideoURL    string    `json:"video_url"`
	ThumbnailURL string   `json:"thumbnail_url"`
	Duration    float64   `json:"duration"`
	FileSize    int64     `json:"file_size"`
	MIMEType    string    `json:"mime_type"`
	Tags        []string  `json:"tags"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// VideoModerationWorker consumes VideoUploaded Kafka events and runs each
// video through the full moderation pipeline asynchronously.
type VideoModerationWorker struct {
	cfg                *config.Config
	moderationService  *services.ModerationService
	logger             *zap.Logger
	consumerGroup      sarama.ConsumerGroup
	concurrencyLimit   int
}

// NewVideoModerationWorker constructs the worker and connects to Kafka.
func NewVideoModerationWorker(
	cfg *config.Config,
	svc *services.ModerationService,
	logger *zap.Logger,
) (*VideoModerationWorker, error) {
	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V2_6_0_0
	saramaCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	saramaCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	saramaCfg.Consumer.Return.Errors = true

	// Idempotent exactly-once processing: commit offsets only after moderation succeeds.
	saramaCfg.Consumer.Offsets.AutoCommit.Enable = false

	group, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("video_moderation_worker: create consumer group: %w", err)
	}

	return &VideoModerationWorker{
		cfg:               cfg,
		moderationService: svc,
		logger:            logger,
		consumerGroup:     group,
		concurrencyLimit:  10, // process up to 10 videos concurrently
	}, nil
}

// Run starts the Kafka consumer loop. It blocks until ctx is cancelled.
func (w *VideoModerationWorker) Run(ctx context.Context) error {
	topics := []string{w.cfg.Kafka.VideoUploadTopic}

	w.logger.Info("starting video moderation worker",
		zap.Strings("topics", topics),
		zap.String("consumer_group", w.cfg.Kafka.ConsumerGroup),
	)

	handler := &consumerGroupHandler{
		worker: w,
		sem:    make(chan struct{}, w.concurrencyLimit),
	}

	for {
		if err := w.consumerGroup.Consume(ctx, topics, handler); err != nil {
			if ctx.Err() != nil {
				w.logger.Info("video moderation worker shutting down")
				return nil
			}
			w.logger.Error("consumer group error", zap.Error(err))
			// Brief back-off before reconnecting.
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// Close shuts down the Kafka consumer group gracefully.
func (w *VideoModerationWorker) Close() error {
	return w.consumerGroup.Close()
}

// ---- sarama ConsumerGroupHandler implementation ----------------------------

// consumerGroupHandler implements sarama.ConsumerGroupHandler.
type consumerGroupHandler struct {
	worker *VideoModerationWorker
	// sem is a semaphore that limits concurrent moderation goroutines.
	sem chan struct{}
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error {
	h.worker.logger.Info("consumer group session setup")
	return nil
}

func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	h.worker.logger.Info("consumer group session cleanup")
	return nil
}

// ConsumeClaim processes messages from a single partition claim.
func (h *consumerGroupHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.processMessage(session, msg)

		case <-session.Context().Done():
			return nil
		}
	}
}

// processMessage deserialises the Kafka message and launches a moderation goroutine.
func (h *consumerGroupHandler) processMessage(
	session sarama.ConsumerGroupSession,
	msg *sarama.ConsumerMessage,
) {
	var event VideoUploadedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		h.worker.logger.Error("failed to deserialise VideoUploadedEvent",
			zap.Error(err),
			zap.Int32("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
		)
		// Mark malformed message as processed so it doesn't block the partition.
		session.MarkMessage(msg, "")
		return
	}

	h.worker.logger.Info("received VideoUploadedEvent",
		zap.String("video_id", event.VideoID),
		zap.String("user_id", event.UserID),
		zap.Int32("partition", msg.Partition),
		zap.Int64("offset", msg.Offset),
	)

	// Acquire semaphore slot — blocks if concurrency limit is reached.
	h.sem <- struct{}{}

	go func(evt VideoUploadedEvent, m *sarama.ConsumerMessage) {
		defer func() { <-h.sem }()

		if err := h.moderateVideo(session.Context(), &evt); err != nil {
			h.worker.logger.Error("moderation pipeline error",
				zap.String("video_id", evt.VideoID),
				zap.Error(err),
			)
			// Do NOT mark offset on error so the message will be re-processed
			// after a consumer restart (at-least-once semantics).
			return
		}
		// Commit offset only on success.
		session.MarkMessage(m, "")
		session.Commit()
	}(event, msg)
}

// moderateVideo builds a ModerationRequest from the event and calls the service.
func (h *consumerGroupHandler) moderateVideo(ctx context.Context, event *VideoUploadedEvent) error {
	// Build caption/description text for spam detection.
	textParts := []string{event.Title, event.Description}
	for _, tag := range event.Tags {
		textParts = append(textParts, "#"+tag)
	}
	combined := joinStrings(textParts, " ")

	req := &models.ModerationRequest{
		ContentID:    event.VideoID,
		ContentType:  models.ContentTypeVideo,
		UserID:       event.UserID,
		ContentURL:   event.VideoURL,
		ThumbnailURL: event.ThumbnailURL,
		TextContent:  combined,
		Duration:     event.Duration,
		FileSize:     event.FileSize,
		MIMEType:     event.MIMEType,
		Priority:     5, // default priority for async pipeline
	}

	start := time.Now()
	result, err := h.worker.moderationService.ModerateContent(ctx, req)
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("moderate video %s: %w", event.VideoID, err)
	}

	h.worker.logger.Info("video moderation complete",
		zap.String("video_id", event.VideoID),
		zap.String("status", string(result.Status)),
		zap.Float64("nsfw_score", result.NSFWScore),
		zap.Float64("violence_score", result.ViolenceScore),
		zap.Float64("spam_score", result.SpamScore),
		zap.Duration("elapsed", elapsed),
	)

	return nil
}

// joinStrings concatenates non-empty strings with sep.
func joinStrings(parts []string, sep string) string {
	var result string
	for _, p := range parts {
		if p == "" {
			continue
		}
		if result != "" {
			result += sep
		}
		result += p
	}
	return result
}
