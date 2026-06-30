package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/search-service/internal/services"
)

// SearchHandler exposes REST endpoints for all search functionality.
type SearchHandler struct {
	svc    *services.SearchService
	logger *zap.Logger
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(svc *services.SearchService, logger *zap.Logger) *SearchHandler {
	return &SearchHandler{svc: svc, logger: logger}
}

// RegisterRoutes attaches all search routes to a Gin RouterGroup.
//
// Routes:
//
//	GET  /search                   — unified search across all entity types
//	GET  /search/videos            — video-only search with filters
//	GET  /search/users             — user search (autocomplete)
//	GET  /search/hashtags          — hashtag search
//	GET  /search/products          — product search
//	GET  /search/sounds            — sound/music search
//	GET  /search/trending          — trending search queries
//	GET  /search/suggestions       — autocomplete suggestions
//	GET  /search/history           — authenticated user search history
//	DELETE /search/history         — clear search history
func (h *SearchHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.UnifiedSearch)
	rg.GET("/videos", h.SearchVideos)
	rg.GET("/users", h.SearchUsers)
	rg.GET("/hashtags", h.SearchHashtags)
	rg.GET("/products", h.SearchProducts)
	rg.GET("/sounds", h.SearchSounds)
	rg.GET("/trending", h.GetTrendingSearches)
	rg.GET("/suggestions", h.GetSearchSuggestions)
	rg.GET("/history", h.GetSearchHistory)
	rg.DELETE("/history", h.DeleteSearchHistory)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func respondError(c *gin.Context, status int, code, msg string) {
	c.JSON(status, gin.H{"code": code, "message": msg})
}

func pagingParams(c *gin.Context) (page, limit int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return
}

// extractUserID pulls the optional authenticated user ID from the Gin context.
// Returns empty string when the request is unauthenticated.
func extractUserID(c *gin.Context) string {
	if id, exists := c.Get("user_id"); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// UnifiedSearch godoc
// GET /search?q=...&page=1&limit=20
func (h *SearchHandler) UnifiedSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		respondError(c, http.StatusBadRequest, "MISSING_QUERY", "q parameter is required")
		return
	}

	page, limit := pagingParams(c)
	userID := extractUserID(c)

	resp, err := h.svc.UnifiedSearch(c.Request.Context(), userID, query, page, limit)
	if err != nil {
		if errors.Is(err, services.ErrInvalidQuery) {
			respondError(c, http.StatusBadRequest, "INVALID_QUERY", err.Error())
			return
		}
		h.logger.Error("UnifiedSearch", zap.String("query", query), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "SEARCH_ERROR", "search failed")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SearchVideos godoc
// GET /search/videos?q=...&sort_by=views&min_duration=10&max_duration=60&creator_id=&hashtags=a,b
func (h *SearchHandler) SearchVideos(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		respondError(c, http.StatusBadRequest, "MISSING_QUERY", "q parameter is required")
		return
	}

	page, limit := pagingParams(c)

	var hashtags []string
	if raw := c.Query("hashtags"); raw != "" {
		for _, ht := range splitCSV(raw) {
			hashtags = append(hashtags, ht)
		}
	}

	minDur, _ := strconv.Atoi(c.Query("min_duration"))
	maxDur, _ := strconv.Atoi(c.Query("max_duration"))

	filters := services.SearchFilters{
		MinDuration: minDur,
		MaxDuration: maxDur,
		CreatorID:   c.Query("creator_id"),
		Hashtags:    hashtags,
		SortBy:      c.DefaultQuery("sort_by", "relevance"),
		DateFrom:    c.Query("date_from"),
		DateTo:      c.Query("date_to"),
	}

	resp, err := h.svc.SearchVideos(c.Request.Context(), query, filters, page, limit)
	if err != nil {
		if errors.Is(err, services.ErrInvalidQuery) {
			respondError(c, http.StatusBadRequest, "INVALID_QUERY", err.Error())
			return
		}
		h.logger.Error("SearchVideos", zap.String("query", query), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "SEARCH_ERROR", "video search failed")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SearchUsers godoc
// GET /search/users?q=...&page=1&limit=20
func (h *SearchHandler) SearchUsers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		respondError(c, http.StatusBadRequest, "MISSING_QUERY", "q parameter is required")
		return
	}

	page, limit := pagingParams(c)
	resp, err := h.svc.SearchUsers(c.Request.Context(), query, page, limit)
	if err != nil {
		if errors.Is(err, services.ErrInvalidQuery) {
			respondError(c, http.StatusBadRequest, "INVALID_QUERY", err.Error())
			return
		}
		h.logger.Error("SearchUsers", zap.String("query", query), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "SEARCH_ERROR", "user search failed")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SearchHashtags godoc
// GET /search/hashtags?q=...&page=1&limit=20
func (h *SearchHandler) SearchHashtags(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		respondError(c, http.StatusBadRequest, "MISSING_QUERY", "q parameter is required")
		return
	}

	page, limit := pagingParams(c)
	resp, err := h.svc.SearchHashtags(c.Request.Context(), query, page, limit)
	if err != nil {
		if errors.Is(err, services.ErrInvalidQuery) {
			respondError(c, http.StatusBadRequest, "INVALID_QUERY", err.Error())
			return
		}
		h.logger.Error("SearchHashtags", zap.String("query", query), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "SEARCH_ERROR", "hashtag search failed")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SearchProducts godoc
// GET /search/products?q=...&min_price=0&max_price=100&page=1&limit=20
func (h *SearchHandler) SearchProducts(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		respondError(c, http.StatusBadRequest, "MISSING_QUERY", "q parameter is required")
		return
	}

	page, limit := pagingParams(c)
	minPrice, _ := strconv.ParseFloat(c.Query("min_price"), 64)
	maxPrice, _ := strconv.ParseFloat(c.Query("max_price"), 64)

	resp, err := h.svc.SearchProducts(c.Request.Context(), query, minPrice, maxPrice, page, limit)
	if err != nil {
		if errors.Is(err, services.ErrInvalidQuery) {
			respondError(c, http.StatusBadRequest, "INVALID_QUERY", err.Error())
			return
		}
		h.logger.Error("SearchProducts", zap.String("query", query), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "SEARCH_ERROR", "product search failed")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SearchSounds godoc
// GET /search/sounds?q=...&page=1&limit=20
func (h *SearchHandler) SearchSounds(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		respondError(c, http.StatusBadRequest, "MISSING_QUERY", "q parameter is required")
		return
	}

	page, limit := pagingParams(c)
	resp, err := h.svc.SearchSounds(c.Request.Context(), query, page, limit)
	if err != nil {
		if errors.Is(err, services.ErrInvalidQuery) {
			respondError(c, http.StatusBadRequest, "INVALID_QUERY", err.Error())
			return
		}
		h.logger.Error("SearchSounds", zap.String("query", query), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "SEARCH_ERROR", "sound search failed")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetTrendingSearches godoc
// GET /search/trending?limit=10
func (h *SearchHandler) GetTrendingSearches(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	trends, err := h.svc.GetTrendingSearches(c.Request.Context(), limit)
	if err != nil {
		h.logger.Error("GetTrendingSearches", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "TRENDING_ERROR", "failed to fetch trending searches")
		return
	}

	c.JSON(http.StatusOK, gin.H{"trending": trends})
}

// GetSearchSuggestions godoc
// GET /search/suggestions?q=...&limit=8
func (h *SearchHandler) GetSearchSuggestions(c *gin.Context) {
	prefix := c.Query("q")
	if prefix == "" {
		c.JSON(http.StatusOK, gin.H{"suggestions": []interface{}{}})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "8"))
	userID := extractUserID(c)

	suggestions, err := h.svc.GetSearchSuggestions(c.Request.Context(), userID, prefix, limit)
	if err != nil {
		h.logger.Error("GetSearchSuggestions", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "SUGGESTION_ERROR", "failed to get suggestions")
		return
	}

	if suggestions == nil {
		suggestions = []services.SearchSuggestion{}
	}
	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

// GetSearchHistory godoc
// GET /search/history?limit=20  — requires authentication
func (h *SearchHandler) GetSearchHistory(c *gin.Context) {
	userID := extractUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	history, err := h.svc.GetSearchHistory(c.Request.Context(), userID, limit)
	if err != nil {
		h.logger.Error("GetSearchHistory", zap.String("user_id", userID), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "HISTORY_ERROR", "failed to retrieve search history")
		return
	}

	if history == nil {
		history = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"history": history})
}

// DeleteSearchHistory godoc
// DELETE /search/history  — requires authentication
func (h *SearchHandler) DeleteSearchHistory(c *gin.Context) {
	userID := extractUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	if err := h.svc.DeleteSearchHistory(c.Request.Context(), userID); err != nil {
		h.logger.Error("DeleteSearchHistory", zap.String("user_id", userID), zap.Error(err))
		respondError(c, http.StatusInternalServerError, "HISTORY_ERROR", "failed to clear search history")
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// splitCSV splits a comma-separated string into trimmed, non-empty tokens.
func splitCSV(s string) []string {
	var out []string
	for _, part := range splitByComma(s) {
		if t := trimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func splitByComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
