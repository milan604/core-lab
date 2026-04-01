package authz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/controlplane"
	httplib "github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"
)

type DecisionRequest struct {
	SubjectUserID string            `json:"subject_user_id"`
	TenantID      string            `json:"tenant_id,omitempty"`
	Service       string            `json:"service"`
	Category      string            `json:"category"`
	Action        string            `json:"action"`
	ResourceType  string            `json:"resource_type,omitempty"`
	ResourceID    string            `json:"resource_id,omitempty"`
	Context       map[string]string `json:"context,omitempty"`
}

type DecisionResponse struct {
	Allowed        bool     `json:"allowed"`
	SubjectUserID  string   `json:"subject_user_id"`
	Service        string   `json:"service"`
	PermissionCode string   `json:"permission_code"`
	TenantID       string   `json:"tenant_id,omitempty"`
	ResourceType   string   `json:"resource_type,omitempty"`
	ResourceID     string   `json:"resource_id,omitempty"`
	RoleIDs        []string `json:"role_ids,omitempty"`
	GroupIDs       []string `json:"group_ids,omitempty"`
	AllowPolicyIDs []string `json:"allow_policy_ids,omitempty"`
	DenyPolicyIDs  []string `json:"deny_policy_ids,omitempty"`
	DelegationIDs  []string `json:"delegation_ids,omitempty"`
	Reasons        []string `json:"reasons,omitempty"`
	CacheHit       bool     `json:"cache_hit"`
}

type Client struct {
	log        logger.LogManager
	httpClient *httplib.Client
	baseURL    string
	initErr    error
}

func NewClient(log logger.LogManager, cfg *config.Config) *Client {
	api := controlplane.APIFromConfig(cfg)
	httpClient, err := httplib.NewInternalControlPlaneClientForAudience(log, cfg, controlplane.ResolveAuthorizationAudience(cfg))
	return &Client{
		log:        log,
		httpClient: httpClient,
		baseURL:    api.BaseURL,
		initErr:    err,
	}
}

func (c *Client) Decide(ctx context.Context, input DecisionRequest) (DecisionResponse, error) {
	if c == nil {
		return DecisionResponse{}, fmt.Errorf("authorization client is nil")
	}
	if strings.TrimSpace(c.baseURL) == "" {
		return DecisionResponse{}, fmt.Errorf("%s or %s not configured", controlplane.KeyBaseURL, controlplane.LegacyKeyBaseURL)
	}
	if c.initErr != nil {
		return DecisionResponse{}, fmt.Errorf("control-plane client initialization failed: %w", c.initErr)
	}
	if c.httpClient == nil {
		return DecisionResponse{}, fmt.Errorf("control-plane client is not configured")
	}
	if err := validateDecisionRequest(input); err != nil {
		return DecisionResponse{}, err
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(input); err != nil {
		return DecisionResponse{}, fmt.Errorf("encode authorization decision request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controlplane.API{BaseURL: c.baseURL}.AuthorizationDecisionURL(), &body)
	if err != nil {
		return DecisionResponse{}, fmt.Errorf("create authorization decision request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(ctx, req)
	if err != nil {
		return DecisionResponse{}, err
	}
	defer resp.Body.Close()

	var out DecisionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil && err != io.EOF {
		return DecisionResponse{}, fmt.Errorf("decode authorization decision response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return out, fmt.Errorf("authorization decision endpoint returned status %d", resp.StatusCode)
	}

	return out, nil
}

func validateDecisionRequest(input DecisionRequest) error {
	if strings.TrimSpace(input.SubjectUserID) == "" {
		return fmt.Errorf("subject_user_id is required")
	}
	if strings.TrimSpace(input.Service) == "" {
		return fmt.Errorf("service is required")
	}
	if strings.TrimSpace(input.Category) == "" {
		return fmt.Errorf("category is required")
	}
	if strings.TrimSpace(input.Action) == "" {
		return fmt.Errorf("action is required")
	}
	return nil
}
