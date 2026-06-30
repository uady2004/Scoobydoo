package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/video-service/internal/config"
	"github.com/tiktok-clone/video-service/internal/models"
	"github.com/tiktok-clone/video-service/internal/repositories"
)

const (
	videoDetailCacheTTL  = 5 * time.Minute
	trendingCacheTTL     = 2 * time.Minute
	videoDetailKeyPrefix = "video:detail:"
	trendingKey          = "video:trending"
)

// VideoService handles core video lifecycle operations.
type VideoService struct {
	cfg      *config.Config
	repo     *repositories.VideoRepository
	redis    *redis.Client
	producer sarama.SyncProducer
	logger   *zap.Logger
}

// NewVideoService creates a new VideoService.
func NewVideoService(
	cfg *config.Config,
	repo *repositories.VideoRepository,
	redisClient *redis.Client,
	producer sarama.SyncProducer,
	logger *zap.Logger,
) *VideoService {
	return &VideoService{
		cfg:      cfg,
		repo:     repo,
		redis:    redisClient,
		producer: producer,
		logger:   logger,
	}
}

// GetVideo fetches a video by ID. It checks the Redis cache before hitting Postgres.
func (s *VideoService) GetVideo(ctx context.Context, videoID string) (*models.Video, error) {
	// Try cache first.
	cacheKey := videoDetailKeyPrefix + videoID
	cached, err := s.redis.Get(ctx, cacheKey).Bytes()
	if err == nil {
		var v models.Video
		if jsonErr := json.Unmarshal(cached, &v); jsonErr == nil {
			return &v, nil
		}
	}

	v, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, fmt.Errorf("GetVideo: %w", err)
	}

	// Populate cache.
	if data, jsonErr := json.Marshal(v); jsonErr == nil {
		s.redis.Set(ctx, cacheKey, data, videoDetailCacheTTL)
	}
	return v, nil
}

// UpdateVideo applies a partial update to video metadata.
func (s *VideoService) UpdateVideo(ctx context.Context, videoID, requestingUserID string, req *models.UpdateVideoRequest) (*models.Video, error) {
	// Verify ownership.
	existing, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, fmt.Errorf("UpdateVideo fetch: %w", err)
	}
	if existing.UserID != requestingUserID {
		return nil, ErrForbidden
	}

	updated, err := s.repo.UpdateVideo(ctx, videoID, req)
	if err != nil {
		return nil, fmt.Errorf("UpdateVideo: %w", err)
	}

	// Invalidate cache.
	s.redis.Del(ctx, videoDetailKeyPrefix+videoID)

	return updated, nil
}

// DeleteVideo soft-deletes a video, verifying that the requesting user owns it.
func (s *VideoService) DeleteVideo(ctx context.Context, videoID, requestingUserID string) error {
	existing, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrVideoNotFound
		}
		return fmt.Errorf("DeleteVideo fetch: %w", err)
	}
	if existing.UserID != requestingUserID {
		return ErrForbidden
	}

	if err := s.repo.DeleteVideo(ctx, videoID); err != nil {
		return fmt.Errorf("DeleteVideo: %w", err)
	}

	s.redis.Del(ctx, videoDetailKeyPrefix+videoID)
	s.logger.Info("video deleted", zap.String("video_id", videoID), zap.String("user_id", requestingUserID))
	return nil
}

// PublishVideo transitions a video to the public/ready state and emits a
// Kafka event so downstream services (feed, notification) can react.
func (s *VideoService) PublishVideo(ctx context.Context, videoID, requestingUserID string) (*models.Video, error) {
	existing, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, fmt.Errorf("PublishVideo fetch: %w", err)
	}
	if existing.UserID != requestingUserID {
		return nil, ErrForbidden
	}
	if existing.Status != models.StatusReady && existing.Status != models.StatusDraft && existing.Status != models.StatusScheduled {
		return nil, fmt.Errorf("PublishVideo: cannot publish video in status %q", existing.Status)
	}

	if err := s.repo.PublishVideo(ctx, videoID); err != nil {
		return nil, fmt.Errorf("PublishVideo: %w", err)
	}

	// Emit event.
	if err := s.emitEvent(ctx, s.cfg.Kafka.TopicTranscoded, videoID, map[string]any{
		"action":   "published",
		"video_id": videoID,
		"user_id":  requestingUserID,
	}); err != nil {
		s.logger.Warn("PublishVideo emit event failed", zap.Error(err))
	}

	s.redis.Del(ctx, videoDetailKeyPrefix+videoID)
	s.redis.Del(ctx, trendingKey)

	return s.repo.GetByID(ctx, videoID)
}

// SaveDraft ensures a video remains in draft state with updated metadata.
func (s *VideoService) SaveDraft(ctx context.Context, videoID, requestingUserID string, req *models.UpdateVideoRequest) (*models.Video, error) {
	existing, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, fmt.Errorf("SaveDraft fetch: %w", err)
	}
	if existing.UserID != requestingUserID {
		return nil, ErrForbidden
	}

	// Keep visibility private for drafts.
	priv := models.VisibilityPrivate
	req.Visibility = &priv

	updated, err := s.repo.UpdateVideo(ctx, videoID, req)
	if err != nil {
		return nil, fmt.Errorf("SaveDraft update: %w", err)
	}

	// Ensure status remains draft.
	if err := s.repo.UpdateStatus(ctx, videoID, models.StatusDraft); err != nil {
		s.logger.Warn("SaveDraft update status failed", zap.Error(err))
	}

	s.redis.Del(ctx, videoDetailKeyPrefix+videoID)
	return updated, nil
}

// ScheduleVideo sets the publish_at time and transitions the video to scheduled.
func (s *VideoService) ScheduleVideo(ctx context.Context, videoID, requestingUserID string, publishAt time.Time) (*models.Video, error) {
	if publishAt.Before(time.Now().Add(time.Minute)) {
		return nil, fmt.Errorf("ScheduleVideo: publish_at must be at least 1 minute in the future")
	}

	existing, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, fmt.Errorf("ScheduleVideo fetch: %w", err)
	}
	if existing.UserID != requestingUserID {
		return nil, ErrForbidden
	}

	if err := s.repo.SchedulePublish(ctx, videoID, publishAt); err != nil {
		return nil, fmt.Errorf("ScheduleVideo: %w", err)
	}

	// Emit scheduled event.
	if err := s.emitEvent(ctx, s.cfg.Kafka.TopicScheduled, videoID, map[string]any{
		"video_id":   videoID,
		"user_id":    requestingUserID,
		"publish_at": publishAt.Format(time.RFC3339),
	}); err != nil {
		s.logger.Warn("ScheduleVideo emit event failed", zap.Error(err))
	}

	s.redis.Del(ctx, videoDetailKeyPrefix+videoID)

	return s.repo.GetByID(ctx, videoID)
}

// GetVideosByUser returns paginated videos for a given user, respecting visibility
// rules (only the owner sees private/draft videos).
func (s *VideoService) GetVideosByUser(ctx context.Context, targetUserID, requestingUserID string, limit, offset int) ([]*models.Video, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	videos, err := s.repo.GetByUserID(ctx, targetUserID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("GetVideosByUser: %w", err)
	}

	// Filter out private/draft videos if the requester is not the owner.
	if targetUserID != requestingUserID {
		filtered := videos[:0]
		for _, v := range videos {
			if v.Visibility == models.VisibilityPublic && v.Status == models.StatusReady {
				filtered = append(filtered, v)
			}
		}
		videos = filtered
	}
	return videos, nil
}

// GetTrending returns the top-N trending videos with Redis caching.
func (s *VideoService) GetTrending(ctx context.Context, limit int) ([]*models.Video, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Try cache.
	cached, err := s.redis.Get(ctx, trendingKey).Bytes()
	if err == nil {
		var videos []*models.Video
		if jsonErr := json.Unmarshal(cached, &videos); jsonErr == nil {
			if len(videos) >= limit {
				return videos[:limit], nil
			}
			return videos, nil
		}
	}

	videos, err := s.repo.GetTrending(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("GetTrending: %w", err)
	}

	// Cache result.
	if data, jsonErr := json.Marshal(videos); jsonErr == nil {
		s.redis.Set(ctx, trendingKey, data, trendingCacheTTL)
	}
	return videos, nil
}

// GetDrafts returns draft videos for the requesting user.
func (s *VideoService) GetDrafts(ctx context.Context, userID string, limit, offset int) ([]*models.Video, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.GetDrafts(ctx, userID, limit, offset)
}

// ---- sentinel errors --------------------------------------------------------

var (
	ErrVideoNotFound = errors.New("video not found")
	ErrForbidden     = errors.New("access forbidden: you do not own this video")
)

// ---- Kafka helper -----------------------------------------------------------

func (s *VideoService) emitEvent(ctx context.Context, topic, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(data),
	}
	_, _, err = s.producer.SendMessage(msg)
	return err
}
