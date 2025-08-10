package server

import (
	"context"

	"github.com/milan604/core-lab/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// default header names
const (
	HeaderRequestID = "X-Request-ID"
)

type RequestIDConfig struct {
	HeaderName string
	// If true, accept incoming request id header; otherwise always generate new
	AllowIncoming bool
}

func defaultRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{
		HeaderName:    HeaderRequestID,
		AllowIncoming: true,
	}
}

// RequestIDMiddleware returns a Gin middleware that injects request id in context and header
func RequestIDMiddleware(opts ...RequestIDConfig) gin.HandlerFunc {
	cfg := defaultRequestIDConfig()
	if len(opts) > 0 {
		cfg = opts[0]
	}

	return func(c *gin.Context) {
		var reqID string
		if cfg.AllowIncoming {
			reqID = c.GetHeader(cfg.HeaderName)
		}
		if reqID == "" {
			reqID = uuid.New().String()
		}
		// put into gin context and request context
		c.Set(string(logger.RequestIDKey), reqID)
		// also add to request.Context so other libs can extract
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), logger.RequestIDKey, reqID))
		// set header for downstream visibility
		c.Writer.Header().Set(cfg.HeaderName, reqID)
		c.Next()
	}
}
