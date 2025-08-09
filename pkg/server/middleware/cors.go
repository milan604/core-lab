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

// CORSMiddleware returns a gin.HandlerFunc that applies CORS rules.
func CORSMiddleware(cfg CorsConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	allowOrigins := strings.Join(cfg.AllowOrigins, ", ")
	allowMethods := strings.Join(cfg.AllowMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := cfg.MaxAge.Seconds()

	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigins)
		c.Writer.Header().Set("Access-Control-Allow-Methods", allowMethods)
		c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		if exposeHeaders != "" {
			c.Writer.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
		}
		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		c.Writer.Header().Set("Access-Control-Max-Age", strings.TrimRight(strings.TrimRight(strconv.FormatFloat(maxAge, 'f', -1, 64), "0"), "."))

		// Handle preflight OPTIONS request
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
