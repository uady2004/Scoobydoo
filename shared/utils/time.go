package utils

import (
	"fmt"
	"time"
)

// StartOfDay returns t truncated to midnight (00:00:00.000) in its location.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// EndOfDay returns 23:59:59.999999999 on the same date as t.
func EndOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

// StartOfWeek returns the Monday 00:00:00 of the ISO week containing t.
func StartOfWeek(t time.Time) time.Time {
	t = StartOfDay(t)
	offset := int(t.Weekday()) - int(time.Monday)
	if offset < 0 {
		offset += 7
	}
	return t.AddDate(0, 0, -offset)
}

// StartOfMonth returns the first day of the month of t at 00:00:00.
func StartOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

// FormatRelative returns a human-readable relative time string:
//
//	"just now", "2 minutes ago", "3 hours ago", "yesterday", "5 days ago"
func FormatRelative(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	case d < 30*24*time.Hour:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		return t.Format("Jan 2, 2006")
	}
}

// UnixMillis returns t as a Unix timestamp in milliseconds.
func UnixMillis(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

// FromUnixMillis converts a Unix millisecond timestamp to time.Time (UTC).
func FromUnixMillis(ms int64) time.Time {
	return time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond)).UTC()
}

// IsZeroOrNil reports whether t is the zero value.
func IsZeroTime(t time.Time) bool {
	return t.IsZero()
}

// CoalesceTime returns the first non-zero time in ts.
func CoalesceTime(ts ...time.Time) time.Time {
	for _, t := range ts {
		if !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

// DurationString formats d as a compact human-readable string:
// "2h30m", "45s", "1m5s".
func DurationString(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		if m > 0 {
			return fmt.Sprintf("%dh%dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	if m > 0 {
		if s > 0 {
			return fmt.Sprintf("%dm%ds", m, s)
		}
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", s)
}
