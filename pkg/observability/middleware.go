package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GinMiddleware creates a Gin middleware for automatic tracing
func GinMiddleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}

// TraceHandler wraps a handler function with tracing
func TraceHandler(obs ObservabilityIface, handlerName string, handler func(*gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := obs.StartSpan(c.Request.Context(), handlerName,
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", c.FullPath()),
				attribute.String("http.url", c.Request.URL.String()),
			),
		)
		defer span.End()

		// Add span to context
		c.Request = c.Request.WithContext(ctx)

		// Execute handler
		start := time.Now()
		handler(c)
		duration := time.Since(start)

		// Record attributes
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int64("http.duration_ms", duration.Milliseconds()),
		)

		// Set span status based on HTTP status code
		if c.Writer.Status() >= 400 {
			span.SetStatus(codes.Error, "HTTP error")
		} else {
			span.SetStatus(codes.Ok, "Success")
		}
	}
}

// TraceDBOperation traces a database operation
func TraceDBOperation(ctx context.Context, obs ObservabilityIface, operation string, query string) (context.Context, trace.Span) {
	ctx, span := obs.StartSpan(ctx, fmt.Sprintf("db.%s", operation),
		trace.WithAttributes(
			attribute.String("db.operation", operation),
			attribute.String("db.statement", query),
		),
	)
	return ctx, span
}

// TraceExternalCall traces an external API call
func TraceExternalCall(ctx context.Context, obs ObservabilityIface, serviceName string, method string, url string) (context.Context, trace.Span) {
	ctx, span := obs.StartSpan(ctx, fmt.Sprintf("http.%s", serviceName),
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.url", url),
			attribute.String("peer.service", serviceName),
		),
	)
	return ctx, span
}
