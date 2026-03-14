package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	configpkg "github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/controlplane"
	httplib "github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"
)

const permissionDecisionRequestBodyKey = "auth_permission_decision_body"

type permissionDecisionRequest struct {
	SubjectUserID string            `json:"subject_user_id"`
	TenantID      string            `json:"tenant_id,omitempty"`
	Service       string            `json:"service"`
	Category      string            `json:"category"`
	Action        string            `json:"action"`
	ResourceType  string            `json:"resource_type,omitempty"`
	ResourceID    string            `json:"resource_id,omitempty"`
	Context       map[string]string `json:"context,omitempty"`
}

type permissionDecisionResponse struct {
	Allowed bool     `json:"allowed"`
	Reasons []string `json:"reasons,omitempty"`
}

type permissionDecisionClient interface {
	Decide(ctx context.Context, input permissionDecisionRequest) (permissionDecisionResponse, error)
}

type httpPermissionDecisionClient struct {
	baseURL    string
	httpClient *httplib.Client
	initErr    error
}

var newPermissionDecisionClientFunc = func(cfg Config, log logger.LogManager) permissionDecisionClient {
	return newPermissionDecisionClient(cfg, log)
}

func newPermissionDecisionClient(cfg Config, log logger.LogManager) permissionDecisionClient {
	if cfg == nil {
		return nil
	}

	realCfg, ok := cfg.(*configpkg.Config)
	if !ok {
		return nil
	}

	baseURL := controlplane.ResolveBaseURL(realCfg)
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}

	client, err := httplib.NewInternalControlPlaneClientForAudience(log, realCfg, controlplane.ResolveAuthorizationAudience(realCfg))
	return &httpPermissionDecisionClient{
		baseURL:    baseURL,
		httpClient: client,
		initErr:    err,
	}
}

func (c *httpPermissionDecisionClient) Decide(ctx context.Context, input permissionDecisionRequest) (permissionDecisionResponse, error) {
	if c == nil {
		return permissionDecisionResponse{}, fmt.Errorf("permission decision client is nil")
	}
	if strings.TrimSpace(c.baseURL) == "" {
		return permissionDecisionResponse{}, fmt.Errorf("%s or %s not configured", controlplane.KeyBaseURL, controlplane.LegacyKeyBaseURL)
	}
	if c.initErr != nil {
		return permissionDecisionResponse{}, fmt.Errorf("authorization control-plane client initialization failed: %w", c.initErr)
	}
	if c.httpClient == nil {
		return permissionDecisionResponse{}, fmt.Errorf("authorization control-plane client is not configured")
	}
	if strings.TrimSpace(input.SubjectUserID) == "" {
		return permissionDecisionResponse{}, fmt.Errorf("subject_user_id is required")
	}
	if strings.TrimSpace(input.Service) == "" || strings.TrimSpace(input.Category) == "" || strings.TrimSpace(input.Action) == "" {
		return permissionDecisionResponse{}, fmt.Errorf("service, category, and action are required")
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(input); err != nil {
		return permissionDecisionResponse{}, fmt.Errorf("encode permission decision request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controlplane.API{BaseURL: c.baseURL}.AuthorizationDecisionURL(), &body)
	if err != nil {
		return permissionDecisionResponse{}, fmt.Errorf("create permission decision request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(ctx, req)
	if err != nil {
		return permissionDecisionResponse{}, err
	}
	defer resp.Body.Close()

	var out permissionDecisionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil && err != io.EOF {
		return permissionDecisionResponse{}, fmt.Errorf("decode permission decision response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return out, fmt.Errorf("permission decision endpoint returned status %d", resp.StatusCode)
	}

	return out, nil
}

func buildPermissionDecisionRequest(c *gin.Context, claims Claims, code string) (permissionDecisionRequest, error) {
	service, category, action, ok := parsePermissionCode(code)
	if !ok {
		return permissionDecisionRequest{}, fmt.Errorf("invalid permission code: %s", code)
	}

	subjectUserID := strings.TrimSpace(claims.UserID())
	if subjectUserID == "" {
		return permissionDecisionRequest{}, fmt.Errorf("subject_user_id is required")
	}

	tenantID := resolvePermissionDecisionTenantID(c, claims)
	resourceID := resolvePermissionDecisionResourceID(c, category)
	resourceType := inferPermissionDecisionResourceType(category, action, resourceID)
	contextMap := collectPermissionDecisionContext(c)

	return permissionDecisionRequest{
		SubjectUserID: subjectUserID,
		TenantID:      tenantID,
		Service:       service,
		Category:      category,
		Action:        action,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		Context:       contextMap,
	}, nil
}

func parsePermissionCode(code string) (service string, category string, action string, ok bool) {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(code)), "-", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	service = strings.TrimSpace(parts[0])
	category = strings.TrimSpace(parts[1])
	action = strings.TrimSpace(parts[2])
	if service == "" || category == "" || action == "" {
		return "", "", "", false
	}
	return service, category, action, true
}

func resolvePermissionDecisionTenantID(c *gin.Context, claims Claims) string {
	if c == nil {
		return strings.TrimSpace(claims.TenantID())
	}
	if c.Request != nil {
		if tenantID, ok := TenantIDFromContext(c.Request.Context()); ok {
			return tenantID
		}
	}
	if tenantID, ok := GetTenantID(c); ok {
		return tenantID
	}
	if tenantID := strings.TrimSpace(c.Param("tenant_id")); tenantID != "" {
		return tenantID
	}
	if tenantID := strings.TrimSpace(c.Query("tenant_id")); tenantID != "" {
		return tenantID
	}
	if values := collectPermissionDecisionContext(c); len(values) > 0 {
		if tenantID := strings.TrimSpace(values["tenant_id"]); tenantID != "" {
			return tenantID
		}
	}
	return strings.TrimSpace(claims.TenantID())
}

func resolvePermissionDecisionResourceID(c *gin.Context, category string) string {
	if c == nil {
		return ""
	}

	params := c.Params
	if len(params) == 0 {
		return ""
	}

	primaryKeys := map[string]string{
		"tenants":       "tenant_id",
		"users":         "user_id",
		"account_users": "user_id",
		"system_users":  "user_id",
	}
	if key := strings.TrimSpace(primaryKeys[strings.TrimSpace(category)]); key != "" {
		if value := strings.TrimSpace(c.Param(key)); value != "" {
			return value
		}
	}

	for _, param := range params {
		key := strings.TrimSpace(param.Key)
		value := strings.TrimSpace(param.Value)
		if value == "" || !strings.HasSuffix(key, "_id") {
			continue
		}
		if key == "tenant_id" || key == "user_id" {
			continue
		}
		return value
	}

	for _, param := range params {
		value := strings.TrimSpace(param.Value)
		if value == "" {
			continue
		}
		if strings.HasSuffix(strings.TrimSpace(param.Key), "_id") {
			return value
		}
	}

	return ""
}

func inferPermissionDecisionResourceType(category string, action string, resourceID string) string {
	category = strings.TrimSpace(category)
	action = strings.TrimSpace(action)
	if category == "" {
		return ""
	}

	switch action {
	case "authentication":
		return "user_session"
	case "registration":
		return "user_registration"
	case "switch-tenant":
		return "tenant_membership"
	case "list", "create", "tenant-list":
		return category + "_collection"
	}

	if strings.TrimSpace(resourceID) == "" {
		return category + "_collection"
	}
	return singularizeResourceType(category)
}

func singularizeResourceType(category string) string {
	switch {
	case strings.HasSuffix(category, "ies") && len(category) > 3:
		return category[:len(category)-3] + "y"
	case strings.HasSuffix(category, "sses") && len(category) > 4:
		return category[:len(category)-2]
	case strings.HasSuffix(category, "ses") && len(category) > 3:
		return category[:len(category)-2]
	case strings.HasSuffix(category, "s") && len(category) > 1 && !strings.HasSuffix(category, "ss"):
		return category[:len(category)-1]
	default:
		return category
	}
}

func collectPermissionDecisionContext(c *gin.Context) map[string]string {
	if c == nil {
		return nil
	}

	values := make(map[string]string)
	for _, param := range c.Params {
		if key, value, ok := normalizeContextEntry(param.Key, param.Value); ok {
			values[key] = value
		}
	}
	if c.Request != nil && c.Request.URL != nil {
		for key, entries := range c.Request.URL.Query() {
			if len(entries) == 0 {
				continue
			}
			if key, value, ok := normalizeContextEntry(key, entries[0]); ok {
				values[key] = value
			}
		}
	}

	if body := requestJSONContext(c); len(body) > 0 {
		for key, value := range body {
			values[key] = value
		}
	}

	if len(values) == 0 {
		return nil
	}
	return values
}

func requestJSONContext(c *gin.Context) map[string]string {
	body, ok := requestBodyBytes(c)
	if !ok || len(body) == 0 {
		return nil
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(trimmed, &payload); err != nil || len(payload) == 0 {
		return nil
	}

	values := make(map[string]string)
	flattenJSONContext("", payload, values)
	if len(values) == 0 {
		return nil
	}
	return values
}

func flattenJSONContext(prefix string, input map[string]any, out map[string]string) {
	for key, value := range input {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch typed := value.(type) {
		case map[string]any:
			flattenJSONContext(fullKey, typed, out)
		default:
			if stringValue, ok := stringifyPermissionDecisionValue(typed); ok {
				out[fullKey] = stringValue
			}
		}
	}
}

func stringifyPermissionDecisionValue(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", false
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed, trimmed != ""
	case bool:
		return strconv.FormatBool(typed), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case json.Number:
		trimmed := strings.TrimSpace(typed.String())
		return trimmed, trimmed != ""
	default:
		return "", false
	}
}

func requestBodyBytes(c *gin.Context) ([]byte, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, false
	}
	if cached, exists := c.Get(permissionDecisionRequestBodyKey); exists {
		if payload, ok := cached.([]byte); ok {
			return append([]byte(nil), payload...), true
		}
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(payload))
	c.Set(permissionDecisionRequestBodyKey, append([]byte(nil), payload...))
	return payload, true
}

func normalizeContextEntry(key string, value string) (string, string, bool) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}
