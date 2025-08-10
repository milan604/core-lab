package server

import (
	"time"

	"github.com/milan604/core-lab/pkg/logger"

	"github.com/gin-gonic/gin"
)

const loggerKey = "corelab_logger"

// AppLoggerMiddleware injects a request-scoped logger into gin.Context
func AppLoggerMiddleware(l logger.LogManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// create request-scoped logger with request id
		reqLogger := l.With("log_type", "application", "source", c.HandlerName(), "service_name", c.FullPath())
		// store in gin context for handlers to retrieve
		c.Set(loggerKey, reqLogger)
		c.Next()
	}
}

// GetLogger retrieves the request-scoped logger from context (fallback to a global)
func GetLogger(c *gin.Context) logger.LogManager {
	if val, ok := c.Get(loggerKey); ok {
		if lm, yes := val.(logger.LogManager); yes {
			return lm
		}
	}
	// fallback: create default logger
	def := logger.MustNewDefaultLogger()
	return def
}

// AccessLoggerMiddleware logs each request after completion
func AccessLoggerMiddleware(l logger.LogManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		fields := []interface{}{
			"log_type", "access",
			"ip", c.ClientIP(),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"duration_ms", latency.Milliseconds(),
			"size", c.Writer.Size(),
		}
		// add user-agent and request-id if present
		if rid := c.GetHeader(HeaderRequestID); rid != "" {
			fields = append(fields, "request_id", rid)
		}
		// attach route params for debugging (limit length if needed)
		if params := c.Params; len(params) > 0 {
			// sample: route=/users/:id id=123
			for _, p := range params {
				fields = append(fields, p.Key, p.Value)
			}
		}

		entry := l.With(fields...)
		if status >= 500 {
			entry.ErrorF("") // message blank; structured fields carry info
		} else if status >= 400 {
			entry.WarnF("")
		} else {
			entry.InfoF("")
		}
	}
}
