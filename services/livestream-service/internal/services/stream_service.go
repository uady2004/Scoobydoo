package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/tiktok-clone/livestream-service/internal/config"
	"github.com/tiktok-clone/livestream-service/internal/models"
	"github.com/tiktok-clone/livestream-service/internal/repositories"
)

// Redis key helpers
const (
	redisViewerCountKey = "livestream:viewers:%s"   // %s = streamID
	redisStreamMetaKey  = "livestream:meta:%s"       // %s = streamID
	redisStreamKeyIndex = "livestream:rtmp_key:%s"   // %s = rtmpKey
	redisActiveStreams  = "livestream:active"
)

var (
	ErrStreamNotFound  = errors.New("stream not found")
	ErrStreamNotLive   = errors.New("stream is not live")
	ErrStreamNotOwned  = errors.New("not the stream owner")
	ErrViewerNotFound  = errors.New("viewer not found")
)

// StreamService handles stream lifecycle and viewer management.
type StreamService interface {
	StartStream(ctx context.Context, req StartStreamRequest) (*models.LiveStream, error)
	EndStream(ctx context.Context, streamID, userID string) error
	GetStream(ctx context.Context, id string) (*models.LiveStream, error)
	GetActiveStreams(ctx context.Context, limit, offset int) ([]*models.LiveStream, error)
	GetStreamsByUser(ctx context.Context, userID string, limit, offset int) ([]*models.LiveStream, error)
	UpdateViewerCount(ctx context.Context, streamID string, count int64) error
	JoinStream(ctx context.Context, req JoinStreamRequest) (*models.LiveViewer, error)
	LeaveStream(ctx context.Context, streamID, userID string) error
	GetActiveViewers(ctx context.Context, streamID string, limit int) ([]*models.LiveViewer, error)
	ValidateRTMPKey(ctx context.Context, rtmpKey string) (*models.LiveStream, error)
	UpdateHLSPlaylistURL(ctx context.Context, streamID, url string) error
}

// StartStreamRequest carries parameters for starting a new stream.
type StartStreamRequest struct {
	UserID        string
	Title         string
	Description   string
	CategoryID    string
	Tags          []string
	Language      string
	AgeRestricted bool
	AllowComments bool
	IsRecorded    bool
}

// JoinStreamRequest carries parameters for a viewer joining a stream.
type JoinStreamRequest struct {
	StreamID  string
	UserID    string
	Username  string
	AvatarURL string
}

type streamService struct {
	repo        repositories.LivestreamRepository
	rdb         redis.UniversalClient
	kafkaWriter *kafka.Writer
	cfg         *config.Config
	logger      *zap.Logger
}

// NewStreamService creates a StreamService with all its dependencies.
func NewStreamService(
	repo repositories.LivestreamRepository,
	rdb redis.UniversalClient,
	kafkaWriter *kafka.Writer,
	cfg *config.Config,
	logger *zap.Logger,
) StreamService {
	return &streamService{
		repo:        repo,
		rdb:         rdb,
		kafkaWriter: kafkaWriter,
		cfg:         cfg,
		logger:      logger,
	}
}

// StartStream creates a new livestream record, generates an RTMP ingest key,
// and publishes a LivestreamStarted event to Kafka.
func (s *streamService) StartStream(ctx context.Context, req StartStreamRequest) (*models.LiveStream, error) {
	rtmpKey, err := generateStreamKey(s.cfg.RTMP.StreamKeyPrefix)
	if err != nil {
		return nil, fmt.Errorf("generating stream key: %w", err)
	}

	now := time.Now().UTC()
	stream := &models.LiveStream{
		ID:            uuid.New().String(),
		UserID:        req.UserID,
		Title:         req.Title,
		Description:   req.Description,
		RTMPKey:       rtmpKey,
		HLSPlaylistURL: "", // filled in by HLS service after transcoder starts
		Status:        models.StreamStatusPending,
		ViewerCount:   0,
		CategoryID:    req.CategoryID,
		Tags:          req.Tags,
		Language:      req.Language,
		AgeRestricted: req.AgeRestricted,
		AllowComments: req.AllowComments,
		IsRecorded:    req.IsRecorded,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Attach the full ingest URL (not stored in DB for security).
	stream.RTMPIngestURL = fmt.Sprintf("rtmp://%s/%s/%s",
		s.cfg.RTMP.Addr(), s.cfg.RTMP.AppName, rtmpKey,
	)

	if err := s.repo.CreateStream(ctx, stream); err != nil {
		return nil, fmt.Errorf("persisting stream: %w", err)
	}

	// Cache RTMP key -> streamID in Redis for fast ingest-time lookup.
	rtmpKeyIndex := fmt.Sprintf(redisStreamKeyIndex, rtmpKey)
	if err := s.rdb.Set(ctx, rtmpKeyIndex, stream.ID, s.cfg.Redis.StreamMetaTTL).Err(); err != nil {
		s.logger.Warn("failed to cache rtmp key index", zap.Error(err))
	}

	// Cache stream metadata.
	if err := s.cacheStreamMeta(ctx, stream); err != nil {
		s.logger.Warn("failed to cache stream meta", zap.Error(err))
	}

	// Publish LivestreamStarted event.
	event := models.KafkaLivestreamStarted{
		StreamID:  stream.ID,
		UserID:    stream.UserID,
		Title:     stream.Title,
		StartedAt: now,
	}
	if err := s.publishKafka(ctx, s.cfg.Kafka.Topics.LivestreamStarted, stream.ID, event); err != nil {
		s.logger.Warn("failed to publish LivestreamStarted event", zap.Error(err))
	}

	s.logger.Info("stream created",
		zap.String("stream_id", stream.ID),
		zap.String("user_id", stream.UserID),
	)
	return stream, nil
}

// EndStream transitions a stream to ended status and publishes LivestreamEnded.
func (s *streamService) EndStream(ctx context.Context, streamID, userID string) error {
	stream, err := s.repo.GetStreamByID(ctx, streamID)
	if err != nil {
		return ErrStreamNotFound
	}
	if stream.UserID != userID {
		return ErrStreamNotOwned
	}

	now := time.Now().UTC()
	stream.Status = models.StreamStatusEnded
	stream.EndedAt = &now

	if err := s.repo.UpdateStream(ctx, stream); err != nil {
		return fmt.Errorf("updating stream: %w", err)
	}

	// Remove from Redis active set and delete viewer count key.
	pipe := s.rdb.Pipeline()
	pipe.ZRem(ctx, redisActiveStreams, streamID)
	pipe.Del(ctx, fmt.Sprintf(redisViewerCountKey, streamID))
	pipe.Del(ctx, fmt.Sprintf(redisStreamMetaKey, streamID))
	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.Warn("redis cleanup on EndStream failed", zap.Error(err))
	}

	// Compute duration.
	var durationSecs int64
	if stream.StartedAt != nil {
		durationSecs = int64(now.Sub(*stream.StartedAt).Seconds())
	}

	event := models.KafkaLivestreamEnded{
		StreamID:        streamID,
		UserID:          userID,
		EndedAt:         now,
		PeakViewerCount: stream.PeakViewerCount,
		TotalGiftCoins:  stream.TotalGiftCoins,
		DurationSecs:    durationSecs,
	}
	if err := s.publishKafka(ctx, s.cfg.Kafka.Topics.LivestreamEnded, streamID, event); err != nil {
		s.logger.Warn("failed to publish LivestreamEnded event", zap.Error(err))
	}

	s.logger.Info("stream ended",
		zap.String("stream_id", streamID),
		zap.Int64("duration_secs", durationSecs),
	)
	return nil
}

// GetStream returns a stream, preferring the Redis cache.
func (s *streamService) GetStream(ctx context.Context, id string) (*models.LiveStream, error) {
	// Try cache first.
	if cached, err := s.getStreamFromCache(ctx, id); err == nil {
		return cached, nil
	}
	stream, err := s.repo.GetStreamByID(ctx, id)
	if err != nil {
		return nil, ErrStreamNotFound
	}
	_ = s.cacheStreamMeta(ctx, stream)
	return stream, nil
}

// GetActiveStreams returns currently live streams ordered by viewer count.
func (s *streamService) GetActiveStreams(ctx context.Context, limit, offset int) ([]*models.LiveStream, error) {
	return s.repo.GetActiveStreams(ctx, limit, offset)
}

// GetStreamsByUser returns all streams for a given user.
func (s *streamService) GetStreamsByUser(ctx context.Context, userID string, limit, offset int) ([]*models.LiveStream, error) {
	return s.repo.GetStreamsByUserID(ctx, userID, limit, offset)
}

// UpdateViewerCount persists the viewer count (from Redis) to Postgres.
func (s *streamService) UpdateViewerCount(ctx context.Context, streamID string, count int64) error {
	return s.repo.UpdateStreamViewerCount(ctx, streamID, count)
}

// JoinStream registers a viewer, increments the Redis counter, and updates Postgres.
func (s *streamService) JoinStream(ctx context.Context, req JoinStreamRequest) (*models.LiveViewer, error) {
	stream, err := s.repo.GetStreamByID(ctx, req.StreamID)
	if err != nil {
		return nil, ErrStreamNotFound
	}
	if stream.Status != models.StreamStatusLive {
		return nil, ErrStreamNotLive
	}

	viewer := &models.LiveViewer{
		ID:        uuid.New().String(),
		StreamID:  req.StreamID,
		UserID:    req.UserID,
		Username:  req.Username,
		AvatarURL: req.AvatarURL,
		Status:    models.ViewerStatusJoined,
		JoinedAt:  time.Now().UTC(),
	}

	if err := s.repo.UpsertViewer(ctx, viewer); err != nil {
		return nil, fmt.Errorf("upserting viewer: %w", err)
	}

	// INCR viewer count in Redis; update Postgres with the new value.
	viewerKey := fmt.Sprintf(redisViewerCountKey, req.StreamID)
	newCount, err := s.rdb.Incr(ctx, viewerKey).Result()
	if err != nil {
		s.logger.Warn("redis INCR viewer count failed", zap.Error(err))
	} else {
		s.rdb.Expire(ctx, viewerKey, s.cfg.Redis.ViewerCountTTL) //nolint:errcheck
		_ = s.repo.UpdateStreamViewerCount(ctx, req.StreamID, newCount)
	}

	return viewer, nil
}

// LeaveStream marks a viewer as left and DECRs the Redis counter.
func (s *streamService) LeaveStream(ctx context.Context, streamID, userID string) error {
	viewer, err := s.repo.GetViewer(ctx, streamID, userID)
	if err != nil {
		return ErrViewerNotFound
	}

	now := time.Now().UTC()
	watchSecs := int64(now.Sub(viewer.JoinedAt).Seconds())

	if err := s.repo.UpdateViewerStatus(ctx, streamID, userID, models.ViewerStatusLeft, &now, watchSecs); err != nil {
		return fmt.Errorf("updating viewer status: %w", err)
	}

	// DECR viewer count (floor at 0).
	viewerKey := fmt.Sprintf(redisViewerCountKey, streamID)
	newCount, err := s.rdb.Decr(ctx, viewerKey).Result()
	if err != nil {
		s.logger.Warn("redis DECR viewer count failed", zap.Error(err))
	} else {
		if newCount < 0 {
			s.rdb.Set(ctx, viewerKey, 0, s.cfg.Redis.ViewerCountTTL) //nolint:errcheck
			newCount = 0
		}
		_ = s.repo.UpdateStreamViewerCount(ctx, streamID, newCount)
	}

	return nil
}

// GetActiveViewers returns the list of joined viewers.
func (s *streamService) GetActiveViewers(ctx context.Context, streamID string, limit int) ([]*models.LiveViewer, error) {
	return s.repo.GetActiveViewers(ctx, streamID, limit)
}

// ValidateRTMPKey checks whether an incoming RTMP stream key is valid and returns the stream.
func (s *streamService) ValidateRTMPKey(ctx context.Context, rtmpKey string) (*models.LiveStream, error) {
	// Fast path: look up streamID from Redis.
	indexKey := fmt.Sprintf(redisStreamKeyIndex, rtmpKey)
	streamID, err := s.rdb.Get(ctx, indexKey).Result()
	if err == nil && streamID != "" {
		return s.GetStream(ctx, streamID)
	}

	// Slow path: query Postgres.
	stream, err := s.repo.GetStreamByRTMPKey(ctx, rtmpKey)
	if err != nil {
		return nil, ErrStreamNotFound
	}
	if stream.Status == models.StreamStatusEnded || stream.Status == models.StreamStatusBanned {
		return nil, fmt.Errorf("stream key is no longer active: %s", stream.Status)
	}
	return stream, nil
}

// UpdateHLSPlaylistURL stores the generated HLS URL once the transcoder is running.
func (s *streamService) UpdateHLSPlaylistURL(ctx context.Context, streamID, url string) error {
	stream, err := s.repo.GetStreamByID(ctx, streamID)
	if err != nil {
		return ErrStreamNotFound
	}
	stream.HLSPlaylistURL = url
	stream.Status = models.StreamStatusLive

	now := time.Now().UTC()
	stream.StartedAt = &now

	if err := s.repo.UpdateStream(ctx, stream); err != nil {
		return err
	}

	// Add to Redis sorted set of active streams (score = unix timestamp for TTL ordering).
	s.rdb.ZAdd(ctx, redisActiveStreams, redis.Z{
		Score:  float64(now.Unix()),
		Member: streamID,
	}) //nolint:errcheck

	_ = s.cacheStreamMeta(ctx, stream)
	return nil
}

// ─── private helpers ─────────────────────────────────────────────────────────

func generateStreamKey(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

func (s *streamService) cacheStreamMeta(ctx context.Context, stream *models.LiveStream) error {
	data, err := json.Marshal(stream)
	if err != nil {
		return err
	}
	key := fmt.Sprintf(redisStreamMetaKey, stream.ID)
	return s.rdb.Set(ctx, key, data, s.cfg.Redis.StreamMetaTTL).Err()
}

func (s *streamService) getStreamFromCache(ctx context.Context, id string) (*models.LiveStream, error) {
	key := fmt.Sprintf(redisStreamMetaKey, id)
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var stream models.LiveStream
	if err := json.Unmarshal(data, &stream); err != nil {
		return nil, err
	}
	return &stream, nil
}

func (s *streamService) publishKafka(ctx context.Context, topic, key string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
		Time:  time.Now(),
	})
}
