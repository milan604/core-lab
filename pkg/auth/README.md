# Auth Package

This package provides JWT-based authorization middleware for Gin applications with bitmask permission checking.

## Overview

The auth package is designed to be self-contained and easily migratable to `core-lab`. It provides:

- **JWT Token Verification**: RSA-based JWT token verification with configurable issuer and audience validation
- **Bitmask Permission Checking**: Efficient permission checking using bitmask values from JWT claims
- **Service Token Support**: Automatic bypass for service-to-service tokens
- **Gin Middleware Integration**: Seamless integration with Gin framework

## Components

### Authorizer

The main component that handles JWT verification and permission checking.

```go
authorizer, err := auth.NewAuthorizer(cfg, log)
if err != nil {
    // handle error
}

// Use in routes
router.GET("/api/resource", authorizer.RequirePermission("PMS-PRO-CRE"), handler)
```

### Claims

Represents the JWT claims extracted from the token:

```go
type Claims struct {
    Subject            string
    IdentityID         string
    RoleID             string
    TokenUse           string
    ServicePermissions map[string]int64 // service -> bitmask
    Raw                map[string]any
}
```

### PermissionLookup Interface

Services must implement this interface to provide permission metadata:

```go
type PermissionLookup interface {
    LookupPermission(code string) (permissions.Metadata, bool)
}
```

## Configuration

The authorizer requires the following configuration keys:

- `RSAPublicKey`: Base64-encoded RSA public key (required)
- `SentinelTokenIssuer`: JWT issuer to validate (optional)
- `SentinelTokenAudience`: Comma-separated list of audiences to validate (optional)

## Usage

### 1. Initialize Authorizer

```go
import (
    "github.com/milan604/core-lab/pkg/config"
    "github.com/milan604/core-lab/pkg/logger"
    "ecompulse/core/pkg/auth"
)

cfg := config.New(...)
log := logger.MustNewDefaultLogger()

authorizer, err := auth.NewAuthorizer(cfg, log)
if err != nil {
    // handle error
}
```

### 2. Use in Routes

```go
router.GET("/api/resource", authorizer.RequirePermission("PMS-PRO-CRE"), handler)
```

### 3. Retrieve Claims in Handlers

```go
import "ecompulse/core/pkg/auth"

func handler(c *gin.Context) {
    claims, ok := auth.GetClaims(c)
    if !ok {
        // claims not found
        return
    }
    
    userID := claims.Subject
    roleID := claims.RoleID
    // ...
}
```

## Service Integration

Services must:

1. **Implement PermissionLookup**: The service must implement the `PermissionLookup` interface
2. **Set Service in Context**: Use middleware to set the service in the Gin context with key `auth.CtxMiddlewareServiceKey`

Example:

```go
// In middleware
func SetService(service *service.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Set(string(auth.CtxMiddlewareServiceKey), service)
        c.Next()
    }
}
```

## Permission Format

Permissions are checked using a code format: `SERVICE-CATEGORY-SUBCATEGORY`

Example: `PMS-PRO-CRE` (Product Management Service - Products - Create)

The permission code is used to:
1. Look up permission metadata from the permission store
2. Extract the service name and bit value
3. Check if the user's service permissions bitmask includes the required bit

## Service Tokens

Service tokens (tokens with `token_use: "service"`) automatically bypass permission checks. This allows service-to-service communication without explicit permission grants.

## Migration to core-lab

This package is designed to be easily migrated to `core-lab/pkg/auth`. To migrate:

1. Copy the entire `core/pkg/auth` directory to `core-lab/pkg/auth`
2. Update imports from `ecompulse/core/pkg/auth` to `github.com/milan604/core-lab/pkg/auth`
3. Ensure all dependencies are available in `core-lab`:
   - `github.com/gin-gonic/gin`
   - `github.com/golang-jwt/jwt`
   - `github.com/milan604/core-lab/pkg/logger`
   - `github.com/milan604/core-lab/pkg/permissions`

## Dependencies

- `github.com/gin-gonic/gin`: Gin web framework
- `github.com/golang-jwt/jwt`: JWT token parsing and verification
- `github.com/milan604/core-lab/pkg/logger`: Logging interface
- `github.com/milan604/core-lab/pkg/permissions`: Permission metadata types

## Context Keys

The package defines the following context keys:

- `CtxAuthClaims`: Key for storing authenticated claims in context
- `CtxMiddlewareServiceKey`: Key for storing service in context

These are exported as constants for use in middleware and handlers.

