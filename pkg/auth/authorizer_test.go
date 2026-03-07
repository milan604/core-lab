package auth

import (
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
