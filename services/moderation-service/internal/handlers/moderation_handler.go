package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/tiktok-clone/moderation-service/internal/models"
	"github.com/tiktok-clone/moderation-service/internal/services"
)

// ModerationHandler exposes the moderator dashboard REST API.
type ModerationHandler struct {
	svc    *services.ModerationService
	logger *zap.Logger
}

// NewModerationHandler constructs the handler.
func NewModerationHandler(svc *services.ModerationService, logger *zap.Logger) *ModerationHandler {
	return &ModerationHandler{svc: svc, logger: logger}
}

// RegisterRoutes wires up all endpoints on the provided ServeMux.
func (h *ModerationHandler) RegisterRoutes(mux *http.ServeMux) {
	// Content submission (called internally by other services).
	mux.HandleFunc("POST /api/v1/moderation/submit", h.SubmitContent)

	// Moderator dashboard.
	mux.HandleFunc("GET /api/v1/moderation/queue", h.GetQueue)
	mux.HandleFunc("POST /api/v1/moderation/queue/{queueItemID}/review", h.ReviewContent)
	mux.HandleFunc("GET /api/v1/moderation/results/{contentID}", h.GetResult)

	// Appeals (user-facing).
	mux.HandleFunc("POST /api/v1/moderation/appeals", h.SubmitAppeal)
	mux.HandleFunc("GET /api/v1/moderation/appeals/{appealID}", h.GetAppealStatus)
	mux.HandleFunc("POST /api/v1/moderation/appeals/{appealID}/review", h.ReviewAppeal)

	// Dashboard stats.
	mux.HandleFunc("GET /api/v1/moderation/stats", h.GetStats)

	// Health check.
	mux.HandleFunc("GET /healthz", h.HealthCheck)
}

// SubmitContent accepts a new ModerationRequest from internal services.
// POST /api/v1/moderation/submit
func (h *ModerationHandler) SubmitContent(w http.ResponseWriter, r *http.Request) {
	var req models.ModerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.ContentID == "" || req.ContentURL == "" || req.UserID == "" {
		writeError(w, http.StatusBadRequest, "content_id, content_url, and user_id are required")
		return
	}

	result, err := h.svc.ModerateContent(r.Context(), &req)
	if err != nil {
		h.logger.Error("submit content failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "moderation failed")
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

// GetQueue returns the human-review queue with optional filters.
// GET /api/v1/moderation/queue?status=human_review&limit=50&offset=0&min_score=0.5
func (h *ModerationHandler) GetQueue(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := services.QueueFilter{
		Status:      models.ModerationStatus(q.Get("status")),
		ContentType: models.ContentType(q.Get("content_type")),
		AssignedTo:  q.Get("assigned_to"),
		Limit:       parseIntQuery(q.Get("limit"), 50),
		Offset:      parseIntQuery(q.Get("offset"), 0),
		MinScore:    parseFloatQuery(q.Get("min_score"), 0),
	}

	items, total, err := h.svc.GetModeratorQueue(r.Context(), filter)
	if err != nil {
		h.logger.Error("get queue failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to retrieve queue")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// ReviewContent processes a moderator's decision on a queue item.
// POST /api/v1/moderation/queue/{queueItemID}/review
func (h *ModerationHandler) ReviewContent(w http.ResponseWriter, r *http.Request) {
	queueItemID := r.PathValue("queueItemID")
	if queueItemID == "" {
		writeError(w, http.StatusBadRequest, "queueItemID path parameter is required")
		return
	}

	var body struct {
		ModeratorID string                  `json:"moderator_id"`
		Decision    models.ModerationStatus `json:"decision"`
		Reason      models.RejectionReason  `json:"reason,omitempty"`
		Notes       string                  `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ModeratorID == "" {
		writeError(w, http.StatusBadRequest, "moderator_id is required")
		return
	}

	decision := &models.ReviewDecision{
		QueueItemID: queueItemID,
		ModeratorID: body.ModeratorID,
		Decision:    body.Decision,
		Reason:      body.Reason,
		Notes:       body.Notes,
	}

	result, err := h.svc.ReviewContent(r.Context(), decision)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrQueueItemNotFound):
			writeError(w, http.StatusNotFound, "queue item not found")
		case errors.Is(err, services.ErrAlreadyReviewed):
			writeError(w, http.StatusConflict, "item has already been reviewed")
		case errors.Is(err, services.ErrInvalidDecision):
			writeError(w, http.StatusBadRequest, "decision must be approved or rejected")
		default:
			h.logger.Error("review content failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "review failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetResult returns the moderation result for a given content ID.
// GET /api/v1/moderation/results/{contentID}
func (h *ModerationHandler) GetResult(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		writeError(w, http.StatusBadRequest, "contentID path parameter is required")
		return
	}
	// The moderator-facing handler needs a GetResultByContentID method;
	// we expose it via a thin wrapper on the service.
	// For now we call ModerateContent — in production wire to repo directly.
	writeError(w, http.StatusNotImplemented, "use internal repo method GetResultByContentID")
}

// SubmitAppeal allows a user to contest a rejection decision.
// POST /api/v1/moderation/appeals
func (h *ModerationHandler) SubmitAppeal(w http.ResponseWriter, r *http.Request) {
	// In production, userID comes from a JWT middleware.
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req models.AppealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ResultID == "" || req.ContentID == "" || req.AppealText == "" {
		writeError(w, http.StatusBadRequest, "result_id, content_id, and appeal_text are required")
		return
	}
	if len(req.AppealText) < 20 {
		writeError(w, http.StatusBadRequest, "appeal_text must be at least 20 characters")
		return
	}

	appeal, err := h.svc.AppealDecision(r.Context(), userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrResultNotFound):
			writeError(w, http.StatusNotFound, "moderation result not found")
		case errors.Is(err, services.ErrTooManyAppeals):
			writeError(w, http.StatusTooManyRequests, "maximum active appeals reached")
		default:
			h.logger.Error("submit appeal failed", zap.Error(err))
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, appeal)
}

// GetAppealStatus returns the status of a specific appeal for the authenticated user.
// GET /api/v1/moderation/appeals/{appealID}
func (h *ModerationHandler) GetAppealStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	appealID := r.PathValue("appealID")
	if appealID == "" {
		writeError(w, http.StatusBadRequest, "appealID path parameter is required")
		return
	}

	appeal, err := h.svc.GetAppealStatus(r.Context(), appealID, userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAppealNotFound):
			writeError(w, http.StatusNotFound, "appeal not found")
		default:
			h.logger.Error("get appeal status failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to retrieve appeal")
		}
		return
	}
	writeJSON(w, http.StatusOK, appeal)
}

// ReviewAppeal processes a moderator's decision on an appeal.
// POST /api/v1/moderation/appeals/{appealID}/review
func (h *ModerationHandler) ReviewAppeal(w http.ResponseWriter, r *http.Request) {
	appealID := r.PathValue("appealID")
	if appealID == "" {
		writeError(w, http.StatusBadRequest, "appealID path parameter is required")
		return
	}

	var body struct {
		ModeratorID string `json:"moderator_id"`
		Approve     bool   `json:"approve"`
		Note        string `json:"note,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ModeratorID == "" {
		writeError(w, http.StatusBadRequest, "moderator_id is required")
		return
	}

	appeal, err := h.svc.ReviewAppeal(r.Context(), appealID, body.ModeratorID, body.Approve, body.Note)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAppealNotFound):
			writeError(w, http.StatusNotFound, "appeal not found")
		case errors.Is(err, services.ErrAppealNotReviewable):
			writeError(w, http.StatusConflict, "appeal is not in a reviewable state")
		default:
			h.logger.Error("review appeal failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "review failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, appeal)
}

// GetStats returns aggregated moderation statistics.
// GET /api/v1/moderation/stats?start=2024-01-01T00:00:00Z&end=2024-01-31T23:59:59Z
func (h *ModerationHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	now := time.Now().UTC()
	end := now
	start := now.Add(-24 * time.Hour)

	if s := q.Get("start"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			start = t
		}
	}
	if e := q.Get("end"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			end = t
		}
	}

	stats, err := h.svc.GetStats(r.Context(), start, end)
	if err != nil {
		h.logger.Error("get stats failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to retrieve stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// HealthCheck is a simple liveness probe.
func (h *ModerationHandler) HealthCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseIntQuery(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseFloatQuery(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}
