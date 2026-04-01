package runtimeconfig

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/configmanager"
	"github.com/milan604/core-lab/pkg/controlplane"
	"github.com/milan604/core-lab/pkg/logger"
)

type fakeResolver struct {
	response configmanager.NamespaceResolveResponse
	err      error
}

func (f fakeResolver) ResolveNamespace(context.Context, configmanager.NamespaceResolveRequest) (configmanager.NamespaceResolveResponse, error) {
	if f.err != nil {
		return configmanager.NamespaceResolveResponse{}, f.err
	}
	return f.response, nil
}

func TestResolveIntoWithResolverUsesResolvedConfigAndWritesCache(t *testing.T) {
	t.Setenv("ENVIRONMENT", "staging")

	tempDir := t.TempDir()
	bootstrapPath := filepath.Join(tempDir, "env", "config.json")
	cachePath := DefaultCachePath(bootstrapPath)
	statePath := DefaultStatePath(bootstrapPath)

	cfg := config.New()
	cfg.Set(controlplane.KeyServiceID, "tenant-management")
	cfg.Set(controlplane.KeyBaseURL, "https://control-plane.example.com")
	cfg.Set(controlplane.KeyServiceAPIKey, "bootstrap-key")
	cfg.Set(controlplane.KeyMTLSCertFile, "/tmp/client.crt")
	cfg.Set(controlplane.KeyMTLSKeyFile, "/tmp/client.key")
	cfg.Set(controlplane.KeyMTLSCAFile, "/tmp/ca.crt")
	cfg.Set(controlplane.KeyServiceAPIKey, "bootstrap-key")

	result, err := resolveIntoWithResolver(
		context.Background(),
		cfg,
		logger.MustNewDefaultLogger(),
		ResolveOptions{
			BootstrapPath: bootstrapPath,
			Required:      true,
		},
		fakeResolver{
			response: configmanager.NamespaceResolveResponse{
				ReleaseID: "release-1",
				Version:   "1.2.3",
				ETag:      "etag-1",
				Config: map[string]any{
					"DatabaseHost":        "db.internal",
					"PlatformTokenIssuer": "https://issuer.example.com",
				},
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ResolveInto returned error: %v", err)
	}

	if result.Source != SourceResolved {
		t.Fatalf("expected source %q, got %q", SourceResolved, result.Source)
	}
	if got := cfg.GetString("DatabaseHost"); got != "db.internal" {
		t.Fatalf("expected merged DatabaseHost, got %q", got)
	}
	if got := cfg.GetString(controlplane.KeyServiceAPIKey); got != "bootstrap-key" {
		t.Fatalf("expected bootstrap key to remain present, got %q", got)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected cache file to exist: %v", err)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}
}

func TestResolveIntoWithResolverFallsBackToCache(t *testing.T) {
	tempDir := t.TempDir()
	bootstrapPath := filepath.Join(tempDir, "env", "config.json")
	cachePath := DefaultCachePath(bootstrapPath)

	if err := writeJSONFile(cachePath, cacheEnvelope{
		ServiceID:    "upload-service",
		NamespaceKey: DefaultNamespaceKey,
		Version:      "9.9.9",
		ETag:         "etag-cache",
		Config: map[string]any{
			"DatabaseHost": "cached-db.internal",
		},
	}); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	cfg := config.New()

	result, err := resolveIntoWithResolver(
		context.Background(),
		cfg,
		logger.MustNewDefaultLogger(),
		ResolveOptions{
			BootstrapPath: bootstrapPath,
			Required:      true,
		},
		fakeResolver{err: errors.New("sentinel unavailable")},
		nil,
	)
	if err != nil {
		t.Fatalf("ResolveInto returned error: %v", err)
	}

	if result.Source != SourceCache {
		t.Fatalf("expected source %q, got %q", SourceCache, result.Source)
	}
	if got := cfg.GetString("DatabaseHost"); got != "cached-db.internal" {
		t.Fatalf("expected cached DatabaseHost, got %q", got)
	}
}

func TestResolveIntoWithResolverReturnsBootstrapWhenOptional(t *testing.T) {
	tempDir := t.TempDir()
	bootstrapPath := filepath.Join(tempDir, "env", "config.json")

	cfg := config.New()
	cfg.Set("DatabaseHost", "local-db.internal")

	result, err := resolveIntoWithResolver(
		context.Background(),
		cfg,
		logger.MustNewDefaultLogger(),
		ResolveOptions{
			BootstrapPath: bootstrapPath,
			Required:      false,
		},
		nil,
		errors.New("machine identity not configured"),
	)
	if err != nil {
		t.Fatalf("ResolveInto returned error: %v", err)
	}

	if result.Source != SourceBootstrap {
		t.Fatalf("expected source %q, got %q", SourceBootstrap, result.Source)
	}
	if got := cfg.GetString("DatabaseHost"); got != "local-db.internal" {
		t.Fatalf("expected bootstrap config to remain, got %q", got)
	}
}
