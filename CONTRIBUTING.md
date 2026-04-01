# Contributing

Thanks for contributing to `core-lab`.

The canonical contributor guide lives in [docs/contributing.md](./docs/contributing.md). This top-level file exists so GitHub and other tooling can discover contribution guidance automatically.

## Quick Workflow

1. Create a focused branch.
2. Keep changes scoped and backwards-compatible unless a breaking change is intentional.
3. Run the local quality gates before opening a PR:

```bash
make fmt
make vet
make test
make ci
```

4. Update documentation when public behavior or package usage changes.
5. Add or extend tests for new behavior.

## Standards

- Prefer clear public APIs with strong package boundaries.
- Keep functions small and dependency injection explicit.
- Avoid hidden side effects and unnecessary global state.
- Favor readable examples and package-level docs for reusable modules.

For release and versioning rules, see [docs/contributing.md](./docs/contributing.md) and [docs/changelog.md](./docs/changelog.md).

For the full release checklist, see [docs/release-process.md](./docs/release-process.md).
