`pkg/authz` provides the shared Sentinel authorization client plus a Gin middleware layer for resource-level decision enforcement.

Typical usage in a service:

```go
authorizer, _ := auth.NewAuthorizer(cfg, log)

protected := router.Group("",
    authorizer.RequireAuthenticated(),
    auth.TenantAccessMiddleware(auth.DefaultTenantAppConfig()),
)

protected.GET("/:tenant_id/sites/:site_id",
    authz.Middleware(cfg, log, authz.NewMiddlewareConfig(cfg).
        WithCategory("sites").
        WithAction("read").
        WithResourceType("site").
        WithRequireTenant(true).
        WithResourcePathParam("site_id"),
    ),
    handler,
)
```

Useful defaults:

- `NewMiddlewareConfig(cfg)` resolves `PlatformServiceID`, enables the middleware by default, and adds a short in-memory decision cache.
- `TenantFromContextOrClaims` reuses tenant scope already resolved by `pkg/auth`.
- `WithActionFromMethod()` maps `GET/HEAD -> read`, `POST -> create`, `PUT/PATCH -> update`, and `DELETE -> delete`.
- `JSONBodyContextResolver(...)` extracts JSON body fields, including nested paths like `admin_user.email`, without consuming the body for downstream handlers.
- `GetDecision(c)` or `DecisionFromContext(ctx)` exposes the matched decision to downstream handlers.

This layer is intended to sit next to the existing JWT authentication and tenant-scope middleware so services can adopt resource-level authz without reimplementing Sentinel request plumbing.
