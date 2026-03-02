package observability

import (
	"context"
	"testing"

	"github.com/milan604/core-lab/pkg/logger"
)

type noopLogManager struct{}

func (n *noopLogManager) Debug(args ...any)                 {}
func (n *noopLogManager) Info(args ...any)                  {}
func (n *noopLogManager) Warn(args ...any)                  {}
func (n *noopLogManager) Error(args ...any)                 {}
func (n *noopLogManager) DebugF(format string, args ...any) {}
func (n *noopLogManager) InfoF(format string, args ...any)  {}
func (n *noopLogManager) WarnF(format string, args ...any)  {}
func (n *noopLogManager) ErrorF(format string, args ...any) {}
func (n *noopLogManager) DebugFCtx(ctx context.Context, format string, args ...any) {
}
func (n *noopLogManager) InfoFCtx(ctx context.Context, format string, args ...any) {
}
func (n *noopLogManager) WarnFCtx(ctx context.Context, format string, args ...any) {
}
func (n *noopLogManager) ErrorFCtx(ctx context.Context, format string, args ...any) {
}
func (n *noopLogManager) With(keyValues ...any) logger.LogManager { return n }
func (n *noopLogManager) Sync() error                             { return nil }
func (n *noopLogManager) SetLogLevel(level string) error          { return nil }

func newTestLogWrapper() (*LogManagerWrapper, *LogExporter) {
	exporter := &LogExporter{
		serviceName:    "test-service",
		serviceVersion: "1.0.0",
		buffer:         make([]LogEntry, 0, 8),
		bufferSize:     100,
	}

	return &LogManagerWrapper{
		original: &noopLogManager{},
		exporter: exporter,
	}, exporter
}

func TestLogManagerWrapperWithIncludesFieldsInExport(t *testing.T) {
	wrapper, exporter := newTestLogWrapper()

	wrapper.With("log_type", "access", "path", "/metrics", "status", 200).Info("http_request")

	if got, want := len(exporter.buffer), 1; got != want {
		t.Fatalf("exported entries = %d, want %d", got, want)
	}

	entry := exporter.buffer[0]
	if got, want := entry.Message, "http_request"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	if got, want := entry.Attributes["log_type"], "access"; got != want {
		t.Fatalf("log_type = %v, want %v", got, want)
	}
	if got, want := entry.Attributes["path"], "/metrics"; got != want {
		t.Fatalf("path = %v, want %v", got, want)
	}
	if got, want := entry.Attributes["status"], 200; got != want {
		t.Fatalf("status = %v, want %v", got, want)
	}
}

func TestLogManagerWrapperWithMergesAndOverridesFields(t *testing.T) {
	wrapper, exporter := newTestLogWrapper()

	wrapper.
		With("component", "http", "status", 201).
		With("status", 202, "method", "GET").
		Info("request_handled")

	if got, want := len(exporter.buffer), 1; got != want {
		t.Fatalf("exported entries = %d, want %d", got, want)
	}

	entry := exporter.buffer[0]
	if got, want := entry.Attributes["component"], "http"; got != want {
		t.Fatalf("component = %v, want %v", got, want)
	}
	if got, want := entry.Attributes["status"], 202; got != want {
		t.Fatalf("status = %v, want %v", got, want)
	}
	if got, want := entry.Attributes["method"], "GET"; got != want {
		t.Fatalf("method = %v, want %v", got, want)
	}
}

func TestLogManagerWrapperNormalizesBlankMessages(t *testing.T) {
	t.Run("uses log_type when message is blank", func(t *testing.T) {
		wrapper, exporter := newTestLogWrapper()

		wrapper.With("log_type", "access").InfoF("")

		if got, want := len(exporter.buffer), 1; got != want {
			t.Fatalf("exported entries = %d, want %d", got, want)
		}
		if got, want := exporter.buffer[0].Message, "access"; got != want {
			t.Fatalf("message = %q, want %q", got, want)
		}
	})

	t.Run("uses fallback when log_type is missing", func(t *testing.T) {
		wrapper, exporter := newTestLogWrapper()

		wrapper.InfoF("")

		if got, want := len(exporter.buffer), 1; got != want {
			t.Fatalf("exported entries = %d, want %d", got, want)
		}
		if got, want := exporter.buffer[0].Message, "log_entry"; got != want {
			t.Fatalf("message = %q, want %q", got, want)
		}
	})
}
