package auth

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetClaims retrieves the verified Claims from the request context.
func GetClaims(c *gin.Context) (Claims, bool) {
	val, exists := c.Get(string(CtxAuthClaims))
	if !exists {
		return Claims{}, false
	}
	claims, ok := val.(Claims)
	return claims, ok
}

// ClaimsFromContext retrieves verified claims from a standard context.Context.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	if ctx == nil {
		return Claims{}, false
	}
	val := ctx.Value(string(CtxAuthClaims))
	claims, ok := val.(Claims)
	return claims, ok
}

// ContextWithClaims stores verified claims in a standard context.Context.
func ContextWithClaims(ctx context.Context, claims Claims) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, string(CtxAuthClaims), claims)
}

// ContextWithTenantID stores the resolved tenant ID in a standard context.Context.
func ContextWithTenantID(ctx context.Context, tenantID string) context.Context {
	scopedTenantID := strings.TrimSpace(tenantID)
	if ctx == nil {
		ctx = context.Background()
	}
	if scopedTenantID == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, ctxTenantIDKey, scopedTenantID)
	return context.WithValue(ctx, tenantIDContextKey, scopedTenantID)
}

// TenantIDFromContext retrieves the resolved tenant ID from a standard context.Context.
func TenantIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	for _, key := range []string{ctxTenantIDKey, tenantIDContextKey} {
		if value, ok := ctx.Value(key).(string); ok {
			if tenantID := strings.TrimSpace(value); tenantID != "" {
				return tenantID, true
			}
		}
	}
	return "", false
}
