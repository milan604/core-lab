package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/milan604/core-lab/pkg/auth"
	"github.com/milan604/core-lab/pkg/logger"
)

func TestManagerProcessesJobAndStats(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.RegisterHandler("email.send", func(ctx context.Context, job Job) (any, error) {
		var payload map[string]any
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return nil, err
		}
		return map[string]any{
			"message_id": "msg-123",
			"to":         payload["to"],
		}, nil
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer manager.Stop(context.Background())

	job, err := manager.Enqueue(context.Background(), EnqueueRequest{
		Type:    "email.send",
		Payload: json.RawMessage(`{"to":"user@example.com"}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	finalJob := waitForStatus(t, manager, job.ID, StatusSucceeded)
	if finalJob.Attempt != 1 {
		t.Fatalf("expected 1 attempt, got %d", finalJob.Attempt)
	}
	if len(finalJob.Result) == 0 {
		t.Fatalf("expected result payload")
	}

	stats, err := manager.Stats(context.Background())
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Totals[StatusSucceeded] != 1 {
		t.Fatalf("expected succeeded=1, got %d", stats.Totals[StatusSucceeded])
	}
	if stats.JobsStored != 1 {
		t.Fatalf("expected jobs stored=1, got %d", stats.JobsStored)
	}
}

func TestManagerRetriesAndEventuallyFails(t *testing.T) {
	manager := newTestManager(t)

	var attempts atomic.Int32
	if err := manager.RegisterHandler(
		"webhook.deliver",
		func(ctx context.Context, job Job) (any, error) {
			attempts.Add(1)
			return nil, errors.New("upstream unavailable")
		},
		WithHandlerMaxAttempts(2),
	); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer manager.Stop(context.Background())

	job, err := manager.Enqueue(context.Background(), EnqueueRequest{
		Type: "webhook.deliver",
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	finalJob := waitForStatus(t, manager, job.ID, StatusFailed)
	if finalJob.Attempt != 2 {
		t.Fatalf("expected 2 attempts, got %d", finalJob.Attempt)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected handler to run twice, got %d", attempts.Load())
	}
}

func TestManagerDelayedJob(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.RegisterHandler("report.generate", func(ctx context.Context, job Job) (any, error) {
		return map[string]string{"status": "ready"}, nil
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("start manager: %v", err)
	}
	defer manager.Stop(context.Background())

	runAfter := time.Now().UTC().Add(60 * time.Millisecond)
	job, err := manager.Enqueue(context.Background(), EnqueueRequest{
		Type:     "report.generate",
		RunAfter: &runAfter,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	time.Sleep(25 * time.Millisecond)
	current, err := manager.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if current.Status != StatusScheduled && current.Status != StatusQueued {
		t.Fatalf("expected scheduled or queued before due time, got %s", current.Status)
	}

	waitForStatus(t, manager, job.ID, StatusSucceeded)
}

func TestManagerEnqueueAddsCanonicalRequestMetadata(t *testing.T) {
	manager := newTestManager(t)
	ctx := auth.ContextWithCorrelationID(
		auth.ContextWithServiceID(
			auth.ContextWithUserID(
				auth.ContextWithTenantID(context.Background(), "tenant-1"),
				"user-1",
			),
			"sites-service",
		),
		"req-1",
	)
	ctx = auth.ContextWithSuperAdmin(ctx, true)
	ctx = context.WithValue(ctx, logger.RequestIDKey, "req-1")

	job, err := manager.Enqueue(ctx, EnqueueRequest{
		Type: "demo.job",
	})
	if err == nil {
		t.Fatalf("expected unregistered job type error, got nil")
	}

	manager.cfg.AllowEnqueueWithoutHandler = true
	job, err = manager.Enqueue(ctx, EnqueueRequest{
		Type: "demo.job",
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if got := job.Metadata["tenant_id"]; got != "tenant-1" {
		t.Fatalf("tenant_id metadata = %q, want tenant-1", got)
	}
	if got := job.Metadata["actor_user_id"]; got != "user-1" {
		t.Fatalf("actor_user_id metadata = %q, want user-1", got)
	}
	if got := job.Metadata["service_id"]; got != "sites-service" {
		t.Fatalf("service_id metadata = %q, want sites-service", got)
	}
	if got := job.Metadata["source"]; got != "sites-service" {
		t.Fatalf("source metadata = %q, want sites-service", got)
	}
	if got := job.Metadata["correlation_id"]; got != "req-1" {
		t.Fatalf("correlation_id metadata = %q, want req-1", got)
	}
	if got := job.Metadata["is_super_admin"]; got != "true" {
		t.Fatalf("is_super_admin metadata = %q, want true", got)
	}
	if got := job.Metadata["initiator"]; got != "user:user-1" {
		t.Fatalf("initiator metadata = %q, want user:user-1", got)
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()

	manager, err := NewManager(Config{
		Name:               "test-jobs",
		Workers:            2,
		QueueBuffer:        16,
		ClaimInterval:      5 * time.Millisecond,
		CleanupInterval:    30 * time.Second,
		Retention:          time.Minute,
		DefaultMaxAttempts: 2,
		DefaultTimeout:     100 * time.Millisecond,
		RetryBaseDelay:     10 * time.Millisecond,
		RetryMaxDelay:      20 * time.Millisecond,
	}, NewMemoryStore())
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return manager
}

func waitForStatus(t *testing.T, manager *Manager, id string, want Status) Job {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := manager.GetJob(context.Background(), id)
		if err == nil && job.Status == want {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}

	job, err := manager.GetJob(context.Background(), id)
	if err != nil {
		t.Fatalf("job %s not found: %v", id, err)
	}
	t.Fatalf("job %s did not reach status %s, current status=%s", id, want, job.Status)
	return Job{}
}
