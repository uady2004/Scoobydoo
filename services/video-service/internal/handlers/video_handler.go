package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/video-service/internal/models"
	"github.com/tiktok-clone/video-service/internal/services"
)

// VideoHandler handles CRUD and lifecycle management HTTP requests for videos.
type VideoHandler struct {
	videoSvc *services.VideoService
	logger   *zap.Logger
}

// NewVideoHandler constructs a VideoHandler.
func NewVideoHandler(videoSvc *services.VideoService, logger *zap.Logger) *VideoHandler {
	return &VideoHandler{videoSvc: videoSvc, logger: logger}
}

// RegisterRoutes attaches video routes to the given router group.
//
//	GET    /videos/trending            — trending videos
//	GET    /videos/:videoID            — get single video
//	PATCH  /videos/:videoID            — update video metadata
//	DELETE /videos/:videoID            — soft-delete video
//	POST   /videos/:videoID/publish    — publish a video
//	POST   /videos/:videoID/draft      — save as draft
//	POST   /videos/:videoID/schedule   — schedule future publication
//	GET    /users/:userID/videos       — list user's videos
//	GET    /users/:userID/drafts       — list user's drafts
func (h *VideoHandler) RegisterRoutes(rg *gin.RouterGroup) {
	videos := rg.Group("/videos")
	{
		videos.GET("/trending", h.GetTrending)
		videos.GET("/:videoID", h.GetVideo)
		videos.PATCH("/:videoID", h.UpdateVideo)
		videos.DELETE("/:videoID", h.DeleteVideo)
		videos.POST("/:videoID/publish", h.PublishVideo)
		videos.POST("/:videoID/draft", h.SaveDraft)
		videos.POST("/:videoID/schedule", h.ScheduleVideo)
	}

	users := rg.Group("/users")
	{
		users.GET("/:userID/videos", h.GetVideosByUser)
		users.GET("/:userID/drafts", h.GetDrafts)
	}
}

// GetVideo godoc
// @Summary  Get a video by ID
// @Tags     videos
// @Produce  json
// @Param    videoID  path  string  true  "Video ID"
// @Success  200  {object}  models.Video
// @Failure  404  {object}  ErrorResponse
// @Router   /videos/{videoID} [get]
func (h *VideoHandler) GetVideo(c *gin.Context) {
	videoID := c.Param("videoID")
	if videoID == "" {
		respondError(c, http.StatusBadRequest, "videoID is required")
		return
	}

	video, err := h.videoSvc.GetVideo(c.Request.Context(), videoID)
	if err != nil {
		if errors.Is(err, services.ErrVideoNotFound) {
			respondError(c, http.StatusNotFound, "video not found")
		} else {
			h.logger.Error("GetVideo failed", zap.String("video_id", videoID), zap.Error(err))
			respondError(c, http.StatusInternalServerError, "internal error")
		}
		return
	}

	c.JSON(http.StatusOK, video)
}

// UpdateVideo godoc
// @Summary  Update video metadata
// @Tags     videos
// @Accept   json
// @Produce  json
// @Param    videoID  path   string                    true  "Video ID"
// @Param    body     body   models.UpdateVideoRequest true  "Fields to update"
// @Success  200  {object}  models.Video
// @Failure  400  {object}  ErrorResponse
// @Failure  403  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /videos/{videoID} [patch]
func (h *VideoHandler) UpdateVideo(c *gin.Context) {
	videoID := c.Param("videoID")
	userID := extractUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "missing user identity")
		return
	}

	var req models.UpdateVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	video, err := h.videoSvc.UpdateVideo(c.Request.Context(), videoID, userID, &req)
	if err != nil {
		h.handleVideoError(c, videoID, err)
		return
	}

	c.JSON(http.StatusOK, video)
}

// DeleteVideo godoc
// @Summary  Soft-delete a video
// @Tags     videos
// @Produce  json
// @Param    videoID  path  string  true  "Video ID"
// @Success  204
// @Failure  403  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /videos/{videoID} [delete]
func (h *VideoHandler) DeleteVideo(c *gin.Context) {
	videoID := c.Param("videoID")
	userID := extractUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "missing user identity")
		return
	}

	if err := h.videoSvc.DeleteVideo(c.Request.Context(), videoID, userID); err != nil {
		h.handleVideoError(c, videoID, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// PublishVideo godoc
// @Summary  Publish a video (make it publicly visible)
// @Tags     videos
// @Produce  json
// @Param    videoID  path  string  true  "Video ID"
// @Success  200  {object}  models.Video
// @Failure  403  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /videos/{videoID}/publish [post]
func (h *VideoHandler) PublishVideo(c *gin.Context) {
	videoID := c.Param("videoID")
	userID := extractUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "missing user identity")
		return
	}

	video, err := h.videoSvc.PublishVideo(c.Request.Context(), videoID, userID)
	if err != nil {
		h.handleVideoError(c, videoID, err)
		return
	}

	c.JSON(http.StatusOK, video)
}

// SaveDraft godoc
// @Summary  Save video as draft (keeps it private)
// @Tags     videos
// @Accept   json
// @Produce  json
// @Param    videoID  path   string                    true  "Video ID"
// @Param    body     body   models.UpdateVideoRequest false "Draft metadata"
// @Success  200  {object}  models.Video
// @Failure  403  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /videos/{videoID}/draft [post]
func (h *VideoHandler) SaveDraft(c *gin.Context) {
	videoID := c.Param("videoID")
	userID := extractUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "missing user identity")
		return
	}

	var req models.UpdateVideoRequest
	// Body is optional for save-draft.
	_ = c.ShouldBindJSON(&req)

	video, err := h.videoSvc.SaveDraft(c.Request.Context(), videoID, userID, &req)
	if err != nil {
		h.handleVideoError(c, videoID, err)
		return
	}

	c.JSON(http.StatusOK, video)
}

// ScheduleVideo godoc
// @Summary  Schedule a video to publish at a future time
// @Tags     videos
// @Accept   json
// @Produce  json
// @Param    videoID  path   string              true  "Video ID"
// @Param    body     body   ScheduleRequest     true  "Schedule parameters"
// @Success  200  {object}  models.Video
// @Failure  400  {object}  ErrorResponse
// @Failure  403  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /videos/{videoID}/schedule [post]
func (h *VideoHandler) ScheduleVideo(c *gin.Context) {
	videoID := c.Param("videoID")
	userID := extractUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "missing user identity")
		return
	}

	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	video, err := h.videoSvc.ScheduleVideo(c.Request.Context(), videoID, userID, req.PublishAt)
	if err != nil {
		h.handleVideoError(c, videoID, err)
		return
	}

	c.JSON(http.StatusOK, video)
}

// GetTrending godoc
// @Summary  Get trending videos
// @Tags     videos
// @Produce  json
// @Param    limit  query  int  false  "Max results (default 20, max 100)"
// @Success  200  {array}  models.Video
// @Router   /videos/trending [get]
func (h *VideoHandler) GetTrending(c *gin.Context) {
	limit := queryInt(c, "limit", 20)
	if limit > 100 {
		limit = 100
	}

	videos, err := h.videoSvc.GetTrending(c.Request.Context(), limit)
	if err != nil {
		h.logger.Error("GetTrending failed", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"count":  len(videos),
	})
}

// GetVideosByUser godoc
// @Summary  List videos uploaded by a user
// @Tags     users, videos
// @Produce  json
// @Param    userID  path   string  true   "User ID"
// @Param    limit   query  int     false  "Page size (default 20)"
// @Param    offset  query  int     false  "Page offset (default 0)"
// @Success  200  {array}  models.Video
// @Router   /users/{userID}/videos [get]
func (h *VideoHandler) GetVideosByUser(c *gin.Context) {
	targetUserID := c.Param("userID")
	requestingUserID := extractUserID(c)
	limit := queryInt(c, "limit", 20)
	offset := queryInt(c, "offset", 0)

	videos, err := h.videoSvc.GetVideosByUser(c.Request.Context(), targetUserID, requestingUserID, limit, offset)
	if err != nil {
		h.logger.Error("GetVideosByUser failed",
			zap.String("target_user", targetUserID),
			zap.Error(err),
		)
		respondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"count":  len(videos),
		"limit":  limit,
		"offset": offset,
	})
}

// GetDrafts godoc
// @Summary  List draft videos for a user
// @Tags     users, videos
// @Produce  json
// @Param    userID  path   string  true   "User ID (must match authenticated user)"
// @Param    limit   query  int     false  "Page size (default 20)"
// @Param    offset  query  int     false  "Page offset (default 0)"
// @Success  200  {array}  models.Video
// @Failure  403  {object}  ErrorResponse
// @Router   /users/{userID}/drafts [get]
func (h *VideoHandler) GetDrafts(c *gin.Context) {
	targetUserID := c.Param("userID")
	requestingUserID := extractUserID(c)
	if requestingUserID == "" {
		respondError(c, http.StatusUnauthorized, "missing user identity")
		return
	}
	if targetUserID != requestingUserID {
		respondError(c, http.StatusForbidden, "you can only view your own drafts")
		return
	}

	limit := queryInt(c, "limit", 20)
	offset := queryInt(c, "offset", 0)

	videos, err := h.videoSvc.GetDrafts(c.Request.Context(), targetUserID, limit, offset)
	if err != nil {
		h.logger.Error("GetDrafts failed", zap.String("user_id", targetUserID), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"count":  len(videos),
	})
}

// ---- request / response types -----------------------------------------------

// ScheduleRequest is the body for the schedule endpoint.
type ScheduleRequest struct {
	PublishAt time.Time `json:"publish_at" binding:"required"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

// ---- shared handler helpers -------------------------------------------------

func (h *VideoHandler) handleVideoError(c *gin.Context, videoID string, err error) {
	switch {
	case errors.Is(err, services.ErrVideoNotFound):
		respondError(c, http.StatusNotFound, "video not found")
	case errors.Is(err, services.ErrForbidden):
		respondError(c, http.StatusForbidden, err.Error())
	default:
		h.logger.Error("video operation failed",
			zap.String("video_id", videoID),
			zap.Error(err),
		)
		respondError(c, http.StatusInternalServerError, "internal error")
	}
}

// respondError writes a JSON error envelope and sets the HTTP status.
func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, ErrorResponse{Error: msg, Code: status})
}

// extractUserID reads the authenticated user ID from the request context.
// In production this would be populated by a JWT / auth middleware.
func extractUserID(c *gin.Context) string {
	if id, exists := c.Get("user_id"); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	// Fall back to header for service-to-service calls.
	return c.GetHeader("X-User-ID")
}

// queryInt reads an integer query parameter with a fallback default.
func queryInt(c *gin.Context, key string, defaultVal int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}
