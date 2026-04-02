# Platform Events

This document defines the canonical business event contract for multi-service platform communication.

## Why This Exists

The platform previously had multiple event shapes:
- audit events
- tenant lifecycle events
- service-local Kafka payloads
- background job payload metadata

Those serve different purposes. Platform business events are now standardized through `core-lab/pkg/events`.

## Canonical Envelope

All cross-service domain events should use the `pkg/events` envelope with:
- `event_id`
- `event_type`
- `event_version`
- `tenant_id`
- `resource_type`
- `resource_id`
- `actor_user_id`
- `service_id`
- `correlation_id`
- `occurred_at`
- `metadata`
- `payload`

## Publishing Rules

- Publish business facts, not implementation details.
- Include `tenant_id` whenever the event is tenant-scoped.
- Prefer the authoritative service as the publisher.
- Use `resource_type` and `resource_id` consistently.
- Keep payloads minimal and stable.
- Put routing and provenance hints in `metadata`, not in ad hoc top-level fields.

## Jobs vs Events

Use jobs when:
- the work is asynchronous
- retries and operational visibility matter
- the task is internal to one service boundary

Use platform events when:
- another service may react
- the message represents a domain fact
- the message must remain stable over time

A service may do both:
- commit the state change
- enqueue a local job if background work is needed
- publish a platform event so other services can react

For authoritative business events, prefer the durable outbox pattern described in:

- [outbox-pattern.md](/Users/milanadhikari/personal/multiTenantPlatform/core-lab/docs/outbox-pattern.md)

## Topic Guidance

The default shared business-events topic is `platform.domain.events`.

Services may still use dedicated internal topics for workflow-specific processing, but:
- those internal topics are not the platform contract
- `pkg/events` payloads should go to the shared business-events topic unless there is a deliberate reason not to

## Current Adopters

- `tenant-management-service`: emits canonical events for tenant creation, updates, deletion, invitation lifecycle, and membership resync
- `subscription-service`: emits canonical events from the authoritative subscription audit path and now delivers them through a durable outbox
- `notification-service`: emits canonical events for notification creation, queueing, delivery, and terminal failure
- `upload-service`: emits canonical events for image upload completion and image processing completion while keeping its workflow-specific Kafka topics in place
- `sites-service`: emits canonical events for site draft saves, publishes, revision restores, page lifecycle changes, and custom-domain verification updates

## Rollout Guidance

- Start at authoritative write points or durable audit append points.
- Keep legacy internal messages in place until consumers migrate.
- Introduce consumers against the canonical envelope before removing service-local payload formats.
