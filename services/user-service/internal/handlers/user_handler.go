package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/tiktok-clone/user-service/internal/middleware"
	"github.com/tiktok-clone/user-service/internal/models"
	"github.com/tiktok-clone/user-service/internal/repositories"
	"github.com/tiktok-clone/user-service/internal/services"
	"github.com/tiktok-clone/user-service/internal/validators"
)

// UserHandler groups all REST handlers for user profile endpoints.
type UserHandler struct {
	svc       services.ProfileService
	validator *validators.UserValidator
	logger    *zap.Logger
}

// NewUserHandler creates a UserHandler and registers its routes on the given Echo group.
func NewUserHandler(g *echo.Group, svc services.ProfileService, val *validators.UserValidator, logger *zap.Logger) *UserHandler {
	h := &UserHandler{svc: svc, validator: val, logger: logger}

	// Public / optional-auth routes.
	g.GET("/users/:userID", h.GetProfile)
	g.GET("/users/by-username/:username", h.GetProfileByUsername)
	g.GET("/users/search", h.SearchUsers)

	// Authenticated routes.
	g.GET("/me", h.GetMyProfile)
	g.PATCH("/me", h.UpdateMyProfile)

	g.POST("/me/avatar/initiate", h.InitiateAvatarUpload)
	g.POST("/me/avatar/confirm", h.ConfirmAvatarUpload)
	g.POST("/me/avatar", h.UploadAvatar)

	g.GET("/me/privacy", h.GetPrivacySettings)
	g.PUT("/me/privacy", h.UpdatePrivacySettings)

	g.GET("/me/analytics", h.GetAccountAnalytics)

	g.GET("/me/followers/count", h.GetMyFollowerCount)
	g.GET("/me/following/count", h.GetMyFollowingCount)
	g.GET("/users/:userID/followers/count", h.GetFollowerCount)
	g.GET("/users/:userID/following/count", h.GetFollowingCount)

	g.POST("/me/blocks/:targetID", h.BlockUser)
	g.DELETE("/me/blocks/:targetID", h.UnblockUser)

	return h
}

// ---------- GetProfile ----------

// GetProfile godoc
// @Summary      Get a user profile by user ID
// @Description  Returns the public profile for the given user UUID.
// @Tags         profiles
// @Produce      json
// @Param        userID  path  string  true  "User UUID"
// @Success      200  {object}  models.PublicUserProfile
// @Failure      400  {object}  errorResponse
// @Failure      404  {object}  errorResponse
// @Router       /users/{userID} [get]
func (h *UserHandler) GetProfile(c echo.Context) error {
	userID, err := parseUUID(c.Param("userID"))
	if err != nil {
		return badRequest(c, "invalid user ID")
	}

	profile, err := h.svc.GetProfile(c.Request().Context(), userID)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(profile))
}

// ---------- GetProfileByUsername ----------

// GetProfileByUsername godoc
// @Summary      Get a user profile by username
// @Tags         profiles
// @Produce      json
// @Param        username  path  string  true  "Username"
// @Success      200  {object}  models.PublicUserProfile
// @Failure      404  {object}  errorResponse
// @Router       /users/by-username/{username} [get]
func (h *UserHandler) GetProfileByUsername(c echo.Context) error {
	username := c.Param("username")
	if username == "" {
		return badRequest(c, "username must not be empty")
	}

	profile, err := h.svc.GetProfileByUsername(c.Request().Context(), username)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(profile))
}

// ---------- GetMyProfile ----------

// GetMyProfile godoc
// @Summary      Get the authenticated user's own profile
// @Tags         profiles
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  models.PublicUserProfile
// @Failure      401  {object}  errorResponse
// @Router       /me [get]
func (h *UserHandler) GetMyProfile(c echo.Context) error {
	userID := middleware.MustGetUserID(c)
	profile, err := h.svc.GetProfile(c.Request().Context(), userID)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(profile))
}

// ---------- UpdateMyProfile ----------

// UpdateMyProfile godoc
// @Summary      Partially update the authenticated user's profile
// @Tags         profiles
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  models.UpdateProfile  true  "Profile update fields"
// @Success      200  {object}  models.PublicUserProfile
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Router       /me [patch]
func (h *UserHandler) UpdateMyProfile(c echo.Context) error {
	userID := middleware.MustGetUserID(c)

	var req models.UpdateProfile
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "malformed request body")
	}

	if err := h.validator.ValidateUpdateProfile(&req); err != nil {
		return validationError(c, err)
	}

	updated, err := h.svc.UpdateProfile(c.Request().Context(), userID, &req)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(updated))
}

// ---------- Avatar upload (presigned URL flow) ----------

// initiateAvatarUploadRequest is the request body for InitiateAvatarUpload.
type initiateAvatarUploadRequest struct {
	ContentType string `json:"content_type" validate:"required"`
}

// InitiateAvatarUpload godoc
// @Summary      Generate a presigned MinIO URL for avatar upload
// @Description  Returns a presigned PUT URL. The client must PUT the image file directly to that URL.
// @Tags         avatars
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  initiateAvatarUploadRequest  true  "MIME type of the avatar"
// @Success      200  {object}  models.AvatarUploadRequest
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Router       /me/avatar/initiate [post]
func (h *UserHandler) InitiateAvatarUpload(c echo.Context) error {
	userID := middleware.MustGetUserID(c)

	var req initiateAvatarUploadRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "malformed request body")
	}
	if err := h.validator.ValidateAvatarContentType(req.ContentType); err != nil {
		return validationError(c, err)
	}

	uploadReq, err := h.svc.InitiateAvatarUpload(c.Request().Context(), userID, req.ContentType)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(uploadReq))
}

// confirmAvatarUploadRequest carries the object key returned by MinIO after upload.
type confirmAvatarUploadRequest struct {
	ObjectKey string `json:"object_key" validate:"required"`
}

// ConfirmAvatarUpload godoc
// @Summary      Confirm that an avatar has been uploaded to MinIO
// @Description  Must be called after the client finishes the presigned PUT. Updates the profile avatar_url.
// @Tags         avatars
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  confirmAvatarUploadRequest  true  "Object key of the uploaded file"
// @Success      204  "No Content"
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Router       /me/avatar/confirm [post]
func (h *UserHandler) ConfirmAvatarUpload(c echo.Context) error {
	userID := middleware.MustGetUserID(c)

	var req confirmAvatarUploadRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "malformed request body")
	}
	if req.ObjectKey == "" {
		return badRequest(c, "object_key is required")
	}

	if err := h.svc.ConfirmAvatarUpload(c.Request().Context(), userID, req.ObjectKey); err != nil {
		return h.handleServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// UploadAvatar godoc
// @Summary      Server-proxied multipart avatar upload
// @Description  Accepts a multipart/form-data upload with field "avatar" and streams it to MinIO.
// @Tags         avatars
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        avatar  formData  file  true  "Avatar image file"
// @Success      200  {object}  map[string]string  "avatar_url"
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Router       /me/avatar [post]
func (h *UserHandler) UploadAvatar(c echo.Context) error {
	userID := middleware.MustGetUserID(c)

	fh, err := c.FormFile("avatar")
	if err != nil {
		return badRequest(c, "avatar file is required in form field 'avatar'")
	}

	avatarURL, err := h.svc.UploadAvatar(c.Request().Context(), userID, fh)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, successResponse(map[string]string{"avatar_url": avatarURL}))
}

// ---------- Privacy settings ----------

// GetPrivacySettings godoc
// @Summary      Get the authenticated user's privacy settings
// @Tags         privacy
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  models.PrivacySettings
// @Failure      401  {object}  errorResponse
// @Router       /me/privacy [get]
func (h *UserHandler) GetPrivacySettings(c echo.Context) error {
	userID := middleware.MustGetUserID(c)
	settings, err := h.svc.GetPrivacySettings(c.Request().Context(), userID)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(settings))
}

// UpdatePrivacySettings godoc
// @Summary      Replace the authenticated user's privacy settings
// @Tags         privacy
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  models.PrivacySettings  true  "Full privacy settings object"
// @Success      204  "No Content"
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Router       /me/privacy [put]
func (h *UserHandler) UpdatePrivacySettings(c echo.Context) error {
	userID := middleware.MustGetUserID(c)

	var req models.PrivacySettings
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "malformed request body")
	}

	if err := h.validator.ValidatePrivacySettings(&req); err != nil {
		return validationError(c, err)
	}

	if err := h.svc.UpdatePrivacySettings(c.Request().Context(), userID, &req); err != nil {
		return h.handleServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ---------- Analytics ----------

// GetAccountAnalytics godoc
// @Summary      Get account analytics for the authenticated user
// @Tags         analytics
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  models.AccountAnalytics
// @Failure      401  {object}  errorResponse
// @Router       /me/analytics [get]
func (h *UserHandler) GetAccountAnalytics(c echo.Context) error {
	userID := middleware.MustGetUserID(c)
	analytics, err := h.svc.GetAccountAnalytics(c.Request().Context(), userID)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(analytics))
}

// ---------- Social counters ----------

func (h *UserHandler) GetMyFollowerCount(c echo.Context) error {
	userID := middleware.MustGetUserID(c)
	return h.getFollowerCountForUser(c, userID)
}

func (h *UserHandler) GetMyFollowingCount(c echo.Context) error {
	userID := middleware.MustGetUserID(c)
	return h.getFollowingCountForUser(c, userID)
}

func (h *UserHandler) GetFollowerCount(c echo.Context) error {
	userID, err := parseUUID(c.Param("userID"))
	if err != nil {
		return badRequest(c, "invalid user ID")
	}
	return h.getFollowerCountForUser(c, userID)
}

func (h *UserHandler) GetFollowingCount(c echo.Context) error {
	userID, err := parseUUID(c.Param("userID"))
	if err != nil {
		return badRequest(c, "invalid user ID")
	}
	return h.getFollowingCountForUser(c, userID)
}

func (h *UserHandler) getFollowerCountForUser(c echo.Context, userID uuid.UUID) error {
	count, err := h.svc.GetFollowerCount(c.Request().Context(), userID)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(map[string]int64{"follower_count": count}))
}

func (h *UserHandler) getFollowingCountForUser(c echo.Context, userID uuid.UUID) error {
	count, err := h.svc.GetFollowingCount(c.Request().Context(), userID)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(map[string]int64{"following_count": count}))
}

// ---------- Search ----------

// SearchUsers godoc
// @Summary      Search for users by username or display name
// @Tags         search
// @Produce      json
// @Param        q         query  string  true   "Search query"
// @Param        page      query  int     false  "Page number (default 1)"
// @Param        page_size query  int     false  "Results per page (default 20, max 50)"
// @Success      200  {object}  models.SearchUsersResult
// @Failure      400  {object}  errorResponse
// @Router       /users/search [get]
func (h *UserHandler) SearchUsers(c echo.Context) error {
	q := c.QueryParam("q")
	if err := h.validator.ValidateSearchQuery(q); err != nil {
		return validationError(c, err)
	}

	page := queryParamInt(c, "page", 1)
	pageSize := queryParamInt(c, "page_size", 20)

	result, err := h.svc.SearchUsers(c.Request().Context(), q, page, pageSize)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, successResponse(result))
}

// ---------- Blocking ----------

// BlockUser godoc
// @Summary      Block another user
// @Tags         blocks
// @Security     BearerAuth
// @Param        targetID  path  string  true  "UUID of the user to block"
// @Success      204  "No Content"
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Router       /me/blocks/{targetID} [post]
func (h *UserHandler) BlockUser(c echo.Context) error {
	blockerID := middleware.MustGetUserID(c)
	blockedID, err := parseUUID(c.Param("targetID"))
	if err != nil {
		return badRequest(c, "invalid target user ID")
	}

	if err := h.svc.BlockUser(c.Request().Context(), blockerID, blockedID); err != nil {
		return h.handleServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// UnblockUser godoc
// @Summary      Unblock a previously blocked user
// @Tags         blocks
// @Security     BearerAuth
// @Param        targetID  path  string  true  "UUID of the user to unblock"
// @Success      204  "No Content"
// @Failure      400  {object}  errorResponse
// @Failure      401  {object}  errorResponse
// @Router       /me/blocks/{targetID} [delete]
func (h *UserHandler) UnblockUser(c echo.Context) error {
	blockerID := middleware.MustGetUserID(c)
	blockedID, err := parseUUID(c.Param("targetID"))
	if err != nil {
		return badRequest(c, "invalid target user ID")
	}

	if err := h.svc.UnblockUser(c.Request().Context(), blockerID, blockedID); err != nil {
		return h.handleServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ---------- error handling helpers ----------

func (h *UserHandler) handleServiceError(c echo.Context, err error) error {
	if errors.Is(err, repositories.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "resource not found")
	}
	if errors.Is(err, repositories.ErrDuplicateUsername) {
		return echo.NewHTTPError(http.StatusConflict, "username already taken")
	}
	if errors.Is(err, services.ErrUnauthorized) {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}
	if errors.Is(err, services.ErrInvalidInput) {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if errors.Is(err, services.ErrAvatarTooLarge) {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "avatar file exceeds the maximum allowed size")
	}
	if errors.Is(err, services.ErrUnsupportedAvatarType) {
		return echo.NewHTTPError(http.StatusUnsupportedMediaType, "unsupported avatar file type")
	}
	if validators.IsValidationError(err) {
		return validationError(c, err)
	}
	h.logger.Error("unhandled service error", zap.Error(err))
	return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
}

// ---------- response envelope helpers ----------

type apiResponse struct {
	Data interface{} `json:"data"`
}

type errorResponse struct {
	Error   string      `json:"error"`
	Details interface{} `json:"details,omitempty"`
}

func successResponse(data interface{}) *apiResponse {
	return &apiResponse{Data: data}
}

func badRequest(c echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, &errorResponse{Error: msg})
}

func validationError(c echo.Context, err error) error {
	ve := validators.ToValidationError(err)
	if ve != nil {
		return c.JSON(http.StatusUnprocessableEntity, &errorResponse{
			Error:   "validation failed",
			Details: ve.Fields,
		})
	}
	return c.JSON(http.StatusUnprocessableEntity, &errorResponse{Error: err.Error()})
}

// ---------- utility helpers ----------

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func queryParamInt(c echo.Context, key string, defaultVal int) int {
	v := c.QueryParam(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}
