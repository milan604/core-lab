package quota

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Limits maps quota names to their numeric limits.
type Limits map[string]float64

// Config controls the quota middleware behaviour.
type Config struct {
	Enabled         bool
	DefaultLimits   Limits
	LimitsExtractor func(c *gin.Context) Limits
	CleanupInterval time.Duration
}

// DefaultConfig returns production-safe defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		DefaultLimits:   Limits{"api_calls_per_day": 10000},
		CleanupInterval: 10 * time.Minute,
	}
}

// Enforcer tracks per-tenant counters and enforces limits.
type Enforcer struct {
	cfg     Config
	mu      sync.RWMutex
	buckets map[string]*bucket
	stopCh  chan struct{}
}

type bucket struct {
	counters map[string]float64
	day      string
	lastSeen time.Time
}

// New creates a new Enforcer and starts the cleanup goroutine.
func New(cfg Config) *Enforcer {
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 10 * time.Minute
	}
	e := &Enforcer{
		cfg:     cfg,
		buckets: make(map[string]*bucket),
		stopCh:  make(chan struct{}),
	}
	go e.cleanupLoop()
	return e
}

// Stop terminates the background cleanup goroutine.
func (e *Enforcer) Stop() {
	close(e.stopCh)
}

// Middleware returns a Gin middleware that increments the api_calls_per_day
// counter and rejects requests that exceed limits.
func (e *Enforcer) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !e.cfg.Enabled {
			c.Next()
			return
		}

		tenantID := extractTenantID(c)
		if tenantID == "" {
			c.Next()
			return
		}

		limits := e.resolveLimits(c)
		if limits == nil || len(limits) == 0 {
			c.Next()
			return
		}

		today := time.Now().UTC().Format("2006-01-02")
		quotaName := DefaultMetricAPICallsPerDay

		e.mu.Lock()
		b := e.getOrCreateBucket(tenantID, today)
		b.lastSeen = time.Now()

		limit, hasLimit := limits[quotaName]
		if hasLimit {
			if b.counters[quotaName] >= limit {
				e.mu.Unlock()
				c.Header("X-RateLimit-Limit", strconv.FormatFloat(limit, 'f', 0, 64))
				c.Header("X-RateLimit-Remaining", "0")
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":   ReasonQuotaExceeded,
					"message": "Tenant has exceeded the " + quotaName + " quota",
					"limit":   limit,
				})
				return
			}
			b.counters[quotaName]++
			remaining := limit - b.counters[quotaName]
			c.Header("X-RateLimit-Limit", strconv.FormatFloat(limit, 'f', 0, 64))
			c.Header("X-RateLimit-Remaining", strconv.FormatFloat(remaining, 'f', 0, 64))
		}
		e.mu.Unlock()

		c.Next()
	}
}

// GetUsage returns the current counter values for a tenant.
func (e *Enforcer) GetUsage(tenantID string) map[string]float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	b, exists := e.buckets[tenantID]
	if !exists {
		return nil
	}
	out := make(map[string]float64, len(b.counters))
	for k, v := range b.counters {
		out[k] = v
	}
	return out
}

func (e *Enforcer) resolveLimits(c *gin.Context) Limits {
	if e.cfg.LimitsExtractor != nil {
		if l := e.cfg.LimitsExtractor(c); l != nil {
			return l
		}
	}
	return e.cfg.DefaultLimits
}

func (e *Enforcer) getOrCreateBucket(tenantID, today string) *bucket {
	b, exists := e.buckets[tenantID]
	if !exists || b.day != today {
		b = &bucket{
			counters: make(map[string]float64),
			day:      today,
		}
		e.buckets[tenantID] = b
	}
	return b
}

func (e *Enforcer) cleanupLoop() {
	ticker := time.NewTicker(e.cfg.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.mu.Lock()
			cutoff := time.Now().Add(-2 * e.cfg.CleanupInterval)
			for k, b := range e.buckets {
				if b.lastSeen.Before(cutoff) {
					delete(e.buckets, k)
				}
			}
			e.mu.Unlock()
		}
	}
}

func extractTenantID(c *gin.Context) string {
	if v, exists := c.Get("tenant_id"); exists {
		if s, _ := v.(string); s != "" {
			return s
		}
	}
	if raw, exists := c.Get("claims_raw"); exists {
		if m, _ := raw.(map[string]interface{}); m != nil {
			if tid, _ := m["tenant_id"].(string); tid != "" {
				return tid
			}
		}
	}
	return ""
}
