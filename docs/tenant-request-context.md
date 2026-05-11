# Tenant Request Context Contract

`core-lab` now provides one canonical tenant-aware request contract for services, jobs, and audit flows.

Implementation:

- [pkg/tenant/context.go](/Users/milanadhikari/personal/multiTenantPlatform/core-lab/pkg/tenant/context.go)

## Fields

- `tenant_id`
- `actor_user_id`
- `service_id`
- `correlation_id`
- `is_super_admin`

## Where It Is Used

- `pkg/auth`
  - verified claims populate the request context
  - helper setters can enrich tenant, actor, service, correlation, and super-admin state
- `pkg/jobs`
  - enqueue automatically propagates canonical metadata into jobs
- `pkg/audit`
  - audit events and metadata include the same tenant-aware fields
- `pkg/postgres`
  - tenant-aware SQL helpers read the same resolved tenant context

## Contract Rules

- `tenant_id` is empty only for platform-level operations.
- `actor_user_id` is for end-user initiated requests.
- `service_id` identifies the calling workload when the initiator is a service.
- `correlation_id` must be propagated across HTTP and async boundaries.
- `is_super_admin` indicates global admin capability and must be audited.

## Canonical Metadata Keys

- `tenant_id`
- `actor_user_id`
- `service_id`
- `correlation_id`
- `is_super_admin`
- `initiator`
- `source`

Use the shared metadata merge helper rather than inventing service-local keys.
