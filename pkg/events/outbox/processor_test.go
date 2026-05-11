package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	coreevents "github.com/milan604/core-lab/pkg/events"
)

type fakeStore struct {
	claimed         []Record
	delivered       []string
	failed          []string
	failedNextTimes []time.Time
}

func (f *fakeStore) ClaimBatch(_ context.Context, _ string, _ int, _ time.Duration, _ time.Time) ([]Record, error) {
	return append([]Record(nil), f.claimed...), nil
}

func (f *fakeStore) MarkDelivered(_ context.Context, recordID string, _ time.Time) error {
	f.delivered = append(f.delivered, recordID)
	return nil
}

func (f *fakeStore) MarkFailed(_ context.Context, recordID string, nextAttemptAt time.Time, _ string, _ time.Time) error {
	f.failed = append(f.failed, recordID)
	f.failedNextTimes = append(f.failedNextTimes, nextAttemptAt)
	return nil
}

type fakePublisher struct {
	err error
}

func (f fakePublisher) Publish(_ context.Context, _ string, _ coreevents.Envelope) error {
	return f.err
}

func TestProcessorMarksDeliveredOnPublishSuccess(t *testing.T) {
	store := &fakeStore{
		claimed: []Record{{
			ID:       "evt-1",
			Topic:    coreevents.DefaultTopic,
			Envelope: coreevents.Envelope{EventType: "subscription.created"},
		}},
	}
	processor := NewProcessor(store, fakePublisher{}, ProcessorOptions{
		Name:  "test-processor",
		Clock: func() time.Time { return time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC) },
	})

	if err := processor.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if len(store.delivered) != 1 || store.delivered[0] != "evt-1" {
		t.Fatalf("expected delivered evt-1, got %#v", store.delivered)
	}
	if len(store.failed) != 0 {
		t.Fatalf("expected no failures, got %#v", store.failed)
	}
}

func TestProcessorMarksFailedOnPublishError(t *testing.T) {
	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimed: []Record{{
			ID:       "evt-2",
			Topic:    coreevents.DefaultTopic,
			Attempts: 1,
			Envelope: coreevents.Envelope{EventType: "subscription.updated"},
		}},
	}
	processor := NewProcessor(store, fakePublisher{err: errors.New("broker unavailable")}, ProcessorOptions{
		Name:            "test-processor",
		Clock:           func() time.Time { return now },
		MinRetryBackoff: 5 * time.Second,
		MaxRetryBackoff: time.Minute,
	})

	if err := processor.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if len(store.failed) != 1 || store.failed[0] != "evt-2" {
		t.Fatalf("expected failed evt-2, got %#v", store.failed)
	}
	if len(store.failedNextTimes) != 1 {
		t.Fatalf("expected one retry time, got %#v", store.failedNextTimes)
	}
	expectedNext := now.Add(10 * time.Second)
	if !store.failedNextTimes[0].Equal(expectedNext) {
		t.Fatalf("expected next attempt %s, got %s", expectedNext, store.failedNextTimes[0])
	}
}
