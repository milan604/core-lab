package auth

import "strings"

// Claims captures the verified JWT context extracted from incoming requests.
type Claims struct {
	Subject            string
	IdentityID         string
	RoleID             string
	TokenUse           string
	ServicePermissions map[string]int64 // service -> bitmask
	Raw                map[string]any
}

// IsServiceToken reports whether the token represents a service credential.
func (c Claims) IsServiceToken() bool {
	return strings.EqualFold(strings.TrimSpace(c.TokenUse), "service")
}

// HasPermission evaluates whether the caller holds the bitmask required for the given service.
func (c Claims) HasPermission(service string, bitValue int64) bool {
	if bitValue == 0 {
		return false
	}
	mask := c.ServicePermissions[strings.ToLower(strings.TrimSpace(service))]
	return mask&bitValue == bitValue
}
