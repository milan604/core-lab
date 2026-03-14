package authz

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/milan604/core-lab/pkg/auth"
	"github.com/milan604/core-lab/pkg/config"
	httplib "github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"
)

func TestMiddlewareAllowsAndStoresDecision(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldFactory := newAuthzClientFunc
	newAuthzClientFunc = func(log logger.LogManager, cfg *config.Config) *Client {
		return &Client{
			baseURL: "http://iam.test/control-plane",
			httpClient: httplib.NewClient(httplib.WithHTTPClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/control-plane/internal/api/v1/authz/decide" {
						t.Fatalf("path = %q", req.URL.Path)
					}
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read body: %v", err)
					}
					payload := string(body)
					if !strings.Contains(payload, `"subject_user_id":"user-1"`) {
						t.Fatalf("missing subject in body: %s", payload)
					}
					if !strings.Contains(payload, `"resource_id":"site-123"`) {
						t.Fatalf("missing resource id in body: %s", payload)
					}
					return jsonResponse(http.StatusOK, `{"allowed":true,"subject_user_id":"user-1","service":"sites","permission_code":"sites-sites-read","resource_id":"site-123"}`), nil
				}),
			})),
		}
	}
	defer func() { newAuthzClientFunc = oldFactory }()

	cfg := config.New(config.WithDefaults(map[string]any{
		"PlatformServiceID": "sites",
	}))

	engine := gin.New()
	engine.Use(withVerifiedClaims(auth.Claims{
		TokenUse:   "access",
		IdentityID: "user-1",
		Raw: map[string]any{
			"tenant_id": "tenant-1",
		},
	}))
	engine.Use(Middleware(cfg, logger.MustNewDefaultLogger(), NewMiddlewareConfig(cfg).
		WithCategory("sites").
		WithAction("read").
		WithResourceType("site").
		WithRequireTenant(true).
		WithResourcePathParam("site_id")))
	engine.GET("/sites/:site_id", func(c *gin.Context) {
		decision, ok := GetDecision(c)
		if !ok {
			t.Fatalf("expected decision in gin context")
		}
		if !decision.Allowed {
			t.Fatalf("expected allowed decision in handler")
		}
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sites/site-123", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestMiddlewareDeniesAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldFactory := newAuthzClientFunc
	newAuthzClientFunc = func(log logger.LogManager, cfg *config.Config) *Client {
		return &Client{
			baseURL: "http://iam.test/control-plane",
			httpClient: httplib.NewClient(httplib.WithHTTPClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return jsonResponse(http.StatusOK, `{"allowed":false,"subject_user_id":"user-1","service":"sites","permission_code":"sites-sites-delete","reasons":["denied_by_policy"]}`), nil
				}),
			})),
		}
	}
	defer func() { newAuthzClientFunc = oldFactory }()

	cfg := config.New(config.WithDefaults(map[string]any{
		"PlatformServiceID": "sites",
	}))

	engine := gin.New()
	engine.Use(withVerifiedClaims(auth.Claims{
		TokenUse:   "access",
		IdentityID: "user-1",
	}))
	engine.Use(Middleware(cfg, logger.MustNewDefaultLogger(), NewMiddlewareConfig(cfg).
		WithCategory("sites").
		WithAction("delete")))
	engine.GET("/sites", func(c *gin.Context) {
		t.Fatalf("handler should not execute on denied decision")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sites", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "access_denied") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestMiddlewareUsesCachedDecisionOnFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var shouldFail atomic.Bool
	oldFactory := newAuthzClientFunc
	newAuthzClientFunc = func(log logger.LogManager, cfg *config.Config) *Client {
		return &Client{
			baseURL: "http://iam.test/control-plane",
			httpClient: httplib.NewClient(httplib.WithHTTPClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if shouldFail.Load() {
						return jsonResponse(http.StatusServiceUnavailable, `{"error":"temporary_failure"}`), nil
					}
					return jsonResponse(http.StatusOK, `{"allowed":true,"subject_user_id":"user-1","service":"sites","permission_code":"sites-sites-read"}`), nil
				}),
			})),
		}
	}
	defer func() { newAuthzClientFunc = oldFactory }()

	cfg := config.New(config.WithDefaults(map[string]any{
		"PlatformServiceID": "sites",
	}))

	engine := gin.New()
	engine.Use(withVerifiedClaims(auth.Claims{
		TokenUse:   "access",
		IdentityID: "user-1",
	}))
	engine.Use(Middleware(cfg, logger.MustNewDefaultLogger(), NewMiddlewareConfig(cfg).
		WithCategory("sites").
		WithAction("read")))
	engine.GET("/sites", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	first := httptest.NewRecorder()
	engine.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/sites", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d; body=%s", first.Code, http.StatusNoContent, first.Body.String())
	}

	shouldFail.Store(true)

	second := httptest.NewRecorder()
	engine.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/sites", nil))
	if second.Code != http.StatusNoContent {
		t.Fatalf("second status = %d, want %d; body=%s", second.Code, http.StatusNoContent, second.Body.String())
	}
}

func TestMiddlewareMarksCachedDecisionAsCacheHit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var shouldFail atomic.Bool
	oldFactory := newAuthzClientFunc
	newAuthzClientFunc = func(log logger.LogManager, cfg *config.Config) *Client {
		return &Client{
			baseURL: "http://iam.test/control-plane",
			httpClient: httplib.NewClient(httplib.WithHTTPClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if shouldFail.Load() {
						return jsonResponse(http.StatusServiceUnavailable, `{"error":"temporary_failure"}`), nil
					}
					return jsonResponse(http.StatusOK, `{"allowed":true,"subject_user_id":"user-1","service":"sites","permission_code":"sites-sites-read"}`), nil
				}),
			})),
		}
	}
	defer func() { newAuthzClientFunc = oldFactory }()

	cfg := config.New(config.WithDefaults(map[string]any{
		"PlatformServiceID": "sites",
	}))

	engine := gin.New()
	engine.Use(withVerifiedClaims(auth.Claims{
		TokenUse:   "access",
		IdentityID: "user-1",
	}))
	engine.Use(Middleware(cfg, logger.MustNewDefaultLogger(), NewMiddlewareConfig(cfg).
		WithCategory("sites").
		WithAction("read")))
	engine.GET("/sites", func(c *gin.Context) {
		decision, ok := GetDecision(c)
		if !ok {
			t.Fatalf("expected decision in context")
		}
		c.JSON(http.StatusOK, gin.H{"cache_hit": decision.CacheHit})
	})

	first := httptest.NewRecorder()
	engine.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/sites", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d; body=%s", first.Code, http.StatusOK, first.Body.String())
	}
	if strings.Contains(first.Body.String(), `"cache_hit":true`) {
		t.Fatalf("first response unexpectedly reported cache hit: %s", first.Body.String())
	}

	shouldFail.Store(true)

	second := httptest.NewRecorder()
	engine.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/sites", nil))
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d; body=%s", second.Code, http.StatusOK, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), `"cache_hit":true`) {
		t.Fatalf("second response did not report cache hit: %s", second.Body.String())
	}
}

func TestMiddlewareBypassesServiceTokensWhenAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var called atomic.Int32
	oldFactory := newAuthzClientFunc
	newAuthzClientFunc = func(log logger.LogManager, cfg *config.Config) *Client {
		return &Client{
			baseURL: "http://iam.test/control-plane",
			httpClient: httplib.NewClient(httplib.WithHTTPClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					called.Add(1)
					return jsonResponse(http.StatusOK, `{"allowed":true}`), nil
				}),
			})),
		}
	}
	defer func() { newAuthzClientFunc = oldFactory }()

	cfg := config.New(config.WithDefaults(map[string]any{
		"PlatformServiceID": "sites",
	}))

	engine := gin.New()
	engine.Use(withVerifiedClaims(auth.Claims{TokenUse: "service"}))
	engine.Use(Middleware(cfg, logger.MustNewDefaultLogger(), NewMiddlewareConfig(cfg).
		WithCategory("sites").
		WithAction("read")))
	engine.GET("/sites", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sites", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if called.Load() != 0 {
		t.Fatalf("expected authz client not to be called for service token bypass")
	}
}

func TestMiddlewareReadsJSONBodyContextAndPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldFactory := newAuthzClientFunc
	newAuthzClientFunc = func(log logger.LogManager, cfg *config.Config) *Client {
		return &Client{
			baseURL: "http://iam.test/control-plane",
			httpClient: httplib.NewClient(httplib.WithHTTPClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read authz body: %v", err)
					}
					payload := string(body)
					if !strings.Contains(payload, `"plan_tier":"enterprise"`) {
						t.Fatalf("missing plan_tier context in authz request: %s", payload)
					}
					if !strings.Contains(payload, `"admin_user.email":"owner@example.com"`) {
						t.Fatalf("missing nested email context in authz request: %s", payload)
					}
					if !strings.Contains(payload, `"limit":"25"`) {
						t.Fatalf("missing numeric limit context in authz request: %s", payload)
					}
					return jsonResponse(http.StatusOK, `{"allowed":true,"subject_user_id":"user-1","service":"TEN","permission_code":"ten-tenants-create"}`), nil
				}),
			})),
		}
	}
	defer func() { newAuthzClientFunc = oldFactory }()

	cfg := config.New(config.WithDefaults(map[string]any{
		"PlatformServiceID": "TEN",
	}))

	engine := gin.New()
	engine.Use(withVerifiedClaims(auth.Claims{
		TokenUse:   "access",
		IdentityID: "user-1",
	}))
	engine.Use(Middleware(cfg, logger.MustNewDefaultLogger(), NewMiddlewareConfig(cfg).
		WithService("TEN").
		WithCategory("tenants").
		WithAction("create").
		WithResourceType("tenant_collection").
		WithContextResolver(JSONBodyContextResolver("plan_tier", "admin_user.email", "limit"))))
	engine.POST("/tenants", func(c *gin.Context) {
		var payload struct {
			PlanTier  string `json:"plan_tier"`
			Limit     int    `json:"limit"`
			AdminUser struct {
				Email string `json:"email"`
			} `json:"admin_user"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			t.Fatalf("downstream bind failed after authz body read: %v", err)
		}
		if payload.PlanTier != "enterprise" || payload.AdminUser.Email != "owner@example.com" || payload.Limit != 25 {
			t.Fatalf("unexpected downstream payload: %+v", payload)
		}
		c.Status(http.StatusNoContent)
	})

	reqBody, err := json.Marshal(map[string]any{
		"plan_tier": "enterprise",
		"limit":     25,
		"admin_user": map[string]any{
			"email": "owner@example.com",
		},
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tenants", strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestDecisionCacheExpires(t *testing.T) {
	cache := newDecisionCache(10 * time.Millisecond)
	cache.store("key", DecisionResponse{Allowed: true})
	result, ok := cache.lookup("key")
	if !ok {
		t.Fatalf("expected cached decision")
	}
	if !result.CacheHit {
		t.Fatalf("expected cached decision to be marked as cache hit")
	}
	time.Sleep(20 * time.Millisecond)
	if _, ok := cache.lookup("key"); ok {
		t.Fatalf("expected expired cache entry")
	}
}

func TestMethodActionResolverDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPatch, "/resource", nil)

	action := MethodActionResolver(nil)(c, auth.Claims{})
	if action != "update" {
		t.Fatalf("action = %q, want %q", action, "update")
	}
}

func withVerifiedClaims(claims auth.Claims) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(string(auth.CtxAuthClaims), claims)
		c.Request = c.Request.WithContext(auth.ContextWithClaims(c.Request.Context(), claims))
		c.Next()
	}
}
