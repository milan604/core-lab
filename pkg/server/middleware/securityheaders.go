package server

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersConfig defines which security headers to apply.
type SecurityHeadersConfig struct {
	Enabled bool

	// ContentTypeNosniff sets X-Content-Type-Options: nosniff (default: true).
	ContentTypeNosniff bool
	// FrameOptions sets X-Frame-Options (default: "DENY"). Use "SAMEORIGIN" if embedding is needed.
	FrameOptions string
	// XSSProtection sets X-XSS-Protection (default: "1; mode=block"). Legacy but still useful.
	XSSProtection string
	// ReferrerPolicy sets Referrer-Policy (default: "strict-origin-when-cross-origin").
	ReferrerPolicy string
	// HSTS sets Strict-Transport-Security. Empty string disables. Default: "max-age=31536000; includeSubDomains".
	HSTS string
	// ContentSecurityPolicy sets Content-Security-Policy. Empty string disables.
	ContentSecurityPolicy string
	// PermissionsPolicy sets Permissions-Policy. Empty string disables.
	PermissionsPolicy string
}

// DefaultSecurityHeadersConfig returns a safe default configuration.
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		Enabled:               true,
		ContentTypeNosniff:    true,
		FrameOptions:          "DENY",
		XSSProtection:         "1; mode=block",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		HSTS:                  "max-age=31536000; includeSubDomains",
		ContentSecurityPolicy: "",
		PermissionsPolicy:     "camera=(), microphone=(), geolocation=()",
	}
}

// SecurityHeadersMiddleware returns a gin middleware that sets standard security response headers.
func SecurityHeadersMiddleware(cfg SecurityHeadersConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		h := c.Writer.Header()

		if cfg.ContentTypeNosniff {
			h.Set("X-Content-Type-Options", "nosniff")
		}
		if cfg.FrameOptions != "" {
			h.Set("X-Frame-Options", cfg.FrameOptions)
		}
		if cfg.XSSProtection != "" {
			h.Set("X-XSS-Protection", cfg.XSSProtection)
		}
		if cfg.ReferrerPolicy != "" {
			h.Set("Referrer-Policy", cfg.ReferrerPolicy)
		}
		if cfg.HSTS != "" {
			h.Set("Strict-Transport-Security", cfg.HSTS)
		}
		if cfg.ContentSecurityPolicy != "" {
			h.Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
		}
		if cfg.PermissionsPolicy != "" {
			h.Set("Permissions-Policy", cfg.PermissionsPolicy)
		}

		c.Next()
	}
}
