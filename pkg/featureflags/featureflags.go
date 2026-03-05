package featureflags

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const contextKey = "tenant_feature_flags"

// Flags is the canonical type for feature flags (flag-name to value).
type Flags map[string]interface{}

// Set stores the flags in the Gin context.
func Set(c *gin.Context, f Flags) {
	c.Set(contextKey, f)
}

// Get retrieves the flags from the Gin context.
func Get(c *gin.Context) Flags {
	v, exists := c.Get(contextKey)
	if !exists {
		return nil
	}
	f, _ := v.(Flags)
	return f
}

// Enabled returns true when the named flag exists and is truthy.
func Enabled(c *gin.Context, flag string) bool {
	f := Get(c)
	if f == nil {
		return false
	}
	return isTruthy(f[flag])
}

// Value returns the raw value of a flag (useful for A/B variant strings).
func Value(c *gin.Context, flag string) (interface{}, bool) {
	f := Get(c)
	if f == nil {
		return nil, false
	}
	v, exists := f[flag]
	return v, exists
}

// Middleware extracts feature flags from JWT claims and injects them into context.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, exists := c.Get("claims_raw")
		if !exists {
			c.Next()
			return
		}
		rawMap, _ := raw.(map[string]interface{})
		if rawMap == nil {
			c.Next()
			return
		}
		ffRaw, exists := rawMap["feature_flags"]
		if !exists {
			c.Next()
			return
		}
		ffMap, _ := ffRaw.(map[string]interface{})
		if ffMap == nil {
			c.Next()
			return
		}
		Set(c, Flags(ffMap))
		c.Next()
	}
}

// RequireFeature returns middleware that aborts with 403 when the flag is not enabled.
func RequireFeature(flag string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !Enabled(c, flag) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "feature_disabled",
				"message": "Feature '" + flag + "' is not enabled for this tenant",
			})
			return
		}
		c.Next()
	}
}

func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	case float64:
		return val != 0
	case int:
		return val != 0
	case int64:
		return val != 0
	default:
		return true
	}
}
