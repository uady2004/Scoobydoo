package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/tiktok-clone/comment-service/internal/config"
	"github.com/tiktok-clone/comment-service/internal/models"
	"github.com/tiktok-clone/comment-service/internal/repositories"
)

type CommentService interface {
	CreateComment(ctx context.Context, req models.CreateCommentReq) (*models.Comment, error)
	GetComment(ctx context.Context, id string) (*models.Comment, error)
	ListComments(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error)
	ListReplies(ctx context.Context, parentID string, limit, offset int) ([]*models.Comment, error)
	DeleteComment(ctx context.Context, id, userID string) error
	LikeComment(ctx context.Context, userID, commentID string) error
	UnlikeComment(ctx context.Context, userID, commentID string) error
	IsCommentLiked(ctx context.Context, userID, commentID string) (bool, error)
}

type commentService struct {
	repo   repositories.CommentRepository
	writer *kafka.Writer
	cfg    *config.Config
	logger *zap.Logger
}

func NewCommentService(
	repo repositories.CommentRepository,
	writer *kafka.Writer,
	cfg *config.Config,
	logger *zap.Logger,
) CommentService {
	return &commentService{repo: repo, writer: writer, cfg: cfg, logger: logger}
}

func (s *commentService) CreateComment(ctx context.Context, req models.CreateCommentReq) (*models.Comment, error) {
	if req.VideoID == "" {
		return nil, errors.New("video_id is required")
	}
	if req.Content == "" || len(req.Content) > 1000 {
		return nil, errors.New("content must be 1–1000 characters")
	}

	comment, err := s.repo.Create(ctx, req)
	if err != nil {
		s.logger.Error("create comment failed", zap.Error(err))
		return nil, fmt.Errorf("creating comment: %w", err)
	}

	s.publishEvent(s.cfg.Kafka.Topics.CommentCreated, map[string]any{
		"comment_id": comment.ID,
		"video_id":   comment.VideoID,
		"user_id":    comment.UserID,
		"content":    comment.Content,
		"parent_id":  comment.ParentID,
		"created_at": comment.CreatedAt,
	})
	return comment, nil
}

func (s *commentService) GetComment(ctx context.Context, id string) (*models.Comment, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *commentService) ListComments(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListByVideo(ctx, videoID, limit, offset)
}

func (s *commentService) ListReplies(ctx context.Context, parentID string, limit, offset int) ([]*models.Comment, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListReplies(ctx, parentID, limit, offset)
}

func (s *commentService) DeleteComment(ctx context.Context, id, userID string) error {
	if err := s.repo.SoftDelete(ctx, id, userID); err != nil {
		return err
	}
	s.publishEvent(s.cfg.Kafka.Topics.CommentDeleted, map[string]any{
		"comment_id": id,
		"user_id":    userID,
		"deleted_at": time.Now().UTC(),
	})
	return nil
}

func (s *commentService) LikeComment(ctx context.Context, userID, commentID string) error {
	if err := s.repo.LikeComment(ctx, userID, commentID); err != nil {
		return err
	}
	return s.repo.IncrementLikes(ctx, commentID)
}

func (s *commentService) UnlikeComment(ctx context.Context, userID, commentID string) error {
	if err := s.repo.UnlikeComment(ctx, userID, commentID); err != nil {
		return err
	}
	return s.repo.DecrementLikes(ctx, commentID)
}

func (s *commentService) IsCommentLiked(ctx context.Context, userID, commentID string) (bool, error) {
	return s.repo.IsCommentLiked(ctx, userID, commentID)
}

func (s *commentService) publishEvent(topic string, payload any) {
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
