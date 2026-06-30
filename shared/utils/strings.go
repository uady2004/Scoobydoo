// Package utils provides general-purpose helper functions used across the
// TikTok-clone microservices.
package utils

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ---- String helpers ----------------------------------------------------------

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts s into a URL-safe lowercase slug:
//
//	"Hello World!" → "hello-world"
func Slugify(s string) string {
	s = strings.ToLower(s)
	// Map non-ASCII letters to their ASCII equivalents where possible.
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	slug := slugRe.ReplaceAllString(b.String(), "-")
	return strings.Trim(slug, "-")
}

// Truncate returns s truncated to at most maxRunes Unicode code points.
// If truncation occurs, suffix is appended (pass "" for none).
func Truncate(s, suffix string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	cut := maxRunes
	if suffix != "" {
		cut = maxRunes - utf8.RuneCountInString(suffix)
		if cut < 0 {
			cut = 0
		}
	}
	return string(runes[:cut]) + suffix
}

// MaskEmail obscures an email address: "user@example.com" → "us**@example.com".
func MaskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "****"
	}
	local := []rune(parts[0])
	if len(local) <= 2 {
		return strings.Repeat("*", len(local)) + "@" + parts[1]
	}
	masked := string(local[:2]) + strings.Repeat("*", len(local)-2)
	return masked + "@" + parts[1]
}

// MaskPhone obscures a phone number, showing only the last 4 digits:
//
//	"+12125551234" → "****1234"
func MaskPhone(phone string) string {
	r := []rune(phone)
	if len(r) <= 4 {
		return strings.Repeat("*", len(r))
	}
	return strings.Repeat("*", len(r)-4) + string(r[len(r)-4:])
}

// ContainsAny reports whether s contains any of the substrings.
func ContainsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// CoalesceString returns the first non-empty string in vals.
func CoalesceString(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// PadLeft pads s on the left with padChar until len(s) >= width.
func PadLeft(s, padChar string, width int) string {
	for utf8.RuneCountInString(s) < width {
		s = padChar + s
	}
	return s
}
