package http

import "strings"

// NormalizeTenantManagementInternalBaseURL accepts either:
// - "http://host:port"
// - "http://host:port/tenant-management"
// - "http://host:port/tenant-management/internal"
// and returns the internal tenant-management API base URL without a trailing slash.
func NormalizeTenantManagementInternalBaseURL(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		return ""
	}

	base = strings.TrimRight(base, "/")
	lower := strings.ToLower(base)

	switch {
	case strings.HasSuffix(lower, "/tenant-management/internal"):
		return base
	case strings.HasSuffix(lower, "/tenant-management"):
		return base + "/internal"
	default:
		return base + "/tenant-management/internal"
	}
}
