package roles

import (
	"context"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/logger"
)

// Bootstrap bootstraps roles by syncing definitions to Sentinel
// This is similar to permissions.Bootstrap - it handles the entire flow internally
// Simply loops through the definitions and syncs them
func Bootstrap(ctx context.Context, definitions []Definition, cfg *config.Config, log logger.LogManager) error {
	// Delegate to Sync function which handles everything
	return Sync(ctx, definitions, cfg, log)
}

