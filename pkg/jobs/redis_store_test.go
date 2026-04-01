package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisStoreSharedVisibilityAcrossManagers(t *testing.T) {
	t.Parallel()

	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mini.Close()

	ctx := context.Background()

	storeA, err := NewRedisStoreFromConfig(ctx, RedisStoreConfig{
		Address:   mini.Addr(),
		Namespace: "test-shared-jobs",
	})
	if err != nil {
		t.Fatalf("failed to create store A: %v", err)
	}
	defer storeA.Close()

	storeB, err := NewRedisStoreFromConfig(ctx, RedisStoreConfig{
		Address:   mini.Addr(),
		Namespace: "test-shared-jobs",
	})
	if err != nil {
		t.Fatalf("failed to create store B: %v", err)
	}
	defer storeB.Close()

	managerA, err := NewManager(Config{
		Name:                       "svc-a",
		Workers:                    1,
		QueueBuffer:                8,
		AllowEnqueueWithoutHandler: true,
	}, storeA)
	if err != nil {
		t.Fatalf("failed to create manager A: %v", err)
	}

	managerB, err := NewManager(Config{
		Name:                       "svc-b",
		Workers:                    1,
		QueueBuffer:                8,
		AllowEnqueueWithoutHandler: true,
	}, storeB)
	if err != nil {
		t.Fatalf("failed to create manager B: %v", err)
	}

	job, err := managerA.Enqueue(ctx, EnqueueRequest{
		Type: "demo.shared",
		Metadata: map[string]string{
			"source": "service-a",
		},
	})
	if err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	jobs, err := managerB.ListJobs(ctx, JobFilter{Limit: 10})
	if err != nil {
		t.Fatalf("failed to list jobs from manager B: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 shared job, got %d", len(jobs))
	}
	if jobs[0].ID != job.ID {
		t.Fatalf("expected shared job id %s, got %s", job.ID, jobs[0].ID)
	}
	if jobs[0].Metadata["source"] != "service-a" {
		t.Fatalf("expected source metadata to survive, got %#v", jobs[0].Metadata)
	}
	if jobs[0].Metadata["job_manager"] != "svc-a" {
		t.Fatalf("expected job_manager metadata to be stamped, got %#v", jobs[0].Metadata)
	}
}

func TestRedisStoreClaimAndLifecycle(t *testing.T) {
	t.Parallel()

	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mini.Close()

	ctx := context.Background()
	store, err := NewRedisStoreFromConfig(ctx, RedisStoreConfig{
		Address:   mini.Addr(),
		Namespace: "test-lifecycle-jobs",
	})
	if err != nil {
		t.Fatalf("failed to create redis store: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	job := Job{
		ID:          "job-1",
		Type:        "demo.lifecycle",
		Queue:       "default",
		Status:      StatusQueued,
		MaxAttempts: 3,
		Timeout:     Duration(10 * time.Second),
		CreatedAt:   now,
		UpdatedAt:   now,
		AvailableAt: now,
	}

	if _, err := store.Create(ctx, job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	claimed, err := store.ClaimReady(ctx, now, 1)
	if err != nil {
		t.Fatalf("failed to claim jobs: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected 1 claimed job, got %d", len(claimed))
	}
	if claimed[0].Status != StatusRunning {
		t.Fatalf("expected running status after claim, got %s", claimed[0].Status)
	}
	if claimed[0].Attempt != 1 {
		t.Fatalf("expected attempt 1 after claim, got %d", claimed[0].Attempt)
	}

	failedAt := now.Add(2 * time.Second)
	if _, err := store.MarkFailed(ctx, job.ID, "boom", failedAt); err != nil {
		t.Fatalf("failed to mark job failed: %v", err)
	}

	retried, err := store.Retry(ctx, job.ID, failedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("failed to retry job: %v", err)
	}
	if retried.Status != StatusQueued {
		t.Fatalf("expected queued status after retry, got %s", retried.Status)
	}

	if _, err := store.Cancel(ctx, job.ID, failedAt.Add(2*time.Second)); err != nil {
		t.Fatalf("failed to cancel retried job: %v", err)
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.Totals[StatusCanceled] != 1 {
		t.Fatalf("expected 1 canceled job in stats, got %#v", stats.Totals)
	}
}
