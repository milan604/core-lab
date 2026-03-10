package quota

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/milan604/core-lab/pkg/config"
	httplib "github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"
)

func TestSentinelMiddlewareUsesCachedDecisionWhenSentinelUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var shouldFail atomic.Bool

	cfg := config.New(config.WithDefaults(map[string]any{
		"SentinelServiceEndpoint": "http://sentinel.test",
		"SentinelServiceAPIKey":   "internal-key",
		"SentinelServiceID":       "sites",
		"QuotaFailOpen":           false,
		"QuotaCacheTTLSeconds":    60,
	}))

	oldFactory := newSentinelClientFunc
	newSentinelClientFunc = func(log logger.LogManager, cfg *config.Config) *SentinelClient {
		client := NewSentinelClient(log, cfg)
		client.httpClient = httplib.NewClient(httplib.WithLogger(log), httplib.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/sentinel/internal/api/v1/quota/check" {
					return responseWithStatus(http.StatusNotFound, `{"error":"not_found"}`), nil
				}
				if shouldFail.Load() {
					return responseWithStatus(http.StatusInternalServerError, `{"error":"temporary_failure"}`), nil
				}
				return responseWithStatus(http.StatusOK, `{
					"allowed": true,
					"reason": "ok",
					"tenant_id": "tenant-1",
					"service_id": "sites",
					"metric": "api_calls_per_day",
					"limit": 10,
					"used": 1,
					"remaining": 9,
					"reset_at": "2030-01-01T00:00:00Z"
				}`), nil
			}),
		}))
		return client
	}
	defer func() {
		newSentinelClientFunc = oldFactory
	}()

	engine := gin.New()
	engine.Use(SentinelMiddleware(cfg, logger.MustNewDefaultLogger()))
	engine.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	firstReq.Header.Set("Authorization", "Bearer token-1")
	engine.ServeHTTP(first, firstReq)

	if first.Code != http.StatusNoContent {
		t.Fatalf("first response status = %d, want %d; body=%s", first.Code, http.StatusNoContent, first.Body.String())
	}

	shouldFail.Store(true)

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	secondReq.Header.Set("Authorization", "Bearer token-1")
	engine.ServeHTTP(second, secondReq)

	if second.Code != http.StatusNoContent {
		t.Fatalf("second response status = %d, want %d; body=%s", second.Code, http.StatusNoContent, second.Body.String())
	}
	if got := second.Header().Get("X-RateLimit-Remaining"); got != "8" {
		t.Fatalf("remaining header = %q, want %q", got, "8")
	}
}

func TestSentinelMiddlewareFailsClosedWithoutCachedDecision(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.New(config.WithDefaults(map[string]any{
		"SentinelServiceEndpoint": "http://sentinel.test",
		"SentinelServiceAPIKey":   "internal-key",
		"SentinelServiceID":       "sites",
		"QuotaFailOpen":           false,
		"QuotaCacheTTLSeconds":    60,
	}))

	oldFactory := newSentinelClientFunc
	newSentinelClientFunc = func(log logger.LogManager, cfg *config.Config) *SentinelClient {
		client := NewSentinelClient(log, cfg)
		client.httpClient = httplib.NewClient(httplib.WithLogger(log), httplib.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return responseWithStatus(http.StatusInternalServerError, `{"error":"temporary_failure"}`), nil
			}),
		}))
		return client
	}
	defer func() {
		newSentinelClientFunc = oldFactory
	}()

	engine := gin.New()
	engine.Use(SentinelMiddleware(cfg, logger.MustNewDefaultLogger()))
	engine.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer token-1")
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
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
