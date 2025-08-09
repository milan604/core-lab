# Contributing

Thanks for taking the time to contribute! This document sets expectations and describes how to get changes merged smoothly.

## Development setup

- Go 1.24+
- Linux/macOS recommended
- `make build` compiles the module with embedded version info.

## Workflow

1. Fork and create a feature branch.
2. Write focused, well-tested changes. Keep public APIs stable unless justified.
3. Keep commits small and meaningful. Update docs when behavior changes.
4. Run linters/formatters and build before opening a PR.
5. Open a PR with a clear description and link related issues.

## Coding standards

- Follow effective Go and idiomatic patterns.
- Keep functions small; prefer composition over inheritance.
- Avoid global state; prefer dependency injection and options.
- Public APIs must be documented with clear comments and examples when helpful.

## Testing

- Add unit tests for new behavior and edge cases.
- Favor deterministic tests; avoid timing-based flakiness.
- Keep tests fast and readable.

## Versioning and releases

We follow Semantic Versioning (SemVer). Versions are managed via git tags and embedded at build time.

- Bump versions according to changes:
	- MAJOR: incompatible API changes
	- MINOR: backwards-compatible functionality
	- PATCH: backwards-compatible bug fixes
- Update `docs/changelog.md` under Unreleased; on release, create a new section for the version/date and move entries.
- Create an annotated tag: `git tag -a vX.Y.Z -m "Release vX.Y.Z" && git push --tags`.
- Builds embed version metadata via ldflags (see `build/Makefile`).

## Commit messages

Use present tense, short subject (<72 chars), and a body explaining what/why when needed. Reference issues like `Fixes #123` when applicable.

## Security

Report vulnerabilities privately to the maintainers. Avoid filing public issues for sensitive disclosures.

## Code of Conduct

Be respectful and inclusive. Assume good intent. Disagree constructively.
