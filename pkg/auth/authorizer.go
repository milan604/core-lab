package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/milan604/core-lab/pkg/logger"
	"github.com/milan604/core-lab/pkg/permissions"
)

// PermissionLookup defines the interface for looking up permission metadata.
// This avoids import cycles between auth and service packages.
type PermissionLookup interface {
	LookupPermission(code string) (permissions.Metadata, bool)
}

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

const (
	// CtxAuthClaims is the key for storing authenticated claims in context.
	CtxAuthClaims ContextKey = "auth_claims"
	// CtxMiddlewareServiceKey is the key for storing service in context.
	CtxMiddlewareServiceKey ContextKey = "middleware_service"
)

// Authorizer handles JWT-based authorization with bitmask permission checking.
type Authorizer struct {
	verifier *jwtVerifier
	log      logger.LogManager
}

// Config provides configuration for the authorizer.
type Config interface {
	GetString(key string) string
}

// NewAuthorizer creates a new authorizer with JWT verification capabilities.
func NewAuthorizer(cfg Config, log logger.LogManager) (*Authorizer, error) {
	verifier, err := newJWTVerifier(cfg)
	if err != nil {
		return nil, err
	}
	return &Authorizer{
		verifier: verifier,
		log:      log,
	}, nil
}

// RequirePermission creates a middleware that enforces permission checking.
// It validates that the caller has the required bitmask permission.
func (a *Authorizer) RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get logger from context if available, otherwise use stored logger
		log := logger.GetLogger(c)
		if log == nil {
			log = a.log
		}

		claims, err := a.authenticate(c)
		if err != nil {
			log.ErrorFCtx(c.Request.Context(), "Authentication failed: %v", err)
			a.abortAuthError(c, err, log)
			return
		}

		// Service tokens bypass permission checks
		if claims.IsServiceToken() {
			c.Next()
			return
		}

		// Get permission lookup from context to access permission store
		// This avoids import cycles by using an interface
		val, exists := c.Get(string(CtxMiddlewareServiceKey))
		if !exists {
			log.ErrorFCtx(c.Request.Context(), "Permission check failed: service not available in context", "permission", code)
			a.abortWithJSON(c, http.StatusInternalServerError, "service_not_available", "service not available in context", log)
			return
		}
		lookup, ok := val.(PermissionLookup)
		if !ok {
			log.ErrorFCtx(c.Request.Context(), "Permission check failed: service does not implement PermissionLookup", "permission", code)
			a.abortWithJSON(c, http.StatusInternalServerError, "service_invalid", "service does not implement PermissionLookup", log)
			return
		}
		metadata, ok := lookup.LookupPermission(code)
		if !ok {
			log.WarnFCtx(c.Request.Context(), "Permission check failed: permission not registered in sentinel", "permission", code)
			a.abortWithJSON(c, http.StatusForbidden, "permission_not_registered", "permission is not registered in sentinel", log)
			return
		}

		// Check if caller has the required bitmask permission
		if !claims.HasPermission(metadata.Service, metadata.BitValue) {
			log.WarnFCtx(c.Request.Context(), "Permission check failed: caller lacks required permission",
				"permission", code,
				"service", metadata.Service,
				"bit_value", metadata.BitValue,
				"subject", claims.Subject,
			)
			a.abortWithJSON(c, http.StatusForbidden, "permission_denied", "caller lacks required permission", log)
			return
		}

		c.Next()
	}
}

// authenticate extracts and verifies the JWT token from the request.
func (a *Authorizer) authenticate(c *gin.Context) (Claims, error) {
	// Get logger from context if available, otherwise use stored logger
	log := logger.GetLogger(c)
	if log == nil {
		log = a.log
	}

	header := c.GetHeader("Authorization")
	token, err := extractBearerToken(header)
	if err != nil {
		log.ErrorFCtx(c.Request.Context(), "Failed to extract bearer token: %v", err)
		return Claims{}, err
	}

	claims, err := a.verifier.Verify(token)
	if err != nil {
		log.ErrorFCtx(c.Request.Context(), "Failed to verify JWT token: %v", err)
		return Claims{}, err
	}

	// Store claims in context for later use
	c.Set(string(CtxAuthClaims), claims)
	return claims, nil
}

// abortAuthError handles authentication/authorization errors.
func (a *Authorizer) abortAuthError(c *gin.Context, err error, log logger.LogManager) {
	status := authorizationErrorStatus(err)
	if status == http.StatusUnauthorized {
		log.ErrorFCtx(c.Request.Context(), "Authentication error: %v", err)
		a.abortWithJSON(c, status, "invalid_token", "authentication required", log)
		return
	}
	log.ErrorFCtx(c.Request.Context(), "Authorization error: %v", err)
	a.abortWithJSON(c, status, "authorization_failed", "authorization failed", log)
}

// abortWithJSON aborts the request with a JSON error response.
func (a *Authorizer) abortWithJSON(c *gin.Context, status int, code, message string, log logger.LogManager) {
	// Log the error with context for observability
	log.ErrorFCtx(c.Request.Context(), "Request aborted: %s - %s", code, message,
		"status", status,
		"path", c.FullPath(),
		"method", c.Request.Method,
	)

	c.AbortWithStatusJSON(status, gin.H{
		"error":   code,
		"message": message,
	})
}

// jwtVerifier handles JWT token verification.
type jwtVerifier struct {
	publicKey interface{}
	issuer    string
	audiences []string
}

// newJWTVerifier creates a new JWT verifier from configuration.
func newJWTVerifier(cfg Config) (*jwtVerifier, error) {
	// Get RSA public key - config key should be provided by service
	pubKey := strings.TrimSpace(cfg.GetString("RSAPublicKey"))
	if pubKey == "" {
		return nil, fmt.Errorf("jwt authorizer: RSAPublicKey not configured")
	}

	parsedKey, err := parsePublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("jwt authorizer: parse public key: %w", err)
	}

	// Issuer and audience are optional - use empty strings if not configured
	issuer := strings.TrimSpace(cfg.GetString("SentinelTokenIssuer"))
	aud := parseAudience(cfg.GetString("SentinelTokenAudience"))

	return &jwtVerifier{
		publicKey: parsedKey,
		issuer:    issuer,
		audiences: aud,
	}, nil
}

// Verify validates and parses a JWT token string.
func (v *jwtVerifier) Verify(tokenString string) (Claims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method matches the public key type
		switch key := v.publicKey.(type) {
		case *rsa.PublicKey:
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v (expected RSA)", token.Header["alg"])
			}
		case *ecdsa.PublicKey:
			if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v (expected ECDSA)", token.Header["alg"])
			}
			// Validate curve matches the key
			ecdsaMethod, ok := token.Method.(*jwt.SigningMethodECDSA)
			if ok {
				expectedCurve := getCurveForECDSAKey(key)
				if ecdsaMethod.CurveBits != expectedCurve {
					return nil, fmt.Errorf("ECDSA curve mismatch: token uses %d bits, key is %d bits", ecdsaMethod.CurveBits, expectedCurve)
				}
			}
		default:
			return nil, fmt.Errorf("unsupported public key type: %T", v.publicKey)
		}
		return v.publicKey, nil
	})
	if err != nil {
		return Claims{}, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return Claims{}, fmt.Errorf("invalid token claims")
	}

	// Validate issuer if configured
	if v.issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != v.issuer {
			return Claims{}, fmt.Errorf("invalid issuer")
		}
	}

	// Validate audience if configured
	if len(v.audiences) > 0 {
		aud, ok := claims["aud"]
		if !ok {
			return Claims{}, fmt.Errorf("missing audience claim")
		}
		audValid := false
		switch a := aud.(type) {
		case string:
			for _, expected := range v.audiences {
				if a == expected {
					audValid = true
					break
				}
			}
		case []interface{}:
			for _, expected := range v.audiences {
				for _, claimAud := range a {
					if claimAudStr, ok := claimAud.(string); ok && claimAudStr == expected {
						audValid = true
						break
					}
				}
			}
		}
		if !audValid {
			return Claims{}, fmt.Errorf("invalid audience")
		}
	}

	return mapClaimsToAuthClaims(claims), nil
}

// mapClaimsToAuthClaims converts jwt.MapClaims to our Claims struct.
func mapClaimsToAuthClaims(claims jwt.MapClaims) Claims {
	raw := make(map[string]any, len(claims))
	for k, v := range claims {
		raw[k] = v
	}

	tokenUse := strings.TrimSpace(fmt.Sprint(raw["token_use"]))
	if tokenUse == "" {
		tokenUse = "access"
	}

	svcPermRaw := ""
	if value, ok := raw["svc_perm"]; ok {
		svcPermRaw = fmt.Sprint(value)
	}

	return Claims{
		Subject:            strings.TrimSpace(fmt.Sprint(raw["sub"])),
		IdentityID:         strings.TrimSpace(fmt.Sprint(raw["identity_id"])),
		RoleID:             strings.TrimSpace(fmt.Sprint(raw["role_id"])),
		TokenUse:           tokenUse,
		ServicePermissions: decodeServicePermissionsMultiRange(svcPermRaw),
		Raw:                raw,
	}
}

// extractBearerToken extracts the bearer token from the Authorization header.
func extractBearerToken(header string) (string, error) {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return "", errors.New("authorization header missing")
	}
	const prefix = "Bearer "
	if len(trimmed) <= len(prefix) || !strings.EqualFold(trimmed[:len(prefix)], prefix) {
		return "", errors.New("authorization header must be a bearer token")
	}
	token := strings.TrimSpace(trimmed[len(prefix):])
	if token == "" {
		return "", errors.New("authorization header must be a bearer token")
	}
	return token, nil
}

// authorizationErrorStatus determines the HTTP status code for an authorization error.
func authorizationErrorStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	// Most auth errors are unauthorized
	return http.StatusUnauthorized
}

func decodeServicePermissionsMultiRange(raw string) map[string][]int64 {
	perms := make(map[string][]int64)
	if strings.TrimSpace(raw) == "" {
		return perms
	}
	entries := strings.Split(raw, ";")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			continue
		}
		serviceKey := strings.ToLower(strings.TrimSpace(parts[0]))
		if serviceKey == "" {
			continue
		}
		rangesStr := strings.TrimSpace(parts[1])
		if rangesStr == "" {
			continue
		}

		// Parse comma-separated ranges
		rangeStrs := strings.Split(rangesStr, ",")
		ranges := make([]int64, 0, len(rangeStrs))
		for _, rangeStr := range rangeStrs {
			rangeStr = strings.TrimSpace(rangeStr)
			if rangeStr == "" {
				continue
			}
			mask, err := strconv.ParseInt(rangeStr, 36, 64)
			if err != nil {
				// Skip invalid ranges
				continue
			}
			ranges = append(ranges, mask)
		}

		if len(ranges) > 0 {
			perms[serviceKey] = ranges
		}
	}
	return perms
}

// parsePublicKey parses a public key from a base64-encoded PEM string.
// Supports all formats that Sentinel service uses:
// - PKCS#8 format: "-----BEGIN PUBLIC KEY-----" (uses x509.ParsePKIXPublicKey) - supports RSA and ECDSA
// - PKCS#1 format: "-----BEGIN RSA PUBLIC KEY-----" (uses x509.ParsePKCS1PublicKey) - RSA only
// - EC format: "-----BEGIN EC PUBLIC KEY-----" (uses x509.ParsePKIXPublicKey) - ECDSA only
func parsePublicKey(pubKeyBase64 string) (interface{}, error) {
	// Decode base64 if needed
	normalized := strings.TrimSpace(pubKeyBase64)
	if normalized == "" {
		return nil, errors.New("empty public key data")
	}

	// If it doesn't contain PEM headers, try to decode from base64
	if !strings.Contains(normalized, "-----BEGIN") {
		decoded, err := base64.StdEncoding.DecodeString(normalized)
		if err != nil {
			// Try URL-safe base64
			decodedURL, errURL := base64.RawStdEncoding.DecodeString(normalized)
			if errURL != nil {
				return nil, fmt.Errorf("failed to decode public key: %w", err)
			}
			normalized = string(decodedURL)
		} else {
			normalized = string(decoded)
		}
	}

	// Decode PEM block
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	// Parse based on block type (matching Sentinel's ParsePublicKey implementation)
	switch block.Type {
	case "PUBLIC KEY":
		// PKCS#8 format - supports RSA, ECDSA, etc.
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKIX public key: %w", err)
		}
		// Support both RSA and ECDSA keys
		switch k := key.(type) {
		case *rsa.PublicKey:
			return k, nil
		case *ecdsa.PublicKey:
			return k, nil
		default:
			return nil, fmt.Errorf("unsupported public key type in PKIX format: %T", key)
		}
	case "RSA PUBLIC KEY":
		// PKCS#1 format - RSA only
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case "EC PUBLIC KEY":
		// EC format - ECDSA only
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse EC public key: %w", err)
		}
		if ecdsaKey, ok := key.(*ecdsa.PublicKey); ok {
			return ecdsaKey, nil
		}
		return nil, fmt.Errorf("EC public key block does not contain ECDSA key (got %T)", key)
	default:
		return nil, fmt.Errorf("unsupported public key type: %s", block.Type)
	}
}

// getCurveForECDSAKey returns the curve bit size for an ECDSA public key.
func getCurveForECDSAKey(key *ecdsa.PublicKey) int {
	switch key.Curve {
	case elliptic.P256():
		return 256
	case elliptic.P384():
		return 384
	case elliptic.P521():
		return 521
	default:
		return 0
	}
}

// parseAudience parses the audience string into a slice.
func parseAudience(audStr string) []string {
	if audStr == "" {
		return nil
	}
	// Simple comma-separated parsing
	parts := strings.Split(audStr, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
