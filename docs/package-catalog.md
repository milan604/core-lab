# Package Catalog

This document is the top-level package index for `core-lab`. It helps contributors and adopters understand where functionality belongs before adding new APIs.

## Service Bootstrap

| Package | Purpose |
| --- | --- |
| [`pkg/app`](../pkg/app/README.md) | Shared application bootstrap and lifecycle orchestration |
| [`pkg/server`](../pkg/server/README.md) | Gin server assembly, options, and middleware composition |
| [`pkg/version`](../pkg/version/README.md) | Embedded build metadata |

## Auth, Authorization, and Policy

| Package | Purpose |
| --- | --- |
| [`pkg/auth`](../pkg/auth/README.md) | JWT verification, claims, auth middleware, tenant access helpers |
| [`pkg/authz`](../pkg/authz/README.md) | Authorization decision client and middleware |
| [`pkg/permissions`](../pkg/permissions/README.md) | Permission catalogs, loading, bootstrapping, and conversion |
| [`pkg/roles`](../pkg/roles/README.md) | Role catalog definitions and synchronization helpers |
| `pkg/quota` | Quota enforcement helpers and Sentinel-backed checks |

## Platform and Control Plane Integration

| Package | Purpose |
| --- | --- |
| [`pkg/controlplane`](../pkg/controlplane/README.md) | Shared control-plane URL, identity, and endpoint contracts |
| `pkg/configmanager` | Control-plane configuration management client |
| [`pkg/runtimeconfig`](../pkg/runtimeconfig/README.md) | Runtime config resolution and watch helpers |
| [`pkg/http`](../pkg/http/README.md) | Shared HTTP client, service-token transport, Sentinel URL helpers |

## API Ergonomics

| Package | Purpose |
| --- | --- |
| [`pkg/errors`](../pkg/errors/README.md) | Structured service errors and normalization |
| [`pkg/apperr`](../pkg/apperr/README.md) | Application error envelope and code mapping |
| [`pkg/response`](../pkg/response/README.md) | Consistent JSON responses |
| [`pkg/validator`](../pkg/validator/README.md) | Binding and validation helpers |

## Configuration, Data, and Tenancy

| Package | Purpose |
| --- | --- |
| [`pkg/config`](../pkg/config/README.md) | Shared config loading and defaults |
| [`pkg/postgres`](../pkg/postgres/README.md) | Postgres helpers, migrations, tenant context helpers |
| `pkg/tenant` | Shared tenant lifecycle helpers and canonical tenant request context |

## Observability and Operations

| Package | Purpose |
| --- | --- |
| [`pkg/logger`](../pkg/logger/README.md) | Structured logging and context-aware logging helpers |
| [`pkg/observability`](../pkg/observability/README.md) | Metrics, tracing, endpoint instrumentation, observability wiring |
| [`pkg/jobs`](../pkg/jobs/README.md) | Background job manager, worker pool, retries, stats, and admin APIs |
| [`pkg/events`](../pkg/events/README.md) | Canonical cross-service business event envelope and publication helpers |
| [`pkg/events/outbox`](../pkg/events/outbox/README.md) | Durable outbox processor for authoritative business-event delivery |
| `pkg/audit` | Audit event middleware, helpers, and Kafka integration |

## Localization and Utilities

| Package | Purpose |
| --- | --- |
| [`pkg/i18n`](../pkg/i18n/README.md) | Translation bundles and locale helpers |
| [`pkg/utils`](../pkg/utils/README.md) | Generic utility helpers |
| `pkg/featureflags` | Simple feature-flag utilities |

## Guidance for New Additions

- Add new functionality to an existing package when it clearly belongs to that package’s responsibility.
- Create a new package only when the boundary is stable and reusable across services.
- Public packages should include package comments, tests when practical, and a README when the API surface is non-trivial.
- If behavior affects consumers, update the top-level [README](../README.md) and [docs/changelog.md](./changelog.md).
