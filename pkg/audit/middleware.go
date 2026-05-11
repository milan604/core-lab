package audit

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/milan604/core-lab/pkg/auth"
	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/logger"
	coretenant "github.com/milan604/core-lab/pkg/tenant"
)

var defaultAuditedMethods = []string{
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
}

var defaultSkipPathSuffixes = []string{
	"/quota/check",
}

type MiddlewareConfig struct {
	Enabled          bool
	Service          string
	Publisher        Publisher
	Logger           logger.LogManager
	Methods          []string
	SkipPathPrefixes []string
	SkipPathSuffixes []string
	ShouldAudit      func(*gin.Context) bool
}

func NewMiddlewareConfig(cfg *config.Config, defaultService string, publisher Publisher, log logger.LogManager) MiddlewareConfig {
	service := strings.TrimSpace(defaultService)
	if cfg != nil {
		if configured := strings.TrimSpace(cfg.GetString("service_name")); configured != "" {
			service = configured
		}
	}

	skipPrefixes := []string{}
	if cfg != nil {
		skipPrefixes = splitCSV(cfg.GetString("AuditSkipPathPrefixes"))
	}

	skipSuffixes := append([]string{}, defaultSkipPathSuffixes...)
	if cfg != nil {
		skipSuffixes = append(skipSuffixes, splitCSV(cfg.GetString("AuditSkipPathSuffixes"))...)
	}

	methods := append([]string{}, defaultAuditedMethods...)
	if cfg != nil {
		if configuredMethods := splitCSV(cfg.GetString("AuditMethods")); len(configuredMethods) > 0 {
			methods = configuredMethods
		}
	}

	return MiddlewareConfig{
		Enabled:          cfg == nil || cfg.GetBoolD("AuditEnabled", true),
		Service:          service,
		Publisher:        publisher,
		Logger:           log,
		Methods:          methods,
		SkipPathPrefixes: skipPrefixes,
		SkipPathSuffixes: skipSuffixes,
	}
}

func Middleware(cfg MiddlewareConfig) gin.HandlerFunc {
	if !cfg.Enabled || cfg.Publisher == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	methods := cfg.Methods
	if len(methods) == 0 {
		methods = defaultAuditedMethods
	}

	allowedMethods := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method == "" {
			continue
		}
		allowedMethods[method] = struct{}{}
	}

	skipPathSuffixes := append([]string{}, defaultSkipPathSuffixes...)
	skipPathSuffixes = append(skipPathSuffixes, cfg.SkipPathSuffixes...)
	cfg.SkipPathSuffixes = skipPathSuffixes

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if !shouldAudit(c, cfg, allowedMethods) {
			return
		}

		status := getStatus(c)
		if status == "" {
			status = resolveOutcomeStatus(c.Writer.Status())
		}

		resource := getResource(c)
		if resource == "" {
			resource = resolveResource(c)
		}

		action := getAction(c)
		if action == "" {
			action = resolveAction(c.Request.Method, resource)
		}

		resourceID := getResourceID(c)
		if resourceID == "" {
			resourceID = resolveResourceID(c)
		}

		event := NewEvent(c, cfg.Service, action, resource, resourceID, status)
		event.Metadata = buildMetadata(c, start)

		if metadata := getMetadata(c); len(metadata) > 0 {
			if event.Metadata == nil {
				event.Metadata = map[string]any{}
			}
			for k, v := range metadata {
				event.Metadata[k] = v
			}
		}

		if err := cfg.Publisher.Publish(c.Request.Context(), event); err != nil && cfg.Logger != nil {
			cfg.Logger.WarnFCtx(c.Request.Context(), "failed to enqueue audit event %s (%s): %v", event.Action, event.Resource, err)
		}
	}
}

func shouldAudit(c *gin.Context, cfg MiddlewareConfig, allowedMethods map[string]struct{}) bool {
	if c == nil {
		return false
	}
	if isSkipped(c) {
		return false
	}
	if isForced(c) {
		return true
	}
	if cfg.ShouldAudit != nil && !cfg.ShouldAudit(c) {
		return false
	}

	method := strings.ToUpper(strings.TrimSpace(c.Request.Method))
	if _, ok := allowedMethods[method]; !ok {
		return false
	}

	route := strings.TrimSpace(c.FullPath())
	path := route
	if path == "" {
		path = strings.TrimSpace(c.Request.URL.Path)
	}
	if path == "" {
		return false
	}

	for _, prefix := range cfg.SkipPathPrefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return false
		}
	}

	for _, suffix := range cfg.SkipPathSuffixes {
		suffix = strings.TrimSpace(suffix)
		if suffix != "" && strings.HasSuffix(path, suffix) {
			return false
		}
	}

	if route == "" {
		return false
	}

	return true
}

func buildMetadata(c *gin.Context, started time.Time) map[string]any {
	metadata := map[string]any{
		"http_method":      c.Request.Method,
		"http_path":        c.Request.URL.Path,
		"http_route":       c.FullPath(),
		"http_status_code": c.Writer.Status(),
		"duration_ms":      time.Since(started).Milliseconds(),
	}

	if claims, ok := auth.GetClaims(c); ok {
		if tokenUse := strings.TrimSpace(claims.TokenUse); tokenUse != "" {
			metadata["token_use"] = tokenUse
		}
		if claims.IsServiceToken() {
			metadata["actor_type"] = "service"
		} else if claims.Subject != "" {
			metadata["actor_type"] = "user"
		}
		if tenantStatus := strings.TrimSpace(claims.TenantStatus()); tenantStatus != "" {
			metadata["tenant_status"] = tenantStatus
		}
		if membershipStatus := rawClaimString(claims.Raw, "tenant_membership_status"); membershipStatus != "" {
			metadata["tenant_membership_status"] = membershipStatus
		}
	}
	if requestContext, ok := coretenant.RequestContextFromContext(c.Request.Context()); ok {
		if requestContext.TenantID != "" {
			metadata[coretenant.MetadataTenantID] = requestContext.TenantID
		}
		if requestContext.ActorUserID != "" {
			metadata[coretenant.MetadataActorUserID] = requestContext.ActorUserID
		}
		if requestContext.ServiceID != "" {
			metadata[coretenant.MetadataServiceID] = requestContext.ServiceID
		}
		if requestContext.CorrelationID != "" {
			metadata[coretenant.MetadataCorrelationID] = requestContext.CorrelationID
		}
		if requestContext.IsSuperAdmin {
			metadata[coretenant.MetadataIsSuperAdmin] = true
		}
	}

	if len(c.Errors) > 0 {
		metadata["error_count"] = len(c.Errors)
	}

	return metadata
}

func resolveOutcomeStatus(httpStatus int) string {
	switch {
	case httpStatus >= http.StatusOK && httpStatus < http.StatusBadRequest:
		return "success"
	case httpStatus == http.StatusUnauthorized || httpStatus == http.StatusForbidden:
		return "denied"
	default:
		return "failure"
	}
}

func resolveAction(method, resource string) string {
	verb := "change"
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost:
		verb = "create"
	case http.MethodPut, http.MethodPatch:
		verb = "update"
	case http.MethodDelete:
		verb = "delete"
	case http.MethodGet:
		verb = "read"
	}

	resource = strings.TrimSpace(resource)
	if resource == "" {
		resource = "request"
	}

	return resource + "." + verb
}

func resolveResource(c *gin.Context) string {
	path := strings.TrimSpace(c.FullPath())
	if path == "" {
		path = strings.TrimSpace(c.Request.URL.Path)
	}
	if path == "" {
		return "request"
	}

	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" || strings.HasPrefix(part, ":") {
			continue
		}
		return normalizeSegment(part)
	}

	return "request"
}

func resolveResourceID(c *gin.Context) string {
	for i := len(c.Params) - 1; i >= 0; i-- {
		param := c.Params[i]
		key := strings.TrimSpace(param.Key)
		if key == "" {
			continue
		}
		if key == "id" || strings.HasSuffix(key, "_id") {
			return strings.TrimSpace(param.Value)
		}
	}
	return ""
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func normalizeSegment(part string) string {
	part = strings.TrimSpace(strings.Trim(part, "/"))
	if part == "" {
		return ""
	}

	replacer := strings.NewReplacer("-", "_", ".", "_", " ", "_", ":", "_")
	return replacer.Replace(strings.ToLower(part))
}

func rawClaimString(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}
