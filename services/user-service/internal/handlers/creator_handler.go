package handlers

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/tiktok-clone/user-service/internal/middleware"
	"github.com/tiktok-clone/user-service/internal/models"
	"github.com/tiktok-clone/user-service/internal/repositories"
	"github.com/tiktok-clone/user-service/internal/services"
	"github.com/tiktok-clone/user-service/internal/validators"
)

// CreatorHandler groups REST handlers for creator-specific endpoints.
type CreatorHandler struct {
	svc       services.ProfileService
	validator *validators.UserValidator
	logger    *zap.Logger
}

// NewCreatorHandler creates a CreatorHandler and registers its routes on the given Echo group.
// The group is expected to already have AuthMiddleware applied.
func NewCreatorHandler(g *echo.Group, svc services.ProfileService, val *validators.UserValidator, logger *zap.Logger) *CreatorHandler {
	h := &CreatorHandler{svc: svc, validator: val, logger: logger}

	// Authenticated creator routes.
	g.GET("/me/creator", h.GetMyCreatorProfile)
	g.PUT("/me/creator", h.UpdateMyCreatorProfile)

	// Public read routes.
	g.GET("/users/:userID/creator", h.GetCreatorProfile)

	// Admin-only verification route.
	g.POST("/admin/users/:userID/verify", h.VerifyCreator, middleware.AdminOnly())

	return h
}

// ---------- GetMyCreatorProfile ----------

// GetMyCreatorProfile godoc
// @Summary      Get the authenticated user's creator profile
// @Tags         creator
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  models.CreatorProfile
// @Failure      401  {object}  errorResponse
// @Failure      404  {object}  errorResponse
// @Router       /me/creator [get]
func (h *CreatorHandler) GetMyCreatorProfile(c echo.Context) error {
	userID := middleware.MustGetUserID(c)
	return h.getCreatorProfileForUser(c, userID)
}

// ---------- GetCreatorProfile ----------

// GetCreatorProfile godoc
// @Summary      Get a creator's public profile by user ID
// @Tags         creator
// @Produce      json
// @Param        userID  path  string  true  "User UUID"
// @Success      200  {object}  models.CreatorProfile
// @Failure      400  {object}  errorResponse
// @Failure      404  {object}  errorResponse
// @Router       /users/{userID}/creator [get]
func (h *CreatorHandler) GetCreatorProfile(c echo.Context) error {
	userID, err := parseUUID(c.Param("userID"))
	if err != nil {
		return badRequest(c, "invalid user ID")
	}
	return h.getCreatorProfileForUser(c, userID)
}

func (h *CreatorHandler) getCreatorProfileForUser(c echo.Context, userID uuid.UUID) error {
	cp, err := h.svc.GetCreatorProfile(c.Request().Context(), userID)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(cp))
}

// ---------- UpdateMyCreatorProfile ----------

// UpdateMyCreatorProfile godoc
// @Summary      Update the authenticated user's creator profile settings
// @Tags         creator
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  updateCreatorProfileRequest  true  "Creator profile update"
// @Success      200  {object}  models.CreatorProfile
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Failure      404  {object}  errorResponse
// @Router       /me/creator [put]
func (h *CreatorHandler) UpdateMyCreatorProfile(c echo.Context) error {
	userID := middleware.MustGetUserID(c)

	var req updateCreatorProfileRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "malformed request body")
	}

	cp := req.toCreatorProfile(userID)

	if err := h.validator.ValidateCreatorProfile(cp); err != nil {
		return validationError(c, err)
	}

	updated, err := h.svc.UpdateCreatorProfile(c.Request().Context(), userID, cp)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(updated))
}

// ---------- VerifyCreator (admin only) ----------

// verifyCreatorRequest is the request body for the admin verification endpoint.
type verifyCreatorRequest struct {
	Tier   models.VerificationTier `json:"tier"   validate:"required"`
	Reason string                  `json:"reason" validate:"required,min=5,max=500"`
}

// VerifyCreator godoc
// @Summary      Grant a verification badge to a user (admin only)
// @Tags         admin
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        userID  path  string                 true  "UUID of the user to verify"
// @Param        body    body  verifyCreatorRequest   true  "Verification details"
// @Success      204  "No Content"
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Failure      403  {object}  errorResponse
// @Failure      404  {object}  errorResponse
// @Router       /admin/users/{userID}/verify [post]
func (h *CreatorHandler) VerifyCreator(c echo.Context) error {
	adminID := middleware.MustGetUserID(c)

	targetUserID, err := parseUUID(c.Param("userID"))
	if err != nil {
		return badRequest(c, "invalid user ID")
	}

	var req verifyCreatorRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "malformed request body")
	}

	if err := validateVerifyRequest(&req); err != nil {
		return validationError(c, err)
	}

	if err := h.svc.VerifyCreator(c.Request().Context(), adminID, targetUserID, req.Tier, req.Reason); err != nil {
		return h.handleServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ---------- creator analytics (extended view) ----------

// GetCreatorAnalytics godoc
// @Summary      Get detailed analytics for the authenticated creator
// @Tags         creator
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  models.AccountAnalytics
// @Failure      401  {object}  errorResponse
// @Router       /me/creator/analytics [get]
func (h *CreatorHandler) GetCreatorAnalytics(c echo.Context) error {
	userID := middleware.MustGetUserID(c)
	analytics, err := h.svc.GetAccountAnalytics(c.Request().Context(), userID)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(analytics))
}

// ---------- request DTOs ----------

// updateCreatorProfileRequest is the JSON body for creator profile updates.
// It deliberately excludes analytics fields (they are read-only).
type updateCreatorProfileRequest struct {
	Category           string   `json:"category"             validate:"required,max=50"`
	SubCategories      []string `json:"sub_categories"       validate:"omitempty,max=5"`
	IsMonetised        bool     `json:"is_monetised"`
	CreatorFundEnabled bool     `json:"creator_fund_enabled"`
	TipEnabled         bool     `json:"tip_enabled"`
	MinimumTipAmount   float64  `json:"minimum_tip_amount"   validate:"min=0,max=100000"`
	BusinessName       string   `json:"business_name"        validate:"omitempty,max=100"`
	BusinessContact    string   `json:"business_contact"     validate:"omitempty,max=200"`
}

func (r *updateCreatorProfileRequest) toCreatorProfile(userID uuid.UUID) *models.CreatorProfile {
	return &models.CreatorProfile{
		UserID:             userID,
		Category:           r.Category,
		SubCategories:      r.SubCategories,
		IsMonetised:        r.IsMonetised,
		CreatorFundEnabled: r.CreatorFundEnabled,
		TipEnabled:         r.TipEnabled,
		MinimumTipAmount:   r.MinimumTipAmount,
		BusinessName:       r.BusinessName,
		BusinessContact:    r.BusinessContact,
	}
}

// ---------- validation helpers ----------

func validateVerifyRequest(req *verifyCreatorRequest) error {
	var fieldErrs []validators.FieldError

	switch req.Tier {
	case models.VerificationTierVerified,
		models.VerificationTierOfficial,
		models.VerificationTierCreator:
		// valid
	default:
		fieldErrs = append(fieldErrs, validators.FieldError{
			Field:   "tier",
			Tag:     "oneof",
			Message: "tier must be one of: verified, official, creator",
		})
	}

	if len(req.Reason) < 5 {
		fieldErrs = append(fieldErrs, validators.FieldError{
			Field:   "reason",
			Tag:     "min",
			Message: "reason must be at least 5 characters",
		})
	}
	if len(req.Reason) > 500 {
		fieldErrs = append(fieldErrs, validators.FieldError{
			Field:   "reason",
			Tag:     "max",
			Message: "reason must not exceed 500 characters",
		})
	}

	if len(fieldErrs) > 0 {
		return &validators.ValidationError{Fields: fieldErrs}
	}
	return nil
}

// ---------- error handling ----------

func (h *CreatorHandler) handleServiceError(c echo.Context, err error) error {
	if errors.Is(err, repositories.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "resource not found")
	}
	if errors.Is(err, services.ErrUnauthorized) {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}
	if errors.Is(err, services.ErrInvalidInput) {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if validators.IsValidationError(err) {
		return validationError(c, err)
	}
	h.logger.Error("unhandled creator service error", zap.Error(err))
	return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
}
