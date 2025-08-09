package server

import (
	"net"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimitConfig encapsulates both configuration and runtime state for per-IP rate limiting.
type RateLimitConfig struct {
	Enabled         bool
	RPS             float64
	Burst           int
	CleanupInterval time.Duration

	limit   rate.Limit
	clients sync.Map // map[string]*rate.Limiter
}

// NewRateLimitConfig creates a new RateLimitConfig and initializes runtime state.
func NewRateLimitConfig(enabled bool, rps float64, burst int, cleanupInterval time.Duration) *RateLimitConfig {
	rl := &RateLimitConfig{
		Enabled:         enabled,
		RPS:             rps,
		Burst:           burst,
		CleanupInterval: cleanupInterval,
		limit:           rate.Limit(rps),
	}
	if cleanupInterval > 0 {
		go rl.cleanupLoop()
	}
	return rl
}

// getLimiter returns the rate limiter for the given IP, creating one if needed.
func (rl *RateLimitConfig) getLimiter(ip string) *rate.Limiter {
	if v, ok := rl.clients.Load(ip); ok {
		return v.(*rate.Limiter)
	}
	lim := rate.NewLimiter(rl.limit, rl.Burst)
	rl.clients.Store(ip, lim)
	return lim
}

// cleanupLoop runs periodic cleanup of stale entries (extend for production use).
func (rl *RateLimitConfig) cleanupLoop() {
	t := time.NewTicker(rl.CleanupInterval)
	defer t.Stop()
	for range t.C {
		rl.clients.Range(func(key, value interface{}) bool {
			return true
		})
	}
}

// getRemoteIP attempts to obtain a reliable client IP
func getRemoteIP(c *gin.Context) string {
	// prefer X-Forwarded-For if present
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may contain comma separated list; take first
		if ip := xff; ip != "" {
			return ip
		}
	}
	// fallback to remote addr
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil {
		return host
	}
	return c.ClientIP()
}

// Middleware returns the gin middleware enforcing per-IP rate limits.
// It returns 429 if limiter.Allow() is false.
// Middleware returns the gin middleware enforcing per-IP rate limits.
// Returns 429 if limiter.Allow() is false.
func (rl *RateLimitConfig) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.Enabled {
			c.Next()
			return
		}
		ip := getRemoteIP(c)
		lim := rl.getLimiter(ip)
		if !lim.Allow() {
			c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
