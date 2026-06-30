package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/reporting-service/internal/models"
	"github.com/tiktok-clone/reporting-service/internal/services"
)

// ReportHandler handles report submission and retrieval endpoints.
type ReportHandler struct {
	svc    services.ReportService
	logger *zap.Logger
}

// NewReportHandler creates a ReportHandler.
func NewReportHandler(svc services.ReportService, logger *zap.Logger) *ReportHandler {
	return &ReportHandler{svc: svc, logger: logger}
}

// RegisterRoutes mounts all report routes.
func (h *ReportHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/reports", h.CreateReport)
	rg.GET("/reports", h.ListReports)
	rg.GET("/reports/:id", h.GetReport)
	rg.PUT("/reports/:id/status", h.UpdateStatus)
}

// CreateReport godoc
// POST /api/v1/reports
func (h *ReportHandler) CreateReport(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		ContentID   string              `json:"content_id" binding:"required"`
		ContentType models.ReportType   `json:"content_type" binding:"required"`
		Reason      models.ReportReason `json:"reason" binding:"required"`
		Description string              `json:"description" binding:"max=500"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	report, err := h.svc.CreateReport(c.Request.Context(), services.CreateReportReq{
		ReporterID:  userID,
		ContentID:   req.ContentID,
		ContentType: req.ContentType,
		Reason:      req.Reason,
		Description: req.Description,
	})
	if err != nil {
		h.logger.Error("CreateReport failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit report"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"report": report})
}

// ListReports godoc
// GET /api/v1/reports?status=pending&limit=20&offset=0
func (h *ReportHandler) ListReports(c *gin.Context) {
	status := models.ReportStatus(c.DefaultQuery("status", string(models.ReportStatusPending)))
	contentType := models.ReportType(c.Query("content_type"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	reports, total, err := h.svc.ListReports(c.Request.Context(), status, contentType, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list reports"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reports": reports, "total": total})
}

// GetReport godoc
// GET /api/v1/reports/:id
func (h *ReportHandler) GetReport(c *gin.Context) {
	report, err := h.svc.GetReport(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"report": report})
}

// UpdateStatus godoc
// PUT /api/v1/reports/:id/status
func (h *ReportHandler) UpdateStatus(c *gin.Context) {
	adminID := c.GetString("user_id")
	var req struct {
		Status           models.ReportStatus `json:"status" binding:"required"`
		ResolutionAction string              `json:"resolution_action"`
		Notes            string              `json:"notes" binding:"max=1000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.ResolveReport(c.Request.Context(), c.Param("id"), adminID, req.Status, req.ResolutionAction, req.Notes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update report"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "report updated"})
}
