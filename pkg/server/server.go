package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"corelab/pkg/logger"
	middleware "corelab/pkg/server/middleware"

	"github.com/gin-gonic/gin"
)

func NewEngine(opts ...EngineOption) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	// base minimal middlewares: Recovery should be attached if requested by options
	// additional middlewares attached below via options

	var opt engineOptions
	for _, o := range opts {
		o(&opt)
	}

	// --- Stable Middleware Ordering ---
	// 1. Request ID
	engine.Use(middleware.RequestIDMiddleware())

	// 2. Access Logger (fallback to default if not provided)
	logMgr := opt.logger
	if logMgr == nil {
		logMgr = logger.MustNewDefaultLogger()
	}
	engine.Use(middleware.AccessLoggerMiddleware(logMgr))

	// 3. App Logger Injector
	engine.Use(middleware.AppLoggerMiddleware(logMgr))

	// 4. CORS (if enabled)
	if opt.corsConfig.Enabled {
		engine.Use(middleware.CORSMiddleware(opt.corsConfig))
	}

	// 5. Rate Limiting (if enabled)
	if opt.rateLimitConfig.Enabled {
		engine.Use(opt.rateLimitConfig.Middleware())
	}

	// 6. Prometheus (optional, extensible)
	if opt.prometheus {
		prom := middleware.NewPrometheusCollector("/metrics")
		engine.Use(prom.PrometheusMiddleware())
		prom.RegisterMetricsEndpoint(engine)
	}

	// 7. Error Handler
	engine.Use(middleware.ErrorHandlerMiddleware())

	// 8. User-provided middlewares
	for _, m := range opt.addMiddleware {
		engine.Use(m)
	}

	// 9. Recovery (last, to catch panics)
	if opt.recovery {
		engine.Use(middleware.RecoveryMiddleware(logMgr))
	}

	return engine
}

// Start starts the HTTP server with graceful shutdown. It blocks until shutdown completes or error occurs.
func Start(engine *gin.Engine, opts ...StartOption) error {
	// default options
	so := &startOptions{
		shutdownTimeout: 15 * time.Second,
	}

	for _, o := range opts {
		o(so)
	}

	// determine address: look at provided addr option, then config, then env fallback
	addr := so.addr
	if addr == "" && so.cfg != nil {
		// expected keys: service.endpoint, service.port
		host := so.cfg.GetStringD("service.endpoint", "0.0.0.0")
		port := so.cfg.GetStringD("service.port", "8080")
		addr = fmt.Sprintf("%s:%s", host, port)
	}
	if addr == "" {
		addr = ":8080"
	}

	// ensure addr available
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if so.logger != nil {
			so.logger.ErrorF("port %s is already in use: %v", addr, err)
		} else {
			log.Printf("port %s is already in use: %v", addr, err)
		}
		return err
	}
	// close listener (we will use http.Server with same addr)
	ln.Close()

	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
		// sensible timeouts - can be tuned via config or StartOption extension
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// TLS support
	if so.tlsCertFile != "" && so.tlsKeyFile != "" {
		// validate files exist
		if _, err := os.Stat(so.tlsCertFile); err != nil {
			return err
		}
		if _, err := os.Stat(so.tlsKeyFile); err != nil {
			return err
		}
		// start TLS listener in goroutine
		go func() {
			if so.logger != nil {
				so.logger.InfoF("starting TLS server on %s", addr)
			}
			if err := srv.ListenAndServeTLS(so.tlsCertFile, so.tlsKeyFile); err != nil && err != http.ErrServerClosed {
				if so.logger != nil {
					so.logger.ErrorF("ListenAndServeTLS error: %v", err)
				} else {
					log.Printf("ListenAndServeTLS error: %v", err)
				}
			}
		}()
	} else {
		go func() {
			if so.logger != nil {
				so.logger.InfoF("starting server on %s", addr)
			} else {
				log.Printf("starting server on %s", addr)
			}
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				if so.logger != nil {
					so.logger.ErrorF("ListenAndServe error: %v", err)
				} else {
					log.Printf("ListenAndServe error: %v", err)
				}
			}
		}()
	}

	// graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	if so.logger != nil {
		so.logger.InfoF("shutdown initiated")
	} else {
		log.Print("shutdown initiated")
	}

	ctx, cancel := context.WithTimeout(context.Background(), so.shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		if so.logger != nil {
			so.logger.ErrorF("server shutdown error: %v", err)
		} else {
			log.Printf("server shutdown error: %v", err)
		}
		return err
	}

	if so.logger != nil {
		so.logger.InfoF("server stopped gracefully")
	} else {
		log.Print("server stopped gracefully")
	}
	return nil
}
