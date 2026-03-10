package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type capturePublisher struct {
	events []Event
}

func (p *capturePublisher) Publish(_ context.Context, event Event) error {
	p.events = append(p.events, event)
	return nil
}

func (p *capturePublisher) Close() error {
	return nil
}

func TestMiddlewareAuditsMutatingRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	publisher := &capturePublisher{}
	engine := gin.New()
	engine.Use(Middleware(MiddlewareConfig{
		Enabled:   true,
		Service:   "sites-service",
		Publisher: publisher,
		Methods:   defaultAuditedMethods,
	}))
	engine.POST("/sites/:site_id", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/sites/site-123", nil)
	req.Header.Set("X-Request-ID", "req-1")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.Code)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(publisher.events))
	}

	event := publisher.events[0]
	if event.Service != "sites-service" {
		t.Fatalf("expected service sites-service, got %q", event.Service)
	}
	if event.Action != "sites.create" {
		t.Fatalf("expected action sites.create, got %q", event.Action)
	}
	if event.Resource != "sites" {
		t.Fatalf("expected resource sites, got %q", event.Resource)
	}
	if event.ResourceID != "site-123" {
		t.Fatalf("expected resource ID site-123, got %q", event.ResourceID)
	}
	if event.Status != "success" {
		t.Fatalf("expected status success, got %q", event.Status)
	}
	if got := event.Metadata["http_route"]; got != "/sites/:site_id" {
		t.Fatalf("expected route metadata /sites/:site_id, got %#v", got)
	}
}

func TestMiddlewareSkipsQuotaCheckByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	publisher := &capturePublisher{}
	engine := gin.New()
	engine.Use(Middleware(MiddlewareConfig{
		Enabled:   true,
		Service:   "sentinel",
		Publisher: publisher,
		Methods:   defaultAuditedMethods,
	}))
	engine.POST("/internal/api/v1/quota/check", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/internal/api/v1/quota/check", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if len(publisher.events) != 0 {
		t.Fatalf("expected quota check to be skipped, got %d events", len(publisher.events))
	}
}

func TestMiddlewareCanForceAuditForStateChangingGET(t *testing.T) {
	gin.SetMode(gin.TestMode)

	publisher := &capturePublisher{}
	engine := gin.New()
	engine.Use(Middleware(MiddlewareConfig{
		Enabled:   true,
		Service:   "sentinel",
		Publisher: publisher,
		Methods:   defaultAuditedMethods,
	}))
	engine.GET("/api/v1/auth/oauth/:provider/callback", func(c *gin.Context) {
		ForceRequest(c)
		SetAction(c, "auth.oauth_callback")
		SetResource(c, "auth_session")
		AddMetadata(c, "oauth_provider", c.Param("provider"))
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/google/callback", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if len(publisher.events) != 1 {
		t.Fatalf("expected forced GET to emit 1 audit event, got %d", len(publisher.events))
	}

	event := publisher.events[0]
	if event.Action != "auth.oauth_callback" {
		t.Fatalf("expected action auth.oauth_callback, got %q", event.Action)
	}
	if event.Resource != "auth_session" {
		t.Fatalf("expected resource auth_session, got %q", event.Resource)
	}
	if got := event.Metadata["oauth_provider"]; got != "google" {
		t.Fatalf("expected oauth_provider metadata google, got %#v", got)
	}
}
