# Platform Go Service Standards

This document defines the paved-road standard for Go services built on `core-lab`
 in the multi-tenant platform.

## Service Shape

Every service should follow the same high-level structure:

- `main.go` owns bootstrap only
- `config/` owns service-specific config keys and defaults
- `router/` owns HTTP surface and middleware composition
- `core/service/` owns business logic
- `core/db/` owns persistence
- `core/pkg/api/` owns outbound service clients
- `scripts/start.sh` owns local/bootstrap config generation only

Business logic should not live in route handlers, and route handlers should not
perform direct SQL access.

## Control Plane Integration

Canonical shared packages:

- `pkg/auth` for JWT verification and tenant context propagation
- `pkg/authz` for authorization decisions
- `pkg/quota` for quota checks
- `pkg/http` for service-token HTTP clients
- `pkg/controlplane` for platform audience and endpoint resolution
- `pkg/runtimeconfig` / `pkg/configmanager` for CNM and runtime config
- `pkg/jobs` for background execution
- `pkg/audit` for audit events
- `pkg/postgres` for tenant-aware DB helpers

Services should prefer `Platform*` config keys as the canonical names. Legacy
`Sentinel*` aliases should only exist as migration compatibility shims.

## API Standards

Expected operational routes:

- `service-status`
- `healthz`
- `readyz`
- `metrics`

Expected API conventions:

- consistent JSON error envelope
- consistent authn/authz failure semantics
- explicit machine-only internal routes
- user-scoped routes derive tenant from auth context where practical
- cross-tenant admin routes use explicit tenant path parameters

## Tenant Model

Use the shared tenant request context contract from
[tenant-request-context.md](./tenant-request-context.md):

- `tenant_id`
- `actor_user_id`
- `service_id`
- `correlation_id`
- `is_super_admin`

Tenant-scoped jobs, audit events, cache keys, and object paths should carry tenant
metadata explicitly.

## Internal Routes

Internal routes are machine-to-machine APIs.

Requirements:

- authenticated service token
- audience-scoped machine auth where applicable
- no new implicit `X-Internal-Key` trust paths
- compatibility-only internal keys must be explicit, temporary, and documented

Examples of acceptable internal routes:

- tenant repair/resync
- entitlement projection sync
- config resolve/publish
- authz/quota decision APIs

## Orchestration Boundary

Services like `ecompulse` may expose orchestration and composition routes, but they
must not become source-of-truth owners for domains that already have authoritative
services.

When a compatibility/orchestration route exists:

- mark it clearly in docs
- prefer response headers that identify route mode and authoritative service
- keep writes delegated to the authoritative service

## Background Work

Use:

- `pkg/jobs` + Redis for background execution and operator visibility
- Kafka/events for cross-service business integration

Jobs should not be the primary correctness path for business state transfer between
services.

## Testing Standards

Each service should have:

- route-level auth contract tests
- tenant-scope enforcement tests
- outbound client contract tests for authoritative dependencies
- regression tests for machine-only internal routes

When a route depends on the service layer, prefer narrow interfaces in handlers so
contract tests can inject fakes without depending on the full concrete service.

## Deployment and Local Development

Each service should document:

- runtime config/CNM expectations
- required machine-auth config
- health diagnostics
- local run commands
- hybrid-local development path if the service is commonly used from a local app

See also [hybrid-local-runbook.md](./hybrid-local-runbook.md).
