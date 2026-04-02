package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/milan604/core-lab/pkg/logger"
)

type stubConfig map[string]string

func (c stubConfig) GetString(key string) string {
	return c[key]
}

func TestRequirePermissionBypassesServiceTokenByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey": publicKeyPEM,
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":       "svc-ecompulse",
		"token_use": "service",
	})

	router := gin.New()
	router.GET("/protected", authorizer.RequirePermission("TEN-TENANTS-LIST"), func(c *gin.Context) {
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

func TestRequirePermissionCanDisableServiceTokenBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey":                  publicKeyPEM,
		"BypassServiceTokenPermissions": "false",
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":       "svc-ecompulse",
		"token_use": "service",
	})

	router := gin.New()
	router.GET("/protected", authorizer.RequirePermission("TEN-TENANTS-LIST"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "service_not_available") {
		t.Fatalf("body = %q, want service_not_available error", recorder.Body.String())
	}
}

func TestRequireServiceTokenRejectsUserAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey": publicKeyPEM,
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":         "user-1",
		"identity_id": "user-1",
		"token_use":   "access",
	})

	router := gin.New()
	router.GET("/internal", authorizer.RequireServiceToken(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/internal", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "service_token_required") {
		t.Fatalf("body = %q, want service_token_required error", recorder.Body.String())
	}
}

func TestRequirePermissionUsesDecisionClientWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	client := &stubPermissionDecisionClient{
		response: permissionDecisionResponse{Allowed: true},
	}

	restore := overridePermissionDecisionClientFactory(func(cfg Config, log logger.LogManager) permissionDecisionClient {
		return client
	})
	defer restore()

	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey": publicKeyPEM,
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":         "user-1",
		"identity_id": "user-1",
		"tenant_id":   "tenant-1",
	})

	router := gin.New()
	router.GET("/brands/:brand_id", authorizer.RequirePermission("ECO-BRANDS-UPDATE"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/brands/brand-123?channel=web", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	if client.callCount != 1 {
		t.Fatalf("Decide() calls = %d, want 1", client.callCount)
	}
	if client.lastRequest.SubjectUserID != "user-1" {
		t.Fatalf("SubjectUserID = %q, want user-1", client.lastRequest.SubjectUserID)
	}
	if client.lastRequest.TenantID != "tenant-1" {
		t.Fatalf("TenantID = %q, want tenant-1", client.lastRequest.TenantID)
	}
	if client.lastRequest.Service != "eco" {
		t.Fatalf("Service = %q, want eco", client.lastRequest.Service)
	}
	if client.lastRequest.Category != "brands" {
		t.Fatalf("Category = %q, want brands", client.lastRequest.Category)
	}
	if client.lastRequest.Action != "update" {
		t.Fatalf("Action = %q, want update", client.lastRequest.Action)
	}
	if client.lastRequest.ResourceType != "brand" {
		t.Fatalf("ResourceType = %q, want brand", client.lastRequest.ResourceType)
	}
	if client.lastRequest.ResourceID != "brand-123" {
		t.Fatalf("ResourceID = %q, want brand-123", client.lastRequest.ResourceID)
	}
	if got := client.lastRequest.Context["channel"]; got != "web" {
		t.Fatalf("Context[channel] = %q, want web", got)
	}
}

func TestRequirePermissionReturnsForbiddenWhenDecisionDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	restore := overridePermissionDecisionClientFactory(func(cfg Config, log logger.LogManager) permissionDecisionClient {
		return &stubPermissionDecisionClient{
			response: permissionDecisionResponse{Allowed: false, Reasons: []string{"policy_denied"}},
		}
	})
	defer restore()

	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey": publicKeyPEM,
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":         "user-1",
		"identity_id": "user-1",
	})

	router := gin.New()
	router.GET("/brands/:brand_id", authorizer.RequirePermission("ECO-BRANDS-UPDATE"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/brands/brand-123", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "permission_denied") {
		t.Fatalf("body = %q, want permission_denied error", recorder.Body.String())
	}
}

func TestRequirePermissionReturnsServiceUnavailableWhenDecisionFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	restore := overridePermissionDecisionClientFactory(func(cfg Config, log logger.LogManager) permissionDecisionClient {
		return &stubPermissionDecisionClient{
			err: context.DeadlineExceeded,
		}
	})
	defer restore()

	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey": publicKeyPEM,
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":         "user-1",
		"identity_id": "user-1",
	})

	router := gin.New()
	router.GET("/brands/:brand_id", authorizer.RequirePermission("ECO-BRANDS-UPDATE"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/brands/brand-123", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "authorization_unavailable") {
		t.Fatalf("body = %q, want authorization_unavailable error", recorder.Body.String())
	}
}

func TestRequirePermissionParsesHyphenatedActionsForDecisionRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, publicKeyPEM := testKeyPair(t)
	client := &stubPermissionDecisionClient{
		response: permissionDecisionResponse{Allowed: true},
	}

	restore := overridePermissionDecisionClientFactory(func(cfg Config, log logger.LogManager) permissionDecisionClient {
		return client
	})
	defer restore()

	authorizer := testAuthorizer(t, stubConfig{
		"RSAPublicKey": publicKeyPEM,
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub":         "user-1",
		"identity_id": "user-1",
		"tenant_id":   "tenant-1",
	})

	router := gin.New()
	router.GET("/brands/tenant/:tenant_id", authorizer.RequirePermission("ECO-BRANDS-TENANT-LIST"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/brands/tenant/tenant-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	if client.lastRequest.Action != "tenant-list" {
		t.Fatalf("Action = %q, want tenant-list", client.lastRequest.Action)
	}
	if client.lastRequest.TenantID != "tenant-1" {
		t.Fatalf("TenantID = %q, want tenant-1", client.lastRequest.TenantID)
	}
}

func testAuthorizer(t *testing.T, cfg stubConfig) *Authorizer {
	t.Helper()

	authorizer, err := NewAuthorizer(cfg, logger.MustNewDefaultLogger())
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	return authorizer
}

func testKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	return privateKey, string(publicKeyPEM)
}

func signTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()

	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return token
}

type stubPermissionDecisionClient struct {
	response    permissionDecisionResponse
	err         error
	lastRequest permissionDecisionRequest
	callCount   int
}

func (s *stubPermissionDecisionClient) Decide(ctx context.Context, input permissionDecisionRequest) (permissionDecisionResponse, error) {
	s.callCount++
	s.lastRequest = input
	return s.response, s.err
}

func overridePermissionDecisionClientFactory(factory func(cfg Config, log logger.LogManager) permissionDecisionClient) func() {
	previous := newPermissionDecisionClientFunc
	newPermissionDecisionClientFunc = factory
	return func() {
		newPermissionDecisionClientFunc = previous
	}
}
