package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig configures JWTAuth middleware.
type JWTConfig struct {
	Issuer      string        // expected iss
	Audience    string        // expected aud
	JWKSURL     string        // URL to fetch JWKS
	CacheTTL    time.Duration // cache JWKS for this long
	AllowedAlgs []string      // e.g., ["RS256"]
	HeaderName  string        // default: Authorization
	ContextKey  string        // default: "auth_claims"
}

// claimsKey returns the key used to store claims in Gin context.
func (c JWTConfig) claimsKey() string {
	if c.ContextKey != "" {
		return c.ContextKey
	}
	return "auth_claims"
}

// JWTAuth validates bearer tokens and injects claims into context.
func JWTAuth(cfg JWTConfig) gin.HandlerFunc {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "Authorization"
	}
	jwks := newJWKSCache(cfg.JWKSURL, cfg.CacheTTL)

	allowed := map[string]struct{}{}
	for _, a := range cfg.AllowedAlgs {
		allowed[strings.ToUpper(a)] = struct{}{}
	}

	return func(c *gin.Context) {
		h := c.GetHeader(cfg.HeaderName)
		if h == "" || !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		tok := strings.TrimSpace(h[len("bearer "):])

		parser := jwt.NewParser(jwt.WithValidMethods(cfg.AllowedAlgs))
		claims := jwt.MapClaims{}
		_, err := parser.ParseWithClaims(tok, claims, func(t *jwt.Token) (any, error) {
			alg := strings.ToUpper(t.Method.Alg())
			if len(allowed) > 0 {
				if _, ok := allowed[alg]; !ok {
					return nil, fmt.Errorf("alg not allowed: %s", alg)
				}
			}
			kid, _ := t.Header["kid"].(string)
			key, err := jwks.key(c.Request.Context(), kid)
			if err != nil {
				return nil, err
			}
			return key, nil
		})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		if cfg.Issuer != "" && claims["iss"] != cfg.Issuer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid issuer"})
			return
		}
		if cfg.Audience != "" {
			if !claimHasAudience(claims, cfg.Audience) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid audience"})
				return
			}
		}

		c.Set(cfg.claimsKey(), Claims{m: claims})
		c.Next()
	}
}

// RequireScopes enforces presence of ALL scopes.
func RequireScopes(scopes ...string) gin.HandlerFunc {
	set := toSet(scopes)
	return func(c *gin.Context) {
		cl, ok := GetClaims(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}
		have := toSet(cl.Scopes())
		if !have.containsAll(set) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient_scope"})
			return
		}
		c.Next()
	}
}

// RequireAnyScope enforces ANY match.
func RequireAnyScope(scopes ...string) gin.HandlerFunc {
	want := toSet(scopes)
	return func(c *gin.Context) {
		cl, ok := GetClaims(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}
		have := toSet(cl.Scopes())
		if !have.overlaps(want) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient_scope"})
			return
		}
		c.Next()
	}
}

// RequireRoles checks top-level roles claim for ALL roles.
func RequireRoles(roles ...string) gin.HandlerFunc {
	set := toSet(roles)
	return func(c *gin.Context) {
		cl, ok := GetClaims(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}
		have := toSet(cl.Roles())
		if !have.containsAll(set) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient_role"})
			return
		}
		c.Next()
	}
}

// Claims is a thin wrapper around JWT claims with helpers.
// For Keycloak-specific helpers, see the keycloak package.

type Claims struct{ m jwt.MapClaims }

func (c Claims) All() map[string]any { return c.m }
func (c Claims) Subject() string     { s, _ := c.m["sub"].(string); return s }
func (c Claims) Issuer() string      { s, _ := c.m["iss"].(string); return s }
func (c Claims) Audience() []string  { return claimAudience(c.m) }

func (c Claims) Scopes() []string {
	// scope can be space-separated string or array in scp
	if s, ok := c.m["scope"].(string); ok {
		parts := strings.Fields(s)
		return parts
	}
	if arr, ok := c.m["scp"].([]any); ok {
		out := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (c Claims) Roles() []string {
	if arr, ok := c.m["roles"].([]any); ok {
		out := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// GetClaims pulls our Claims from gin context.
func GetClaims(c *gin.Context) (Claims, bool) {
	v, ok := c.Get("auth_claims")
	if !ok {
		return Claims{}, false
	}
	cl, ok := v.(Claims)
	return cl, ok
}

// --- helpers ---

type stringSet map[string]struct{}

func toSet(ss []string) stringSet {
	s := make(stringSet, len(ss))
	for _, v := range ss {
		s[v] = struct{}{}
	}
	return s
}
func (s stringSet) containsAll(other stringSet) bool {
	for k := range other {
		if _, ok := s[k]; !ok {
			return false
		}
	}
	return true
}
func (s stringSet) overlaps(other stringSet) bool {
	for k := range other {
		if _, ok := s[k]; ok {
			return true
		}
	}
	return false
}

func claimAudience(mc jwt.MapClaims) []string {
	if s, ok := mc["aud"].(string); ok {
		return []string{s}
	}
	if arr, ok := mc["aud"].([]any); ok {
		out := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func claimHasAudience(mc jwt.MapClaims, aud string) bool {
	for _, a := range claimAudience(mc) {
		if a == aud {
			return true
		}
	}
	return false
}

// --- JWKS cache ---

type jwksCache struct {
	url   string
	ttl   time.Duration
	mu    sync.RWMutex
	keys  map[string]*rsa.PublicKey
	until time.Time
}

func newJWKSCache(url string, ttl time.Duration) *jwksCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &jwksCache{url: url, ttl: ttl, keys: map[string]*rsa.PublicKey{}}
}

func (j *jwksCache) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	j.mu.RLock()
	if k, ok := j.keys[kid]; ok && time.Now().Before(j.until) {
		j.mu.RUnlock()
		return k, nil
	}
	j.mu.RUnlock()
	return j.refresh(ctx, kid)
}

func (j *jwksCache) refresh(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if k, ok := j.keys[kid]; ok && time.Now().Before(j.until) {
		return k, nil
	}
	if j.url == "" {
		return nil, errors.New("jwks url is empty")
	}
	// minimal JWKS fetcher using stdlib
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, j.url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("jwks http %d", resp.StatusCode)
	}
	var body struct {
		Keys []struct{ Kty, Kid, N, E string }
	}
	if err := jsonNewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	// parse keys
	j.keys = map[string]*rsa.PublicKey{}
	for _, k := range body.Keys {
		if strings.ToUpper(k.Kty) != "RSA" {
			continue
		}
		pub, err := parseRSAPublicKeyFromModExp(k.N, k.E)
		if err == nil {
			j.keys[k.Kid] = pub
		}
	}
	j.until = time.Now().Add(j.ttl)
	if k, ok := j.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("kid not found")
}

// small wrappers to keep imports minimal
func jsonNewDecoder(r io.Reader) *json.Decoder { return json.NewDecoder(r) }

// parseRSAPublicKeyFromModExp converts base64url modulus and exponent into rsa.PublicKey
func parseRSAPublicKeyFromModExp(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	var e int
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	n := new(big.Int).SetBytes(nBytes)
	return &rsa.PublicKey{N: n, E: e}, nil
}
