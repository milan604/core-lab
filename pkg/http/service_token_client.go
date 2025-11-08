package http

import (
	"context"
	"fmt"
	"time"

	"github.com/milan604/core-lab/pkg/config"
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
	if len(cfg.Audience) == 0 {
		cfg.Audience = []string{"sentinel"}
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

	req := struct {
		ServiceID string   `json:"service_id"`
		APIKey    string   `json:"api_key"`
		Scope     string   `json:"scope"`
		Audience  []string `json:"audience"`
	}{
		ServiceID: p.ServiceID,
		APIKey:    p.APIKey,
		Scope:     p.Scope,
		Audience:  p.Audience,
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
	if err := p.HTTPClient.PostJSON(ctx, url, req, &resp); err != nil {
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
// - SentinelServiceEndpoint: URL of the sentinel service
// - SentinelServiceID: Service ID for authentication
// - SentinelServiceAPIKey: API key for authentication
func NewClientWithServiceToken(log logger.LogManager, cfg *config.Config) (*Client, error) {
	// Get configuration
	sentinelServiceURL := cfg.GetString("SentinelServiceEndpoint")
	serviceID := cfg.GetString("SentinelServiceID")
	apiKey := cfg.GetString("SentinelServiceAPIKey")

	// Validate configuration
	if err := cfg.ValidateRequired("SentinelServiceEndpoint", "SentinelServiceID", "SentinelServiceAPIKey"); err != nil {
		return nil, fmt.Errorf("service token configuration: %w", err)
	}

	// Create base HTTP client for calling service token API
	// This client doesn't need token provider - it's used to get the token
	baseClient := NewClient(WithLogger(log))

	// Create service token provider
	tokenProvider := NewServiceTokenProvider(ServiceTokenProviderConfig{
		ServiceURL: sentinelServiceURL,
		ServiceID:  serviceID,
		APIKey:     apiKey,
		HTTPClient: baseClient,
	})

	// Create HTTP client with token provider configured
	return NewClient(
		WithLogger(log),
		WithTokenProvider(tokenProvider, 1*time.Minute), // Refresh buffer of 1 minute
	), nil
}
