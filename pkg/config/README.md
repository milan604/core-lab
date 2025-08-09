# Config Package

This package wraps [Viper](https://github.com/spf13/viper) to provide flexible configuration management for Go applications, with extra helpers and options for common use cases.

## Features
- Centralized config loading from files, environment variables, flags, and .env files
- Hot-reload support (watch for config changes)
- Support for config profiles (e.g., dev, prod)
- Sensitive key masking for logs/prints
- Typed getters with defaults
- Merge additional config files
- Save current config to file
- Custom error handling and validation

## Usage Example
```go
import "github.com/yourorg/corelab/pkg/config"

cfg := config.New(
    config.WithDefaults(map[string]interface{}{"service.port": 8080}),
    config.WithFile("config.yaml"),
    config.WithEnv("APP"),
    config.WithPFlags(pflag.CommandLine),
    config.WithWatch(func() { fmt.Println("Config reloaded!") }),
)

// Access config values
port := cfg.GetIntD("service.port", 8080)
user := cfg.GetStringD("database.user", "root")

// Validate required keys
if err := cfg.ValidateRequired("service.port", "database.user"); err != nil {
    log.Fatal(err)
}

// Print config with sensitive keys masked
cfg.Print(true)
```

## API Reference

### Options (for `config.New`)
- `WithDefaults(map[string]interface{})` — Set default values
- `WithFile(path string)` — Load exact config file
- `WithConfigNamePaths(name string, paths ...string)` — Search for config file by name in paths
- `WithFormat(format string)` — Force config format (yaml/json/toml)
- `WithProfile(baseName, profile string, paths ...string)` — Load profile-specific config (e.g., config.dev.yaml)
- `WithEnv(prefix string)` — Enable environment variable overrides
- `WithPFlags(flags *pflag.FlagSet)` — Bind CLI flags
- `WithAutoPFlags()` — Register common flags automatically
- `WithDotEnv(path string)` — Load .env file
- `WithWatch(onChange func())` — Enable hot-reload
- `WithSensitiveKeys(keys ...string)` — Register sensitive keys for masking
- `WithRemoteProvider(loader func(*viper.Viper) error)` — Load config from remote provider

### Methods
- `GetStringD(key, def string) string` — Get string value or default
- `GetIntD(key string, def int) int` — Get int value or default
- `GetBoolD(key string, def bool) bool` — Get bool value or default
- `GetDurationD(key string, def time.Duration) time.Duration` — Get duration or default
- `ValidateRequired(keys ...string) error` — Ensure required keys are set
- `MaskedSettings() map[string]interface{}` — Get config with sensitive keys redacted
- `Print(mask bool)` — Print config to stdout, mask sensitive keys if true
- `MergeInFile(path string) error` — Merge another config file
- `Save(path string) error` — Save current config to file

## Hot Reload Example
```go
cfg := config.New(
    config.WithFile("config.yaml"),
    config.WithWatch(func() {
        fmt.Println("Config file changed!")
    }),
)
```

## Remote Provider Example
```go
cfg := config.New(
    config.WithRemoteProvider(func(v *viper.Viper) error {
        // Load from Consul, etcd, S3, etc.
        return nil
    }),
)
```

## Notes
- All Viper methods are available via `cfg.Viper`.
- Errors during config loading are logged but do not stop execution (unless you handle them).
- Sensitive keys are masked in output if registered via `WithSensitiveKeys`.
