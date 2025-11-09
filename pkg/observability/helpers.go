package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SpanFromContext retrieves the current span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// RecordSpanError records an error on the current span
func RecordSpanError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() && err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// Common span attribute keys
var (
	AttrHTTPMethod     = attribute.Key("http.method")
	AttrHTTPRoute      = attribute.Key("http.route")
	AttrHTTPStatusCode = attribute.Key("http.status_code")
	AttrHTTPURL        = attribute.Key("http.url")
	AttrDBOperation    = attribute.Key("db.operation")
	AttrDBStatement    = attribute.Key("db.statement")
	AttrServiceName    = attribute.Key("service.name")
	AttrUserID         = attribute.Key("user.id")
	AttrRequestID      = attribute.Key("request.id")
)
