package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config is the wrapper around viper with extra helpers.
type Config struct {
	*viper.Viper

	// private
	sensitiveKeys map[string]struct{}
	onChange      func()
}

// Option is a functional option for New.
type Option func(*Config) error

// New creates a Config instance. Use options to customize behavior.
// Example:
//
//	cfg := config.New(
//	  config.WithDefaults(map[string]interface{}{"service.port":8080}),
//	  config.WithFile("config.yaml"),
//	  config.WithEnv("APP"),
//	  config.WithPFlags(pflag.CommandLine),
//	  config.WithWatch(func(){ fmt.Println("reloaded") }),
//	)
func New(opts ...Option) *Config {
	v := viper.New()
	cfg := &Config{
		Viper:         v,
		sensitiveKeys: map[string]struct{}{},
	}

	// apply options
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			log.Fatalf("config: applying option failed: %v", err)
		}
	}

	// try to read config (if file was set via options)
	if err := cfg.readConfigIfPossible(); err != nil {
		// non-fatal; user might only want env/flags/defaults
		log.Printf("config: read config warning: %v", err)
	}

	return cfg
}

func (c *Config) readConfigIfPossible() error {
	// Try to read config if a config file is set
	if c.ConfigFileUsed() != "" {
		return c.ReadInConfig()
	}

	// If no config file, but config paths and name are set, try to read config
	// Viper does not expose ConfigFile() or ConfigName(), so we rely on configFileUsed
	// and assume that SetConfigName was called if user wants to search for a config
	// file by name in the configured paths.
	// If no config file was found, ReadInConfig will error, which is fine.
	if c.ConfigFileUsed() == "" {
		err := c.ReadInConfig()
		// If no config found, return error (caller can decide if it's fatal)
		return err
	}
	return nil
}

/* ---------------------------
   Options
----------------------------*/

// WithDefaults sets default values (applied first)
func WithDefaults(defaults map[string]interface{}) Option {
	return func(c *Config) error {
		for k, v := range defaults {
			c.SetDefault(k, v)
		}
		return nil
	}
}

// WithFile sets an exact config file (absolute or relative).
// viper will use SetConfigFile(path) so the extension determines type.
func WithFile(path string) Option {
	return func(c *Config) error {
		if path == "" {
			return nil
		}
		c.SetConfigFile(path)
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		if ext != "" {
			c.SetConfigType(ext)
		}
		return nil
	}
}

// WithConfigNamePaths sets config name (without ext) and search paths.
// format may be empty (viper will infer from file names/extensions).
func WithConfigNamePaths(name string, paths ...string) Option {
	return func(c *Config) error {
		if name != "" {
			c.SetConfigName(name)
		}
		if len(paths) == 0 {
			paths = []string{".", "./env", "/etc/core-lab"}
		}
		for _, p := range paths {
			c.AddConfigPath(p)
		}
		return nil
	}
}

// WithFormat forces config format (yaml/json/toml/etc.) when files don't have extension.
func WithFormat(format string) Option {
	return func(c *Config) error {
		if format != "" {
			c.SetConfigType(format)
		}
		return nil
	}
}

// WithProfile loads a profile-specific file name (e.g., config.dev.yaml)
func WithProfile(baseName, profile string, paths ...string) Option {
	return func(c *Config) error {
		name := baseName
		if profile == "" {
			profile = os.Getenv("APP_ENV")
		}
		if profile != "" {
			name = fmt.Sprintf("%s.%s", baseName, profile)
		}
		// reuse WithConfigNamePaths
		return WithConfigNamePaths(name, paths...)(c)
	}
}

// WithEnv enables environment variable overrides.
// prefix = "APP" means APP_FOO will override foo.
// replacer maps dots/hyphens in keys to underscores in envs.
func WithEnv(prefix string) Option {
	return func(c *Config) error {
		if prefix != "" {
			c.SetEnvPrefix(prefix)
		}
		c.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
		c.AutomaticEnv()
		return nil
	}
}

// WithPFlags binds a pflag.FlagSet to viper. If flags are nil, we bind the default command line.
func WithPFlags(flags *pflag.FlagSet) Option {
	return func(c *Config) error {
		if flags == nil {
			flags = pflag.CommandLine
		}
		// NOTE: do not define flags here; the application should define them.
		// Here we just bind whatever flags are present.
		if err := c.BindPFlags(flags); err != nil {
			return err
		}
		return nil
	}
}

// WithAutoPFlags will attempt to register a few common flags (helpful for examples).
// Use WithPFlags if you want full control.
func WithAutoPFlags() Option {
	return func(c *Config) error {
		// Common examples; apps can still define their own flags prior to New()
		pflag.String("service.port", "", "service port")
		pflag.String("database.host", "", "database host")
		pflag.String("database.user", "", "database user")
		pflag.String("database.password", "", "database password")
		pflag.String("log.level", "", "log level")
		pflag.Parse()
		return WithPFlags(pflag.CommandLine)(c)
	}
}

// WithDotEnv reads key=val lines from a .env file (path) and merges into viper.
// If path is empty, attempts ".env" in working directory.
func WithDotEnv(path string) Option {
	return func(c *Config) error {
		if path == "" {
			path = ".env"
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil
		}
		envV := viper.New()
		envV.SetConfigFile(path)
		envV.SetConfigType("env")
		if err := envV.ReadInConfig(); err != nil {
			return err
		}
		for _, k := range envV.AllKeys() {
			c.Set(k, envV.Get(k))
		}
		return nil
	}
}

// WithWatch enables hot-reload. onChange will be called after a successful reload.
func WithWatch(onChange func()) Option {
	return func(c *Config) error {
		c.WatchConfig()
		c.onChange = onChange
		c.OnConfigChange(func(e fsnotify.Event) {
			log.Printf("config: file changed: %s", e.Name)
			// merge again (viper handles reload) - call callback
			if c.onChange != nil {
				c.onChange()
			}
		})
		return nil
	}
}

// WithSensitiveKeys registers keys which should be redacted when printing/logging.
func WithSensitiveKeys(keys ...string) Option {
	return func(c *Config) error {
		for _, k := range keys {
			c.sensitiveKeys[k] = struct{}{}
		}
		return nil
	}
}

// WithRemoteProvider is a hook for remote providers (Consul/etcd/S3).
// Pass a function that will perform remote load/merge using the provided viper instance.
func WithRemoteProvider(loader func(v *viper.Viper) error) Option {
	return func(c *Config) error {
		if loader == nil {
			return nil
		}
		return loader(c.Viper)
	}
}

/* ---------------------------
   Helpers / Actions
----------------------------*/

// MergeInFile merges another config file into the current config (keeps overrides)
func (c *Config) MergeInFile(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	// create a temporary viper to read that file
	tmp := viper.New()
	tmp.SetConfigFile(path)
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext != "" {
		tmp.SetConfigType(ext)
	}
	if err := tmp.ReadInConfig(); err != nil {
		return err
	}
	return c.MergeConfigMap(tmp.AllSettings())
}

// Save writes current effective configuration as a file of given type.
// path: full path to write, format inferred from extension (e.g., .yaml)
func (c *Config) Save(path string) error {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "" {
		return fmt.Errorf("cannot infer format from path: %s", path)
	}
	// ensure dir exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return c.WriteConfigAs(path)
}

/* ---------------------------
   Typed getters with defaults
----------------------------*/

// GetStringD returns string or def
func (c *Config) GetStringD(key, def string) string {
	if val := c.GetString(key); val != "" {
		return val
	}
	return def
}

// GetIntD returns int or def
func (c *Config) GetIntD(key string, def int) int {
	if c.IsSet(key) {
		return c.GetInt(key)
	}
	return def
}

// GetBoolD returns bool or def
func (c *Config) GetBoolD(key string, def bool) bool {
	if c.IsSet(key) {
		return c.GetBool(key)
	}
	return def
}

// GetDurationD returns time.Duration or def
func (c *Config) GetDurationD(key string, def time.Duration) time.Duration {
	if c.IsSet(key) {
		return c.GetDuration(key)
	}
	return def
}

/* ---------------------------
   Validation & Utilities
----------------------------*/

// ValidateRequired ensures keys exist and are non-empty.
func (c *Config) ValidateRequired(keys ...string) error {
	var missing []string
	for _, k := range keys {
		if !c.IsSet(k) || c.GetString(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required keys: %v", strings.Join(missing, ", "))
	}
	return nil
}

// MaskedSettings returns a copy of AllSettings with sensitive keys redacted.
func (c *Config) MaskedSettings() map[string]interface{} {
	all := c.AllSettings()
	redacted := map[string]interface{}{}
	for k, v := range all {
		if _, ok := c.sensitiveKeys[k]; ok {
			redacted[k] = "***REDACTED***"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// Print prints all settings to stdout with optional masking for sensitive keys.
func (c *Config) Print(mask bool) {
	all := c.AllSettings()
	for k, v := range all {
		if mask {
			if _, ok := c.sensitiveKeys[k]; ok {
				fmt.Printf("%s = %s\n", k, "***REDACTED***")
				continue
			}
		}
		fmt.Printf("%s = %v\n", k, v)
	}
}
