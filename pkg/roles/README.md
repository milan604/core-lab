# Roles Package

A generic, reusable roles management package for Go services with permission assignment.

## Overview

This package provides:
- **Role Catalog**: Management of role definitions with permissions
- **Bootstrap/Sync**: Synchronization with external role services (e.g., Sentinel)
- **Permission Assignment**: Automatic assignment of permissions to roles
- **Role Validation**: Validation of role IDs against Sentinel service
- **Self-contained Models**: All models are defined within the package

## Features

- Role definition management with permission references
- Bootstrap synchronization with external services
- Bulk role validation
- Automatic permission assignment to roles
- Integration with permissions package
- Direct integration with core-lab's HTTP and config packages

## Models

### Definition

Represents a role definition with its permissions:

```go
type Definition struct {
    RoleID      string                  // UUID from Sentinel service
    Name        string                  // Role name (e.g., "Admin", "Manager", "User")
    Permissions []permissions.Reference // List of permission references
}
```

### Catalog

Manages a collection of role definitions:

```go
type Catalog struct {
    definitions []Definition
}
```

## Usage

### 1. Define Roles

```go
import (
    "github.com/milan604/core-lab/pkg/roles"
    "github.com/milan604/core-lab/pkg/permissions"
)

definitions := []roles.Definition{
    {
        RoleID: "550e8400-e29b-41d4-a716-446655440000", // UUID from Sentinel
        Name:   "Admin",
        Permissions: []permissions.Reference{
            {
                Service:     "USR",
                Category:    "users",
                SubCategory: "create",
            },
            {
                Service:     "USR",
                Category:    "users",
                SubCategory: "delete",
            },
            // ... more permissions
        },
    },
    {
        RoleID: "660e8400-e29b-41d4-a716-446655440001",
        Name:   "Manager",
        Permissions: []permissions.Reference{
            {
                Service:     "USR",
                Category:    "users",
                SubCategory: "view",
            },
            // ... more permissions
        },
    },
}

catalog := roles.NewCatalog(definitions)
```

### 2. Bootstrap Roles

The `Bootstrap` function validates role IDs and assigns permissions to roles in Sentinel:

```go
import (
    "context"
    "github.com/milan604/core-lab/pkg/config"
    "github.com/milan604/core-lab/pkg/logger"
    "github.com/milan604/core-lab/pkg/roles"
)

cfg := config.New(...)
log := logger.MustNewDefaultLogger()

ctx := context.Background()

// Bootstrap roles - validates role IDs and assigns permissions
if err := roles.Bootstrap(ctx, definitions, cfg, log); err != nil {
    log.FatalF("Failed to bootstrap roles: %v", err)
}
```

### 3. Use Catalog

```go
// Get all role definitions
allRoles := catalog.Definitions()

// Get role by ID
role := catalog.GetRoleByID("550e8400-e29b-41d4-a716-446655440000")
if role != nil {
    fmt.Printf("Role: %s has %d permissions\n", role.Name, role.PermissionCount())
}

// Get all role IDs
roleIDs := catalog.GetAllRoleIDs()

// Count roles
count := catalog.Count()
```

## Service Integration

Since role APIs and token provider are standardized across all services, the roles package makes HTTP calls directly to the sentinel service using `http.NewClientWithServiceToken`. **Services don't need to implement any API methods or create token providers!**

```go
import (
    "context"
    "github.com/milan604/core-lab/pkg/config"
    "github.com/milan604/core-lab/pkg/logger"
    "github.com/milan604/core-lab/pkg/roles"
    "github.com/milan604/core-lab/pkg/permissions"
)

// Define roles with permissions
definitions := []roles.Definition{
    {
        RoleID: "550e8400-e29b-41d4-a716-446655440000",
        Name:   "Admin",
        Permissions: []permissions.Reference{
            {Service: "USR", Category: "users", SubCategory: "create"},
            {Service: "USR", Category: "users", SubCategory: "delete"},
        },
    },
}

// Bootstrap - roles package makes HTTP calls directly to sentinel service
// Uses http.NewClientWithServiceToken internally - services don't need to handle it!
if err := roles.Bootstrap(ctx, definitions, cfg, log); err != nil {
    log.FatalF("Failed to bootstrap roles: %v", err)
}
```

**Note**: The roles package makes HTTP calls directly to the sentinel service. Services only need to:
1. Create role definitions with permission references
2. Pass config and logger to `Bootstrap` or `Sync`
3. The package uses `http.NewClientWithServiceToken` internally to create HTTP clients with token provider

**No API methods to implement!** The roles package handles all HTTP calls internally using `http.NewClientWithServiceToken` directly from the http package.

## Bootstrap vs Sync

Both `Bootstrap` and `Sync` perform the same operations:
- Validate role IDs exist in Sentinel
- Assign permissions to roles

`Bootstrap` is a convenience wrapper that calls `Sync` internally. Use either function based on your preference.

## What Bootstrap/Sync Does

1. **Validates Role Definitions**: Checks that all role definitions have valid RoleIDs
2. **Validates Role IDs in Sentinel**: Makes bulk API call to verify all role IDs exist in Sentinel
3. **Assigns Permissions to Roles**: For each role:
   - Converts permission references to permission codes
   - Fetches permission IDs from Sentinel using codes
   - Assigns permission IDs to the role in Sentinel

## Configuration

The roles package requires the following configuration (same as permissions and http packages):

- `SentinelServiceEndpoint`: URL of the sentinel service (required)
- `SentinelServiceID`: Service ID for authentication (required)
- `SentinelServiceAPIKey`: API key for authentication (required)

See `docs/config-variables.md` for complete configuration documentation.

## Dependencies

- `github.com/milan604/core-lab/pkg/permissions`: Permission reference types
- `github.com/milan604/core-lab/pkg/http`: HTTP client with service token authentication
- `github.com/milan604/core-lab/pkg/config`: Configuration management
- `github.com/milan604/core-lab/pkg/logger`: Logging interface

## Integration with Permissions Package

The roles package integrates seamlessly with the permissions package:

- Uses `permissions.Reference` for permission references
- Uses `permissions.GenerateCode()` for generating permission codes
- Works alongside `permissions.Bootstrap()` for complete permission and role management

## Example: Complete Setup

```go
import (
    "context"
    "github.com/milan604/core-lab/pkg/config"
    "github.com/milan604/core-lab/pkg/logger"
    "github.com/milan604/core-lab/pkg/permissions"
    "github.com/milan604/core-lab/pkg/roles"
)

cfg := config.New(...)
log := logger.MustNewDefaultLogger()
ctx := context.Background()

// 1. Define permissions
permDefinitions := []permissions.Definition{
    {
        Reference: permissions.Reference{
            Service:     "USR",
            Category:    "users",
            SubCategory: "create",
        },
        Name:        "CreateUser",
        Description: "Create a new user",
    },
    // ... more permissions
}
permCatalog := permissions.NewCatalog(permDefinitions)

// 2. Bootstrap permissions
permStore := permissions.NewStore(permissions.LoaderFromHTTP(cfg, log))
if err := permissions.Bootstrap(ctx, permCatalog, cfg, log, permStore); err != nil {
    log.FatalF("Failed to bootstrap permissions: %v", err)
}

// 3. Define roles with permissions
roleDefinitions := []roles.Definition{
    {
        RoleID: "550e8400-e29b-41d4-a716-446655440000",
        Name:   "Admin",
        Permissions: []permissions.Reference{
            {Service: "USR", Category: "users", SubCategory: "create"},
        },
    },
}

// 4. Bootstrap roles
if err := roles.Bootstrap(ctx, roleDefinitions, cfg, log); err != nil {
    log.FatalF("Failed to bootstrap roles: %v", err)
}
```

