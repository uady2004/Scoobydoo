package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tiktok-clone/recommendation-service/internal/config"
	"github.com/tiktok-clone/recommendation-service/internal/models"
	"github.com/tiktok-clone/recommendation-service/internal/services"
)

// RecommendationHandler handles both gRPC and REST requests for the
// recommendation service.  The gRPC interface is expressed via plain method
// receivers; a separate gRPC proto adapter (generated code) calls these.
type RecommendationHandler struct {
	cfg              *config.Config
	candidateGen     *services.CandidateGenerator
	ranker           *services.RankingService
	featureStore     *services.FeatureStore
	embeddingSvc     *services.EmbeddingService
	abTesting        *services.ABTestingService
	logger           *zap.Logger
}

// NewRecommendationHandler constructs a RecommendationHandler wiring together
// all service dependencies.
func NewRecommendationHandler(
	cfg *config.Config,
	candidateGen *services.CandidateGenerator,
	ranker *services.RankingService,
	featureStore *services.FeatureStore,
	embeddingSvc *services.EmbeddingService,
	abTesting *services.ABTestingService,
	logger *zap.Logger,
) *RecommendationHandler {
	return &RecommendationHandler{
		cfg:          cfg,
		candidateGen: candidateGen,
		ranker:       ranker,
		featureStore: featureStore,
		embeddingSvc: embeddingSvc,
		abTesting:    abTesting,
		logger:       logger,
	}
}

// RegisterRoutes attaches all REST routes to the provided Gin router group.
func (h *RecommendationHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/recommendations/:user_id", h.HTTPGetRecommendations)
	rg.POST("/recommendations", h.HTTPPostRecommendations)
	rg.POST("/impressions", h.HTTPRecordImpression)
	rg.POST("/feedback", h.HTTPRecordFeedback)
	rg.GET("/health", h.HTTPHealth)
}

// =============================================================================
// gRPC handler methods
// These are called by the generated gRPC server adapter.
// =============================================================================

// GetRecommendations is the core gRPC/business-logic handler.  It:
//  1. Loads the user's feature vector from the feature store.
//  2. Computes / retrieves the user's embedding.
//  3. Assigns an A/B experiment variant (if any).
//  4. Runs multi-source candidate generation.
//  5. Passes candidates through the multi-stage ranker.
//  6. Marks served videos as seen to prevent repetition.
//  7. Returns the final ranked feed.
func (h *RecommendationHandler) GetRecommendations(
	ctx context.Context,
	req *models.RecommendationRequest,
) (*models.RecommendationResponse, error) {

	if req.UserID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = h.cfg.Recommendation.FinalFeedSize
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// ---- Step 1: load user features ----------------------------------------
	userFeatures, err := h.featureStore.GetUserFeatures(ctx, req.UserID, req.Context)
	if err != nil {
		h.logger.Error("get user features failed",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, status.Errorf(codes.Internal, "feature store error: %v", err)
	}

	// ---- Step 2: compute / refresh user embedding --------------------------
	// Use cached embedding if available; recompute if stale or absent.
	if len(userFeatures.Embedding) != h.cfg.Embedding.Dimensions {
		embedding, embErr := h.embeddingSvc.ComputeUserEmbedding(ctx, userFeatures)
		if embErr != nil {
			h.logger.Warn("user embedding computation failed, continuing without",
				zap.String("user_id", req.UserID),
				zap.Error(embErr))
		} else {
			userFeatures.Embedding = embedding
		}
	}

	// ---- Step 3: A/B experiment assignment ---------------------------------
	experimentID := ""
	if assignment := h.abTesting.AssignVariant(ctx, req.UserID, req.Context.CountryCode); assignment != nil {
		experimentID = assignment.ExperimentID
		h.logger.Debug("A/B assignment",
			zap.String("user_id", req.UserID),
			zap.String("experiment_id", experimentID),
			zap.String("variant_id", assignment.VariantID))
	}

	// ---- Step 4: candidate generation --------------------------------------
	candidates, err := h.candidateGen.GenerateAll(ctx, userFeatures)
	if err != nil {
		h.logger.Error("candidate generation failed",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		// Non-fatal: rank whatever candidates we managed to gather so far.
	}

	if len(candidates) == 0 {
		h.logger.Warn("no candidates generated",
			zap.String("user_id", req.UserID))
		return &models.RecommendationResponse{
			UserID:       req.UserID,
			Items:        []*models.RankedResult{},
			ExperimentID: experimentID,
			GeneratedAt:  time.Now(),
		}, nil
	}

	h.logger.Debug("candidates generated",
		zap.String("user_id", req.UserID),
		zap.Int("count", len(candidates)))

	// ---- Step 5: multi-stage ranking ---------------------------------------
	rankedItems, err := h.ranker.Rank(ctx, candidates, userFeatures, experimentID)
	if err != nil {
		h.logger.Error("ranking failed",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		return nil, status.Errorf(codes.Internal, "ranking error: %v", err)
	}

	// Apply page size limit.
	if len(rankedItems) > pageSize {
		rankedItems = rankedItems[:pageSize]
	}

	// ---- Step 6: mark videos as seen ---------------------------------------
	seenIDs := make([]string, len(rankedItems))
	for i, item := range rankedItems {
		seenIDs[i] = item.VideoID
	}
	if markErr := h.ranker.MarkSeen(ctx, req.UserID, seenIDs); markErr != nil {
		h.logger.Warn("mark-seen failed",
			zap.String("user_id", req.UserID),
			zap.Error(markErr))
	}

	// ---- Step 7: track A/B impressions ------------------------------------
	if experimentID != "" {
		if assignment := h.abTesting.GetAssignment(ctx, req.UserID, experimentID); assignment != nil {
			h.abTesting.TrackImpression(ctx, experimentID, assignment.VariantID)
		}
	}

	h.logger.Info("recommendations generated",
		zap.String("user_id", req.UserID),
		zap.Int("count", len(rankedItems)),
		zap.String("experiment_id", experimentID))

	return &models.RecommendationResponse{
		UserID:       req.UserID,
		Items:        rankedItems,
		ExperimentID: experimentID,
		GeneratedAt:  time.Now(),
	}, nil
}

// RecordImpression persists that a set of videos was shown to a user.
// It updates the seen-set (already done by GetRecommendations for normal flows)
// and tracks A/B experiment impressions.
func (h *RecommendationHandler) RecordImpression(
	ctx context.Context,
	events []*models.ImpressionEvent,
) error {
	if len(events) == 0 {
		return nil
	}

	// Group by user to batch Redis writes.
	byUser := make(map[string][]string, len(events))
	for _, ev := range events {
		if ev.UserID == "" || ev.VideoID == "" {
			continue
		}
		byUser[ev.UserID] = append(byUser[ev.UserID], ev.VideoID)
	}

	for userID, videoIDs := range byUser {
		if err := h.ranker.MarkSeen(ctx, userID, videoIDs); err != nil {
			h.logger.Warn("mark seen in RecordImpression failed",
				zap.String("user_id", userID),
				zap.Error(err))
		}
	}

	// Track A/B impressions.
	for _, ev := range events {
		if ev.ExperimentID == "" {
			continue
		}
		if assignment := h.abTesting.GetAssignment(ctx, ev.UserID, ev.ExperimentID); assignment != nil {
			h.abTesting.TrackImpression(ctx, ev.ExperimentID, assignment.VariantID)
		}
	}

	return nil
}

// RecordFeedback processes engagement signals (likes, shares, watches, etc.)
// and updates the user's feature store.
func (h *RecommendationHandler) RecordFeedback(
	ctx context.Context,
	event *models.FeedbackEvent,
) error {
	if event == nil || event.UserID == "" || event.VideoID == "" {
		return status.Error(codes.InvalidArgument, "user_id and video_id are required")
	}

	weight, ok := models.EngagementWeights[event.Interaction]
	if !ok {
		return status.Errorf(codes.InvalidArgument, "unknown interaction type: %s", event.Interaction)
	}

	// Construct a normalised engagement event.
	engEvent := &models.EngagementEvent{
		UserID:    event.UserID,
		VideoID:   event.VideoID,
		Type:      event.Interaction,
		Score:     weight,
		Timestamp: event.Timestamp,
	}
	if engEvent.Timestamp.IsZero() {
		engEvent.Timestamp = time.Now()
	}

	// Persist to the feature store (updates liked set and co-user set).
	if err := h.featureStore.RecordEngagement(ctx, engEvent); err != nil {
		h.logger.Warn("record engagement failed",
			zap.String("user_id", event.UserID),
			zap.String("video_id", event.VideoID),
			zap.Error(err))
	}

	// Update watch history on significant watch events.
	if event.Interaction == models.InteractionView && event.WatchPercentage >= 0.8 {
		if err := h.featureStore.UpdateWatchHistory(ctx, event.UserID, event.VideoID); err != nil {
			h.logger.Warn("update watch history failed",
				zap.String("user_id", event.UserID),
				zap.Error(err))
		}
	}

	// Update category affinity if video features are available.
	videoFeats, featErr := h.featureStore.GetVideoFeatures(ctx, event.VideoID)
	if featErr == nil && videoFeats != nil && videoFeats.Category != "" {
		affinityDelta := weight * 0.1 // dampen the direct signal
		if affinityDelta < 0 {
			affinityDelta = 0
		}
		if err := h.featureStore.IncrementCategoryAffinity(
			ctx, event.UserID, videoFeats.Category, affinityDelta,
		); err != nil {
			h.logger.Warn("category affinity update failed",
				zap.String("user_id", event.UserID),
				zap.Error(err))
		}
	}

	// Track A/B conversion if within an experiment.
	if event.ExperimentID != "" {
		if assignment := h.abTesting.GetAssignment(ctx, event.UserID, event.ExperimentID); assignment != nil {
			h.abTesting.TrackConversion(ctx, event.ExperimentID, assignment.VariantID, string(event.Interaction))
		}
	}

	h.logger.Debug("feedback recorded",
		zap.String("user_id", event.UserID),
		zap.String("video_id", event.VideoID),
		zap.String("interaction", string(event.Interaction)))

	return nil
}

// =============================================================================
// REST (HTTP/JSON) endpoints – thin adapters that call the gRPC handlers.
// =============================================================================

// HTTPGetRecommendations handles GET /recommendations/:user_id
// Query params: page_size, country_code, language_code, device_type
func (h *RecommendationHandler) HTTPGetRecommendations(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	pageSize := h.cfg.Recommendation.FinalFeedSize
	if ps := c.Query("page_size"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 {
			pageSize = n
		}
	}

	req := &models.RecommendationRequest{
		UserID:   userID,
		PageSize: pageSize,
		Cursor:   c.Query("cursor"),
		Context: models.RequestContext{
			DeviceType:   models.DeviceType(c.Query("device_type")),
			CountryCode:  c.Query("country_code"),
			LanguageCode: c.Query("language_code"),
			Timezone:     c.Query("timezone"),
			AppVersion:   c.Query("app_version"),
			ClientTime:   time.Now(),
		},
	}

	resp, err := h.GetRecommendations(c.Request.Context(), req)
	if err != nil {
		httpStatus, msg := grpcStatusToHTTP(err)
		c.JSON(httpStatus, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// HTTPPostRecommendations handles POST /recommendations with a JSON body.
func (h *RecommendationHandler) HTTPPostRecommendations(c *gin.Context) {
	var req models.RecommendationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	resp, err := h.GetRecommendations(c.Request.Context(), &req)
	if err != nil {
		httpStatus, msg := grpcStatusToHTTP(err)
		c.JSON(httpStatus, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// HTTPRecordImpression handles POST /impressions
func (h *RecommendationHandler) HTTPRecordImpression(c *gin.Context) {
	var events []*models.ImpressionEvent
	if err := c.ShouldBindJSON(&events); err != nil {
		// Also accept a single event.
		var single models.ImpressionEvent
		if jsonErr := json.NewDecoder(c.Request.Body).Decode(&single); jsonErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
			return
		}
		events = []*models.ImpressionEvent{&single}
	}

	if err := h.RecordImpression(c.Request.Context(), events); err != nil {
		httpStatus, msg := grpcStatusToHTTP(err)
		c.JSON(httpStatus, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// HTTPRecordFeedback handles POST /feedback
func (h *RecommendationHandler) HTTPRecordFeedback(c *gin.Context) {
	var event models.FeedbackEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	if err := h.RecordFeedback(c.Request.Context(), &event); err != nil {
		httpStatus, msg := grpcStatusToHTTP(err)
		c.JSON(httpStatus, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// HTTPHealth handles GET /health
func (h *RecommendationHandler) HTTPHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "recommendation-service",
		"timestamp": time.Now().UTC(),
	})
}

// =============================================================================
// Helpers
// =============================================================================

// grpcStatusToHTTP converts a gRPC status error to an HTTP status code and
// message suitable for a REST response.
func grpcStatusToHTTP(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError, err.Error()
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return http.StatusBadRequest, st.Message()
	case codes.NotFound:
		return http.StatusNotFound, st.Message()
	case codes.AlreadyExists:
		return http.StatusConflict, st.Message()
	case codes.PermissionDenied:
		return http.StatusForbidden, st.Message()
	case codes.Unauthenticated:
		return http.StatusUnauthorized, st.Message()
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, st.Message()
	case codes.Unavailable:
		return http.StatusServiceUnavailable, st.Message()
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, st.Message()
	default:
		return http.StatusInternalServerError, st.Message()
	}
}
