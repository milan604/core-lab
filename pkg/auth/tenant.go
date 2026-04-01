package auth

import (
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
	return ResolveTenantScope(c, requestedTenantID, DefaultTenantAccessConfig())
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
	cfg := DefaultTenantAccessConfig().WithTenantPathParam(param)
	cfg.RequireTenant = true
	return TenantAccessMiddleware(cfg)
}

// ctxTenantIDKey is the gin context key for the resolved tenant ID.
const (
	ctxTenantIDKey     = "resolved_tenant_id"
	tenantIDContextKey = "tenant_id"
	ctxUserIDKey       = "resolved_user_id"
)

// SetTenantID stores the resolved tenant ID in the gin context for downstream handlers.
func SetTenantID(c *gin.Context, tenantID string) {
	scopedTenantID := strings.TrimSpace(tenantID)
	if scopedTenantID == "" {
		return
	}
	c.Set(ctxTenantIDKey, scopedTenantID)
	if c.Request != nil {
		c.Request = c.Request.WithContext(ContextWithTenantID(c.Request.Context(), scopedTenantID))
	}
}

// SetUserID stores the resolved user ID in the gin context for downstream handlers.
func SetUserID(c *gin.Context, userID string) {
	scopedUserID := strings.TrimSpace(userID)
	if scopedUserID == "" {
		return
	}
	c.Set(ctxUserIDKey, scopedUserID)
	if c.Request != nil {
		c.Request = c.Request.WithContext(ContextWithUserID(c.Request.Context(), scopedUserID))
	}
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

// GetUserID retrieves the resolved user ID from the gin context.
func GetUserID(c *gin.Context) (string, bool) {
	v, exists := c.Get(ctxUserIDKey)
	if !exists {
		return "", false
	}
	userID, ok := v.(string)
	return userID, ok && strings.TrimSpace(userID) != ""
}
