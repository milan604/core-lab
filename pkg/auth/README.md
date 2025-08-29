# Auth

Practical auth utilities for Gin services:


## JWT auth quick start

```go
import (
  "github.com/gin-gonic/gin"
  "corelab/pkg/auth/middleware"
)

r := gin.New()
r.Use(middleware.JWTAuth(middleware.JWTConfig{
  Issuer:      "https://keycloak.example.com/realms/myrealm",
  Audience:    "my-client-id",
  JWKSURL:     "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/certs",
  CacheTTL:    10 * time.Minute,
  AllowedAlgs: []string{"RS256"},
}))

r.GET("/me", middleware.RequireScopes("read:profile"), func(c *gin.Context) {
  claims, _ := middleware.GetClaims(c)
  c.JSON(200, gin.H{"sub": claims.Subject(), "scopes": claims.Scopes()})
})
```

## Roles and scopes


For Keycloak roles, use the helpers from `auth/keycloak`:

```go
import kc "corelab/pkg/auth/keycloak"

roles := kc.ExtractRoles(claims.All()) // realm + resource roles
```

## OpenFGA

Define an authorizer that satisfies:

```go
type Authorizer interface {
  Check(ctx context.Context, user, relation, object string) (bool, error)
}
```

Use the in-memory authorizer in tests, or wire your own client.

```go
authz := openfga.NewMemoryAuthorizer()
authz.Allow("user:123", "viewer", "doc:1")
r.GET("/docs/:id", middleware.RequireAuthZ(authz, func(c *gin.Context) (user, relation, object string, err error) {
  claims, _ := middleware.GetClaims(c)
  return claims.Subject(), "viewer", "doc:"+c.Param("id"), nil
}), handler)
```

Private and proprietary. All rights reserved.
