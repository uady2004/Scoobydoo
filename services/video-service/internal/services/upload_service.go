package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/video-service/internal/config"
	"github.com/tiktok-clone/video-service/internal/models"
	"github.com/tiktok-clone/video-service/internal/repositories"
)

// uploadSessionPrefix is the Redis key prefix for upload session metadata.
const uploadSessionPrefix = "upload:session:"

// UploadService manages chunked video uploads.
type UploadService struct {
	cfg      *config.Config
	repo     *repositories.VideoRepository
	s3Client *s3.Client
	redis    *redis.Client
	logger   *zap.Logger
}

// NewUploadService constructs an UploadService.
func NewUploadService(
	cfg *config.Config,
	repo *repositories.VideoRepository,
	s3Client *s3.Client,
	redisClient *redis.Client,
	logger *zap.Logger,
) *UploadService {
	return &UploadService{
		cfg:      cfg,
		repo:     repo,
		s3Client: s3Client,
		redis:    redisClient,
		logger:   logger,
	}
}

// InitiateUpload validates the request, creates a video draft, and stores an
// UploadSession in Redis. It returns the upload ID and video ID.
func (s *UploadService) InitiateUpload(ctx context.Context, req *models.InitiateUploadRequest, userID string) (*models.InitiateUploadResponse, error) {
	// Validate MIME type.
	if err := s.validateMIME(req.MIMEType); err != nil {
		return nil, err
	}

	// Validate file size.
	if req.TotalSize > s.cfg.Upload.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed %d bytes",
			req.TotalSize, s.cfg.Upload.MaxFileSize)
	}

	// Validate chunk parameters.
	if req.ChunkSize > req.TotalSize {
		return nil, errors.New("chunk_size must not exceed total_size")
	}

	// Derive a safe extension from the MIME type.
	exts, _ := mime.ExtensionsByType(req.MIMEType)
	ext := ".mp4"
	if len(exts) > 0 {
		ext = exts[0]
	}

	uploadID := uuid.New().String()
	videoID := uuid.New().String()

	// Create draft video record.
	video := &models.Video{
		ID:         videoID,
		UserID:     userID,
		Title:      sanitizeFilename(req.Filename),
		Status:     models.StatusUploading,
		Visibility: models.VisibilityPrivate,
		RawKey:     rawS3Key(videoID, ext),
	}
	if _, err := s.repo.CreateVideo(ctx, video); err != nil {
		return nil, fmt.Errorf("InitiateUpload create video: %w", err)
	}

	// Store session in Redis.
	session := &models.UploadSession{
		UploadID:       uploadID,
		VideoID:        videoID,
		UserID:         userID,
		Filename:       req.Filename,
		MIMEType:       req.MIMEType,
		TotalSize:      req.TotalSize,
		TotalChunks:    req.TotalChunks,
		ChunkSize:      req.ChunkSize,
		ReceivedChunks: []int{},
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      time.Now().UTC().Add(s.cfg.Upload.ExpireAfter),
	}
	if err := s.saveSession(ctx, session); err != nil {
		return nil, fmt.Errorf("InitiateUpload save session: %w", err)
	}

	s.logger.Info("upload initiated",
		zap.String("upload_id", uploadID),
		zap.String("video_id", videoID),
		zap.String("user_id", userID),
		zap.Int64("total_size", req.TotalSize),
		zap.Int("total_chunks", req.TotalChunks),
	)

	return &models.InitiateUploadResponse{
		UploadID:    uploadID,
		VideoID:     videoID,
		TotalChunks: req.TotalChunks,
		ChunkSize:   req.ChunkSize,
	}, nil
}

// UploadChunk stores a single chunk to S3 and records it in Redis + Postgres.
func (s *UploadService) UploadChunk(ctx context.Context, uploadID string, chunkIndex int, data io.Reader, size int64) error {
	session, err := s.loadSession(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("UploadChunk load session: %w", err)
	}

	if chunkIndex < 0 || chunkIndex >= session.TotalChunks {
		return fmt.Errorf("chunk index %d out of range [0, %d)", chunkIndex, session.TotalChunks)
	}

	// Buffer chunk so we can compute checksum and also upload to S3.
	buf := &bytes.Buffer{}
	written, err := io.CopyN(buf, data, size)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("UploadChunk read data: %w", err)
	}
	if written == 0 {
		return errors.New("UploadChunk: empty chunk data")
	}

	// Compute SHA-256 checksum.
	h := sha256.Sum256(buf.Bytes())
	checksum := hex.EncodeToString(h[:])

	// Build S3 key for this chunk.
	chunkKey := chunkS3Key(session.VideoID, uploadID, chunkIndex)

	// Upload chunk to S3 temp bucket.
	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.cfg.S3.TempBucket),
		Key:           aws.String(chunkKey),
		Body:          bytes.NewReader(buf.Bytes()),
		ContentLength: aws.Int64(written),
		ContentType:   aws.String("application/octet-stream"),
		Metadata: map[string]string{
			"upload-id":   uploadID,
			"video-id":    session.VideoID,
			"chunk-index": fmt.Sprintf("%d", chunkIndex),
			"checksum":    checksum,
		},
	})
	if err != nil {
		return fmt.Errorf("UploadChunk S3 put: %w", err)
	}

	// Persist chunk record to Postgres.
	chunk := &models.VideoChunk{
		UploadID: uploadID,
		VideoID:  session.VideoID,
		Index:    chunkIndex,
		S3Key:    chunkKey,
		Size:     written,
		Checksum: checksum,
	}
	if err := s.repo.SaveChunk(ctx, chunk); err != nil {
		return fmt.Errorf("UploadChunk save chunk record: %w", err)
	}

	// Update session progress in Redis.
	session.ReceivedChunks = addChunkIndex(session.ReceivedChunks, chunkIndex)
	if err := s.saveSession(ctx, session); err != nil {
		return fmt.Errorf("UploadChunk save session: %w", err)
	}

	s.logger.Debug("chunk received",
		zap.String("upload_id", uploadID),
		zap.Int("chunk_index", chunkIndex),
		zap.Int64("size", written),
	)
	return nil
}

// CompleteUpload assembles all received chunks into a single object in the main
// S3 bucket, triggers transcoding, then cleans up temp chunks.
func (s *UploadService) CompleteUpload(ctx context.Context, uploadID string) (*models.Video, error) {
	session, err := s.loadSession(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("CompleteUpload load session: %w", err)
	}

	// Verify all chunks have been received.
	if len(session.ReceivedChunks) != session.TotalChunks {
		missing := missingChunks(session.ReceivedChunks, session.TotalChunks)
		return nil, fmt.Errorf("CompleteUpload: %d chunks missing: %v",
			len(missing), missing)
	}

	// Fetch ordered chunk records from DB.
	chunks, err := s.repo.GetChunks(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("CompleteUpload get chunks: %w", err)
	}

	// Assemble chunks into a local temp file.
	if err := os.MkdirAll(s.cfg.Upload.TempDir, 0o750); err != nil {
		return nil, fmt.Errorf("CompleteUpload mkdir: %w", err)
	}
	tmpFile, err := os.CreateTemp(s.cfg.Upload.TempDir, fmt.Sprintf("upload-%s-*.tmp", uploadID))
	if err != nil {
		return nil, fmt.Errorf("CompleteUpload create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	for _, chunk := range chunks {
		if err := s.appendChunkToFile(ctx, tmpFile, chunk); err != nil {
			return nil, fmt.Errorf("CompleteUpload assemble chunk %d: %w", chunk.Index, err)
		}
	}

	// Seek back to the beginning before uploading to the final bucket.
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("CompleteUpload seek: %w", err)
	}

	stat, err := tmpFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("CompleteUpload stat: %w", err)
	}

	// Upload assembled file to the main bucket at the video's raw key.
	video, err := s.repo.GetByID(ctx, session.VideoID)
	if err != nil {
		return nil, fmt.Errorf("CompleteUpload get video: %w", err)
	}

	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.cfg.S3.Bucket),
		Key:           aws.String(video.RawKey),
		Body:          tmpFile,
		ContentLength: aws.Int64(stat.Size()),
		ContentType:   aws.String(session.MIMEType),
	})
	if err != nil {
		return nil, fmt.Errorf("CompleteUpload S3 final put: %w", err)
	}

	// Transition video status to processing.
	if err := s.repo.UpdateStatus(ctx, session.VideoID, models.StatusProcessing); err != nil {
		return nil, fmt.Errorf("CompleteUpload update status: %w", err)
	}

	// Clean up temp S3 objects and DB chunk records.
	go s.cleanupChunks(context.Background(), uploadID, chunks)

	// Delete Redis session.
	s.redis.Del(ctx, uploadSessionKey(uploadID))

	s.logger.Info("upload completed",
		zap.String("upload_id", uploadID),
		zap.String("video_id", session.VideoID),
		zap.Int64("assembled_size", stat.Size()),
	)

	return s.repo.GetByID(ctx, session.VideoID)
}

// ResumeUpload returns which chunks are still missing so the client can retransmit them.
func (s *UploadService) ResumeUpload(ctx context.Context, uploadID string) (*models.UploadProgress, error) {
	session, err := s.loadSession(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("ResumeUpload: %w", err)
	}
	missing := missingChunks(session.ReceivedChunks, session.TotalChunks)
	pct := 0.0
	if session.TotalChunks > 0 {
		pct = float64(len(session.ReceivedChunks)) / float64(session.TotalChunks) * 100
	}
	return &models.UploadProgress{
		UploadID:       uploadID,
		VideoID:        session.VideoID,
		TotalChunks:    session.TotalChunks,
		ReceivedChunks: len(session.ReceivedChunks),
		PercentDone:    pct,
		MissingChunks:  missing,
		Status:         "uploading",
	}, nil
}

// GetUploadProgress returns the current progress for an in-flight upload.
func (s *UploadService) GetUploadProgress(ctx context.Context, uploadID string) (*models.UploadProgress, error) {
	return s.ResumeUpload(ctx, uploadID)
}

// ---- private helpers --------------------------------------------------------

func (s *UploadService) validateMIME(mimeType string) error {
	for _, allowed := range s.cfg.Upload.AllowedMIMEs {
		if allowed == mimeType {
			return nil
		}
	}
	return fmt.Errorf("MIME type %q is not allowed", mimeType)
}

func (s *UploadService) saveSession(ctx context.Context, session *models.UploadSession) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		ttl = s.cfg.Upload.ExpireAfter
	}
	return s.redis.Set(ctx, uploadSessionKey(session.UploadID), data, ttl).Err()
}

func (s *UploadService) loadSession(ctx context.Context, uploadID string) (*models.UploadSession, error) {
	data, err := s.redis.Get(ctx, uploadSessionKey(uploadID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("upload session %q not found or expired", uploadID)
		}
		return nil, err
	}
	var session models.UploadSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *UploadService) appendChunkToFile(ctx context.Context, dst *os.File, chunk *models.VideoChunk) error {
	out, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.S3.TempBucket),
		Key:    aws.String(chunk.S3Key),
	})
	if err != nil {
		return fmt.Errorf("get chunk from S3: %w", err)
	}
	defer out.Body.Close()

	if _, err := io.Copy(dst, out.Body); err != nil {
		return fmt.Errorf("write chunk to temp file: %w", err)
	}
	return nil
}

func (s *UploadService) cleanupChunks(ctx context.Context, uploadID string, chunks []*models.VideoChunk) {
	for _, c := range chunks {
		_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.cfg.S3.TempBucket),
			Key:    aws.String(c.S3Key),
		})
		if err != nil {
			s.logger.Warn("failed to delete temp chunk",
				zap.String("upload_id", uploadID),
				zap.String("s3_key", c.S3Key),
				zap.Error(err),
			)
		}
	}
	if err := s.repo.DeleteChunks(ctx, uploadID); err != nil {
		s.logger.Warn("failed to delete chunk records",
			zap.String("upload_id", uploadID),
			zap.Error(err),
		)
	}
}

// ---- key helpers ------------------------------------------------------------

func uploadSessionKey(uploadID string) string {
	return uploadSessionPrefix + uploadID
}

func rawS3Key(videoID, ext string) string {
	return filepath.Join("raw", videoID, "original"+ext)
}

func chunkS3Key(videoID, uploadID string, index int) string {
	return filepath.Join("chunks", videoID, uploadID, fmt.Sprintf("chunk-%06d", index))
}

func sanitizeFilename(name string) string {
	base := filepath.Base(name)
	// Strip extension for use as a title.
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}
	if base == "" {
		return "Untitled"
	}
	return base
}

// addChunkIndex inserts idx into sorted slice (no-op if already present).
func addChunkIndex(slice []int, idx int) []int {
	for _, v := range slice {
		if v == idx {
			return slice
		}
	}
	slice = append(slice, idx)
	sort.Ints(slice)
	return slice
}

// missingChunks returns the sorted list of chunk indices not in received.
func missingChunks(received []int, total int) []int {
	set := make(map[int]struct{}, len(received))
	for _, v := range received {
		set[v] = struct{}{}
	}
	var missing []int
	for i := 0; i < total; i++ {
		if _, ok := set[i]; !ok {
			missing = append(missing, i)
		}
	}
	return missing
}
