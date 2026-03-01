package observability

import (
	"testing"

	"github.com/milan604/core-lab/pkg/config"
)

func TestResolveSignozEndpoint(t *testing.T) {
	t.Run("prefers SignozEndpoint key", func(t *testing.T) {
		cfg := config.New(config.WithDefaults(map[string]any{
			"SIGNOZ_ENDPOINT": "http://legacy:4318",
			"SignozEndpoint":  "http://preferred:4318",
		}))
		got := resolveSignozEndpoint(cfg)
		want := "http://preferred:4318"
		if got != want {
			t.Fatalf("resolveSignozEndpoint() = %q, want %q", got, want)
		}
	})

	t.Run("falls back to SIGNOZ_ENDPOINT key", func(t *testing.T) {
		cfg := config.New(config.WithDefaults(map[string]any{
			"SIGNOZ_ENDPOINT": "http://legacy:4318",
		}))
		got := resolveSignozEndpoint(cfg)
		want := "http://legacy:4318"
		if got != want {
			t.Fatalf("resolveSignozEndpoint() = %q, want %q", got, want)
		}
	})

	t.Run("uses default when unset", func(t *testing.T) {
		cfg := config.New()
		got := resolveSignozEndpoint(cfg)
		if got != defaultSignozEndpoint {
			t.Fatalf("resolveSignozEndpoint() = %q, want %q", got, defaultSignozEndpoint)
		}
	})

	t.Run("handles nil config", func(t *testing.T) {
		got := resolveSignozEndpoint(nil)
		if got != defaultSignozEndpoint {
			t.Fatalf("resolveSignozEndpoint(nil) = %q, want %q", got, defaultSignozEndpoint)
		}
	})
}

func TestBuildSignozLogsURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "host and port",
			endpoint: "localhost:4318",
			want:     "http://localhost:4318/v1/logs",
		},
		{
			name:     "http endpoint",
			endpoint: "http://localhost:4318",
			want:     "http://localhost:4318/v1/logs",
		},
		{
			name:     "https endpoint",
			endpoint: "https://otel.example.com:4318",
			want:     "https://otel.example.com:4318/v1/logs",
		},
		{
			name:     "base path preserved",
			endpoint: "https://otel.example.com/collector",
			want:     "https://otel.example.com/collector/v1/logs",
		},
		{
			name:     "traces path rewritten to logs path",
			endpoint: "https://otel.example.com/v1/traces",
			want:     "https://otel.example.com/v1/logs",
		},
		{
			name:     "logs path kept",
			endpoint: "https://otel.example.com/v1/logs",
			want:     "https://otel.example.com/v1/logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSignozLogsURL(tt.endpoint)
			if got != tt.want {
				t.Fatalf("buildSignozLogsURL(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}
