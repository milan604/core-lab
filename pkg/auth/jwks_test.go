package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/milan604/core-lab/pkg/logger"
)

func TestAuthorizerUsesJWKSDiscoveryWhenPublicKeyNotConfigured(t *testing.T) {
	privateKey, kid, jwksPayload := testJWKSKey(t)

	authorizer, err := NewAuthorizer(stubConfig{
		"SentinelServiceEndpoint": "http://sentinel.test",
		"SentinelTokenIssuer":     "Sentinel",
		"SentinelTokenAudience":   "sentinel-clients",
	}, logger.MustNewDefaultLogger())
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	authorizer.verifier.remote.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/sentinel/.well-known/openid-configuration":
			payload, _ := json.Marshal(map[string]any{
				"issuer":   "Sentinel",
				"jwks_uri": "http://sentinel.test/sentinel/.well-known/jwks.json",
			})
			return responseWithStatus(http.StatusOK, string(payload)), nil
		case "/sentinel/.well-known/jwks.json":
			return responseWithStatus(http.StatusOK, string(jwksPayload)), nil
		default:
			return responseWithStatus(http.StatusNotFound, `{"error":"not_found"}`), nil
		}
	})

	token := signTestTokenWithHeader(t, privateKey, kid, jwt.MapClaims{
		"sub": "user-1",
		"iss": "Sentinel",
		"aud": "sentinel-clients",
	})

	claims, err := authorizer.verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("subject = %q, want %q", claims.Subject, "user-1")
	}
}

func TestAuthorizerFallsBackToStaticKeyWhenJWKSUnavailable(t *testing.T) {
	privateKey, publicKeyPEM := testKeyPair(t)

	authorizer, err := NewAuthorizer(stubConfig{
		"RSAPublicKey":            publicKeyPEM,
		"SentinelServiceEndpoint": "http://sentinel.test",
	}, logger.MustNewDefaultLogger())
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	authorizer.verifier.remote.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return responseWithStatus(http.StatusServiceUnavailable, `{"error":"unavailable"}`), nil
	})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"sub": "user-1",
	})

	claims, err := authorizer.verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("subject = %q, want %q", claims.Subject, "user-1")
	}
}

func testJWKSKey(t *testing.T) (*rsa.PrivateKey, string, []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}

	kid := base64.RawURLEncoding.EncodeToString(publicKeyDER[:16])
	payload := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": kid,
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return privateKey, kid, body
}

func signTestTokenWithHeader(t *testing.T, privateKey *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	return signed
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func responseWithStatus(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
