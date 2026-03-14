package configmanager

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	httplib "github.com/milan604/core-lab/pkg/http"
)

func TestResolveNamespace(t *testing.T) {
	t.Parallel()

	client := &Client{
		baseURL: "https://control-plane.internal",
		httpClient: httplib.NewClient(httplib.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost {
					t.Fatalf("expected POST request, got %s", r.Method)
				}
				if r.URL.Path != "/internal/api/v1/config/resolve" {
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
				return jsonResponse(`{
			"namespace":{"id":"namespace-1","service_id":"svc","namespace_key":"runtime"},
			"release_id":"release-1",
			"version":"1.0.0",
			"delivery_mode":"pull",
			"watch_mode":"poll",
			"watch_interval_seconds":30,
			"etag":"etag-1",
			"config":{"endpoint":"https://example.com"}
		}`)
			}),
		})),
	}

	resp, err := client.ResolveNamespace(context.Background(), NamespaceResolveRequest{
		ServiceID:    "svc",
		NamespaceKey: "runtime",
		Environment:  "prod",
	})
	if err != nil {
		t.Fatalf("resolve namespace: %v", err)
	}
	if resp.ReleaseID != "release-1" {
		t.Fatalf("expected release-1, got %s", resp.ReleaseID)
	}
	if resp.Config["endpoint"] != "https://example.com" {
		t.Fatalf("expected endpoint in resolved config, got %#v", resp.Config)
	}
}

func TestGetNamespaceWatch(t *testing.T) {
	t.Parallel()

	client := &Client{
		baseURL: "https://control-plane.internal",
		httpClient: httplib.NewClient(httplib.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet {
					t.Fatalf("expected GET request, got %s", r.Method)
				}
				if r.URL.Path != "/internal/api/v1/config/watch" {
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
				if got := r.URL.Query().Get("service_id"); got != "svc" {
					t.Fatalf("expected service_id query, got %s", got)
				}
				if got := r.URL.Query().Get("namespace_key"); got != "runtime" {
					t.Fatalf("expected namespace_key query, got %s", got)
				}
				return jsonResponse(`{
			"namespace_id":"namespace-1",
			"service_id":"svc",
			"namespace_key":"runtime",
			"release_id":"release-1",
			"version":"1.0.0",
			"status":"approved",
			"delivery_mode":"pull",
			"watch_mode":"poll",
			"watch_interval_seconds":30,
			"etag":"etag-1"
		}`)
			}),
		})),
	}

	resp, err := client.GetNamespaceWatch(context.Background(), NamespaceWatchRequest{
		ServiceID:    "svc",
		NamespaceKey: "runtime",
	})
	if err != nil {
		t.Fatalf("get namespace watch: %v", err)
	}
	if resp.ReleaseID != "release-1" {
		t.Fatalf("expected release-1, got %s", resp.ReleaseID)
	}
	if resp.WatchMode != "poll" {
		t.Fatalf("expected poll watch mode, got %s", resp.WatchMode)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}
