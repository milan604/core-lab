package http

import "strings"

// NormalizeSentinelBaseURL accepts either "http://host:port" or "http://host:port/sentinel"
// and returns the sentinel API base URL without a trailing slash.
func NormalizeSentinelBaseURL(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		return ""
	}

	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(strings.ToLower(base), "/sentinel") {
		return base
	}

	return base + "/sentinel"
}
