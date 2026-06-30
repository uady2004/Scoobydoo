// Package errors defines domain error types for the TikTok-clone platform.
// Every error carries an HTTP status code, a gRPC status code, a machine-
// readable code string, and an optional list of field-level validation details.
//
// Usage:
//
//	if user == nil {
//	    return errors.NewNotFound("user", id)
//	}
package errors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- Error codes ----------------------------------------------------------

// Code is a machine-readable identifier for the error class.
type Code string

const (
	CodeNotFound       Code = "NOT_FOUND"
	CodeUnauthorized   Code = "UNAUTHORIZED"
	CodeForbidden      Code = "FORBIDDEN"
	CodeValidation     Code = "VALIDATION_ERROR"
	CodeConflict       Code = "CONFLICT"
	CodeRateLimit      Code = "RATE_LIMIT_EXCEEDED"
	CodeInternal       Code = "INTERNAL_ERROR"
	CodeUnavailable    Code = "SERVICE_UNAVAILABLE"
	CodeBadRequest     Code = "BAD_REQUEST"
	CodeUnimplemented  Code = "UNIMPLEMENTED"
	CodeTimeout        Code = "TIMEOUT"
)

// FieldError represents a single field-level validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

func (f FieldError) String() string {
	return fmt.Sprintf("field %q: %s", f.Field, f.Message)
}

// ---- AppError ---------------------------------------------------------------

// AppError is the canonical error type returned by every service layer. It
// implements the standard error interface and can be converted to gRPC or HTTP
// responses via the helpers below.
type AppError struct {
	// Code is the machine-readable error class.
	Code Code `json:"code"`
	// Message is a human-readable description safe to expose to API consumers.
	Message string `json:"message"`
	// Details carries field-level validation errors when Code == CodeValidation.
	Details []FieldError `json:"details,omitempty"`
	// cause is the original (internal) error; never serialised.
	cause error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap allows errors.Is / errors.As to traverse the chain.
func (e *AppError) Unwrap() error { return e.cause }

// Is reports whether the target error has the same Code as e, enabling
// errors.Is(err, ErrNotFound) style checks.
func (e *AppError) Is(target error) bool {
	var t *AppError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// WithCause attaches an internal (non-public) cause to the error and returns e
// to support fluent chaining.
func (e *AppError) WithCause(err error) *AppError {
	e.cause = err
	return e
}

// WithDetail appends a field-level error detail and returns e.
func (e *AppError) WithDetail(field, message string) *AppError {
	e.Details = append(e.Details, FieldError{Field: field, Message: message})
	return e
}

// WithDetails replaces any existing field errors and returns e.
func (e *AppError) WithDetails(details []FieldError) *AppError {
	e.Details = details
	return e
}

// ---- Sentinel errors for use with errors.Is --------------------------------

var (
	ErrNotFound      = &AppError{Code: CodeNotFound}
	ErrUnauthorized  = &AppError{Code: CodeUnauthorized}
	ErrForbidden     = &AppError{Code: CodeForbidden}
	ErrValidation    = &AppError{Code: CodeValidation}
	ErrConflict      = &AppError{Code: CodeConflict}
	ErrRateLimit     = &AppError{Code: CodeRateLimit}
	ErrInternal      = &AppError{Code: CodeInternal}
	ErrUnavailable   = &AppError{Code: CodeUnavailable}
	ErrUnimplemented = &AppError{Code: CodeUnimplemented}
	ErrTimeout       = &AppError{Code: CodeTimeout}
)

// ---- Constructors -----------------------------------------------------------

// NewNotFound returns a NOT_FOUND error for the given resource type and id.
func NewNotFound(resource, id string) *AppError {
	return &AppError{
		Code:    CodeNotFound,
		Message: fmt.Sprintf("%s with id %q was not found", resource, id),
	}
}

// NewNotFoundMsg returns a NOT_FOUND error with a custom message.
func NewNotFoundMsg(msg string) *AppError {
	return &AppError{Code: CodeNotFound, Message: msg}
}

// NewUnauthorized returns an UNAUTHORIZED error.
func NewUnauthorized(msg string) *AppError {
	if msg == "" {
		msg = "authentication is required"
	}
	return &AppError{Code: CodeUnauthorized, Message: msg}
}

// NewForbidden returns a FORBIDDEN error.
func NewForbidden(msg string) *AppError {
	if msg == "" {
		msg = "you do not have permission to perform this action"
	}
	return &AppError{Code: CodeForbidden, Message: msg}
}

// NewValidation returns a VALIDATION_ERROR with optional field details.
func NewValidation(msg string, fields ...FieldError) *AppError {
	if msg == "" {
		msg = "request validation failed"
	}
	return &AppError{Code: CodeValidation, Message: msg, Details: fields}
}

// NewConflict returns a CONFLICT error (e.g. duplicate key).
func NewConflict(resource, field, value string) *AppError {
	return &AppError{
		Code:    CodeConflict,
		Message: fmt.Sprintf("%s with %s=%q already exists", resource, field, value),
	}
}

// NewRateLimit returns a RATE_LIMIT_EXCEEDED error.
func NewRateLimit(msg string) *AppError {
	if msg == "" {
		msg = "rate limit exceeded, please slow down"
	}
	return &AppError{Code: CodeRateLimit, Message: msg}
}

// NewInternal wraps an internal error that should NOT be surfaced to clients.
func NewInternal(cause error) *AppError {
	return &AppError{
		Code:    CodeInternal,
		Message: "an internal error occurred",
		cause:   cause,
	}
}

// NewInternalMsg returns an INTERNAL error with a custom (but still safe) message.
func NewInternalMsg(msg string, cause error) *AppError {
	return &AppError{Code: CodeInternal, Message: msg, cause: cause}
}

// NewUnavailable returns a SERVICE_UNAVAILABLE error.
func NewUnavailable(msg string) *AppError {
	if msg == "" {
		msg = "service is temporarily unavailable"
	}
	return &AppError{Code: CodeUnavailable, Message: msg}
}

// NewBadRequest returns a BAD_REQUEST error.
func NewBadRequest(msg string) *AppError {
	return &AppError{Code: CodeBadRequest, Message: msg}
}

// NewTimeout returns a TIMEOUT error.
func NewTimeout(msg string) *AppError {
	if msg == "" {
		msg = "the request timed out"
	}
	return &AppError{Code: CodeTimeout, Message: msg}
}

// ---- Type-assertion helpers -------------------------------------------------

// AsAppError extracts the *AppError from err (if present).
// Returns nil if err is not an *AppError.
func AsAppError(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}

// IsNotFound reports whether err is a NOT_FOUND AppError.
func IsNotFound(err error) bool { return hasCode(err, CodeNotFound) }

// IsUnauthorized reports whether err is an UNAUTHORIZED AppError.
func IsUnauthorized(err error) bool { return hasCode(err, CodeUnauthorized) }

// IsForbidden reports whether err is a FORBIDDEN AppError.
func IsForbidden(err error) bool { return hasCode(err, CodeForbidden) }

// IsValidation reports whether err is a VALIDATION_ERROR AppError.
func IsValidation(err error) bool { return hasCode(err, CodeValidation) }

// IsConflict reports whether err is a CONFLICT AppError.
func IsConflict(err error) bool { return hasCode(err, CodeConflict) }

// IsInternal reports whether err is an INTERNAL AppError.
func IsInternal(err error) bool { return hasCode(err, CodeInternal) }

func hasCode(err error, code Code) bool {
	ae := AsAppError(err)
	return ae != nil && ae.Code == code
}

// ---- HTTP mapping -----------------------------------------------------------

// HTTPStatus returns the HTTP status code corresponding to the AppError's Code.
// Falls back to 500 for unknown codes.
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeValidation, CodeBadRequest:
		return http.StatusBadRequest
	case CodeConflict:
		return http.StatusConflict
	case CodeRateLimit:
		return http.StatusTooManyRequests
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeUnimplemented:
		return http.StatusNotImplemented
	case CodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// HTTPStatusFromErr returns the HTTP status code for err.
// If err is not an *AppError it returns 500.
func HTTPStatusFromErr(err error) int {
	ae := AsAppError(err)
	if ae == nil {
		return http.StatusInternalServerError
	}
	return ae.HTTPStatus()
}

// ---- gRPC mapping -----------------------------------------------------------

// GRPCStatus converts an AppError to a gRPC *status.Status.
func (e *AppError) GRPCStatus() *status.Status {
	return status.New(e.grpcCode(), e.Message)
}

func (e *AppError) grpcCode() codes.Code {
	switch e.Code {
	case CodeNotFound:
		return codes.NotFound
	case CodeUnauthorized:
		return codes.Unauthenticated
	case CodeForbidden:
		return codes.PermissionDenied
	case CodeValidation, CodeBadRequest:
		return codes.InvalidArgument
	case CodeConflict:
		return codes.AlreadyExists
	case CodeRateLimit:
		return codes.ResourceExhausted
	case CodeUnavailable:
		return codes.Unavailable
	case CodeUnimplemented:
		return codes.Unimplemented
	case CodeTimeout:
		return codes.DeadlineExceeded
	default:
		return codes.Internal
	}
}

// GRPCStatusFromErr converts any error to a gRPC status.
// Non-AppErrors are wrapped as codes.Internal.
func GRPCStatusFromErr(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}
	ae := AsAppError(err)
	if ae != nil {
		return ae.GRPCStatus()
	}
	return status.New(codes.Internal, "internal error")
}

// FromGRPCStatus converts a gRPC status back to an AppError.
func FromGRPCStatus(s *status.Status) *AppError {
	if s.Code() == codes.OK {
		return nil
	}
	codeMap := map[codes.Code]Code{
		codes.NotFound:          CodeNotFound,
		codes.Unauthenticated:   CodeUnauthorized,
		codes.PermissionDenied:  CodeForbidden,
		codes.InvalidArgument:   CodeValidation,
		codes.AlreadyExists:     CodeConflict,
		codes.ResourceExhausted: CodeRateLimit,
		codes.Unavailable:       CodeUnavailable,
		codes.Unimplemented:     CodeUnimplemented,
		codes.DeadlineExceeded:  CodeTimeout,
	}
	code, ok := codeMap[s.Code()]
	if !ok {
		code = CodeInternal
	}
	return &AppError{Code: code, Message: s.Message()}
}

// ---- Wrap -------------------------------------------------------------------

// Wrap adds context to an error. If err is already an *AppError the message is
// prepended to its Message. Otherwise a new INTERNAL AppError is created.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	ae := AsAppError(err)
	if ae != nil {
		return &AppError{
			Code:    ae.Code,
			Message: msg + ": " + ae.Message,
			Details: ae.Details,
			cause:   ae.cause,
		}
	}
	return NewInternal(fmt.Errorf("%s: %w", msg, err))
}

// Wrapf is like Wrap but with a formatted message.
func Wrapf(err error, format string, args ...any) error {
	return Wrap(err, fmt.Sprintf(format, args...))
}

// ---- Validation builder -----------------------------------------------------

// ValidationBuilder accumulates field errors and produces a single
// VALIDATION_ERROR AppError.
type ValidationBuilder struct {
	fields []FieldError
}

// NewValidationBuilder returns a fresh ValidationBuilder.
func NewValidationBuilder() *ValidationBuilder { return &ValidationBuilder{} }

// Add records a field error.
func (b *ValidationBuilder) Add(field, msg string, value ...any) *ValidationBuilder {
	fe := FieldError{Field: field, Message: msg}
	if len(value) > 0 {
		fe.Value = value[0]
	}
	b.fields = append(b.fields, fe)
	return b
}

// HasErrors reports whether any field errors have been accumulated.
func (b *ValidationBuilder) HasErrors() bool { return len(b.fields) > 0 }

// Build returns a *AppError if there are accumulated errors, otherwise nil.
func (b *ValidationBuilder) Build() *AppError {
	if !b.HasErrors() {
		return nil
	}
	msgs := make([]string, len(b.fields))
	for i, f := range b.fields {
		msgs[i] = f.String()
	}
	return NewValidation(strings.Join(msgs, "; "), b.fields...)
}
