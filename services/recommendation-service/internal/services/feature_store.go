package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/recommendation-service/internal/config"
	"github.com/tiktok-clone/recommendation-service/internal/models"
)

// FeatureStore retrieves and caches user and video feature vectors from Redis.
//
// Redis key schema:
//
//	rec:user:features:<userID>       – Hash: serialised UserFeatures JSON
//	rec:user:watch:<userID>          – List: last N video IDs (LPUSH/LTRIM)
//	rec:user:liked_cats:<userID>     – Hash: category → score (float)
//	rec:user:followed:<userID>       – Set: creator IDs
//	rec:video:features:<videoID>     – Hash: serialised VideoFeatures JSON
type FeatureStore struct {
	cfg    *config.Config
	rdb    redis.UniversalClient
	logger *zap.Logger
}

// NewFeatureStore constructs a FeatureStore.
func NewFeatureStore(
	cfg *config.Config,
	rdb redis.UniversalClient,
	logger *zap.Logger,
) *FeatureStore {
	return &FeatureStore{
		cfg:    cfg,
		rdb:    rdb,
		logger: logger,
	}
}

// -----------------------------------------------------------------
// User features
// -----------------------------------------------------------------

// GetUserFeatures retrieves all feature signals for a single user.
// It assembles the result from multiple Redis keys in a single pipeline.
func (fs *FeatureStore) GetUserFeatures(
	ctx context.Context,
	userID string,
	reqCtx models.RequestContext,
) (*models.UserFeatures, error) {

	watchKey := userWatchKey(userID)
	catsKey := userLikedCatsKey(userID)
	followedKey := userFollowedKey(userID)
	embeddingKey := userEmbeddingKey(userID)
	metaKey := userMetaKey(userID)

	fsCfg := fs.cfg.FeatureStore

	pipe := fs.rdb.Pipeline()

	// 1. Watch history – last N entries (LRANGE 0 N-1).
	watchCmd := pipe.LRange(ctx, watchKey, 0, int64(fsCfg.WatchHistorySize-1))
	// 2. Liked categories – Hash field→score.
	catsCmd := pipe.HGetAll(ctx, catsKey)
	// 3. Followed creators – Set members.
	followedCmd := pipe.SMembers(ctx, followedKey)
	// 4. User embedding – stored as a JSON-encoded float64 slice.
	embCmd := pipe.Get(ctx, embeddingKey)
	// 5. User meta (device_type, country, language, timezone).
	metaCmd := pipe.HGetAll(ctx, metaKey)

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("user feature pipeline exec: %w", err)
	}

	// ---- Watch history ------------------------------------------------
	watchHistory, _ := watchCmd.Result()

	// ---- Liked categories --------------------------------------------
	likedCategories := map[string]float64{}
	if cats, err := catsCmd.Result(); err == nil {
		likedCategories = parseFloatHash(cats)
	}

	// ---- Followed creators -------------------------------------------
	followedCreators, _ := followedCmd.Result()

	// ---- Embedding ---------------------------------------------------
	var embedding []float64
	if embJSON, err := embCmd.Result(); err == nil {
		_ = json.Unmarshal([]byte(embJSON), &embedding)
	}

	// ---- Device / locale meta ----------------------------------------
	deviceType := models.DeviceUnknown
	countryCode := ""
	languageCode := ""
	timezone := ""

	if meta, err := metaCmd.Result(); err == nil {
		if dt, ok := meta["device_type"]; ok {
			deviceType = models.DeviceType(dt)
		}
		countryCode = meta["country_code"]
		languageCode = meta["language_code"]
		timezone = meta["timezone"]
	}

	// Override with request-time context when available.
	if reqCtx.DeviceType != "" {
		deviceType = reqCtx.DeviceType
	}
	if reqCtx.CountryCode != "" {
		countryCode = reqCtx.CountryCode
	}
	if reqCtx.LanguageCode != "" {
		languageCode = reqCtx.LanguageCode
	}
	if reqCtx.Timezone != "" {
		timezone = reqCtx.Timezone
	}

	daypart := 0
	if !reqCtx.ClientTime.IsZero() {
		daypart = reqCtx.ClientTime.Hour()
	} else {
		daypart = time.Now().UTC().Hour()
	}

	isNewUser := len(watchHistory) < fs.cfg.ModelUpdate.MinInteractionsForItem

	return &models.UserFeatures{
		UserID:           userID,
		WatchHistory:     watchHistory,
		LikedCategories:  likedCategories,
		FollowedCreators: followedCreators,
		Embedding:        embedding,
		DeviceType:       deviceType,
		CountryCode:      countryCode,
		LanguageCode:     languageCode,
		Timezone:         timezone,
		DaypartIndex:     daypart,
		IsNewUser:        isNewUser,
		RetrievedAt:      time.Now(),
	}, nil
}

// UpdateWatchHistory prepends a video ID to the user's watch-history list and
// trims it to the configured maximum size.  Called after a confirmed view.
func (fs *FeatureStore) UpdateWatchHistory(ctx context.Context, userID, videoID string) error {
	key := userWatchKey(userID)
	pipe := fs.rdb.Pipeline()
	pipe.LPush(ctx, key, videoID)
	pipe.LTrim(ctx, key, 0, int64(fs.cfg.FeatureStore.WatchHistorySize-1))
	pipe.Expire(ctx, key, fs.cfg.FeatureStore.UserFeatureTTL*24) // extend TTL on activity
	_, err := pipe.Exec(ctx)
	return err
}

// IncrementCategoryAffinity increases the user's affinity score for a given
// category by delta and normalises all scores to [0, 1] relative to the max.
func (fs *FeatureStore) IncrementCategoryAffinity(
	ctx context.Context,
	userID, category string,
	delta float64,
) error {
	key := userLikedCatsKey(userID)
	pipe := fs.rdb.Pipeline()
	// HINCRBYFLOAT is atomic.
	incrCmd := pipe.HIncrByFloat(ctx, key, category, delta)
	pipe.Expire(ctx, key, fs.cfg.FeatureStore.UserFeatureTTL*48)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("increment category affinity: %w", err)
	}

	// After increment, check the new value and renormalise if it exceeds 1.
	newVal, err := incrCmd.Result()
	if err != nil {
		return err
	}
	if newVal > 1.0 {
		// Fetch all scores and divide by max to keep in [0, 1].
		if err := fs.normaliseCategoryScores(ctx, userID); err != nil {
			fs.logger.Warn("category normalisation failed", zap.Error(err))
		}
	}
	return nil
}

func (fs *FeatureStore) normaliseCategoryScores(ctx context.Context, userID string) error {
	key := userLikedCatsKey(userID)
	all, err := fs.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return err
	}
	maxVal := 0.0
	parsed := make(map[string]float64, len(all))
	for cat, scoreStr := range all {
		s, _ := strconv.ParseFloat(scoreStr, 64)
		parsed[cat] = s
		if s > maxVal {
			maxVal = s
		}
	}
	if maxVal == 0 {
		return nil
	}
	pipe := fs.rdb.Pipeline()
	for cat, s := range parsed {
		pipe.HSet(ctx, key, cat, s/maxVal)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// UpdateUserEmbedding stores the updated user embedding vector.
func (fs *FeatureStore) UpdateUserEmbedding(ctx context.Context, userID string, emb []float64) error {
	key := userEmbeddingKey(userID)
	data, err := json.Marshal(emb)
	if err != nil {
		return err
	}
	return fs.rdb.Set(ctx, key, data, fs.cfg.FeatureStore.UserFeatureTTL).Err()
}

// -----------------------------------------------------------------
// Video features
// -----------------------------------------------------------------

// GetVideoFeatures retrieves video features for a single video.
func (fs *FeatureStore) GetVideoFeatures(
	ctx context.Context,
	videoID string,
) (*models.VideoFeatures, error) {
	m, err := fs.GetVideoFeaturesBatch(ctx, []string{videoID})
	if err != nil {
		return nil, err
	}
	f, ok := m[videoID]
	if !ok {
		return nil, fmt.Errorf("video %s not found in feature store", videoID)
	}
	return f, nil
}

// GetVideoFeaturesBatch retrieves features for multiple videos in a single
// pipeline call.  Missing videos are silently omitted from the result map.
func (fs *FeatureStore) GetVideoFeaturesBatch(
	ctx context.Context,
	videoIDs []string,
) (map[string]*models.VideoFeatures, error) {

	if len(videoIDs) == 0 {
		return map[string]*models.VideoFeatures{}, nil
	}

	pipe := fs.rdb.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(videoIDs))
	for _, id := range videoIDs {
		key := videoFeaturesKey(id)
		cmds[id] = pipe.Get(ctx, key)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("video feature pipeline exec: %w", err)
	}

	result := make(map[string]*models.VideoFeatures, len(videoIDs))
	for id, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil {
			if err != redis.Nil {
				fs.logger.Warn("video feature get failed",
					zap.String("video_id", id), zap.Error(err))
			}
			continue
		}
		var f models.VideoFeatures
		if err := json.Unmarshal([]byte(data), &f); err != nil {
			fs.logger.Warn("video feature unmarshal failed",
				zap.String("video_id", id), zap.Error(err))
			continue
		}
		result[id] = &f
	}
	return result, nil
}

// SetVideoFeatures writes (or refreshes) a video's feature vector in Redis.
func (fs *FeatureStore) SetVideoFeatures(
	ctx context.Context,
	f *models.VideoFeatures,
) error {
	data, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal video features: %w", err)
	}
	key := videoFeaturesKey(f.VideoID)
	return fs.rdb.Set(ctx, key, data, fs.cfg.FeatureStore.VideoFeatureTTL).Err()
}

// InvalidateVideoFeatures removes cached video features, forcing a refresh on
// the next request.
func (fs *FeatureStore) InvalidateVideoFeatures(ctx context.Context, videoID string) error {
	return fs.rdb.Del(ctx, videoFeaturesKey(videoID)).Err()
}

// -----------------------------------------------------------------
// Engagement signal writers (called by the model-update worker)
// -----------------------------------------------------------------

// RecordEngagement persists a weighted engagement event to the user's liked
// set and to the video's co-user set.  Both are sorted sets scored by the
// interaction weight so downstream consumers can prioritise high-value signals.
func (fs *FeatureStore) RecordEngagement(
	ctx context.Context,
	ev *models.EngagementEvent,
) error {
	pipe := fs.rdb.Pipeline()

	// User's liked set (for graph-walk candidate generation).
	userLiked := userLikedKey(ev.UserID)
	pipe.ZAdd(ctx, userLiked, redis.Z{Score: ev.Score, Member: ev.VideoID})
	pipe.ZRemRangeByRank(ctx, userLiked, 0, -1001) // cap at 1000 entries

	// Video's co-user set (for graph-walk candidate generation).
	coUsers := videoCoUsersKey(ev.VideoID)
	pipe.ZAdd(ctx, coUsers, redis.Z{Score: ev.Score, Member: ev.UserID})
	pipe.ZRemRangeByRank(ctx, coUsers, 0, -501) // cap at 500 entries

	_, err := pipe.Exec(ctx)
	return err
}

// -----------------------------------------------------------------
// Redis key helpers
// -----------------------------------------------------------------

func userWatchKey(userID string) string {
	return fmt.Sprintf("rec:user:watch:%s", userID)
}

func userLikedCatsKey(userID string) string {
	return fmt.Sprintf("rec:user:liked_cats:%s", userID)
}

func userFollowedKey(userID string) string {
	return fmt.Sprintf("rec:user:followed:%s", userID)
}

func userEmbeddingKey(userID string) string {
	return fmt.Sprintf("rec:user:emb:%s", userID)
}

func userMetaKey(userID string) string {
	return fmt.Sprintf("rec:user:meta:%s", userID)
}

func videoFeaturesKey(videoID string) string {
	return fmt.Sprintf("rec:video:features:%s", videoID)
}

// -----------------------------------------------------------------
// Utility: parse a Redis hash into a float64 map
// -----------------------------------------------------------------

func parseFloatHash(m map[string]string) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			out[k] = f
		}
	}
	return out
}
