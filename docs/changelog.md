# Changelog

All notable changes to this project are documented here, following the Keep a Changelog format and Semantic Versioning.

## [Unreleased]
### Added
- Version package with build metadata and Makefile ldflags integration.
- i18n package with JSON bundles, interpolation, pluralization, fallbacks, Accept-Language, and Gin middleware.
- Centralized error handler middleware and improved server composition.
- Validator helpers for JSON/Query/URI/Header and combined binders.
- Utility packages (stringutil, timeutil, validation) with generics.

### Changed
- Refactored server options and middleware ordering for clarity and maintainability.

### Fixed
- Import path alignment to module `corelab`.

## [v0.2.0] - 2026-03-14
### Added
- Shared `controlplane` package for platform-native endpoint, audience, and machine-identity resolution.
- `pkg/authz` decision client and Gin middleware for resource-level authorization with request-context extraction and caching.
- Namespace-based configuration support through `pkg/configmanager` and `pkg/runtimeconfig`.

### Changed
- Standardized shared integrations around `Platform*` config keys and platform control-plane contracts.
- Updated machine-auth HTTP clients, quota helpers, and auth integration paths to align with service-token + mTLS control-plane access.
- Expanded shared authorization support so services can route legacy permission checks through centralized decision flows during migration.

### Security
- Strengthened internal control-plane integration patterns around explicit machine identity and mTLS-based service authentication.

## [v0.1.0] - 2025-08-09
### Added
- Initial public structure, base server, logger, config, db skeletons, response, errors, events scaffolds.

---

Guidelines:
- Keep this file human-readable and chronological.
- Use sections: Added, Changed, Fixed, Removed, Deprecated, Security.
- Reference issues/PRs where helpful.
