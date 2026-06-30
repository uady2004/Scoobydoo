package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/interaction-service/internal/repositories"
	"github.com/tiktok-clone/interaction-service/internal/services"
)

// InteractionHandler handles all HTTP interaction endpoints.
type InteractionHandler struct {
	svc    services.InteractionService
	logger *zap.Logger
}

// NewInteractionHandler creates an InteractionHandler.
func NewInteractionHandler(svc services.InteractionService, logger *zap.Logger) *InteractionHandler {
	return &InteractionHandler{svc: svc, logger: logger}
}

// RegisterRoutes mounts all routes.
func (h *InteractionHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// Likes
	rg.POST("/likes/videos/:videoId", h.LikeVideo)
	rg.DELETE("/likes/videos/:videoId", h.UnlikeVideo)
	rg.GET("/likes/videos/:videoId/status", h.IsLiked)
	rg.GET("/likes/videos/:videoId/count", h.GetLikeCount)
	rg.GET("/likes/users/me/videos", h.GetLikedVideos)

	// Comments
	rg.POST("/comments", h.CreateComment)
	rg.GET("/comments/:commentId/replies", h.ListReplies)
	rg.DELETE("/comments/:commentId", h.DeleteComment)
	rg.POST("/comments/:commentId/like", h.LikeComment)
	rg.DELETE("/comments/:commentId/like", h.UnlikeComment)
	rg.POST("/comments/:commentId/pin", h.PinComment)
	rg.POST("/comments/:commentId/report", h.ReportComment)

	// Bookmarks
	rg.POST("/bookmarks/videos/:videoId", h.BookmarkVideo)
	rg.DELETE("/bookmarks/videos/:videoId", h.UnbookmarkVideo)
	rg.GET("/bookmarks", h.ListBookmarks)
	rg.POST("/bookmarks/collections", h.CreateCollection)
	rg.GET("/bookmarks/collections", h.ListCollections)

	// Alias routes matching gateway/Flutter path style
	rg.POST("/videos/:videoId/like", h.LikeVideo)
	rg.DELETE("/videos/:videoId/like", h.UnlikeVideo)
	rg.GET("/videos/:videoId/likes", h.GetLikeCount)
	rg.GET("/videos/:videoId/like-status", h.IsLiked)
	rg.GET("/videos/:videoId/comments", h.ListComments)
	rg.POST("/videos/:videoId/comments", h.CreateCommentByVideoID)
	rg.DELETE("/videos/:videoId/comments/:commentId", h.DeleteComment)
	rg.POST("/videos/:videoId/bookmark", h.BookmarkVideo)
	rg.DELETE("/videos/:videoId/bookmark", h.UnbookmarkVideo)
	rg.GET("/me/liked-videos", h.GetLikedVideos)
	rg.GET("/me/bookmarks", h.ListBookmarks)
}

// ─── Likes ────────────────────────────────────────────────────────────────────

func (h *InteractionHandler) LikeVideo(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.svc.LikeVideo(c.Request.Context(), userID, c.Param("videoId")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to like video"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": true})
}

func (h *InteractionHandler) UnlikeVideo(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.svc.UnlikeVideo(c.Request.Context(), userID, c.Param("videoId")); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "like not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unlike video"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": false})
}

func (h *InteractionHandler) IsLiked(c *gin.Context) {
	userID := c.GetString("user_id")
	liked, err := h.svc.IsLiked(c.Request.Context(), userID, c.Param("videoId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check like status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": liked})
}

func (h *InteractionHandler) GetLikeCount(c *gin.Context) {
	count, err := h.svc.GetVideoLikeCount(c.Request.Context(), c.Param("videoId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get like count"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// ─── Comments ─────────────────────────────────────────────────────────────────

func (h *InteractionHandler) CreateComment(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		VideoID   string `json:"video_id" binding:"required"`
		Content   string `json:"content" binding:"required,min=1,max=1000"`
		ParentID  string `json:"parent_id"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment, err := h.svc.CreateComment(c.Request.Context(), services.CreateCommentReq{
		VideoID:   req.VideoID,
		UserID:    userID,
		Username:  req.Username,
		AvatarURL: req.AvatarURL,
		Content:   req.Content,
		ParentID:  req.ParentID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

func (h *InteractionHandler) CreateCommentByVideoID(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Content   string `json:"content" binding:"required,min=1,max=1000"`
		ParentID  string `json:"parent_id"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment, err := h.svc.CreateComment(c.Request.Context(), services.CreateCommentReq{
		VideoID:   c.Param("videoId"),
		UserID:    userID,
		Username:  req.Username,
		AvatarURL: req.AvatarURL,
		Content:   req.Content,
		ParentID:  req.ParentID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

func (h *InteractionHandler) ListComments(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	comments, err := h.svc.ListComments(c.Request.Context(), c.Param("videoId"), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list comments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"comments": comments, "total": len(comments)})
}

func (h *InteractionHandler) ListReplies(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	replies, err := h.svc.ListReplies(c.Request.Context(), c.Param("commentId"), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list replies"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"replies": replies, "total": len(replies)})
}

func (h *InteractionHandler) DeleteComment(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.svc.DeleteComment(c.Request.Context(), c.Param("commentId"), userID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete comment"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *InteractionHandler) LikeComment(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.svc.LikeComment(c.Request.Context(), userID, c.Param("commentId")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to like comment"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": true})
}

func (h *InteractionHandler) UnlikeComment(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.svc.UnlikeComment(c.Request.Context(), userID, c.Param("commentId")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unlike comment"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": false})
}

// ─── Bookmarks ────────────────────────────────────────────────────────────────

func (h *InteractionHandler) BookmarkVideo(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		CollectionID string `json:"collection_id"`
	}
	_ = c.ShouldBindJSON(&req)

	if err := h.svc.BookmarkVideo(c.Request.Context(), userID, c.Param("videoId"), req.CollectionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bookmark video"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bookmarked": true})
}

func (h *InteractionHandler) UnbookmarkVideo(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.svc.UnbookmarkVideo(c.Request.Context(), userID, c.Param("videoId")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unbookmark video"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bookmarked": false})
}

func (h *InteractionHandler) ListBookmarks(c *gin.Context) {
	userID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	bookmarks, err := h.svc.ListBookmarks(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list bookmarks"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bookmarks": bookmarks, "total": len(bookmarks)})
}

func (h *InteractionHandler) CreateCollection(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Name      string `json:"name" binding:"required,max=50"`
		IsPrivate bool   `json:"is_private"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	col, err := h.svc.CreateCollection(c.Request.Context(), userID, req.Name, req.IsPrivate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create collection"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"collection": col})
}

func (h *InteractionHandler) ListCollections(c *gin.Context) {
	userID := c.GetString("user_id")
	cols, err := h.svc.ListCollections(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list collections"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"collections": cols})
}

// ─── Extended ─────────────────────────────────────────────────────────────────

func (h *InteractionHandler) GetLikedVideos(c *gin.Context) {
	userID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	videoIDs, err := h.svc.GetLikedVideos(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get liked videos"})
		return
	}
	if videoIDs == nil {
		videoIDs = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": videoIDs, "total": len(videoIDs)})
}

func (h *InteractionHandler) PinComment(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.svc.PinComment(c.Request.Context(), c.Param("commentId"), userID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to pin comment"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pinned": true})
}

func (h *InteractionHandler) ReportComment(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ReportContent(c.Request.Context(), "comment", c.Param("commentId"), userID, req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit report"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reported": true})
}
