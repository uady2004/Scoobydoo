package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tiktok-clone/interaction-service/internal/models"
)

var ErrNotFound = errors.New("not found")
var ErrAlreadyExists = errors.New("already exists")

// InteractionRepository defines all DB operations for interactions.
type InteractionRepository interface {
	// Likes
	LikeVideo(ctx context.Context, userID, videoID string) error
	UnlikeVideo(ctx context.Context, userID, videoID string) error
	IsLiked(ctx context.Context, userID, videoID string) (bool, error)
	GetVideoLikeCount(ctx context.Context, videoID string) (int64, error)

	// Comments
	CreateComment(ctx context.Context, comment *models.Comment) error
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
	CreateCollection(ctx context.Context, col *models.BookmarkCollection) error
	ListCollections(ctx context.Context, userID string) ([]*models.BookmarkCollection, error)

	// Extended
	PinComment(ctx context.Context, commentID, userID string) error
	ReportContent(ctx context.Context, contentType, contentID, reporterID, reason string) error
	GetLikedVideos(ctx context.Context, userID string, limit, offset int) ([]string, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewInteractionRepository creates a postgres-backed InteractionRepository.
func NewInteractionRepository(pool *pgxpool.Pool) InteractionRepository {
	return &pgRepository{pool: pool}
}

// ─── Likes ────────────────────────────────────────────────────────────────────

func (r *pgRepository) LikeVideo(ctx context.Context, userID, videoID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO video_likes (id, user_id, video_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, video_id) DO NOTHING`,
		uuid.New().String(), userID, videoID, time.Now().UTC(),
	)
	return err
}

func (r *pgRepository) UnlikeVideo(ctx context.Context, userID, videoID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM video_likes WHERE user_id=$1 AND video_id=$2`, userID, videoID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) IsLiked(ctx context.Context, userID, videoID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM video_likes WHERE user_id=$1 AND video_id=$2)`,
		userID, videoID,
	).Scan(&exists)
	return exists, err
}

func (r *pgRepository) GetVideoLikeCount(ctx context.Context, videoID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM video_likes WHERE video_id=$1`, videoID,
	).Scan(&count)
	return count, err
}

// ─── Comments ─────────────────────────────────────────────────────────────────

func (r *pgRepository) CreateComment(ctx context.Context, c *models.Comment) error {
	c.ID = uuid.New().String()
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO comments (id, video_id, user_id, username, avatar_url, content, parent_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9)`,
		c.ID, c.VideoID, c.UserID, c.Username, c.AvatarURL, c.Content,
		c.ParentID, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

func (r *pgRepository) GetComment(ctx context.Context, id string) (*models.Comment, error) {
	c := &models.Comment{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, video_id, user_id, username, avatar_url, content,
		       COALESCE(parent_id,''), like_count, reply_count, is_deleted, created_at, updated_at
		FROM comments WHERE id=$1 AND is_deleted=false`, id,
	).Scan(&c.ID, &c.VideoID, &c.UserID, &c.Username, &c.AvatarURL, &c.Content,
		&c.ParentID, &c.LikeCount, &c.ReplyCount, &c.IsDeleted, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (r *pgRepository) ListComments(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, video_id, user_id, username, avatar_url, content,
		       COALESCE(parent_id,''), like_count, reply_count, is_deleted, created_at, updated_at
		FROM comments
		WHERE video_id=$1 AND parent_id IS NULL AND is_deleted=false
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, videoID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComments(rows)
}

func (r *pgRepository) ListReplies(ctx context.Context, parentID string, limit, offset int) ([]*models.Comment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, video_id, user_id, username, avatar_url, content,
		       COALESCE(parent_id,''), like_count, reply_count, is_deleted, created_at, updated_at
		FROM comments
		WHERE parent_id=$1 AND is_deleted=false
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`, parentID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComments(rows)
}

func (r *pgRepository) DeleteComment(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE comments SET is_deleted=true, updated_at=$1 WHERE id=$2 AND user_id=$3`,
		time.Now().UTC(), id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) LikeComment(ctx context.Context, userID, commentID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		INSERT INTO comment_likes (id, user_id, comment_id, created_at)
		VALUES ($1,$2,$3,$4) ON CONFLICT (user_id, comment_id) DO NOTHING`,
		uuid.New().String(), userID, commentID, time.Now().UTC())
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE comments SET like_count=like_count+1 WHERE id=$1`, commentID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *pgRepository) UnlikeComment(ctx context.Context, userID, commentID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx,
		`DELETE FROM comment_likes WHERE user_id=$1 AND comment_id=$2`, userID, commentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = tx.Exec(ctx,
		`UPDATE comments SET like_count=GREATEST(like_count-1,0) WHERE id=$1`, commentID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ─── Bookmarks ────────────────────────────────────────────────────────────────

func (r *pgRepository) BookmarkVideo(ctx context.Context, userID, videoID, collectionID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bookmarks (id, user_id, video_id, collection_id, created_at)
		VALUES ($1,$2,$3,NULLIF($4,''),$5)
		ON CONFLICT (user_id, video_id) DO NOTHING`,
		uuid.New().String(), userID, videoID, collectionID, time.Now().UTC())
	return err
}

func (r *pgRepository) UnbookmarkVideo(ctx context.Context, userID, videoID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM bookmarks WHERE user_id=$1 AND video_id=$2`, userID, videoID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) IsBookmarked(ctx context.Context, userID, videoID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM bookmarks WHERE user_id=$1 AND video_id=$2)`,
		userID, videoID,
	).Scan(&exists)
	return exists, err
}

func (r *pgRepository) ListBookmarks(ctx context.Context, userID string, limit, offset int) ([]*models.Bookmark, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, video_id, COALESCE(collection_id,''), created_at
		FROM bookmarks WHERE user_id=$1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Bookmark
	for rows.Next() {
		b := &models.Bookmark{}
		if err := rows.Scan(&b.ID, &b.UserID, &b.VideoID, &b.CollectionID, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *pgRepository) CreateCollection(ctx context.Context, col *models.BookmarkCollection) error {
	col.ID = uuid.New().String()
	col.CreatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bookmark_collections (id, user_id, name, is_private, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		col.ID, col.UserID, col.Name, col.IsPrivate, col.CreatedAt)
	return err
}

func (r *pgRepository) ListCollections(ctx context.Context, userID string) ([]*models.BookmarkCollection, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT bc.id, bc.user_id, bc.name, bc.is_private, bc.created_at,
		       COUNT(b.id) AS video_count
		FROM bookmark_collections bc
		LEFT JOIN bookmarks b ON b.collection_id=bc.id
		WHERE bc.user_id=$1
		GROUP BY bc.id ORDER BY bc.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.BookmarkCollection
	for rows.Next() {
		c := &models.BookmarkCollection{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.IsPrivate, &c.CreatedAt, &c.VideoCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ─── Extended ─────────────────────────────────────────────────────────────────

func (r *pgRepository) PinComment(ctx context.Context, commentID, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE comments SET is_pinned=true, updated_at=$1 WHERE id=$2 AND is_deleted=false`,
		time.Now().UTC(), commentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) ReportContent(ctx context.Context, contentType, contentID, reporterID, reason string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO content_reports (id, content_type, content_id, reporter_id, reason, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT DO NOTHING`,
		uuid.New().String(), contentType, contentID, reporterID, reason, time.Now().UTC())
	return err
}

func (r *pgRepository) GetLikedVideos(ctx context.Context, userID string, limit, offset int) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT video_id FROM video_likes WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func scanComments(rows pgx.Rows) ([]*models.Comment, error) {
	var out []*models.Comment
	for rows.Next() {
		c := &models.Comment{}
		if err := rows.Scan(
			&c.ID, &c.VideoID, &c.UserID, &c.Username, &c.AvatarURL,
			&c.Content, &c.ParentID, &c.LikeCount, &c.ReplyCount,
			&c.IsDeleted, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
