package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestRequireAuthenticatedSeedsRequestContextClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey": publicKeyPEM,
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":                      "user-123",
		"identity_id":              "identity-123",
		"tenant_id":                "tenant-123",
		"tenant_status":            "active",
		"tenant_membership_status": "active",
	})

	router := gin.New()
	protected := router.Group("/protected", authorizer.RequireAuthenticated())
	protected.GET("", func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c.Request.Context())
		if !ok {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if claims.TenantID() != "tenant-123" || claims.UserID() != "identity-123" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}

func TestTenantAccessMiddlewareEnforcesTenantAppScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey": publicKeyPEM,
	})

	router := gin.New()
	tenantApp := router.Group(
		"/tenant-app",
		authorizer.RequireAuthenticated(),
		TenantAccessMiddleware(DefaultTenantAppConfig()),
	)
	tenantApp.GET("", func(c *gin.Context) {
		tenantID, ok := GetTenantID(c)
		if !ok || tenantID != "tenant-123" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	activeToken := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":                      "user-123",
		"identity_id":              "identity-123",
		"tenant_id":                "tenant-123",
		"tenant_status":            "active",
		"tenant_membership_status": "active",
	})

	activeReq := httptest.NewRequest(http.MethodGet, "/tenant-app", nil)
	activeReq.Header.Set("Authorization", "Bearer "+activeToken)
	activeRecorder := httptest.NewRecorder()
	router.ServeHTTP(activeRecorder, activeReq)

	if activeRecorder.Code != http.StatusNoContent {
		t.Fatalf("active status = %d, want %d; body=%s", activeRecorder.Code, http.StatusNoContent, activeRecorder.Body.String())
	}

	platformToken := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":         "user-456",
		"identity_id": "identity-456",
	})

	platformReq := httptest.NewRequest(http.MethodGet, "/tenant-app", nil)
	platformReq.Header.Set("Authorization", "Bearer "+platformToken)
	platformRecorder := httptest.NewRecorder()
	router.ServeHTTP(platformRecorder, platformReq)

	if platformRecorder.Code != http.StatusForbidden {
		t.Fatalf("platform status = %d, want %d; body=%s", platformRecorder.Code, http.StatusForbidden, platformRecorder.Body.String())
	}
}

func TestTenantAccessMiddlewareSeedsResolvedUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey": publicKeyPEM,
	})

	router := gin.New()
	scoped := router.Group(
		"/scoped",
		authorizer.RequireAuthenticated(),
		TenantAccessMiddleware(DefaultTenantAccessConfig().WithTenantQueryParam("tenant_id").WithUserQueryParam("user_id")),
	)
	scoped.GET("", func(c *gin.Context) {
		if tenantID, ok := GetTenantID(c); !ok || tenantID != "tenant-123" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if userID, ok := GetUserID(c); !ok || userID != "identity-123" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if userID, ok := UserIDFromContext(c.Request.Context()); !ok || userID != "identity-123" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":                      "user-123",
		"identity_id":              "identity-123",
		"tenant_id":                "tenant-123",
		"tenant_status":            "active",
		"tenant_membership_status": "active",
	})

	req := httptest.NewRequest(http.MethodGet, "/scoped?tenant_id=tenant-123&user_id=identity-123", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}
