package validators

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ── Regular expressions ───────────────────────────────────────────────────────

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	phoneRegex    = regexp.MustCompile(`^\+?[1-9]\d{7,14}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\.]{3,30}$`)
	otpCodeRegex  = regexp.MustCompile(`^\d{6}$`)
	totpRegex     = regexp.MustCompile(`^\d{6}$`)
)

// ── Request types ─────────────────────────────────────────────────────────────

// RegisterRequest is the JSON body for POST /auth/register.
type RegisterRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginRequest is the JSON body for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password" binding:"required"`
}

// RefreshTokenRequest is the JSON body for POST /auth/refresh.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest is the JSON body for POST /auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// GoogleOAuthRequest is the JSON body for POST /auth/oauth/google.
type GoogleOAuthRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// AppleOAuthRequest is the JSON body for POST /auth/oauth/apple.
type AppleOAuthRequest struct {
	IdentityToken string `json:"identity_token" binding:"required"`
	AuthCode      string `json:"authorization_code"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
}

// SendOTPRequest is the JSON body for POST /auth/otp/send.
type SendOTPRequest struct {
	Phone string `json:"phone"`
	Email string `json:"email"`
	Type  string `json:"type" binding:"required"`
}

// VerifyOTPRequest is the JSON body for POST /auth/otp/verify.
type VerifyOTPRequest struct {
	Code string `json:"code" binding:"required"`
	Type string `json:"type" binding:"required"`
}

// VerifyEmailRequest is the query param wrapper for GET /auth/email/verify.
type VerifyEmailRequest struct {
	Token string `form:"token" binding:"required"`
}

// SendPasswordResetRequest is the JSON body for POST /auth/password/reset.
type SendPasswordResetRequest struct {
	Email string `json:"email" binding:"required"`
}

// ResetPasswordRequest is the JSON body for PUT /auth/password/reset.
type ResetPasswordRequest struct {
	Token       string `json:"token"        binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePasswordRequest is the JSON body for PUT /auth/password/change.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// VerifyMFARequest is the JSON body for POST /auth/mfa/verify.
type VerifyMFARequest struct {
	Code string `json:"code" binding:"required"`
}

// DisableMFARequest is the JSON body for DELETE /auth/mfa.
type DisableMFARequest struct {
	Code string `json:"code" binding:"required"`
}

// ── Validation helpers ────────────────────────────────────────────────────────

// ValidationError aggregates one or more field-level errors.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	msgs := make([]string, 0, len(e.Fields))
	for field, msg := range e.Fields {
		msgs = append(msgs, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(msgs, "; ")
}

func newValidationError() *ValidationError {
	return &ValidationError{Fields: make(map[string]string)}
}

func (e *ValidationError) add(field, msg string) {
	e.Fields[field] = msg
}

func (e *ValidationError) hasErrors() bool {
	return len(e.Fields) > 0
}

// ValidateRegister validates a RegisterRequest and returns a ValidationError if invalid.
func ValidateRegister(r *RegisterRequest) error {
	ve := newValidationError()

	if r.Email == "" && r.Phone == "" {
		ve.add("email/phone", "at least one of email or phone is required")
	}
	if r.Email != "" && !emailRegex.MatchString(r.Email) {
		ve.add("email", "invalid email format")
	}
	if r.Phone != "" && !phoneRegex.MatchString(r.Phone) {
		ve.add("phone", "invalid phone format (use E.164, e.g. +12125551234)")
	}
	if !usernameRegex.MatchString(r.Username) {
		ve.add("username", "must be 3-30 characters: letters, numbers, underscores, dots")
	}
	if err := validatePassword(r.Password); err != nil {
		ve.add("password", err.Error())
	}

	if ve.hasErrors() {
		return ve
	}
	return nil
}

// ValidateLogin validates a LoginRequest.
func ValidateLogin(r *LoginRequest) error {
	ve := newValidationError()

	if r.Email == "" && r.Phone == "" {
		ve.add("email/phone", "at least one of email or phone is required")
	}
	if r.Email != "" && !emailRegex.MatchString(r.Email) {
		ve.add("email", "invalid email format")
	}
	if r.Phone != "" && !phoneRegex.MatchString(r.Phone) {
		ve.add("phone", "invalid phone format")
	}
	if r.Password == "" {
		ve.add("password", "password is required")
	}

	if ve.hasErrors() {
		return ve
	}
	return nil
}

// ValidateSendOTP validates a SendOTPRequest.
func ValidateSendOTP(r *SendOTPRequest) error {
	ve := newValidationError()

	if r.Phone == "" && r.Email == "" {
		ve.add("phone/email", "at least one of phone or email is required")
	}
	if r.Phone != "" && !phoneRegex.MatchString(r.Phone) {
		ve.add("phone", "invalid phone format")
	}
	if r.Email != "" && !emailRegex.MatchString(r.Email) {
		ve.add("email", "invalid email format")
	}
	if r.Type == "" {
		ve.add("type", "otp type is required")
	}

	if ve.hasErrors() {
		return ve
	}
	return nil
}

// ValidateVerifyOTP validates a VerifyOTPRequest.
func ValidateVerifyOTP(r *VerifyOTPRequest) error {
	ve := newValidationError()

	if !otpCodeRegex.MatchString(r.Code) {
		ve.add("code", "OTP must be 6 digits")
	}

	if ve.hasErrors() {
		return ve
	}
	return nil
}

// ValidateVerifyMFA validates a VerifyMFARequest TOTP code.
func ValidateVerifyMFA(r *VerifyMFARequest) error {
	ve := newValidationError()

	if !totpRegex.MatchString(r.Code) {
		ve.add("code", "TOTP code must be 6 digits")
	}

	if ve.hasErrors() {
		return ve
	}
	return nil
}

// ValidateResetPassword validates a ResetPasswordRequest.
func ValidateResetPassword(r *ResetPasswordRequest) error {
	ve := newValidationError()

	if r.Token == "" {
		ve.add("token", "token is required")
	}
	if err := validatePassword(r.NewPassword); err != nil {
		ve.add("new_password", err.Error())
	}

	if ve.hasErrors() {
		return ve
	}
	return nil
}

// ValidateChangePassword validates a ChangePasswordRequest.
func ValidateChangePassword(r *ChangePasswordRequest) error {
	ve := newValidationError()

	if r.OldPassword == "" {
		ve.add("old_password", "old password is required")
	}
	if err := validatePassword(r.NewPassword); err != nil {
		ve.add("new_password", err.Error())
	}

	if ve.hasErrors() {
		return ve
	}
	return nil
}

// validatePassword enforces a minimum password policy.
func validatePassword(pw string) error {
	if len(pw) < 8 {
		return fmt.Errorf("must be at least 8 characters")
	}
	if len(pw) > 128 {
		return fmt.Errorf("must not exceed 128 characters")
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("must contain at least one digit")
	}
	return nil
}
