package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/auth-service/internal/middleware"
	"github.com/tiktok-clone/auth-service/internal/models"
	"github.com/tiktok-clone/auth-service/internal/services"
	"github.com/tiktok-clone/auth-service/internal/validators"
)

// AuthHandler wires together the Gin router and the AuthService.
type AuthHandler struct {
	svc services.AuthService
	log *zap.Logger
}

// NewAuthHandler constructs an AuthHandler and registers all routes.
func NewAuthHandler(svc services.AuthService, log *zap.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log}
}

// RegisterRoutes attaches all auth endpoints to the given Gin router group.
// All routes sit under the supplied group (typically /api/v1/auth).
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/register", h.Register)
	rg.POST("/login", h.Login)
	rg.POST("/refresh", h.RefreshToken)
	rg.POST("/logout", h.Logout)
	rg.POST("/logout/all", h.LogoutAll)

	rg.POST("/oauth/google", h.GoogleOAuth)
	rg.POST("/oauth/apple", h.AppleOAuth)

	rg.POST("/otp/send", h.SendOTP)
	rg.POST("/otp/verify", h.VerifyOTP)

	rg.POST("/email/verify/send", h.SendEmailVerification)
	rg.GET("/email/verify", h.VerifyEmail)

	rg.POST("/password/reset/send", h.SendPasswordReset)
	rg.POST("/password/reset", h.ResetPassword)
	rg.POST("/password/change", h.ChangePassword)

	rg.POST("/mfa/setup", h.SetupMFA)
	rg.POST("/mfa/verify", h.VerifyMFA)
	rg.DELETE("/mfa", h.DisableMFA)

	rg.GET("/health", h.Health)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

type errorResponse struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

func (h *AuthHandler) respondError(c *gin.Context, status int, code, message string, details map[string]string) {
	reqID, _ := c.Get(middleware.ContextKeyRequestID)
	reqIDStr, _ := reqID.(string)
	c.JSON(status, errorResponse{
		Code:      code,
		Message:   message,
		RequestID: reqIDStr,
		Details:   details,
	})
}

func (h *AuthHandler) respondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

func (h *AuthHandler) respondCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, data)
}

// mapServiceError translates service sentinel errors into HTTP status + codes.
func (h *AuthHandler) mapServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrInvalidCredentials):
		h.respondError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email/phone or password", nil)
	case errors.Is(err, services.ErrUserNotFound):
		h.respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
	case errors.Is(err, services.ErrUserAlreadyExists):
		h.respondError(c, http.StatusConflict, "USER_EXISTS", "A user with this email or phone already exists", nil)
	case errors.Is(err, services.ErrInvalidToken):
		h.respondError(c, http.StatusUnauthorized, "INVALID_TOKEN", "Token is invalid or malformed", nil)
	case errors.Is(err, services.ErrTokenExpired):
		h.respondError(c, http.StatusUnauthorized, "TOKEN_EXPIRED", "Token has expired", nil)
	case errors.Is(err, services.ErrSessionRevoked):
		h.respondError(c, http.StatusUnauthorized, "SESSION_REVOKED", "Session has been revoked", nil)
	case errors.Is(err, services.ErrOTPInvalid):
		h.respondError(c, http.StatusUnauthorized, "OTP_INVALID", "OTP is invalid or has expired", nil)
	case errors.Is(err, services.ErrMFARequired):
		h.respondError(c, http.StatusAccepted, "MFA_REQUIRED", "MFA verification required", nil)
	case errors.Is(err, services.ErrMFAAlreadyEnabled):
		h.respondError(c, http.StatusConflict, "MFA_ALREADY_ENABLED", "MFA is already enabled", nil)
	case errors.Is(err, services.ErrInvalidMFACode):
		h.respondError(c, http.StatusUnauthorized, "MFA_CODE_INVALID", "Invalid MFA code", nil)
	case errors.Is(err, services.ErrAccountSuspended):
		h.respondError(c, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Account is suspended", nil)
	default:
		h.log.Error("internal error", zap.Error(err))
		h.respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}

// extractUserID pulls the authenticated user's UUID from the Gin context.
// Handlers that require authentication must call this after the JWT middleware has run.
func extractUserID(c *gin.Context) (uuid.UUID, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, false
	}
	switch v := raw.(type) {
	case uuid.UUID:
		return v, true
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return uuid.Nil, false
		}
		return id, true
	}
	return uuid.Nil, false
}

// clientMeta extracts User-Agent, IP address and optional Device-ID from the request.
func clientMeta(c *gin.Context) (userAgent, ipAddress, deviceID string) {
	userAgent = c.GetHeader("User-Agent")
	ipAddress = c.ClientIP()
	deviceID = c.GetHeader("X-Device-ID")
	return
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Register godoc
// POST /auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var body validators.RegisterRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validators.ValidateRegister(&body); err != nil {
		var ve *validators.ValidationError
		if errors.As(err, &ve) {
			h.respondError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", ve.Fields)
			return
		}
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	ua, ip, did := clientMeta(c)
	user, tokens, err := h.svc.Register(c.Request.Context(), services.RegisterRequest{
		Email:     body.Email,
		Phone:     body.Phone,
		Username:  body.Username,
		Password:  body.Password,
		UserAgent: ua,
		IPAddress: ip,
		DeviceID:  did,
	})
	if err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondCreated(c, gin.H{
		"user":   sanitizeUser(user),
		"tokens": tokens,
	})
}

// Login godoc
// POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var body validators.LoginRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validators.ValidateLogin(&body); err != nil {
		var ve *validators.ValidationError
		if errors.As(err, &ve) {
			h.respondError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", ve.Fields)
			return
		}
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	ua, ip, did := clientMeta(c)
	user, tokens, err := h.svc.Login(c.Request.Context(), services.LoginRequest{
		Email:     body.Email,
		Phone:     body.Phone,
		Password:  body.Password,
		UserAgent: ua,
		IPAddress: ip,
		DeviceID:  did,
	})
	if err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{
		"user":   sanitizeUser(user),
		"tokens": tokens,
	})
}

// RefreshToken godoc
// POST /auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var body validators.RefreshTokenRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	tokens, err := h.svc.RefreshToken(c.Request.Context(), body.RefreshToken)
	if err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{"tokens": tokens})
}

// Logout godoc
// POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	var body validators.LogoutRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	if err := h.svc.Logout(c.Request.Context(), body.RefreshToken); err != nil {
		h.mapServiceError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// LogoutAll godoc
// POST /auth/logout/all  — requires authenticated user context (JWT middleware)
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		h.respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	if err := h.svc.LogoutAll(c.Request.Context(), userID); err != nil {
		h.mapServiceError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// GoogleOAuth godoc
// POST /auth/oauth/google
func (h *AuthHandler) GoogleOAuth(c *gin.Context) {
	var body validators.GoogleOAuthRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	ua, ip, _ := clientMeta(c)
	user, tokens, err := h.svc.GoogleOAuth(c.Request.Context(), services.GoogleOAuthRequest{
		IDToken:   body.IDToken,
		UserAgent: ua,
		IPAddress: ip,
	})
	if err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{
		"user":   sanitizeUser(user),
		"tokens": tokens,
	})
}

// AppleOAuth godoc
// POST /auth/oauth/apple
func (h *AuthHandler) AppleOAuth(c *gin.Context) {
	var body validators.AppleOAuthRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	ua, ip, _ := clientMeta(c)
	user, tokens, err := h.svc.AppleOAuth(c.Request.Context(), services.AppleOAuthRequest{
		IdentityToken: body.IdentityToken,
		AuthCode:      body.AuthCode,
		GivenName:     body.GivenName,
		FamilyName:    body.FamilyName,
		UserAgent:     ua,
		IPAddress:     ip,
	})
	if err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{
		"user":   sanitizeUser(user),
		"tokens": tokens,
	})
}

// SendOTP godoc
// POST /auth/otp/send
func (h *AuthHandler) SendOTP(c *gin.Context) {
	var body validators.SendOTPRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validators.ValidateSendOTP(&body); err != nil {
		var ve *validators.ValidationError
		if errors.As(err, &ve) {
			h.respondError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", ve.Fields)
			return
		}
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	if err := h.svc.SendOTP(c.Request.Context(), services.SendOTPRequest{
		Phone: body.Phone,
		Email: body.Email,
		Type:  models.OTPType(body.Type),
	}); err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{"message": "OTP sent"})
}

// VerifyOTP godoc
// POST /auth/otp/verify  — requires authenticated user context
func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		h.respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var body validators.VerifyOTPRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validators.ValidateVerifyOTP(&body); err != nil {
		var ve *validators.ValidationError
		if errors.As(err, &ve) {
			h.respondError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", ve.Fields)
			return
		}
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	tokens, err := h.svc.VerifyOTP(c.Request.Context(), services.VerifyOTPRequest{
		UserID: userID,
		Code:   body.Code,
		Type:   models.OTPType(body.Type),
	})
	if err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{"tokens": tokens})
}

// SendEmailVerification godoc
// POST /auth/email/verify/send  — requires authenticated user context
func (h *AuthHandler) SendEmailVerification(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		h.respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	if err := h.svc.SendEmailVerification(c.Request.Context(), userID); err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{"message": "Verification email sent"})
}

// VerifyEmail godoc
// GET /auth/email/verify?token=...
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var q validators.VerifyEmailRequest
	if err := c.ShouldBindQuery(&q); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	if err := h.svc.VerifyEmail(c.Request.Context(), q.Token); err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{"message": "Email verified successfully"})
}

// SendPasswordReset godoc
// POST /auth/password/reset/send
func (h *AuthHandler) SendPasswordReset(c *gin.Context) {
	var body validators.SendPasswordResetRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	// Always respond 200 to prevent user enumeration.
	_ = h.svc.SendPasswordReset(c.Request.Context(), body.Email)
	h.respondOK(c, gin.H{"message": "If that email is registered you will receive a reset link"})
}

// ResetPassword godoc
// POST /auth/password/reset
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var body validators.ResetPasswordRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validators.ValidateResetPassword(&body); err != nil {
		var ve *validators.ValidationError
		if errors.As(err, &ve) {
			h.respondError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", ve.Fields)
			return
		}
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	if err := h.svc.ResetPassword(c.Request.Context(), services.ResetPasswordRequest{
		Token:       body.Token,
		NewPassword: body.NewPassword,
	}); err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{"message": "Password reset successfully"})
}

// ChangePassword godoc
// POST /auth/password/change  — requires authenticated user context
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		h.respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var body validators.ChangePasswordRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validators.ValidateChangePassword(&body); err != nil {
		var ve *validators.ValidationError
		if errors.As(err, &ve) {
			h.respondError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", ve.Fields)
			return
		}
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	if err := h.svc.ChangePassword(c.Request.Context(), services.ChangePasswordRequest{
		UserID:      userID,
		OldPassword: body.OldPassword,
		NewPassword: body.NewPassword,
	}); err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{"message": "Password changed successfully"})
}

// SetupMFA godoc
// POST /auth/mfa/setup  — requires authenticated user context
func (h *AuthHandler) SetupMFA(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		h.respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	resp, err := h.svc.EnableMFA(c.Request.Context(), userID)
	if err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, resp)
}

// VerifyMFA godoc
// POST /auth/mfa/verify  — requires authenticated user context
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		h.respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var body validators.VerifyMFARequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validators.ValidateVerifyMFA(&body); err != nil {
		var ve *validators.ValidationError
		if errors.As(err, &ve) {
			h.respondError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", ve.Fields)
			return
		}
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	ua, ip, _ := clientMeta(c)
	tokens, err := h.svc.VerifyMFA(c.Request.Context(), services.VerifyMFARequest{
		UserID:    userID,
		Code:      body.Code,
		UserAgent: ua,
		IPAddress: ip,
	})
	if err != nil {
		h.mapServiceError(c, err)
		return
	}

	h.respondOK(c, gin.H{"tokens": tokens})
}

// DisableMFA godoc
// DELETE /auth/mfa  — requires authenticated user context
func (h *AuthHandler) DisableMFA(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		h.respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var body validators.DisableMFARequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	if err := h.svc.DisableMFA(c.Request.Context(), userID, body.Code); err != nil {
		h.mapServiceError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// Health godoc
// GET /auth/health
func (h *AuthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ── Private utilities ─────────────────────────────────────────────────────────

// sanitizeUser strips sensitive fields before returning a user to the caller.
func sanitizeUser(u *models.User) gin.H {
	if u == nil {
		return nil
	}
	return gin.H{
		"id":             u.ID,
		"email":          u.Email,
		"phone":          u.Phone,
		"username":       u.Username,
		"provider":       u.Provider,
		"email_verified": u.EmailVerified,
		"phone_verified": u.PhoneVerified,
		"mfa_enabled":    u.MFAEnabled,
		"status":         u.Status,
		"display_name":   u.DisplayName,
		"avatar_url":     u.AvatarURL,
		"created_at":     u.CreatedAt,
	}
}
