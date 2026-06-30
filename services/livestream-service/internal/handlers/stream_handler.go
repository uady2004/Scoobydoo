package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/livestream-service/internal/models"
	"github.com/tiktok-clone/livestream-service/internal/services"
)

// StreamHandler handles REST endpoints for livestream lifecycle management.
type StreamHandler struct {
	streamSvc services.StreamService
	hub       *Hub
	logger    *zap.Logger
}

// NewStreamHandler creates a StreamHandler.
func NewStreamHandler(svc services.StreamService, hub *Hub, logger *zap.Logger) *StreamHandler {
	return &StreamHandler{streamSvc: svc, hub: hub, logger: logger}
}

// RegisterRoutes mounts all stream routes on the provided RouterGroup.
func (h *StreamHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("", h.StartStream)
	rg.GET("", h.ListActiveStreams)
	rg.GET("/:id", h.GetStream)
	rg.DELETE("/:id", h.EndStream)
	rg.POST("/:id/join", h.JoinStream)
	rg.DELETE("/:id/leave", h.LeaveStream)
	rg.GET("/:id/viewers", h.ListViewers)
	rg.PUT("/:id/hls", h.UpdateHLSURL)

	// Internal RTMP callback — called by the RTMP server on stream start.
	rg.POST("/rtmp/validate", h.ValidateRTMPKey)
}

// StartStream godoc
// POST /api/v1/streams
func (h *StreamHandler) StartStream(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Title         string   `json:"title" binding:"required,max=100"`
		Description   string   `json:"description" binding:"max=500"`
		CategoryID    string   `json:"category_id"`
		Tags          []string `json:"tags"`
		Language      string   `json:"language"`
		AgeRestricted bool     `json:"age_restricted"`
		AllowComments bool     `json:"allow_comments"`
		IsRecorded    bool     `json:"is_recorded"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stream, err := h.streamSvc.StartStream(c.Request.Context(), services.StartStreamRequest{
		UserID:        userID,
		Title:         req.Title,
		Description:   req.Description,
		CategoryID:    req.CategoryID,
		Tags:          req.Tags,
		Language:      req.Language,
		AgeRestricted: req.AgeRestricted,
		AllowComments: req.AllowComments,
		IsRecorded:    req.IsRecorded,
	})
	if err != nil {
		h.logger.Error("StartStream failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start stream"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"stream": stream})
}

// EndStream godoc
// DELETE /api/v1/streams/:id
func (h *StreamHandler) EndStream(c *gin.Context) {
	userID := c.GetString("user_id")
	streamID := c.Param("id")

	if err := h.streamSvc.EndStream(c.Request.Context(), streamID, userID); err != nil {
		switch {
		case errors.Is(err, services.ErrStreamNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "stream not found"})
		case errors.Is(err, services.ErrStreamNotOwned):
			c.JSON(http.StatusForbidden, gin.H{"error": "not the stream owner"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to end stream"})
		}
		return
	}

	// Push stream.end event to all connected WebSocket clients.
	h.hub.EndStream(streamID)

	c.JSON(http.StatusOK, gin.H{"message": "stream ended"})
}

// GetStream godoc
// GET /api/v1/streams/:id
func (h *StreamHandler) GetStream(c *gin.Context) {
	stream, err := h.streamSvc.GetStream(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stream not found"})
		return
	}

	// Augment with real-time WebSocket viewer count.
	stream.ViewerCount = int64(h.hub.ViewerCount(stream.ID))
	c.JSON(http.StatusOK, gin.H{"stream": stream})
}

// ListActiveStreams godoc
// GET /api/v1/streams?limit=20&offset=0
func (h *StreamHandler) ListActiveStreams(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 50 {
		limit = 50
	}

	streams, err := h.streamSvc.GetActiveStreams(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch streams"})
		return
	}

	// Augment viewer counts from WebSocket hub.
	for _, s := range streams {
		if wsc := h.hub.ViewerCount(s.ID); wsc > 0 {
			s.ViewerCount = int64(wsc)
		}
	}

	c.JSON(http.StatusOK, gin.H{"streams": streams, "total": len(streams)})
}

// JoinStream godoc
// POST /api/v1/streams/:id/join
func (h *StreamHandler) JoinStream(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	}
	_ = c.ShouldBindJSON(&req)

	viewer, err := h.streamSvc.JoinStream(c.Request.Context(), services.JoinStreamRequest{
		StreamID:  c.Param("id"),
		UserID:    userID,
		Username:  req.Username,
		AvatarURL: req.AvatarURL,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrStreamNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "stream not found"})
		case errors.Is(err, services.ErrStreamNotLive):
			c.JSON(http.StatusConflict, gin.H{"error": "stream is not live"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join stream"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"viewer": viewer})
}

// LeaveStream godoc
// DELETE /api/v1/streams/:id/leave
func (h *StreamHandler) LeaveStream(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.streamSvc.LeaveStream(c.Request.Context(), c.Param("id"), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to leave stream"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "left stream"})
}

// ListViewers godoc
// GET /api/v1/streams/:id/viewers?limit=50
func (h *StreamHandler) ListViewers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	viewers, err := h.streamSvc.GetActiveViewers(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch viewers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"viewers": viewers, "total": len(viewers)})
}

// UpdateHLSURL is called by the transcoding pipeline once HLS is ready.
// PUT /api/v1/streams/:id/hls
func (h *StreamHandler) UpdateHLSURL(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.streamSvc.UpdateHLSPlaylistURL(c.Request.Context(), c.Param("id"), req.URL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update HLS URL"})
		return
	}

	// Notify all connected viewers that the stream is live.
	stream, _ := h.streamSvc.GetStream(c.Request.Context(), c.Param("id"))
	if stream != nil {
		h.hub.BroadcastToRoom(stream.ID, models.WSEvent{
			Type:      models.WSEventStreamStart,
			StreamID:  stream.ID,
			Payload:   map[string]string{"hls_url": req.URL},
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "HLS URL updated"})
}

// ValidateRTMPKey is called by the RTMP server to validate an incoming stream key.
// POST /api/v1/streams/rtmp/validate
func (h *StreamHandler) ValidateRTMPKey(c *gin.Context) {
	var req struct {
		RTMPKey string `json:"rtmp_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stream, err := h.streamSvc.ValidateRTMPKey(c.Request.Context(), req.RTMPKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid stream key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stream_id": stream.ID,
		"user_id":   stream.UserID,
		"title":     stream.Title,
	})
}
