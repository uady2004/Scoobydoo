package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tiktok-clone/like-service/internal/models"
)

var ErrNotFound = errors.New("not found")
var ErrAlreadyLiked = errors.New("already liked")

type LikeRepository interface {
	LikeVideo(ctx context.Context, userID, videoID string) error
	UnlikeVideo(ctx context.Context, userID, videoID string) error
	IsLiked(ctx context.Context, userID, videoID string) (bool, error)
	GetLikeCount(ctx context.Context, videoID string) (int64, error)
	BatchIsLiked(ctx context.Context, userID string, videoIDs []string) (map[string]bool, error)
	GetUserLikedVideos(ctx context.Context, userID string, limit, offset int) ([]*models.Like, error)
	GetTopLikedVideos(ctx context.Context, limit int) ([]*models.VideoLikeCount, error)
}

type pgLikeRepository struct {
	pool *pgxpool.Pool
}

func NewLikeRepository(pool *pgxpool.Pool) LikeRepository {
	return &pgLikeRepository{pool: pool}
}

func (r *pgLikeRepository) LikeVideo(ctx context.Context, userID, videoID string) error {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO video_likes (id, user_id, video_id, created_at)
		VALUES ($1,$2,$3,NOW())
		ON CONFLICT (user_id, video_id) DO NOTHING`,
		uuid.NewString(), userID, videoID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAlreadyLiked
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO video_like_counts (video_id, like_count)
		 VALUES ($1, 1)
		 ON CONFLICT (video_id) DO UPDATE SET like_count = video_like_counts.like_count + 1`, videoID)
	return err
}

func (r *pgLikeRepository) UnlikeVideo(ctx context.Context, userID, videoID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM video_likes WHERE user_id = $1 AND video_id = $2`, userID, videoID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE video_like_counts SET like_count = GREATEST(like_count - 1, 0) WHERE video_id = $1`, videoID)
	return err
}

func (r *pgLikeRepository) IsLiked(ctx context.Context, userID, videoID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM video_likes WHERE user_id = $1 AND video_id = $2)`,
		userID, videoID).Scan(&exists)
	return exists, err
}

func (r *pgLikeRepository) GetLikeCount(ctx context.Context, videoID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(like_count, 0) FROM video_like_counts WHERE video_id = $1`, videoID).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return count, err
}

func (r *pgLikeRepository) BatchIsLiked(ctx context.Context, userID string, videoIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(videoIDs))
	for _, id := range videoIDs {
		result[id] = false
	}

	rows, err := r.pool.Query(ctx,
		`SELECT video_id FROM video_likes WHERE user_id = $1 AND video_id = ANY($2)`,
		userID, videoIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var vid string
		if err := rows.Scan(&vid); err != nil {
			return nil, err
		}
		result[vid] = true
	}
	return result, rows.Err()
}

func (r *pgLikeRepository) GetUserLikedVideos(ctx context.Context, userID string, limit, offset int) ([]*models.Like, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, video_id, created_at FROM video_likes
		 WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Like
	for rows.Next() {
		l := &models.Like{}
		if err := rows.Scan(&l.ID, &l.UserID, &l.VideoID, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *pgLikeRepository) GetTopLikedVideos(ctx context.Context, limit int) ([]*models.VideoLikeCount, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT video_id, like_count FROM video_like_counts ORDER BY like_count DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.VideoLikeCount
	for rows.Next() {
		v := &models.VideoLikeCount{}
		if err := rows.Scan(&v.VideoID, &v.LikeCount); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
