package http

import "github.com/milan604/core-lab/pkg/controlplane"

// NormalizeSentinelBaseURL accepts either "http://host:port" or "http://host:port/sentinel"
// and returns the sentinel API base URL without a trailing slash.
func NormalizeSentinelBaseURL(raw string) string {
	return controlplane.NormalizeLegacySentinelBaseURL(raw)
}
