package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	stdhttp "net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/milan604/core-lab/pkg/config"
)

func TestServiceTokenProviderFetchTokenPostsServiceCredentials(t *testing.T) {
	t.Parallel()

	client := NewClient(WithHTTPClient(&stdhttp.Client{
		Transport: roundTripFunc(func(req *stdhttp.Request) (*stdhttp.Response, error) {
			if got := req.Header.Get("X-Internal-Key"); got != "" {
				t.Fatalf("X-Internal-Key = %q, want empty", got)
			}
			if req.URL.Path != "/sentinel/internal/api/v1/service-token" {
				t.Fatalf("path = %q, want %q", req.URL.Path, "/sentinel/internal/api/v1/service-token")
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			if !strings.Contains(string(body), `"service_id":"sites"`) {
				t.Fatalf("unexpected request body: %s", string(body))
			}
			return jsonResponse(stdhttp.StatusOK, `{"access_token":"token-123","expires_in":3600}`), nil
		}),
	}))

	provider := NewServiceTokenProvider(ServiceTokenProviderConfig{
		ServiceURL: "http://sentinel.test/sentinel",
		ServiceID:  "sites",
		APIKey:     "service-api-key",
		Scope:      "sentinel.admin",
		HTTPClient: client,
	})

	token, expiresAt, err := provider.FetchToken(context.Background())
	if err != nil {
		t.Fatalf("FetchToken() error = %v", err)
	}
	if token != "token-123" {
		t.Fatalf("FetchToken() token = %q, want %q", token, "token-123")
	}
	if expiresAt.IsZero() {
		t.Fatalf("FetchToken() returned zero expiration")
	}
}

func TestServiceTokenProviderFetchTokenDoesNotRequireInternalKey(t *testing.T) {
	t.Parallel()

	client := NewClient(WithHTTPClient(&stdhttp.Client{
		Transport: roundTripFunc(func(req *stdhttp.Request) (*stdhttp.Response, error) {
			if got := req.Header.Get("X-Internal-Key"); got != "" {
				t.Fatalf("X-Internal-Key = %q, want empty", got)
			}
			return jsonResponse(stdhttp.StatusOK, `{"access_token":"token-123","expires_in":3600}`), nil
		}),
	}))

	provider := NewServiceTokenProvider(ServiceTokenProviderConfig{
		ServiceURL: "http://sentinel.test/sentinel",
		ServiceID:  "sites",
		APIKey:     "service-api-key",
		Scope:      "sentinel.admin",
		HTTPClient: client,
	})

	token, _, err := provider.FetchToken(context.Background())
	if err != nil {
		t.Fatalf("FetchToken() error = %v", err)
	}
	if token != "token-123" {
		t.Fatalf("FetchToken() token = %q, want %q", token, "token-123")
	}
}

func TestNewClientWithServiceTokenRequiresMTLSConfig(t *testing.T) {
	t.Parallel()

	cfg := config.New(config.WithDefaults(map[string]any{
		"SentinelServiceEndpoint": "http://sentinel.test/sentinel",
		"PlatformServiceID":       "sites",
		"PlatformServiceAPIKey":   "service-api-key",
	}))

	_, err := NewClientWithServiceToken(nil, cfg)
	if err == nil || !strings.Contains(err.Error(), "PlatformMTLSCertFile") {
		t.Fatalf("NewClientWithServiceToken() error = %v, want missing mTLS configuration", err)
	}
}

func TestNewClientWithServiceTokenSupportsPlatformKeys(t *testing.T) {
	t.Parallel()

	certFile, keyFile, caFile := writeTestMTLSFiles(t)

	cfg := config.New(config.WithDefaults(map[string]any{
		"PlatformControlPlaneEndpoint": "http://iam.test/control-plane",
		"PlatformServiceID":            "sites",
		"PlatformServiceAPIKey":        "service-api-key",
		"PlatformMTLSCertFile":         certFile,
		"PlatformMTLSKeyFile":          keyFile,
		"PlatformMTLSCAFile":           caFile,
		"PlatformTokenAudience":        []string{"config-api"},
	}))

	client, err := NewClientWithServiceToken(nil, cfg)
	if err != nil {
		t.Fatalf("NewClientWithServiceToken() error = %v", err)
	}
	if client == nil {
		t.Fatalf("NewClientWithServiceToken() returned nil client")
	}
	transport, ok := client.httpClient.Transport.(*stdhttp.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected http transport with TLS config, got %T", client.httpClient.Transport)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("expected TLS client config to be set on returned client")
	}
	if transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("expected custom RootCAs to be configured on returned client")
	}
	if len(transport.TLSClientConfig.Certificates) != 1 {
		t.Fatalf("expected client certificate to be configured, got %d", len(transport.TLSClientConfig.Certificates))
	}
}

func TestNewInternalControlPlaneClientForAudienceSupportsPlatformKeys(t *testing.T) {
	t.Parallel()

	certFile, keyFile, caFile := writeTestMTLSFiles(t)

	cfg := config.New(config.WithDefaults(map[string]any{
		"PlatformControlPlaneEndpoint": "http://iam.test/control-plane",
		"PlatformServiceID":            "sites",
		"PlatformServiceAPIKey":        "service-api-key",
		"PlatformMTLSCertFile":         certFile,
		"PlatformMTLSKeyFile":          keyFile,
		"PlatformMTLSCAFile":           caFile,
	}))

	client, err := NewInternalControlPlaneClientForAudience(nil, cfg, []string{"platform.config"})
	if err != nil {
		t.Fatalf("NewInternalControlPlaneClientForAudience() error = %v", err)
	}
	if client == nil {
		t.Fatalf("NewInternalControlPlaneClientForAudience() returned nil client")
	}
}

type roundTripFunc func(*stdhttp.Request) (*stdhttp.Response, error)

func (fn roundTripFunc) RoundTrip(req *stdhttp.Request) (*stdhttp.Response, error) {
	return fn(req)
}

func jsonResponse(status int, body string) *stdhttp.Response {
	return &stdhttp.Response{
		StatusCode: status,
		Header:     stdhttp.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func writeTestMTLSFiles(t *testing.T) (certFile, keyFile, caFile string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "sites",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	dir := t.TempDir()
	certFile = filepath.Join(dir, "client.crt")
	keyFile = filepath.Join(dir, "client.key")
	caFile = filepath.Join(dir, "ca.crt")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	if err := os.WriteFile(caFile, certPEM, 0600); err != nil {
		t.Fatalf("write ca file: %v", err)
	}

	return certFile, keyFile, caFile
}
