package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/social-graph-service/internal/models"
	"github.com/tiktok-clone/social-graph-service/internal/repositories"
	"github.com/tiktok-clone/social-graph-service/internal/services"
)

// contextUserIDKey is the gin context key under which the authenticated user's
// ID is stored by the upstream JWT middleware.
const contextUserIDKey = "user_id"

// ---------------------------------------------------------------------------
// SocialHandler
// ---------------------------------------------------------------------------

// SocialHandler exposes the social-graph REST API via Gin.
// All mutation endpoints expect the caller to be authenticated; the JWT
// middleware must set "user_id" in the Gin context before reaching these
// handlers.
type SocialHandler struct {
	svc    *services.SocialService
	logger *zap.Logger
}

// NewSocialHandler creates a new SocialHandler.
func NewSocialHandler(svc *services.SocialService, logger *zap.Logger) *SocialHandler {
	return &SocialHandler{svc: svc, logger: logger}
}

// RegisterRoutes mounts all social-graph routes onto the given RouterGroup.
//
// Routes:
//
//	POST   /follow          — follow a user
//	DELETE /follow          — unfollow a user
//	GET    /followers       — list a user's followers (query param: user_id)
//	GET    /following       — list the users a user follows (query param: user_id)
//	GET    /mutual          — list mutual followers between viewer and target
//	GET    /relationship    — check the relationship between viewer and target
//	GET    /suggestions     — BFS-based friend suggestions for the authenticated user
//	POST   /block           — block a user
//	GET    /blocks          — list the authenticated user's blocked users
func (h *SocialHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/follow", h.Follow)
	rg.DELETE("/follow", h.Unfollow)
	rg.GET("/followers", h.GetFollowers)
	rg.GET("/following", h.GetFollowing)
	rg.GET("/mutual", h.GetMutualFollowers)
	rg.GET("/relationship", h.CheckRelationship)
	rg.GET("/suggestions", h.GetSuggestions)
	rg.POST("/block", h.BlockUser)
	rg.GET("/blocks", h.GetBlockList)

	// Gateway-style path aliases — target user id in the URL path segment.
	rg.POST("/users/:userId/follow", h.FollowUser)
	rg.DELETE("/users/:userId/follow", h.UnfollowUser)
	rg.GET("/users/:userId/followers", h.GetUserFollowers)
	rg.GET("/users/:userId/following", h.GetUserFollowing)
}

// ---------------------------------------------------------------------------
// Follow / Unfollow
// ---------------------------------------------------------------------------

// Follow creates a follow edge from the authenticated user to the target user.
//
//	POST /api/v1/follow
//	Body: { "followee_id": "<uuid>" }
//
// Responses:
//
//	201 Created   — { "data": Follow }
//	400 Bad Request — invalid body
//	409 Conflict  — already following
//	403 Forbidden — user blocked
//	400 Bad Request — self-follow
func (h *SocialHandler) Follow(c *gin.Context) {
	viewerID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	var req models.FollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, "invalid request body: "+err.Error())
		return
	}
	if req.FolloweeID == "" {
		h.badRequest(c, "followee_id is required")
		return
	}

	follow, err := h.svc.Follow(c.Request.Context(), viewerID, req.FolloweeID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": follow})
}

// Unfollow removes the follow edge from the authenticated user to the target user.
//
//	DELETE /api/v1/follow
//	Body: { "followee_id": "<uuid>" }
//
// Responses:
//
//	200 OK        — { "message": "unfollowed" }
//	400 Bad Request — invalid body
//	404 Not Found — not following
func (h *SocialHandler) Unfollow(c *gin.Context) {
	viewerID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	var req models.FollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, "invalid request body: "+err.Error())
		return
	}
	if req.FolloweeID == "" {
		h.badRequest(c, "followee_id is required")
		return
	}

	if err := h.svc.Unfollow(c.Request.Context(), viewerID, req.FolloweeID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "unfollowed"})
}

// ---------------------------------------------------------------------------
// List endpoints
// ---------------------------------------------------------------------------

// GetFollowers returns a paginated list of followers for the given user.
//
//	GET /api/v1/followers?user_id=<uuid>&limit=20&offset=0
//
// If user_id is omitted the authenticated user's followers are returned.
//
// Responses:
//
//	200 OK — FollowListResponse
func (h *SocialHandler) GetFollowers(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		var ok bool
		userID, ok = h.extractUserID(c)
		if !ok {
			return
		}
	}

	limit, offset := h.parsePagination(c)
	resp, err := h.svc.GetFollowers(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetFollowing returns a paginated list of users that the given user follows.
//
//	GET /api/v1/following?user_id=<uuid>&limit=20&offset=0
//
// If user_id is omitted the authenticated user's following list is returned.
//
// Responses:
//
//	200 OK — FollowListResponse
func (h *SocialHandler) GetFollowing(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		var ok bool
		userID, ok = h.extractUserID(c)
		if !ok {
			return
		}
	}

	limit, offset := h.parsePagination(c)
	resp, err := h.svc.GetFollowing(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetMutualFollowers returns a paginated list of users who follow both the
// authenticated viewer and the specified target.
//
//	GET /api/v1/mutual?target_id=<uuid>&limit=20&offset=0
//
// Responses:
//
//	200 OK  — FollowListResponse
//	400     — target_id missing
func (h *SocialHandler) GetMutualFollowers(c *gin.Context) {
	viewerID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	targetID := c.Query("target_id")
	if targetID == "" {
		h.badRequest(c, "target_id query parameter is required")
		return
	}

	limit, offset := h.parsePagination(c)
	resp, err := h.svc.GetMutualFollowers(c.Request.Context(), viewerID, targetID, limit, offset)
	if err != nil {
		h.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CheckRelationship returns the full relationship view between the
// authenticated viewer and a target user.
//
//	GET /api/v1/relationship?target_id=<uuid>
//
// Responses:
//
//	200 OK  — Relationship
//	400     — target_id missing
func (h *SocialHandler) CheckRelationship(c *gin.Context) {
	viewerID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	targetID := c.Query("target_id")
	if targetID == "" {
		h.badRequest(c, "target_id query parameter is required")
		return
	}

	rel, err := h.svc.CheckRelationship(c.Request.Context(), viewerID, targetID)
	if err != nil {
		h.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, rel)
}

// ---------------------------------------------------------------------------
// Suggestions
// ---------------------------------------------------------------------------

// GetSuggestions returns BFS-based friend suggestions for the authenticated user.
//
//	GET /api/v1/suggestions
//
// Responses:
//
//	200 OK — SuggestionListResponse
func (h *SocialHandler) GetSuggestions(c *gin.Context) {
	viewerID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	resp, err := h.svc.GetFriendSuggestions(c.Request.Context(), viewerID)
	if err != nil {
		h.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Block
// ---------------------------------------------------------------------------

// blockRequest is the JSON body for POST /block.
type blockRequest struct {
	BlockedID string `json:"blocked_id" binding:"required"`
}

// BlockUser blocks the specified user on behalf of the authenticated user.
//
//	POST /api/v1/block
//	Body: { "blocked_id": "<uuid>" }
//
// Responses:
//
//	200 OK        — { "message": "blocked" }
//	400 Bad Request — invalid body
func (h *SocialHandler) BlockUser(c *gin.Context) {
	blockerID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	var req blockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, "invalid request body: "+err.Error())
		return
	}
	if req.BlockedID == "" {
		h.badRequest(c, "blocked_id is required")
		return
	}

	if err := h.svc.BlockUser(c.Request.Context(), blockerID, req.BlockedID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "blocked"})
}

// GetBlockList returns a paginated list of users the authenticated user has
// blocked.
//
//	GET /api/v1/blocks?limit=20&offset=0
//
// Responses:
//
//	200 OK — { "blocks": []Block, "pagination": PaginationMeta }
func (h *SocialHandler) GetBlockList(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	limit, offset := h.parsePagination(c)
	blocks, total, err := h.svc.GetBlockList(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"blocks": blocks,
		"pagination": models.PaginationMeta{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: int64(offset+limit) < total,
		},
	})
}

// ---------------------------------------------------------------------------
// Path-based aliases (userId in URL segment instead of query/body)
// ---------------------------------------------------------------------------

func (h *SocialHandler) FollowUser(c *gin.Context) {
	viewerID, ok := h.extractUserID(c)
	if !ok {
		return
	}
	follow, err := h.svc.Follow(c.Request.Context(), viewerID, c.Param("userId"))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": follow})
}

func (h *SocialHandler) UnfollowUser(c *gin.Context) {
	viewerID, ok := h.extractUserID(c)
	if !ok {
		return
	}
	if err := h.svc.Unfollow(c.Request.Context(), viewerID, c.Param("userId")); err != nil {
		h.handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unfollowed"})
}

func (h *SocialHandler) GetUserFollowers(c *gin.Context) {
	limit, offset := h.parsePagination(c)
	resp, err := h.svc.GetFollowers(c.Request.Context(), c.Param("userId"), limit, offset)
	if err != nil {
		h.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *SocialHandler) GetUserFollowing(c *gin.Context) {
	limit, offset := h.parsePagination(c)
	resp, err := h.svc.GetFollowing(c.Request.Context(), c.Param("userId"), limit, offset)
	if err != nil {
		h.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Error envelope
// ---------------------------------------------------------------------------

// errorResponse is the standard JSON error envelope returned by all handlers.
type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractUserID retrieves the authenticated user's ID from the Gin context.
// It writes a 401 response and returns false if the ID is missing or invalid.
func (h *SocialHandler) extractUserID(c *gin.Context) (string, bool) {
	id, exists := c.Get(contextUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return "", false
	}
	userID, ok := id.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, errorResponse{
			Error: "unauthorized",
			Code:  "UNAUTHORIZED",
		})
		return "", false
	}
	return userID, true
}

// parsePagination reads limit and offset from query parameters with defaults.
func (h *SocialHandler) parsePagination(c *gin.Context) (limit, offset int) {
	limit = 20
	offset = 0
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}
	return
}

func (h *SocialHandler) badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, errorResponse{Error: msg, Code: "BAD_REQUEST"})
}

func (h *SocialHandler) internalError(c *gin.Context, err error) {
	h.logger.Error("internal server error", zap.Error(err))
	c.JSON(http.StatusInternalServerError, errorResponse{
		Error: "internal server error",
		Code:  "INTERNAL_ERROR",
	})
}

// handleServiceError maps known service/repository sentinel errors to
// appropriate HTTP status codes.
func (h *SocialHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repositories.ErrAlreadyFollowing):
		c.JSON(http.StatusConflict, errorResponse{
			Error: "already following this user",
			Code:  "ALREADY_FOLLOWING",
		})
	case errors.Is(err, repositories.ErrNotFollowing):
		c.JSON(http.StatusNotFound, errorResponse{
			Error: "not following this user",
			Code:  "NOT_FOLLOWING",
		})
	case errors.Is(err, repositories.ErrSelfFollow):
		c.JSON(http.StatusBadRequest, errorResponse{
			Error: "cannot follow yourself",
			Code:  "SELF_FOLLOW",
		})
	case errors.Is(err, repositories.ErrUserBlocked):
		c.JSON(http.StatusForbidden, errorResponse{
			Error: "action not permitted: user is blocked",
			Code:  "USER_BLOCKED",
		})
	default:
		h.internalError(c, err)
	}
}
