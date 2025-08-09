# Architecture

This repository provides a modular toolkit for building Go backend services with a clean, production-ready architecture.

## High-level layout

- `pkg/` — Reusable packages intended to be imported by applications:
	- `server/` — Gin-based HTTP server setup with middleware (request ID, logging, CORS, rate limit, Prometheus, error handling, recovery) and functional options.
	- `logger/` — Zap-based structured logging with context helpers.
	- `config/` — Configuration loader (Viper) with sane defaults and environment overrides.
	- `db/` — Database clients (Postgres, Mongo, Redis) with connection helpers.
	- `auth/` — Auth integrations (Keycloak, OpenFGA) and middleware hooks.
	- `events/` — Messaging integrations (Kafka, NATS, SQS).
	- `errors/` — Canonical application error model and helpers.
	- `response/` — Consistent JSON response envelope.
	- `validator/` — Binding + validation helpers on top of gin/validator.
	- `i18n/` — Lightweight translation with interpolation, pluralization, fallbacks, and Gin middleware.
	- `utils/` — Practical generics and helpers (strings, time, validation).
	- `version/` — Build-time injected version metadata.

- `internal/` — Internal utilities and test helpers not intended for external import.

- `examples/` — Small runnable examples showing common integration patterns.

## Server composition

The server uses Gin and assembles middlewares in a stable order:

1. Request ID injection
2. Access logging and app logger injection
3. CORS (configurable)
4. Rate limiting (optional, per-IP token bucket)
5. Prometheus metrics (optional)
6. Centralized error handler
7. User-provided middlewares
8. Recovery

Engine and Start functions use functional options so features can be toggled with minimal ceremony.

## Errors and responses

- `errors` exposes typed error codes and helpers to create structured errors.
- `response` ensures all API responses follow a consistent envelope with success/error payloads.

## Validation

`validator` wraps gin binding and go-playground/validator with convenience helpers for JSON, query, URI, and header binding, plus combined binders and error parsing to application errors.

## i18n

The translator supports JSON bundles per domain and locale, interpolation with `{{var}}`, pluralization via `key.one/key.other`, fallbacks, and Accept-Language negotiation. Gin middleware sets locale per request and exposes it via context.

## Versioning and build info

The `version` package exposes build metadata (version, commit, date, go). The Makefile injects these via `-ldflags` so binaries can report their version at runtime.

## Observability

Logging is structured via Zap. Metrics are exposed with Prometheus when enabled. Errors are normalized by a central handler.

## Extensibility

All components are pluggable and follow small, focused interfaces. Packages are decoupled so you can replace parts (e.g., swap auth provider or event bus) without impacting the rest.
