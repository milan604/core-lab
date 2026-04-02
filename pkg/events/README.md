# pkg/events

`pkg/events` provides the canonical platform business event envelope for cross-service event publication.

Use this package when:
- a service needs to publish a durable domain event to Kafka
- the event should carry tenant-aware metadata and request correlation
- consumers need a stable event shape across multiple services

Do not use this package for:
- request-local audit logging
- background job execution state
- service-internal ephemeral messages that never leave the service boundary

## Contract

The envelope includes:
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

`NewEnvelope(...)` merges the explicit publish request with the canonical tenant request context so the event automatically inherits:
- tenant identity
- actor user id
- service identity
- correlation id
- super-admin marker

## Example

```go
envelope, err := events.NewEnvelope(ctx, events.PublishRequest{
    ServiceID:    "subscription-service",
    EventType:    "subscription.created",
    TenantID:     tenantID,
    ResourceType: "subscription",
    ResourceID:   subscriptionID,
    Payload: map[string]any{
        "status": "active",
        "plan_id": planID,
    },
})
if err != nil {
    return err
}

if err := kafka.Publish(ctx, envelope.PartitionKey(), envelope); err != nil {
    return err
}
```

For authoritative writes, prefer appending the envelope to a durable outbox and letting
[`pkg/events/outbox`](../events/outbox/README.md) publish it asynchronously.

## Jobs vs Events

- Use `pkg/jobs` for background execution, retries, worker pools, and operational visibility.
- Use `pkg/events` for cross-service business facts that other services may consume.
- If a business state change matters across services, publish an event even if a background job also exists.
- Jobs must not be the primary cross-service correctness contract.
