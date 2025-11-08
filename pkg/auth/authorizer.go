package auth

import (
	"encoding/base64"
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
		claims, err := a.authenticate(c)
		if err != nil {
			a.abortAuthError(c, err)
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
			a.abortWithJSON(c, http.StatusInternalServerError, "service_not_available", "service not available in context")
			return
		}
		lookup, ok := val.(PermissionLookup)
		if !ok {
			a.abortWithJSON(c, http.StatusInternalServerError, "service_invalid", "service does not implement PermissionLookup")
			return
		}
		metadata, ok := lookup.LookupPermission(code)
		if !ok {
			a.abortWithJSON(c, http.StatusForbidden, "permission_not_registered", "permission is not registered in sentinel")
			return
		}

		// Check if caller has the required bitmask permission
		if !claims.HasPermission(metadata.Service, metadata.BitValue) {
			a.abortWithJSON(c, http.StatusForbidden, "permission_denied", "caller lacks required permission")
			return
		}

		c.Next()
	}
}

// authenticate extracts and verifies the JWT token from the request.
func (a *Authorizer) authenticate(c *gin.Context) (Claims, error) {
	header := c.GetHeader("Authorization")
	token, err := extractBearerToken(header)
	if err != nil {
		return Claims{}, err
	}

	claims, err := a.verifier.Verify(token)
	if err != nil {
		return Claims{}, err
	}

	// Store claims in context for later use
	c.Set(string(CtxAuthClaims), claims)
	return claims, nil
}

// abortAuthError handles authentication/authorization errors.
func (a *Authorizer) abortAuthError(c *gin.Context, err error) {
	status := authorizationErrorStatus(err)
	if status == http.StatusUnauthorized {
		a.abortWithJSON(c, status, "invalid_token", "authentication required")
		return
	}
	a.abortWithJSON(c, status, "authorization_failed", "authorization failed")
}

// abortWithJSON aborts the request with a JSON error response.
func (a *Authorizer) abortWithJSON(c *gin.Context, status int, code, message string) {
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

	parsedKey, err := parseRSAPublicKey(pubKey)
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
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
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
		ServicePermissions: decodeServicePermissions(svcPermRaw),
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

// decodeServicePermissions decodes service permissions from the token claim.
func decodeServicePermissions(raw string) map[string]int64 {
	perms := make(map[string]int64)
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
		maskStr := strings.TrimSpace(parts[1])
		if maskStr == "" {
			continue
		}
		mask, err := strconv.ParseInt(maskStr, 36, 64)
		if err != nil {
			continue
		}
		perms[serviceKey] = mask
	}
	return perms
}

// parseRSAPublicKey parses an RSA public key from a base64-encoded PEM string.
func parseRSAPublicKey(pubKeyBase64 string) (interface{}, error) {
	publicKey, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}
	return jwt.ParseRSAPublicKeyFromPEM(publicKey)
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
