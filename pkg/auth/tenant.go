package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// TenantScopeResult contains the resolved tenant ID and whether scoping succeeded.
type TenantScopeResult struct {
	TenantID string
	OK       bool
}

// EnforceTenantScope resolves and validates tenant scope for the current request.
// It compares the requested tenant ID against the tenant_id claim in the JWT.
//
// Rules:
//   - Service tokens bypass tenant scoping (returns requestedTenantID as-is).
//   - If the JWT has no tenant_id claim, the request is allowed (platform-level user).
//   - If the JWT has a tenant_id claim and requestedTenantID differs, the request is rejected (403).
//   - If requestedTenantID is empty, it defaults to the JWT's tenant_id.
//
// On failure, an appropriate HTTP error is written to gin.Context and false is returned.
func EnforceTenantScope(c *gin.Context, requestedTenantID string) (string, bool) {
	claims, ok := GetClaims(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return "", false
	}

	// Service tokens are trusted — no tenant scoping applied.
	if claims.IsServiceToken() {
		return requestedTenantID, true
	}

	claimTenantID := claims.TenantID()

	// No tenant claim in token — platform-level user, allow through.
	if claimTenantID == "" {
		return requestedTenantID, true
	}

	// Requested tenant differs from claim — cross-tenant access denied.
	if requestedTenantID != "" && requestedTenantID != claimTenantID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant_scope_mismatch"})
		return "", false
	}

	// Default to claim's tenant if none requested.
	if requestedTenantID == "" {
		return claimTenantID, true
	}

	return requestedTenantID, true
}

// TenantScopeFromPath returns a gin middleware that enforces tenant scope
// by comparing the JWT's tenant_id claim against a URL path parameter.
//
// Usage:
//
//	tenantGroup := router.Group("/tenants/:tenant_id", auth.TenantScopeFromPath("tenant_id"))
//
// Service tokens bypass the check. If the JWT has no tenant_id claim, the request is allowed.
func TenantScopeFromPath(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := GetClaims(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if claims.IsServiceToken() {
			c.Next()
			return
		}

		claimTenantID := claims.TenantID()
		if claimTenantID == "" {
			c.Next()
			return
		}

		requestedTenantID := strings.TrimSpace(c.Param(param))
		if requestedTenantID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
			return
		}

		if requestedTenantID != claimTenantID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant_scope_mismatch"})
			return
		}

		c.Next()
	}
}

// ctxTenantIDKey is the gin context key for the resolved tenant ID.
const ctxTenantIDKey = "resolved_tenant_id"

// SetTenantID stores the resolved tenant ID in the gin context for downstream handlers.
func SetTenantID(c *gin.Context, tenantID string) {
	c.Set(ctxTenantIDKey, tenantID)
}

// GetTenantID retrieves the resolved tenant ID from the gin context.
func GetTenantID(c *gin.Context) (string, bool) {
	v, exists := c.Get(ctxTenantIDKey)
	if !exists {
		return "", false
	}
	tid, ok := v.(string)
	return tid, ok && tid != ""
}
