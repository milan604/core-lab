# Durable Event Outbox

`pkg/events/outbox` provides a reusable polling processor for durable business-event delivery.

Use it when a service:

- owns authoritative state
- writes that state to its database
- must publish canonical platform events without making request correctness depend on Kafka availability

## Model

1. Write business state.
2. Write audit/event metadata.
3. Append an outbox record in the same database transaction.
4. Let the processor claim and publish pending records asynchronously.

The package intentionally splits responsibilities:

- the shared processor owns polling, claiming cadence, backoff, and delivery transitions
- each service owns its database schema and `Store` implementation

## Guarantees

- request-path success does not depend on Kafka being available
- retries are explicit and observable
- stale claims can be reclaimed by lease expiry
- event payloads stay tenant-aware because the canonical envelope is preserved end to end
