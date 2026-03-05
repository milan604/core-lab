package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/milan604/core-lab/pkg/auth"
)

// TenantStatusConfig configures the tenant status middleware.
type TenantStatusConfig struct {
	// Enabled toggles the middleware on/off.
	Enabled bool
	// BlockedStatuses lists tenant statuses that should be rejected with 403.
	// Default: ["suspended", "cancelled", "inactive"].
	BlockedStatuses []string
}

// DefaultTenantStatusConfig returns a config that blocks suspended, cancelled, and inactive tenants.
func DefaultTenantStatusConfig() TenantStatusConfig {
	return TenantStatusConfig{
		Enabled:         true,
		BlockedStatuses: []string{"suspended", "cancelled", "inactive"},
	}
}

// TenantStatusMiddleware checks the tenant_status claim in the JWT and blocks
// requests from tenants whose status is in the blocked list.
//
// Service tokens bypass this check (service-to-service calls are always allowed).
// Requests without a tenant_status claim (platform-level users or legacy tokens)
// are allowed through.
//
// Usage:
//
//	engine.Use(middleware.TenantStatusMiddleware(middleware.DefaultTenantStatusConfig()))
func TenantStatusMiddleware(cfg TenantStatusConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	blocked := make(map[string]bool, len(cfg.BlockedStatuses))
	for _, s := range cfg.BlockedStatuses {
		blocked[s] = true
	}

	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			// No JWT claims yet (unauthenticated route) — let the auth middleware handle it.
			c.Next()
			return
		}

		authClaims, ok := claims.(*auth.Claims)
		if !ok {
			c.Next()
			return
		}

		// Service tokens bypass tenant status checks.
		if authClaims.IsServiceToken() {
			c.Next()
			return
		}

		status := authClaims.TenantStatus()
		if status == "" {
			// No tenant_status claim — platform-level user or legacy token.
			c.Next()
			return
		}

		if blocked[status] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "tenant_suspended",
				"message": "Your tenant account is currently " + status + ". Please contact support.",
			})
			return
		}

		c.Next()
	}
}
