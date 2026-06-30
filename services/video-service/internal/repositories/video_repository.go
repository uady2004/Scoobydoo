package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tiktok-clone/video-service/internal/models"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

// VideoRepository performs all database operations related to videos.
type VideoRepository struct {
	db *pgxpool.Pool
}

// NewVideoRepository creates a new VideoRepository backed by the given pool.
func NewVideoRepository(db *pgxpool.Pool) *VideoRepository {
	return &VideoRepository{db: db}
}

// ---- Video CRUD -------------------------------------------------------------

// CreateVideo inserts a new video record and returns the fully populated model.
func (r *VideoRepository) CreateVideo(ctx context.Context, v *models.Video) (*models.Video, error) {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	v.CreatedAt = now
	v.UpdatedAt = now
	if v.Status == "" {
		v.Status = models.StatusDraft
	}
	if v.Visibility == "" {
		v.Visibility = models.VisibilityPrivate
	}

	tagsJSON, err := json.Marshal(v.Tags)
	if err != nil {
		return nil, fmt.Errorf("CreateVideo: marshal tags: %w", err)
	}

	const q = `
		INSERT INTO videos (
			id, user_id, title, description, status, visibility,
			raw_key, hls_key, hls_url,
			like_count, comment_count, share_count, view_count,
			tags, sound_id, publish_at, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,
			$10,$11,$12,$13,
			$14,$15,$16,$17,$18
		)
		RETURNING id, created_at, updated_at`

	row := r.db.QueryRow(ctx, q,
		v.ID, v.UserID, v.Title, v.Description, v.Status, v.Visibility,
		v.RawKey, v.HLSKey, v.HLSUrl,
		v.LikeCount, v.CommentCount, v.ShareCount, v.ViewCount,
		tagsJSON, v.SoundID, v.PublishAt, v.CreatedAt, v.UpdatedAt,
	)
	if err := row.Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, fmt.Errorf("CreateVideo: %w", err)
	}
	return v, nil
}

// UpdateStatus changes the processing status of a video.
func (r *VideoRepository) UpdateStatus(ctx context.Context, videoID string, status models.VideoStatus) error {
	const q = `UPDATE videos SET status=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, q, status, videoID)
	if err != nil {
		return fmt.Errorf("UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID retrieves a video by its primary key. Returns ErrNotFound if absent.
func (r *VideoRepository) GetByID(ctx context.Context, videoID string) (*models.Video, error) {
	const q = `
		SELECT id, user_id, title, description, status, visibility,
		       raw_key, hls_key, hls_url,
		       like_count, comment_count, share_count, view_count,
		       tags, sound_id, publish_at, created_at, updated_at, deleted_at
		FROM videos
		WHERE id=$1 AND deleted_at IS NULL`

	row := r.db.QueryRow(ctx, q, videoID)
	v, err := scanVideo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

// GetByUserID returns all non-deleted videos belonging to a user, newest first.
func (r *VideoRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.Video, error) {
	const q = `
		SELECT id, user_id, title, description, status, visibility,
		       raw_key, hls_key, hls_url,
		       like_count, comment_count, share_count, view_count,
		       tags, sound_id, publish_at, created_at, updated_at, deleted_at
		FROM videos
		WHERE user_id=$1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("GetByUserID: %w", err)
	}
	defer rows.Close()
	return collectVideos(rows)
}

// UpdateVideo applies partial updates from the request to an existing video.
func (r *VideoRepository) UpdateVideo(ctx context.Context, videoID string, req *models.UpdateVideoRequest) (*models.Video, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("UpdateVideo begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	v, err := getByIDTx(ctx, tx, videoID)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		v.Title = *req.Title
	}
	if req.Description != nil {
		v.Description = *req.Description
	}
	if req.Visibility != nil {
		v.Visibility = *req.Visibility
	}
	if req.Tags != nil {
		v.Tags = req.Tags
	}
	if req.SoundID != nil {
		v.SoundID = *req.SoundID
	}
	if req.PublishAt != nil {
		v.PublishAt = req.PublishAt
	}

	tagsJSON, _ := json.Marshal(v.Tags)

	const q = `
		UPDATE videos
		SET title=$1, description=$2, visibility=$3, tags=$4,
		    sound_id=$5, publish_at=$6, updated_at=NOW()
		WHERE id=$7
		RETURNING updated_at`

	if err := tx.QueryRow(ctx, q,
		v.Title, v.Description, v.Visibility, tagsJSON,
		v.SoundID, v.PublishAt, videoID,
	).Scan(&v.UpdatedAt); err != nil {
		return nil, fmt.Errorf("UpdateVideo exec: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("UpdateVideo commit: %w", err)
	}
	return v, nil
}

// DeleteVideo soft-deletes a video by setting deleted_at.
func (r *VideoRepository) DeleteVideo(ctx context.Context, videoID string) error {
	const q = `UPDATE videos SET deleted_at=NOW(), status=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, q, models.StatusDeleted, videoID)
	if err != nil {
		return fmt.Errorf("DeleteVideo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetDrafts returns all draft videos for a user.
func (r *VideoRepository) GetDrafts(ctx context.Context, userID string, limit, offset int) ([]*models.Video, error) {
	const q = `
		SELECT id, user_id, title, description, status, visibility,
		       raw_key, hls_key, hls_url,
		       like_count, comment_count, share_count, view_count,
		       tags, sound_id, publish_at, created_at, updated_at, deleted_at
		FROM videos
		WHERE user_id=$1 AND status=$2 AND deleted_at IS NULL
		ORDER BY updated_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, q, userID, models.StatusDraft, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("GetDrafts: %w", err)
	}
	defer rows.Close()
	return collectVideos(rows)
}

// SchedulePublish sets publish_at and transitions status to StatusScheduled.
func (r *VideoRepository) SchedulePublish(ctx context.Context, videoID string, publishAt time.Time) error {
	const q = `
		UPDATE videos
		SET publish_at=$1, status=$2, updated_at=NOW()
		WHERE id=$3 AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, publishAt, models.StatusScheduled, videoID)
	if err != nil {
		return fmt.Errorf("SchedulePublish: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetScheduledDue returns all videos with status=scheduled and publish_at <= now.
func (r *VideoRepository) GetScheduledDue(ctx context.Context) ([]*models.Video, error) {
	const q = `
		SELECT id, user_id, title, description, status, visibility,
		       raw_key, hls_key, hls_url,
		       like_count, comment_count, share_count, view_count,
		       tags, sound_id, publish_at, created_at, updated_at, deleted_at
		FROM videos
		WHERE status=$1 AND publish_at <= NOW() AND deleted_at IS NULL`

	rows, err := r.db.Query(ctx, q, models.StatusScheduled)
	if err != nil {
		return nil, fmt.Errorf("GetScheduledDue: %w", err)
	}
	defer rows.Close()
	return collectVideos(rows)
}

// GetTrending returns the top-N public/ready videos ordered by view_count desc.
func (r *VideoRepository) GetTrending(ctx context.Context, limit int) ([]*models.Video, error) {
	const q = `
		SELECT id, user_id, title, description, status, visibility,
		       raw_key, hls_key, hls_url,
		       like_count, comment_count, share_count, view_count,
		       tags, sound_id, publish_at, created_at, updated_at, deleted_at
		FROM videos
		WHERE status=$1 AND visibility=$2 AND deleted_at IS NULL
		ORDER BY view_count DESC, like_count DESC
		LIMIT $3`

	rows, err := r.db.Query(ctx, q, models.StatusReady, models.VisibilityPublic, limit)
	if err != nil {
		return nil, fmt.Errorf("GetTrending: %w", err)
	}
	defer rows.Close()
	return collectVideos(rows)
}

// ---- Chunk operations -------------------------------------------------------

// SaveChunk inserts a VideoChunk record (one chunk within a resumable upload).
func (r *VideoRepository) SaveChunk(ctx context.Context, chunk *models.VideoChunk) error {
	if chunk.ID == "" {
		chunk.ID = uuid.New().String()
	}
	chunk.CreatedAt = time.Now().UTC()

	const q = `
		INSERT INTO video_chunks (id, upload_id, video_id, chunk_index, s3_key, size, checksum, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (upload_id, chunk_index) DO UPDATE
		  SET s3_key=$5, size=$6, checksum=$7, created_at=$8`

	_, err := r.db.Exec(ctx, q,
		chunk.ID, chunk.UploadID, chunk.VideoID, chunk.Index,
		chunk.S3Key, chunk.Size, chunk.Checksum, chunk.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveChunk: %w", err)
	}
	return nil
}

// GetChunks returns all stored chunks for a given upload, ordered by index.
func (r *VideoRepository) GetChunks(ctx context.Context, uploadID string) ([]*models.VideoChunk, error) {
	const q = `
		SELECT id, upload_id, video_id, chunk_index, s3_key, size, checksum, created_at
		FROM video_chunks
		WHERE upload_id=$1
		ORDER BY chunk_index ASC`

	rows, err := r.db.Query(ctx, q, uploadID)
	if err != nil {
		return nil, fmt.Errorf("GetChunks: %w", err)
	}
	defer rows.Close()

	var chunks []*models.VideoChunk
	for rows.Next() {
		c := &models.VideoChunk{}
		if err := rows.Scan(&c.ID, &c.UploadID, &c.VideoID, &c.Index,
			&c.S3Key, &c.Size, &c.Checksum, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("GetChunks scan: %w", err)
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// DeleteChunks removes all chunk records for a completed or abandoned upload.
func (r *VideoRepository) DeleteChunks(ctx context.Context, uploadID string) error {
	const q = `DELETE FROM video_chunks WHERE upload_id=$1`
	_, err := r.db.Exec(ctx, q, uploadID)
	return err
}

// ---- Thumbnail operations ---------------------------------------------------

// SaveThumbnail inserts or updates a thumbnail record for a video.
func (r *VideoRepository) SaveThumbnail(ctx context.Context, t *models.Thumbnail) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	t.CreatedAt = time.Now().UTC()

	const q = `
		INSERT INTO thumbnails (id, video_id, s3_key, url, width, height, offset_secs, is_cover, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (video_id, offset_secs) DO UPDATE
		  SET s3_key=$3, url=$4, width=$5, height=$6, is_cover=$8`

	_, err := r.db.Exec(ctx, q,
		t.ID, t.VideoID, t.S3Key, t.URL,
		t.Width, t.Height, t.OffsetSecs, t.IsCover, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveThumbnail: %w", err)
	}
	return nil
}

// GetThumbnails returns all thumbnails for a video.
func (r *VideoRepository) GetThumbnails(ctx context.Context, videoID string) ([]*models.Thumbnail, error) {
	const q = `
		SELECT id, video_id, s3_key, url, width, height, offset_secs, is_cover, created_at
		FROM thumbnails
		WHERE video_id=$1
		ORDER BY is_cover DESC, offset_secs ASC`

	rows, err := r.db.Query(ctx, q, videoID)
	if err != nil {
		return nil, fmt.Errorf("GetThumbnails: %w", err)
	}
	defer rows.Close()

	var thumbs []*models.Thumbnail
	for rows.Next() {
		t := &models.Thumbnail{}
		if err := rows.Scan(&t.ID, &t.VideoID, &t.S3Key, &t.URL,
			&t.Width, &t.Height, &t.OffsetSecs, &t.IsCover, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("GetThumbnails scan: %w", err)
		}
		thumbs = append(thumbs, t)
	}
	return thumbs, rows.Err()
}

// ---- Subtitle operations ----------------------------------------------------

// SaveSubtitle inserts or updates a subtitle track.
func (r *VideoRepository) SaveSubtitle(ctx context.Context, s *models.Subtitle) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	s.CreatedAt = time.Now().UTC()

	const q = `
		INSERT INTO subtitles (id, video_id, language, format, s3_key, url, auto_generated, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (video_id, language, format) DO UPDATE
		  SET s3_key=$5, url=$6, auto_generated=$7`

	_, err := r.db.Exec(ctx, q,
		s.ID, s.VideoID, s.Language, s.Format,
		s.S3Key, s.URL, s.AutoGenerated, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveSubtitle: %w", err)
	}
	return nil
}

// GetSubtitles returns all subtitle tracks for a video.
func (r *VideoRepository) GetSubtitles(ctx context.Context, videoID string) ([]*models.Subtitle, error) {
	const q = `
		SELECT id, video_id, language, format, s3_key, url, auto_generated, created_at
		FROM subtitles
		WHERE video_id=$1
		ORDER BY language ASC`

	rows, err := r.db.Query(ctx, q, videoID)
	if err != nil {
		return nil, fmt.Errorf("GetSubtitles: %w", err)
	}
	defer rows.Close()

	var subs []*models.Subtitle
	for rows.Next() {
		s := &models.Subtitle{}
		if err := rows.Scan(&s.ID, &s.VideoID, &s.Language, &s.Format,
			&s.S3Key, &s.URL, &s.AutoGenerated, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("GetSubtitles scan: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// ---- Metadata operations ----------------------------------------------------

// SaveMetadata upserts the technical metadata record for a video.
func (r *VideoRepository) SaveMetadata(ctx context.Context, m *models.VideoMetadata) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	m.CreatedAt = now
	m.UpdatedAt = now

	qualJSON, err := json.Marshal(m.Qualities)
	if err != nil {
		return fmt.Errorf("SaveMetadata marshal qualities: %w", err)
	}

	const q = `
		INSERT INTO video_metadata (
			id, video_id, duration_secs, width, height,
			video_codec, audio_codec, frame_rate,
			file_size_bytes, mime_type, qualities, aspect_ratio,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (video_id) DO UPDATE
		  SET duration_secs=$3, width=$4, height=$5,
		      video_codec=$6, audio_codec=$7, frame_rate=$8,
		      file_size_bytes=$9, mime_type=$10, qualities=$11,
		      aspect_ratio=$12, updated_at=$14`

	_, err = r.db.Exec(ctx, q,
		m.ID, m.VideoID, m.DurationSecs, m.Width, m.Height,
		m.VideoCodec, m.AudioCodec, m.FrameRate,
		m.FileSizeBytes, m.MIMEType, qualJSON, m.AspectRatio,
		m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveMetadata: %w", err)
	}
	return nil
}

// UpdateMetadata applies a partial update to video_metadata.
func (r *VideoRepository) UpdateMetadata(ctx context.Context, m *models.VideoMetadata) error {
	return r.SaveMetadata(ctx, m)
}

// GetMetadata returns the metadata record for a video.
func (r *VideoRepository) GetMetadata(ctx context.Context, videoID string) (*models.VideoMetadata, error) {
	const q = `
		SELECT id, video_id, duration_secs, width, height,
		       video_codec, audio_codec, frame_rate,
		       file_size_bytes, mime_type, qualities, aspect_ratio,
		       created_at, updated_at
		FROM video_metadata
		WHERE video_id=$1`

	row := r.db.QueryRow(ctx, q, videoID)

	m := &models.VideoMetadata{}
	var qualJSON []byte
	err := row.Scan(
		&m.ID, &m.VideoID, &m.DurationSecs, &m.Width, &m.Height,
		&m.VideoCodec, &m.AudioCodec, &m.FrameRate,
		&m.FileSizeBytes, &m.MIMEType, &qualJSON, &m.AspectRatio,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetMetadata: %w", err)
	}
	if len(qualJSON) > 0 {
		if err := json.Unmarshal(qualJSON, &m.Qualities); err != nil {
			return nil, fmt.Errorf("GetMetadata unmarshal qualities: %w", err)
		}
	}
	return m, nil
}

// ---- HLS / publish ----------------------------------------------------------

// UpdateHLS sets the HLS master-playlist key and URL, and transitions status to ready.
func (r *VideoRepository) UpdateHLS(ctx context.Context, videoID, hlsKey, hlsURL string) error {
	const q = `
		UPDATE videos
		SET hls_key=$1, hls_url=$2, status=$3, updated_at=NOW()
		WHERE id=$4 AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, hlsKey, hlsURL, models.StatusReady, videoID)
	if err != nil {
		return fmt.Errorf("UpdateHLS: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PublishVideo transitions a video to StatusReady and sets visibility to public.
func (r *VideoRepository) PublishVideo(ctx context.Context, videoID string) error {
	const q = `
		UPDATE videos
		SET status=$1, visibility=$2, publish_at=NOW(), updated_at=NOW()
		WHERE id=$3 AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, models.StatusReady, models.VisibilityPublic, videoID)
	if err != nil {
		return fmt.Errorf("PublishVideo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- internal helpers -------------------------------------------------------

// getByIDTx retrieves a video within an existing transaction.
func getByIDTx(ctx context.Context, tx pgx.Tx, videoID string) (*models.Video, error) {
	const q = `
		SELECT id, user_id, title, description, status, visibility,
		       raw_key, hls_key, hls_url,
		       like_count, comment_count, share_count, view_count,
		       tags, sound_id, publish_at, created_at, updated_at, deleted_at
		FROM videos
		WHERE id=$1 AND deleted_at IS NULL`

	row := tx.QueryRow(ctx, q, videoID)
	v, err := scanVideo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

// scanVideo scans a single video row from any pgx Row.
func scanVideo(row pgx.Row) (*models.Video, error) {
	v := &models.Video{}
	var tagsJSON []byte
	err := row.Scan(
		&v.ID, &v.UserID, &v.Title, &v.Description, &v.Status, &v.Visibility,
		&v.RawKey, &v.HLSKey, &v.HLSUrl,
		&v.LikeCount, &v.CommentCount, &v.ShareCount, &v.ViewCount,
		&tagsJSON, &v.SoundID, &v.PublishAt,
		&v.CreatedAt, &v.UpdatedAt, &v.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &v.Tags)
	}
	return v, nil
}

// collectVideos iterates pgx.Rows into a slice of videos.
func collectVideos(rows pgx.Rows) ([]*models.Video, error) {
	var videos []*models.Video
	for rows.Next() {
		v := &models.Video{}
		var tagsJSON []byte
		err := rows.Scan(
			&v.ID, &v.UserID, &v.Title, &v.Description, &v.Status, &v.Visibility,
			&v.RawKey, &v.HLSKey, &v.HLSUrl,
			&v.LikeCount, &v.CommentCount, &v.ShareCount, &v.ViewCount,
			&tagsJSON, &v.SoundID, &v.PublishAt,
			&v.CreatedAt, &v.UpdatedAt, &v.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("collectVideos scan: %w", err)
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &v.Tags)
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}
