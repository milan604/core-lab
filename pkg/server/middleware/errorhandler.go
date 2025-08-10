package server

import (
	"github.com/milan604/core-lab/pkg/response"

	"github.com/gin-gonic/gin"
)

// ErrorHandlerMiddleware inspects c.Errors and writes JSON error if present.
// This middleware should be used after handlers (i.e., added before routes are handled) OR
// it can be used to centralize c.Error -> response translation.
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		// If handler already wrote a response (c.Writer.Status()!=0) we skip only if response written.
		// But we prioritize c.Errors to return a standardized error shape.
		if len(c.Errors) > 0 {
			last := c.Errors.Last()
			if last == nil || last.Err == nil {
				return
			}
			response.HandleError(c, last.Err)
			// Abort to avoid any subsequent rendering
			c.Abort()
			return
		}
	}
}
