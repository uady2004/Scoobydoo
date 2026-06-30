package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/tiktok-clone/like-service/internal/config"
	"github.com/tiktok-clone/like-service/internal/models"
	"github.com/tiktok-clone/like-service/internal/repositories"
)

const likeCountKeyFmt = "like:count:%s"

type LikeService interface {
	LikeVideo(ctx context.Context, userID, videoID string) error
	UnlikeVideo(ctx context.Context, userID, videoID string) error
	IsLiked(ctx context.Context, userID, videoID string) (bool, error)
	GetLikeCount(ctx context.Context, videoID string) (int64, error)
	BatchCheckLikes(ctx context.Context, userID string, videoIDs []string) (map[string]bool, error)
	GetUserLikedVideos(ctx context.Context, userID string, limit, offset int) ([]*models.Like, error)
	GetTopLikedVideos(ctx context.Context, limit int) ([]*models.VideoLikeCount, error)
}

type likeService struct {
	repo   repositories.LikeRepository
	rdb    *redis.Client
	writer *kafka.Writer
	cfg    *config.Config
	logger *zap.Logger
}

func NewLikeService(
	repo repositories.LikeRepository,
	rdb *redis.Client,
	writer *kafka.Writer,
	cfg *config.Config,
	logger *zap.Logger,
) LikeService {
	return &likeService{repo: repo, rdb: rdb, writer: writer, cfg: cfg, logger: logger}
}

func (s *likeService) LikeVideo(ctx context.Context, userID, videoID string) error {
	err := s.repo.LikeVideo(ctx, userID, videoID)
	if err != nil {
		if errors.Is(err, repositories.ErrAlreadyLiked) {
			return err
		}
		return fmt.Errorf("liking video: %w", err)
	}

	key := fmt.Sprintf(likeCountKeyFmt, videoID)
	s.rdb.Incr(ctx, key)

	s.publishEvent(s.cfg.Kafka.Topics.VideoLiked, map[string]any{
		"user_id":  userID,
		"video_id": videoID,
		"liked_at": time.Now().UTC(),
	})
	return nil
}

func (s *likeService) UnlikeVideo(ctx context.Context, userID, videoID string) error {
	if err := s.repo.UnlikeVideo(ctx, userID, videoID); err != nil {
		return err
	}

	key := fmt.Sprintf(likeCountKeyFmt, videoID)
	s.rdb.Decr(ctx, key)

	s.publishEvent(s.cfg.Kafka.Topics.VideoUnliked, map[string]any{
		"user_id":    userID,
		"video_id":   videoID,
		"unliked_at": time.Now().UTC(),
	})
	return nil
}

func (s *likeService) IsLiked(ctx context.Context, userID, videoID string) (bool, error) {
	return s.repo.IsLiked(ctx, userID, videoID)
}

func (s *likeService) GetLikeCount(ctx context.Context, videoID string) (int64, error) {
	key := fmt.Sprintf(likeCountKeyFmt, videoID)
	count, err := s.rdb.Get(ctx, key).Int64()
	if err == nil {
		return count, nil
	}
	count, err = s.repo.GetLikeCount(ctx, videoID)
	if err != nil {
		return 0, err
	}
	s.rdb.Set(ctx, key, count, 0)
	return count, nil
}

func (s *likeService) BatchCheckLikes(ctx context.Context, userID string, videoIDs []string) (map[string]bool, error) {
	return s.repo.BatchIsLiked(ctx, userID, videoIDs)
}

func (s *likeService) GetUserLikedVideos(ctx context.Context, userID string, limit, offset int) ([]*models.Like, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.GetUserLikedVideos(ctx, userID, limit, offset)
}

func (s *likeService) GetTopLikedVideos(ctx context.Context, limit int) ([]*models.VideoLikeCount, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.GetTopLikedVideos(ctx, limit)
}

func (s *likeService) publishEvent(topic string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("marshal event failed", zap.Error(err))
		return
	}
	if err := s.writer.WriteMessages(context.Background(), kafka.Message{
		Topic: topic,
		Value: data,
	}); err != nil {
		s.logger.Warn("publish event failed", zap.String("topic", topic), zap.Error(err))
	}
}
