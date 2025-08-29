# Logger Package

This package provides structured, context-aware logging for Go applications, built on top of [zap](https://github.com/uber-go/zap). It is designed for flexibility, performance, and easy integration with modern cloud-native services.

## Features

## Usage Example
```go
import "github.com/yourorg/corelab/pkg/logger"

log := logger.MustNewDefaultLogger()
log.Info("Service started")
log.DebugF("Debug value: %v", value)

ctx := context.WithValue(context.Background(), logger.RequestIDKey, "req-123")
log.InfoFCtx(ctx, "User login: %s", userID)

log.SetLogLevel("debug") // Change log level at runtime
```
# logger

Structured logging using zap with context support.

## Usage
```go
import "github.com/milan604/core-lab/pkg/logger"
log := logger.MustNewDefaultLogger()
log.Info("Hello world")
```

## API Reference
- `LoggerOptions`: Options for custom logger configuration
- `ContextKey`: Type for context keys (request/user IDs)

### Functions
- `NewLogger(opts LoggerOptions) (LogManager, error)`: Create a new logger with options
- `MustNewDefaultLogger() LogManager`: Create a default production logger (console, info level)

### Methods (LogManager)
- `Debug(args ...any)` — Log debug message
- `Info(args ...any)` — Log info message
- `Warn(args ...any)` — Log warning
- `Error(args ...any)` — Log error
- `DebugF(format string, args ...any)` — Debug with formatting
- `InfoF(format string, args ...any)` — Info with formatting
- `WarnF(format string, args ...any)` — Warn with formatting
- `ErrorF(format string, args ...any)` — Error with formatting
- `DebugFCtx(ctx, format, args...)` — Debug with context
- `InfoFCtx(ctx, format, args...)` — Info with context
- `WarnFCtx(ctx, format, args...)` — Warn with context
- `ErrorFCtx(ctx, format, args...)` — Error with context
- `With(fields ...any) LogManager` — Add custom fields to logger
- `Sync() error` — Flush logs
- `SetLogLevel(level string) error` — Change log level at runtime


### Context Integration & Custom Fields
- Use `logger.RequestIDKey` and `logger.UserIDKey` to inject request/user IDs into context for automatic logging.
- Register custom context keys for logging fields using:
    - `RegisterContextKey(ctxKey, logField)` — Register a context key to be logged as a field
    - `UnregisterContextKey(ctxKey)` — Remove a context key from logging
- All registered context keys will be automatically extracted and logged via context-aware methods (e.g., `InfoFCtx`).

## Customization
- Set log encoding: `"console"` or `"json"`
- Set output paths: file, stdout, etc.
- Enable/disable caller and stacktrace info
- Custom time format

## Example: Custom Logger
```go
opts := logger.LoggerOptions{
    Level:        "debug",
    Encoding:     "json",
    OutputPaths:  []string{"app.log"},
    ErrorPaths:   []string{"error.log"},
    EnableCaller: true,
    EnableStack:  true,
    TimeFormat:   time.RFC3339Nano,
}
log, err := logger.NewLogger(opts)
if err != nil {
    panic(err)
}
```


## Internal Details
- The internal logger struct wraps zap's SugaredLogger and supports dynamic log level changes.
- Context fields are extracted using a registry, allowing flexible logging of any context value.

## Notes
- All zap features are available via `LogManager.Log` (if needed)
- Safe for concurrent use
- Designed for cloud, microservices, and CLI tools
- All variadic arguments use Go 1.18+ `any` type
