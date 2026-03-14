package authz

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/milan604/core-lab/pkg/auth"
	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/controlplane"
	"github.com/milan604/core-lab/pkg/logger"
)

type contextKey string

const (
	ctxDecisionKey    contextKey = "authz_decision"
	ctxRequestBodyKey contextKey = "authz_request_body"
	ctxRequestJSONKey contextKey = "authz_request_json"
)

type StringResolver func(*gin.Context, auth.Claims) string
type ContextResolver func(*gin.Context, auth.Claims) map[string]string

type MiddlewareConfig struct {
	Enabled               bool
	Service               string
	Category              string
	Action                string
	ResourceType          string
	FailOpen              bool
	AllowServiceTokens    bool
	RequireTenant         bool
	CacheTTL              time.Duration
	SubjectUserIDResolver StringResolver
	TenantIDResolver      StringResolver
	CategoryResolver      StringResolver
	ActionResolver        StringResolver
	ResourceTypeResolver  StringResolver
	ResourceIDResolver    StringResolver
	ContextResolver       ContextResolver
}

var newAuthzClientFunc = NewClient

func NewMiddlewareConfig(cfg *config.Config) MiddlewareConfig {
	cacheTTLSeconds := 60
	if cfg != nil {
		cacheTTLSeconds = cfg.GetIntD("AuthorizationCacheTTLSeconds", 60)
	}
	if cacheTTLSeconds < 0 {
		cacheTTLSeconds = 0
	}

	return MiddlewareConfig{
		Enabled:               cfg == nil || cfg.GetBoolD("AuthorizationEnabled", true),
		Service:               controlplane.ResolveServiceID(cfg),
		FailOpen:              cfg != nil && cfg.GetBoolD("AuthorizationFailOpen", false),
		AllowServiceTokens:    true,
		RequireTenant:         false,
		CacheTTL:              time.Duration(cacheTTLSeconds) * time.Second,
		SubjectUserIDResolver: SubjectFromClaims,
		TenantIDResolver:      TenantFromContextOrClaims,
	}
}

func (cfg MiddlewareConfig) WithService(service string) MiddlewareConfig {
	cfg.Service = strings.TrimSpace(service)
	return cfg
}

func (cfg MiddlewareConfig) WithCategory(category string) MiddlewareConfig {
	cfg.Category = strings.TrimSpace(category)
	return cfg
}

func (cfg MiddlewareConfig) WithAction(action string) MiddlewareConfig {
	cfg.Action = strings.TrimSpace(action)
	return cfg
}

func (cfg MiddlewareConfig) WithActionFromMethod() MiddlewareConfig {
	cfg.ActionResolver = MethodActionResolver(nil)
	return cfg
}

func (cfg MiddlewareConfig) WithResourceType(resourceType string) MiddlewareConfig {
	cfg.ResourceType = strings.TrimSpace(resourceType)
	return cfg
}

func (cfg MiddlewareConfig) WithRequireTenant(required bool) MiddlewareConfig {
	cfg.RequireTenant = required
	return cfg
}

func (cfg MiddlewareConfig) WithAllowServiceTokens(allow bool) MiddlewareConfig {
	cfg.AllowServiceTokens = allow
	return cfg
}

func (cfg MiddlewareConfig) WithSubjectUserIDResolver(resolver StringResolver) MiddlewareConfig {
	cfg.SubjectUserIDResolver = resolver
	return cfg
}

func (cfg MiddlewareConfig) WithTenantIDResolver(resolver StringResolver) MiddlewareConfig {
	cfg.TenantIDResolver = resolver
	return cfg
}

func (cfg MiddlewareConfig) WithCategoryResolver(resolver StringResolver) MiddlewareConfig {
	cfg.CategoryResolver = resolver
	return cfg
}

func (cfg MiddlewareConfig) WithActionResolver(resolver StringResolver) MiddlewareConfig {
	cfg.ActionResolver = resolver
	return cfg
}

func (cfg MiddlewareConfig) WithResourceTypeResolver(resolver StringResolver) MiddlewareConfig {
	cfg.ResourceTypeResolver = resolver
	return cfg
}

func (cfg MiddlewareConfig) WithResourceIDResolver(resolver StringResolver) MiddlewareConfig {
	cfg.ResourceIDResolver = resolver
	return cfg
}

func (cfg MiddlewareConfig) WithContextResolver(resolver ContextResolver) MiddlewareConfig {
	cfg.ContextResolver = resolver
	return cfg
}

func (cfg MiddlewareConfig) WithResourcePathParam(param string) MiddlewareConfig {
	cfg.ResourceIDResolver = ParamResolver(param)
	return cfg
}

func Middleware(cfg *config.Config, log logger.LogManager, runtimeCfg MiddlewareConfig) gin.HandlerFunc {
	client := newAuthzClientFunc(log, cfg)
	cache := newDecisionCache(runtimeCfg.CacheTTL)

	return func(c *gin.Context) {
		if !runtimeCfg.Enabled {
			c.Next()
			return
		}

		reqLog := logger.GetLogger(c)
		if reqLog == nil {
			reqLog = log
		}

		claims, ok := auth.GetClaims(c)
		if !ok {
			if reqClaims, found := auth.ClaimsFromContext(c.Request.Context()); found {
				claims = reqClaims
				ok = true
			}
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
			return
		}

		if claims.IsServiceToken() {
			if runtimeCfg.AllowServiceTokens {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "service_token_not_allowed", "message": "service token not allowed"})
			return
		}

		if tokenUse := strings.TrimSpace(claims.TokenUse); tokenUse != "" && !strings.EqualFold(tokenUse, "access") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "access_token_required", "message": "access token required"})
			return
		}

		input, err := buildDecisionRequest(c, claims, runtimeCfg)
		if err != nil {
			if reqLog != nil {
				reqLog.ErrorFCtx(c.Request.Context(), "authorization middleware configuration error: %v", err)
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authz_config_invalid", "message": err.Error()})
			return
		}

		cacheKey := decisionCacheKey(input)
		decision, err := client.Decide(c.Request.Context(), input)
		if err != nil {
			if cached, found := cache.lookup(cacheKey); found {
				storeDecision(c, cached)
				if cached.Allowed {
					c.Next()
					return
				}
				abortDenied(c, cached)
				return
			}
			if runtimeCfg.FailOpen {
				fallback := DecisionResponse{
					Allowed:        true,
					SubjectUserID:  input.SubjectUserID,
					Service:        input.Service,
					PermissionCode: permissionCode(input.Service, input.Category, input.Action),
					TenantID:       input.TenantID,
					ResourceType:   input.ResourceType,
					ResourceID:     input.ResourceID,
					Reasons:        []string{"authz_fail_open"},
				}
				storeDecision(c, fallback)
				c.Next()
				return
			}
			if reqLog != nil {
				reqLog.ErrorFCtx(c.Request.Context(), "authorization decision failed: %v", err)
			}
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "authorization_unavailable", "message": "authorization service unavailable"})
			return
		}

		cache.store(cacheKey, decision)
		storeDecision(c, decision)
		if !decision.Allowed {
			abortDenied(c, decision)
			return
		}

		c.Next()
	}
}

func GetDecision(c *gin.Context) (DecisionResponse, bool) {
	if c == nil {
		return DecisionResponse{}, false
	}
	val, exists := c.Get(string(ctxDecisionKey))
	if !exists {
		return DecisionResponse{}, false
	}
	decision, ok := val.(DecisionResponse)
	return decision, ok
}

func DecisionFromContext(ctx context.Context) (DecisionResponse, bool) {
	if ctx == nil {
		return DecisionResponse{}, false
	}
	val := ctx.Value(string(ctxDecisionKey))
	decision, ok := val.(DecisionResponse)
	return decision, ok
}

func ContextWithDecision(ctx context.Context, decision DecisionResponse) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, string(ctxDecisionKey), decision)
}

func SubjectFromClaims(_ *gin.Context, claims auth.Claims) string {
	return strings.TrimSpace(claims.UserID())
}

func TenantFromContextOrClaims(c *gin.Context, claims auth.Claims) string {
	if c != nil {
		if tenantID, ok := auth.TenantIDFromContext(c.Request.Context()); ok {
			return tenantID
		}
		if value, exists := c.Get("tenant_id"); exists {
			if tenantID, ok := value.(string); ok {
				trimmed := strings.TrimSpace(tenantID)
				if trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return strings.TrimSpace(claims.TenantID())
}

func StaticResolver(value string) StringResolver {
	value = strings.TrimSpace(value)
	return func(_ *gin.Context, _ auth.Claims) string {
		return value
	}
}

func ParamResolver(name string) StringResolver {
	name = strings.TrimSpace(name)
	return func(c *gin.Context, _ auth.Claims) string {
		if c == nil || name == "" {
			return ""
		}
		return strings.TrimSpace(c.Param(name))
	}
}

func QueryResolver(name string) StringResolver {
	name = strings.TrimSpace(name)
	return func(c *gin.Context, _ auth.Claims) string {
		if c == nil || name == "" {
			return ""
		}
		return strings.TrimSpace(c.Query(name))
	}
}

func HeaderResolver(name string) StringResolver {
	name = strings.TrimSpace(name)
	return func(c *gin.Context, _ auth.Claims) string {
		if c == nil || name == "" {
			return ""
		}
		return strings.TrimSpace(c.GetHeader(name))
	}
}

func MethodActionResolver(overrides map[string]string) StringResolver {
	resolved := map[string]string{
		http.MethodGet:    "read",
		http.MethodHead:   "read",
		http.MethodPost:   "create",
		http.MethodPut:    "update",
		http.MethodPatch:  "update",
		http.MethodDelete: "delete",
	}
	for method, action := range overrides {
		method = strings.ToUpper(strings.TrimSpace(method))
		action = strings.TrimSpace(action)
		if method == "" || action == "" {
			continue
		}
		resolved[method] = action
	}
	return func(c *gin.Context, _ auth.Claims) string {
		if c == nil || c.Request == nil {
			return ""
		}
		return strings.TrimSpace(resolved[strings.ToUpper(strings.TrimSpace(c.Request.Method))])
	}
}

func QueryContextResolver(names ...string) ContextResolver {
	clean := cleanNames(names)
	return func(c *gin.Context, _ auth.Claims) map[string]string {
		if c == nil || len(clean) == 0 {
			return nil
		}
		values := make(map[string]string)
		for _, name := range clean {
			if value := strings.TrimSpace(c.Query(name)); value != "" {
				values[name] = value
			}
		}
		if len(values) == 0 {
			return nil
		}
		return values
	}
}

func JSONBodyContextResolver(paths ...string) ContextResolver {
	clean := cleanNames(paths)
	return func(c *gin.Context, _ auth.Claims) map[string]string {
		if c == nil || len(clean) == 0 {
			return nil
		}

		body, ok := requestJSONBody(c)
		if !ok || len(body) == 0 {
			return nil
		}

		values := make(map[string]string)
		for _, path := range clean {
			value, found := lookupJSONPath(body, path)
			if !found {
				continue
			}
			stringValue, ok := stringifyJSONContextValue(value)
			if !ok {
				continue
			}
			values[path] = stringValue
		}

		if len(values) == 0 {
			return nil
		}
		return values
	}
}

func HeaderContextResolver(names ...string) ContextResolver {
	clean := cleanNames(names)
	return func(c *gin.Context, _ auth.Claims) map[string]string {
		if c == nil || len(clean) == 0 {
			return nil
		}
		values := make(map[string]string)
		for _, name := range clean {
			if value := strings.TrimSpace(c.GetHeader(name)); value != "" {
				values[name] = value
			}
		}
		if len(values) == 0 {
			return nil
		}
		return values
	}
}

func MergeContextResolvers(resolvers ...ContextResolver) ContextResolver {
	return func(c *gin.Context, claims auth.Claims) map[string]string {
		merged := make(map[string]string)
		for _, resolver := range resolvers {
			if resolver == nil {
				continue
			}
			for key, value := range resolver(c, claims) {
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				if key == "" || value == "" {
					continue
				}
				merged[key] = value
			}
		}
		if len(merged) == 0 {
			return nil
		}
		return merged
	}
}

type decisionCache struct {
	ttl time.Duration
	mu  sync.Mutex
	set map[string]*cachedDecision
}

type cachedDecision struct {
	result    DecisionResponse
	expiresAt time.Time
}

func newDecisionCache(ttl time.Duration) *decisionCache {
	return &decisionCache{
		ttl: ttl,
		set: make(map[string]*cachedDecision),
	}
}

func (c *decisionCache) store(key string, result DecisionResponse) {
	if c == nil || c.ttl <= 0 || strings.TrimSpace(key) == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.set[key] = &cachedDecision{
		result:    cloneDecision(result),
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *decisionCache) lookup(key string) (DecisionResponse, bool) {
	if c == nil || c.ttl <= 0 || strings.TrimSpace(key) == "" {
		return DecisionResponse{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.set[key]
	if !ok {
		return DecisionResponse{}, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.set, key)
		return DecisionResponse{}, false
	}

	result := cloneDecision(entry.result)
	result.CacheHit = true
	return result, true
}

func buildDecisionRequest(c *gin.Context, claims auth.Claims, cfg MiddlewareConfig) (DecisionRequest, error) {
	input := DecisionRequest{
		SubjectUserID: resolveString(c, claims, cfg.SubjectUserIDResolver, cfg.SubjectUserIDResolver == nil),
		TenantID:      resolveString(c, claims, cfg.TenantIDResolver, true),
		Service:       strings.TrimSpace(cfg.Service),
		Category:      strings.TrimSpace(cfg.Category),
		Action:        strings.TrimSpace(cfg.Action),
		ResourceType:  strings.TrimSpace(cfg.ResourceType),
	}

	if cfg.Service != "" {
		input.Service = strings.TrimSpace(cfg.Service)
	}
	if cfg.CategoryResolver != nil {
		input.Category = resolveString(c, claims, cfg.CategoryResolver, true)
	}
	if cfg.ActionResolver != nil {
		input.Action = resolveString(c, claims, cfg.ActionResolver, true)
	}
	if cfg.ResourceTypeResolver != nil {
		input.ResourceType = resolveString(c, claims, cfg.ResourceTypeResolver, true)
	}
	if cfg.ResourceIDResolver != nil {
		input.ResourceID = resolveString(c, claims, cfg.ResourceIDResolver, true)
	}
	if cfg.ContextResolver != nil {
		input.Context = sanitizeContext(cfg.ContextResolver(c, claims))
	}

	if strings.TrimSpace(input.SubjectUserID) == "" {
		return DecisionRequest{}, httpError("subject_user_id resolver returned empty")
	}
	if strings.TrimSpace(input.Service) == "" {
		return DecisionRequest{}, httpError("service is required")
	}
	if strings.TrimSpace(input.Category) == "" {
		return DecisionRequest{}, httpError("category is required")
	}
	if strings.TrimSpace(input.Action) == "" {
		return DecisionRequest{}, httpError("action is required")
	}
	if cfg.RequireTenant && strings.TrimSpace(input.TenantID) == "" {
		return DecisionRequest{}, httpError("tenant_id is required")
	}

	return input, nil
}

func resolveString(c *gin.Context, claims auth.Claims, resolver StringResolver, optional bool) string {
	if resolver == nil {
		if optional {
			return ""
		}
		return ""
	}
	return strings.TrimSpace(resolver(c, claims))
}

func sanitizeContext(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	sanitized := make(map[string]string)
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		sanitized[key] = value
	}
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

func cleanNames(names []string) []string {
	out := make([]string, 0, len(names))
	seen := make(map[string]struct{})
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	return out
}

type cachedRequestJSON struct {
	loaded bool
	body   map[string]any
}

func requestBodyBytes(c *gin.Context) ([]byte, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, false
	}

	if cached, exists := c.Get(string(ctxRequestBodyKey)); exists {
		if payload, ok := cached.([]byte); ok {
			return append([]byte(nil), payload...), true
		}
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(payload))
	c.Set(string(ctxRequestBodyKey), append([]byte(nil), payload...))
	return payload, true
}

func requestJSONBody(c *gin.Context) (map[string]any, bool) {
	if c == nil {
		return nil, false
	}

	if cached, exists := c.Get(string(ctxRequestJSONKey)); exists {
		entry, ok := cached.(cachedRequestJSON)
		if !ok {
			return nil, false
		}
		if !entry.loaded || len(entry.body) == 0 {
			return nil, false
		}
		return entry.body, true
	}

	payload, ok := requestBodyBytes(c)
	if !ok || len(bytes.TrimSpace(payload)) == 0 {
		c.Set(string(ctxRequestJSONKey), cachedRequestJSON{loaded: true})
		return nil, false
	}

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		c.Set(string(ctxRequestJSONKey), cachedRequestJSON{loaded: true})
		return nil, false
	}

	c.Set(string(ctxRequestJSONKey), cachedRequestJSON{
		loaded: true,
		body:   body,
	})
	return body, true
}

func lookupJSONPath(input map[string]any, path string) (any, bool) {
	if len(input) == 0 {
		return nil, false
	}

	current := any(input)
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}

func stringifyJSONContextValue(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", false
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			return trimmed, true
		}
		return "", false
	case bool:
		return strconv.FormatBool(typed), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), true
	case int:
		return strconv.Itoa(typed), true
	case int8, int16, int32, int64:
		return strconv.FormatInt(reflectInt64(typed), 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8, uint16, uint32, uint64:
		return strconv.FormatUint(reflectUint64(typed), 10), true
	case json.Number:
		if trimmed := strings.TrimSpace(typed.String()); trimmed != "" {
			return trimmed, true
		}
		return "", false
	default:
		return "", false
	}
}

func reflectInt64(value any) int64 {
	switch typed := value.(type) {
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	default:
		return int64(value.(int))
	}
}

func reflectUint64(value any) uint64 {
	switch typed := value.(type) {
	case uint8:
		return uint64(typed)
	case uint16:
		return uint64(typed)
	case uint32:
		return uint64(typed)
	case uint64:
		return typed
	default:
		return uint64(value.(uint))
	}
}

func decisionCacheKey(input DecisionRequest) string {
	payload, _ := json.Marshal(input)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func cloneDecision(in DecisionResponse) DecisionResponse {
	out := in
	out.RoleIDs = append([]string(nil), in.RoleIDs...)
	out.GroupIDs = append([]string(nil), in.GroupIDs...)
	out.AllowPolicyIDs = append([]string(nil), in.AllowPolicyIDs...)
	out.DenyPolicyIDs = append([]string(nil), in.DenyPolicyIDs...)
	out.DelegationIDs = append([]string(nil), in.DelegationIDs...)
	out.Reasons = append([]string(nil), in.Reasons...)
	return out
}

func storeDecision(c *gin.Context, decision DecisionResponse) {
	if c == nil {
		return
	}
	c.Set(string(ctxDecisionKey), decision)
	c.Request = c.Request.WithContext(ContextWithDecision(c.Request.Context(), decision))
}

func abortDenied(c *gin.Context, decision DecisionResponse) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error":           "access_denied",
		"message":         "authorization denied",
		"permission_code": decision.PermissionCode,
		"reasons":         decision.Reasons,
	})
}

func permissionCode(service, category, action string) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(service)),
		strings.ToLower(strings.TrimSpace(category)),
		strings.ToLower(strings.TrimSpace(action)),
	}
	return strings.Join(parts, "-")
}

type middlewareConfigError struct {
	message string
}

func (e middlewareConfigError) Error() string { return e.message }

func httpError(message string) error {
	return middlewareConfigError{message: strings.TrimSpace(message)}
}
