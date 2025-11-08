package auth

import (
	"github.com/gin-gonic/gin"
)

// GetClaims retrieves the verified Claims from the request context.
func GetClaims(c *gin.Context) (Claims, bool) {
	val, exists := c.Get(string(CtxAuthClaims))
	if !exists {
		return Claims{}, false
	}
	claims, ok := val.(Claims)
	return claims, ok
}
