package server

import (
	"time"

	coreaudit "github.com/milan604/core-lab/pkg/audit"
	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/logger"
	"github.com/milan604/core-lab/pkg/validator"

	middleware "github.com/milan604/core-lab/pkg/server/middleware"

	"github.com/gin-gonic/gin"
)

// StartOption configures Start behavior (functional options)
type StartOption func(*startOptions)

type startOptions struct {
	cfg    *config.Config
	logger logger.LogManager

	// server-level: graceful shutdown timeout
	shutdownTimeout time.Duration

	// TLS
	tlsCertFile string
	tlsKeyFile  string

	// mTLS — optional CA for verifying client certificates
	tlsClientCAFile   string
	tlsClientAuthMode int

	addr string
}

// StartWithConfig passes config to the server startup
func StartWithConfig(c *config.Config) StartOption {
	return func(o *startOptions) { o.cfg = c }
}

// StartWithLogger passes a logger
func StartWithLogger(l logger.LogManager) StartOption {
	return func(o *startOptions) { o.logger = l }
}

// StartWithShutdownTimeout custom shutdown timeout
func StartWithShutdownTimeout(d time.Duration) StartOption {
	return func(o *startOptions) { o.shutdownTimeout = d }
}

// StartWithAddr override listen address (host:port)
func StartWithAddr(addr string) StartOption {
	return func(o *startOptions) { o.addr = addr }
}

// StartWithTLS enables TLS with cert/key files
func StartWithTLS(certFile, keyFile string) StartOption {
	return func(o *startOptions) {
		o.tlsCertFile = certFile
		o.tlsKeyFile = keyFile
	}
}

// StartWithMTLS enables mutual TLS. The server will require and verify
// client certificates against the provided CA file. Must be used together
// with StartWithTLS.
func StartWithMTLS(clientCAFile string) StartOption {
	return func(o *startOptions) {
		o.tlsClientCAFile = clientCAFile
		o.tlsClientAuthMode = 1
	}
}

// StartWithOptionalMTLS enables client-certificate verification without making
// client certificates mandatory for every route. Pair with route middleware
// that requires verified client certificates where needed.
func StartWithOptionalMTLS(clientCAFile string) StartOption {
	return func(o *startOptions) {
		o.tlsClientCAFile = clientCAFile
		o.tlsClientAuthMode = 2
	}
}

// NewEngine builds a gin engine with recommended middlewares and returns it.
// Accepts options such as logger, custom middleware, recovery toggle, CORS config, metrics toggle.
type EngineOption func(*engineOptions)

type engineOptions struct {
	logger                logger.LogManager
	recovery              bool
	corsConfig            middleware.CorsConfig
	prometheus            bool
	rateLimitConfig       *middleware.RateLimitConfig
	securityHeadersConfig middleware.SecurityHeadersConfig
	tenantStatusConfig    middleware.TenantStatusConfig
	auditConfig           *coreaudit.MiddlewareConfig
	addMiddleware         []gin.HandlerFunc
}

// Enables rate limiting with custom parameters
func WithRateLimit(cfg *middleware.RateLimitConfig) EngineOption {
	return func(e *engineOptions) {
		e.rateLimitConfig = cfg
	}
}

// Engine option helpers
func WithLogger(l logger.LogManager) EngineOption {
	return func(e *engineOptions) { e.logger = l }
}

func WithRecovery(enabled bool) EngineOption {
	return func(e *engineOptions) { e.recovery = enabled }
}

func WithCors(c middleware.CorsConfig) EngineOption {
	return func(e *engineOptions) { e.corsConfig = c }
}

func WithPrometheus(enabled bool) EngineOption {
	return func(e *engineOptions) { e.prometheus = enabled }
}

func WithValidator(vi *validator.Validator) EngineOption {
	return func(e *engineOptions) {
		e.addMiddleware = append(e.addMiddleware, middleware.ValidatorMiddleware(vi))
	}
}

// WithSecurityHeaders enables standard security response headers.
// Pass middleware.DefaultSecurityHeadersConfig() for safe defaults.
func WithSecurityHeaders(cfg middleware.SecurityHeadersConfig) EngineOption {
	return func(e *engineOptions) {
		e.securityHeadersConfig = cfg
	}
}

// WithTenantStatus enables the tenant suspension middleware.
// Pass middleware.DefaultTenantStatusConfig() to block suspended/cancelled/inactive tenants.
func WithTenantStatus(cfg middleware.TenantStatusConfig) EngineOption {
	return func(e *engineOptions) {
		e.tenantStatusConfig = cfg
	}
}

func WithAudit(cfg coreaudit.MiddlewareConfig) EngineOption {
	return func(e *engineOptions) {
		e.auditConfig = &cfg
	}
}

func WithMiddleware(m ...gin.HandlerFunc) EngineOption {
	return func(e *engineOptions) { e.addMiddleware = append(e.addMiddleware, m...) }
}
