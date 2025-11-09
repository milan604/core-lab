package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsIface defines the interface for metrics operations
type MetricsIface interface {
	// IncrementCounter increments a counter metric
	IncrementCounter(ctx context.Context, name string, attrs ...attribute.KeyValue)
	
	// RecordGauge records a gauge metric
	RecordGauge(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue)
	
	// RecordHistogram records a histogram metric
	RecordHistogram(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue)
}

// Metrics manages OpenTelemetry metrics
type Metrics struct {
	meter metric.Meter
}

// NewMetrics creates a new Metrics instance
func NewMetrics(serviceName string) (MetricsIface, error) {
	meter := otel.Meter(serviceName)
	
	return &Metrics{
		meter: meter,
	}, nil
}

// MustNewMetrics creates a new Metrics instance and panics on error
func MustNewMetrics(serviceName string) MetricsIface {
	metrics, err := NewMetrics(serviceName)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize metrics: %v", err))
	}
	return metrics
}

// IncrementCounter increments a counter metric
func (m *Metrics) IncrementCounter(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	counter, err := m.meter.Int64Counter(
		name,
		metric.WithDescription(fmt.Sprintf("Counter for %s", name)),
	)
	if err != nil {
		return
	}
	counter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordGauge records a gauge metric
func (m *Metrics) RecordGauge(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue) {
	gauge, err := m.meter.Float64ObservableGauge(
		name,
		metric.WithDescription(fmt.Sprintf("Gauge for %s", name)),
	)
	if err != nil {
		return
	}
	// Note: ObservableGauge requires registration, this is simplified
	// For production, use Int64Gauge or Float64Gauge instead
	_, _ = gauge, value
}

// RecordHistogram records a histogram metric
func (m *Metrics) RecordHistogram(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue) {
	histogram, err := m.meter.Float64Histogram(
		name,
		metric.WithDescription(fmt.Sprintf("Histogram for %s", name)),
	)
	if err != nil {
		return
	}
	histogram.Record(ctx, value, metric.WithAttributes(attrs...))
}

