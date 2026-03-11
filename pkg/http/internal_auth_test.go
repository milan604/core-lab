package http

import (
	"reflect"
	"testing"

	"github.com/milan604/core-lab/pkg/config"
)

func TestInternalAuthKeyPrefersInternalAdminKey(t *testing.T) {
	cfg := config.New(config.WithDefaults(map[string]any{
		"InternalAdminKey":      "platform-internal",
		"SentinelServiceAPIKey": "service-api-key",
	}))

	got := InternalAuthKey(cfg, "SentinelServiceAPIKey")
	if got != "platform-internal" {
		t.Fatalf("InternalAuthKey() = %q, want %q", got, "platform-internal")
	}
}

func TestInternalAuthKeysDeduplicatesAndFallsBack(t *testing.T) {
	cfg := config.New(config.WithDefaults(map[string]any{
		"TenantServiceAPIKey":   "tenant-key",
		"SentinelServiceAPIKey": "tenant-key",
	}))

	got := InternalAuthKeys(cfg, "TenantServiceAPIKey", "SentinelServiceAPIKey")
	want := []string{"tenant-key"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InternalAuthKeys() = %#v, want %#v", got, want)
	}
}
