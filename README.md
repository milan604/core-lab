# CoreLab

`core-lab` is a shared Go foundation library for building production-grade backend services with consistent configuration, auth, authorization, observability, validation, error handling, runtime configuration, and platform integration patterns.

It is designed for multi-service platforms that want one standard way to:
- bootstrap services
- expose HTTP APIs
- verify user and service tokens
- make authorization decisions
- normalize errors and responses
- publish metrics, logs, and audit events
- integrate with control-plane style infrastructure

## Highlights

- Modular public packages under `pkg/` with small, focused responsibilities.
- Consistent service bootstrapping through [`pkg/app`](./pkg/app/README.md) and [`pkg/server`](./pkg/server/README.md).
- Shared auth and policy patterns through [`pkg/auth`](./pkg/auth/README.md) and [`pkg/authz`](./pkg/authz/README.md).
- Production-oriented support for config, observability, runtime config, quotas, and control-plane clients.
- Build metadata injection through [`pkg/version`](./pkg/version/README.md).

## Installation

```bash
go get github.com/milan604/core-lab
```

## Quick Start

Common local commands:

```bash
make help
make fmt
make vet
make test
make build
make ci
```

Run the full test suite directly:

```bash
GOCACHE=/tmp/core-lab-gocache go test ./...
```

## Repository Layout

- `pkg/`: reusable public packages intended for service consumption
- `docs/`: architecture, changelog, and contribution guidance
- `examples/`: small runnable examples
- `build/`: build-time metadata injection and Docker-related helpers

## Package Map

The full package index lives in [docs/package-catalog.md](./docs/package-catalog.md). The main building blocks are:

| Area | Packages |
| --- | --- |
| App bootstrap | [`pkg/app`](./pkg/app/README.md), [`pkg/server`](./pkg/server/README.md), [`pkg/version`](./pkg/version/README.md) |
| Auth and authz | [`pkg/auth`](./pkg/auth/README.md), [`pkg/authz`](./pkg/authz/README.md), [`pkg/permissions`](./pkg/permissions/README.md), [`pkg/roles`](./pkg/roles/README.md), [`pkg/quota`](./pkg/quota/quota.go) |
| Platform integration | [`pkg/controlplane`](./pkg/controlplane/README.md), [`pkg/configmanager`](./pkg/configmanager/client.go), [`pkg/runtimeconfig`](./pkg/runtimeconfig/README.md), [`pkg/http`](./pkg/http/README.md) |
| API ergonomics | [`pkg/errors`](./pkg/errors/README.md), [`pkg/apperr`](./pkg/apperr/README.md), [`pkg/response`](./pkg/response/README.md), [`pkg/validator`](./pkg/validator/README.md) |
| Infra and data | [`pkg/config`](./pkg/config/README.md), [`pkg/postgres`](./pkg/postgres/README.md), [`pkg/tenant`](./pkg/tenant/lifecycle.go) |
| Runtime services | [`pkg/jobs`](./pkg/jobs/README.md), [`pkg/audit`](./pkg/audit/audit.go), [`pkg/logger`](./pkg/logger/README.md), [`pkg/observability`](./pkg/observability/README.md) |
| Utilities | [`pkg/i18n`](./pkg/i18n/README.md), [`pkg/utils`](./pkg/utils/README.md), [`pkg/featureflags`](./pkg/featureflags/featureflags.go) |

## Documentation

- [Architecture](./docs/architecture.md)
- [Platform Foundation Architecture](./docs/platform-foundation-architecture.md)
- [Package Catalog](./docs/package-catalog.md)
- [Examples](./examples/README.md)
- [Changelog](./docs/changelog.md)
- [Contributing](./CONTRIBUTING.md)
- [Release Process](./docs/release-process.md)
- [Security Policy](./SECURITY.md)
- [Code of Conduct](./CODE_OF_CONDUCT.md)

## Development Standards

- Keep public APIs intentional and documented.
- Prefer small, composable packages over broad utility buckets.
- Preserve backwards compatibility unless a breaking change is explicitly planned.
- Add tests and documentation for externally visible behavior changes.
- Keep service-level conventions centralized here when they are broadly reusable.

## Build and Release

Version info is embedded at build time through [`build/Makefile`](./build/Makefile) and surfaced via [`pkg/version`](./pkg/version/README.md).

To build with embedded metadata:

```bash
make build
```

Release notes and changelog process:
- update [docs/changelog.md](./docs/changelog.md)
- follow [CONTRIBUTING.md](./CONTRIBUTING.md)
- tag using semantic versioning

## License

[LICENSE](./LICENSE)
