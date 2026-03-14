package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	coreaudit "github.com/milan604/core-lab/pkg/audit"
	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/logger"
	middleware "github.com/milan604/core-lab/pkg/server/middleware"

	"github.com/gin-gonic/gin"
)

// NewEngine creates a Gin engine with recommended middleware ordering and modular options.
func NewEngine(opts ...EngineOption) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// Collect options
	var opt engineOptions
	for _, o := range opts {
		o(&opt)
	}

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

	// 4. Request Audit (optional)
	if opt.auditConfig != nil && opt.auditConfig.Enabled {
		engine.Use(coreaudit.Middleware(*opt.auditConfig))
	}

	// 5. Security Headers (optional)
	if opt.securityHeadersConfig.Enabled {
		engine.Use(middleware.SecurityHeadersMiddleware(opt.securityHeadersConfig))
	}

	// 6. CORS (optional)
	if opt.corsConfig.Enabled {
		engine.Use(middleware.CORSMiddleware(opt.corsConfig))
	}

	// 7. Rate Limiting (optional)
	if opt.rateLimitConfig != nil && opt.rateLimitConfig.Enabled {
		engine.Use(opt.rateLimitConfig.Middleware())
	}

	// 8. Tenant Status Check (optional — blocks suspended/cancelled tenants)
	if opt.tenantStatusConfig.Enabled {
		engine.Use(middleware.TenantStatusMiddleware(opt.tenantStatusConfig))
	}

	// 9. Prometheus (optional)
	if opt.prometheus {
		prom := middleware.NewPrometheusCollector("/metrics")
		engine.Use(prom.PrometheusMiddleware())
		prom.RegisterMetricsEndpoint(engine)
	}

	// 10. Error Handler
	engine.Use(middleware.ErrorHandlerMiddleware())

	// 11. User-provided middlewares
	for _, m := range opt.addMiddleware {
		engine.Use(m)
	}

	// 12. Recovery (optional, last)
	if opt.recovery {
		engine.Use(middleware.RecoveryMiddleware(logMgr))
	}

	return engine
}

func resolveAddress(so *startOptions) string {
	addr := so.addr
	if addr == "" && so.cfg != nil {
		host := so.cfg.GetStringD("service.endpoint", "0.0.0.0")
		port := so.cfg.GetStringD("service.port", "8080")
		addr = fmt.Sprintf("%s:%s", host, port)
	}
	if addr == "" {
		addr = ":8080"
	}
	return addr
}

func logServiceInfo(addr string, logger logger.LogManager) {
	svcInfo, _ := config.LoadServiceConfig(".serviceconfig")
	var block string
	if svcInfo != nil {
		block = fmt.Sprintf(
			"\n==============================\n"+
				" Service: %s (%s)\n"+
				" Version: %s\n"+
				"------------------------------\n"+
				" Description: %s\n"+
				" Author:      %s\n"+
				" Contact:     %s\n"+
				" Repository:  %s\n"+
				"------------------------------\n"+
				" Listening on: %s\n"+
				"==============================\n",
			svcInfo.ServiceName,
			svcInfo.ServiceCode,
			svcInfo.Version,
			svcInfo.Description,
			svcInfo.Author,
			svcInfo.Contact,
			svcInfo.Repository,
			addr,
		)
	} else {
		block = fmt.Sprintf("\n==============================\n Service starting on %s\n==============================\n", addr)
	}
	fmt.Print(block)
}

func startHTTPServer(srv *http.Server, so *startOptions) {
	logServiceInfo(srv.Addr, so.logger)
	fmt.Println("Server started 🚀")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		if so.logger != nil {
			so.logger.ErrorF("ListenAndServe error: %v", err)
		} else {
			log.Printf("ListenAndServe error: %v", err)
		}
	}
}

func startTLSServer(srv *http.Server, so *startOptions) {
	if _, err := os.Stat(so.tlsCertFile); err != nil {
		log.Printf("TLS cert file error: %v", err)
		return
	}
	if _, err := os.Stat(so.tlsKeyFile); err != nil {
		log.Printf("TLS key file error: %v", err)
		return
	}

	// mTLS: require and verify client certificates if CA file is provided
	if so.tlsClientCAFile != "" {
		caCert, err := os.ReadFile(so.tlsClientCAFile)
		if err != nil {
			log.Printf("mTLS CA file error: %v", err)
			return
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			log.Print("mTLS: failed to parse CA certificate")
			return
		}
		clientAuthMode := tls.RequireAndVerifyClientCert
		if so.tlsClientAuthMode == 2 {
			clientAuthMode = tls.VerifyClientCertIfGiven
		}
		srv.TLSConfig = &tls.Config{
			ClientCAs:  caPool,
			ClientAuth: clientAuthMode,
			MinVersion: tls.VersionTLS12,
		}
		if so.tlsClientAuthMode == 2 {
			fmt.Println("Server started 🚀 (TLS + optional client certificates)")
		} else {
			fmt.Println("Server started 🚀 (mTLS)")
		}
	} else {
		fmt.Println("Server started 🚀 (TLS)")
	}

	logServiceInfo(srv.Addr, so.logger)
	if err := srv.ListenAndServeTLS(so.tlsCertFile, so.tlsKeyFile); err != nil && err != http.ErrServerClosed {
		if so.logger != nil {
			so.logger.ErrorF("ListenAndServeTLS error: %v", err)
		} else {
			log.Printf("ListenAndServeTLS error: %v", err)
		}
	}
}

func handleShutdown(srv *http.Server, so *startOptions) error {
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

// Start runs the HTTP server with graceful shutdown. Blocks until shutdown or error.
func Start(engine *gin.Engine, opts ...StartOption) error {
	so := &startOptions{shutdownTimeout: 15 * time.Second}
	for _, o := range opts {
		o(so)
	}

	addr := resolveAddress(so)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if so.logger != nil {
			so.logger.ErrorF("port %s is already in use: %v", addr, err)
		} else {
			log.Printf("port %s is already in use: %v", addr, err)
		}
		return err
	}
	ln.Close()

	srv := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if so.tlsCertFile != "" && so.tlsKeyFile != "" {
			startTLSServer(srv, so)
		} else {
			startHTTPServer(srv, so)
		}
	}()

	return handleShutdown(srv, so)
}
