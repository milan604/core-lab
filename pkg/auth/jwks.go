package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	corehttp "github.com/milan604/core-lab/pkg/http"
)

const (
	defaultJWKSCacheTTL    = 5 * time.Minute
	defaultJWKSHTTPTimeout = 5 * time.Second
)

type remoteKeyProvider struct {
	discoveryURL    string
	explicitJWKSURL string
	fallbackJWKSURL string
	client          *http.Client
	cacheTTL        time.Duration

	mu            sync.RWMutex
	cachedJWKSURL string
	cachedKeys    *cachedKeySet
}

type cachedKeySet struct {
	keysByID  map[string]interface{}
	allKeys   []interface{}
	expiresAt time.Time
}

type oidcDiscoveryDocument struct {
	JWKSURI string `json:"jwks_uri"`
}

type jsonWebKeySet struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid,omitempty"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

func newRemoteKeyProvider(cfg Config) *remoteKeyProvider {
	if cfg == nil {
		return nil
	}

	explicitJWKSURL := strings.TrimSpace(cfg.GetString("SentinelJWKSURL"))
	discoveryURL := strings.TrimSpace(cfg.GetString("SentinelOIDCDiscoveryURL"))
	fallbackJWKSURL := ""

	if discoveryURL == "" {
		if baseURL := corehttp.NormalizeSentinelBaseURL(cfg.GetString("SentinelServiceEndpoint")); baseURL != "" {
			discoveryURL = baseURL + "/.well-known/openid-configuration"
			fallbackJWKSURL = baseURL + "/.well-known/jwks.json"
		}
	}

	if explicitJWKSURL == "" && discoveryURL == "" && fallbackJWKSURL == "" {
		return nil
	}

	cacheTTL := time.Duration(parseIntStringDefault(cfg.GetString("SentinelJWKSCacheTTLSeconds"), int(defaultJWKSCacheTTL/time.Second))) * time.Second
	if cacheTTL <= 0 {
		cacheTTL = defaultJWKSCacheTTL
	}

	return &remoteKeyProvider{
		discoveryURL:    discoveryURL,
		explicitJWKSURL: explicitJWKSURL,
		fallbackJWKSURL: fallbackJWKSURL,
		client: &http.Client{
			Timeout: defaultJWKSHTTPTimeout,
		},
		cacheTTL: cacheTTL,
	}
}

func (p *remoteKeyProvider) LookupKeys(token *jwt.Token) ([]interface{}, error) {
	if p == nil {
		return nil, fmt.Errorf("jwks provider is nil")
	}

	kid := strings.TrimSpace(fmt.Sprint(token.Header["kid"]))
	keySet, err := p.loadKeySet(false)
	if err != nil && keySet == nil {
		return nil, err
	}

	keys := keySet.selectKeys(kid)
	if len(keys) == 0 && kid != "" {
		keySet, err = p.loadKeySet(true)
		if err == nil && keySet != nil {
			keys = keySet.selectKeys(kid)
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no jwks keys matched kid=%q", kid)
	}

	return keys, nil
}

func (p *remoteKeyProvider) loadKeySet(force bool) (*cachedKeySet, error) {
	now := time.Now()
	if !force {
		if snapshot := p.cachedSnapshot(now); snapshot != nil {
			return snapshot, nil
		}
	}

	jwksURL, err := p.resolveJWKSURL(force)
	if err != nil {
		if snapshot := p.cachedSnapshot(time.Time{}); snapshot != nil {
			return snapshot, nil
		}
		return nil, err
	}

	fresh, err := p.fetchKeySet(jwksURL)
	if err != nil {
		if snapshot := p.cachedSnapshot(time.Time{}); snapshot != nil {
			return snapshot, nil
		}
		return nil, err
	}

	p.mu.Lock()
	p.cachedKeys = fresh
	p.mu.Unlock()

	return fresh, nil
}

func (p *remoteKeyProvider) resolveJWKSURL(force bool) (string, error) {
	if p == nil {
		return "", fmt.Errorf("jwks provider is nil")
	}

	if p.explicitJWKSURL != "" {
		return p.explicitJWKSURL, nil
	}

	p.mu.RLock()
	cached := p.cachedJWKSURL
	p.mu.RUnlock()
	if !force && cached != "" {
		return cached, nil
	}

	if p.discoveryURL != "" {
		req, err := http.NewRequest(http.MethodGet, p.discoveryURL, nil)
		if err != nil {
			return "", fmt.Errorf("build discovery request: %w", err)
		}

		resp, err := p.client.Do(req)
		if err == nil {
			defer resp.Body.Close()

			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				var doc oidcDiscoveryDocument
				if decodeErr := json.NewDecoder(resp.Body).Decode(&doc); decodeErr == nil {
					jwksURL := strings.TrimSpace(doc.JWKSURI)
					if jwksURL != "" {
						p.mu.Lock()
						p.cachedJWKSURL = jwksURL
						p.mu.Unlock()
						return jwksURL, nil
					}
				}
			}
		}
	}

	if p.fallbackJWKSURL != "" {
		return p.fallbackJWKSURL, nil
	}

	return "", fmt.Errorf("jwks url unavailable")
}

func (p *remoteKeyProvider) fetchKeySet(jwksURL string) (*cachedKeySet, error) {
	req, err := http.NewRequest(http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build jwks request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("jwks endpoint returned status %d", resp.StatusCode)
	}

	var payload jsonWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode jwks response: %w", err)
	}

	keysByID := make(map[string]interface{}, len(payload.Keys))
	allKeys := make([]interface{}, 0, len(payload.Keys))
	for _, jwk := range payload.Keys {
		if use := strings.TrimSpace(jwk.Use); use != "" && !strings.EqualFold(use, "sig") {
			continue
		}

		publicKey, err := parseJSONWebKey(jwk)
		if err != nil {
			return nil, err
		}

		if kid := strings.TrimSpace(jwk.Kid); kid != "" {
			keysByID[kid] = publicKey
		}
		allKeys = append(allKeys, publicKey)
	}

	if len(allKeys) == 0 {
		return nil, fmt.Errorf("jwks response did not contain any signing keys")
	}

	return &cachedKeySet{
		keysByID:  keysByID,
		allKeys:   allKeys,
		expiresAt: time.Now().Add(p.cacheTTL),
	}, nil
}

func (p *remoteKeyProvider) cachedSnapshot(now time.Time) *cachedKeySet {
	if p == nil {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.cachedKeys == nil {
		return nil
	}
	if !now.IsZero() && now.After(p.cachedKeys.expiresAt) {
		return nil
	}

	return p.cachedKeys
}

func (c *cachedKeySet) selectKeys(kid string) []interface{} {
	if c == nil {
		return nil
	}

	kid = strings.TrimSpace(kid)
	if kid != "" {
		if key, ok := c.keysByID[kid]; ok {
			return []interface{}{key}
		}
		return nil
	}

	return append([]interface{}{}, c.allKeys...)
}

func parseJSONWebKey(jwk jsonWebKey) (interface{}, error) {
	switch strings.ToUpper(strings.TrimSpace(jwk.Kty)) {
	case "RSA":
		return parseRSAJSONWebKey(jwk)
	case "EC":
		return parseECDSAJSONWebKey(jwk)
	default:
		return nil, fmt.Errorf("unsupported jwk key type %q", jwk.Kty)
	}
}

func parseRSAJSONWebKey(jwk jsonWebKey) (*rsa.PublicKey, error) {
	nBytes, err := decodeBase64URL(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("decode rsa modulus: %w", err)
	}
	eBytes, err := decodeBase64URL(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("decode rsa exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	if n.Sign() == 0 || e.Sign() == 0 {
		return nil, fmt.Errorf("invalid rsa jwk")
	}

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

func parseECDSAJSONWebKey(jwk jsonWebKey) (*ecdsa.PublicKey, error) {
	xBytes, err := decodeBase64URL(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("decode ecdsa x coordinate: %w", err)
	}
	yBytes, err := decodeBase64URL(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("decode ecdsa y coordinate: %w", err)
	}

	curve, err := parseJWKCurve(jwk.Crv)
	if err != nil {
		return nil, err
	}

	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)

	return &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}, nil
}

func parseJWKCurve(curveName string) (elliptic.Curve, error) {
	switch strings.TrimSpace(curveName) {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported jwk curve %q", curveName)
	}
}

func decodeBase64URL(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("empty base64url value")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err == nil {
		return decoded, nil
	}

	return base64.URLEncoding.DecodeString(trimmed)
}

func parseIntStringDefault(raw string, fallback int) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return fallback
	}

	return parsed
}
