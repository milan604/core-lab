package http

import (
	"strings"

	"github.com/milan604/core-lab/pkg/config"
)

// InternalAuthKeys returns the configured internal auth secrets in priority order.
// InternalAdminKey is the canonical control-plane key; additional keys are kept
// for backward-compatible rollouts of older service-to-service contracts.
func InternalAuthKeys(cfg *config.Config, extraKeys ...string) []string {
	if cfg == nil {
		return nil
	}

	candidates := make([]string, 0, 1+len(extraKeys))
	seen := make(map[string]struct{}, 1+len(extraKeys))

	appendKey := func(key string) {
		value := strings.TrimSpace(cfg.GetString(strings.TrimSpace(key)))
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	appendKey("InternalAdminKey")
	for _, key := range extraKeys {
		appendKey(key)
	}

	return candidates
}

func InternalAuthKey(cfg *config.Config, extraKeys ...string) string {
	keys := InternalAuthKeys(cfg, extraKeys...)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}
