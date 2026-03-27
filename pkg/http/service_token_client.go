package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/controlplane"
	"github.com/milan604/core-lab/pkg/logger"
)

// ServiceTokenProvider implements TokenProvider for service token authentication.
// It calls a service token API to fetch tokens for service-to-service communication.
type ServiceTokenProvider struct {
	ServiceURL string
	ServiceID  string
	APIKey     string
	Scope      string
	Audience   []string
	HTTPClient HTTPClient
}

// ServiceTokenProviderConfig holds configuration for ServiceTokenProvider.
type ServiceTokenProviderConfig struct {
	ServiceURL string
	ServiceID  string
	APIKey     string
	Scope      string
	Audience   []string
	HTTPClient HTTPClient
}

// NewServiceTokenProvider creates a new service token provider.
func NewServiceTokenProvider(cfg ServiceTokenProviderConfig) *ServiceTokenProvider {
	if cfg.Scope == "" {
		cfg.Scope = "service"
	}

	// Create a basic client without token provider for fetching tokens if not provided
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = NewClient()
	}

	return &ServiceTokenProvider{
		ServiceURL: cfg.ServiceURL,
		ServiceID:  cfg.ServiceID,
		APIKey:     cfg.APIKey,
		Scope:      cfg.Scope,
		Audience:   cfg.Audience,
		HTTPClient: httpClient,
	}
}

// FetchToken retrieves a token from the service token API.
func (p *ServiceTokenProvider) FetchToken(ctx context.Context) (string, time.Time, error) {
	if p.ServiceURL == "" {
		return "", time.Time{}, fmt.Errorf("service URL is required")
	}
	if p.ServiceID == "" {
		return "", time.Time{}, fmt.Errorf("service ID is required")
	}
	if p.APIKey == "" {
		return "", time.Time{}, fmt.Errorf("API key is required")
	}
	req := map[string]any{
		"service_id": p.ServiceID,
		"api_key":    p.APIKey,
	}
	if scope := strings.TrimSpace(p.Scope); scope != "" {
		req["scope"] = scope
	}
	if len(p.Audience) > 0 {
		req["audience"] = p.Audience
	}

	var resp struct {
		AccessToken string   `json:"access_token"`
		TokenType   string   `json:"token_type"`
		ExpiresIn   int      `json:"expires_in"`
		ExpiresAt   string   `json:"expires_at"` // RFC3339 formatted string
		Scope       string   `json:"scope"`
		Audience    []string `json:"audience"`
	}

	url := fmt.Sprintf("%s/internal/api/v1/service-token", p.ServiceURL)
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to marshal service token request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create service token request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if err := p.HTTPClient.DoJSON(ctx, httpReq, &resp); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to call service token API: %w", err)
	}

	if resp.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("empty access token in response")
	}

	expiresAt := p.parseExpirationTime(resp.ExpiresAt, resp.ExpiresIn)
	return resp.AccessToken, expiresAt, nil
}

// parseExpirationTime parses the expiration time from ExpiresAt (RFC3339) or ExpiresIn (seconds).
func (p *ServiceTokenProvider) parseExpirationTime(expiresAtStr string, expiresIn int) time.Time {
	// Try to parse ExpiresAt first (RFC3339 format)
	if expiresAtStr != "" {
		if parsed, err := time.Parse(time.RFC3339, expiresAtStr); err == nil {
			return parsed
		}
	}

	// Fallback to ExpiresIn
	if expiresIn > 0 {
		return time.Now().Add(time.Duration(expiresIn) * time.Second)
	}

	// Default to 1 hour if neither is provided
	return time.Now().Add(1 * time.Hour)
}

// NewClientWithServiceToken creates a new HTTP client with service token provider configured.
// This function handles service token fetching internally - services don't need to call the API themselves.
//
// Services need to pass config with:
// - PlatformControlPlaneEndpoint or SentinelServiceEndpoint
// - PlatformServiceID
// - PlatformServiceAPIKey
// - PlatformMTLSCertFile / PlatformMTLSKeyFile / PlatformMTLSCAFile
func NewClientWithServiceToken(log logger.LogManager, cfg *config.Config) (*Client, error) {
	return NewClientWithServiceTokenForAudience(log, cfg, nil)
}

// NewClientWithServiceTokenForAudience creates a token-authenticated HTTP client
// and optionally overrides the requested token audience.
func NewClientWithServiceTokenForAudience(log logger.LogManager, cfg *config.Config, audience []string) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config not configured")
	}

	settings := controlplane.ResolveServiceTokenConfig(cfg)
	if missing := settings.MissingRequiredFields(); len(missing) > 0 {
		return nil, fmt.Errorf("service token configuration missing required keys: %s", strings.Join(missing, ", "))
	}
	if len(audience) > 0 {
		settings.Audience = audience
	}

	mtlsOpts, err := controlPlaneMTLSOptions(log, settings.MTLS)
	if err != nil {
		return nil, err
	}

	// Create base HTTP client for calling service token API
	// This client doesn't need token provider - it's used to get the token
	baseClient := NewClient(mtlsOpts...)

	// Create service token provider
	tokenProvider := NewServiceTokenProvider(ServiceTokenProviderConfig{
		ServiceURL: settings.BaseURL,
		ServiceID:  settings.ServiceID,
		APIKey:     settings.APIKey,
		Scope:      settings.Scope,
		Audience:   settings.Audience,
		HTTPClient: baseClient,
	})

	clientOpts := append([]ClientOption{}, mtlsOpts...)
	clientOpts = append(clientOpts, WithTokenProvider(tokenProvider, 1*time.Minute))

	// Create HTTP client with token provider configured and the same trust
	// configuration used for token retrieval.
	return NewClient(clientOpts...), nil
}

// NewInternalControlPlaneClient creates a token-authenticated client scoped to
// privileged control-plane APIs via the configured internal audience.
func NewInternalControlPlaneClient(log logger.LogManager, cfg *config.Config) (*Client, error) {
	return NewInternalControlPlaneClientForAudience(log, cfg, controlplane.ResolveInternalAudience(cfg))
}

func NewInternalControlPlaneClientForAudience(log logger.LogManager, cfg *config.Config, audience []string) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config not configured")
	}

	settings := controlplane.ResolveServiceTokenConfig(cfg)
	if missing := settings.MissingRequiredFields(); len(missing) > 0 {
		return nil, fmt.Errorf("service token configuration missing required keys: %s", strings.Join(missing, ", "))
	}
	if len(audience) > 0 {
		settings.Audience = audience
	}

	mtlsOpts, err := controlPlaneMTLSOptions(log, settings.MTLS)
	if err != nil {
		return nil, err
	}

	baseClient := NewClient(mtlsOpts...)
	tokenProvider := NewServiceTokenProvider(ServiceTokenProviderConfig{
		ServiceURL: settings.BaseURL,
		ServiceID:  settings.ServiceID,
		APIKey:     settings.APIKey,
		Scope:      settings.Scope,
		Audience:   settings.Audience,
		HTTPClient: baseClient,
	})

	clientOpts := append([]ClientOption{}, mtlsOpts...)
	clientOpts = append(clientOpts, WithTokenProvider(tokenProvider, 1*time.Minute))
	return NewClient(clientOpts...), nil
}

func controlPlaneMTLSOptions(log logger.LogManager, mtls controlplane.MTLSConfig) ([]ClientOption, error) {
	if missing := mtls.MissingRequiredFields(); len(missing) > 0 {
		return nil, fmt.Errorf("control-plane mTLS configuration missing required keys: %s", strings.Join(missing, ", "))
	}

	return []ClientOption{
		WithLogger(log),
		WithMTLS(mtls.CertFile, mtls.KeyFile, mtls.CAFile),
	}, nil
}
