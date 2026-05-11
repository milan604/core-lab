# Config Namespace Layering

This document defines the target config layering model for the hybrid shared + dedicated tenant platform.

## Goals

- keep one service image per service
- keep one runtime contract across shared and dedicated deployments
- allow tenant-specific overrides without creating ad hoc config forks
- make control-plane resolution deterministic and auditable

## Namespace Layers

Config should resolve in this precedence order, from most specific to least specific:

1. `tenant/<tenant-id>/service/<service-name>/env/<environment>`
2. `tenant/<tenant-id>/service/<service-name>`
3. `tenant/<tenant-id>/env/<environment>`
4. `tenant/<tenant-id>`
5. `service/<service-name>/env/<environment>`
6. `service/<service-name>`
7. `platform/env/<environment>`
8. `platform/shared`

## Rules

- Service runtime keys stay canonical across all layers.
- Tenant-specific overrides must be explicit and sparse.
- Secrets and endpoints should not be duplicated across layers unless an override is truly needed.
- Dedicated tenant deployments should use the same key names as shared SaaS.
- Platform-wide defaults should remain valid even when no tenant-specific override exists.

## Recommended Usage

### Platform shared

Use for:

- shared Kafka brokers
- shared Redis cluster
- tracing endpoints
- default rate limits
- global feature flags

### Service scoped

Use for:

- service-specific tuning
- topic names
- outbox poll cadence
- internal API endpoints

### Tenant scoped

Use for:

- dedicated custom domain routing
- dedicated storage bucket or prefix overrides
- premium tenant feature toggles
- dedicated infra endpoints for premium tenants

## Dedicated Tenant Blueprint

Dedicated tenants should not require bespoke service code. The deployment model changes, not the contract.

Recommended dedicated overrides:

- storage prefix or bucket
- publish and render endpoints
- tenant-specific SMTP or webhook destinations when contractually required
- dedicated database, Redis, or Kafka endpoints only where the isolation tier requires them

## Auditability

Every resolved runtime config response should preserve:

- winning namespace
- fallback chain
- version
- resolved timestamp

That keeps shared and dedicated environments debuggable with the same tooling.
