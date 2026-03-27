// Package app provides a builder for bootstrapping multi-tenant SaaS
// microservices with standardized infrastructure setup.
//
// It eliminates the ~200 lines of boilerplate repeated in every service's
// main.go by encapsulating the common startup sequence:
//
//	logger → config → runtime config → observability → audit → engine → server
//
// Services plug in their custom logic via callback hooks.
package app

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/milan604/core-lab/pkg/audit"
	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/logger"
	"github.com/milan604/core-lab/pkg/observability"
	"github.com/milan604/core-lab/pkg/runtimeconfig"
	"github.com/milan604/core-lab/pkg/server"
	servermiddleware "github.com/milan604/core-lab/pkg/server/middleware"
	"github.com/milan604/core-lab/pkg/validator"
)

// ConfigType is an alias for config.Config to allow use in service config packages.
type ConfigType = config.Config

// SetupResult holds the output of the OnSetup hook.
type SetupResult struct {
	// Middleware to add to the Gin engine (applied before routes).
	Middleware []gin.HandlerFunc
	// Shutdown hooks run after the HTTP server stops, in reverse order.
	Shutdown []ShutdownFunc
}

// ShutdownFunc runs during service shutdown after the server has stopped.
type ShutdownFunc func(Context) error

// Context exposes the initialized infrastructure components to service hooks.
type Context struct {
	Logger         logger.LogManager
	Config         *config.Config
	Validator      *validator.Validator
	AuditPublisher audit.Publisher
	Observability  observability.ObservabilityIface
}

// App holds the builder configuration for a service.
type App struct {
	serviceName    string
	serviceVersion string
	servicePort    string

	// Config
	configFile    string
	configOptions []config.Option

	// Runtime config
	runtimeConfigEnabled bool
	runtimeConfigOpts    runtimeconfig.ResolveOptions
	runtimeConfigTimeout time.Duration

	// Validation
	configValidator func(*config.Config) (bool, error)

	// Feature toggles
	observabilityEnabled bool
	auditEnabled         bool

	// Service setup hook — called after infrastructure is ready, before server starts.
	// Use this to create database connections, service layers, Kafka consumers, etc.
	// Returns a SetupResult with middleware to apply to the engine.
	setupFn func(ctx Context) (*SetupResult, error)

	// Route registration hook
	routesFn func(engine *gin.Engine, ctx Context)

	// Extra engine options (service-specific middleware, etc.)
	engineOptions []server.EngineOption

	// Extra server start options
	startOptions []server.StartOption

	// Post-setup hooks that run after setup but before server start.
	// Useful for background workers, backfill tasks, etc.
	postSetupFns []func(ctx Context)

	// Shutdown hooks that run after the server stops.
	shutdownFns []ShutdownFunc
}

// New creates a new App builder.
func New(serviceName, serviceVersion string) *App {
	return &App{
		serviceName:          serviceName,
		serviceVersion:       serviceVersion,
		servicePort:          "8080",
		configFile:           "env/config.json",
		runtimeConfigTimeout: 15 * time.Second,
		observabilityEnabled: true,
		auditEnabled:         true,
	}
}

// WithPort sets the service port.
func (a *App) WithPort(port string) *App {
	a.servicePort = port
	return a
}

// WithConfigFile sets the path to the config file.
func (a *App) WithConfigFile(path string) *App {
	a.configFile = path
	return a
}

// WithConfigOptions adds additional config options (e.g., config.WithDotEnv("")).
func (a *App) WithConfigOptions(opts ...config.Option) *App {
	a.configOptions = append(a.configOptions, opts...)
	return a
}

// WithRuntimeConfig enables runtime configuration resolution from Sentinel/CNM.
func (a *App) WithRuntimeConfig(opts runtimeconfig.ResolveOptions) *App {
	a.runtimeConfigEnabled = true
	a.runtimeConfigOpts = opts
	return a
}

// WithRuntimeConfigTimeout sets the timeout for runtime config resolution.
func (a *App) WithRuntimeConfigTimeout(d time.Duration) *App {
	a.runtimeConfigTimeout = d
	return a
}

// WithConfigValidator sets a function to validate service-specific config.
func (a *App) WithConfigValidator(fn func(*config.Config) (bool, error)) *App {
	a.configValidator = fn
	return a
}

// WithObservability enables/disables observability (tracing, metrics). Enabled by default.
func (a *App) WithObservability(enabled bool) *App {
	a.observabilityEnabled = enabled
	return a
}

// WithAudit enables/disables Kafka audit publishing. Enabled by default.
func (a *App) WithAudit(enabled bool) *App {
	a.auditEnabled = enabled
	return a
}

// OnSetup registers a callback invoked after infrastructure (logger, config,
// observability, audit) is initialized but before the HTTP server starts.
// Use this to create database connections, service layers, and return middleware.
// Return nil for SetupResult if no middleware is needed.
func (a *App) OnSetup(fn func(ctx Context) (*SetupResult, error)) *App {
	a.setupFn = fn
	return a
}

// WithRoutes registers a callback for adding routes to the Gin engine.
func (a *App) WithRoutes(fn func(engine *gin.Engine, ctx Context)) *App {
	a.routesFn = fn
	return a
}

// WithEngineOptions adds additional server.EngineOption values.
func (a *App) WithEngineOptions(opts ...server.EngineOption) *App {
	a.engineOptions = append(a.engineOptions, opts...)
	return a
}

// WithStartOptions adds additional server.StartOption values.
func (a *App) WithStartOptions(opts ...server.StartOption) *App {
	a.startOptions = append(a.startOptions, opts...)
	return a
}

// OnPostSetup registers a callback to run after setup but before server start.
// Useful for launching background goroutines. Pair resource cleanup with
// OnShutdown or SetupResult.Shutdown.
func (a *App) OnPostSetup(fn func(ctx Context)) *App {
	a.postSetupFns = append(a.postSetupFns, fn)
	return a
}

// OnShutdown registers a callback to run after the server stops.
// Hooks are executed in reverse registration order.
func (a *App) OnShutdown(fn ShutdownFunc) *App {
	a.shutdownFns = append(a.shutdownFns, fn)
	return a
}

// Run executes the full service lifecycle: init → setup → serve → shutdown.
func (a *App) Run() {
	// 1. Logger
	log := logger.MustNewDefaultLogger()

	// 2. Config
	configOpts := []config.Option{
		config.WithDefaults(map[string]any{
			"service_name":    a.serviceName,
			"service_version": a.serviceVersion,
		}),
		config.WithFile(a.configFile),
	}
	configOpts = append(configOpts, a.configOptions...)
	cfg := config.New(configOpts...)

	// 3. Runtime config resolution
	if a.runtimeConfigEnabled {
		if a.runtimeConfigOpts.BootstrapPath == "" {
			a.runtimeConfigOpts.BootstrapPath = a.configFile
		}
		runtimeCtx, runtimeCancel := context.WithTimeout(context.Background(), a.runtimeConfigTimeout)
		result, err := runtimeconfig.ResolveInto(runtimeCtx, cfg, log, a.runtimeConfigOpts)
		runtimeCancel()
		if err != nil {
			log.ErrorF("failed to load runtime configuration: %v", err)
			return
		}
		if result.Version != "" {
			log.InfoF("runtime configuration loaded from %s (namespace=%s version=%s)", result.Source, result.NamespaceKey, result.Version)
		} else {
			log.InfoF("runtime configuration loaded from %s (namespace=%s)", result.Source, result.NamespaceKey)
		}
	}

	// 4. Config validation
	if a.configValidator != nil {
		valid, err := a.configValidator(cfg)
		if !valid {
			log.ErrorF("error validating configurations: %v", err)
			return
		}
	}

	// 5. SigNoz endpoint normalization
	if cfg.GetString("SIGNOZ_ENDPOINT") == "" {
		if signozEndpoint := cfg.GetString("SignozEndpoint"); signozEndpoint != "" {
			cfg.Set("SIGNOZ_ENDPOINT", signozEndpoint)
		}
	}

	// 6. Observability (SigNoz logger + tracing)
	var obs observability.ObservabilityIface
	if a.observabilityEnabled {
		if signozLogger, loggerErr := observability.NewLoggerWithSigNoz(cfg, logger.LoggerOptions{
			Level:        "info",
			Encoding:     "console",
			EnableCaller: true,
			EnableStack:  false,
		}); loggerErr != nil {
			log.WarnF("failed to initialize SigNoz log exporter: %v", loggerErr)
		} else {
			log = signozLogger
			log.InfoF("SigNoz log exporter enabled")
		}

		var obsErr error
		obs, obsErr = observability.New(log, cfg)
		if obsErr != nil {
			log.WarnF("failed to initialize observability: %v", obsErr)
		} else {
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer shutdownCancel()
				if shutdownErr := obs.Shutdown(shutdownCtx); shutdownErr != nil {
					log.WarnF("failed to shutdown observability: %v", shutdownErr)
				}
			}()
		}
	}

	// 7. Audit publisher
	var auditPublisher audit.Publisher
	if a.auditEnabled {
		auditPublisher = audit.NewKafkaPublisherFromConfig(log, cfg)
		defer func() {
			if err := auditPublisher.Close(); err != nil {
				log.WarnF("failed to close audit publisher: %v", err)
			}
		}()
	}

	// 8. Validator
	v := validator.New()

	// Build context for hooks
	appCtx := Context{
		Logger:         log,
		Config:         cfg,
		Validator:      v,
		AuditPublisher: auditPublisher,
		Observability:  obs,
	}

	// 9. Service-specific setup
	var setupResult *SetupResult
	if a.setupFn != nil {
		var err error
		setupResult, err = a.setupFn(appCtx)
		if err != nil {
			log.ErrorF("failed to setup service: %v", err)
			return
		}
	}
	defer func() {
		runShutdownHooks(log, appCtx, a.shutdownFns, "app")
		if setupResult != nil {
			runShutdownHooks(log, appCtx, setupResult.Shutdown, "setup")
		}
	}()

	// 10. Build engine with standard middleware
	engineOpts := []server.EngineOption{
		server.WithLogger(log),
		server.WithRecovery(true),
		server.WithPrometheus(true),
		server.WithRateLimit(BuildRateLimitConfig(cfg)),
		server.WithCors(BuildCorsConfig(cfg)),
		server.WithSecurityHeaders(servermiddleware.DefaultSecurityHeadersConfig()),
		server.WithValidator(v),
	}
	if a.auditEnabled && auditPublisher != nil {
		engineOpts = append(engineOpts, server.WithAudit(audit.NewMiddlewareConfig(cfg, a.serviceName, auditPublisher, log)))
	}
	// Add service middleware from setup result
	if setupResult != nil && len(setupResult.Middleware) > 0 {
		engineOpts = append(engineOpts, server.WithMiddleware(setupResult.Middleware...))
	}
	engineOpts = append(engineOpts, a.engineOptions...)
	engine := server.NewEngine(engineOpts...)

	// Observability middleware
	if obs != nil {
		serviceName := cfg.GetString("service_name")
		if serviceName == "" {
			serviceName = a.serviceName
		}
		engine.Use(observability.GinMiddleware(serviceName))
		log.InfoF("observability middleware enabled for service: %s", serviceName)
	}

	// 11. Register routes
	if a.routesFn != nil {
		a.routesFn(engine, appCtx)
	}

	// Log registered routes
	for _, r := range engine.Routes() {
		log.InfoF("route registered: %s %s", r.Method, r.Path)
	}

	// 12. Post-setup hooks (background workers, etc.)
	for _, fn := range a.postSetupFns {
		fn(appCtx)
	}

	// 13. Start server
	startOpts := []server.StartOption{
		server.StartWithLogger(log),
		server.StartWithConfig(cfg),
		server.StartWithAddr(":" + a.servicePort),
	}
	startOpts = append(startOpts, a.startOptions...)

	if err := server.Start(engine, startOpts...); err != nil {
		log.ErrorF("failed to start server: %v", err)
	}
}

func runShutdownHooks(log logger.LogManager, ctx Context, hooks []ShutdownFunc, scope string) {
	for i := len(hooks) - 1; i >= 0; i-- {
		hook := hooks[i]
		if hook == nil {
			continue
		}
		if err := hook(ctx); err != nil {
			log.WarnF("%s shutdown hook failed: %v", scope, err)
		}
	}
}

// BuildRateLimitConfig creates a RateLimitConfig from the service config.
// Exported so services can customize or override.
func BuildRateLimitConfig(cfg *config.Config) *servermiddleware.RateLimitConfig {
	enabled := cfg.GetBoolD("RateLimitEnabled", true)

	rps := cfg.GetIntD("RateLimitRPS", 120)
	if rps <= 0 {
		rps = 120
	}

	burst := cfg.GetIntD("RateLimitBurst", 240)
	if burst <= 0 {
		burst = 240
	}

	cleanup := cfg.GetDurationD("RateLimitCleanupInterval", 5*time.Minute)
	if cleanup <= 0 {
		cleanup = 5 * time.Minute
	}

	return servermiddleware.NewRateLimitConfig(enabled, float64(rps), burst, cleanup)
}

// BuildCorsConfig creates a CorsConfig from the service config.
// Exported so services can customize or override.
func BuildCorsConfig(cfg *config.Config) servermiddleware.CorsConfig {
	allowedOrigins := cfg.GetStringD("CorsAllowedOrigins", "*")
	origins := strings.Split(allowedOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	cors := servermiddleware.DefaultCorsConfig()
	cors.AllowOrigins = origins
	cors.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "X-Tenant-ID"}
	cors.ExposeHeaders = []string{"X-Request-ID"}

	if allowedOrigins != "*" {
		cors.AllowCredentials = true
	}

	return cors
}
