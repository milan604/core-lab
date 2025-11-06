package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuth2ClientCredentialsProvider implements TokenProvider for OAuth2 client credentials flow.
// This is a common pattern for service-to-service authentication.
type OAuth2ClientCredentialsProvider struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scope        string
	HTTPClient   *http.Client
}

// NewOAuth2ClientCredentialsProvider creates a new OAuth2 client credentials token provider.
func NewOAuth2ClientCredentialsProvider(tokenURL, clientID, clientSecret, scope string) *OAuth2ClientCredentialsProvider {
	return &OAuth2ClientCredentialsProvider{
		TokenURL:     tokenURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        scope,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchToken retrieves a token using OAuth2 client credentials flow.
func (p *OAuth2ClientCredentialsProvider) FetchToken(ctx context.Context) (string, time.Time, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)
	if p.Scope != "" {
		data.Set("scope", p.Scope)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to fetch token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"` // seconds until expiration
		Scope       string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("empty access token in response")
	}

	// Calculate expiration time
	expiresAt := time.Now()
	if tokenResp.ExpiresIn > 0 {
		// Subtract 10 seconds as a safety margin
		expiresAt = expiresAt.Add(time.Duration(tokenResp.ExpiresIn-10) * time.Second)
	} else {
		// Default to 1 hour if no expiration provided
		expiresAt = expiresAt.Add(1 * time.Hour)
	}

	return tokenResp.AccessToken, expiresAt, nil
}

// StaticTokenProvider provides a static token that never expires.
// Useful for testing or when tokens are managed externally.
type StaticTokenProvider struct {
	Token string
}

// NewStaticTokenProvider creates a new static token provider.
func NewStaticTokenProvider(token string) *StaticTokenProvider {
	return &StaticTokenProvider{Token: token}
}

// FetchToken returns the static token with a far-future expiration.
func (p *StaticTokenProvider) FetchToken(ctx context.Context) (string, time.Time, error) {
	return p.Token, time.Now().Add(24 * 365 * time.Hour), nil // 1 year from now
}

// CustomTokenProvider allows you to provide a custom function for fetching tokens.
type CustomTokenProvider struct {
	FetchFunc func(ctx context.Context) (token string, expiresAt time.Time, err error)
}

// NewCustomTokenProvider creates a new custom token provider.
func NewCustomTokenProvider(fetchFunc func(ctx context.Context) (string, time.Time, error)) *CustomTokenProvider {
	return &CustomTokenProvider{FetchFunc: fetchFunc}
}

// FetchToken calls the custom fetch function.
func (p *CustomTokenProvider) FetchToken(ctx context.Context) (string, time.Time, error) {
	if p.FetchFunc == nil {
		return "", time.Time{}, fmt.Errorf("fetch function is nil")
	}
	return p.FetchFunc(ctx)
}

