// Package configmanager provides a client for services to publish their config to Sentinel
// and to fetch other services' config (with optional version and retry).
package configmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/milan604/core-lab/pkg/config"
	corehttp "github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"
)

const (
	// Config key for Sentinel base URL (e.g. http://sentinel-nginx:4000/sentinel).
	KeySentinelBaseURL = "SentinelServiceEndpoint"
	// Config key for internal API auth (X-Internal-Key header).
	KeyInternalAdminKey = "InternalAdminKey"
)

// Client calls Sentinel's service config API (publish and fetch).
type Client struct {
	baseURL     string
	internalKey string
	httpClient  *http.Client
	log         logger.LogManager
}

// NewClient builds a client from config. Requires SentinelServiceEndpoint and InternalAdminKey.
func NewClient(cfg *config.Config, log logger.LogManager) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config not configured")
	}
	base := corehttp.NormalizeSentinelBaseURL(cfg.GetString(KeySentinelBaseURL))
	if base == "" {
		return nil, fmt.Errorf("%s is required for config manager", KeySentinelBaseURL)
	}
	key := strings.TrimSpace(cfg.GetString(KeyInternalAdminKey))
	if key == "" {
		return nil, fmt.Errorf("%s is required to publish/fetch config", KeyInternalAdminKey)
	}
	return &Client{
		baseURL:     base,
		internalKey: key,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		log:         log,
	}, nil
}

// ServiceConfig is the response shape for GET config. Version is semver (0.0.0).
type ServiceConfig struct {
	ServiceID string         `json:"service_id"`
	Version   string         `json:"version"`
	Payload   map[string]any `json:"payload"`
	CreatedAt string         `json:"created_at,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
}

// Publish updates the service's config in Sentinel (PATCH). Idempotent; use when service is ready.
func (c *Client) Publish(ctx context.Context, serviceID string, payload map[string]any) error {
	if err := validateServiceID(serviceID); err != nil {
		return err
	}
	if payload == nil {
		payload = make(map[string]any)
	}
	reqURL := c.serviceURL(serviceID, "/config")
	req, err := c.newJSONRequest(ctx, http.MethodPatch, reqURL, map[string]any{"payload": payload})
	if err != nil {
		return err
	}
	if err := c.doJSON(req, nil, http.StatusOK, http.StatusCreated); err != nil {
		return fmt.Errorf("publish config for %s: %w", serviceID, err)
	}
	if c.log != nil {
		c.log.InfoFCtx(ctx, "published config for service %s (version in response)", serviceID)
	}
	return nil
}

// Get fetches a service's config at the exact version (e.g. 1.0.0). Version is required; all parts (major.minor.patch) matter.
func (c *Client) Get(ctx context.Context, serviceID string, version string) (ServiceConfig, error) {
	if err := validateServiceID(serviceID); err != nil {
		return ServiceConfig{}, err
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return ServiceConfig{}, fmt.Errorf("version is required (e.g. 1.0.0)")
	}
	q := url.Values{}
	q.Set("version", version)
	reqURL := c.serviceURL(serviceID, "/config") + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return ServiceConfig{}, err
	}
	req.Header.Set("X-Internal-Key", c.internalKey)
	var out ServiceConfig
	if err := c.doJSON(req, &out, http.StatusOK); err != nil {
		return ServiceConfig{}, fmt.Errorf("get config for %s@%s: %w", serviceID, version, err)
	}
	if out.Payload == nil {
		out.Payload = make(map[string]any)
	}
	return out, nil
}

// GetWithRetry fetches config and retries with backoff until payload is non-empty or max attempts reached.
func (c *Client) GetWithRetry(ctx context.Context, serviceID string, version string, maxAttempts int, initialBackoff time.Duration) (ServiceConfig, error) {
	var lastErr error
	maxAttempts, backoff := normalizeRetry(maxAttempts, initialBackoff)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		cfg, err := c.Get(ctx, serviceID, version)
		if err != nil {
			lastErr = err
			if c.log != nil {
				c.log.WarnFCtx(ctx, "config get %s attempt %d: %v", serviceID, attempt+1, err)
			}
			if attempt < maxAttempts-1 {
				select {
				case <-ctx.Done():
					return ServiceConfig{}, ctx.Err()
				case <-time.After(backoff):
					backoff *= 2
				}
			}
			continue
		}
		// Consider "empty" if payload has no endpoint_url (or is empty map)
		if len(cfg.Payload) > 0 {
			return cfg, nil
		}
		lastErr = fmt.Errorf("config payload empty")
		if attempt < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return ServiceConfig{}, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}
	return ServiceConfig{}, lastErr
}

// SetParameters stores parameter name->value for a service (PATCH .../parameters). Values live in DB; config payload references param names.
func (c *Client) SetParameters(ctx context.Context, serviceID string, params map[string]string) error {
	if err := validateServiceID(serviceID); err != nil {
		return err
	}
	if len(params) == 0 {
		return nil
	}
	reqURL := c.serviceURL(serviceID, "/parameters")
	req, err := c.newJSONRequest(ctx, http.MethodPatch, reqURL, map[string]any{"parameters": params})
	if err != nil {
		return err
	}
	if err := c.doJSON(req, nil, http.StatusOK); err != nil {
		return fmt.Errorf("set parameters for %s: %w", serviceID, err)
	}
	return nil
}

// GetParameters returns parameter name->value for a service. Names optional (empty = all).
func (c *Client) GetParameters(ctx context.Context, serviceID string, names []string) (map[string]string, error) {
	if err := validateServiceID(serviceID); err != nil {
		return nil, err
	}
	reqURL := c.serviceURL(serviceID, "/parameters")
	if len(names) > 0 {
		cleanNames := make([]string, 0, len(names))
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name != "" {
				cleanNames = append(cleanNames, name)
			}
		}
		if len(cleanNames) > 0 {
			q := url.Values{}
			q.Set("names", strings.Join(cleanNames, ","))
			reqURL += "?" + q.Encode()
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Key", c.internalKey)
	var out struct {
		Parameters map[string]string `json:"parameters"`
	}
	if err := c.doJSON(req, &out, http.StatusOK); err != nil {
		return nil, fmt.Errorf("get parameters for %s: %w", serviceID, err)
	}
	if out.Parameters == nil {
		out.Parameters = make(map[string]string)
	}
	return out.Parameters, nil
}

// ResolvedConfig is key->value after resolving config (key->param name) with parameters (param name->value). Version is semver (0.0.0).
type ResolvedConfig struct {
	ServiceID string            `json:"service_id"`
	Version   string            `json:"version"`
	Config    map[string]string `json:"config"`
}

// GetResolvedConfig returns config key->value at the exact version (e.g. 1.0.0). Version is required; all parts matter.
func (c *Client) GetResolvedConfig(ctx context.Context, serviceID string, version string) (ResolvedConfig, error) {
	if err := validateServiceID(serviceID); err != nil {
		return ResolvedConfig{}, err
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return ResolvedConfig{}, fmt.Errorf("version is required (e.g. 1.0.0)")
	}
	q := url.Values{}
	q.Set("version", version)
	reqURL := c.serviceURL(serviceID, "/config/resolved") + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return ResolvedConfig{}, err
	}
	req.Header.Set("X-Internal-Key", c.internalKey)
	var out ResolvedConfig
	if err := c.doJSON(req, &out, http.StatusOK); err != nil {
		return ResolvedConfig{}, fmt.Errorf("get resolved config for %s@%s: %w", serviceID, version, err)
	}
	if out.Config == nil {
		out.Config = make(map[string]string)
	}
	return out, nil
}

// GetResolvedWithRetry fetches resolved config with retry until config is non-empty or max attempts reached.
func (c *Client) GetResolvedWithRetry(ctx context.Context, serviceID string, version string, maxAttempts int, initialBackoff time.Duration) (ResolvedConfig, error) {
	var lastErr error
	maxAttempts, backoff := normalizeRetry(maxAttempts, initialBackoff)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		res, err := c.GetResolvedConfig(ctx, serviceID, version)
		if err != nil {
			lastErr = err
			if c.log != nil {
				c.log.WarnFCtx(ctx, "resolved config get %s attempt %d: %v", serviceID, attempt+1, err)
			}
			if attempt < maxAttempts-1 {
				select {
				case <-ctx.Done():
					return ResolvedConfig{}, ctx.Err()
				case <-time.After(backoff):
					backoff *= 2
				}
			}
			continue
		}
		if len(res.Config) > 0 {
			return res, nil
		}
		lastErr = fmt.Errorf("resolved config empty")
		if attempt < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return ResolvedConfig{}, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}
	return ResolvedConfig{}, lastErr
}

func (c *Client) serviceURL(serviceID, suffix string) string {
	return c.baseURL + "/internal/api/v1/services/" + url.PathEscape(strings.TrimSpace(serviceID)) + suffix
}

func (c *Client) newJSONRequest(ctx context.Context, method, reqURL string, body any) (*http.Request, error) {
	var reader *bytes.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(bodyBytes)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Key", c.internalKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) doJSON(req *http.Request, out any, allowedStatus ...int) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !statusAllowed(resp.StatusCode, allowedStatus) {
		return readStatusError(resp)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func statusAllowed(got int, allowed []int) bool {
	for _, code := range allowed {
		if got == code {
			return true
		}
	}
	return false
}

func readStatusError(resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	body := strings.TrimSpace(string(bodyBytes))
	if body == "" {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}
	return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
}

func validateServiceID(serviceID string) error {
	if strings.TrimSpace(serviceID) == "" {
		return fmt.Errorf("serviceID is required")
	}
	return nil
}

func normalizeRetry(maxAttempts int, initialBackoff time.Duration) (int, time.Duration) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if initialBackoff <= 0 {
		initialBackoff = 200 * time.Millisecond
	}
	return maxAttempts, initialBackoff
}
