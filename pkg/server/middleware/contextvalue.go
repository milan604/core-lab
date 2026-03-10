package server

import (
	"github.com/gin-gonic/gin"
	"github.com/milan604/core-lab/pkg/auth"
)

// SetContextValue stores a fixed value in Gin context for the duration of the request.
func SetContextValue(key string, value any) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(key, value)
		c.Next()
	}
}

// GetContextValue retrieves a typed value from Gin context and panics if missing.
func GetContextValue[T any](c *gin.Context, key string) T {
	return c.MustGet(key).(T)
}

// SetServiceContext stores the application service under the shared core-lab context key.
func SetServiceContext(service any) gin.HandlerFunc {
	return SetContextValue(string(auth.CtxMiddlewareServiceKey), service)
}

// GetServiceContext retrieves the application service from the shared core-lab context key.
func GetServiceContext[T any](c *gin.Context) T {
	return GetContextValue[T](c, string(auth.CtxMiddlewareServiceKey))
}
