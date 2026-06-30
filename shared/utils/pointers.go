package utils

// Ptr returns a pointer to the value v.
// Useful for optional struct fields that require a *T.
func Ptr[T any](v T) *T { return &v }

// Deref dereferences p; returns def if p is nil.
func Deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// PtrString wraps a non-empty string in a pointer; returns nil for "".
func PtrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// DerefString safely dereferences a *string; returns "" for nil.
func DerefString(p *string) string { return Deref(p, "") }

// PtrInt wraps n in a pointer.
func PtrInt(n int) *int { return &n }

// DerefInt safely dereferences a *int; returns 0 for nil.
func DerefInt(p *int) int { return Deref(p, 0) }

// PtrInt64 wraps n in a pointer.
func PtrInt64(n int64) *int64 { return &n }

// DerefInt64 safely dereferences a *int64; returns 0 for nil.
func DerefInt64(p *int64) int64 { return Deref(p, 0) }

// PtrBool wraps b in a pointer.
func PtrBool(b bool) *bool { return &b }

// DerefBool safely dereferences a *bool; returns false for nil.
func DerefBool(p *bool) bool { return Deref(p, false) }

// PtrFloat64 wraps f in a pointer.
func PtrFloat64(f float64) *float64 { return &f }

// DerefFloat64 safely dereferences a *float64; returns 0 for nil.
func DerefFloat64(p *float64) float64 { return Deref(p, 0) }
