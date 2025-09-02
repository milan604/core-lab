# CoreLab

A modular Go project template for scalable backend services.

## Usage
- See `docs/` for architecture and contribution guidelines.
- Example usage in `examples/`.
- Error handling: see `pkg/errors` for the advanced `ServiceError` and implementation process.

## Structure
- `pkg/`: Public reusable packages
- `internal/`: Private internal packages
- `build/`: Dockerfiles, Makefile
- `docs/`: Developer documentation

## Build and versioning

Use the Makefile to build with embedded version metadata:

```bash
make build
```

Version info is injected at build time (see `pkg/version`). To release, follow `docs/contributing.md` and update `docs/changelog.md`.

## Error handling implementation process (short)

- Create service-layer errors with `pkg/errors.ServiceError` helpers (e.g., `errors.NotFound`, `errors.Internal`).
- Add details/suggestions via options or fluent setters.
- Normalize any `error` with `errors.ParseServiceError(err)`.
- Convert to `apperr.AppError` with `se.ToAppError()` and return via `pkg/response.JSONError`.

See full guide in `pkg/errors/README.md`.
