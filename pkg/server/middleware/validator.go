package server

import (
	"github.com/gin-gonic/gin"
	"github.com/milan604/core-lab/pkg/validator"
)

const ValidatorKey = "corelab_validator"

func ValidatorMiddleware(vi *validator.Validator) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ValidatorKey, vi)
		c.Next()
	}
}

// GetValidator retrieves the validator from Gin context.
func GetValidator(c *gin.Context) (*validator.Validator, bool) {
	v, ok := c.Get(ValidatorKey)
	if !ok {
		return nil, false
	}
	vi, ok := v.(*validator.Validator)
	return vi, ok
}
