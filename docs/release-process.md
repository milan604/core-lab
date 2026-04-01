# Release Process

This guide describes the standard release flow for `core-lab`.

## Goals

- keep public package changes intentional
- preserve backwards compatibility unless a major release is planned
- ensure changelog, docs, and build metadata stay aligned

## Before a release

1. Review `docs/changelog.md` and make sure unreleased entries are accurate.
2. Confirm public-facing changes are documented in package READMEs or the top-level [README](../README.md).
3. Run the full verification suite:

```bash
make verify
```

4. Review dependency updates and outstanding CI failures.

## Versioning

`core-lab` follows Semantic Versioning.

- `MAJOR`: breaking API or behavior changes
- `MINOR`: backwards-compatible features or package additions
- `PATCH`: backwards-compatible fixes, documentation updates, or internal improvements

## Cutting a release

1. Finalize the changelog entry under the target version and date.
2. Create an annotated git tag:

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

3. Build artifacts with embedded metadata:

```bash
make build
make build-cmd
```

The embedded metadata is sourced from [build/Makefile](../build/Makefile) and surfaced through [`pkg/version`](../pkg/version/README.md).

## After a release

- start a new `Unreleased` section in `docs/changelog.md`
- update any release notes or migration guidance if the release changed public APIs
- follow up on consumers that need upgrade coordination for breaking changes
