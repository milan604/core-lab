package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/logger"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap/zapcore"
)

// LogExporter manages log export to SigNoz via HTTP
type LogExporter struct {
	signozEndpoint string
	serviceName    string
	serviceVersion string
	httpClient     *http.Client
	mu             sync.Mutex
	buffer         []LogEntry
	bufferSize     int
	flushInterval  time.Duration
	stopChan       chan struct{}
}

// LogEntry represents a log entry to be sent to SigNoz
type LogEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Service    string                 `json:"service"`
	Version    string                 `json:"version"`
	TraceID    string                 `json:"trace_id,omitempty"`
	SpanID     string                 `json:"span_id,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	Caller     string                 `json:"caller,omitempty"`
	Stacktrace string                 `json:"stacktrace,omitempty"`
}

// NewLogExporter creates a new log exporter for sending logs to SigNoz
func NewLogExporter(cfg *config.Config) (*LogExporter, error) {
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

	exporter := &LogExporter{
		signozEndpoint: signozEndpoint,
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		buffer:        make([]LogEntry, 0, 100),
		bufferSize:    100,
		flushInterval: 5 * time.Second,
		stopChan:      make(chan struct{}),
	}

	// Start background flush goroutine
	go exporter.flushLoop()

	return exporter, nil
}

// flushLoop periodically flushes buffered logs
func (le *LogExporter) flushLoop() {
	ticker := time.NewTicker(le.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			le.Flush(context.Background())
		case <-le.stopChan:
			// Final flush on shutdown
			le.Flush(context.Background())
			return
		}
	}
}

// EmitLog sends a log record to SigNoz (buffered)
func (le *LogExporter) EmitLog(ctx context.Context, level string, message string, fields map[string]interface{}) {
	// Extract trace context if available
	var traceID, spanID string
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
		spanID = span.SpanContext().SpanID().String()
	}

	entry := LogEntry{
		Timestamp:  time.Now(),
		Level:      level,
		Message:    message,
		Service:    le.serviceName,
		Version:    le.serviceVersion,
		TraceID:    traceID,
		SpanID:     spanID,
		Attributes: fields,
	}

	le.mu.Lock()
	le.buffer = append(le.buffer, entry)
	shouldFlush := len(le.buffer) >= le.bufferSize
	le.mu.Unlock()

	if shouldFlush {
		le.Flush(ctx)
	}
}

// Flush sends buffered logs to SigNoz
func (le *LogExporter) Flush(ctx context.Context) error {
	le.mu.Lock()
	if len(le.buffer) == 0 {
		le.mu.Unlock()
		return nil
	}

	entries := make([]LogEntry, len(le.buffer))
	copy(entries, le.buffer)
	le.buffer = le.buffer[:0]
	le.mu.Unlock()

	// Send logs to SigNoz via HTTP
	// SigNoz accepts logs via OTLP HTTP endpoint
	// We'll send logs in OTLP format to /v1/logs endpoint
	payload := map[string]interface{}{
		"resourceLogs": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"attributes": []map[string]interface{}{
						{"key": "service.name", "value": map[string]interface{}{"stringValue": le.serviceName}},
						{"key": "service.version", "value": map[string]interface{}{"stringValue": le.serviceVersion}},
					},
				},
				"scopeLogs": []map[string]interface{}{
					{
						"logRecords": le.convertToOTLPFormat(entries),
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	// Send to SigNoz OTLP logs endpoint
	url := fmt.Sprintf("%s/v1/logs", le.signozEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := le.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// convertToOTLPFormat converts log entries to OTLP format
func (le *LogExporter) convertToOTLPFormat(entries []LogEntry) []map[string]interface{} {
	otlpRecords := make([]map[string]interface{}, 0, len(entries))

	for _, entry := range entries {
		// Map log level to OTLP severity
		severityNumber := mapLogLevelToSeverity(entry.Level)

		// Build attributes
		attrs := []map[string]interface{}{
			{"key": "message", "value": map[string]interface{}{"stringValue": entry.Message}},
		}

		if entry.TraceID != "" {
			attrs = append(attrs, map[string]interface{}{
				"key": "trace_id", "value": map[string]interface{}{"stringValue": entry.TraceID},
			})
		}
		if entry.SpanID != "" {
			attrs = append(attrs, map[string]interface{}{
				"key": "span_id", "value": map[string]interface{}{"stringValue": entry.SpanID},
			})
		}

		// Add custom attributes
		for k, v := range entry.Attributes {
			attrs = append(attrs, map[string]interface{}{
				"key": k, "value": map[string]interface{}{"stringValue": fmt.Sprintf("%v", v)},
			})
		}

		record := map[string]interface{}{
			"timeUnixNano":         entry.Timestamp.UnixNano(),
			"observedTimeUnixNano": time.Now().UnixNano(),
			"severityNumber":       severityNumber,
			"severityText":         entry.Level,
			"body": map[string]interface{}{
				"stringValue": entry.Message,
			},
			"attributes": attrs,
		}

		otlpRecords = append(otlpRecords, record)
	}

	return otlpRecords
}

// mapLogLevelToSeverity maps log level to OTLP severity number
func mapLogLevelToSeverity(level string) int {
	switch level {
	case "DEBUG", "debug":
		return 5 // DEBUG
	case "INFO", "info":
		return 9 // INFO
	case "WARN", "warn", "WARNING", "warning":
		return 13 // WARN
	case "ERROR", "error":
		return 17 // ERROR
	default:
		return 9 // Default to INFO
	}
}

// Shutdown gracefully shuts down the log exporter
func (le *LogExporter) Shutdown(ctx context.Context) error {
	close(le.stopChan)
	return le.Flush(ctx)
}

// ZapHook is a zap hook that sends logs to SigNoz
type ZapHook struct {
	exporter *LogExporter
}

// NewZapHook creates a new zap hook for sending logs to SigNoz
func NewZapHook(exporter *LogExporter) *ZapHook {
	return &ZapHook{exporter: exporter}
}

// Levels returns the log levels this hook should be called for
func (h *ZapHook) Levels() []zapcore.Level {
	return []zapcore.Level{
		zapcore.DebugLevel,
		zapcore.InfoLevel,
		zapcore.WarnLevel,
		zapcore.ErrorLevel,
	}
}

// Fire is called when a log entry is written
func (h *ZapHook) Fire(entry zapcore.Entry) error {
	// Extract trace context from entry fields if available
	ctx := context.Background()
	fields := make(map[string]interface{})

	// Add caller information
	if entry.Caller.Defined {
		fields["caller"] = entry.Caller.String()
	}

	// Add stacktrace if available
	if entry.Stack != "" {
		fields["stacktrace"] = entry.Stack
	}

	// Convert zap fields to map
	// Note: This is simplified - in production you'd want to properly extract all fields
	level := entry.Level.String()

	// Emit log to SigNoz
	h.exporter.EmitLog(ctx, level, entry.Message, fields)

	return nil
}

// NewLoggerWithSigNoz creates a logger that sends logs to SigNoz
func NewLoggerWithSigNoz(cfg *config.Config, logOpts logger.LoggerOptions) (logger.LogManager, error) {
	// Create log exporter
	exporter, err := NewLogExporter(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create log exporter: %w", err)
	}

	// Create logger
	originalLogger, err := logger.NewLogger(logOpts)
	if err != nil {
		return nil, err
	}

	// Return a wrapper that sends logs to SigNoz
	return &LogManagerWrapper{
		original: originalLogger,
		exporter: exporter,
	}, nil
}

// LogManagerWrapper wraps the logger.LogManager to send logs to SigNoz
type LogManagerWrapper struct {
	original logger.LogManager
	exporter *LogExporter
}

// Debug logs a debug message
func (l *LogManagerWrapper) Debug(args ...any) {
	l.original.Debug(args...)
	l.exporter.EmitLog(context.Background(), "DEBUG", fmt.Sprint(args...), nil)
}

// Info logs an info message
func (l *LogManagerWrapper) Info(args ...any) {
	l.original.Info(args...)
	l.exporter.EmitLog(context.Background(), "INFO", fmt.Sprint(args...), nil)
}

// Warn logs a warning message
func (l *LogManagerWrapper) Warn(args ...any) {
	l.original.Warn(args...)
	l.exporter.EmitLog(context.Background(), "WARN", fmt.Sprint(args...), nil)
}

// Error logs an error message
func (l *LogManagerWrapper) Error(args ...any) {
	l.original.Error(args...)
	l.exporter.EmitLog(context.Background(), "ERROR", fmt.Sprint(args...), nil)
}

// DebugF logs a formatted debug message
func (l *LogManagerWrapper) DebugF(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	l.original.DebugF(format, args...)
	l.exporter.EmitLog(context.Background(), "DEBUG", message, nil)
}

// InfoF logs a formatted info message
func (l *LogManagerWrapper) InfoF(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	l.original.InfoF(format, args...)
	l.exporter.EmitLog(context.Background(), "INFO", message, nil)
}

// WarnF logs a formatted warning message
func (l *LogManagerWrapper) WarnF(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	l.original.WarnF(format, args...)
	l.exporter.EmitLog(context.Background(), "WARN", message, nil)
}

// ErrorF logs a formatted error message
func (l *LogManagerWrapper) ErrorF(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	l.original.ErrorF(format, args...)
	l.exporter.EmitLog(context.Background(), "ERROR", message, nil)
}

// DebugFCtx logs a formatted debug message with context
func (l *LogManagerWrapper) DebugFCtx(ctx context.Context, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	l.original.DebugFCtx(ctx, format, args...)
	l.exporter.EmitLog(ctx, "DEBUG", message, nil)
}

// InfoFCtx logs a formatted info message with context
func (l *LogManagerWrapper) InfoFCtx(ctx context.Context, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	l.original.InfoFCtx(ctx, format, args...)
	l.exporter.EmitLog(ctx, "INFO", message, nil)
}

// WarnFCtx logs a formatted warning message with context
func (l *LogManagerWrapper) WarnFCtx(ctx context.Context, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	l.original.WarnFCtx(ctx, format, args...)
	l.exporter.EmitLog(ctx, "WARN", message, nil)
}

// ErrorFCtx logs a formatted error message with context
func (l *LogManagerWrapper) ErrorFCtx(ctx context.Context, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	l.original.ErrorFCtx(ctx, format, args...)
	l.exporter.EmitLog(ctx, "ERROR", message, nil)
}

// With adds fields to the logger
func (l *LogManagerWrapper) With(keyValues ...any) logger.LogManager {
	return &LogManagerWrapper{
		original: l.original.With(keyValues...),
		exporter: l.exporter,
	}
}

// Sync flushes buffered logs
func (l *LogManagerWrapper) Sync() error {
	return l.original.Sync()
}

// SetLogLevel sets the log level
func (l *LogManagerWrapper) SetLogLevel(level string) error {
	return l.original.SetLogLevel(level)
}
