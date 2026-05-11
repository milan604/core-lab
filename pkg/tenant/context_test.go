package tenant

import (
	"context"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	ctx := ContextWithRequestContext(context.Background(), RequestContext{
		TenantID:      "tenant-1",
		ActorUserID:   "user-1",
		ServiceID:     "sites-service",
		CorrelationID: "req-1",
		IsSuperAdmin:  true,
	})

	got, ok := RequestContextFromContext(ctx)
	if !ok {
		t.Fatalf("expected request context to be present")
	}
	if got.TenantID != "tenant-1" {
		t.Fatalf("TenantID = %q, want tenant-1", got.TenantID)
	}
	if got.ActorUserID != "user-1" {
		t.Fatalf("ActorUserID = %q, want user-1", got.ActorUserID)
	}
	if got.ServiceID != "sites-service" {
		t.Fatalf("ServiceID = %q, want sites-service", got.ServiceID)
	}
	if got.CorrelationID != "req-1" {
		t.Fatalf("CorrelationID = %q, want req-1", got.CorrelationID)
	}
	if !got.IsSuperAdmin {
		t.Fatalf("expected IsSuperAdmin = true")
	}
}

func TestMergeMetadataAddsCanonicalFields(t *testing.T) {
	metadata := MergeMetadata(map[string]string{
		"existing": "value",
	}, RequestContext{
		TenantID:      "tenant-1",
		ActorUserID:   "user-1",
		ServiceID:     "notification-service",
		CorrelationID: "req-1",
		IsSuperAdmin:  true,
	})

	if metadata[MetadataTenantID] != "tenant-1" {
		t.Fatalf("tenant metadata = %q, want tenant-1", metadata[MetadataTenantID])
	}
	if metadata[MetadataActorUserID] != "user-1" {
		t.Fatalf("actor metadata = %q, want user-1", metadata[MetadataActorUserID])
	}
	if metadata[MetadataServiceID] != "notification-service" {
		t.Fatalf("service metadata = %q, want notification-service", metadata[MetadataServiceID])
	}
	if metadata[MetadataCorrelationID] != "req-1" {
		t.Fatalf("correlation metadata = %q, want req-1", metadata[MetadataCorrelationID])
	}
	if metadata[MetadataIsSuperAdmin] != "true" {
		t.Fatalf("is_super_admin metadata = %q, want true", metadata[MetadataIsSuperAdmin])
	}
	if metadata[MetadataSource] != "notification-service" {
		t.Fatalf("source metadata = %q, want notification-service", metadata[MetadataSource])
	}
	if metadata[MetadataInitiator] != "user:user-1" {
		t.Fatalf("initiator metadata = %q, want user:user-1", metadata[MetadataInitiator])
	}
}
