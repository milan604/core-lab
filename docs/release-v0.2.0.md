# Release v0.2.0

## Title

Platform Control Plane Foundation

## Release Description

`core-lab v0.2.0` turns the library into the shared integration foundation for the platform control plane. This release standardizes machine identity, authorization, and configuration consumption across services so backend teams can integrate once and reuse the same runtime contract everywhere.

### Highlights

- Added a shared `controlplane` contract for endpoint discovery, machine identity settings, token audiences, and platform-native `Platform*` configuration keys.
- Added first-class authorization clients and Gin middleware in `pkg/authz` for resource-level decision enforcement, including request-context mapping, body-aware context extraction, in-memory decision caching, and stale-cache fallback behavior.
- Added namespace-based configuration consumption through `pkg/configmanager` and `pkg/runtimeconfig`, so services can resolve and hydrate runtime config from the platform control plane instead of depending on handwritten config plumbing.
- Updated shared machine-auth HTTP clients to use service-token plus mTLS control-plane access as the standard path for internal platform integrations.
- Aligned quota, roles, permissions, JWKS, and authorizer helpers with the shared control-plane contract so existing services can migrate toward the same integration model without duplicating Sentinel-specific logic.

### Why This Release Matters

This release moves `core-lab` from a generic backend toolkit into a reusable platform SDK layer. Services now have one consistent way to:

- authenticate to the control plane
- request authorization decisions
- resolve runtime configuration
- consume policy, quota, and identity metadata

That reduces service-by-service drift and makes the broader platform easier to run as a standalone product.

### Upgrade Notes

- Prefer `Platform*` config keys for new integrations, especially:
  - `PlatformBaseURL`
  - `PlatformServiceID`
  - `PlatformServiceAPIKey`
  - `PlatformMTLSCertFile`
  - `PlatformMTLSKeyFile`
  - `PlatformMTLSCAFile`
- For resource-level authz adoption, use `pkg/authz` middleware instead of hand-rolled route checks.
- For runtime configuration adoption, use `pkg/runtimeconfig` and `pkg/configmanager` rather than external config-fetch wrappers.
- Services using control-plane integrations should ensure machine identity and mTLS client credentials are configured before enabling these flows.

### Recommended Release Summary

`core-lab v0.2.0` introduces the platform control-plane foundation: shared machine-auth contracts, resource-level authz clients and middleware, and namespace-based runtime configuration support for all backend services.
