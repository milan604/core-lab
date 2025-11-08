# Permissions Package

A generic, reusable permissions management package for Go services.

## Overview

This package provides:
- **Permission Store**: Thread-safe in-memory cache for permission metadata
- **Permission Catalog**: Management of permission definitions
- **Bootstrap**: Synchronization with external permission services (e.g., Sentinel)
- **Code Generation**: Automatic permission code generation from service/category/subcategory
- **Self-contained Models**: All models are defined within the package for easy migration

## Features

- Thread-safe permission storage
- Lazy loading with configurable loaders
- Automatic permission code generation
- Bootstrap synchronization with external services
- Generic interfaces for easy integration
- Self-contained models (no external dependencies)
- Service-specific adapters for easy integration

## Models

All models are defined within the package:

- `CreateRequest` - Request to create a permission
- `CreateResponse` - Response from creating permissions
- `CatalogResponse` - Permission catalog from sentinel service
- `CatalogEntry` - Individual permission entry in catalog
- `GroupCatalogEntry` - Permission group entry in catalog
- `Metadata` - Permission metadata stored in the store

## Usage

### 1. Define Permissions

```go
definitions := []permissions.Definition{
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

catalog := permissions.NewCatalog(definitions)
```

### 2. Create API Client

Implement the `permissions.API` interface:

```go
type MyAPIClient struct {
    // your HTTP client
}

func (c *MyAPIClient) CreatePermissions(ctx context.Context, perms []permissions.CreateRequest) (*permissions.CreateResponse, error) {
    // implement bulk permission creation
}

func (c *MyAPIClient) GetCatalog(ctx context.Context) (*permissions.CatalogResponse, error) {
    // implement catalog fetching
}
```

### 3. Initialize Store

```go
apiClient := &MyAPIClient{}
loader := permissions.LoaderFromAPI(apiClient)
store := permissions.NewStore(loader)
```

### 4. Bootstrap

```go
if err := permissions.Bootstrap(ctx, catalog, apiClient, store); err != nil {
    log.Fatal(err)
}
```

### 5. Use Store

```go
// Lookup permission
meta, ok := store.Lookup("USR-users-create")
if !ok {
    // permission not found
}

// Check bitmask
if userBitmask&meta.BitValue == meta.BitValue {
    // user has permission
}

// List by service
userPerms := store.ListByService("USR")
```

## Service Integration

Since permission APIs and token provider are standardized across all services, the permissions package makes HTTP calls directly to the sentinel service using `http.NewClientWithServiceToken`. **Services don't need to implement any API methods or create token providers!**

```go
import (
    "github.com/milan604/core-lab/pkg/permissions"
    "github.com/milan604/core-lab/pkg/config"
    "github.com/milan604/core-lab/pkg/logger"
)

// Create catalog from existing permissions
catalog := service.NewPermissionsCatalog()

// Create store with loader - uses http.NewClientWithServiceToken directly
loader := permissions.LoaderFromHTTP(config, logger)
store := permissions.NewStore(loader)

// Bootstrap - permissions package makes HTTP calls directly to sentinel service
// Uses http.NewClientWithServiceToken internally - services don't need to handle it!
if err := permissions.Bootstrap(ctx, catalog, config, logger, store); err != nil {
    log.Fatal(err)
}
```

**Note**: The permissions package makes HTTP calls directly to the sentinel service. Services only need to:
1. Create a catalog from their permission definitions
2. Pass config and logger to `LoaderFromHTTP` and `Bootstrap`
3. The package uses `http.NewClientWithServiceToken` internally to create HTTP clients with token provider

**No API methods to implement!** The permissions package handles all HTTP calls internally using `http.NewClientWithServiceToken` directly from the http package.

## Interfaces

### HTTPClient Interface

The permissions package uses the HTTPClient interface for making HTTP requests:

```go
type HTTPClient interface {
    PostJSON(ctx context.Context, url string, body interface{}, response interface{}) error
    GetJSON(ctx context.Context, url string, response interface{}) error
}
```

The package uses `http.NewClientWithServiceToken` internally, which returns a client that implements this interface.

### Config and Logger

Services need to provide:
- `*config.Config` - Core-lab's config package instance
- `logger.LogManager` - Core-lab's logger package instance

Both are passed directly to `LoaderFromHTTP` and `Bootstrap` functions.

### Loader Function

```go
type Loader func(ctx context.Context) (map[string]Metadata, error)
```

## Migration to core-lab

This package is designed to be easily moved to the `core-lab` repository:

### Files to Copy to core-lab

Copy these files to `core-lab/pkg/permissions`:
- `models.go` ✅ - Self-contained, no dependencies
- `store.go` ✅ - Self-contained, no dependencies
- `catalog.go` ✅ - Self-contained, no dependencies
- `bootstrap.go` ✅ - Self-contained, no dependencies
- `loader.go` ✅ - Self-contained, no dependencies
- `adapter.go` ✅ - Self-contained, no dependencies
- `converter.go` ✅ - Generic converter, no dependencies
- `README.md` ✅ - Documentation

### Files to KEEP in Service Repo (DO NOT COPY)

Service-specific adapters and converters are located outside the permissions package:
- `core/service/permissions_adapter.go` ❌ - Service-specific adapter (NOT in permissions package)
- `core/service/permissions_converter.go` ❌ - Service-specific converter (NOT in permissions package)

### After Migration

1. Update import paths in your service code:
   ```go
   // Before
   import "ecompulse/core/pkg/permissions"
   
   // After
   import "github.com/milan604/core-lab/pkg/permissions"
   ```

2. Keep service-specific files in your service repo:
   - `service_adapter.go` - Create your own adapter for your service
   - `service_converter.go` - Create your own converter for your service models

3. No other changes needed - the core package is self-contained with all models included

