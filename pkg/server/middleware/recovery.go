package server

import (
	"fmt"
	"runtime/debug"

	"corelab/pkg/logger"

	"github.com/gin-gonic/gin"
)

type RecoveryOptions struct {
	LogStack bool
	OnPanic  func(c *gin.Context, err any)
}

func RecoveryMiddleware(l logger.LogManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// log with stacktrace
				fields := []any{
					"log_type", "panic",
					"path", c.Request.URL.Path,
				}
				if l != nil {
					entry := l.With(fields...)
					entry.ErrorF("panic recovered: %v\n%s", r, string(debug.Stack()))
				} else {
					fmt.Printf("panic recovered: %v\n%s", r, debug.Stack())
				}

				// abort with 500
				c.AbortWithStatusJSON(500, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}
