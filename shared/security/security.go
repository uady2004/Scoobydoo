// Package security provides input sanitisation, password strength validation,
// and request-level security helpers for the TikTok-clone platform.
package security

import (
	"errors"
	"html"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ---- Input sanitisation -------------------------------------------------------

// SanitizeHTML escapes HTML special characters in s, neutralising XSS payloads.
func SanitizeHTML(s string) string {
	return html.EscapeString(s)
}

// StripControlChars removes ASCII control characters (except tab, LF, CR).
func StripControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		if r == 127 { // DEL
			return -1
		}
		return r
	}, s)
}

// TruncateRunes truncates s to at most maxRunes Unicode code points.
func TruncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	return string([]rune(s)[:maxRunes])
}

// NormalizeWhitespace collapses all whitespace runs to a single space and trims
// leading/trailing whitespace.
func NormalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// SanitizeUsername keeps only alphanumerics, underscores, dots, and hyphens.
var usernameRe = regexp.MustCompile(`[^\w.\-]`)

func SanitizeUsername(s string) string {
	return usernameRe.ReplaceAllString(s, "")
}

// ---- Password strength --------------------------------------------------------

// PasswordRequirements holds the policy for password validation.
type PasswordRequirements struct {
	MinLength      int
	MaxLength      int
	RequireUpper   bool
	RequireLower   bool
	RequireDigit   bool
	RequireSpecial bool
}

// DefaultPasswordRequirements is the platform-wide password policy.
var DefaultPasswordRequirements = PasswordRequirements{
	MinLength:      8,
	MaxLength:      72, // bcrypt limit
	RequireUpper:   true,
	RequireLower:   true,
	RequireDigit:   true,
	RequireSpecial: false,
}

// ValidatePassword checks p against the given requirements and returns an error
// describing the first violation.
func ValidatePassword(p string, req PasswordRequirements) error {
	n := utf8.RuneCountInString(p)
	if n < req.MinLength {
		return errors.New("password must be at least " + itoa(req.MinLength) + " characters")
	}
	if req.MaxLength > 0 && n > req.MaxLength {
		return errors.New("password must not exceed " + itoa(req.MaxLength) + " characters")
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

	if req.RequireUpper && !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if req.RequireLower && !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if req.RequireDigit && !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if req.RequireSpecial && !hasSpecial {
		return errors.New("password must contain at least one special character")
	}
	return nil
}

// ---- Email validation ---------------------------------------------------------

// ValidateEmail returns nil if email is a syntactically valid RFC 5322 address.
func ValidateEmail(email string) error {
	if email == "" {
		return errors.New("email must not be empty")
	}
	if utf8.RuneCountInString(email) > 254 {
		return errors.New("email must not exceed 254 characters")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("email is not a valid address")
	}
	return nil
}

// NormalizeEmail lower-cases the local part and domain, trims whitespace.
func NormalizeEmail(email string) string {
	email = strings.TrimSpace(email)
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return strings.ToLower(email)
	}
	return strings.ToLower(parts[0]) + "@" + strings.ToLower(parts[1])
}

// ---- Phone validation ---------------------------------------------------------

var phoneRe = regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)

// ValidatePhone checks that phone is an E.164-like number (7-15 digits, optional +).
func ValidatePhone(phone string) error {
	stripped := regexp.MustCompile(`[\s\-\(\)]`).ReplaceAllString(phone, "")
	if !phoneRe.MatchString(stripped) {
		return errors.New("phone number is invalid; expected E.164 format, e.g. +12125551234")
	}
	return nil
}

// ---- Content policy ----------------------------------------------------------

var (
	// sqlInjectionRe catches the most common SQL injection patterns.
	sqlInjectionRe = regexp.MustCompile(`(?i)(--|;|/\*|\*/|xp_|exec\s*\(|UNION\s+SELECT|DROP\s+TABLE|INSERT\s+INTO|DELETE\s+FROM)`)
)

// ContainsSQLInjection returns true if s contains common SQL injection markers.
// This is a lightweight heuristic; use parameterised queries as the primary defence.
func ContainsSQLInjection(s string) bool {
	return sqlInjectionRe.MatchString(s)
}

// IsSafeRedirectURL returns true if u is a relative URL or matches the given
// allowedHost, preventing open redirect attacks.
func IsSafeRedirectURL(u, allowedHost string) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	// Relative URLs are always safe.
	if strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//") {
		return true
	}
	// Must be http/https and match the allowed host.
	for _, scheme := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(u), scheme) {
			rest := u[len(scheme):]
			host := strings.SplitN(rest, "/", 2)[0]
			return strings.EqualFold(host, allowedHost)
		}
	}
	return false
}

// ---- Helper ------------------------------------------------------------------

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
