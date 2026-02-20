package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CorsConfig defines Cross-Origin Resource Sharing settings for the server.
type CorsConfig struct {
	Enabled          bool
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// DefaultCorsConfig returns a safe default CORS configuration.
func DefaultCorsConfig() CorsConfig {
	return CorsConfig{
		Enabled:          true,
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
}

// originSet builds a set of allowed origins for fast lookup. Empty slice means allow none.
func originSet(origins []string) map[string]bool {
	m := make(map[string]bool, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o != "" {
			m[o] = true
		}
	}
	return m
}

// CORSMiddleware returns a gin.HandlerFunc that applies CORS rules.
// When AllowOrigins contains multiple specific origins, the request's Origin is reflected
// if it is in the list (per CORS spec, the header must be a single origin or "*").
func CORSMiddleware(cfg CorsConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	allowAll := false
	var allowed map[string]bool
	if len(cfg.AllowOrigins) == 1 && strings.TrimSpace(cfg.AllowOrigins[0]) == "*" {
		allowAll = true
	} else {
		allowed = originSet(cfg.AllowOrigins)
	}

	allowMethods := strings.Join(cfg.AllowMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := cfg.MaxAge.Seconds()
	maxAgeStr := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(maxAge, 'f', -1, 64), "0"), ".")

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if allowAll {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && allowed[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", allowMethods)
		c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		if exposeHeaders != "" {
			c.Writer.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
		}
		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		c.Writer.Header().Set("Access-Control-Max-Age", maxAgeStr)

		// Handle preflight OPTIONS request
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
