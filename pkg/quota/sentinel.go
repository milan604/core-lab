package quota

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/milan604/core-lab/pkg/auth"
	"github.com/milan604/core-lab/pkg/config"
	coreerrors "github.com/milan604/core-lab/pkg/errors"
	httplib "github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"

	"github.com/gin-gonic/gin"
)

const (
	DefaultMetricAPICallsPerDay = "api_calls_per_day"

	ReasonAccessDenied      = "access_denied"
	ReasonQuotaDenied       = "quota_denied"
	ReasonQuotaExceeded     = "quota_exceeded"
	ReasonServiceNotAllowed = "service_not_allowed"
	ReasonTenantInactive    = "tenant_inactive"
	ReasonInvalidToken      = "invalid_token"
	ReasonServiceToken      = "service_token"
	ReasonNonTenantToken    = "non_tenant_token"
)

type CheckRequest struct {
	Token     string `json:"token"`
	ServiceID string `json:"service_id"`
	Metric    string `json:"metric"`
	Delta     int64  `json:"delta"`
}

type CheckResponse struct {
	Allowed    bool    `json:"allowed"`
	Bypassed   bool    `json:"bypassed,omitempty"`
	Reason     string  `json:"reason"`
	TenantID   string  `json:"tenant_id,omitempty"`
	ServiceID  string  `json:"service_id,omitempty"`
	Metric     string  `json:"metric,omitempty"`
	Limit      *int64  `json:"limit,omitempty"`
	Used       int64   `json:"used,omitempty"`
	Remaining  *int64  `json:"remaining,omitempty"`
	ResetAt    *string `json:"reset_at,omitempty"`
	StatusCode int     `json:"-"`
}

type SentinelQuotaResponse = CheckResponse

func (r CheckResponse) HTTPDecisionStatus() int {
	if r.StatusCode != 0 {
		return r.StatusCode
	}
	if strings.EqualFold(strings.TrimSpace(r.Reason), ReasonQuotaExceeded) {
		return http.StatusTooManyRequests
	}
	return http.StatusForbidden
}

func ApplyHeaders(c *gin.Context, result CheckResponse) {
	if result.Limit != nil {
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", *result.Limit))
	}
	if result.Remaining != nil {
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", *result.Remaining))
	}
	if result.ResetAt != nil && strings.TrimSpace(*result.ResetAt) != "" {
		c.Header("X-RateLimit-Reset", strings.TrimSpace(*result.ResetAt))
	}
}

func DefaultReasonForStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return ReasonInvalidToken
	case http.StatusTooManyRequests:
		return ReasonQuotaExceeded
	case http.StatusForbidden:
		return ReasonAccessDenied
	default:
		return ReasonQuotaDenied
	}
}

func MessageForReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case ReasonQuotaExceeded:
		return "tenant quota exceeded"
	case ReasonServiceNotAllowed:
		return "service not enabled for tenant plan"
	case ReasonTenantInactive:
		return "tenant subscription is not active"
	case ReasonInvalidToken:
		return "authentication required"
	default:
		return "tenant access denied"
	}
}

type SentinelClient struct {
	log         logger.LogManager
	httpClient  *httplib.Client
	cfg         *config.Config
	baseURL     string
	internalKey string
}

var newSentinelClientFunc = NewSentinelClient

func NewSentinelClient(log logger.LogManager, cfg *config.Config) *SentinelClient {
	return &SentinelClient{
		log:         log,
		httpClient:  httplib.NewClient(httplib.WithLogger(log)),
		cfg:         cfg,
		baseURL:     httplib.NormalizeSentinelBaseURL(cfg.GetString("SentinelServiceEndpoint")),
		internalKey: strings.TrimSpace(cfg.GetString("SentinelServiceAPIKey")),
	}
}

func (c *SentinelClient) CheckTenantQuota(ctx context.Context, token, serviceID, metric string, delta int64) (CheckResponse, error) {
	if c == nil {
		return CheckResponse{}, fmt.Errorf("quota client is nil")
	}
	if strings.TrimSpace(c.baseURL) == "" {
		return CheckResponse{}, fmt.Errorf("SentinelServiceEndpoint not configured")
	}
	if strings.TrimSpace(c.internalKey) == "" {
		return CheckResponse{}, fmt.Errorf("SentinelServiceAPIKey not configured")
	}

	payload := CheckRequest{
		Token:     strings.TrimSpace(token),
		ServiceID: strings.TrimSpace(serviceID),
		Metric:    strings.TrimSpace(metric),
		Delta:     delta,
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return CheckResponse{}, fmt.Errorf("encode quota request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/api/v1/quota/check", &body)
	if err != nil {
		return CheckResponse{}, fmt.Errorf("create quota request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", c.internalKey)

	resp, err := c.httpClient.Do(ctx, req)
	if err != nil {
		return CheckResponse{}, err
	}
	defer resp.Body.Close()

	var out CheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil && err != io.EOF {
		return CheckResponse{}, fmt.Errorf("decode quota response: %w", err)
	}
	out.StatusCode = resp.StatusCode

	if resp.StatusCode >= http.StatusInternalServerError {
		return out, fmt.Errorf("sentinel quota endpoint returned status %d", resp.StatusCode)
	}

	return out, nil
}

type SentinelMiddlewareConfig struct {
	Enabled   bool
	ServiceID string
	Metric    string
	FailOpen  bool
	CacheTTL  time.Duration
}

func NewSentinelMiddlewareConfig(cfg *config.Config) SentinelMiddlewareConfig {
	cacheTTLSeconds := cfg.GetIntD("QuotaCacheTTLSeconds", 60)
	if cacheTTLSeconds < 0 {
		cacheTTLSeconds = 0
	}

	return SentinelMiddlewareConfig{
		Enabled:   cfg.GetBoolD("QuotaEnabled", true),
		ServiceID: strings.TrimSpace(cfg.GetString("SentinelServiceID")),
		Metric:    strings.TrimSpace(cfg.GetStringD("QuotaMetricName", DefaultMetricAPICallsPerDay)),
		FailOpen:  cfg.GetBoolD("QuotaFailOpen", false),
		CacheTTL:  time.Duration(cacheTTLSeconds) * time.Second,
	}
}

func SentinelMiddleware(cfg *config.Config, log logger.LogManager) gin.HandlerFunc {
	runtimeCfg := NewSentinelMiddlewareConfig(cfg)
	client := newSentinelClientFunc(log, cfg)
	cache := newDecisionCache(runtimeCfg.CacheTTL)

	return func(c *gin.Context) {
		if !runtimeCfg.Enabled {
			c.Next()
			return
		}

		token, err := auth.ExtractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.Next()
			return
		}

		if runtimeCfg.ServiceID == "" {
			c.Next()
			return
		}

		cacheKey := quotaCacheKey(token, runtimeCfg.ServiceID, runtimeCfg.Metric)
		delta := int64(1)
		result, err := client.CheckTenantQuota(c.Request.Context(), token, runtimeCfg.ServiceID, runtimeCfg.Metric, delta)
		if err != nil {
			if cached, ok := cache.fallback(cacheKey, delta); ok {
				ApplyHeaders(c, cached)
				if cached.Allowed {
					if log != nil {
						log.WarnFCtx(c, "tenant quota check failed, using cached decision: %v", err)
					}
					c.Next()
					return
				}

				status := cached.HTTPDecisionStatus()
				reason := strings.TrimSpace(cached.Reason)
				if reason == "" {
					reason = DefaultReasonForStatus(status)
				}
				c.AbortWithStatusJSON(status, gin.H{
					"error":     reason,
					"message":   MessageForReason(reason),
					"tenant_id": cached.TenantID,
					"service":   cached.ServiceID,
					"metric":    cached.Metric,
				})
				return
			}

			if runtimeCfg.FailOpen {
				if log != nil {
					log.WarnFCtx(c, "tenant quota check failed, allowing request through: %v", err)
				}
				c.Next()
				return
			}

			se := coreerrors.ServiceUnavailable("tenant quota service is unavailable")
			c.AbortWithStatusJSON(se.HTTPStatus, gin.H{
				"error":   se.Code,
				"message": se.Message,
			})
			return
		}

		cache.store(cacheKey, result)
		ApplyHeaders(c, result)

		if result.Allowed {
			c.Next()
			return
		}

		status := result.HTTPDecisionStatus()
		reason := strings.TrimSpace(result.Reason)
		if reason == "" {
			reason = DefaultReasonForStatus(status)
		}
		message := MessageForReason(reason)

		c.AbortWithStatusJSON(status, gin.H{
			"error":     reason,
			"message":   message,
			"tenant_id": result.TenantID,
			"service":   result.ServiceID,
			"metric":    result.Metric,
		})
	}
}

type decisionCache struct {
	ttl time.Duration
	mu  sync.Mutex
	set map[string]*cachedDecision
}

type cachedDecision struct {
	result    CheckResponse
	expiresAt time.Time
}

func newDecisionCache(ttl time.Duration) *decisionCache {
	return &decisionCache{
		ttl: ttl,
		set: make(map[string]*cachedDecision),
	}
}

func (c *decisionCache) store(key string, result CheckResponse) {
	if c == nil || c.ttl <= 0 || strings.TrimSpace(key) == "" {
		return
	}

	expiresAt := time.Now().Add(c.ttl)
	if result.ResetAt != nil {
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*result.ResetAt)); err == nil && !parsed.IsZero() && parsed.Before(expiresAt) {
			expiresAt = parsed
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.set[key] = &cachedDecision{
		result:    cloneCheckResponse(result),
		expiresAt: expiresAt,
	}
}

func (c *decisionCache) fallback(key string, delta int64) (CheckResponse, bool) {
	if c == nil || c.ttl <= 0 || strings.TrimSpace(key) == "" {
		return CheckResponse{}, false
	}

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.set[key]
	if !ok {
		return CheckResponse{}, false
	}
	if now.After(entry.expiresAt) {
		delete(c.set, key)
		return CheckResponse{}, false
	}

	result := cloneCheckResponse(entry.result)
	if !result.Allowed {
		return result, true
	}

	if result.Remaining == nil {
		entry.result = cloneCheckResponse(result)
		return result, true
	}

	remaining := *result.Remaining
	if remaining < delta {
		remaining = 0
		result.Allowed = false
		result.Reason = ReasonQuotaExceeded
		result.StatusCode = http.StatusTooManyRequests
		result.Remaining = &remaining
		entry.result = cloneCheckResponse(result)
		return result, true
	}

	remaining -= delta
	result.Used += delta
	result.Remaining = &remaining
	entry.result = cloneCheckResponse(result)

	return result, true
}

func quotaCacheKey(token, serviceID, metric string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token) + "|" + strings.TrimSpace(serviceID) + "|" + strings.TrimSpace(metric)))
	return hex.EncodeToString(sum[:])
}

func cloneCheckResponse(in CheckResponse) CheckResponse {
	out := in
	if in.Limit != nil {
		limit := *in.Limit
		out.Limit = &limit
	}
	if in.Remaining != nil {
		remaining := *in.Remaining
		out.Remaining = &remaining
	}
	if in.ResetAt != nil {
		resetAt := strings.TrimSpace(*in.ResetAt)
		out.ResetAt = &resetAt
	}
	return out
}
