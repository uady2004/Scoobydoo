package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/messaging-service/internal/models"
	"github.com/tiktok-clone/messaging-service/internal/repositories"
	"github.com/tiktok-clone/messaging-service/internal/services"
)

// MessageHandler exposes the REST API for the messaging domain.
type MessageHandler struct {
	svc services.MessageService
	log *zap.Logger
}

// NewMessageHandler creates a new MessageHandler.
func NewMessageHandler(svc services.MessageService, log *zap.Logger) *MessageHandler {
	return &MessageHandler{svc: svc, log: log}
}

// ---------------------------------------------------------------------------
// Helper: extract authenticated user ID from Gin context.
// The JWT middleware must have set the "user_id" key before reaching handlers.
// ---------------------------------------------------------------------------

func callerID(c *gin.Context) (uuid.UUID, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing auth context"})
		return uuid.Nil, false
	}
	switch v := raw.(type) {
	case uuid.UUID:
		return v, true
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id in context"})
			return uuid.Nil, false
		}
		return id, true
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id type"})
		return uuid.Nil, false
	}
}

func handleSvcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repositories.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, repositories.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// ---------------------------------------------------------------------------
// Conversations
// ---------------------------------------------------------------------------

// CreateConversation godoc
// POST /conversations
// Body: { "recipient_id": "<uuid>" }
// Response: models.Conversation
func (h *MessageHandler) CreateConversation(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	var req models.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conv, err := h.svc.CreateConversation(c.Request.Context(), userID, req.RecipientID)
	if err != nil {
		h.log.Error("CreateConversation failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.JSON(http.StatusCreated, conv)
}

// GetConversations godoc
// GET /conversations?cursor=<base64>&limit=<int>
// Response: models.ConversationsPage
func (h *MessageHandler) GetConversations(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	cursor := c.Query("cursor")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	page, err := h.svc.GetConversations(c.Request.Context(), userID, cursor, limit)
	if err != nil {
		h.log.Error("GetConversations failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.JSON(http.StatusOK, page)
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// SendMessage godoc
// POST /messages
// Body: models.SendMessageRequest
// Response: models.Message
func (h *MessageHandler) SendMessage(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	var req models.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := h.svc.SendMessage(c.Request.Context(), userID, &req)
	if err != nil {
		h.log.Error("SendMessage failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.JSON(http.StatusCreated, msg)
}

// GetMessages godoc
// GET /messages?conversation_id=<uuid>&cursor=<base64>&limit=<int>
// Response: models.MessagesPage
func (h *MessageHandler) GetMessages(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	convIDStr := c.Query("conversation_id")
	if convIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
		return
	}
	convID, err := uuid.Parse(convIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation_id"})
		return
	}

	cursor := c.Query("cursor")
	limit := 30
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	page, err := h.svc.GetMessages(c.Request.Context(), userID, convID, cursor, limit)
	if err != nil {
		h.log.Error("GetMessages failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.JSON(http.StatusOK, page)
}

// MarkRead godoc
// POST /messages/read
// Body: { "conversation_id": "<uuid>" }
func (h *MessageHandler) MarkRead(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	var body struct {
		ConversationID uuid.UUID `json:"conversation_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.MarkRead(c.Request.Context(), body.ConversationID, userID); err != nil {
		h.log.Error("MarkRead failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// DeleteMessage godoc
// DELETE /messages/:id
func (h *MessageHandler) DeleteMessage(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	msgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}

	if err = h.svc.DeleteMessage(c.Request.Context(), msgID, userID); err != nil {
		h.log.Error("DeleteMessage failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetUnreadCount godoc
// GET /conversations/:id/unread
func (h *MessageHandler) GetUnreadCount(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	count, err := h.svc.GetUnreadCount(c.Request.Context(), convID, userID)
	if err != nil {
		h.log.Error("GetUnreadCount failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}

// GetTotalUnreadCount godoc
// GET /conversations/unread/total
func (h *MessageHandler) GetTotalUnreadCount(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	count, err := h.svc.GetTotalUnreadCount(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("GetTotalUnreadCount failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"total_unread_count": count})
}

// ---------------------------------------------------------------------------
// Groups
// ---------------------------------------------------------------------------

// CreateGroup godoc
// POST /groups
// Body: models.CreateGroupRequest
func (h *MessageHandler) CreateGroup(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	var req models.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conv, err := h.svc.CreateGroup(c.Request.Context(), userID, &req)
	if err != nil {
		h.log.Error("CreateGroup failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.JSON(http.StatusCreated, conv)
}

// AddGroupMembers godoc
// POST /groups/:id/members
// Body: models.AddGroupMemberRequest
func (h *MessageHandler) AddGroupMembers(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	var req models.AddGroupMemberRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err = h.svc.AddGroupMembers(c.Request.Context(), convID, userID, &req); err != nil {
		h.log.Error("AddGroupMembers failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// RemoveGroupMember godoc
// DELETE /groups/:id/members
// Body: models.RemoveGroupMemberRequest
func (h *MessageHandler) RemoveGroupMember(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	var req models.RemoveGroupMemberRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err = h.svc.RemoveGroupMember(c.Request.Context(), convID, userID, &req); err != nil {
		h.log.Error("RemoveGroupMember failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Media upload
// ---------------------------------------------------------------------------

// ShareMedia godoc
// POST /media/upload?conversation_id=<uuid>
// Multipart form field: "file"
func (h *MessageHandler) ShareMedia(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	convIDStr := c.Query("conversation_id")
	if convIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
		return
	}
	convID, err := uuid.Parse(convIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation_id"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required: " + err.Error()})
		return
	}

	resp, err := h.svc.ShareMedia(c.Request.Context(), userID, convID, fileHeader)
	if err != nil {
		h.log.Error("ShareMedia failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ---------------------------------------------------------------------------
// Reactions
// ---------------------------------------------------------------------------

// AddReaction godoc
// POST /messages/:id/reactions
// Body: { "emoji": "..." }
func (h *MessageHandler) AddReaction(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	msgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}

	var body struct {
		Emoji string `json:"emoji" binding:"required"`
	}
	if err = c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reaction, err := h.svc.AddReaction(c.Request.Context(), msgID, userID, body.Emoji)
	if err != nil {
		h.log.Error("AddReaction failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.JSON(http.StatusCreated, reaction)
}

// RemoveReaction godoc
// DELETE /messages/:id/reactions/:emoji
func (h *MessageHandler) RemoveReaction(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		return
	}

	msgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}

	emoji := c.Param("emoji")
	if emoji == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "emoji is required"})
		return
	}

	if err = h.svc.RemoveReaction(c.Request.Context(), msgID, userID, emoji); err != nil {
		h.log.Error("RemoveReaction failed", zap.Error(err))
		handleSvcError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
