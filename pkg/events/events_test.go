package events

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/milan604/core-lab/pkg/auth"
	coretenant "github.com/milan604/core-lab/pkg/tenant"
)

func TestNewEnvelopeUsesRequestContextFallbacks(t *testing.T) {
	ctx := context.Background()
	ctx = auth.ContextWithTenantID(ctx, "tenant-1")
	ctx = auth.ContextWithUserID(ctx, "user-1")
	ctx = auth.ContextWithCorrelationID(ctx, "corr-1")
	ctx = auth.ContextWithSuperAdmin(ctx, true)

	envelope, err := NewEnvelope(ctx, PublishRequest{
		ServiceID:    "subscription-service",
		EventType:    "subscription.created",
		ResourceType: "subscription",
		ResourceID:   "sub-1",
		Metadata: map[string]string{
			"custom": "value",
		},
		Payload: map[string]any{
			"status": "active",
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope returned error: %v", err)
	}

	if envelope.ServiceID != "subscription-service" {
		t.Fatalf("unexpected service id: %s", envelope.ServiceID)
	}
	if envelope.TenantID != "tenant-1" {
		t.Fatalf("unexpected tenant id: %s", envelope.TenantID)
	}
	if envelope.ActorUserID != "user-1" {
		t.Fatalf("unexpected actor user id: %s", envelope.ActorUserID)
	}
	if envelope.CorrelationID != "corr-1" {
		t.Fatalf("unexpected correlation id: %s", envelope.CorrelationID)
	}
	if !envelope.IsSuperAdmin {
		t.Fatalf("expected super admin flag")
	}
	if envelope.Metadata["custom"] != "value" {
		t.Fatalf("expected custom metadata to survive merge")
	}
	if envelope.Metadata[coretenant.MetadataTenantID] != "tenant-1" {
		t.Fatalf("expected tenant metadata to be merged")
	}
	if envelope.Metadata[coretenant.MetadataInitiator] != "user:user-1" {
		t.Fatalf("unexpected initiator metadata: %s", envelope.Metadata[coretenant.MetadataInitiator])
	}

	var payload map[string]any
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if payload["status"] != "active" {
		t.Fatalf("unexpected payload status: %#v", payload["status"])
	}
}

func TestNewEnvelopeRequiresEventTypeAndServiceID(t *testing.T) {
	if _, err := NewEnvelope(context.Background(), PublishRequest{}); err == nil {
		t.Fatalf("expected missing event type to fail")
	}

	if _, err := NewEnvelope(context.Background(), PublishRequest{
		EventType: "tenant.updated",
	}); err == nil {
		t.Fatalf("expected missing service id to fail")
	}
}

func TestPartitionKeyPrefersTenantThenResourceThenEventType(t *testing.T) {
	event := Envelope{TenantID: "tenant-1", ResourceID: "resource-1", EventType: "tenant.updated"}
	if key := event.PartitionKey(); key != "tenant-1" {
		t.Fatalf("unexpected tenant partition key: %s", key)
	}

	event = Envelope{ResourceID: "resource-1", EventType: "tenant.updated"}
	if key := event.PartitionKey(); key != "resource-1" {
		t.Fatalf("unexpected resource partition key: %s", key)
	}

	event = Envelope{EventType: "tenant.updated"}
	if key := event.PartitionKey(); key != "tenant.updated" {
		t.Fatalf("unexpected fallback partition key: %s", key)
	}
}
