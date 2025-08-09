package logger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// logger wraps zap for structured + flexible logging
type logger struct {
	Log         *zap.SugaredLogger
	atomicLevel zap.AtomicLevel
}

func (l *logger) Debug(args ...any) {
	l.Log.Debug(args...)
}
func (l *logger) Info(args ...any) {
	l.Log.Info(args...)
}
func (l *logger) Warn(args ...any) {
	l.Log.Warn(args...)
}
func (l *logger) Error(args ...any) {
	l.Log.Error(args...)
}

func (l *logger) DebugF(format string, args ...any) {
	l.Log.Debug(fmt.Sprintf(format, args...))
}
func (l *logger) InfoF(format string, args ...any) {
	l.Log.Info(fmt.Sprintf(format, args...))
}
func (l *logger) WarnF(format string, args ...any) {
	l.Log.Warn(fmt.Sprintf(format, args...))
}
func (l *logger) ErrorF(format string, args ...any) {
	l.Log.Error(fmt.Sprintf(format, args...))
}

func (l *logger) DebugFCtx(ctx context.Context, format string, args ...any) {
	l.Log.With(withContext(ctx)...).Debug(fmt.Sprintf(format, args...))
}
func (l *logger) InfoFCtx(ctx context.Context, format string, args ...any) {
	l.Log.With(withContext(ctx)...).Info(fmt.Sprintf(format, args...))
}
func (l *logger) WarnFCtx(ctx context.Context, format string, args ...any) {
	l.Log.With(withContext(ctx)...).Warn(fmt.Sprintf(format, args...))
}
func (l *logger) ErrorFCtx(ctx context.Context, format string, args ...any) {
	l.Log.With(withContext(ctx)...).Error(fmt.Sprintf(format, args...))
}

func (l *logger) With(fields ...any) LogManager {
	return &logger{
		Log:         l.Log.With(fields...),
		atomicLevel: l.atomicLevel,
	}
}

func (l *logger) Sync() error {
	return l.Log.Sync()
}

func (l *logger) SetLogLevel(level string) error {
	return l.atomicLevel.UnmarshalText([]byte(level))
}
