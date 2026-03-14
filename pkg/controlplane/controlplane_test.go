package controlplane

import (
	"reflect"
	"testing"
	"time"

	"github.com/milan604/core-lab/pkg/config"
)

func TestResolveBaseURLPrefersPlatformKey(t *testing.T) {
	t.Parallel()

	cfg := config.New(config.WithDefaults(map[string]any{
		KeyBaseURL:       "https://iam.example.com/control-plane/",
		LegacyKeyBaseURL: "https://legacy.example.com",
	}))

	got := ResolveBaseURL(cfg)
	if got != "https://iam.example.com/control-plane" {
		t.Fatalf("ResolveBaseURL() = %q, want %q", got, "https://iam.example.com/control-plane")
	}
}

func TestResolveBaseURLFallsBackToLegacySentinelPath(t *testing.T) {
	t.Parallel()

	cfg := config.New(config.WithDefaults(map[string]any{
		LegacyKeyBaseURL: "https://legacy.example.com",
	}))

	got := ResolveBaseURL(cfg)
	if got != "https://legacy.example.com/sentinel" {
		t.Fatalf("ResolveBaseURL() = %q, want %q", got, "https://legacy.example.com/sentinel")
	}
}

func TestResolveServiceTokenConfigSupportsNewKeys(t *testing.T) {
	t.Parallel()

	cfg := config.New(config.WithDefaults(map[string]any{
		KeyBaseURL:           "https://iam.example.com/control-plane",
		KeyServiceID:         "sites",
		KeyServiceAPIKey:     "service-key",
		KeyServiceTokenScope: "config.read",
		KeyTokenAudience:     []string{"config-api", "decision-api"},
		KeyMTLSCertFile:      "/tmp/sites.crt",
		KeyMTLSKeyFile:       "/tmp/sites.key",
		KeyMTLSCAFile:        "/tmp/ca.crt",
	}))

	got := ResolveServiceTokenConfig(cfg)
	want := ServiceTokenConfig{
		BaseURL:   "https://iam.example.com/control-plane",
		ServiceID: "sites",
		APIKey:    "service-key",
		Scope:     "config.read",
		Audience:  []string{"config-api", "decision-api"},
		MTLS: MTLSConfig{
			CertFile: "/tmp/sites.crt",
			KeyFile:  "/tmp/sites.key",
			CAFile:   "/tmp/ca.crt",
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveServiceTokenConfig() = %#v, want %#v", got, want)
	}
}

func TestResolveInternalAudienceDefaultsToPlatformInternal(t *testing.T) {
	t.Parallel()

	cfg := config.New()

	got := ResolveInternalAudience(cfg)
	want := []string{DefaultInternalAudience}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveInternalAudience() = %#v, want %#v", got, want)
	}
}

func TestResolveInternalAudienceSupportsPlatformAlias(t *testing.T) {
	t.Parallel()

	cfg := config.New(config.WithDefaults(map[string]any{
		KeyInternalAudience: []string{"config-api", "quota-api"},
	}))

	got := ResolveInternalAudience(cfg)
	want := []string{"config-api", "quota-api"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveInternalAudience() = %#v, want %#v", got, want)
	}
}

func TestResolveConfigAudienceDefaults(t *testing.T) {
	t.Parallel()

	cfg := config.New()

	got := ResolveConfigAudience(cfg)
	want := []string{DefaultConfigAudience}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveConfigAudience() = %#v, want %#v", got, want)
	}
}

func TestResolveQuotaAudienceSupportsPlatformAlias(t *testing.T) {
	t.Parallel()

	cfg := config.New(config.WithDefaults(map[string]any{
		KeyQuotaAudience: []string{"platform.quota", "platform.quota.write"},
	}))

	got := ResolveQuotaAudience(cfg)
	want := []string{"platform.quota", "platform.quota.write"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveQuotaAudience() = %#v, want %#v", got, want)
	}
}

func TestResolveMTLSConfigSupportsPlatformKeys(t *testing.T) {
	t.Parallel()

	cfg := config.New(config.WithDefaults(map[string]any{
		KeyMTLSCertFile: "/etc/certs/client.crt",
		KeyMTLSKeyFile:  "/etc/certs/client.key",
		KeyMTLSCAFile:   "/etc/certs/ca.crt",
	}))

	got := ResolveMTLSConfig(cfg)
	want := MTLSConfig{
		CertFile: "/etc/certs/client.crt",
		KeyFile:  "/etc/certs/client.key",
		CAFile:   "/etc/certs/ca.crt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveMTLSConfig() = %#v, want %#v", got, want)
	}
}

func TestResolveJWKSCacheTTLFallsBackToDefault(t *testing.T) {
	t.Parallel()

	cfg := config.New(config.WithDefaults(map[string]any{
		KeyJWKSCacheTTLSeconds: "invalid",
	}))

	got := ResolveJWKSCacheTTL(cfg, 5*time.Minute)
	if got != 5*time.Minute {
		t.Fatalf("ResolveJWKSCacheTTL() = %v, want %v", got, 5*time.Minute)
	}
}

func TestAPIEndpoints(t *testing.T) {
	t.Parallel()

	api := API{BaseURL: "https://iam.example.com/control-plane"}

	if got := api.ServiceTokenURL(); got != "https://iam.example.com/control-plane/internal/api/v1/service-token" {
		t.Fatalf("ServiceTokenURL() = %q", got)
	}
	if got := api.RolePermissionsURL("role/with spaces"); got != "https://iam.example.com/control-plane/api/v1/roles/role%2Fwith%20spaces/permissions" {
		t.Fatalf("RolePermissionsURL() = %q", got)
	}
	if got := api.AuthorizationDecisionURL(); got != "https://iam.example.com/control-plane/internal/api/v1/authz/decide" {
		t.Fatalf("AuthorizationDecisionURL() = %q", got)
	}
	if got := api.ConfigResolveURL(); got != "https://iam.example.com/control-plane/internal/api/v1/config/resolve" {
		t.Fatalf("ConfigResolveURL() = %q", got)
	}
	if got := api.ConfigPublishURL(); got != "https://iam.example.com/control-plane/internal/api/v1/config/publish" {
		t.Fatalf("ConfigPublishURL() = %q", got)
	}
}
