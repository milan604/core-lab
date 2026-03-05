package audit

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/milan604/core-lab/pkg/logger"
)

// NewEvent creates an Event pre-populated from the Gin context.
// It extracts tenant_id, user_id, request_id, and client IP automatically.
func NewEvent(c *gin.Context, service, action, resource, resourceID, status string) Event {
	ev := Event{
		EventID:    uuid.New().String(),
		Timestamp:  time.Now().UTC(),
		Service:    service,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Status:     status,
		IPAddress:  c.ClientIP(),
	}

	if tid, ok := c.Get("tenant_id"); ok {
		ev.TenantID, _ = tid.(string)
	}
	if uid, ok := c.Get("user_id"); ok {
		ev.UserID, _ = uid.(string)
	}
	if rid := c.Value(logger.RequestIDKey); rid != nil {
		ev.RequestID, _ = rid.(string)
	}
	// Also check the header directly as a fallback
	if ev.RequestID == "" {
		ev.RequestID = c.GetHeader("X-Request-ID")
	}

	return ev
}
