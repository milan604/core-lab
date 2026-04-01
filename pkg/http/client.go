package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/milan604/core-lab/pkg/logger"
	"github.com/sony/gobreaker/v2"
)

// Client is an HTTP client with automatic token management, retry logic, and circuit breaker.
type Client struct {
	httpClient     *http.Client
	tokenCache     *TokenCache
	logger         logger.LogManager
	retryMax       int
	retryDelay     time.Duration
	requestHooks   []RequestHook
	responseHooks  []ResponseHook
	circuitBreaker *gobreaker.CircuitBreaker[*http.Response]
}

// RequestHook is a function that can modify a request before it's sent.
type RequestHook func(*http.Request) error

// ResponseHook is a function that can process a response after it's received.
type ResponseHook func(*http.Response) error

// ClientOption configures the HTTP client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(c *http.Client) ClientOption {
	return func(cl *Client) {
		cl.httpClient = c
	}
}

// WithTokenProvider sets the token provider for service authentication.
func WithTokenProvider(provider TokenProvider, refreshBuffer time.Duration) ClientOption {
	return func(c *Client) {
		c.tokenCache = NewTokenCache(provider, refreshBuffer)
	}
}

// WithLogger sets a logger for the client.
func WithLogger(l logger.LogManager) ClientOption {
	return func(c *Client) {
		c.logger = l
	}
}

// WithRetry configures retry behavior for failed requests.
// maxAttempts is the maximum number of attempts (including the first).
// delay is the initial delay between retries (will be exponential backoff).
func WithRetry(maxAttempts int, delay time.Duration) ClientOption {
	return func(c *Client) {
		if maxAttempts <= 0 {
			maxAttempts = 1
		}
		if delay <= 0 {
			delay = 100 * time.Millisecond
		}
		c.retryMax = maxAttempts
		c.retryDelay = delay
	}
}

// WithCircuitBreaker enables a circuit breaker for the client.
// The breaker opens after consecutive failures (5xx, timeouts, connection errors)
// and short-circuits requests, returning an error immediately until the breaker
// transitions to half-open and a probe request succeeds.
//
// Default settings if nil is passed: name="http-client", MaxRequests=5 (half-open),
// Interval=60s (closed-state counter reset), Timeout=30s (open→half-open).
func WithCircuitBreaker(settings *gobreaker.Settings) ClientOption {
	return func(c *Client) {
		if settings == nil {
			settings = &gobreaker.Settings{
				Name:        "http-client",
				MaxRequests: 5,
				Interval:    60 * time.Second,
				Timeout:     30 * time.Second,
				ReadyToTrip: func(counts gobreaker.Counts) bool {
					// Open the circuit after 5 consecutive failures.
					return counts.ConsecutiveFailures >= 5
				},
			}
		}
		c.circuitBreaker = gobreaker.NewCircuitBreaker[*http.Response](*settings)
	}
}

// WithRequestHook adds a hook that runs before each request.
func WithRequestHook(hook RequestHook) ClientOption {
	return func(c *Client) {
		c.requestHooks = append(c.requestHooks, hook)
	}
}

// WithMTLS configures mutual TLS on the HTTP client.
// certFile/keyFile are the client certificate and key.
// caFile is the CA certificate used to verify the server (optional — if empty,
// the system root CAs are used).
func WithMTLS(certFile, keyFile, caFile string) ClientOption {
	return func(c *Client) {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			if c.logger != nil {
				c.logger.ErrorF("mTLS: failed to load client cert/key: %v", err)
			}
			return
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		if caFile != "" {
			caCert, err := os.ReadFile(caFile)
			if err != nil {
				if c.logger != nil {
					c.logger.ErrorF("mTLS: failed to read CA file: %v", err)
				}
				return
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caCert) {
				if c.logger != nil {
					c.logger.ErrorF("mTLS: failed to parse CA certificate")
				}
				return
			}
			tlsCfg.RootCAs = caPool
		}
		c.httpClient.Transport = &http.Transport{TLSClientConfig: tlsCfg}
	}
}

// WithResponseHook adds a hook that runs after each response.
func WithResponseHook(hook ResponseHook) ClientOption {
	return func(c *Client) {
		c.responseHooks = append(c.responseHooks, hook)
	}
}

// NewClient creates a new HTTP client with the given options.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryMax:   3,
		retryDelay: 100 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Do executes an HTTP request with automatic token injection and retry logic.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.prepareRequest(ctx, req); err != nil {
		return nil, err
	}

	bodyBytes, err := c.readRequestBody(req)
	if err != nil {
		return nil, err
	}

	return c.executeWithRetry(ctx, req, bodyBytes)
}

// prepareRequest applies request hooks, token injection, and request ID propagation.
func (c *Client) prepareRequest(ctx context.Context, req *http.Request) error {
	if err := c.applyRequestHooks(req); err != nil {
		return err
	}

	// Propagate X-Request-ID from context to outgoing request for distributed tracing.
	if req.Header.Get("X-Request-ID") == "" {
		if rid, ok := ctx.Value(logger.RequestIDKey).(string); ok && rid != "" {
			req.Header.Set("X-Request-ID", rid)
		}
	}

	return c.injectToken(ctx, req)
}

// applyRequestHooks applies all request hooks.
func (c *Client) applyRequestHooks(req *http.Request) error {
	for _, hook := range c.requestHooks {
		if err := hook(req); err != nil {
			return fmt.Errorf("request hook failed: %w", err)
		}
	}
	return nil
}

// injectToken injects the authorization token if token cache is available.
func (c *Client) injectToken(ctx context.Context, req *http.Request) error {
	if c.tokenCache == nil {
		return nil
	}

	token, err := c.tokenCache.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// readRequestBody reads the request body once for retries.
func (c *Client) readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return bodyBytes, nil
}

// executeWithRetry executes the request with retry logic.
func (c *Client) executeWithRetry(ctx context.Context, req *http.Request, bodyBytes []byte) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt < c.retryMax; attempt++ {
		if attempt > 0 {
			if err := c.waitForRetry(ctx, attempt); err != nil {
				return nil, err
			}
		}

		resp, err := c.executeRequest(ctx, req, bodyBytes, attempt)
		if err != nil {
			lastErr = err
			// Don't retry when the circuit breaker is open — fail fast.
			if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
				if c.logger != nil {
					c.logger.WarnF("circuit breaker open, failing fast: %v", err)
				}
				break
			}
			if c.logger != nil {
				c.logger.WarnF("request failed: %v (attempt %d/%d)", err, attempt+1, c.retryMax)
			}
			continue
		}

		if err := c.applyResponseHooks(resp); err != nil {
			resp.Body.Close()
			return nil, err
		}

		if c.shouldRetryOn401(resp, attempt) {
			resp.Body.Close()
			c.handle401()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.retryMax, lastErr)
}

// waitForRetry waits for the retry delay with exponential backoff.
func (c *Client) waitForRetry(ctx context.Context, attempt int) error {
	delay := c.retryDelay * time.Duration(1<<uint(attempt-1))
	if c.logger != nil {
		c.logger.DebugF("retrying request after %v (attempt %d/%d)", delay, attempt+1, c.retryMax)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// executeRequest executes a single request attempt, optionally through the circuit breaker.
func (c *Client) executeRequest(ctx context.Context, req *http.Request, bodyBytes []byte, attempt int) (*http.Response, error) {
	reqClone := req.Clone(ctx)
	if len(bodyBytes) > 0 {
		reqClone.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	if c.tokenCache != nil && attempt > 0 {
		token, err := c.tokenCache.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get token for retry: %w", err)
		}
		reqClone.Header.Set("Authorization", "Bearer "+token)
	}

	if c.circuitBreaker != nil {
		return c.circuitBreaker.Execute(func() (*http.Response, error) {
			resp, err := c.httpClient.Do(reqClone)
			if err != nil {
				return nil, err
			}
			// Treat server errors as circuit breaker failures so consecutive 5xx trips the breaker.
			if resp.StatusCode >= 500 {
				return resp, fmt.Errorf("server error: %d", resp.StatusCode)
			}
			return resp, nil
		})
	}

	return c.httpClient.Do(reqClone)
}

// IsCircuitOpen returns true if the circuit breaker is currently open.
// Callers can use this to fail fast before constructing a request.
func (c *Client) IsCircuitOpen() bool {
	if c.circuitBreaker == nil {
		return false
	}
	return c.circuitBreaker.State() == gobreaker.StateOpen
}

// applyResponseHooks applies all response hooks.
func (c *Client) applyResponseHooks(resp *http.Response) error {
	for _, hook := range c.responseHooks {
		if err := hook(resp); err != nil {
			return fmt.Errorf("response hook failed: %w", err)
		}
	}
	return nil
}

// shouldRetryOn401 checks if we should retry on 401.
func (c *Client) shouldRetryOn401(resp *http.Response, attempt int) bool {
	return resp.StatusCode == http.StatusUnauthorized && c.tokenCache != nil && attempt < c.retryMax-1
}

// handle401 handles a 401 response by invalidating the token cache.
func (c *Client) handle401() {
	if c.logger != nil {
		c.logger.InfoF("received 401, invalidating token and retrying")
	}
	c.tokenCache.Invalidate()
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req)
}

// Post performs a POST request with JSON body.
func (c *Client) Post(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	return c.postPutPatch(ctx, http.MethodPost, url, body)
}

// Put performs a PUT request with JSON body.
func (c *Client) Put(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	return c.postPutPatch(ctx, http.MethodPut, url, body)
}

// Patch performs a PATCH request with JSON body.
func (c *Client) Patch(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	return c.postPutPatch(ctx, http.MethodPatch, url, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req)
}

// postPutPatch is a helper for POST, PUT, and PATCH requests.
func (c *Client) postPutPatch(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.Do(ctx, req)
}

// DoJSON performs a request and unmarshals the JSON response.
func (c *Client) DoJSON(ctx context.Context, req *http.Request, v interface{}) error {
	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// GetJSON performs a GET request and unmarshals the JSON response.
func (c *Client) GetJSON(ctx context.Context, url string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return c.DoJSON(ctx, req, v)
}

// PostJSON performs a POST request with JSON body and unmarshals the JSON response.
func (c *Client) PostJSON(ctx context.Context, url string, body interface{}, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
	}

	return c.DoJSON(ctx, req, v)
}

// PutJSON performs a PUT request with a JSON body and unmarshals the JSON response.
func (c *Client) PutJSON(ctx context.Context, url string, body interface{}, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
	}

	return c.DoJSON(ctx, req, v)
}
