package permissions

import (
	"context"
	"fmt"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"
)

// HTTPClient is the interface for making HTTP requests.
// Services can pass core-lab's http.Client directly.
type HTTPClient interface {
	PostJSON(ctx context.Context, url string, body interface{}, response interface{}) error
	GetJSON(ctx context.Context, url string, response interface{}) error
}

// Bootstrap synchronizes permissions with the sentinel service and loads them into the store.
// Since permission APIs and token provider are standardized, this function makes HTTP calls directly.
// Services only need to provide config and logger - no API methods or token providers needed!
// The function uses http.NewClientWithServiceToken directly from the http package.
func Bootstrap(ctx context.Context, catalog *Catalog, cfg *config.Config, log logger.LogManager, store *Store) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if cfg == nil {
		return fmt.Errorf("config not configured")
	}

	if log == nil {
		return fmt.Errorf("logger not configured")
	}

	if catalog == nil {
		return fmt.Errorf("permission catalog not configured")
	}

	// Get sentinel service URL from config
	sentinelURL := cfg.GetString("SentinelServiceEndpoint")
	if sentinelURL == "" {
		return fmt.Errorf("SentinelServiceEndpoint not configured")
	}

	// Create HTTP client with token provider using http package directly
	httpClient, err := http.NewClientWithServiceToken(log, cfg)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client with token provider: %w", err)
	}

	// Ensure permissions are created in sentinel service
	if err := ensurePermissions(ctx, catalog, sentinelURL, httpClient); err != nil {
		return fmt.Errorf("failed to ensure permissions: %w", err)
	}

	// Load permissions from sentinel service into the permission store
	if store != nil {
		if err := loadPermissions(ctx, sentinelURL, httpClient, store); err != nil {
			return fmt.Errorf("failed to load permissions: %w", err)
		}
	}

	return nil
}

// ensurePermissions creates permissions in the sentinel service if they don't exist.
// Makes HTTP call directly to the sentinel service.
func ensurePermissions(ctx context.Context, catalog *Catalog, sentinelURL string, httpClient HTTPClient) error {
	// Prepare bulk create request
	requests := make([]StandardCreateRequest, 0, catalog.Count())
	for _, def := range catalog.All() {
		requests = append(requests, StandardCreateRequest{
			Name:        def.Name,
			Description: def.Description,
			Service:     def.Reference.Service,
			Category:    def.Reference.Category,
			Action: def.Reference.Action,
		})
	}

	requestBody := map[string]interface{}{
		"permissions": requests,
	}

	// Make HTTP call directly to sentinel service
	url := fmt.Sprintf("%s/api/v1/permissions/bulk", sentinelURL)
	var response struct {
		Permissions []StandardCreateResponseEntry `json:"permissions"`
	}

	err := httpClient.PostJSON(ctx, url, requestBody, &response)
	if err != nil {
		return fmt.Errorf("failed to create permissions in sentinel service: %w", err)
	}

	return nil
}

// loadPermissions loads permissions from the sentinel service into the store.
// Makes HTTP call directly to the sentinel service.
func loadPermissions(ctx context.Context, sentinelURL string, httpClient HTTPClient, store *Store) error {
	// Make HTTP call directly to sentinel service
	url := fmt.Sprintf("%s/api/v1/permissions/bitmask", sentinelURL)
	var catalogResponse StandardCatalogResponse

	err := httpClient.GetJSON(ctx, url, &catalogResponse)
	if err != nil {
		return fmt.Errorf("failed to fetch permission catalog: %w", err)
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

	// Update store with fetched permissions
	store.Replace(metadata)

	return nil
}
