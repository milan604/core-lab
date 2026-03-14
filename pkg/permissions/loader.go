package permissions

import (
	"context"
	"fmt"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/controlplane"
	"github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"
)

// LoaderFromHTTP creates a loader function that fetches permissions from the sentinel service.
// Since permission APIs are standardized, this makes HTTP calls directly using http.NewClientWithServiceToken.
func LoaderFromHTTP(cfg *config.Config, log logger.LogManager) Loader {
	return func(ctx context.Context) (map[string]Metadata, error) {
		if cfg == nil {
			return nil, fmt.Errorf("config not configured")
		}

		if log == nil {
			return nil, fmt.Errorf("logger not configured")
		}

		api := controlplane.APIFromConfig(cfg)
		if !api.Valid() {
			return nil, fmt.Errorf("%s or %s not configured", controlplane.KeyBaseURL, controlplane.LegacyKeyBaseURL)
		}

		// Create HTTP client with token provider using http package directly
		httpClient, err := http.NewClientWithServiceToken(log, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP client with token provider: %w", err)
		}

		var catalogResponse StandardCatalogResponse

		err = httpClient.GetJSON(ctx, api.PermissionCatalogURL(), &catalogResponse)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch permission catalog: %w", err)
		}

		// Convert catalog response to internal metadata map
		metadata := make(map[string]Metadata, 0)
		for service, serviceCatalog := range catalogResponse.Services {
			for code, perm := range serviceCatalog.Permissions {
				metadata[code] = Metadata{
					ID:       perm.ID,
					Service:  service,
					BitValue: perm.BitValue,
				}
			}
		}

		return metadata, nil
	}
}
