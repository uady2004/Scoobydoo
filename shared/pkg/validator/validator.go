// Package validator wraps go-playground/validator/v10 with TikTok-clone
// specific custom rules and a translation layer that maps validation errors
// back to the shared errors.AppError type.
package validator

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/go-playground/validator/v10"

	apperrors "github.com/tiktok-clone/shared/pkg/errors"
)

// ---- Singleton --------------------------------------------------------------

var (
	instance *Validator
	once     sync.Once
)

// Get returns the package-level singleton Validator, initialising it on first
// call.
func Get() *Validator {
	once.Do(func() {
		v, err := New()
		if err != nil {
			panic(fmt.Sprintf("validator: failed to init singleton: %v", err))
		}
		instance = v
	})
	return instance
}

// ---- Validator type ---------------------------------------------------------

// Validator wraps validator.Validate and exposes Validate / ValidateStruct that
// return *apperrors.AppError.
type Validator struct {
	v *validator.Validate
}

// New creates and configures a new Validator.
func New() (*Validator, error) {
	v := validator.New()

	// Use JSON tag name (if present) as the field name in errors.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return fld.Name
		}
		return name
	})

	val := &Validator{v: v}
	if err := val.registerCustomRules(); err != nil {
		return nil, err
	}
	return val, nil
}

// ---- Public API -------------------------------------------------------------

// Validate validates any struct and returns an *apperrors.AppError of type
// VALIDATION_ERROR on failure, or nil on success.
func (val *Validator) Validate(s any) error {
	if err := val.v.Struct(s); err != nil {
		return convertErrors(err)
	}
	return nil
}

// ValidateVar validates a single variable against the supplied tag string.
func (val *Validator) ValidateVar(field any, tag string) error {
	if err := val.v.Var(field, tag); err != nil {
		return convertErrors(err)
	}
	return nil
}

// Engine exposes the underlying *validator.Validate for advanced usage.
func (val *Validator) Engine() *validator.Validate { return val.v }

// ---- Package-level helpers --------------------------------------------------

// Validate validates s using the package singleton.
func Validate(s any) error { return Get().Validate(s) }

// ValidateVar validates a single value using the package singleton.
func ValidateVar(field any, tag string) error { return Get().ValidateVar(field, tag) }

// ---- Custom rules -----------------------------------------------------------

// Common compiled regexes.
var (
	reUsername    = regexp.MustCompile(`^[a-zA-Z0-9_\.]{3,30}$`)
	rePassword    = regexp.MustCompile(`^.{8,128}$`)
	reE164        = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)
	reVideoTitle  = regexp.MustCompile(`^[\p{L}\p{N}\p{P}\p{Z}]{1,150}$`)
	reHashtag     = regexp.MustCompile(`^#[a-zA-Z0-9_\p{L}]{1,100}$`)
	reObjectKey   = regexp.MustCompile(`^[a-zA-Z0-9!_.*'()\-/]{1,1024}$`)
)

func (val *Validator) registerCustomRules() error {
	rules := []struct {
		tag string
		fn  validator.Func
	}{
		// username: 3-30 chars, alphanumeric, underscore, dot.
		{"username", func(fl validator.FieldLevel) bool {
			return reUsername.MatchString(fl.Field().String())
		}},

		// strong_password: 8-128 chars, at least one upper, one lower, one digit,
		// one special.
		{"strong_password", func(fl validator.FieldLevel) bool {
			p := fl.Field().String()
			if !rePassword.MatchString(p) {
				return false
			}
			var hasUpper, hasLower, hasDigit, hasSpecial bool
			for _, r := range p {
				switch {
				case unicode.IsUpper(r):
					hasUpper = true
				case unicode.IsLower(r):
					hasLower = true
				case unicode.IsDigit(r):
					hasDigit = true
				case unicode.IsPunct(r) || unicode.IsSymbol(r):
					hasSpecial = true
				}
			}
			return hasUpper && hasLower && hasDigit && hasSpecial
		}},

		// e164: international phone number in E.164 format.
		{"e164", func(fl validator.FieldLevel) bool {
			return reE164.MatchString(fl.Field().String())
		}},

		// video_title: 1-150 printable characters.
		{"video_title", func(fl validator.FieldLevel) bool {
			return reVideoTitle.MatchString(fl.Field().String())
		}},

		// hashtag: starts with #, 1-100 word chars.
		{"hashtag", func(fl validator.FieldLevel) bool {
			return reHashtag.MatchString(fl.Field().String())
		}},

		// object_key: valid S3/MinIO object key.
		{"object_key", func(fl validator.FieldLevel) bool {
			return reObjectKey.MatchString(fl.Field().String())
		}},

		// no_html: field must not contain any HTML tags.
		{"no_html", func(fl validator.FieldLevel) bool {
			s := fl.Field().String()
			return !strings.Contains(s, "<") && !strings.Contains(s, ">")
		}},

		// not_blank: field must not be all whitespace.
		{"not_blank", func(fl validator.FieldLevel) bool {
			return strings.TrimSpace(fl.Field().String()) != ""
		}},
	}

	for _, r := range rules {
		if err := val.v.RegisterValidation(r.tag, r.fn); err != nil {
			return fmt.Errorf("validator: registering rule %q: %w", r.tag, err)
		}
	}
	return nil
}

// ---- Error conversion -------------------------------------------------------

// convertErrors maps go-playground ValidationErrors to *apperrors.AppError.
func convertErrors(err error) *apperrors.AppError {
	var ve validator.ValidationErrors
	if !isValidationErrors(err, &ve) {
		// Unexpected error type; wrap as internal.
		return apperrors.NewInternal(err)
	}

	b := apperrors.NewValidationBuilder()
	for _, fe := range ve {
		b.Add(fe.Field(), humanise(fe), fe.Value())
	}
	return b.Build()
}

func isValidationErrors(err error, out *validator.ValidationErrors) bool {
	// Type-assert directly; errors.As does not work well with the
	// go-playground type because it is a slice alias.
	v, ok := err.(validator.ValidationErrors)
	if ok {
		*out = v
	}
	return ok
}

// humanise converts a validator.FieldError into a readable message.
func humanise(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "url":
		return "must be a valid URL"
	case "min":
		if fe.Type().Kind().String() == "string" {
			return fmt.Sprintf("must be at least %s characters", fe.Param())
		}
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		if fe.Type().Kind().String() == "string" {
			return fmt.Sprintf("must be at most %s characters", fe.Param())
		}
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fe.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", fe.Param())
	case "gt":
		return fmt.Sprintf("must be greater than %s", fe.Param())
	case "lt":
		return fmt.Sprintf("must be less than %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of [%s]", strings.ReplaceAll(fe.Param(), " ", ", "))
	case "uuid":
		return "must be a valid UUID"
	case "uuid4":
		return "must be a valid UUID v4"
	case "numeric":
		return "must contain only numeric characters"
	case "alpha":
		return "must contain only alphabetic characters"
	case "alphanum":
		return "must contain only alphanumeric characters"
	case "username":
		return "must be 3-30 characters, letters, digits, underscores, or dots"
	case "strong_password":
		return "must be 8-128 characters and include upper, lower, digit, and special characters"
	case "e164":
		return "must be a valid E.164 phone number (e.g. +15551234567)"
	case "video_title":
		return "must be 1-150 printable characters"
	case "hashtag":
		return "must start with # followed by 1-100 word characters"
	case "object_key":
		return "must be a valid storage object key"
	case "no_html":
		return "must not contain HTML tags"
	case "not_blank":
		return "must not be blank"
	default:
		return fmt.Sprintf("failed validation rule %q", fe.Tag())
	}
}
