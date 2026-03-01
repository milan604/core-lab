package observability

import (
	"net/url"
	"strings"

	"github.com/milan604/core-lab/pkg/config"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

const defaultSignozEndpoint = "http://localhost:4318"

// resolveSignozEndpoint resolves the SigNoz endpoint from config.
// Preferred key is SignozEndpoint, with SIGNOZ_ENDPOINT kept for compatibility.
func resolveSignozEndpoint(cfg *config.Config) string {
	if cfg == nil {
		return defaultSignozEndpoint
	}

	if endpoint := strings.TrimSpace(cfg.GetString("SignozEndpoint")); endpoint != "" {
		return endpoint
	}
	if endpoint := strings.TrimSpace(cfg.GetString("SIGNOZ_ENDPOINT")); endpoint != "" {
		return endpoint
	}

	return defaultSignozEndpoint
}

func otlpTraceExporterOptions(endpoint string) []otlptracehttp.Option {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultSignozEndpoint
	}

	if hasHTTPSScheme(endpoint) {
		return []otlptracehttp.Option{
			otlptracehttp.WithEndpointURL(endpoint),
		}
	}
	if hasHTTPScheme(endpoint) {
		return []otlptracehttp.Option{
			otlptracehttp.WithEndpointURL(endpoint),
			otlptracehttp.WithInsecure(),
		}
	}

	return []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	}
}

func buildSignozLogsURL(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultSignozEndpoint
	}
	if !hasHTTPScheme(endpoint) && !hasHTTPSScheme(endpoint) {
		endpoint = "http://" + endpoint
	}

	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return defaultSignozEndpoint + "/v1/logs"
	}

	u.RawQuery = ""
	u.Fragment = ""
	u.Path = normalizeLogsPath(u.Path)
	return u.String()
}

func normalizeLogsPath(path string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return "/v1/logs"
	}
	if strings.HasSuffix(trimmed, "/v1/logs") {
		return trimmed
	}
	if strings.HasSuffix(trimmed, "/v1/traces") {
		return strings.TrimSuffix(trimmed, "/v1/traces") + "/v1/logs"
	}
	return trimmed + "/v1/logs"
}

func hasHTTPScheme(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(endpoint), "http://")
}

func hasHTTPSScheme(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(endpoint), "https://")
}
