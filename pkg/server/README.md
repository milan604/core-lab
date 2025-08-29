# core-lab Server Package

## Overview
This package provides a robust, extensible HTTP server foundation for Go applications using [Gin](https://github.com/gin-gonic/gin). It includes:
- Flexible configuration via functional options
- Integrated logging
- CORS support
- Per-IP rate limiting
- Prometheus metrics
- Graceful shutdown
- TLS support
- Custom middleware injection

## Features

### 1. Configuration
Use the `config` package to load settings from files, environment variables, flags, and more. Pass a config instance to the server for dynamic setup.

### 2. Logging
Plug in your own logger or use the built-in default. All server events and middleware support structured logging.

### 3. CORS
Enable and configure CORS using the `CorsConfig` struct:
```go
server.WithCors(server.CorsConfig{
    Enabled: true,
    AllowOrigins: []string{"*"},
    AllowMethods: []string{"GET", "POST"},
    // ...other options
})
```

### 4. Rate Limiting
Per-IP rate limiting is provided via a single `RateLimitConfig` struct:
```go
rl := middleware.NewRateLimitConfig(true, 5, 10, time.Minute)
server.WithRateLimit(rl)
```
- `Enabled`: turn on/off
- `RPS`: requests per second
- `Burst`: burst size
- `CleanupInterval`: how often to clean up old IPs

### 5. Prometheus Metrics
Enable metrics collection and expose `/metrics` endpoint:
```go
server.WithPrometheus(true)
```

### 6. Graceful Shutdown
Handles SIGINT/SIGTERM and shuts down cleanly, waiting for in-flight requests to finish.

### 7. TLS Support
Provide certificate and key files to enable HTTPS:
```go
server.StartWithTLS("cert.pem", "key.pem")
```

### 8. Custom Middleware
Inject any Gin middleware:
```go
server.WithMiddleware(myCustomMiddleware)
```

## Usage Example
```go
import (
    "corelab/pkg/server"
    "corelab/pkg/config"
    "corelab/pkg/logger"
    "corelab/pkg/server/middleware"
    "time"
)

func main() {
    cfg := config.New(
        config.WithFile("config.yaml"),
        config.WithEnv("APP"),
    )

    rl := middleware.NewRateLimitConfig(true, 10, 20, time.Minute)

    engine := server.NewEngine(
        server.WithLogger(logger.MustNewDefaultLogger()),
        server.WithCors(server.CorsConfig{Enabled: true}),
        server.WithRateLimit(rl),
        server.WithPrometheus(true),
        server.WithRecovery(true),
    )

    // Register routes
    engine.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Hello, world!"})
    })

    // Start server
    if err := server.Start(engine, server.StartWithConfig(cfg)); err != nil {
        panic(err)
    }
}
```
# server

Gin server setup with advanced middleware (CORS, logging, Prometheus, rate limiting).

## Usage
```go
import "github.com/milan604/core-lab/pkg/server"
engine := server.NewEngine()
server.Start(engine)
```

## Implementation Guide

1. **Install dependencies**
   - Run `go mod tidy` to install required modules.

2. **Configure your server**
   - Use the functional options API to enable features as needed.

3. **Add your routes**
   - Register Gin routes on the engine returned by `NewEngine`.

4. **Start the server**
   - Use `server.Start` with any desired startup options.

## Extending
- Add your own middleware using `WithMiddleware`.
- Extend configuration via the `config` package.
- Add more metrics or logging as needed.

## File Structure
- `server.go`: main server logic
- `options.go`: functional options and config structs
- `middleware/`: CORS, rate limiting, logging, recovery, metrics
- `config/`: configuration loader and helpers
- `logger/`: logging utilities

## License
This repository is private and proprietary. All rights reserved.
Unauthorized copying, distribution, or use of this codebase is strictly prohibited.
