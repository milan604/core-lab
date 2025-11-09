# Observability Package

This package provides easy-to-use OpenTelemetry integration for SigNoz observability platform. It's designed to be migrated to `core-lab` later for use across all microservices.

## Features

- ✅ **Automatic HTTP Tracing** - Automatic tracing for all HTTP requests
- ✅ **Manual Span Creation** - Create custom spans for business logic
- ✅ **Database Operation Tracing** - Trace database queries
- ✅ **External API Tracing** - Trace external service calls
- ✅ **Metrics Support** - Record counters, gauges, and histograms
- ✅ **Log Export** - Automatic log sending to SigNoz dashboard
- ✅ **Error Tracking** - Automatic error recording in spans
- ✅ **Context Propagation** - Automatic trace context propagation
- ✅ **Trace-Log Correlation** - Logs automatically include trace IDs for correlation

## Quick Start

### 1. Initialize Observability

```go
import (
    "ecompulse/core/pkg/observability"
    "github.com/milan604/core-lab/pkg/config"
    "github.com/milan604/core-lab/pkg/logger"
)

func main() {
    logger := logger.MustNewDefaultLogger()
    cfg := config.New(...)
    
    // Initialize observability
    obs, err := observability.New(logger, cfg)
    if err != nil {
        logger.ErrorF("failed to initialize observability: %v", err)
        return
    }
    defer obs.Shutdown(context.Background())
    
    // Add Gin middleware for automatic HTTP tracing
    engine.Use(observability.GinMiddleware("my-service"))
}
```

### 2. Configuration

Add to your config file or environment variables:

```json
{
  "service_name": "my-service",
  "service_version": "1.0.0",
  "SIGNOZ_ENDPOINT": "http://localhost:4318"
}
```

Or via environment variable:
```bash
export SIGNOZ_ENDPOINT=http://localhost:4318
```

### 3. Automatic HTTP Tracing

The Gin middleware automatically traces all HTTP requests:

```go
engine.Use(observability.GinMiddleware("my-service"))
```

This automatically:
- Creates spans for each HTTP request
- Records HTTP method, route, status code, duration
- Propagates trace context
- Records errors automatically

## Usage Examples

### Manual Span Creation

```go
func (s *Service) ProcessOrder(ctx context.Context, orderID string) error {
    // Start a new span
    ctx, span := obs.StartSpan(ctx, "process_order",
        trace.WithAttributes(
            attribute.String("order.id", orderID),
        ),
    )
    defer span.End()
    
    // Your business logic
    order, err := s.getOrder(ctx, orderID)
    if err != nil {
        // Record error in span
        observability.RecordSpanError(ctx, err)
        return err
    }
    
    // Add span attributes
    observability.AddSpanAttributes(ctx,
        attribute.String("order.status", order.Status),
        attribute.Float64("order.total", order.Total),
    )
    
    return nil
}
```

### Database Operation Tracing

```go
func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
    ctx, span := observability.TraceDBOperation(ctx, obs, "select", "SELECT * FROM users WHERE id = ?")
    defer span.End()
    
    // Your database query
    user, err := s.db.GetUser(ctx, userID)
    if err != nil {
        observability.RecordSpanError(ctx, err)
        return nil, err
    }
    
    return user, nil
}
```

### External API Call Tracing

```go
func (s *Service) CallExternalAPI(ctx context.Context, data interface{}) error {
    ctx, span := observability.TraceExternalCall(ctx, obs, "payment-service", "POST", "https://api.payment.com/charge")
    defer span.End()
    
    // Make HTTP request
    resp, err := http.Post(...)
    if err != nil {
        observability.RecordSpanError(ctx, err)
        return err
    }
    
    // Add response attributes
    observability.AddSpanAttributes(ctx,
        attribute.Int("http.status_code", resp.StatusCode),
    )
    
    return nil
}
```

### Adding Events to Spans

```go
func (s *Service) ProcessPayment(ctx context.Context, paymentID string) error {
    // Add event to current span
    observability.AddSpanEvent(ctx, "payment.initiated",
        attribute.String("payment.id", paymentID),
    )
    
    // Process payment...
    
    observability.AddSpanEvent(ctx, "payment.completed",
        attribute.String("payment.id", paymentID),
        attribute.String("payment.status", "success"),
    )
    
    return nil
}
```

### Metrics

```go
import "ecompulse/core/pkg/observability"

// Initialize metrics
metrics, err := observability.NewMetrics("my-service")
if err != nil {
    log.Fatal(err)
}

// Increment counter
metrics.IncrementCounter(ctx, "orders.created",
    attribute.String("order.type", "premium"),
)

// Record histogram (e.g., request duration)
metrics.RecordHistogram(ctx, "http.request.duration", 150.5,
    attribute.String("http.method", "GET"),
    attribute.String("http.route", "/api/orders"),
)
```

### Using in Endpoints

```go
type OrderHandler struct {
    Service OrderServiceIface
    Obs     observability.ObservabilityIface
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
    // Span is automatically created by Gin middleware
    ctx := c.Request.Context()
    
    // Add custom attributes
    observability.AddSpanAttributes(ctx,
        attribute.String("user.id", getUserID(c)),
    )
    
    // Your handler logic
    order, err := h.Service.CreateOrder(ctx, ...)
    if err != nil {
        observability.RecordSpanError(ctx, err)
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, order)
}
```

## Integration with Existing Code

### In Service Layer

```go
type Service struct {
    Log logger.LogManager
    Obs observability.ObservabilityIface
    // ... other fields
}

func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    // Start span
    ctx, span := s.Obs.StartSpan(ctx, "create_user")
    defer span.End()
    
    // Add attributes
    observability.AddSpanAttributes(ctx,
        attribute.String("user.email", req.Email),
    )
    
    // Your business logic
    user, err := s.db.CreateUser(ctx, req)
    if err != nil {
        observability.RecordSpanError(ctx, err)
        return nil, err
    }
    
    return user, nil
}
```

### In Database Layer

```go
func (db *DB) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    // Trace database operation
    ctx, span := observability.TraceDBOperation(ctx, db.Obs, "insert", 
        "INSERT INTO users (email, name) VALUES ($1, $2)")
    defer span.End()
    
    // Execute query
    // ...
}
```

## Best Practices

1. **Always defer span.End()** - Ensures spans are properly closed
2. **Use meaningful span names** - Use descriptive names like "create_order" instead of "operation"
3. **Add relevant attributes** - Add attributes that help with debugging and filtering
4. **Record errors** - Always use `RecordSpanError` when errors occur
5. **Propagate context** - Always pass context through function calls
6. **Use consistent naming** - Use consistent span and attribute naming conventions

## Span Naming Conventions

- **HTTP requests**: Automatically handled by middleware
- **Database operations**: `db.{operation}` (e.g., `db.select`, `db.insert`)
- **External calls**: `http.{service_name}` (e.g., `http.payment-service`)
- **Business logic**: `{action}_{resource}` (e.g., `create_order`, `process_payment`)

## Attribute Naming

Use standard OpenTelemetry semantic conventions:
- `http.method`, `http.route`, `http.status_code`
- `db.operation`, `db.statement`
- `user.id`, `order.id`, `payment.id`
- `service.name`, `service.version`

## SigNoz Setup

1. Start SigNoz stack:
```bash
docker-compose -f docker-compose.signoz.yml up -d
```

2. Access SigNoz UI:
- Frontend: http://localhost:3301
- Query Service: http://localhost:8080

3. View traces, metrics, and logs in the SigNoz dashboard

## Log Export

The observability package automatically sends logs to SigNoz when initialized. Logs are:

- **Buffered** - Logs are buffered and sent in batches for efficiency
- **Correlated with Traces** - Logs automatically include trace IDs and span IDs when available
- **Structured** - Logs include service name, version, level, message, and custom attributes
- **Automatic** - All logs from your logger are automatically sent to SigNoz

### How Log Sending Works

1. **Initialization**: When you call `observability.New()`, a `LogExporter` is automatically created
2. **Log Capture**: The exporter captures logs from your logger via the `LogManagerWrapper`
3. **Buffering**: Logs are buffered in memory (default: 100 logs or 5 seconds)
4. **Export**: Buffered logs are sent to SigNoz via HTTP in OTLP format
5. **Dashboard**: Logs appear in the SigNoz dashboard with trace correlation

### Using Log Export

Log export is **automatic** when you initialize observability:

```go
obs, err := observability.New(logger, cfg)
// Logs are now automatically sent to SigNoz!
```

All your existing logger calls will automatically send logs to SigNoz:

```go
logger.InfoF("User created: %s", userID)
logger.ErrorF("Failed to process order: %v", err)
logger.InfoFCtx(ctx, "Processing payment") // Includes trace ID automatically
```

### Viewing Logs in SigNoz

1. Open SigNoz dashboard: http://localhost:3301
2. Navigate to **Logs** section
3. Filter by service name, log level, or trace ID
4. Click on a log to see full details and correlated traces

### Log Attributes

Each log entry includes:
- `timestamp` - When the log was created
- `level` - Log level (DEBUG, INFO, WARN, ERROR)
- `message` - Log message
- `service` - Service name from config
- `version` - Service version from config
- `trace_id` - Trace ID (if available from context)
- `span_id` - Span ID (if available from context)
- `attributes` - Custom fields (caller, stacktrace, etc.)

### Manual Log Export

You can also create a logger that sends logs to SigNoz:

```go
import "github.com/milan604/core-lab/pkg/observability"

logOpts := logger.LoggerOptions{
    Level:        "info",
    Encoding:     "json",
    EnableCaller: true,
}

sigNozLogger, err := observability.NewLoggerWithSigNoz(cfg, logOpts)
if err != nil {
    log.Fatal(err)
}

// Use this logger - all logs go to SigNoz
sigNozLogger.InfoF("Service started")
```

## Migration to core-lab

This package is designed to be easily migrated to `core-lab`:

1. Move `core/pkg/observability` to `core-lab/pkg/observability`
2. Update imports from `ecompulse/core/pkg/observability` to `github.com/milan604/core-lab/pkg/observability`
3. No code changes needed in microservices using this package

## Troubleshooting

### Traces not appearing in SigNoz

1. Check SigNoz is running: `docker ps | grep signoz`
2. Verify endpoint: Check `SIGNOZ_ENDPOINT` config
3. Check logs: `docker logs signoz-otel-collector`
4. Verify network: Ensure services are on the same Docker network

### High memory usage

1. Reduce sampling rate in `observability.New()`:
```go
sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)) // Sample 10% of traces
```

2. Use batch processor (already configured in SigNoz collector)

## References

- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
- [SigNoz Documentation](https://signoz.io/docs/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

