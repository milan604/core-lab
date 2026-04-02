package tenant

import (
	"context"
	"strconv"
	"strings"
)

type requestContextKey string

const (
	requestContextContextKey requestContextKey = "corelab_tenant_request_context"
)

const (
	MetadataTenantID      = "tenant_id"
	MetadataActorUserID   = "actor_user_id"
	MetadataServiceID     = "service_id"
	MetadataCorrelationID = "correlation_id"
	MetadataIsSuperAdmin  = "is_super_admin"
	MetadataInitiator     = "initiator"
	MetadataSource        = "source"
)

// RequestContext is the canonical tenant-aware request contract shared across
// services, background jobs, and audit/event flows.
type RequestContext struct {
	TenantID      string `json:"tenant_id,omitempty"`
	ActorUserID   string `json:"actor_user_id,omitempty"`
	ServiceID     string `json:"service_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	IsSuperAdmin  bool   `json:"is_super_admin,omitempty"`
}

func (rc RequestContext) Normalize() RequestContext {
	rc.TenantID = strings.TrimSpace(rc.TenantID)
	rc.ActorUserID = strings.TrimSpace(rc.ActorUserID)
	rc.ServiceID = strings.TrimSpace(rc.ServiceID)
	rc.CorrelationID = strings.TrimSpace(rc.CorrelationID)
	return rc
}

func (rc RequestContext) IsEmpty() bool {
	normalized := rc.Normalize()
	return normalized.TenantID == "" &&
		normalized.ActorUserID == "" &&
		normalized.ServiceID == "" &&
		normalized.CorrelationID == "" &&
		!normalized.IsSuperAdmin
}

func (rc RequestContext) WithFallbacks(other RequestContext) RequestContext {
	rc = rc.Normalize()
	other = other.Normalize()

	if rc.TenantID == "" {
		rc.TenantID = other.TenantID
	}
	if rc.ActorUserID == "" {
		rc.ActorUserID = other.ActorUserID
	}
	if rc.ServiceID == "" {
		rc.ServiceID = other.ServiceID
	}
	if rc.CorrelationID == "" {
		rc.CorrelationID = other.CorrelationID
	}
	if !rc.IsSuperAdmin {
		rc.IsSuperAdmin = other.IsSuperAdmin
	}
	return rc
}

func ContextWithRequestContext(ctx context.Context, rc RequestContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	rc = rc.Normalize()
	if rc.IsEmpty() {
		return ctx
	}
	return context.WithValue(ctx, requestContextContextKey, rc)
}

func RequestContextFromContext(ctx context.Context) (RequestContext, bool) {
	if ctx == nil {
		return RequestContext{}, false
	}
	val := ctx.Value(requestContextContextKey)
	rc, ok := val.(RequestContext)
	if !ok {
		return RequestContext{}, false
	}
	rc = rc.Normalize()
	if rc.IsEmpty() {
		return RequestContext{}, false
	}
	return rc, true
}

func MergeMetadata(metadata map[string]string, rc RequestContext) map[string]string {
	rc = rc.Normalize()
	if metadata == nil {
		metadata = make(map[string]string)
	}

	setIfEmpty := func(key, value string) {
		if strings.TrimSpace(metadata[key]) != "" {
			return
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		metadata[key] = value
	}

	setIfEmpty(MetadataTenantID, rc.TenantID)
	setIfEmpty(MetadataActorUserID, rc.ActorUserID)
	setIfEmpty(MetadataServiceID, rc.ServiceID)
	setIfEmpty(MetadataCorrelationID, rc.CorrelationID)
	if _, exists := metadata[MetadataIsSuperAdmin]; !exists && rc.IsSuperAdmin {
		metadata[MetadataIsSuperAdmin] = strconv.FormatBool(true)
	}
	if strings.TrimSpace(metadata[MetadataSource]) == "" && rc.ServiceID != "" {
		metadata[MetadataSource] = rc.ServiceID
	}
	if strings.TrimSpace(metadata[MetadataInitiator]) == "" {
		switch {
		case rc.ActorUserID != "":
			metadata[MetadataInitiator] = "user:" + rc.ActorUserID
		case rc.ServiceID != "":
			metadata[MetadataInitiator] = "service:" + rc.ServiceID
		case rc.IsSuperAdmin:
			metadata[MetadataInitiator] = "super-admin"
		}
	}
	return metadata
}
