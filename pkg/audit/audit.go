// Package audit provides a structured audit event model and publisher interface
// for cross-service audit logging via a message bus (e.g., Kafka).
//
// Services should implement the Publisher interface using their own transport
// (Kafka, NATS, etc.) and publish events for every state-changing operation.
package audit

import (
	"context"
	"time"
)

// Event represents a structured audit record emitted by any service.
type Event struct {
	// EventID is a unique identifier for this audit event (UUID).
	EventID string `json:"event_id"`
	// Timestamp is when the event occurred (UTC).
	Timestamp time.Time `json:"timestamp"`
	// Service is the originating service name (e.g., "sentinel", "subscription-service").
	Service string `json:"service"`
	// TenantID is the tenant scope. Empty for platform-level operations.
	TenantID string `json:"tenant_id,omitempty"`
	// UserID is the acting user. Empty for system/service-token operations.
	UserID string `json:"user_id,omitempty"`
	// Action describes what happened (e.g., "user.login", "plan.create", "site.publish").
	Action string `json:"action"`
	// Resource is the type of entity affected (e.g., "user", "plan", "tenant_site").
	Resource string `json:"resource"`
	// ResourceID is the primary key of the affected entity.
	ResourceID string `json:"resource_id,omitempty"`
	// RequestID is the X-Request-ID for distributed tracing correlation.
	RequestID string `json:"request_id,omitempty"`
	// IPAddress is the client IP that initiated the request.
	IPAddress string `json:"ip_address,omitempty"`
	// Status is the outcome (e.g., "success", "failure", "denied").
	Status string `json:"status"`
	// Metadata holds additional context specific to the action.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Publisher is the interface that services implement to emit audit events.
type Publisher interface {
	// Publish sends an audit event asynchronously. Implementations should be non-blocking
	// and handle errors internally (log + discard) to avoid impacting request latency.
	Publish(ctx context.Context, event Event) error
	// Close gracefully shuts down the publisher, flushing pending events.
	Close() error
}

// NoopPublisher is a Publisher that discards all events. Use for testing or
// when audit logging is disabled.
type NoopPublisher struct{}

// Publish discards the event.
func (NoopPublisher) Publish(_ context.Context, _ Event) error { return nil }

// Close is a no-op.
func (NoopPublisher) Close() error { return nil }

// DefaultTopic is the recommended Kafka topic for audit events.
const DefaultTopic = "platform.audit.events"
