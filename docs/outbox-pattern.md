# Platform Outbox Pattern

The platform now uses a durable outbox pattern for authoritative business events.

## Why

Publishing directly to Kafka in the request path makes correctness depend on broker availability. That creates two bad failure modes:

- the business write succeeds but the event is lost
- the event is published but the business transaction later rolls back

The durable outbox pattern solves that by storing the event intent in the same database transaction as the business write, then publishing asynchronously.

## Standard Flow

1. Write authoritative business state.
2. Append audit metadata if the service has an audit log.
3. Append an outbox record in the same transaction.
4. Let a background outbox processor claim and publish pending records.
5. Mark records delivered or reschedule them with backoff.

## Shared Foundation

`core-lab/pkg/events/outbox` is the shared processor contract.

It standardizes:

- polling cadence
- claim leasing
- publish timeout
- exponential retry backoff
- delivered and failed state transitions

It intentionally does not force one database schema across services. Each authoritative service owns its own outbox table and `Store` implementation.

## Current Adopter

- `subscription-service`
  - appends canonical platform events to `platform_event_outbox`
  - publishes asynchronously through the shared processor
  - keeps request correctness independent from Kafka availability

## Adoption Rules

- Use the outbox for authoritative cross-service business events.
- Keep background jobs for service-local work and operational visibility.
- Keep workflow-specific Kafka topics only when they serve an internal processing pipeline.
- Preserve the canonical `pkg/events` envelope inside the outbox payload.
- Include tenant metadata in every tenant-scoped outbox event.

## Minimum Table Shape

Each service outbox table should carry at least:

- `id`
- `topic`
- `partition_key`
- `tenant_id`
- `event_type`
- `service_id`
- `correlation_id`
- `payload`
- `available_at`
- `attempt_count`
- `claimed_by`
- `claimed_until`
- `last_error`
- `delivered_at`
- `created_at`
- `updated_at`

## Rollout Sequence

1. Add canonical event publishing at the authoritative write seam.
2. Change direct publish to transactional append + async publish.
3. Validate replay/retry semantics in staging.
4. Migrate the next authoritative service one domain at a time.
