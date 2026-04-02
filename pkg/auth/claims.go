package auth

import (
	"strconv"
	"strings"
)

// Claims captures the verified JWT context extracted from incoming requests.
type Claims struct {
	Subject            string
	IdentityID         string
	RoleID             string
	TokenUse           string
	ServicePermissions map[string][]int64 // Multiple ranges per service: [range0, range1, range2, ...]
	Raw                map[string]any
}

// IsServiceToken reports whether the token represents a service credential.
func (c Claims) IsServiceToken() bool {
	return strings.EqualFold(strings.TrimSpace(c.TokenUse), "service")
}

// TenantID returns the tenant_id from the token claims, if present.
func (c Claims) TenantID() string {
	return c.ClaimString("tenant_id")
}

// TenantStatus returns the tenant_status from the token claims, if present.
// Returns empty string if not set (platform-level user or legacy token without the claim).
func (c Claims) TenantStatus() string {
	return c.ClaimString("tenant_status")
}

// TenantMembershipStatus returns the tenant_membership_status from the token claims, if present.
func (c Claims) TenantMembershipStatus() string {
	return c.ClaimString("tenant_membership_status")
}

// ServiceID returns the calling service identifier for service tokens, if present.
func (c Claims) ServiceID() string {
	return c.ClaimString("service_id")
}

// IsSuperAdmin reports whether the verified caller is marked as a global super admin.
func (c Claims) IsSuperAdmin() bool {
	if c.Raw == nil {
		return false
	}
	rawValue, ok := c.Raw["is_super_admin"]
	if !ok || rawValue == nil {
		return false
	}

	switch value := rawValue.(type) {
	case bool:
		return value
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		return err == nil && parsed
	default:
		return false
	}
}

// UserID returns the authenticated user identifier, preferring identity_id over sub.
func (c Claims) UserID() string {
	if userID := strings.TrimSpace(c.IdentityID); userID != "" {
		return userID
	}
	return strings.TrimSpace(c.Subject)
}

// ClaimString returns a trimmed string claim from the raw claim set.
func (c Claims) ClaimString(key string) string {
	if c.Raw == nil {
		return ""
	}
	value, ok := c.Raw[key]
	if !ok {
		return ""
	}
	stringValue, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringValue)
}

// HasPermission evaluates whether the caller holds the permission for the given service.
// bitValue is a sequential position (0, 1, 2, 3, ...) that gets mapped to a range and position within that range.
func (c Claims) HasPermission(service string, bitValue int64) bool {
	if bitValue < 0 {
		return false
	}
	ranges := c.ServicePermissions[strings.ToLower(strings.TrimSpace(service))]
	if len(ranges) == 0 {
		return false
	}

	// Calculate which range and position within that range
	rangeIndex := bitValue / 63
	positionInRange := bitValue % 63
	bitMask := int64(1) << positionInRange

	// Check if the range exists and the bit is set
	if rangeIndex < int64(len(ranges)) {
		return (ranges[rangeIndex] & bitMask) == bitMask
	}

	return false
}
