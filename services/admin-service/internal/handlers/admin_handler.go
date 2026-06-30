package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/admin-service/internal/models"
	"github.com/tiktok-clone/admin-service/internal/services"
)

// AdminHandler handles all admin panel HTTP endpoints.
type AdminHandler struct {
	svc    services.AdminService
	logger *zap.Logger
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(svc services.AdminService, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{svc: svc, logger: logger}
}

// RegisterRoutes mounts all admin routes.
func (h *AdminHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// Auth
	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/logout", h.Logout)

	// Admin users (superadmin only)
	rg.POST("/admins", h.CreateAdmin)
	rg.GET("/admins", h.ListAdmins)
	rg.PUT("/admins/:id/role", h.UpdateAdminRole)
	rg.DELETE("/admins/:id", h.DeactivateAdmin)

	// Platform users
	rg.GET("/users", h.ListUsers)
	rg.GET("/users/:id", h.GetUser)
	rg.POST("/users/:id/ban", h.BanUser)
	rg.DELETE("/users/:id/ban", h.UnbanUser)
	rg.POST("/users/:id/verify", h.VerifyUser)

	// Content moderation
	rg.GET("/content/pending", h.ListPendingContent)
	rg.POST("/content/:id/approve", h.ApproveContent)
	rg.POST("/content/:id/remove", h.RemoveContent)

	// Reports
	rg.GET("/reports", h.ListReports)
	rg.PUT("/reports/:id/resolve", h.ResolveReport)

	// Dashboard
	rg.GET("/stats", h.GetPlatformStats)
	rg.GET("/audit-log", h.GetAuditLog)
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

func (h *AdminHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, admin, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.logger.Warn("admin login failed", zap.String("email", req.Email), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"admin": gin.H{
			"id":       admin.ID,
			"email":    admin.Email,
			"name":     admin.FullName,
			"role":     admin.Role,
		},
	})
}

func (h *AdminHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// ─── Admin users ──────────────────────────────────────────────────────────────

func (h *AdminHandler) CreateAdmin(c *gin.Context) {
	var req struct {
		Email    string           `json:"email" binding:"required,email"`
		Password string           `json:"password" binding:"required,min=8"`
		FullName string           `json:"full_name" binding:"required"`
		Role     models.AdminRole `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	admin, err := h.svc.CreateAdmin(c.Request.Context(), req.Email, req.Password, req.FullName, req.Role)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"admin": admin})
}

func (h *AdminHandler) ListAdmins(c *gin.Context) {
	admins, err := h.svc.ListAdmins(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list admins"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"admins": admins})
}

func (h *AdminHandler) UpdateAdminRole(c *gin.Context) {
	var req struct {
		Role models.AdminRole `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.UpdateAdminRole(c.Request.Context(), c.Param("id"), req.Role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "role updated"})
}

func (h *AdminHandler) DeactivateAdmin(c *gin.Context) {
	if err := h.svc.DeactivateAdmin(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to deactivate admin"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Platform users ───────────────────────────────────────────────────────────

func (h *AdminHandler) ListUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	search := c.Query("q")

	users, total, err := h.svc.ListUsers(c.Request.Context(), search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "total": total})
}

func (h *AdminHandler) GetUser(c *gin.Context) {
	user, err := h.svc.GetUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *AdminHandler) BanUser(c *gin.Context) {
	adminID := c.GetString("admin_id")
	var req struct {
		Reason    string     `json:"reason" binding:"required"`
		BanType   string     `json:"ban_type" binding:"required,oneof=temporary permanent"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ban, err := h.svc.BanUser(c.Request.Context(), c.Param("id"), adminID, req.Reason, req.BanType, req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to ban user"})
		return
	}

	h.svc.LogAudit(c.Request.Context(), adminID, "user.ban", "user", c.Param("id"),
		map[string]interface{}{"reason": req.Reason, "ban_type": req.BanType},
		c.ClientIP(), c.GetHeader("User-Agent"),
	)

	c.JSON(http.StatusOK, gin.H{"ban": ban})
}

func (h *AdminHandler) UnbanUser(c *gin.Context) {
	adminID := c.GetString("admin_id")
	if err := h.svc.UnbanUser(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unban user"})
		return
	}

	h.svc.LogAudit(c.Request.Context(), adminID, "user.unban", "user", c.Param("id"), nil,
		c.ClientIP(), c.GetHeader("User-Agent"))

	c.JSON(http.StatusOK, gin.H{"message": "user unbanned"})
}

func (h *AdminHandler) VerifyUser(c *gin.Context) {
	adminID := c.GetString("admin_id")
	if err := h.svc.VerifyUser(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify user"})
		return
	}
	h.svc.LogAudit(c.Request.Context(), adminID, "user.verify", "user", c.Param("id"), nil,
		c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "user verified"})
}

// ─── Content moderation ───────────────────────────────────────────────────────

func (h *AdminHandler) ListPendingContent(c *gin.Context) {
	contentType := c.DefaultQuery("type", "video")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items, total, err := h.svc.ListPendingContent(c.Request.Context(), contentType, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list pending content"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *AdminHandler) ApproveContent(c *gin.Context) {
	adminID := c.GetString("admin_id")
	contentType := c.DefaultQuery("type", "video")

	mod, err := h.svc.ModerateContent(c.Request.Context(), c.Param("id"), contentType, adminID, "approve", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve content"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"moderation": mod})
}

func (h *AdminHandler) RemoveContent(c *gin.Context) {
	adminID := c.GetString("admin_id")
	var req struct {
		Reason      string `json:"reason" binding:"required"`
		ContentType string `json:"content_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mod, err := h.svc.ModerateContent(c.Request.Context(), c.Param("id"), req.ContentType, adminID, "remove", req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove content"})
		return
	}

	h.svc.LogAudit(c.Request.Context(), adminID, "content.remove", req.ContentType, c.Param("id"),
		map[string]interface{}{"reason": req.Reason}, c.ClientIP(), c.GetHeader("User-Agent"))

	c.JSON(http.StatusOK, gin.H{"moderation": mod})
}

// ─── Reports ──────────────────────────────────────────────────────────────────

func (h *AdminHandler) ListReports(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	reports, total, err := h.svc.ListReports(c.Request.Context(), status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list reports"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reports": reports, "total": total})
}

func (h *AdminHandler) ResolveReport(c *gin.Context) {
	adminID := c.GetString("admin_id")
	var req struct {
		Action  string `json:"action" binding:"required,oneof=dismiss remove warn ban"`
		Notes   string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.ResolveReport(c.Request.Context(), c.Param("id"), adminID, req.Action, req.Notes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve report"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "report resolved"})
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

func (h *AdminHandler) GetPlatformStats(c *gin.Context) {
	stats, err := h.svc.GetPlatformStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

func (h *AdminHandler) GetAuditLog(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	adminID := c.Query("admin_id")

	logs, total, err := h.svc.GetAuditLog(c.Request.Context(), adminID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get audit log"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": total})
}
