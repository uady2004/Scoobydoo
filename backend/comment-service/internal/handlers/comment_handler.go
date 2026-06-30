package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/comment-service/internal/models"
	"github.com/tiktok-clone/comment-service/internal/repositories"
	"github.com/tiktok-clone/comment-service/internal/services"
)

type CommentHandler struct {
	svc    services.CommentService
	logger *zap.Logger
}

func NewCommentHandler(svc services.CommentService, logger *zap.Logger) *CommentHandler {
	return &CommentHandler{svc: svc, logger: logger}
}

func (h *CommentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/comments", h.CreateComment)
	rg.GET("/comments", h.ListComments)
	rg.GET("/comments/:id", h.GetComment)
	rg.DELETE("/comments/:id", h.DeleteComment)
	rg.GET("/comments/:id/replies", h.ListReplies)
	rg.POST("/comments/:id/like", h.LikeComment)
	rg.DELETE("/comments/:id/like", h.UnlikeComment)
	rg.GET("/comments/:id/like/status", h.IsCommentLiked)
}

func (h *CommentHandler) CreateComment(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req models.CreateCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = userID

	comment, err := h.svc.CreateComment(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

func (h *CommentHandler) GetComment(c *gin.Context) {
	comment, err := h.svc.GetComment(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get comment"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"comment": comment})
}

func (h *CommentHandler) ListComments(c *gin.Context) {
	videoID := c.Query("video_id")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video_id is required"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	comments, err := h.svc.ListComments(c.Request.Context(), videoID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list comments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"comments": comments, "total": len(comments)})
}

func (h *CommentHandler) ListReplies(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	replies, err := h.svc.ListReplies(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list replies"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"replies": replies, "total": len(replies)})
}

func (h *CommentHandler) DeleteComment(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.svc.DeleteComment(c.Request.Context(), c.Param("id"), userID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete comment"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CommentHandler) LikeComment(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.svc.LikeComment(c.Request.Context(), userID, c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to like comment"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": true})
}

func (h *CommentHandler) UnlikeComment(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.svc.UnlikeComment(c.Request.Context(), userID, c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unlike comment"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": false})
}

func (h *CommentHandler) IsCommentLiked(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	liked, err := h.svc.IsCommentLiked(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check like status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": liked})
}
