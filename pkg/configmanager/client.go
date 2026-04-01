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

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/controlplane"
	httplib "github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"
)

const (
	KeyControlPlaneBaseURL = controlplane.KeyBaseURL
	KeySentinelBaseURL     = controlplane.LegacyKeyBaseURL
)

type Client struct {
	baseURL    string
	httpClient *httplib.Client
	log        logger.LogManager
}

func NewClient(cfg *config.Config, log logger.LogManager) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config not configured")
	}
	base := controlplane.ResolveBaseURL(cfg)
	if base == "" {
		return nil, fmt.Errorf("%s or %s is required for config manager", KeyControlPlaneBaseURL, KeySentinelBaseURL)
	}

	httpClient, err := httplib.NewInternalControlPlaneClientForAudience(log, cfg, controlplane.ResolveConfigAudience(cfg))
	if err != nil {
		return nil, fmt.Errorf("control-plane machine identity requires %s/%s/%s plus mTLS client credentials: %w", controlplane.KeyBaseURL, controlplane.KeyServiceID, controlplane.KeyServiceAPIKey, err)
	}

	return &Client{
		baseURL:    base,
		httpClient: httpClient,
		log:        log,
	}, nil
}

type Namespace struct {
	ID                          string `json:"id"`
	ServiceID                   string `json:"service_id"`
	NamespaceKey                string `json:"namespace_key"`
	DisplayName                 string `json:"display_name"`
	Description                 string `json:"description"`
	DefaultDeliveryMode         string `json:"default_delivery_mode"`
	DefaultWatchMode            string `json:"default_watch_mode"`
	DefaultWatchIntervalSeconds int    `json:"default_watch_interval_seconds"`
	RequireApproval             bool   `json:"require_approval"`
	Active                      bool   `json:"active"`
	CreatedBy                   string `json:"created_by,omitempty"`
	CreatedAt                   string `json:"created_at,omitempty"`
	UpdatedAt                   string `json:"updated_at,omitempty"`
}

type NamespaceLayer struct {
	ID          string         `json:"id,omitempty"`
	ReleaseID   string         `json:"release_id,omitempty"`
	LayerName   string         `json:"layer_name"`
	Environment string         `json:"environment,omitempty"`
	TenantID    string         `json:"tenant_id,omitempty"`
	ProjectID   string         `json:"project_id,omitempty"`
	Priority    int            `json:"priority"`
	Payload     map[string]any `json:"payload"`
	CreatedAt   string         `json:"created_at,omitempty"`
	UpdatedAt   string         `json:"updated_at,omitempty"`
}

type NamespaceLayerInput struct {
	LayerName   string         `json:"layer_name"`
	Environment string         `json:"environment,omitempty"`
	TenantID    string         `json:"tenant_id,omitempty"`
	ProjectID   string         `json:"project_id,omitempty"`
	Priority    int            `json:"priority,omitempty"`
	Payload     map[string]any `json:"payload"`
}

type NamespacePublishRequest struct {
	ServiceID            string                `json:"service_id"`
	NamespaceKey         string                `json:"namespace_key"`
	DisplayName          string                `json:"display_name"`
	Description          string                `json:"description,omitempty"`
	Version              string                `json:"version,omitempty"`
	ApprovalRequired     *bool                 `json:"approval_required,omitempty"`
	DeliveryMode         string                `json:"delivery_mode,omitempty"`
	WatchMode            string                `json:"watch_mode,omitempty"`
	WatchIntervalSeconds int                   `json:"watch_interval_seconds,omitempty"`
	Layers               []NamespaceLayerInput `json:"layers"`
}

type NamespacePublishResponse struct {
	Namespace Namespace        `json:"namespace"`
	Release   NamespaceRelease `json:"release"`
}

type NamespaceRelease struct {
	ID                   string           `json:"id"`
	NamespaceID          string           `json:"namespace_id"`
	Version              string           `json:"version"`
	SchemaID             string           `json:"schema_id,omitempty"`
	Status               string           `json:"status"`
	Summary              string           `json:"summary,omitempty"`
	ApprovalRequired     bool             `json:"approval_required"`
	DeliveryMode         string           `json:"delivery_mode"`
	WatchMode            string           `json:"watch_mode"`
	WatchIntervalSeconds int              `json:"watch_interval_seconds"`
	ETag                 string           `json:"etag"`
	CreatedBy            string           `json:"created_by,omitempty"`
	ApprovedBy           string           `json:"approved_by,omitempty"`
	ApprovedAt           string           `json:"approved_at,omitempty"`
	RejectedBy           string           `json:"rejected_by,omitempty"`
	RejectedAt           string           `json:"rejected_at,omitempty"`
	RejectionReason      string           `json:"rejection_reason,omitempty"`
	CreatedAt            string           `json:"created_at,omitempty"`
	UpdatedAt            string           `json:"updated_at,omitempty"`
	Layers               []NamespaceLayer `json:"layers,omitempty"`
}

type NamespaceResolveRequest struct {
	NamespaceID  string `json:"namespace_id,omitempty"`
	ServiceID    string `json:"service_id,omitempty"`
	NamespaceKey string `json:"namespace_key,omitempty"`
	Version      string `json:"version,omitempty"`
	Environment  string `json:"environment,omitempty"`
	TenantID     string `json:"tenant_id,omitempty"`
	ProjectID    string `json:"project_id,omitempty"`
}

type NamespaceResolveResponse struct {
	Namespace            Namespace        `json:"namespace"`
	ReleaseID            string           `json:"release_id"`
	Version              string           `json:"version"`
	DeliveryMode         string           `json:"delivery_mode"`
	WatchMode            string           `json:"watch_mode"`
	WatchIntervalSeconds int              `json:"watch_interval_seconds"`
	ETag                 string           `json:"etag"`
	MatchedLayers        []NamespaceLayer `json:"matched_layers,omitempty"`
	Config               map[string]any   `json:"config"`
}

type NamespaceWatchRequest struct {
	NamespaceID  string
	ServiceID    string
	NamespaceKey string
}

type NamespaceWatchResponse struct {
	NamespaceID          string `json:"namespace_id"`
	ServiceID            string `json:"service_id"`
	NamespaceKey         string `json:"namespace_key"`
	ReleaseID            string `json:"release_id"`
	Version              string `json:"version"`
	Status               string `json:"status"`
	DeliveryMode         string `json:"delivery_mode"`
	WatchMode            string `json:"watch_mode"`
	WatchIntervalSeconds int    `json:"watch_interval_seconds"`
	ETag                 string `json:"etag"`
	UpdatedAt            string `json:"updated_at,omitempty"`
}

func (c *Client) PublishNamespaceRelease(ctx context.Context, req NamespacePublishRequest) (NamespacePublishResponse, error) {
	req.ServiceID = strings.TrimSpace(req.ServiceID)
	req.NamespaceKey = strings.TrimSpace(req.NamespaceKey)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Description = strings.TrimSpace(req.Description)
	req.Version = strings.TrimSpace(req.Version)
	req.DeliveryMode = strings.TrimSpace(req.DeliveryMode)
	req.WatchMode = strings.TrimSpace(req.WatchMode)

	if req.ServiceID == "" || req.NamespaceKey == "" || req.DisplayName == "" {
		return NamespacePublishResponse{}, fmt.Errorf("service_id, namespace_key, and display_name are required")
	}
	if len(req.Layers) == 0 {
		return NamespacePublishResponse{}, fmt.Errorf("at least one layer is required")
	}

	request, err := c.newJSONRequest(ctx, http.MethodPost, controlplane.API{BaseURL: c.baseURL}.ConfigPublishURL(), req)
	if err != nil {
		return NamespacePublishResponse{}, err
	}
	var out NamespacePublishResponse
	if err := c.doJSON(request, &out, http.StatusOK, http.StatusCreated); err != nil {
		return NamespacePublishResponse{}, fmt.Errorf("publish namespace release: %w", err)
	}
	return out, nil
}

func (c *Client) ResolveNamespace(ctx context.Context, req NamespaceResolveRequest) (NamespaceResolveResponse, error) {
	req.NamespaceID = strings.TrimSpace(req.NamespaceID)
	req.ServiceID = strings.TrimSpace(req.ServiceID)
	req.NamespaceKey = strings.TrimSpace(req.NamespaceKey)
	req.Version = strings.TrimSpace(req.Version)
	req.Environment = strings.TrimSpace(req.Environment)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.ProjectID = strings.TrimSpace(req.ProjectID)

	if req.NamespaceID == "" && (req.ServiceID == "" || req.NamespaceKey == "") {
		return NamespaceResolveResponse{}, fmt.Errorf("namespace_id or service_id + namespace_key is required")
	}

	request, err := c.newJSONRequest(ctx, http.MethodPost, controlplane.API{BaseURL: c.baseURL}.ConfigResolveURL(), req)
	if err != nil {
		return NamespaceResolveResponse{}, err
	}
	var out NamespaceResolveResponse
	if err := c.doJSON(request, &out, http.StatusOK); err != nil {
		return NamespaceResolveResponse{}, fmt.Errorf("resolve namespace config: %w", err)
	}
	if out.Config == nil {
		out.Config = make(map[string]any)
	}
	return out, nil
}

func (c *Client) GetNamespaceWatch(ctx context.Context, req NamespaceWatchRequest) (NamespaceWatchResponse, error) {
	req.NamespaceID = strings.TrimSpace(req.NamespaceID)
	req.ServiceID = strings.TrimSpace(req.ServiceID)
	req.NamespaceKey = strings.TrimSpace(req.NamespaceKey)

	if req.NamespaceID == "" && (req.ServiceID == "" || req.NamespaceKey == "") {
		return NamespaceWatchResponse{}, fmt.Errorf("namespace_id or service_id + namespace_key is required")
	}

	query := url.Values{}
	if req.NamespaceID != "" {
		query.Set("namespace_id", req.NamespaceID)
	}
	if req.ServiceID != "" {
		query.Set("service_id", req.ServiceID)
	}
	if req.NamespaceKey != "" {
		query.Set("namespace_key", req.NamespaceKey)
	}

	requestURL := controlplane.API{BaseURL: c.baseURL}.ConfigWatchURL()
	if encoded := query.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return NamespaceWatchResponse{}, err
	}
	var out NamespaceWatchResponse
	if err := c.doJSON(request, &out, http.StatusOK); err != nil {
		return NamespaceWatchResponse{}, fmt.Errorf("get namespace watch metadata: %w", err)
	}
	return out, nil
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
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) doJSON(req *http.Request, out any, allowedStatus ...int) error {
	resp, err := c.httpClient.Do(req.Context(), req)
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
