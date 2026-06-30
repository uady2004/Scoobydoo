package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tiktok-clone/comment-service/internal/models"
)

var ErrNotFound = errors.New("not found")
var ErrAlreadyExists = errors.New("already exists")

type CommentRepository interface {
	Create(ctx context.Context, req models.CreateCommentReq) (*models.Comment, error)
	GetByID(ctx context.Context, id string) (*models.Comment, error)
	ListByVideo(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error)
	ListReplies(ctx context.Context, parentID string, limit, offset int) ([]*models.Comment, error)
	SoftDelete(ctx context.Context, id, userID string) error
	IncrementLikes(ctx context.Context, commentID string) error
	DecrementLikes(ctx context.Context, commentID string) error
	LikeComment(ctx context.Context, userID, commentID string) error
	UnlikeComment(ctx context.Context, userID, commentID string) error
	IsCommentLiked(ctx context.Context, userID, commentID string) (bool, error)
}

type pgCommentRepository struct {
	pool *pgxpool.Pool
}

func NewCommentRepository(pool *pgxpool.Pool) CommentRepository {
	return &pgCommentRepository{pool: pool}
}

func (r *pgCommentRepository) Create(ctx context.Context, req models.CreateCommentReq) (*models.Comment, error) {
	c := &models.Comment{
		ID:        uuid.NewString(),
		VideoID:   req.VideoID,
		UserID:    req.UserID,
		Username:  req.Username,
		AvatarURL: req.AvatarURL,
		Content:   req.Content,
		ParentID:  req.ParentID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO comments (id, video_id, user_id, username, avatar_url, content, parent_id, like_count, reply_count, is_deleted, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,$12)`,
		c.ID, c.VideoID, c.UserID, c.Username, c.AvatarURL, c.Content, c.ParentID,
		0, 0, false, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if req.ParentID != "" {
		_, _ = r.pool.Exec(ctx,
			`UPDATE comments SET reply_count = reply_count + 1 WHERE id = $1`, req.ParentID)
	}
	return c, nil
}

func (r *pgCommentRepository) GetByID(ctx context.Context, id string) (*models.Comment, error) {
	c := &models.Comment{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, video_id, user_id, username, avatar_url, content,
		       COALESCE(parent_id,''), like_count, reply_count, is_deleted, created_at, updated_at
		FROM comments WHERE id = $1`, id).Scan(
		&c.ID, &c.VideoID, &c.UserID, &c.Username, &c.AvatarURL, &c.Content,
		&c.ParentID, &c.LikeCount, &c.ReplyCount, &c.IsDeleted, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (r *pgCommentRepository) ListByVideo(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, video_id, user_id, username, avatar_url, content,
		       COALESCE(parent_id,''), like_count, reply_count, is_deleted, created_at, updated_at
		FROM comments
		WHERE video_id = $1 AND (parent_id IS NULL OR parent_id = '') AND is_deleted = false
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, videoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComments(rows)
}

func (r *pgCommentRepository) ListReplies(ctx context.Context, parentID string, limit, offset int) ([]*models.Comment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, video_id, user_id, username, avatar_url, content,
		       COALESCE(parent_id,''), like_count, reply_count, is_deleted, created_at, updated_at
		FROM comments
		WHERE parent_id = $1 AND is_deleted = false
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`, parentID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComments(rows)
}

func (r *pgCommentRepository) SoftDelete(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE comments SET is_deleted = true, updated_at = NOW() WHERE id = $1 AND user_id = $2`,
		id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgCommentRepository) IncrementLikes(ctx context.Context, commentID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE comments SET like_count = like_count + 1 WHERE id = $1`, commentID)
	return err
}

func (r *pgCommentRepository) DecrementLikes(ctx context.Context, commentID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE comments SET like_count = GREATEST(like_count - 1, 0) WHERE id = $1`, commentID)
	return err
}

func (r *pgCommentRepository) LikeComment(ctx context.Context, userID, commentID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO comment_likes (id, user_id, comment_id, created_at)
		VALUES ($1,$2,$3,NOW())
		ON CONFLICT (user_id, comment_id) DO NOTHING`,
		uuid.NewString(), userID, commentID)
	return err
}

func (r *pgCommentRepository) UnlikeComment(ctx context.Context, userID, commentID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM comment_likes WHERE user_id = $1 AND comment_id = $2`, userID, commentID)
	return err
}

func (r *pgCommentRepository) IsCommentLiked(ctx context.Context, userID, commentID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM comment_likes WHERE user_id = $1 AND comment_id = $2)`,
		userID, commentID).Scan(&exists)
	return exists, err
}

func scanComments(rows pgx.Rows) ([]*models.Comment, error) {
	var out []*models.Comment
	for rows.Next() {
		c := &models.Comment{}
		if err := rows.Scan(
			&c.ID, &c.VideoID, &c.UserID, &c.Username, &c.AvatarURL, &c.Content,
			&c.ParentID, &c.LikeCount, &c.ReplyCount, &c.IsDeleted, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
