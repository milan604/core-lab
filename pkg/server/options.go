package server

import (
	"corelab/pkg/config"
	"corelab/pkg/logger"
	"time"

	middleware "corelab/pkg/server/middleware"

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
	addr        string
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

// NewEngine builds a gin engine with recommended middlewares and returns it.
// Accepts options such as logger, custom middleware, recovery toggle, CORS config, metrics toggle.
type EngineOption func(*engineOptions)

type engineOptions struct {
	logger          logger.LogManager
	recovery        bool
	corsConfig      middleware.CorsConfig
	prometheus      bool
	rateLimitConfig *middleware.RateLimitConfig
	addMiddleware   []gin.HandlerFunc
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

func WithMiddleware(m ...gin.HandlerFunc) EngineOption {
	return func(e *engineOptions) { e.addMiddleware = append(e.addMiddleware, m...) }
}
