package utils

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ToUpper returns s in upper case.
func ToUpper(s string) string { return strings.ToUpper(s) }

// ToLower returns s in lower case.
func ToLower(s string) string { return strings.ToLower(s) }

// TrimSpace trims leading and trailing white space.
func TrimSpace(s string) string { return strings.TrimSpace(s) }

// Trim removes the specified cutset from both ends of the string.
func Trim(s, cutset string) string { return strings.Trim(s, cutset) }

// Coalesce returns the first non-empty string among candidates.
func Coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// DefaultIfEmpty returns def if s is empty (after TrimSpace), otherwise s.
func DefaultIfEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

var slugNotAllowed = regexp.MustCompile(`[^a-z0-9\-]+`)

// Slugify converts a string to a URL-friendly slug (lowercase, hyphen-separated).
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// replace whitespace with hyphen
	s = strings.ReplaceAll(s, " ", "-")
	// collapse multiple hyphens
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	// remove any non-allowed chars
	s = slugNotAllowed.ReplaceAllString(s, "")
	// trim hyphens
	s = strings.Trim(s, "-")
	return s
}

// Truncate returns a string not exceeding maxRunes runes. Adds ellipsis if truncated and addEllipsis is true.
func Truncate(s string, maxRunes int, addEllipsis bool) string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	var b strings.Builder
	count := 0
	for _, r := range s {
		if count == maxRunes {
			break
		}
		b.WriteRune(r)
		count++
	}
	out := b.String()
	if addEllipsis && !strings.HasSuffix(out, "…") {
		out += "…"
	}
	return out
}

// SplitAndTrim splits by sep and trims each part, dropping empty parts when dropEmpty is true.
func SplitAndTrim(s, sep string, dropEmpty bool) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if dropEmpty && p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// JoinNonEmpty joins non-empty strings using sep after trimming.
func JoinNonEmpty(sep string, parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, sep)
}

// ContainsIgnoreCase reports whether substr is within s (case-insensitive).
func ContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// IsASCII reports whether s contains only ASCII characters.
func IsASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}
