package logger

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ContextKey string

const (
	RequestIDKey ContextKey = "requestID"
	UserIDKey    ContextKey = "userID"
)

type LogManager interface {
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)

	DebugF(format string, args ...any)
	InfoF(format string, args ...any)
	WarnF(format string, args ...any)
	ErrorF(format string, args ...any)

	DebugFCtx(ctx context.Context, format string, args ...any)
	InfoFCtx(ctx context.Context, format string, args ...any)
	WarnFCtx(ctx context.Context, format string, args ...any)
	ErrorFCtx(ctx context.Context, format string, args ...any)

	With(keyValues ...any) LogManager

	Sync() error
	SetLogLevel(level string) error
}

// LoggerOptions for custom configuration
type LoggerOptions struct {
	Level        string
	Encoding     string // "json" or "console"
	OutputPaths  []string
	ErrorPaths   []string
	EnableCaller bool
	EnableStack  bool
	TimeFormat   string
}

// NewLogger creates a new Lumina logger with options
func NewLogger(opts LoggerOptions) (LogManager, error) {
	// Dynamic log level (can be changed at runtime)
	atomicLevel := zap.NewAtomicLevel()

	if err := atomicLevel.UnmarshalText([]byte(opts.Level)); err != nil {
		atomicLevel.SetLevel(zap.InfoLevel)
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalColorLevelEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			if opts.TimeFormat != "" {
				enc.AppendString(t.Format(opts.TimeFormat))
			} else {
				enc.AppendString(t.Format(time.RFC3339))
			}
		},
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	if opts.Encoding == "" {
		opts.Encoding = "console"
	}
	if len(opts.OutputPaths) == 0 {
		opts.OutputPaths = []string{"stdout"}
	}
	if len(opts.ErrorPaths) == 0 {
		opts.ErrorPaths = []string{"stderr"}
	}

	cfg := zap.Config{
		Level:            atomicLevel,
		Development:      opts.Level == "debug",
		Encoding:         opts.Encoding,
		EncoderConfig:    encoderCfg,
		OutputPaths:      opts.OutputPaths,
		ErrorOutputPaths: opts.ErrorPaths,
	}

	if opts.EnableCaller {
		cfg.EncoderConfig.CallerKey = "caller"
	} else {
		cfg.EncoderConfig.CallerKey = ""
	}

	zapLogger, err := cfg.Build(zap.AddStacktrace(zap.ErrorLevel))
	if err != nil {
		return nil, err
	}

	if opts.EnableStack {
		zapLogger = zapLogger.WithOptions(zap.AddStacktrace(zap.WarnLevel))
	}

	return &logger{
		Log:         zapLogger.Sugar(),
		atomicLevel: atomicLevel,
	}, nil
}

// MustNewDefaultLogger creates a production-ready logger quickly
func MustNewDefaultLogger() LogManager {
	logger, err := NewLogger(LoggerOptions{
		Level:        "info",
		Encoding:     "console",
		EnableCaller: true,
		EnableStack:  false,
	})
	if err != nil {
		fmt.Println("Failed to init logger:", err)
		os.Exit(1)
	}
	return logger
}
