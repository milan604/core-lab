package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/milan604/core-lab/pkg/logger"
)

type testLogger struct {
	warns []string
}

func (l *testLogger) Debug(args ...any)                 {}
func (l *testLogger) Info(args ...any)                  {}
func (l *testLogger) Warn(args ...any)                  {}
func (l *testLogger) Error(args ...any)                 {}
func (l *testLogger) DebugF(format string, args ...any) {}
func (l *testLogger) InfoF(format string, args ...any)  {}
func (l *testLogger) ErrorF(format string, args ...any) {}
func (l *testLogger) DebugFCtx(context.Context, string, ...any) {
}
func (l *testLogger) InfoFCtx(context.Context, string, ...any)  {}
func (l *testLogger) WarnFCtx(context.Context, string, ...any)  {}
func (l *testLogger) ErrorFCtx(context.Context, string, ...any) {}
func (l *testLogger) With(keyValues ...any) logger.LogManager   { return l }
func (l *testLogger) Sync() error                               { return nil }
func (l *testLogger) SetLogLevel(level string) error            { return nil }

func (l *testLogger) WarnF(format string, args ...any) {
	l.warns = append(l.warns, fmt.Sprintf(format, args...))
}

func TestRunShutdownHooksRunsInReverseOrderAndContinuesAfterErrors(t *testing.T) {
	log := &testLogger{}
	var order []string

	runShutdownHooks(log, Context{}, []ShutdownFunc{
		func(Context) error {
			order = append(order, "first")
			return nil
		},
		func(Context) error {
			order = append(order, "second")
			return errors.New("boom")
		},
		nil,
		func(Context) error {
			order = append(order, "third")
			return nil
		},
	}, "setup")

	want := []string{"third", "second", "first"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("shutdown order = %v, want %v", order, want)
	}
	if len(log.warns) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(log.warns))
	}
	if got := log.warns[0]; !strings.Contains(got, "setup shutdown hook failed") || !strings.Contains(got, "boom") {
		t.Fatalf("unexpected warning %q", got)
	}
}
