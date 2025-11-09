package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

// ObservabilityIface defines the interface for observability operations
type ObservabilityIface interface {
	// StartSpan creates a new span for tracing
	StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)

	// Shutdown gracefully shuts down the observability system
	Shutdown(ctx context.Context) error

	// GetTracer returns the tracer instance
	GetTracer() trace.Tracer
}

// Observability manages OpenTelemetry tracing, metrics, and logs
type Observability struct {
	tracerProvider *sdktrace.TracerProvider
	tracer         trace.Tracer
	logExporter    *LogExporter
	log            logger.LogManager
	serviceName    string
	serviceVersion string
}

// New creates a new Observability instance with SigNoz/OpenTelemetry integration
func New(log logger.LogManager, cfg *config.Config) (ObservabilityIface, error) {
	serviceName := cfg.GetString("service_name")
	if serviceName == "" {
		serviceName = "unknown-service"
	}

	serviceVersion := cfg.GetString("service_version")
	if serviceVersion == "" {
		serviceVersion = "1.0.0"
	}

	// Get SigNoz endpoint from config (defaults to localhost:4318 for OTLP HTTP)
	signozEndpoint := cfg.GetString("SIGNOZ_ENDPOINT")
	if signozEndpoint == "" {
		signozEndpoint = "http://localhost:4318"
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP HTTP exporter for SigNoz
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(signozEndpoint),
		otlptracehttp.WithInsecure(), // Use WithTLSClientConfig for production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Use sdktrace.TraceIDRatioBased(0.1) for production
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create tracer
	tracer := tp.Tracer(
		serviceName,
		trace.WithInstrumentationVersion(serviceVersion),
	)

	// Create log exporter for sending logs to SigNoz
	logExporter, err := NewLogExporter(cfg)
	if err != nil {
		// Log error but don't fail - logs are optional
		log.WarnF("Failed to create log exporter: %v", err)
	}

	obs := &Observability{
		tracerProvider: tp,
		tracer:         tracer,
		logExporter:    logExporter,
		log:            log,
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
	}

	log.InfoF("Observability initialized: service=%s, version=%s, endpoint=%s",
		serviceName, serviceVersion, signozEndpoint)

	return obs, nil
}

// MustNew creates a new Observability instance and panics on error
func MustNew(log logger.LogManager, cfg *config.Config) ObservabilityIface {
	obs, err := New(log, cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize observability: %v", err))
	}
	return obs
}

// StartSpan creates a new span for tracing
func (o *Observability) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return o.tracer.Start(ctx, name, opts...)
}

// Shutdown gracefully shuts down the observability system
func (o *Observability) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := o.tracerProvider.Shutdown(ctx); err != nil {
		o.log.ErrorF("failed to shutdown tracer provider: %v", err)
		return err
	}

	// Shutdown log exporter if available
	if o.logExporter != nil {
		if err := o.logExporter.Shutdown(ctx); err != nil {
			o.log.ErrorF("failed to shutdown log exporter: %v", err)
		}
	}

	o.log.InfoF("Observability shutdown completed")
	return nil
}

// GetTracer returns the tracer instance
func (o *Observability) GetTracer() trace.Tracer {
	return o.tracer
}
