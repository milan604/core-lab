package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/milan604/core-lab/pkg/apperr"
	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/i18n"
	"github.com/milan604/core-lab/pkg/logger"
	"github.com/milan604/core-lab/pkg/observability"
	"github.com/milan604/core-lab/pkg/response"
	"github.com/milan604/core-lab/pkg/server"
	smw "github.com/milan604/core-lab/pkg/server/middleware"
	"github.com/milan604/core-lab/pkg/utils"
	"github.com/milan604/core-lab/pkg/validator"
	"github.com/milan604/core-lab/pkg/version"
)

func main() {
	// Config and logger
	cfg := config.New(
		config.WithDefaults(map[string]any{
			"service.endpoint": "0.0.0.0",
			"service.port":     "8080",
			"service_name":     "observability-example",
			"service_version":  version.Info().Version,
			"SIGNOZ_ENDPOINT":  "http://localhost:4318",
		}),
		config.WithEnv("APP"),
		config.WithAutoPFlags(),
	)
	log := logger.MustNewDefaultLogger()
	log.InfoF("starting example server", "version", version.Info())

	// Initialize observability (traces, metrics, and logs)
	obs, err := observability.New(log, cfg)
	if err != nil {
		log.WarnF("failed to initialize observability: %v", err)
	} else {
		defer obs.Shutdown(context.Background())
		log.InfoF("observability initialized - logs will be sent to SigNoz")
	}

	// Translator with JSON bundles under examples/auth_example/locales
	tr := i18n.New(
		i18n.WithDefaultLocale("fr"),
		i18n.WithFallbackLocales("fr"),
		i18n.WithJSONDir("default", "./examples/auth_example/locales"),
	)

	// Server engine with CORS, rate limit, recovery
	rl := smw.NewRateLimitConfig(true, 5, 10, time.Minute)
	engine := server.NewEngine(
		server.WithLogger(log),
		server.WithRecovery(true),
		server.WithCors(smw.DefaultCorsConfig()),
		server.WithRateLimit(rl),
		server.WithPrometheus(false),
	)

	// Observability middleware for automatic HTTP tracing and log correlation
	if obs != nil {
		serviceName := cfg.GetString("service_name")
		if serviceName == "" {
			serviceName = "observability-example"
		}
		engine.Use(observability.GinMiddleware(serviceName))
		log.InfoF("observability middleware enabled - HTTP requests will be traced")
	}

	// i18n middleware
	engine.Use(tr.GinMiddleware())

	// Create a validator instance for binding helpers
	vi := validator.New()

	// Routes
	engine.GET("/healthz", func(c *gin.Context) {
		// Logs are automatically sent to SigNoz with trace correlation
		log.InfoFCtx(c.Request.Context(), "health check requested")
		response.Success(c, gin.H{"status": "ok"})
	})
	engine.GET("/version", func(c *gin.Context) {
		// Logs include trace ID automatically when context is passed
		log.InfoFCtx(c.Request.Context(), "version requested")
		response.Success(c, version.Info())
	})

	// i18n example
	engine.GET("/i18n/hello", func(c *gin.Context) {
		ctx := c.Request.Context()

		// Create a custom span for this operation
		if obs != nil {
			ctx, span := obs.StartSpan(ctx, "i18n.hello")
			defer span.End()

			// Add span attributes
			observability.AddSpanAttributes(ctx,
				observability.AttrHTTPMethod.String(c.Request.Method),
				observability.AttrHTTPRoute.String(c.FullPath()),
			)
		}

		loc := c.GetString("i18n_locale")
		msg := tr.T(loc, "greeting", map[string]any{"name": "world"})

		// Log with context - trace ID will be included automatically
		log.InfoFCtx(ctx, "i18n greeting generated", "locale", loc)

		c.JSON(http.StatusOK, gin.H{"locale": loc, "message": msg})
	})

	// Validation example
	type CreateUser struct {
		Name  string `json:"name" binding:"required,min=2"`
		Email string `json:"email" binding:"required,email"`
	}
	engine.POST("/users", func(c *gin.Context) {
		ctx := c.Request.Context()

		// Create a custom span for user creation
		if obs != nil {
			var span trace.Span
			ctx, span = obs.StartSpan(ctx, "users.create")
			defer span.End()
		}

		body, err := validator.BindJSON[CreateUser](vi, c)
		if err != nil {
			// Record error in span if observability is enabled
			if obs != nil {
				observability.RecordSpanError(ctx, err)
			}
			log.ErrorFCtx(ctx, "failed to create user", "error", err)
			response.HandleError(c, err)
			return
		}

		// Use utils
		slug := utils.Slugify(body.Name)

		// Log success with context - trace ID included automatically
		log.InfoFCtx(ctx, "user created successfully", "user_id", slug, "email", body.Email)

		response.Success(c, gin.H{"id": fmt.Sprintf("user:%s", slug), "email": body.Email})
	})

	// Demonstrate an application error
	engine.GET("/fail", func(c *gin.Context) {
		ctx := c.Request.Context()

		// Create a span for error demonstration
		if obs != nil {
			var span trace.Span
			ctx, span = obs.StartSpan(ctx, "demo.error")
			defer span.End()
		}

		err := apperr.New(apperr.ErrorCodeInternal).WithMessage("demo error")

		// Record error in span and log with context
		if obs != nil {
			observability.RecordSpanError(ctx, err)
		}
		log.ErrorFCtx(ctx, "demo error occurred", "error", err)

		response.HandleError(c, err)
	})

	// Time utils example
	engine.GET("/time", func(c *gin.Context) {
		now := time.Now().UTC()
		c.JSON(200, gin.H{"rfc3339": now.Format(time.RFC3339), "unix": now.Unix()})
	})

	// i18n pluralization example
	engine.GET("/i18n/items", func(c *gin.Context) {
		loc := c.GetString("i18n_locale")
		count := 3
		msg := tr.T(loc, "cart.items", map[string]any{"count": count})
		c.JSON(200, gin.H{"message": msg, "count": count})
	})

	// Example route with custom span and attributes
	engine.GET("/items/:id", func(c *gin.Context) {
		ctx := c.Request.Context()
		itemID := c.Param("id")

		// Create span for item retrieval
		if obs != nil {
			var span trace.Span
			ctx, span = obs.StartSpan(ctx, "items.get")
			defer span.End()

			// Add item ID to span attributes
			observability.AddSpanAttributes(ctx,
				observability.AttrHTTPRoute.String(c.FullPath()),
				attribute.String("item.id", itemID),
			)
		}

		// Log item access with context - trace ID included automatically
		log.InfoFCtx(ctx, "item retrieved", "item_id", itemID)

		c.JSON(200, gin.H{"item": itemID, "status": "ok"})
	})

	// Start server with graceful shutdown
	if err := server.Start(engine,
		server.StartWithConfig(cfg),
		server.StartWithLogger(log),
	); err != nil {
		log.ErrorF("server error: %v", err)
		os.Exit(1)
	}
}
