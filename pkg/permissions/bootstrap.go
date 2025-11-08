package permissions

import (
	"context"
	"fmt"
)

// Config provides configuration for the permissions package.
// Services only need to provide this - no API methods to implement!
type Config interface {
	// GetString gets a string value from config
	GetString(key string) string
}

// HTTPClient is the interface for making HTTP requests.
// Services can pass core-lab's http.Client directly.
type HTTPClient interface {
	PostJSON(ctx context.Context, url string, body interface{}, response interface{}) error
	GetJSON(ctx context.Context, url string, response interface{}) error
}

// Logger is the interface for logging.
type Logger interface {
	ErrorF(format string, args ...interface{})
	InfoF(format string, args ...interface{})
}

// HTTPClientFactory creates HTTP clients with token provider.
// Since token provider is the same for all services, this is handled internally.
// Services can pass a function that creates core-lab's http.Client with token provider.
type HTTPClientFactory interface {
	// NewClientWithTokenProvider creates a new HTTP client with token provider configured.
	// The token provider should fetch tokens from the sentinel service.
	NewClientWithTokenProvider(ctx context.Context) (HTTPClient, error)
}

// Bootstrap synchronizes permissions with the sentinel service and loads them into the store.
// Since permission APIs and token provider are standardized, this function makes HTTP calls directly.
// Services only need to provide config, logger, and HTTP client factory - no API methods or token providers needed!
func Bootstrap(ctx context.Context, catalog *Catalog, cfg Config, logger Logger, clientFactory HTTPClientFactory, store *Store) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if cfg == nil {
		return fmt.Errorf("config not configured")
	}

	if logger == nil {
		return fmt.Errorf("logger not configured")
	}

	if clientFactory == nil {
		return fmt.Errorf("HTTP client factory not configured")
	}

	if catalog == nil {
		return fmt.Errorf("permission catalog not configured")
	}

	// Get sentinel service URL from config
	sentinelURL := cfg.GetString("SentinelServiceEndpoint")
	if sentinelURL == "" {
		return fmt.Errorf("SentinelServiceEndpoint not configured")
	}

	// Create HTTP client with token provider (token provider created internally)
	httpClient, err := clientFactory.NewClientWithTokenProvider(ctx)
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
			SubCategory: def.Reference.SubCategory,
		})
	}

	// Make HTTP call directly to sentinel service
	url := fmt.Sprintf("%s/api/v1/permissions/bulk", sentinelURL)
	var response struct {
		Permissions []StandardCreateResponseEntry `json:"permissions"`
	}

	err := httpClient.PostJSON(ctx, url, requests, &response)
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
