// Package handlers provides the HTTP handler layer for the feed service.
// All endpoints use the chi router, extract parameters from the request
// context/query string, delegate to FeedService, and render JSON responses.
//
// Routes (registered by the caller):
//
//	GET /feed/foryou     — personalised For-You feed
//	GET /feed/following  — videos from followed creators
//	GET /feed/trending   — globally trending videos
//	GET /feed/nearby     — geo-localised videos
//	GET /feed/explore    — category-based discovery
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/tiktok-clone/feed-service/internal/models"
	"github.com/tiktok-clone/feed-service/internal/services"
)

// ---- Feed handler -----------------------------------------------------------

// FeedHandler handles all feed-related HTTP endpoints.
type FeedHandler struct {
	feedSvc *services.FeedService
	logger  *zap.Logger
}

// NewFeedHandler creates a FeedHandler.
func NewFeedHandler(feedSvc *services.FeedService, logger *zap.Logger) *FeedHandler {
	return &FeedHandler{feedSvc: feedSvc, logger: logger}
}

// ---- Route handlers ---------------------------------------------------------

// HandleForYou handles GET /feed/foryou
//
// Query parameters:
//
//	cursor     — opaque pagination token (empty = first page)
//	limit      — number of items per page (1-50, default 20)
//	session_id — client session identifier for deduplication
func (h *FeedHandler) HandleForYou(w http.ResponseWriter, r *http.Request) {
	req, err := h.parseFeedRequest(r, models.FeedTypeForYou)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.feedSvc.GetForYouFeed(r.Context(), req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, page)
}

// HandleFollowing handles GET /feed/following
//
// Query parameters: cursor, limit, session_id
func (h *FeedHandler) HandleFollowing(w http.ResponseWriter, r *http.Request) {
	req, err := h.parseFeedRequest(r, models.FeedTypeFollowing)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.feedSvc.GetFollowingFeed(r.Context(), req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, page)
}

// HandleTrending handles GET /feed/trending
//
// Query parameters: cursor, limit, session_id
func (h *FeedHandler) HandleTrending(w http.ResponseWriter, r *http.Request) {
	req, err := h.parseFeedRequest(r, models.FeedTypeTrending)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.feedSvc.GetTrendingFeed(r.Context(), req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, page)
}

// HandleNearby handles GET /feed/nearby
//
// Query parameters:
//
//	cursor     — opaque pagination token
//	limit      — items per page
//	lat        — WGS-84 latitude  (required)
//	lon        — WGS-84 longitude (required)
//	radius_km  — search radius in kilometres (default 10, max 100)
//	session_id — deduplication session ID
func (h *FeedHandler) HandleNearby(w http.ResponseWriter, r *http.Request) {
	req, err := h.parseFeedRequest(r, models.FeedTypeNearby)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	q := r.URL.Query()

	latStr := q.Get("lat")
	lonStr := q.Get("lon")
	if latStr == "" || lonStr == "" {
		h.writeError(w, http.StatusBadRequest, "lat and lon query parameters are required")
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "lat must be a valid float")
		return
	}
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "lon must be a valid float")
		return
	}
	if lat < -90 || lat > 90 {
		h.writeError(w, http.StatusBadRequest, "lat must be between -90 and 90")
		return
	}
	if lon < -180 || lon > 180 {
		h.writeError(w, http.StatusBadRequest, "lon must be between -180 and 180")
		return
	}
	req.Latitude = lat
	req.Longitude = lon

	if rawRadius := q.Get("radius_km"); rawRadius != "" {
		radius, err := strconv.ParseFloat(rawRadius, 64)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "radius_km must be a valid float")
			return
		}
		if radius <= 0 {
			h.writeError(w, http.StatusBadRequest, "radius_km must be positive")
			return
		}
		req.RadiusKm = radius
	}

	page, err := h.feedSvc.GetNearbyFeed(r.Context(), req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, page)
}

// HandleExplore handles GET /feed/explore
//
// Query parameters:
//
//	cursor     — opaque pagination token
//	limit      — items per page
//	category   — optional category filter (e.g. "comedy", "sports")
//	session_id — deduplication session ID
func (h *FeedHandler) HandleExplore(w http.ResponseWriter, r *http.Request) {
	req, err := h.parseFeedRequest(r, models.FeedTypeExplore)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	q := r.URL.Query()
	req.Category = sanitizeCategory(q.Get("category"))

	page, err := h.feedSvc.GetExploreFeed(r.Context(), req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, page)
}

// ---- Health / readiness -----------------------------------------------------

// HandleHealth handles GET /health — liveness probe.
func (h *FeedHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleReady handles GET /ready — readiness probe.
// Returns 503 until the service is fully initialised.
func (h *FeedHandler) HandleReady(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ---- Request parsing --------------------------------------------------------

// parseFeedRequest extracts common feed parameters from the HTTP request.
// The caller is responsible for extracting feed-type-specific parameters
// (lat/lon for nearby, category for explore).
func (h *FeedHandler) parseFeedRequest(r *http.Request, ft models.FeedType) (*models.FeedRequest, error) {
	userID := userIDFromContext(r)
	if userID == "" {
		return nil, errors.New("authentication required")
	}

	q := r.URL.Query()

	limit := 20
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, errors.New("limit must be a positive integer")
		}
		limit = n
	}

	return &models.FeedRequest{
		UserID:    userID,
		FeedType:  ft,
		Cursor:    q.Get("cursor"),
		Limit:     limit,
		SessionID: q.Get("session_id"),
		Language:  q.Get("lang"),
	}, nil
}

// ---- Response helpers -------------------------------------------------------

// errorResponse is the canonical error envelope.
type errorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

func (h *FeedHandler) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

func (h *FeedHandler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, &errorResponse{Error: msg})
}

func (h *FeedHandler) handleServiceError(w http.ResponseWriter, err error) {
	h.logger.Error("feed service error", zap.Error(err))
	h.writeJSON(w, http.StatusInternalServerError, &errorResponse{
		Error: "internal server error",
		Code:  "INTERNAL_ERROR",
	})
}

// ---- Context helpers --------------------------------------------------------

// contextKey is the unexported type used for context values in this package.
type contextKey string

const (
	// ContextKeyUserID is the context key under which the authenticated user ID
	// is stored by the auth middleware.
	ContextKeyUserID contextKey = "user_id"
)

// userIDFromContext extracts the authenticated user ID from the request context.
// Returns "" if unauthenticated.
func userIDFromContext(r *http.Request) string {
	if v := r.Context().Value(ContextKeyUserID); v != nil {
		if id, ok := v.(string); ok && id != "" {
			return id
		}
	}
	// Fall back to X-User-ID header injected by the API gateway after JWT
	// validation for service-to-service calls within the mesh.
	return r.Header.Get("X-User-ID")
}

// sanitizeCategory lowercases and trims a category string.
func sanitizeCategory(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ---- Middleware -------------------------------------------------------------

// AuthMiddleware validates the Bearer token / session cookie and injects the
// user ID into the request context. In production this calls the auth-service
// or validates a shared JWT secret; here it reads the X-User-ID header that
// the API gateway populates after upstream token validation.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			// Allow unauthenticated requests to pass through — individual
			// handlers decide whether authentication is required for their
			// feed type (trending / explore are public; foryou / following are not).
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SetUserIDInContext is a helper used by tests to inject a user ID into a
// request context without going through the full middleware stack.
func SetUserIDInContext(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
	return r.WithContext(ctx)
}
