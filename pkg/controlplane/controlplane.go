package controlplane

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultInternalAudience            = "platform-internal"
	DefaultConfigAudience              = "platform.config"
	DefaultQuotaAudience               = "platform.quota"
	DefaultEntitlementsAudience        = "platform.entitlements"
	DefaultServiceRegistrationAudience = "platform.service-clients"
	DefaultAuthorizationAudience       = "platform.authorization"

	// KeyBaseURL is the preferred config key for the standalone control-plane endpoint.
	KeyBaseURL = "PlatformControlPlaneEndpoint"
	// LegacyKeyBaseURL is the backward-compatible Sentinel endpoint key.
	LegacyKeyBaseURL = "SentinelServiceEndpoint"

	KeyOIDCDiscoveryURL       = "PlatformOIDCDiscoveryURL"
	LegacyKeyOIDCDiscoveryURL = "SentinelOIDCDiscoveryURL"

	KeyJWKSURL       = "PlatformJWKSURL"
	LegacyKeyJWKSURL = "SentinelJWKSURL"

	KeyJWKSCacheTTLSeconds       = "PlatformJWKSCacheTTLSeconds"
	LegacyKeyJWKSCacheTTLSeconds = "SentinelJWKSCacheTTLSeconds"

	KeyTokenIssuer       = "PlatformTokenIssuer"
	LegacyKeyTokenIssuer = "SentinelTokenIssuer"

	KeyTokenAudience       = "PlatformTokenAudience"
	LegacyKeyTokenAudience = "SentinelTokenAudience"

	KeyInternalAudience       = "PlatformInternalAudience"
	LegacyKeyInternalAudience = "SentinelInternalAudience"

	KeyConfigAudience       = "PlatformConfigAudience"
	LegacyKeyConfigAudience = "SentinelConfigAudience"

	KeyQuotaAudience       = "PlatformQuotaAudience"
	LegacyKeyQuotaAudience = "SentinelQuotaAudience"

	KeyEntitlementsAudience       = "PlatformEntitlementsAudience"
	LegacyKeyEntitlementsAudience = "SentinelEntitlementsAudience"

	KeyServiceRegistrationAudience       = "PlatformServiceRegistrationAudience"
	LegacyKeyServiceRegistrationAudience = "SentinelServiceRegistrationAudience"

	KeyAuthorizationAudience       = "PlatformAuthorizationAudience"
	LegacyKeyAuthorizationAudience = "SentinelAuthorizationAudience"

	KeyServiceID     = "PlatformServiceID"
	KeyServiceAPIKey = "PlatformServiceAPIKey"

	KeyServiceTokenScope       = "PlatformServiceTokenScope"
	LegacyKeyServiceTokenScope = "SentinelServiceTokenScope"

	KeyMTLSCertFile       = "PlatformMTLSCertFile"
	LegacyKeyMTLSCertFile = "SentinelMTLSCertFile"

	KeyMTLSKeyFile       = "PlatformMTLSKeyFile"
	LegacyKeyMTLSKeyFile = "SentinelMTLSKeyFile"

	KeyMTLSCAFile       = "PlatformMTLSCAFile"
	LegacyKeyMTLSCAFile = "SentinelMTLSCAFile"
)

type StringGetter interface {
	GetString(key string) string
}

type ConfigGetter interface {
	StringGetter
	GetStringSlice(key string) []string
}

type ServiceTokenConfig struct {
	BaseURL   string
	ServiceID string
	APIKey    string
	Scope     string
	Audience  []string
	MTLS      MTLSConfig
}

type API struct {
	BaseURL string
}

type MTLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

func NormalizeBaseURL(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/")
}

// NormalizeLegacySentinelBaseURL keeps compatibility with older configs that
// point at the host root and expect "/sentinel" to be appended automatically.
func NormalizeLegacySentinelBaseURL(raw string) string {
	base := NormalizeBaseURL(raw)
	if base == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(base), "/sentinel") {
		return base
	}
	return base + "/sentinel"
}

func ResolveBaseURL(cfg StringGetter) string {
	if cfg == nil {
		return ""
	}
	if base := NormalizeBaseURL(cfg.GetString(KeyBaseURL)); base != "" {
		return base
	}
	return NormalizeLegacySentinelBaseURL(cfg.GetString(LegacyKeyBaseURL))
}

func ResolveOIDCDiscoveryURL(cfg StringGetter) string {
	return firstString(cfg, KeyOIDCDiscoveryURL, LegacyKeyOIDCDiscoveryURL)
}

func ResolveJWKSURL(cfg StringGetter) string {
	return firstString(cfg, KeyJWKSURL, LegacyKeyJWKSURL)
}

func ResolveJWKSCacheTTL(cfg StringGetter, fallback time.Duration) time.Duration {
	raw := firstString(cfg, KeyJWKSCacheTTLSeconds, LegacyKeyJWKSCacheTTLSeconds)
	if raw == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func ResolveTokenIssuer(cfg StringGetter) string {
	return firstString(cfg, KeyTokenIssuer, LegacyKeyTokenIssuer)
}

func ResolveTokenAudienceFromStringGetter(cfg StringGetter) []string {
	return parseAudience(firstString(cfg, KeyTokenAudience, LegacyKeyTokenAudience))
}

func ResolveTokenAudience(cfg ConfigGetter) []string {
	if cfg == nil {
		return nil
	}
	if audience := parseAudienceList(cfg.GetStringSlice(KeyTokenAudience)); len(audience) > 0 {
		return audience
	}
	if audience := parseAudience(cfg.GetString(KeyTokenAudience)); len(audience) > 0 {
		return audience
	}
	if audience := parseAudienceList(cfg.GetStringSlice(LegacyKeyTokenAudience)); len(audience) > 0 {
		return audience
	}
	return parseAudience(cfg.GetString(LegacyKeyTokenAudience))
}

func ResolveInternalAudience(cfg ConfigGetter) []string {
	if cfg == nil {
		return []string{DefaultInternalAudience}
	}
	if audience := parseAudienceList(cfg.GetStringSlice(KeyInternalAudience)); len(audience) > 0 {
		return audience
	}
	if audience := parseAudience(cfg.GetString(KeyInternalAudience)); len(audience) > 0 {
		return audience
	}
	if audience := parseAudienceList(cfg.GetStringSlice(LegacyKeyInternalAudience)); len(audience) > 0 {
		return audience
	}
	if audience := parseAudience(cfg.GetString(LegacyKeyInternalAudience)); len(audience) > 0 {
		return audience
	}
	return []string{DefaultInternalAudience}
}

func ResolveConfigAudience(cfg ConfigGetter) []string {
	return resolveAudienceWithDefault(cfg, []string{DefaultConfigAudience}, KeyConfigAudience, LegacyKeyConfigAudience)
}

func ResolveQuotaAudience(cfg ConfigGetter) []string {
	return resolveAudienceWithDefault(cfg, []string{DefaultQuotaAudience}, KeyQuotaAudience, LegacyKeyQuotaAudience)
}

func ResolveEntitlementsAudience(cfg ConfigGetter) []string {
	return resolveAudienceWithDefault(cfg, []string{DefaultEntitlementsAudience}, KeyEntitlementsAudience, LegacyKeyEntitlementsAudience)
}

func ResolveServiceRegistrationAudience(cfg ConfigGetter) []string {
	return resolveAudienceWithDefault(cfg, []string{DefaultServiceRegistrationAudience}, KeyServiceRegistrationAudience, LegacyKeyServiceRegistrationAudience)
}

func ResolveAuthorizationAudience(cfg ConfigGetter) []string {
	return resolveAudienceWithDefault(cfg, []string{DefaultAuthorizationAudience}, KeyAuthorizationAudience, LegacyKeyAuthorizationAudience)
}

func ResolveServiceID(cfg StringGetter) string {
	return firstString(cfg, KeyServiceID)
}

func ResolveServiceAPIKey(cfg StringGetter) string {
	return firstString(cfg, KeyServiceAPIKey)
}

func ResolveServiceTokenScope(cfg StringGetter) string {
	return firstString(cfg, KeyServiceTokenScope, LegacyKeyServiceTokenScope)
}

func ResolveServiceTokenConfig(cfg ConfigGetter) ServiceTokenConfig {
	return ServiceTokenConfig{
		BaseURL:   ResolveBaseURL(cfg),
		ServiceID: ResolveServiceID(cfg),
		APIKey:    ResolveServiceAPIKey(cfg),
		Scope:     ResolveServiceTokenScope(cfg),
		Audience:  ResolveTokenAudience(cfg),
		MTLS:      ResolveMTLSConfig(cfg),
	}
}

func ResolveMTLSConfig(cfg StringGetter) MTLSConfig {
	if cfg == nil {
		return MTLSConfig{}
	}
	return MTLSConfig{
		CertFile: firstString(cfg, KeyMTLSCertFile, LegacyKeyMTLSCertFile),
		KeyFile:  firstString(cfg, KeyMTLSKeyFile, LegacyKeyMTLSKeyFile),
		CAFile:   firstString(cfg, KeyMTLSCAFile, LegacyKeyMTLSCAFile),
	}
}

func (c ServiceTokenConfig) MissingRequiredFields() []string {
	var missing []string
	if strings.TrimSpace(c.BaseURL) == "" {
		missing = append(missing, KeyBaseURL+" or "+LegacyKeyBaseURL)
	}
	if strings.TrimSpace(c.ServiceID) == "" {
		missing = append(missing, KeyServiceID)
	}
	if strings.TrimSpace(c.APIKey) == "" {
		missing = append(missing, KeyServiceAPIKey)
	}
	missing = append(missing, c.MTLS.MissingRequiredFields()...)
	return missing
}

func (c MTLSConfig) MissingRequiredFields() []string {
	var missing []string
	if strings.TrimSpace(c.CertFile) == "" {
		missing = append(missing, KeyMTLSCertFile+" or "+LegacyKeyMTLSCertFile)
	}
	if strings.TrimSpace(c.KeyFile) == "" {
		missing = append(missing, KeyMTLSKeyFile+" or "+LegacyKeyMTLSKeyFile)
	}
	if strings.TrimSpace(c.CAFile) == "" {
		missing = append(missing, KeyMTLSCAFile+" or "+LegacyKeyMTLSCAFile)
	}
	return missing
}

func APIFromConfig(cfg StringGetter) API {
	return API{BaseURL: ResolveBaseURL(cfg)}
}

func (a API) Valid() bool {
	return strings.TrimSpace(a.BaseURL) != ""
}

func (a API) ServiceTokenURL() string {
	return a.BaseURL + "/internal/api/v1/service-token"
}

func (a API) PermissionBulkURL() string {
	return a.BaseURL + "/api/v1/permissions/bulk"
}

func (a API) PermissionCatalogURL() string {
	return a.BaseURL + "/api/v1/permissions/bitmask"
}

func (a API) PermissionByCodesURL() string {
	return a.BaseURL + "/api/v1/permissions/by-codes"
}

func (a API) RolesBulkURL() string {
	return a.BaseURL + "/api/v1/roles/bulk"
}

func (a API) RolePermissionsURL(roleID string) string {
	return a.BaseURL + "/api/v1/roles/" + url.PathEscape(strings.TrimSpace(roleID)) + "/permissions"
}

func (a API) QuotaCheckURL() string {
	return a.BaseURL + "/internal/api/v1/quota/check"
}

func (a API) AuthorizationDecisionURL() string {
	return a.BaseURL + "/internal/api/v1/authz/decide"
}

func (a API) ConfigPublishURL() string {
	return a.BaseURL + "/internal/api/v1/config/publish"
}

func (a API) ConfigResolveURL() string {
	return a.BaseURL + "/internal/api/v1/config/resolve"
}

func (a API) ConfigWatchURL() string {
	return a.BaseURL + "/internal/api/v1/config/watch"
}

func (a API) OIDCDiscoveryURL() string {
	return a.BaseURL + "/.well-known/openid-configuration"
}

func (a API) JWKSURL() string {
	return a.BaseURL + "/.well-known/jwks.json"
}

func firstString(cfg StringGetter, keys ...string) string {
	if cfg == nil {
		return ""
	}
	for _, key := range keys {
		value := strings.TrimSpace(cfg.GetString(strings.TrimSpace(key)))
		if value != "" {
			return value
		}
	}
	return ""
}

func resolveAudienceWithDefault(cfg ConfigGetter, fallback []string, keys ...string) []string {
	if cfg == nil {
		return fallback
	}
	for _, key := range keys {
		if audience := parseAudienceList(cfg.GetStringSlice(key)); len(audience) > 0 {
			return audience
		}
		if audience := parseAudience(cfg.GetString(key)); len(audience) > 0 {
			return audience
		}
	}
	return fallback
}

func parseAudience(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseAudienceList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
