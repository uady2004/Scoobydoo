package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/analytics-service/internal/services"
)

const (
	defaultDateLayout = "2006-01-02"
)

// AnalyticsHandler exposes REST endpoints for analytics queries.
type AnalyticsHandler struct {
	svc    *services.AnalyticsService
	logger *zap.Logger
}

// NewAnalyticsHandler creates a new AnalyticsHandler.
func NewAnalyticsHandler(svc *services.AnalyticsService, logger *zap.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc, logger: logger}
}

// RegisterRoutes attaches all analytics routes to a Gin RouterGroup.
func (h *AnalyticsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/videos/:video_id", h.GetVideoAnalytics)
	rg.GET("/creators/:creator_id", h.GetCreatorAnalytics)
	rg.GET("/platform", h.GetPlatformMetrics)
	rg.GET("/live/:live_id", h.GetLiveAnalytics)
	rg.GET("/ads/:campaign_id/:ad_id", h.GetAdAnalytics)

	// Gateway-style aliases: Flutter calls /analytics/videos/:id/stats and /analytics/profile/stats.
	// The gateway forwards these full paths, so the service must handle them too.
	rg.GET("/analytics/videos/:video_id/stats", h.GetVideoAnalytics)
	rg.GET("/analytics/profile/stats", h.GetMyCreatorAnalytics)
	rg.GET("/analytics/creators/:creator_id/stats", h.GetCreatorAnalytics)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseDateRange extracts start/end query params (YYYY-MM-DD).
// If absent, it defaults to the last 30 days.
func parseDateRange(c *gin.Context) (start, end time.Time, err error) {
	now := time.Now().UTC()

	startStr := c.DefaultQuery("start", now.AddDate(0, 0, -30).Format(defaultDateLayout))
	endStr := c.DefaultQuery("end", now.Format(defaultDateLayout))

	start, err = time.Parse(defaultDateLayout, startStr)
	if err != nil {
		return
	}
	end, err = time.Parse(defaultDateLayout, endStr)
	if err != nil {
		return
	}
	// end is inclusive — shift to start of next day for half-open interval.
	end = end.AddDate(0, 0, 1)
	return
}

func respondError(c *gin.Context, code int, msg string, err error) {
	body := gin.H{"error": msg}
	if err != nil {
		body["detail"] = err.Error()
	}
	c.JSON(code, body)
}

// GetMyCreatorAnalytics serves GET /analytics/profile/stats — uses the
// authenticated user's ID (set by the JWT middleware) as the creator ID.
func (h *AnalyticsHandler) GetMyCreatorAnalytics(c *gin.Context) {
	raw, exists := c.Get("user_id")
	if !exists {
		respondError(c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	creatorID, _ := raw.(string)

	start, end, err := parseDateRange(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid date range", err)
		return
	}

	result, err := h.svc.GetCreatorAnalytics(c.Request.Context(), creatorID, start, end)
	if err != nil {
		h.logger.Error("GetMyCreatorAnalytics", zap.String("creator_id", creatorID), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "failed to fetch creator analytics", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// GetVideoAnalytics godoc
// GET /analytics/videos/:video_id?start=YYYY-MM-DD&end=YYYY-MM-DD
func (h *AnalyticsHandler) GetVideoAnalytics(c *gin.Context) {
	videoID := c.Param("video_id")
	if videoID == "" {
		respondError(c, http.StatusBadRequest, "video_id is required", nil)
		return
	}

	start, end, err := parseDateRange(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid date range", err)
		return
	}

	result, err := h.svc.GetVideoAnalytics(c.Request.Context(), videoID, start, end)
	if err != nil {
		h.logger.Error("GetVideoAnalytics", zap.String("video_id", videoID), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "failed to fetch video analytics", err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetCreatorAnalytics godoc
// GET /analytics/creators/:creator_id?start=YYYY-MM-DD&end=YYYY-MM-DD
func (h *AnalyticsHandler) GetCreatorAnalytics(c *gin.Context) {
	creatorID := c.Param("creator_id")
	if creatorID == "" {
		respondError(c, http.StatusBadRequest, "creator_id is required", nil)
		return
	}

	start, end, err := parseDateRange(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid date range", err)
		return
	}

	result, err := h.svc.GetCreatorAnalytics(c.Request.Context(), creatorID, start, end)
	if err != nil {
		h.logger.Error("GetCreatorAnalytics", zap.String("creator_id", creatorID), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "failed to fetch creator analytics", err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetPlatformMetrics godoc
// GET /analytics/platform?date=YYYY-MM-DD
func (h *AnalyticsHandler) GetPlatformMetrics(c *gin.Context) {
	dateStr := c.DefaultQuery("date", time.Now().UTC().Format(defaultDateLayout))
	date, err := time.Parse(defaultDateLayout, dateStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid date parameter", err)
		return
	}

	result, err := h.svc.GetPlatformMetrics(c.Request.Context(), date)
	if err != nil {
		h.logger.Error("GetPlatformMetrics", zap.String("date", dateStr), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "failed to fetch platform metrics", err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetLiveAnalytics godoc
// GET /analytics/live/:live_id
func (h *AnalyticsHandler) GetLiveAnalytics(c *gin.Context) {
	liveID := c.Param("live_id")
	if liveID == "" {
		respondError(c, http.StatusBadRequest, "live_id is required", nil)
		return
	}

	result, err := h.svc.GetLiveAnalytics(c.Request.Context(), liveID)
	if err != nil {
		h.logger.Error("GetLiveAnalytics", zap.String("live_id", liveID), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "failed to fetch live analytics", err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetAdAnalytics godoc
// GET /analytics/ads/:campaign_id/:ad_id?start=YYYY-MM-DD&end=YYYY-MM-DD
func (h *AnalyticsHandler) GetAdAnalytics(c *gin.Context) {
	campaignID := c.Param("campaign_id")
	adID := c.Param("ad_id")
	if campaignID == "" || adID == "" {
		respondError(c, http.StatusBadRequest, "campaign_id and ad_id are required", nil)
		return
	}

	start, end, err := parseDateRange(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid date range", err)
		return
	}

	result, err := h.svc.GetAdAnalytics(c.Request.Context(), campaignID, adID, start, end)
	if err != nil {
		h.logger.Error("GetAdAnalytics",
			zap.String("campaign_id", campaignID),
			zap.String("ad_id", adID),
			zap.Error(err))
		respondError(c, http.StatusInternalServerError, "failed to fetch ad analytics", err)
		return
	}

	c.JSON(http.StatusOK, result)
}
