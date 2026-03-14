# Control Plane Package

This package centralizes shared config keys and endpoint construction for external identity, authorization, quota, and configuration control planes.

## Goals

- give all services one control-plane integration model
- use platform-native naming for machine identity and control-plane access
- keep Sentinel-specific compatibility only where it is still intentional

## Preferred Config Keys

- `PlatformControlPlaneEndpoint`
- `PlatformServiceID`
- `PlatformServiceAPIKey`
- `PlatformInternalKey`
- `PlatformServiceTokenScope`
- `PlatformTokenIssuer`
- `PlatformTokenAudience`
- `PlatformOIDCDiscoveryURL`
- `PlatformJWKSURL`

## Sentinel Compatibility

The package still supports Sentinel-specific aliases where the broader control-plane contract intentionally remains backward-compatible:

- `SentinelServiceEndpoint`
- `SentinelServiceTokenScope`
- `SentinelTokenIssuer`
- `SentinelTokenAudience`
- `SentinelOIDCDiscoveryURL`
- `SentinelJWKSURL`
- `InternalAdminKey`

## Usage

```go
api := controlplane.APIFromConfig(cfg)
tokenCfg := controlplane.ResolveServiceTokenConfig(cfg)
audience := controlplane.ResolveTokenAudience(cfg)
internalKey := controlplane.ResolveInternalKey(cfg)
```
