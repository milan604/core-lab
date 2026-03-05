package auth

import "strings"

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
	if c.Raw == nil {
		return ""
	}

	value, ok := c.Raw["tenant_id"]
	if !ok {
		return ""
	}

	tenantID, ok := value.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(tenantID)
}

// TenantStatus returns the tenant_status from the token claims, if present.
// Returns empty string if not set (platform-level user or legacy token without the claim).
func (c Claims) TenantStatus() string {
	if c.Raw == nil {
		return ""
	}
	value, ok := c.Raw["tenant_status"]
	if !ok {
		return ""
	}
	status, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(status)
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
