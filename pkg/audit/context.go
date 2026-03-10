package audit

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ctxAuditActionKey     = "corelab_audit_action"
	ctxAuditResourceKey   = "corelab_audit_resource"
	ctxAuditResourceIDKey = "corelab_audit_resource_id"
	ctxAuditStatusKey     = "corelab_audit_status"
	ctxAuditMetadataKey   = "corelab_audit_metadata"
	ctxAuditForceKey      = "corelab_audit_force"
	ctxAuditSkipKey       = "corelab_audit_skip"
)

func SetAction(c *gin.Context, action string) {
	setTrimmedString(c, ctxAuditActionKey, action)
}

func SetResource(c *gin.Context, resource string) {
	setTrimmedString(c, ctxAuditResourceKey, resource)
}

func SetResourceID(c *gin.Context, resourceID string) {
	setTrimmedString(c, ctxAuditResourceIDKey, resourceID)
}

func SetStatus(c *gin.Context, status string) {
	setTrimmedString(c, ctxAuditStatusKey, status)
}

func SetMetadata(c *gin.Context, metadata map[string]any) {
	if c == nil {
		return
	}
	if metadata == nil {
		c.Set(ctxAuditMetadataKey, map[string]any{})
		return
	}

	cloned := make(map[string]any, len(metadata))
	for k, v := range metadata {
		cloned[k] = v
	}
	c.Set(ctxAuditMetadataKey, cloned)
}

func AddMetadata(c *gin.Context, key string, value any) {
	if c == nil {
		return
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}

	metadata := getMetadata(c)
	metadata[key] = value
	c.Set(ctxAuditMetadataKey, metadata)
}

func ForceRequest(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(ctxAuditForceKey, true)
}

func SkipRequest(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(ctxAuditSkipKey, true)
}

func getAction(c *gin.Context) string {
	return getStringValue(c, ctxAuditActionKey)
}

func getResource(c *gin.Context) string {
	return getStringValue(c, ctxAuditResourceKey)
}

func getResourceID(c *gin.Context) string {
	return getStringValue(c, ctxAuditResourceIDKey)
}

func getStatus(c *gin.Context) string {
	return getStringValue(c, ctxAuditStatusKey)
}

func getMetadata(c *gin.Context) map[string]any {
	if c == nil {
		return map[string]any{}
	}

	val, ok := c.Get(ctxAuditMetadataKey)
	if !ok {
		return map[string]any{}
	}

	metadata, ok := val.(map[string]any)
	if !ok || metadata == nil {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(metadata))
	for k, v := range metadata {
		cloned[k] = v
	}
	return cloned
}

func isForced(c *gin.Context) bool {
	return getBoolValue(c, ctxAuditForceKey)
}

func isSkipped(c *gin.Context) bool {
	return getBoolValue(c, ctxAuditSkipKey)
}

func setTrimmedString(c *gin.Context, key, value string) {
	if c == nil {
		return
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	c.Set(key, value)
}

func getStringValue(c *gin.Context, key string) string {
	if c == nil {
		return ""
	}
	val, ok := c.Get(key)
	if !ok {
		return ""
	}
	value, _ := val.(string)
	return strings.TrimSpace(value)
}

func getBoolValue(c *gin.Context, key string) bool {
	if c == nil {
		return false
	}
	val, ok := c.Get(key)
	if !ok {
		return false
	}
	flag, _ := val.(bool)
	return flag
}
