# Platform Foundation Architecture

## Purpose

`core-lab` should be the reusable runtime and integration foundation for a standalone identity, authorization, and configuration platform.

It should not contain product-specific business logic from Sentinel. It should contain the shared contracts that let any service consume that product cleanly.

## Ownership Split

`core-lab` owns:

- request/response middleware
- JWT and JWKS verification helpers
- machine-auth client foundations
- authorization/bootstrap clients
- config-management clients
- audit and observability primitives
- provider-neutral control-plane config keys and endpoint resolution

Sentinel owns:

- user lifecycle and credential workflows
- RBAC, policy, group, and delegation domain rules
- service identity issuance and trust policy
- configuration schemas, layering, approvals, and storage
- admin APIs, tenant administration, and product packaging

## Design Rule

When a capability is only useful because Sentinel currently implements it, keep the workflow in Sentinel.

When a capability is needed by multiple services regardless of Sentinel internals, keep the client contract in `core-lab`.

## Shared Integration Surface

The shared control-plane surface in `core-lab` should standardize:

- endpoint discovery
- service-token bootstrap configuration
- internal bootstrap auth fallback during migration
- JWKS discovery inputs
- policy/config/quota endpoint construction

That gives dependent services one integration model even while Sentinel evolves behind the API.

## Migration Direction

1. Prefer `Platform*` config keys for new work.
2. Keep `Sentinel*` and `InternalAdminKey` aliases until all services migrate.
3. Move duplicated endpoint and auth bootstrap logic into `core-lab`.
4. Keep product-specific decision semantics and persistence models inside Sentinel.

## Immediate Backlog

- add typed decision/config clients on top of the shared control-plane contract
- replace static internal-key bootstrap with mTLS or signed machine identity
- publish stable SDK-facing interfaces from `core-lab`
- add contract tests between Sentinel APIs and `core-lab` clients
