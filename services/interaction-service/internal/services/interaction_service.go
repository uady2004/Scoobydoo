package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/tiktok-clone/interaction-service/internal/config"
	"github.com/tiktok-clone/interaction-service/internal/models"
	"github.com/tiktok-clone/interaction-service/internal/repositories"
)

const (
	redisCacheKeyLikeCount = "like_count:%s" // %s = videoID
	cacheTTL               = 5 * time.Minute
)

// InteractionService exposes all interaction business logic.
type InteractionService interface {
	// Likes
	LikeVideo(ctx context.Context, userID, videoID string) error
	UnlikeVideo(ctx context.Context, userID, videoID string) error
	IsLiked(ctx context.Context, userID, videoID string) (bool, error)
	GetVideoLikeCount(ctx context.Context, videoID string) (int64, error)

	// Comments
	CreateComment(ctx context.Context, req CreateCommentReq) (*models.Comment, error)
	GetComment(ctx context.Context, id string) (*models.Comment, error)
	ListComments(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error)
	ListReplies(ctx context.Context, parentID string, limit, offset int) ([]*models.Comment, error)
	DeleteComment(ctx context.Context, id, userID string) error
	LikeComment(ctx context.Context, userID, commentID string) error
	UnlikeComment(ctx context.Context, userID, commentID string) error

	// Bookmarks
	BookmarkVideo(ctx context.Context, userID, videoID, collectionID string) error
	UnbookmarkVideo(ctx context.Context, userID, videoID string) error
	IsBookmarked(ctx context.Context, userID, videoID string) (bool, error)
	ListBookmarks(ctx context.Context, userID string, limit, offset int) ([]*models.Bookmark, error)
	CreateCollection(ctx context.Context, userID, name string, isPrivate bool) (*models.BookmarkCollection, error)
	ListCollections(ctx context.Context, userID string) ([]*models.BookmarkCollection, error)

	// Extended
	PinComment(ctx context.Context, commentID, userID string) error
	ReportContent(ctx context.Context, contentType, contentID, reporterID, reason string) error
	GetLikedVideos(ctx context.Context, userID string, limit, offset int) ([]string, error)
}

// CreateCommentReq carries the parameters for creating a comment.
type CreateCommentReq struct {
	VideoID   string
	UserID    string
	Username  string
	AvatarURL string
	Content   string
	ParentID  string // empty for top-level comments
}

type interactionService struct {
	repo        repositories.InteractionRepository
	rdb         *redis.Client
	kafkaWriter *kafka.Writer
	cfg         *config.Config
	logger      *zap.Logger
}

// NewInteractionService creates an InteractionService with all its dependencies.
func NewInteractionService(
	repo repositories.InteractionRepository,
	rdb *redis.Client,
	kafkaWriter *kafka.Writer,
	cfg *config.Config,
	logger *zap.Logger,
) InteractionService {
	return &interactionService{
		repo:        repo,
		rdb:         rdb,
		kafkaWriter: kafkaWriter,
		cfg:         cfg,
		logger:      logger,
	}
}

// ─── Likes ────────────────────────────────────────────────────────────────────

func (s *interactionService) LikeVideo(ctx context.Context, userID, videoID string) error {
	if err := s.repo.LikeVideo(ctx, userID, videoID); err != nil {
		return fmt.Errorf("liking video: %w", err)
	}

	// Invalidate cached like count.
	s.rdb.Del(ctx, fmt.Sprintf(redisCacheKeyLikeCount, videoID)) //nolint:errcheck

	// Publish event.
	s.publishKafka(ctx, s.cfg.Kafka.Topics.VideoLiked, videoID, map[string]string{
		"user_id":  userID,
		"video_id": videoID,
	})

	return nil
}

func (s *interactionService) UnlikeVideo(ctx context.Context, userID, videoID string) error {
	if err := s.repo.UnlikeVideo(ctx, userID, videoID); err != nil {
		return fmt.Errorf("unliking video: %w", err)
	}

	s.rdb.Del(ctx, fmt.Sprintf(redisCacheKeyLikeCount, videoID)) //nolint:errcheck

	s.publishKafka(ctx, s.cfg.Kafka.Topics.VideoUnliked, videoID, map[string]string{
		"user_id":  userID,
		"video_id": videoID,
	})

	return nil
}

func (s *interactionService) IsLiked(ctx context.Context, userID, videoID string) (bool, error) {
	return s.repo.IsLiked(ctx, userID, videoID)
}

func (s *interactionService) GetVideoLikeCount(ctx context.Context, videoID string) (int64, error) {
	// Try Redis cache first.
	key := fmt.Sprintf(redisCacheKeyLikeCount, videoID)
	if val, err := s.rdb.Get(ctx, key).Int64(); err == nil {
		return val, nil
	}

	count, err := s.repo.GetVideoLikeCount(ctx, videoID)
	if err != nil {
		return 0, err
	}

	// Cache the result.
	s.rdb.Set(ctx, key, count, cacheTTL) //nolint:errcheck
	return count, nil
}

// ─── Comments ─────────────────────────────────────────────────────────────────

func (s *interactionService) CreateComment(ctx context.Context, req CreateCommentReq) (*models.Comment, error) {
	if len(req.Content) == 0 || len(req.Content) > 1000 {
		return nil, fmt.Errorf("comment content must be 1-1000 characters")
	}

	c := &models.Comment{
		VideoID:   req.VideoID,
		UserID:    req.UserID,
		Username:  req.Username,
		AvatarURL: req.AvatarURL,
		Content:   req.Content,
		ParentID:  req.ParentID,
	}

	if err := s.repo.CreateComment(ctx, c); err != nil {
		return nil, fmt.Errorf("creating comment: %w", err)
	}

	s.publishKafka(ctx, s.cfg.Kafka.Topics.CommentCreated, c.ID, map[string]string{
		"comment_id": c.ID,
		"video_id":   c.VideoID,
		"user_id":    c.UserID,
	})

	return c, nil
}

func (s *interactionService) GetComment(ctx context.Context, id string) (*models.Comment, error) {
	return s.repo.GetComment(ctx, id)
}

func (s *interactionService) ListComments(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error) {
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListComments(ctx, videoID, limit, offset)
}

func (s *interactionService) ListReplies(ctx context.Context, parentID string, limit, offset int) ([]*models.Comment, error) {
	if limit > 50 {
		limit = 50
	}
	return s.repo.ListReplies(ctx, parentID, limit, offset)
}

func (s *interactionService) DeleteComment(ctx context.Context, id, userID string) error {
	if err := s.repo.DeleteComment(ctx, id, userID); err != nil {
		return fmt.Errorf("deleting comment: %w", err)
	}

	s.publishKafka(ctx, s.cfg.Kafka.Topics.CommentDeleted, id, map[string]string{
		"comment_id": id,
		"user_id":    userID,
	})

	return nil
}

func (s *interactionService) LikeComment(ctx context.Context, userID, commentID string) error {
	return s.repo.LikeComment(ctx, userID, commentID)
}

func (s *interactionService) UnlikeComment(ctx context.Context, userID, commentID string) error {
	return s.repo.UnlikeComment(ctx, userID, commentID)
}

// ─── Bookmarks ────────────────────────────────────────────────────────────────

func (s *interactionService) BookmarkVideo(ctx context.Context, userID, videoID, collectionID string) error {
	return s.repo.BookmarkVideo(ctx, userID, videoID, collectionID)
}

func (s *interactionService) UnbookmarkVideo(ctx context.Context, userID, videoID string) error {
	return s.repo.UnbookmarkVideo(ctx, userID, videoID)
}

func (s *interactionService) IsBookmarked(ctx context.Context, userID, videoID string) (bool, error) {
	return s.repo.IsBookmarked(ctx, userID, videoID)
}

func (s *interactionService) ListBookmarks(ctx context.Context, userID string, limit, offset int) ([]*models.Bookmark, error) {
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListBookmarks(ctx, userID, limit, offset)
}

func (s *interactionService) CreateCollection(ctx context.Context, userID, name string, isPrivate bool) (*models.BookmarkCollection, error) {
	col := &models.BookmarkCollection{
		UserID:    userID,
		Name:      name,
		IsPrivate: isPrivate,
	}
	if err := s.repo.CreateCollection(ctx, col); err != nil {
		return nil, fmt.Errorf("creating collection: %w", err)
	}
	return col, nil
}

func (s *interactionService) ListCollections(ctx context.Context, userID string) ([]*models.BookmarkCollection, error) {
	return s.repo.ListCollections(ctx, userID)
}

// ─── Extended ─────────────────────────────────────────────────────────────────

func (s *interactionService) PinComment(ctx context.Context, commentID, userID string) error {
	return s.repo.PinComment(ctx, commentID, userID)
}

func (s *interactionService) ReportContent(ctx context.Context, contentType, contentID, reporterID, reason string) error {
	if err := s.repo.ReportContent(ctx, contentType, contentID, reporterID, reason); err != nil {
		return fmt.Errorf("reporting content: %w", err)
	}
	s.publishKafka(ctx, "content.reported", contentID, map[string]string{
		"content_type": contentType,
		"content_id":   contentID,
		"reporter_id":  reporterID,
		"reason":       reason,
	})
	return nil
}

func (s *interactionService) GetLikedVideos(ctx context.Context, userID string, limit, offset int) ([]string, error) {
	if limit > 100 {
		limit = 100
	}
	return s.repo.GetLikedVideos(ctx, userID, limit, offset)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func (s *interactionService) publishKafka(ctx context.Context, topic, key string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Warn("kafka marshal failed", zap.Error(err))
		return
	}
	if err := s.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
		Time:  time.Now(),
	}); err != nil {
		s.logger.Warn("kafka write failed", zap.String("topic", topic), zap.Error(err))
	}
}
