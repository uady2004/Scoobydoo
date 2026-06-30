package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/notification-service/internal/models"
	"github.com/tiktok-clone/notification-service/internal/services"
)

// NotificationHandler wires the notification service to HTTP routes.
type NotificationHandler struct {
	svc    services.NotificationService
	logger *zap.Logger
}

// NewNotificationHandler creates a handler bound to the given service.
func NewNotificationHandler(svc services.NotificationService, logger *zap.Logger) *NotificationHandler {
	return &NotificationHandler{svc: svc, logger: logger}
}

// RegisterRoutes attaches the handler to a Gin router group.
// The group should already have the JWT auth middleware applied.
func (h *NotificationHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/notifications", h.GetNotifications)
	rg.PUT("/notifications/read", h.MarkAllRead)
	rg.PUT("/notifications/read-all", h.MarkAllRead) // Flutter alias
	rg.PUT("/notifications/:id/read", h.MarkAsRead)
	rg.GET("/notifications/unread-count", h.GetUnreadCount)
	rg.POST("/devices", h.RegisterDevice)
	rg.DELETE("/devices/:token", h.UnregisterDevice)
	rg.GET("/preferences", h.GetPreferences)
	rg.PUT("/preferences", h.UpdatePreferences)
}

// ---------------------------------------------------------------------------
// userIDFromCtx extracts the authenticated user ID injected by the JWT middleware.
// ---------------------------------------------------------------------------

func userIDFromCtx(c *gin.Context) (string, bool) {
	uid, exists := c.Get("user_id")
	if !exists {
		return "", false
	}
	s, ok := uid.(string)
	return s, ok
}

// ---------------------------------------------------------------------------
// GET /notifications
// ---------------------------------------------------------------------------

// GetNotifications godoc
// @Summary      List notifications for the authenticated user
// @Tags         notifications
// @Produce      json
// @Param        limit       query  int   false  "Page size (1-100, default 20)"
// @Param        offset      query  int   false  "Page offset (default 0)"
// @Param        unread_only query  bool  false  "Return only unread notifications"
// @Success      200  {object}  models.NotificationsResponse
// @Failure      401  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Router       /notifications [get]
func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}

	limit := queryInt(c, "limit", 20)
	offset := queryInt(c, "offset", 0)
	unreadOnly := c.Query("unread_only") == "true"

	req := models.ListNotificationsRequest{
		UserID:     userID,
		Limit:      limit,
		Offset:     offset,
		UnreadOnly: unreadOnly,
	}

	resp, err := h.svc.GetNotifications(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("get notifications", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to retrieve notifications"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// PUT /notifications/:id/read
// ---------------------------------------------------------------------------

// MarkAsRead godoc
// @Summary      Mark a single notification as read
// @Tags         notifications
// @Param        id   path  string  true  "Notification ID"
// @Success      204
// @Failure      401  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Router       /notifications/{id}/read [put]
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}

	notificationID := c.Param("id")
	if notificationID == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "notification id is required"})
		return
	}

	if err := h.svc.MarkAsRead(c.Request.Context(), notificationID, userID); err != nil {
		h.logger.Error("mark as read",
			zap.String("notification_id", notificationID),
			zap.String("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to mark notification as read"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// PUT /notifications/read  (mark all)
// ---------------------------------------------------------------------------

// MarkAllRead godoc
// @Summary      Mark all notifications as read for the authenticated user
// @Tags         notifications
// @Success      204
// @Failure      401  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Router       /notifications/read [put]
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}

	if err := h.svc.MarkAllRead(c.Request.Context(), userID); err != nil {
		h.logger.Error("mark all read", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to mark all notifications as read"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// GET /notifications/unread-count
// ---------------------------------------------------------------------------

// GetUnreadCount returns the number of unread notifications for the authenticated user.
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}

	resp, err := h.svc.GetNotifications(c.Request.Context(), models.ListNotificationsRequest{
		UserID:     userID,
		Limit:      1,
		Offset:     0,
		UnreadOnly: true,
	})
	if err != nil {
		h.logger.Error("get unread count", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to retrieve unread count"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"unread_count": resp.UnreadCount})
}

// ---------------------------------------------------------------------------
// POST /devices
// ---------------------------------------------------------------------------

// RegisterDevice godoc
// @Summary      Register a push notification device token
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        body  body  models.RegisterDeviceRequest  true  "Device registration payload"
// @Success      201
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Router       /devices [post]
func (h *NotificationHandler) RegisterDevice(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}

	var req models.RegisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := h.svc.RegisterDevice(c.Request.Context(), userID, &req); err != nil {
		h.logger.Error("register device", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to register device"})
		return
	}
	c.Status(http.StatusCreated)
}

// ---------------------------------------------------------------------------
// DELETE /devices/:token
// ---------------------------------------------------------------------------

// UnregisterDevice godoc
// @Summary      Unregister a push notification device token
// @Tags         devices
// @Param        token  path  string  true  "FCM device token"
// @Success      204
// @Failure      401  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Router       /devices/{token} [delete]
func (h *NotificationHandler) UnregisterDevice(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}

	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "token is required"})
		return
	}

	if err := h.svc.UnregisterDevice(c.Request.Context(), token, userID); err != nil {
		h.logger.Error("unregister device",
			zap.String("user_id", userID),
			zap.String("token", token),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to unregister device"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// GET /preferences
// ---------------------------------------------------------------------------

// GetPreferences godoc
// @Summary      Get notification preferences for the authenticated user
// @Tags         preferences
// @Produce      json
// @Success      200  {object}  models.NotificationPreference
// @Failure      401  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Router       /preferences [get]
func (h *NotificationHandler) GetPreferences(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}

	prefs, err := h.svc.GetPreferences(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("get preferences", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to retrieve preferences"})
		return
	}
	c.JSON(http.StatusOK, prefs)
}

// ---------------------------------------------------------------------------
// PUT /preferences
// ---------------------------------------------------------------------------

// UpdatePreferences godoc
// @Summary      Update notification preferences for the authenticated user
// @Tags         preferences
// @Accept       json
// @Produce      json
// @Param        body  body  models.UpdatePreferencesRequest  true  "Partial preferences update"
// @Success      200  {object}  models.NotificationPreference
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Router       /preferences [put]
func (h *NotificationHandler) UpdatePreferences(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}

	var req models.UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	prefs, err := h.svc.UpdatePreferences(c.Request.Context(), userID, &req)
	if err != nil {
		h.logger.Error("update preferences", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to update preferences"})
		return
	}
	c.JSON(http.StatusOK, prefs)
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

type errorResponse struct {
	Error string `json:"error"`
}

// queryInt reads a query parameter as an int, returning the default if missing or invalid.
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
