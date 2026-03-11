package http

import (
	"context"
	"io"
	stdhttp "net/http"
	"strings"
	"testing"

	"github.com/milan604/core-lab/pkg/config"
)

func TestServiceTokenProviderFetchTokenIncludesInternalKey(t *testing.T) {
	t.Parallel()

	client := NewClient(WithHTTPClient(&stdhttp.Client{
		Transport: roundTripFunc(func(req *stdhttp.Request) (*stdhttp.Response, error) {
			if got := req.Header.Get("X-Internal-Key"); got != "platform-internal" {
				t.Fatalf("X-Internal-Key = %q, want %q", got, "platform-internal")
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
		ServiceURL:  "http://sentinel.test/sentinel",
		ServiceID:   "sites",
		APIKey:      "service-api-key",
		InternalKey: "platform-internal",
		Scope:       "sentinel.admin",
		HTTPClient:  client,
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

func TestNewClientWithServiceTokenRequiresInternalAdminKey(t *testing.T) {
	t.Parallel()

	cfg := config.New(config.WithDefaults(map[string]any{
		"SentinelServiceEndpoint": "http://sentinel.test/sentinel",
		"SentinelServiceID":       "sites",
		"SentinelServiceAPIKey":   "service-api-key",
	}))

	_, err := NewClientWithServiceToken(nil, cfg)
	if err == nil || !strings.Contains(err.Error(), "InternalAdminKey") {
		t.Fatalf("NewClientWithServiceToken() error = %v, want missing InternalAdminKey", err)
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
