package validators

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"

	"github.com/tiktok-clone/user-service/internal/models"
)

// ValidationError wraps one or more field-level validation failures.
type ValidationError struct {
	Fields []FieldError `json:"fields"`
}

// FieldError describes a single field validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	msgs := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		msgs = append(msgs, fmt.Sprintf("%s: %s", f.Field, f.Message))
	}
	return strings.Join(msgs, "; ")
}

// UserValidator wraps go-playground/validator with custom rules for user-service entities.
type UserValidator struct {
	v *validator.Validate
}

// New returns a configured UserValidator instance.
func New() *UserValidator {
	v := validator.New()

	// Register custom validators.
	_ = v.RegisterValidation("username", validateUsername)
	_ = v.RegisterValidation("bio", validateBio)
	_ = v.RegisterValidation("safeurl", validateSafeURL)

	return &UserValidator{v: v}
}

// ValidateUpdateProfile validates the fields in an UpdateProfile payload.
// It returns a *ValidationError if any field fails, or nil if everything is valid.
func (uv *UserValidator) ValidateUpdateProfile(up *models.UpdateProfile) error {
	var fieldErrs []FieldError

	if up.DisplayName != nil {
		if errs := uv.validateDisplayName(*up.DisplayName); len(errs) > 0 {
			fieldErrs = append(fieldErrs, errs...)
		}
	}
	if up.Bio != nil {
		if errs := uv.validateBio(*up.Bio); len(errs) > 0 {
			fieldErrs = append(fieldErrs, errs...)
		}
	}
	if up.WebsiteURL != nil && *up.WebsiteURL != "" {
		if errs := uv.validateWebsiteURL(*up.WebsiteURL); len(errs) > 0 {
			fieldErrs = append(fieldErrs, errs...)
		}
	}
	if up.Location != nil {
		if errs := uv.validateLocation(*up.Location); len(errs) > 0 {
			fieldErrs = append(fieldErrs, errs...)
		}
	}

	if len(fieldErrs) > 0 {
		return &ValidationError{Fields: fieldErrs}
	}
	return nil
}

// ValidateCreatorProfile validates a creator profile update payload.
func (uv *UserValidator) ValidateCreatorProfile(cp *models.CreatorProfile) error {
	var fieldErrs []FieldError

	if cp.Category == "" {
		fieldErrs = append(fieldErrs, FieldError{
			Field:   "category",
			Tag:     "required",
			Message: "category must not be empty",
		})
	} else if len(cp.Category) > 50 {
		fieldErrs = append(fieldErrs, FieldError{
			Field:   "category",
			Tag:     "max",
			Message: "category must not exceed 50 characters",
		})
	}

	if len(cp.SubCategories) > 5 {
		fieldErrs = append(fieldErrs, FieldError{
			Field:   "sub_categories",
			Tag:     "max",
			Message: "at most 5 sub-categories are allowed",
		})
	}

	if cp.MinimumTipAmount < 0 {
		fieldErrs = append(fieldErrs, FieldError{
			Field:   "minimum_tip_amount",
			Tag:     "min",
			Message: "minimum tip amount must not be negative",
		})
	}
	if cp.MinimumTipAmount > 100_000 {
		fieldErrs = append(fieldErrs, FieldError{
			Field:   "minimum_tip_amount",
			Tag:     "max",
			Message: "minimum tip amount must not exceed 100,000",
		})
	}

	if len(cp.BusinessName) > 100 {
		fieldErrs = append(fieldErrs, FieldError{
			Field:   "business_name",
			Tag:     "max",
			Message: "business name must not exceed 100 characters",
		})
	}
	if len(cp.BusinessContact) > 200 {
		fieldErrs = append(fieldErrs, FieldError{
			Field:   "business_contact",
			Tag:     "max",
			Message: "business contact must not exceed 200 characters",
		})
	}

	if len(fieldErrs) > 0 {
		return &ValidationError{Fields: fieldErrs}
	}
	return nil
}

// ValidatePrivacySettings validates a PrivacySettings update request.
// The function checks that enum values are within the allowed set.
func (uv *UserValidator) ValidatePrivacySettings(ps *models.PrivacySettings) error {
	var fieldErrs []FieldError

	if err := validatePrivacyLevel(ps.ProfileVisibility); err != nil {
		fieldErrs = append(fieldErrs, FieldError{
			Field:   "profile_visibility",
			Tag:     "oneof",
			Message: err.Error(),
		})
	}
	if err := validatePrivacyLevel(ps.VideoVisibility); err != nil {
		fieldErrs = append(fieldErrs, FieldError{
			Field:   "video_visibility",
			Tag:     "oneof",
			Message: err.Error(),
		})
	}

	if len(fieldErrs) > 0 {
		return &ValidationError{Fields: fieldErrs}
	}
	return nil
}

// ValidateSearchQuery validates a user search query string.
func (uv *UserValidator) ValidateSearchQuery(q string) error {
	q = strings.TrimSpace(q)
	if q == "" {
		return &ValidationError{Fields: []FieldError{{
			Field:   "q",
			Tag:     "required",
			Message: "search query must not be empty",
		}}}
	}
	if utf8.RuneCountInString(q) < 1 {
		return &ValidationError{Fields: []FieldError{{
			Field:   "q",
			Tag:     "min",
			Message: "search query must be at least 1 character",
		}}}
	}
	if utf8.RuneCountInString(q) > 100 {
		return &ValidationError{Fields: []FieldError{{
			Field:   "q",
			Tag:     "max",
			Message: "search query must not exceed 100 characters",
		}}}
	}
	return nil
}

// ValidateAvatarContentType validates that the given MIME type is allowed for avatars.
func (uv *UserValidator) ValidateAvatarContentType(contentType string) error {
	allowed := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if !allowed[ct] {
		return &ValidationError{Fields: []FieldError{{
			Field:   "content_type",
			Tag:     "oneof",
			Message: fmt.Sprintf("unsupported avatar content type %q; allowed: image/jpeg, image/png, image/webp, image/gif", contentType),
		}}}
	}
	return nil
}

// IsValidationError returns true when err is a *ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// ToValidationError casts err to *ValidationError if possible, otherwise nil.
func ToValidationError(err error) *ValidationError {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve
	}
	return nil
}

// ---------- field-level validators ----------

var (
	displayNameRe = regexp.MustCompile(`^[\p{L}\p{N}\p{Z}_.\-']+$`)
	locationRe    = regexp.MustCompile(`^[\p{L}\p{N}\p{Z},.\-']+$`)
)

func (uv *UserValidator) validateDisplayName(name string) []FieldError {
	var errs []FieldError
	name = strings.TrimSpace(name)
	if name == "" {
		errs = append(errs, FieldError{Field: "display_name", Tag: "required", Message: "display name must not be blank"})
		return errs
	}
	if utf8.RuneCountInString(name) < 1 {
		errs = append(errs, FieldError{Field: "display_name", Tag: "min", Message: "display name must be at least 1 character"})
	}
	if utf8.RuneCountInString(name) > 50 {
		errs = append(errs, FieldError{Field: "display_name", Tag: "max", Message: "display name must not exceed 50 characters"})
	}
	if !displayNameRe.MatchString(name) {
		errs = append(errs, FieldError{Field: "display_name", Tag: "format", Message: "display name contains invalid characters"})
	}
	return errs
}

func (uv *UserValidator) validateBio(bio string) []FieldError {
	if utf8.RuneCountInString(bio) > 160 {
		return []FieldError{{Field: "bio", Tag: "max", Message: "bio must not exceed 160 characters"}}
	}
	return nil
}

func (uv *UserValidator) validateWebsiteURL(rawURL string) []FieldError {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return []FieldError{{Field: "website_url", Tag: "url", Message: "website_url is not a valid URL"}}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return []FieldError{{Field: "website_url", Tag: "url", Message: "website_url must use http or https"}}
	}
	if len(rawURL) > 200 {
		return []FieldError{{Field: "website_url", Tag: "max", Message: "website_url must not exceed 200 characters"}}
	}
	return nil
}

func (uv *UserValidator) validateLocation(location string) []FieldError {
	if utf8.RuneCountInString(location) > 100 {
		return []FieldError{{Field: "location", Tag: "max", Message: "location must not exceed 100 characters"}}
	}
	if location != "" && !locationRe.MatchString(location) {
		return []FieldError{{Field: "location", Tag: "format", Message: "location contains invalid characters"}}
	}
	return nil
}

// ---------- custom go-playground validator funcs ----------

func validateUsername(fl validator.FieldLevel) bool {
	username := fl.Field().String()
	if len(username) < 3 || len(username) > 30 {
		return false
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`)
	return re.MatchString(username)
}

func validateBio(fl validator.FieldLevel) bool {
	return utf8.RuneCountInString(fl.Field().String()) <= 160
}

func validateSafeURL(fl validator.FieldLevel) bool {
	raw := fl.Field().String()
	if raw == "" {
		return true
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func validatePrivacyLevel(level models.PrivacyLevel) error {
	switch level {
	case models.PrivacyPublic, models.PrivacyFollowers, models.PrivacyFriends, models.PrivacyPrivate:
		return nil
	default:
		return fmt.Errorf("invalid privacy level %q; must be one of: public, followers, friends, private", level)
	}
}
