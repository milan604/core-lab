package middleware

import (
	"net/http"

	"corelab/pkg/auth/openfga"

	"github.com/gin-gonic/gin"
)

// TupleResolver extracts the (user, relation, object) tuple for authorization checks.
type TupleResolver func(c *gin.Context) (user, relation, object string, err error)

// RequireAuthZ checks authorization using the provided Authorizer and resolver.
func RequireAuthZ(authz openfga.Authorizer, resolve TupleResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, relation, object, err := resolve(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid_authorization_tuple"})
			return
		}
		ok, err := authz.Check(c.Request.Context(), user, relation, object)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authz_error"})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
