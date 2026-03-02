package server

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/milan604/core-lab/pkg/logger"
	"go.opentelemetry.io/otel/trace"
)

const loggerKey = "corelab_logger"

var skipAccessLogPaths = map[string]struct{}{
	"/metrics": {},
}

func shouldSkipAccessLog(path string) bool {
	_, skip := skipAccessLogPaths[path]
	return skip
}

func statusClass(status int) string {
	if status >= 100 && status <= 599 {
		return strconv.Itoa(status/100) + "xx"
	}
	return "unknown"
}

func requestIDFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if rid := c.GetString(string(logger.RequestIDKey)); rid != "" {
		return rid
	}
	if rid := c.Writer.Header().Get(HeaderRequestID); rid != "" {
		return rid
	}
	return c.GetHeader(HeaderRequestID)
}

// AppLoggerMiddleware injects a request-scoped logger into gin.Context
func AppLoggerMiddleware(l logger.LogManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := ""
		path := ""
		if c.Request != nil {
			method = c.Request.Method
			if c.Request.URL != nil {
				path = c.Request.URL.Path
			}
		}

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		fields := []interface{}{
			"log_type", "application",
			"source", c.HandlerName(),
			"route", route,
			"method", method,
			"path", path,
		}
		if rid := requestIDFromContext(c); rid != "" {
			fields = append(fields, "request_id", rid)
		}

		// create request-scoped logger with request context fields
		reqLogger := l.With(fields...)
		// store in gin context for handlers to retrieve
		c.Set(loggerKey, reqLogger)
		c.Next()
	}
}

// ...existing code...

// AccessLoggerMiddleware logs each request after completion
func AccessLoggerMiddleware(l logger.LogManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		method := ""
		path := ""
		host := ""
		proto := ""
		userAgent := ""
		referer := ""
		reqCtx := context.Background()
		if c.Request != nil {
			reqCtx = c.Request.Context()
			method = c.Request.Method
			host = c.Request.Host
			proto = c.Request.Proto
			userAgent = c.Request.UserAgent()
			referer = c.Request.Referer()
			if c.Request.URL != nil {
				path = c.Request.URL.Path
			}
		}

		if shouldSkipAccessLog(path) {
			return
		}

		latency := time.Since(start)
		status := c.Writer.Status()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		fields := []interface{}{
			"log_type", "access",
			"ip", c.ClientIP(),
			"method", method,
			"path", path,
			"route", route,
			"status", status,
			"status_class", statusClass(status),
			"duration_ms", latency.Milliseconds(),
			"size", c.Writer.Size(),
			"host", host,
			"proto", proto,
		}
		// add user-agent, referer, request-id, and trace context when present
		if userAgent != "" {
			fields = append(fields, "user_agent", userAgent)
		}
		if referer != "" {
			fields = append(fields, "referer", referer)
		}
		if rid := requestIDFromContext(c); rid != "" {
			fields = append(fields, "request_id", rid)
		}
		if span := trace.SpanFromContext(reqCtx); span.SpanContext().IsValid() {
			fields = append(fields,
				"trace_id", span.SpanContext().TraceID().String(),
				"span_id", span.SpanContext().SpanID().String(),
			)
		}
		if errCount := len(c.Errors); errCount > 0 {
			fields = append(fields, "error_count", errCount, "errors", c.Errors.String())
		}
		// attach route params for debugging (limit length if needed)
		if params := c.Params; len(params) > 0 {
			// sample: route=/users/:id id=123
			for _, p := range params {
				fields = append(fields, p.Key, p.Value)
			}
		}

		entry := l.With(fields...)
		const accessMessage = "http_request"
		if status >= 500 {
			entry.Error(accessMessage)
		} else if status >= 400 {
			entry.Warn(accessMessage)
		} else {
			entry.Info(accessMessage)
		}
	}
}
