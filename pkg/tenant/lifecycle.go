package tenant

import (
	"encoding/json"
	"time"
)

// Kafka topic for tenant lifecycle events.
const TopicTenantLifecycle = "platform.tenant.lifecycle"

// Event type constants.
const (
	EventTenantSuspended   = "tenant.suspended"
	EventTenantReactivated = "tenant.reactivated"
	EventTenantDeleted     = "tenant.deleted"
	EventTenantExportReq   = "tenant.export_requested"
	EventTenantExportDone  = "tenant.export_completed"
)

// LifecycleEvent is the envelope published to TopicTenantLifecycle.
type LifecycleEvent struct {
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"`
	TenantID  string                 `json:"tenant_id"`
	Timestamp time.Time              `json:"timestamp"`
	Actor     string                 `json:"actor,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

// JSON serialises the event.
func (e LifecycleEvent) JSON() ([]byte, error) {
	return json.Marshal(e)
}

// ParseLifecycleEvent deserialises a lifecycle event from JSON.
func ParseLifecycleEvent(data []byte) (LifecycleEvent, error) {
	var e LifecycleEvent
	err := json.Unmarshal(data, &e)
	return e, err
}

// ExportManifest describes the data a service prepared for an export request.
type ExportManifest struct {
	Service     string    `json:"service"`
	TenantID    string    `json:"tenant_id"`
	Files       []string  `json:"files"`
	RecordCount int       `json:"record_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// DeletionConfirmation is returned by downstream services after
// they have purged all tenant data.
type DeletionConfirmation struct {
	Service        string    `json:"service"`
	TenantID       string    `json:"tenant_id"`
	RecordsRemoved int       `json:"records_removed"`
	CompletedAt    time.Time `json:"completed_at"`
}
