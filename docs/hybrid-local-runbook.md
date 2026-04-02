# Hybrid Local Development Runbook

This runbook describes the preferred pattern for running local applications against
shared VPS-backed platform services during development.

## Goal

Allow developers to work locally without copying production browser sessions into
localhost and without exposing service credentials in frontend bundles.

## Preferred Model

- browser talks only to the local app/BFF
- local app/BFF talks to VPS services
- machine-only service auth stays on the server side
- public user auth continues to use normal login/session flows

## For Browser + BFF Apps

Recommended shape:

- local frontend dev server
- local BFF/server process
- VPS backend service URLs
- service-side machine auth only

The browser should not call internal VPS service endpoints directly.

## Session Model

Do not copy VPS cookies into localhost.

Reasons:

- session cookies are domain-bound
- server-side session stores are not shared with local memory stores
- local and VPS auth state become hard to reason about

Instead:

- log in normally through the local BFF
- keep a local session
- let the local BFF call VPS services with the correct user token and machine token

## Machine Auth

Preferred:

- service tokens
- mTLS where the platform requires it

Temporary compatibility modes, such as no-mTLS hybrid local development, should be:

- development-only
- explicit in config
- documented
- removed when no longer needed

## Tenant Context

Local hybrid flows should preserve the same tenant semantics as deployed flows:

- active tenant from auth context for tenant-user flows
- explicit tenant path for cross-tenant admin flows
- machine-only repair/resync paths for drift correction

## Practical Checklist

For a local app using VPS services:

1. run the local server/BFF
2. point service URLs to VPS endpoints
3. keep browser traffic routed through the local BFF
4. use service-token clients for machine-only internal calls
5. use explicit repair/resync APIs rather than ad hoc direct DB assumptions

## Drift Repair

If hybrid local development exposes identity or tenant drift, prefer explicit internal
repair APIs such as:

- tenant membership resync
- entitlement projection resync
- config resolve/publish repair

Do not paper over drift with local superuser shortcuts unless the mode is clearly
development-only and temporary.
