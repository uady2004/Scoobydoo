package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/like-service/internal/models"
	"github.com/tiktok-clone/like-service/internal/repositories"
	"github.com/tiktok-clone/like-service/internal/services"
)

type LikeHandler struct {
	svc    services.LikeService
	logger *zap.Logger
}

func NewLikeHandler(svc services.LikeService, logger *zap.Logger) *LikeHandler {
	return &LikeHandler{svc: svc, logger: logger}
}

func (h *LikeHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/likes/videos/:videoId", h.LikeVideo)
	rg.DELETE("/likes/videos/:videoId", h.UnlikeVideo)
	rg.GET("/likes/videos/:videoId", h.GetLikeCount)
	rg.GET("/likes/videos/:videoId/status", h.IsLiked)
	rg.POST("/likes/videos/batch-status", h.BatchStatus)
	rg.GET("/likes/users/:userId/videos", h.UserLikedVideos)
	rg.GET("/likes/trending", h.TopLikedVideos)
}

func (h *LikeHandler) LikeVideo(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	err := h.svc.LikeVideo(c.Request.Context(), userID, c.Param("videoId"))
	if err != nil {
		if errors.Is(err, repositories.ErrAlreadyLiked) {
			c.JSON(http.StatusConflict, gin.H{"error": "already liked"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to like video"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": true})
}

func (h *LikeHandler) UnlikeVideo(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	err := h.svc.UnlikeVideo(c.Request.Context(), userID, c.Param("videoId"))
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "like not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unlike video"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": false})
}

func (h *LikeHandler) GetLikeCount(c *gin.Context) {
	count, err := h.svc.GetLikeCount(c.Request.Context(), c.Param("videoId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get like count"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"video_id": c.Param("videoId"), "count": count})
}

func (h *LikeHandler) IsLiked(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	liked, err := h.svc.IsLiked(c.Request.Context(), userID, c.Param("videoId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check like status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": liked})
}

func (h *LikeHandler) BatchStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.BatchLikeStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	statuses, err := h.svc.BatchCheckLikes(c.Request.Context(), userID, req.VideoIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to batch check likes"})
		return
	}
	c.JSON(http.StatusOK, models.BatchLikeStatusResp{Statuses: statuses})
}

func (h *LikeHandler) UserLikedVideos(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	likes, err := h.svc.GetUserLikedVideos(c.Request.Context(), c.Param("userId"), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get liked videos"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"likes": likes, "total": len(likes)})
}

func (h *LikeHandler) TopLikedVideos(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	videos, err := h.svc.GetTopLikedVideos(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get trending videos"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"videos": videos})
}
