package utils

import (
	"time"
)

// NowUTC returns current time in UTC.
func NowUTC() time.Time { return time.Now().UTC() }

// NowRFC3339 returns current time in UTC formatted as RFC3339.
func NowRFC3339() string { return NowUTC().Format(time.RFC3339) }

// ParseRFC3339 parses an RFC3339 timestamp.
func ParseRFC3339(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) }

// MustParseDuration parses duration or returns default if invalid.
func MustParseDuration(s string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return def
}

// Ptr returns a pointer to the provided value (generic-like for Go <1.18 code familiarity; here typed overloads).
func Ptr[T any](v T) *T { return &v }

// ClampDuration clamps d between min and max.
func ClampDuration(d, min, max time.Duration) time.Duration {
	if d < min {
		return min
	}
	if d > max {
		return max
	}
	return d
}
