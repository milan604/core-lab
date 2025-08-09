package keycloak

// ExtractRoles flattens Keycloak realm and resource roles from generic claims.
// It looks into `realm_access.roles` and `resource_access.<client>.roles`.
func ExtractRoles(claims map[string]any) []string {
	out := make([]string, 0)
	// realm roles
	if ra, ok := claims["realm_access"].(map[string]any); ok {
		if roles, ok := ra["roles"].([]any); ok {
			for _, r := range roles {
				if s, ok := r.(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	// client roles
	if res, ok := claims["resource_access"].(map[string]any); ok {
		for _, v := range res {
			if m, ok := v.(map[string]any); ok {
				if roles, ok := m["roles"].([]any); ok {
					for _, r := range roles {
						if s, ok := r.(string); ok {
							out = append(out, s)
						}
					}
				}
			}
		}
	}
	return out
}

// HasRealmRole returns true if the realm role exists.
func HasRealmRole(claims map[string]any, role string) bool {
	if ra, ok := claims["realm_access"].(map[string]any); ok {
		if roles, ok := ra["roles"].([]any); ok {
			for _, r := range roles {
				if s, ok := r.(string); ok && s == role {
					return true
				}
			}
		}
	}
	return false
}

// HasClientRole returns true if the client role exists for the given client ID.
func HasClientRole(claims map[string]any, clientID, role string) bool {
	if res, ok := claims["resource_access"].(map[string]any); ok {
		if m, ok := res[clientID].(map[string]any); ok {
			if roles, ok := m["roles"].([]any); ok {
				for _, r := range roles {
					if s, ok := r.(string); ok && s == role {
						return true
					}
				}
			}
		}
	}
	return false
}
