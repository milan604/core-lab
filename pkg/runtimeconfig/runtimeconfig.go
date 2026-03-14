package runtimeconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/configmanager"
	"github.com/milan604/core-lab/pkg/controlplane"
	"github.com/milan604/core-lab/pkg/logger"
)

const (
	DefaultNamespaceKey = "runtime"

	SourceResolved  = "resolved"
	SourceCache     = "cache"
	SourceBootstrap = "bootstrap"
)

type ResolveOptions struct {
	ServiceID     string
	NamespaceKey  string
	Environment   string
	TenantID      string
	ProjectID     string
	Version       string
	BootstrapPath string
	CachePath     string
	StatePath     string
	VersionFiles  []string
	Required      bool
}

type ResolveResult struct {
	Source           string
	ServiceID        string
	NamespaceKey     string
	Version          string
	RequestedVersion string
	ReleaseID        string
	ETag             string
	CachePath        string
	StatePath        string
}

type namespaceResolver interface {
	ResolveNamespace(context.Context, configmanager.NamespaceResolveRequest) (configmanager.NamespaceResolveResponse, error)
}

type cacheEnvelope struct {
	ServiceID    string         `json:"service_id,omitempty"`
	NamespaceKey string         `json:"namespace_key,omitempty"`
	ReleaseID    string         `json:"release_id,omitempty"`
	Version      string         `json:"version,omitempty"`
	ETag         string         `json:"etag,omitempty"`
	Environment  string         `json:"environment,omitempty"`
	TenantID     string         `json:"tenant_id,omitempty"`
	ProjectID    string         `json:"project_id,omitempty"`
	CachedAt     string         `json:"cached_at,omitempty"`
	Config       map[string]any `json:"config"`
}

type stateEnvelope struct {
	Source           string `json:"source"`
	ServiceID        string `json:"service_id,omitempty"`
	NamespaceKey     string `json:"namespace_key,omitempty"`
	Version          string `json:"version,omitempty"`
	RequestedVersion string `json:"requested_version,omitempty"`
	ReleaseID        string `json:"release_id,omitempty"`
	ETag             string `json:"etag,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

func ResolveInto(ctx context.Context, cfg *config.Config, log logger.LogManager, opts ResolveOptions) (ResolveResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return ResolveResult{}, fmt.Errorf("config is required")
	}
	if log == nil {
		log = logger.MustNewDefaultLogger()
	}

	resolver, err := configmanager.NewClient(cfg, log)
	return resolveIntoWithResolver(ctx, cfg, log, opts, resolver, err)
}

func RequiredFromEnv() bool {
	return envFlag("CNM_REQUIRED", false)
}

func DefaultCachePath(bootstrapPath string) string {
	path := strings.TrimSpace(bootstrapPath)
	if path == "" {
		return filepath.Clean(filepath.Join("env", "config.runtime-cache.json"))
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".json"
	}

	return filepath.Clean(filepath.Join(dir, name+".runtime-cache"+ext))
}

func DefaultStatePath(bootstrapPath string) string {
	path := strings.TrimSpace(bootstrapPath)
	if path == "" {
		return filepath.Clean(filepath.Join("env", "config.runtime-state.json"))
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".json"
	}

	return filepath.Clean(filepath.Join(dir, name+".runtime-state"+ext))
}

func DefaultVersionFiles(bootstrapPath string) []string {
	root := "."
	dir := filepath.Dir(strings.TrimSpace(bootstrapPath))
	if dir != "" && dir != "." {
		root = filepath.Dir(dir)
		if root == "" {
			root = "."
		}
	}

	return []string{
		filepath.Clean(filepath.Join(root, ".config-version.json")),
		filepath.Clean(filepath.Join(root, ".service-version.json")),
	}
}

func resolveIntoWithResolver(
	ctx context.Context,
	cfg *config.Config,
	log logger.LogManager,
	opts ResolveOptions,
	resolver namespaceResolver,
	resolverErr error,
) (ResolveResult, error) {
	opts = normalizeOptions(cfg, opts)

	result := ResolveResult{
		ServiceID:        opts.ServiceID,
		NamespaceKey:     opts.NamespaceKey,
		RequestedVersion: opts.Version,
		CachePath:        opts.CachePath,
		StatePath:        opts.StatePath,
	}

	if resolverErr == nil && resolver != nil && opts.ServiceID != "" {
		resolved, err := resolver.ResolveNamespace(ctx, configmanager.NamespaceResolveRequest{
			ServiceID:    opts.ServiceID,
			NamespaceKey: opts.NamespaceKey,
			Version:      opts.Version,
			Environment:  opts.Environment,
			TenantID:     opts.TenantID,
			ProjectID:    opts.ProjectID,
		})
		if err == nil {
			if err := mergeConfig(cfg, resolved.Config); err != nil {
				return result, fmt.Errorf("merge resolved runtime config: %w", err)
			}

			cache := cacheEnvelope{
				ServiceID:    opts.ServiceID,
				NamespaceKey: opts.NamespaceKey,
				ReleaseID:    resolved.ReleaseID,
				Version:      resolved.Version,
				ETag:         resolved.ETag,
				Environment:  opts.Environment,
				TenantID:     opts.TenantID,
				ProjectID:    opts.ProjectID,
				CachedAt:     time.Now().UTC().Format(time.RFC3339),
				Config:       cloneMap(resolved.Config),
			}
			if writeErr := writeJSONFile(opts.CachePath, cache); writeErr != nil {
				log.WarnF("failed to persist runtime config cache at %s: %v", opts.CachePath, writeErr)
			}

			result.Source = SourceResolved
			result.ReleaseID = resolved.ReleaseID
			result.Version = resolved.Version
			result.ETag = resolved.ETag

			if writeErr := writeStateFile(opts.StatePath, result); writeErr != nil {
				log.WarnF("failed to persist runtime config state at %s: %v", opts.StatePath, writeErr)
			}

			return result, nil
		}

		resolverErr = err
		log.WarnF("runtime config resolve failed for %s/%s: %v", opts.ServiceID, opts.NamespaceKey, err)
	} else if resolverErr != nil {
		log.WarnF("runtime config resolver unavailable for %s/%s: %v", opts.ServiceID, opts.NamespaceKey, resolverErr)
	}

	cache, cacheErr := readCacheFile(opts.CachePath)
	if cacheErr == nil {
		if err := mergeConfig(cfg, cache.Config); err != nil {
			return result, fmt.Errorf("merge cached runtime config: %w", err)
		}

		result.Source = SourceCache
		result.ReleaseID = cache.ReleaseID
		result.Version = cache.Version
		result.ETag = cache.ETag

		if writeErr := writeStateFile(opts.StatePath, result); writeErr != nil {
			log.WarnF("failed to persist runtime config state at %s: %v", opts.StatePath, writeErr)
		}

		return result, nil
	}

	if opts.Required {
		switch {
		case resolverErr != nil && cacheErr != nil:
			return result, fmt.Errorf("runtime config unavailable: resolve failed: %v; cache unavailable: %v", resolverErr, cacheErr)
		case resolverErr != nil:
			return result, fmt.Errorf("runtime config unavailable: %w", resolverErr)
		case cacheErr != nil:
			return result, fmt.Errorf("runtime config cache unavailable: %w", cacheErr)
		default:
			return result, fmt.Errorf("runtime config unavailable")
		}
	}

	result.Source = SourceBootstrap
	if writeErr := writeStateFile(opts.StatePath, result); writeErr != nil {
		log.WarnF("failed to persist runtime config state at %s: %v", opts.StatePath, writeErr)
	}
	return result, nil
}

func normalizeOptions(cfg *config.Config, opts ResolveOptions) ResolveOptions {
	if opts.ServiceID == "" {
		opts.ServiceID = controlplane.ResolveServiceID(cfg)
	}
	if opts.NamespaceKey == "" {
		opts.NamespaceKey = DefaultNamespaceKey
	}
	if opts.Environment == "" {
		opts.Environment = firstNonEmptyEnv("CNM_ENVIRONMENT", "CONFIG_ENVIRONMENT", "ENVIRONMENT", "DEPLOY_ENV")
	}
	if opts.TenantID == "" {
		opts.TenantID = firstNonEmptyEnv("CNM_TENANT_ID", "CONFIG_TENANT_ID")
	}
	if opts.ProjectID == "" {
		opts.ProjectID = firstNonEmptyEnv("CNM_PROJECT_ID", "CONFIG_PROJECT_ID")
	}
	if opts.CachePath == "" {
		opts.CachePath = DefaultCachePath(opts.BootstrapPath)
	}
	if opts.StatePath == "" {
		opts.StatePath = DefaultStatePath(opts.BootstrapPath)
	}
	if len(opts.VersionFiles) == 0 {
		opts.VersionFiles = DefaultVersionFiles(opts.BootstrapPath)
	}
	if opts.Version == "" {
		opts.Version = readRequestedVersion(opts.VersionFiles)
	}
	return opts
}

func mergeConfig(cfg *config.Config, values map[string]any) error {
	if len(values) == 0 {
		return nil
	}
	return cfg.MergeConfigMap(values)
}

func readRequestedVersion(paths []string) string {
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}

		content, err := os.ReadFile(trimmed)
		if err != nil {
			continue
		}

		var payload struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(content, &payload); err != nil {
			continue
		}
		if version := strings.TrimSpace(payload.Version); version != "" {
			return version
		}
	}
	return ""
}

func readCacheFile(path string) (cacheEnvelope, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return cacheEnvelope{}, fmt.Errorf("cache path is empty")
	}

	content, err := os.ReadFile(trimmed)
	if err != nil {
		return cacheEnvelope{}, err
	}

	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil {
		return cacheEnvelope{}, err
	}

	if nested, ok := raw["config"].(map[string]any); ok {
		return cacheEnvelope{
			ServiceID:    stringFromRaw(raw["service_id"]),
			NamespaceKey: stringFromRaw(raw["namespace_key"]),
			ReleaseID:    stringFromRaw(raw["release_id"]),
			Version:      stringFromRaw(raw["version"]),
			ETag:         stringFromRaw(raw["etag"]),
			Environment:  stringFromRaw(raw["environment"]),
			TenantID:     stringFromRaw(raw["tenant_id"]),
			ProjectID:    stringFromRaw(raw["project_id"]),
			CachedAt:     stringFromRaw(raw["cached_at"]),
			Config:       nested,
		}, nil
	}

	return cacheEnvelope{
		Config: raw,
	}, nil
}

func writeStateFile(path string, result ResolveResult) error {
	return writeJSONFile(path, stateEnvelope{
		Source:           result.Source,
		ServiceID:        result.ServiceID,
		NamespaceKey:     result.NamespaceKey,
		Version:          result.Version,
		RequestedVersion: result.RequestedVersion,
		ReleaseID:        result.ReleaseID,
		ETag:             result.ETag,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	})
}

func writeJSONFile(path string, payload any) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(trimmed), 0o755); err != nil {
		return err
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')

	return os.WriteFile(trimmed, body, 0o600)
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func envFlag(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	case "":
		return fallback
	default:
		return fallback
	}
}

func stringFromRaw(raw any) string {
	if value, ok := raw.(string); ok {
		return value
	}
	return ""
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}

	body, err := json.Marshal(input)
	if err != nil {
		return input
	}

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return input
	}
	return out
}
