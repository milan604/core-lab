# Installation Guide

## Required Dependencies

Add these dependencies to your `go.mod`:

```bash
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
go get go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin
```

Or add to `go.mod`:

```go
require (
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v0.52.0
    go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.52.0
)
```

Then run:
```bash
go mod tidy
```

## Docker Setup

1. Start SigNoz stack:
```bash
docker-compose -f docker-compose.signoz.yml up -d
```

2. Verify services are running:
```bash
docker ps | grep signoz
```

3. Access SigNoz UI:
- Frontend: http://localhost:3301
- Query Service: http://localhost:8080

## Configuration

Add to your config file (`env/config.json` or environment variables):

```json
{
  "service_name": "ecompulse",
  "service_version": "1.0.0",
  "SIGNOZ_ENDPOINT": "http://localhost:4318"
}
```

Or via environment variable:
```bash
export SIGNOZ_ENDPOINT=http://localhost:4318
```

## Network Setup

Ensure your application and SigNoz are on the same Docker network:

```bash
docker network create shared-net
```

Or use the existing network from your docker-compose.yml.

