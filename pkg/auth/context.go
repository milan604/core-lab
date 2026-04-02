package auth

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	coretenant "github.com/milan604/core-lab/pkg/tenant"
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
	ctx = context.WithValue(ctx, string(CtxAuthClaims), claims)
	return coretenant.ContextWithRequestContext(ctx, coretenant.RequestContext{
		TenantID:     claims.TenantID(),
		ActorUserID:  claims.UserID(),
		ServiceID:    claims.ServiceID(),
		IsSuperAdmin: claims.IsSuperAdmin(),
	})
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
	if existing, ok := coretenant.RequestContextFromContext(ctx); ok {
		ctx = coretenant.ContextWithRequestContext(ctx, (coretenant.RequestContext{
			TenantID: scopedTenantID,
		}).WithFallbacks(existing))
	} else {
		ctx = coretenant.ContextWithRequestContext(ctx, coretenant.RequestContext{TenantID: scopedTenantID})
	}
	ctx = context.WithValue(ctx, ctxTenantIDKey, scopedTenantID)
	return context.WithValue(ctx, tenantIDContextKey, scopedTenantID)
}

// ContextWithUserID stores the resolved user ID in a standard context.Context.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	scopedUserID := strings.TrimSpace(userID)
	if ctx == nil {
		ctx = context.Background()
	}
	if scopedUserID == "" {
		return ctx
	}
	if existing, ok := coretenant.RequestContextFromContext(ctx); ok {
		ctx = coretenant.ContextWithRequestContext(ctx, (coretenant.RequestContext{
			ActorUserID: scopedUserID,
		}).WithFallbacks(existing))
	} else {
		ctx = coretenant.ContextWithRequestContext(ctx, coretenant.RequestContext{ActorUserID: scopedUserID})
	}
	return context.WithValue(ctx, ctxUserIDKey, scopedUserID)
}

// ContextWithServiceID stores the calling service identifier in a standard context.Context.
func ContextWithServiceID(ctx context.Context, serviceID string) context.Context {
	scopedServiceID := strings.TrimSpace(serviceID)
	if ctx == nil {
		ctx = context.Background()
	}
	if scopedServiceID == "" {
		return ctx
	}
	if existing, ok := coretenant.RequestContextFromContext(ctx); ok {
		return coretenant.ContextWithRequestContext(ctx, (coretenant.RequestContext{
			ServiceID: scopedServiceID,
		}).WithFallbacks(existing))
	}
	return coretenant.ContextWithRequestContext(ctx, coretenant.RequestContext{ServiceID: scopedServiceID})
}

// ContextWithCorrelationID stores the request correlation identifier in a standard context.Context.
func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	scopedCorrelationID := strings.TrimSpace(correlationID)
	if ctx == nil {
		ctx = context.Background()
	}
	if scopedCorrelationID == "" {
		return ctx
	}
	if existing, ok := coretenant.RequestContextFromContext(ctx); ok {
		return coretenant.ContextWithRequestContext(ctx, (coretenant.RequestContext{
			CorrelationID: scopedCorrelationID,
		}).WithFallbacks(existing))
	}
	return coretenant.ContextWithRequestContext(ctx, coretenant.RequestContext{CorrelationID: scopedCorrelationID})
}

// ContextWithSuperAdmin marks the caller as a global super admin in the request context.
func ContextWithSuperAdmin(ctx context.Context, isSuperAdmin bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if existing, ok := coretenant.RequestContextFromContext(ctx); ok {
		existing.IsSuperAdmin = existing.IsSuperAdmin || isSuperAdmin
		return coretenant.ContextWithRequestContext(ctx, existing)
	}
	if !isSuperAdmin {
		return ctx
	}
	return coretenant.ContextWithRequestContext(ctx, coretenant.RequestContext{IsSuperAdmin: true})
}

// TenantIDFromContext retrieves the resolved tenant ID from a standard context.Context.
func TenantIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	if requestContext, ok := coretenant.RequestContextFromContext(ctx); ok && strings.TrimSpace(requestContext.TenantID) != "" {
		return requestContext.TenantID, true
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

// UserIDFromContext retrieves the resolved user ID from a standard context.Context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	if requestContext, ok := coretenant.RequestContextFromContext(ctx); ok && strings.TrimSpace(requestContext.ActorUserID) != "" {
		return requestContext.ActorUserID, true
	}
	if value, ok := ctx.Value(ctxUserIDKey).(string); ok {
		if userID := strings.TrimSpace(value); userID != "" {
			return userID, true
		}
	}
	return "", false
}

// ServiceIDFromContext retrieves the calling service identifier from a standard context.Context.
func ServiceIDFromContext(ctx context.Context) (string, bool) {
	requestContext, ok := coretenant.RequestContextFromContext(ctx)
	if !ok || strings.TrimSpace(requestContext.ServiceID) == "" {
		return "", false
	}
	return requestContext.ServiceID, true
}

// CorrelationIDFromContext retrieves the request correlation identifier from a standard context.Context.
func CorrelationIDFromContext(ctx context.Context) (string, bool) {
	requestContext, ok := coretenant.RequestContextFromContext(ctx)
	if !ok || strings.TrimSpace(requestContext.CorrelationID) == "" {
		return "", false
	}
	return requestContext.CorrelationID, true
}

// IsSuperAdminFromContext retrieves the global super-admin marker from a standard context.Context.
func IsSuperAdminFromContext(ctx context.Context) bool {
	requestContext, ok := coretenant.RequestContextFromContext(ctx)
	return ok && requestContext.IsSuperAdmin
}
