# CoreLab

A modular Go project template for scalable backend services.

## Usage
- See `docs/` for architecture and contribution guidelines.
- Example usage in `examples/`.

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
