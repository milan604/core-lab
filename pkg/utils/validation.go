package utils

import (
	"net/mail"
	"reflect"
)

// IsValidEmail validates email using net/mail.
func IsValidEmail(email string) bool {
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

// IsZero reports whether v is the zero value for its type.
func IsZero[T any](v T) bool {
	return reflect.ValueOf(&v).Elem().IsZero()
}

// RequireNonZero returns ok=false if v is zero.
func RequireNonZero[T any](v T) (T, bool) {
	if IsZero(v) {
		var zero T
		return zero, false
	}
	return v, true
}

// CoalesceVal returns the first non-zero value among values; otherwise returns the zero value.
func CoalesceVal[T comparable](values ...T) T {
	var zero T
	for _, v := range values {
		if v != zero {
			return v
		}
	}
	return zero
}

// Map applies fn to each element and returns a new slice.
func Map[T any, R any](in []T, fn func(T) R) []R {
	out := make([]R, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

// Filter returns elements for which fn returns true.
func Filter[T any](in []T, fn func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if fn(v) {
			out = append(out, v)
		}
	}
	return out
}

// Reduce reduces the slice to a single value using fn.
func Reduce[T any, R any](in []T, init R, fn func(R, T) R) R {
	acc := init
	for _, v := range in {
		acc = fn(acc, v)
	}
	return acc
}

// Contains reports whether target is present in in (for comparable types).
func Contains[T comparable](in []T, target T) bool {
	for _, v := range in {
		if v == target {
			return true
		}
	}
	return false
}

// Unique returns a slice with duplicate comparable values removed, preserving order.
func Unique[T comparable](in []T) []T {
	seen := make(map[T]struct{}, len(in))
	out := make([]T, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// MergeMaps merges b into a and returns a new map; does not mutate inputs.
func MergeMaps[K comparable, V any](a, b map[K]V) map[K]V {
	out := make(map[K]V, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
