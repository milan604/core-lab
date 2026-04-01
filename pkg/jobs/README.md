# Jobs

`pkg/jobs` provides a reusable background job runtime for `core-lab` services.

It is designed for cases where a service needs to:
- enqueue work without blocking the request path
- run handlers with a worker pool in background goroutines
- support delayed execution and retry policies
- expose job health, listing, and statistics APIs
- run the job runtime embedded in a service or as its own standalone worker server

## Highlights

- Typed handler registry with per-handler defaults
- Delayed jobs through `RunAfter`
- Automatic retry with exponential backoff
- Retention-based cleanup for terminal jobs
- Queue/job stats and worker runtime snapshots
- HTTP admin routes for enqueue, list, inspect, retry, cancel, and health
- Prometheus metrics hooks

## Storage backends

- `NewMemoryStore()` for embedded single-process workers and local development
- `NewRedisStore(...)` / `NewRedisStoreFromConfig(...)` for shared, cross-service visibility and multi-replica workers

When multiple services point their job managers at the same Redis namespace, the stored jobs become visible through any service exposing the admin APIs for that namespace.

## Core types

- `Manager`: starts workers, dispatchers, and janitors
- `Store`: persistence and state-transition abstraction
- `MemoryStore`: built-in in-memory implementation
- `HandlerFunc`: job handler signature
- `EnqueueRequest`: API-safe job submission input

## Embedded usage

```go
store := jobs.NewMemoryStore()

manager, err := jobs.NewManager(jobs.Config{
	Name:    "thumbnail-jobs",
	Workers: 4,
}, store)
if err != nil {
	panic(err)
}

_ = manager.RegisterHandler("thumbnail.generate", func(ctx context.Context, job jobs.Job) (any, error) {
	// do work
	return map[string]any{"ok": true}, nil
})

_ = manager.Start(context.Background())
defer manager.Stop(context.Background())
```

## Shared Redis usage

```go
store, err := jobs.NewRedisStoreFromConfig(context.Background(), jobs.RedisStoreConfig{
	Address:   "redis:6379",
	Namespace: "platform-background-jobs",
})
if err != nil {
	panic(err)
}

manager, err := jobs.NewManager(jobs.Config{
	Name:    "email-worker",
	Workers: 4,
}, store)
if err != nil {
	panic(err)
}
```

## Standalone admin server

```go
engine := jobs.NewAdminEngine(manager)
if err := server.Start(engine, server.StartWithAddr(":8090")); err != nil {
	panic(err)
}
```

## Admin routes

- `GET /healthz`
- `GET /handlers`
- `GET /stats`
- `GET /jobs`
- `GET /jobs/:id`
- `POST /jobs`
- `POST /jobs/:id/retry`
- `POST /jobs/:id/cancel`

## Operational guidance

- Keep handlers idempotent whenever possible.
- Use small payloads; store references to large blobs rather than embedding them.
- Prefer explicit handler names such as `email.send` or `site.publish`.
- Use queue separation to isolate slow jobs from latency-sensitive jobs.
- Use a shared Redis namespace when you want jobs posted by different services to appear in the same admin/UI surface.
