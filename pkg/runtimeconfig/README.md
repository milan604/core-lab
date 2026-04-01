# Runtime Config Loader

`pkg/runtimeconfig` is the standard runtime configuration bootstrap for Go services.

The intended pattern is:

1. Start with a tiny bootstrap config from env.
   Include only what the service needs to authenticate to Sentinel and start observability.
2. Resolve the `runtime` namespace directly from Sentinel during service bootstrap.
3. Merge the resolved payload before config validation.
4. Persist a last-known-good cache and state file for outage fallback.
5. Publish config out of band during deploy or admin workflows, not on normal process start.

This keeps Sentinel as the configuration authority without forcing every service to depend on external shell wrappers for runtime resolution.

Typical usage:

```go
cfg := config.New(
    config.WithFile(serviceconfig.ConfigFilePath),
)

result, err := runtimeconfig.ResolveInto(ctx, cfg, log, runtimeconfig.ResolveOptions{
    BootstrapPath: serviceconfig.ConfigFilePath,
    Required:      runtimeconfig.RequiredFromEnv(),
})
if err != nil {
    return err
}
```

By default the loader:

- resolves the `runtime` namespace for the current service
- reads requested version selectors from `.config-version.json` and `.service-version.json`
- writes cache to `env/config.runtime-cache.json`
- writes state to `env/config.runtime-state.json`

The active Node service, `ecom-flow`, follows the same pattern with its own bootstrap loader in:

- `/Users/milanadhikari/personal/multiTenantPlatform/ecom-flow/components/app/ecom/server/src/bootstrap/runtimeConfig.ts`
