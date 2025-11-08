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
