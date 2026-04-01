package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// TenantAccessConfig controls how tenant/user scope is resolved and enforced.
type TenantAccessConfig struct {
	RequireTenant           bool
	AllowServiceTokens      bool
	AllowPlatformUsers      bool
	EnforceTenantStatus     bool
	EnforceMembershipStatus bool
	TenantPathParam         string
	TenantQueryParam        string
	UserPathParam           string
	UserQueryParam          string
	BlockedTenantStatuses   []string
	BlockedMemberStatuses   []string
}

// DefaultTenantAccessConfig preserves the existing permissive route helper behaviour:
// service tokens are allowed, platform users may pass explicit tenant IDs, and lifecycle
// checks apply when a tenant-scoped user token is present.
func DefaultTenantAccessConfig() TenantAccessConfig {
	return TenantAccessConfig{
		AllowServiceTokens:      true,
		AllowPlatformUsers:      true,
		EnforceTenantStatus:     true,
		EnforceMembershipStatus: true,
		BlockedTenantStatuses:   []string{"suspended", "cancelled", "inactive"},
		BlockedMemberStatuses:   []string{"suspended", "cancelled", "inactive"},
	}
}

// DefaultTenantAppConfig returns the stricter config for tenant-user application routes.
func DefaultTenantAppConfig() TenantAccessConfig {
	cfg := DefaultTenantAccessConfig()
	cfg.RequireTenant = true
	cfg.AllowPlatformUsers = false
	return cfg
}

// WithTenantPathParam configures the path parameter used to resolve the requested tenant.
func (cfg TenantAccessConfig) WithTenantPathParam(param string) TenantAccessConfig {
	cfg.TenantPathParam = strings.TrimSpace(param)
	return cfg
}

// WithTenantQueryParam configures the query parameter used to resolve the requested tenant.
func (cfg TenantAccessConfig) WithTenantQueryParam(param string) TenantAccessConfig {
	cfg.TenantQueryParam = strings.TrimSpace(param)
	return cfg
}

// WithUserPathParam configures the path parameter used to resolve the requested user.
func (cfg TenantAccessConfig) WithUserPathParam(param string) TenantAccessConfig {
	cfg.UserPathParam = strings.TrimSpace(param)
	return cfg
}

// WithUserQueryParam configures the query parameter used to resolve the requested user.
func (cfg TenantAccessConfig) WithUserQueryParam(param string) TenantAccessConfig {
	cfg.UserQueryParam = strings.TrimSpace(param)
	return cfg
}

// TenantAccessMiddleware enforces tenant access and injects the resolved tenant into the request context.
func TenantAccessMiddleware(cfg TenantAccessConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, _, ok := ResolveTenantUserScope(c, nil, nil, cfg); !ok {
			return
		}
		c.Next()
	}
}

// ResolveTenantScope enforces tenant access for tenant-only handlers.
func ResolveTenantScope(c *gin.Context, tenantID string, cfg TenantAccessConfig) (string, bool) {
	tenantPtr, _, ok := ResolveTenantUserScope(c, strPtr(tenantID), nil, cfg)
	if !ok {
		return "", false
	}
	if tenantPtr == nil {
		return "", true
	}
	return *tenantPtr, true
}

// ResolveOptionalTenantScope enforces tenant access for handlers where tenant scope is optional.
func ResolveOptionalTenantScope(c *gin.Context, tenantID *string, cfg TenantAccessConfig) (*string, bool) {
	tenantPtr, _, ok := ResolveTenantUserScope(c, tenantID, nil, cfg)
	return tenantPtr, ok
}

// ResolveTenantUserScope enforces tenant and optional user scoping from claims and explicit request data.
func ResolveTenantUserScope(c *gin.Context, tenantID *string, userID *string, cfg TenantAccessConfig) (*string, *string, bool) {
	claims, ok := GetClaims(c)
	if !ok {
		if reqClaims, reqOK := ClaimsFromContext(c.Request.Context()); reqOK {
			claims = reqClaims
			ok = true
		}
	}
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, nil, false
	}

	requestedTenantID, err := mergeRequestedScope(tenantID, requestedScopeValue(c, cfg.TenantPathParam, cfg.TenantQueryParam))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "tenant_scope_conflict"})
		return nil, nil, false
	}
	requestedUserID, err := mergeRequestedScope(userID, requestedScopeValue(c, cfg.UserPathParam, cfg.UserQueryParam))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "user_scope_conflict"})
		return nil, nil, false
	}

	if claims.IsServiceToken() {
		if !cfg.AllowServiceTokens {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "service_token_not_allowed"})
			return nil, nil, false
		}
		if cfg.RequireTenant && strings.TrimSpace(requestedTenantID) == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant_scope_required"})
			return nil, nil, false
		}
		if requestedTenantID != "" {
			SetTenantID(c, requestedTenantID)
		}
		return strPtr(requestedTenantID), strPtr(requestedUserID), true
	}

	claimTenantID := strings.TrimSpace(claims.TenantID())
	if claimTenantID != "" {
		if cfg.EnforceTenantStatus && isBlockedTenantAccessStatus(claims.TenantStatus(), cfg.BlockedTenantStatuses) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant_inactive"})
			return nil, nil, false
		}
		if cfg.EnforceMembershipStatus && isBlockedTenantAccessStatus(claims.TenantMembershipStatus(), cfg.BlockedMemberStatuses) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant_membership_inactive"})
			return nil, nil, false
		}
	}

	if claimTenantID != "" {
		if requestedTenantID != "" && requestedTenantID != claimTenantID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant_scope_mismatch"})
			return nil, nil, false
		}
		requestedTenantID = claimTenantID
	} else if !cfg.AllowPlatformUsers && (cfg.RequireTenant || requestedTenantID == "") {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant_scope_required"})
		return nil, nil, false
	}

	if cfg.RequireTenant && requestedTenantID == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant_scope_required"})
		return nil, nil, false
	}

	claimUserID := strings.TrimSpace(claims.UserID())
	if claimUserID != "" {
		if requestedUserID != "" && requestedUserID != claimUserID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "user_scope_mismatch"})
			return nil, nil, false
		}
		requestedUserID = claimUserID
	}

	if requestedTenantID != "" {
		SetTenantID(c, requestedTenantID)
	}
	if requestedUserID != "" {
		SetUserID(c, requestedUserID)
	}

	return strPtr(requestedTenantID), strPtr(requestedUserID), true
}

func requestedScopeValue(c *gin.Context, pathParam string, queryParam string) *string {
	pathValue := strings.TrimSpace(pathParam)
	if pathValue != "" {
		if v := strings.TrimSpace(c.Param(pathValue)); v != "" {
			return strPtr(v)
		}
	}
	queryValue := strings.TrimSpace(queryParam)
	if queryValue != "" {
		if v := strings.TrimSpace(c.Query(queryValue)); v != "" {
			return strPtr(v)
		}
	}
	return nil
}

func mergeRequestedScope(explicit *string, inferred *string) (string, error) {
	explicitValue := ptrString(explicit)
	inferredValue := ptrString(inferred)
	if explicitValue != "" && inferredValue != "" && explicitValue != inferredValue {
		return "", errors.New("scope conflict")
	}
	if explicitValue != "" {
		return explicitValue, nil
	}
	return inferredValue, nil
}

func ptrString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func strPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isBlockedTenantAccessStatus(status string, blockedStatuses []string) bool {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return false
	}
	for _, blocked := range blockedStatuses {
		if status == strings.TrimSpace(strings.ToLower(blocked)) {
			return true
		}
	}
	return false
}
