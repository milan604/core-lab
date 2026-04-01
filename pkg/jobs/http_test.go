package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminRoutesExposeJobLifecycle(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.RegisterHandler("site.publish", func(ctx context.Context, job Job) (any, error) {
		return map[string]string{"published": "true"}, nil
	}, WithHandlerDescription("Publish a site version")); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer manager.Stop(context.Background())

	engine := NewAdminEngine(manager)

	req := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewBufferString(`{"type":"site.publish","payload":{"site_id":"site-1"}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", resp.Code, resp.Body.String())
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    Job  `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode enqueue response: %v", err)
	}
	if !envelope.Success {
		t.Fatalf("expected success response")
	}

	waitForStatus(t, manager, envelope.Data.ID, StatusSucceeded)

	resp = httptest.NewRecorder()
	engine.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/jobs/"+envelope.Data.ID, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 on get job, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	engine.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/stats", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 on stats, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	engine.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/handlers", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 on handlers, got %d", resp.Code)
	}
}
