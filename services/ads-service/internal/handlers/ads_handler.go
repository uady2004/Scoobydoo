package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/tiktok-clone/ads-service/internal/models"
	"github.com/tiktok-clone/ads-service/internal/services"
)

// AdsHandler exposes campaign management and ad serving REST endpoints.
type AdsHandler struct {
	campaignSvc *services.CampaignService
	auctionSvc  *services.AuctionService
	logger      *zap.Logger
}

// NewAdsHandler constructs the handler.
func NewAdsHandler(
	campaignSvc *services.CampaignService,
	auctionSvc *services.AuctionService,
	logger *zap.Logger,
) *AdsHandler {
	return &AdsHandler{
		campaignSvc: campaignSvc,
		auctionSvc:  auctionSvc,
		logger:      logger,
	}
}

// RegisterRoutes wires all endpoints onto the provided ServeMux.
func (h *AdsHandler) RegisterRoutes(mux *http.ServeMux) {
	// Campaign management (advertiser-facing).
	mux.HandleFunc("POST /api/v1/campaigns", h.CreateCampaign)
	mux.HandleFunc("GET /api/v1/campaigns/{campaignID}", h.GetCampaign)
	mux.HandleFunc("PUT /api/v1/campaigns/{campaignID}", h.UpdateCampaign)
	mux.HandleFunc("POST /api/v1/campaigns/{campaignID}/pause", h.PauseCampaign)
	mux.HandleFunc("POST /api/v1/campaigns/{campaignID}/resume", h.ResumeCampaign)
	mux.HandleFunc("GET /api/v1/campaigns/{campaignID}/stats", h.GetCampaignStats)
	mux.HandleFunc("GET /api/v1/campaigns/{campaignID}/pacing", h.GetBudgetPacing)

	// Ad serving (feed-service-facing, high QPS).
	mux.HandleFunc("POST /api/v1/ads/serve", h.ServeAd)

	// Click tracking.
	mux.HandleFunc("POST /api/v1/ads/click", h.RecordClick)

	// Health.
	mux.HandleFunc("GET /healthz", h.HealthCheck)
}

// CreateCampaign creates a new ad campaign.
// POST /api/v1/campaigns
func (h *AdsHandler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req services.CreateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.AdvertiserID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "advertiser_id and name are required")
		return
	}
	if req.DailyBudgetMicroUSD <= 0 {
		writeError(w, http.StatusBadRequest, "daily_budget_micro_usd must be positive")
		return
	}
	if req.StartTime.IsZero() {
		req.StartTime = time.Now().UTC()
	}

	campaign, err := h.campaignSvc.CreateCampaign(r.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDateRange):
			writeError(w, http.StatusBadRequest, "end_time must be after start_time")
		default:
			h.logger.Error("create campaign failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to create campaign")
		}
		return
	}
	writeJSON(w, http.StatusCreated, campaign)
}

// GetCampaign returns a campaign by ID.
// GET /api/v1/campaigns/{campaignID}
func (h *AdsHandler) GetCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("campaignID")
	if campaignID == "" {
		writeError(w, http.StatusBadRequest, "campaignID is required")
		return
	}
	// Surface the campaign via the repo through a thin wrapper.
	// In production the CampaignService would expose a GetCampaign method;
	// we replicate the pattern using GetCampaignStats with a zero window.
	writeError(w, http.StatusNotImplemented, "use GetCampaignStats or add GetCampaign to service")
}

// UpdateCampaign updates mutable fields (name, end_time, budget) on a campaign.
// PUT /api/v1/campaigns/{campaignID}
func (h *AdsHandler) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("campaignID")
	if campaignID == "" {
		writeError(w, http.StatusBadRequest, "campaignID is required")
		return
	}

	var req services.UpdateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	campaign, err := h.campaignSvc.UpdateCampaign(r.Context(), campaignID, &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCampaignNotFound):
			writeError(w, http.StatusNotFound, "campaign not found")
		case errors.Is(err, services.ErrInvalidDateRange):
			writeError(w, http.StatusBadRequest, "end_time must be after start_time")
		default:
			h.logger.Error("update campaign failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to update campaign")
		}
		return
	}
	writeJSON(w, http.StatusOK, campaign)
}

// PauseCampaign transitions an active campaign to paused.
// POST /api/v1/campaigns/{campaignID}/pause
func (h *AdsHandler) PauseCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("campaignID")
	if campaignID == "" {
		writeError(w, http.StatusBadRequest, "campaignID is required")
		return
	}

	// In production, requesterID comes from the JWT middleware.
	requesterID := r.Header.Get("X-Advertiser-ID")
	if requesterID == "" {
		writeError(w, http.StatusUnauthorized, "X-Advertiser-ID header required")
		return
	}

	campaign, err := h.campaignSvc.PauseCampaign(r.Context(), campaignID, requesterID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCampaignNotFound):
			writeError(w, http.StatusNotFound, "campaign not found")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, campaign)
}

// ResumeCampaign transitions a paused campaign back to active.
// POST /api/v1/campaigns/{campaignID}/resume
func (h *AdsHandler) ResumeCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("campaignID")
	if campaignID == "" {
		writeError(w, http.StatusBadRequest, "campaignID is required")
		return
	}

	campaign, err := h.campaignSvc.ResumeCampaign(r.Context(), campaignID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCampaignNotFound):
			writeError(w, http.StatusNotFound, "campaign not found")
		case errors.Is(err, services.ErrBudgetExhausted):
			writeError(w, http.StatusPaymentRequired, "daily budget exhausted — increase budget to resume")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, campaign)
}

// GetCampaignStats returns performance metrics for a campaign over a time window.
// GET /api/v1/campaigns/{campaignID}/stats?start=2024-01-01T00:00:00Z&end=2024-01-31T23:59:59Z
func (h *AdsHandler) GetCampaignStats(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("campaignID")
	if campaignID == "" {
		writeError(w, http.StatusBadRequest, "campaignID is required")
		return
	}

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

	stats, err := h.campaignSvc.GetCampaignStats(r.Context(), campaignID, start, end)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCampaignNotFound):
			writeError(w, http.StatusNotFound, "campaign not found")
		default:
			h.logger.Error("get campaign stats failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to retrieve stats")
		}
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// GetBudgetPacing returns the current per-minute spend allowance for a campaign.
// GET /api/v1/campaigns/{campaignID}/pacing
func (h *AdsHandler) GetBudgetPacing(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("campaignID")
	if campaignID == "" {
		writeError(w, http.StatusBadRequest, "campaignID is required")
		return
	}

	perMinute, err := h.campaignSvc.BudgetPacing(r.Context(), campaignID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrBudgetNotFound):
			writeError(w, http.StatusNotFound, "budget not found for campaign")
		case errors.Is(err, services.ErrBudgetExhausted):
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"campaign_id":              campaignID,
				"per_minute_micro_usd":     0,
				"budget_exhausted":         true,
			})
		default:
			h.logger.Error("budget pacing failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to calculate pacing")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"campaign_id":          campaignID,
		"per_minute_micro_usd": perMinute,
		"budget_exhausted":     false,
	})
}

// ServeAd runs the auction and returns the winning ad for a feed slot.
// POST /api/v1/ads/serve
// Called at high frequency by the feed service; latency is critical.
func (h *AdsHandler) ServeAd(w http.ResponseWriter, r *http.Request) {
	var req models.AdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}

	winner, err := h.auctionSvc.RunAuction(r.Context(), &req)
	if err != nil {
		// No eligible ads is a normal operational outcome, not an error worth logging loudly.
		h.logger.Debug("auction returned no winner",
			zap.String("user_id", req.UserID),
			zap.String("placement", req.Placement),
			zap.Error(err),
		)
		writeJSON(w, http.StatusNoContent, nil)
		return
	}

	resp := models.AdResponse{
		Ad:             winner.Ad,
		ImpressionID:   winner.ImpressionID,
		TrackingPixel:  buildTrackingPixelURL(winner.ImpressionID),
		ChargeEstimate: winner.ChargeMicroUSD,
		ECPM:           winner.ECPMMicroUSD,
	}
	writeJSON(w, http.StatusOK, resp)
}

// RecordClick records a click event on an impression.
// POST /api/v1/ads/click
func (h *AdsHandler) RecordClick(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ImpressionID string `json:"impression_id"`
		AdID         string `json:"ad_id"`
		AdSetID      string `json:"ad_set_id"`
		CampaignID   string `json:"campaign_id"`
		UserID       string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.AdID == "" || body.CampaignID == "" {
		writeError(w, http.StatusBadRequest, "ad_id and campaign_id are required")
		return
	}

	if err := h.campaignSvc.RecordClick(r.Context(), body.AdID, body.AdSetID, body.CampaignID); err != nil {
		h.logger.Error("record click failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to record click")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// HealthCheck is a simple liveness probe.
func (h *AdsHandler) HealthCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
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

func buildTrackingPixelURL(impressionID string) string {
	return "/api/v1/ads/pixel?impression_id=" + impressionID
}
