package server

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// rateLimitEntry wraps a rate.Limiter with a last-seen timestamp for stale-entry cleanup.
type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimitConfig encapsulates both configuration and runtime state for per-IP rate limiting.
type RateLimitConfig struct {
	Enabled         bool
	RPS             float64
	Burst           int
	CleanupInterval time.Duration

	limit   rate.Limit
	clients sync.Map // map[string]*rateLimitEntry
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
	now := time.Now()
	if v, ok := rl.clients.Load(ip); ok {
		entry := v.(*rateLimitEntry)
		entry.lastSeen = now
		return entry.limiter
	}
	entry := &rateLimitEntry{
		limiter:  rate.NewLimiter(rl.limit, rl.Burst),
		lastSeen: now,
	}
	rl.clients.Store(ip, entry)
	return entry.limiter
}

// cleanupLoop runs periodic cleanup of stale entries.
// Entries that have not been seen for 2× the cleanup interval are removed.
func (rl *RateLimitConfig) cleanupLoop() {
	t := time.NewTicker(rl.CleanupInterval)
	defer t.Stop()
	for range t.C {
		expiry := time.Now().Add(-2 * rl.CleanupInterval)
		rl.clients.Range(func(key, value interface{}) bool {
			entry := value.(*rateLimitEntry)
			if entry.lastSeen.Before(expiry) {
				rl.clients.Delete(key)
			}
			return true
		})
	}
}

// getRemoteIP attempts to obtain a reliable client IP
func getRemoteIP(c *gin.Context) string {
	// prefer X-Forwarded-For if present
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
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

// EndpointRateLimiter returns a per-route gin middleware with its own rate limit.
// Use this on sensitive endpoints (login, register, password reset) for stricter limits.
//
// Usage:
//
//	authGroup.POST("/login", middleware.EndpointRateLimiter(5, 10), loginHandler)
//	authGroup.POST("/register", middleware.EndpointRateLimiter(3, 5), registerHandler)
func EndpointRateLimiter(rps float64, burst int) gin.HandlerFunc {
	rl := &RateLimitConfig{
		Enabled:         true,
		RPS:             rps,
		Burst:           burst,
		CleanupInterval: 10 * time.Minute,
		limit:           rate.Limit(rps),
	}
	go rl.cleanupLoop()

	return func(c *gin.Context) {
		ip := getRemoteIP(c)
		lim := rl.getLimiter(ip)
		if !lim.Allow() {
			c.AbortWithStatusJSON(429, gin.H{
				"error":   "rate limit exceeded",
				"message": "too many requests, please try again later",
			})
			return
		}
		c.Next()
	}
}
