package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/tiktok-clone/video-service/internal/config"
	"github.com/tiktok-clone/video-service/internal/models"
	"github.com/tiktok-clone/video-service/internal/repositories"
)

// schedulerCronSpec defines how often the scheduler runs.
// Default: every minute.
const defaultSchedulerCronSpec = "0 * * * * *"

// SchedulerWorker is a cron-based background job that polls for videos whose
// scheduled publish time has arrived and transitions them to the ready state.
type SchedulerWorker struct {
	cfg      *config.Config
	repo     *repositories.VideoRepository
	producer sarama.SyncProducer
	logger   *zap.Logger

	cron     *cron.Cron
	cronSpec string
}

// NewSchedulerWorker creates a new SchedulerWorker. The cronSpec argument
// overrides the default schedule; pass an empty string to use the default.
func NewSchedulerWorker(
	cfg *config.Config,
	repo *repositories.VideoRepository,
	producer sarama.SyncProducer,
	logger *zap.Logger,
	cronSpec string,
) *SchedulerWorker {
	if cronSpec == "" {
		cronSpec = defaultSchedulerCronSpec
	}
	return &SchedulerWorker{
		cfg:      cfg,
		repo:     repo,
		producer: producer,
		logger:   logger,
		cron:     cron.New(cron.WithSeconds()), // second-level precision cron
		cronSpec: cronSpec,
	}
}

// Start registers the publish job with the cron scheduler and begins execution.
// It returns an error if the cron expression is invalid. Start is non-blocking;
// use Stop to shut down gracefully.
func (s *SchedulerWorker) Start() error {
	_, err := s.cron.AddFunc(s.cronSpec, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
		defer cancel()
		s.publishDueVideos(ctx)
	})
	if err != nil {
		return fmt.Errorf("SchedulerWorker: invalid cron spec %q: %w", s.cronSpec, err)
	}

	s.cron.Start()
	s.logger.Info("scheduler worker started", zap.String("cron_spec", s.cronSpec))
	return nil
}

// Stop waits for any running jobs to complete, then halts the scheduler.
func (s *SchedulerWorker) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("scheduler worker stopped")
}

// publishDueVideos fetches all scheduled videos whose publish_at time has
// elapsed and publishes each one, emitting a Kafka event on success.
func (s *SchedulerWorker) publishDueVideos(ctx context.Context) {
	videos, err := s.repo.GetScheduledDue(ctx)
	if err != nil {
		s.logger.Error("scheduler: GetScheduledDue failed", zap.Error(err))
		return
	}

	if len(videos) == 0 {
		return
	}

	s.logger.Info("scheduler: publishing due videos", zap.Int("count", len(videos)))

	for _, v := range videos {
		if err := s.publishOne(ctx, v); err != nil {
			s.logger.Error("scheduler: failed to publish video",
				zap.String("video_id", v.ID),
				zap.Error(err),
			)
		}
	}
}

// publishOne transitions a single video to the ready/public state and emits
// a Kafka event so downstream services (feed, notifications) can react.
func (s *SchedulerWorker) publishOne(ctx context.Context, v *models.Video) error {
	if err := s.repo.PublishVideo(ctx, v.ID); err != nil {
		return fmt.Errorf("publishOne: repo.PublishVideo: %w", err)
	}

	event := scheduledPublishEvent{
		VideoID:     v.ID,
		UserID:      v.UserID,
		Title:       v.Title,
		Visibility:  string(v.Visibility),
		PublishedAt: time.Now().UTC(),
	}

	if err := s.emitPublishedEvent(v.ID, event); err != nil {
		// Non-fatal: log but don't fail — the DB state is already correct.
		s.logger.Warn("scheduler: failed to emit published event",
			zap.String("video_id", v.ID),
			zap.Error(err),
		)
	}

	s.logger.Info("scheduler: video published",
		zap.String("video_id", v.ID),
		zap.String("user_id", v.UserID),
	)
	return nil
}

// emitPublishedEvent sends a Kafka message to the transcoded topic to signal
// that a scheduled video has been published.
func (s *SchedulerWorker) emitPublishedEvent(videoID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: s.cfg.Kafka.TopicTranscoded,
		Key:   sarama.StringEncoder(videoID),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = s.producer.SendMessage(msg)
	return err
}

// ---- event types ------------------------------------------------------------

// scheduledPublishEvent is the Kafka payload emitted when a scheduled video goes live.
type scheduledPublishEvent struct {
	VideoID     string    `json:"video_id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	Visibility  string    `json:"visibility"`
	PublishedAt time.Time `json:"published_at"`
}

// RunNow executes a single publish cycle immediately, outside the cron schedule.
// Useful for testing and administrative triggers.
func (s *SchedulerWorker) RunNow(ctx context.Context) {
	s.publishDueVideos(ctx)
}
