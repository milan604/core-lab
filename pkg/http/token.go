package http

import (
	"context"
	"sync"
	"time"
)

// TokenProvider defines the interface for fetching service tokens.
// Implementations should handle authentication with the token service.
type TokenProvider interface {
	// FetchToken retrieves a new token from the authentication service.
	// It should return the token string and its expiration time.
	// If the token doesn't have an explicit expiration, return a reasonable TTL.
	FetchToken(ctx context.Context) (token string, expiresAt time.Time, err error)
}

// TokenCache manages token storage with expiration handling.
type TokenCache struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
	provider  TokenProvider
	// refreshBuffer is the time before expiration to refresh the token
	refreshBuffer time.Duration
}

// NewTokenCache creates a new token cache with the given provider.
func NewTokenCache(provider TokenProvider, refreshBuffer time.Duration) *TokenCache {
	if refreshBuffer <= 0 {
		refreshBuffer = 30 * time.Second // default: refresh 30s before expiration
	}
	return &TokenCache{
		provider:      provider,
		refreshBuffer: refreshBuffer,
	}
}

// GetToken retrieves a valid token, fetching a new one if needed.
// It is thread-safe and handles token expiration automatically.
func (tc *TokenCache) GetToken(ctx context.Context) (string, error) {
	tc.mu.RLock()
	now := time.Now()
	// Check if we have a valid token that won't expire soon
	if tc.token != "" && now.Before(tc.expiresAt.Add(-tc.refreshBuffer)) {
		token := tc.token
		tc.mu.RUnlock()
		return token, nil
	}
	tc.mu.RUnlock()

	// Need to fetch a new token
	return tc.refreshToken(ctx)
}

// refreshToken fetches a new token and updates the cache.
func (tc *TokenCache) refreshToken(ctx context.Context) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Double-check: another goroutine might have refreshed it
	now := time.Now()
	if tc.token != "" && now.Before(tc.expiresAt.Add(-tc.refreshBuffer)) {
		return tc.token, nil
	}

	// Fetch new token
	token, expiresAt, err := tc.provider.FetchToken(ctx)
	if err != nil {
		return "", err
	}

	tc.token = token
	tc.expiresAt = expiresAt
	return token, nil
}

// Invalidate clears the cached token, forcing a refresh on next GetToken call.
func (tc *TokenCache) Invalidate() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.token = ""
	tc.expiresAt = time.Time{}
}

// IsValid checks if the current cached token is still valid.
func (tc *TokenCache) IsValid() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	now := time.Now()
	return tc.token != "" && now.Before(tc.expiresAt.Add(-tc.refreshBuffer))
}
