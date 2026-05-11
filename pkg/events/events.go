package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	coretenant "github.com/milan604/core-lab/pkg/tenant"
)

const (
	DefaultTopic    = "platform.domain.events"
	SchemaVersionV1 = "v1"
)

// Envelope is the canonical business event contract shared across services.
type Envelope struct {
	EventID       string            `json:"event_id"`
	EventType     string            `json:"event_type"`
	EventVersion  string            `json:"event_version"`
	TenantID      string            `json:"tenant_id,omitempty"`
	ResourceType  string            `json:"resource_type,omitempty"`
	ResourceID    string            `json:"resource_id,omitempty"`
	ActorUserID   string            `json:"actor_user_id,omitempty"`
	ServiceID     string            `json:"service_id"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	IsSuperAdmin  bool              `json:"is_super_admin,omitempty"`
	OccurredAt    time.Time         `json:"occurred_at"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Payload       json.RawMessage   `json:"payload,omitempty"`
}

// PublishRequest describes the inputs required to build an event envelope.
type PublishRequest struct {
	ServiceID    string
	EventType    string
	EventVersion string
	TenantID     string
	ResourceType string
	ResourceID   string
	Metadata     map[string]string
	Payload      any
}

// NewEnvelope builds a canonical event envelope using request context fallbacks.
func NewEnvelope(ctx context.Context, req PublishRequest) (Envelope, error) {
	requestContext := coretenant.RequestContext{
		TenantID:  strings.TrimSpace(req.TenantID),
		ServiceID: strings.TrimSpace(req.ServiceID),
	}
	if existing, ok := coretenant.RequestContextFromContext(ctx); ok {
		requestContext = requestContext.WithFallbacks(existing)
	}

	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		return Envelope{}, fmt.Errorf("event_type is required")
	}
	if strings.TrimSpace(requestContext.ServiceID) == "" {
		return Envelope{}, fmt.Errorf("service_id is required")
	}

	payload, err := marshalPayload(req.Payload)
	if err != nil {
		return Envelope{}, err
	}

	metadata := cloneStringMap(req.Metadata)
	metadata = coretenant.MergeMetadata(metadata, requestContext)
	if len(metadata) == 0 {
		metadata = nil
	}

	eventVersion := strings.TrimSpace(req.EventVersion)
	if eventVersion == "" {
		eventVersion = SchemaVersionV1
	}

	return Envelope{
		EventID:       uuid.NewString(),
		EventType:     eventType,
		EventVersion:  eventVersion,
		TenantID:      requestContext.TenantID,
		ResourceType:  strings.TrimSpace(req.ResourceType),
		ResourceID:    strings.TrimSpace(req.ResourceID),
		ActorUserID:   requestContext.ActorUserID,
		ServiceID:     requestContext.ServiceID,
		CorrelationID: requestContext.CorrelationID,
		IsSuperAdmin:  requestContext.IsSuperAdmin,
		OccurredAt:    time.Now().UTC(),
		Metadata:      metadata,
		Payload:       payload,
	}, nil
}

// PartitionKey returns a stable Kafka key preference for tenant-aware ordering.
func (e Envelope) PartitionKey() string {
	if tenantID := strings.TrimSpace(e.TenantID); tenantID != "" {
		return tenantID
	}
	if resourceID := strings.TrimSpace(e.ResourceID); resourceID != "" {
		return resourceID
	}
	return strings.TrimSpace(e.EventType)
}

// JSON serializes the event envelope.
func (e Envelope) JSON() ([]byte, error) {
	return json.Marshal(e)
}

// ParseEnvelope deserializes a platform event from JSON.
func ParseEnvelope(data []byte) (Envelope, error) {
	var envelope Envelope
	err := json.Unmarshal(data, &envelope)
	return envelope, err
}

func marshalPayload(payload any) (json.RawMessage, error) {
	if payload == nil {
		return nil, nil
	}

	switch typed := payload.(type) {
	case json.RawMessage:
		if len(typed) == 0 {
			return nil, nil
		}
		return typed, nil
	case []byte:
		if len(typed) == 0 {
			return nil, nil
		}
		return json.RawMessage(typed), nil
	default:
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal event payload: %w", err)
		}
		if len(data) == 0 || string(data) == "null" {
			return nil, nil
		}
		return json.RawMessage(data), nil
	}
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(input))
	for k, v := range input {
		cloned[k] = v
	}
	return cloned
}
