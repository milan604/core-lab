package permissions

// Request models for creating permissions

// CreateRequest represents a request to create a permission in the sentinel service.
type CreateRequest struct {
	Name         string   `json:"name" binding:"required"`
	Service      string   `json:"service" binding:"required"`
	Category     string   `json:"category" binding:"required"`
	SubCategory  string   `json:"sub_category" binding:"required"`
	Description  string   `json:"description" binding:"required"`
	FeatureFlags []string `json:"feature_flags,omitempty"`
}

// Response models from the sentinel service

// CreateResponse represents the response from creating permissions.
type CreateResponse struct {
	Permissions []CreateResponseEntry `json:"permissions"`
}

// CreateResponseEntry represents a single permission entry in the create response.
type CreateResponseEntry struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Service  string `json:"service"`
	BitValue int64  `json:"bit_value"`
}

// CatalogResponse represents the permission catalog response from the sentinel service.
type CatalogResponse struct {
	Services map[string]ServiceCatalog `json:"services"`
}

// ServiceCatalog represents permissions for a service in the catalog.
type ServiceCatalog struct {
	Permissions map[string]CatalogEntry      `json:"permissions"`
	Groups      map[string]GroupCatalogEntry `json:"groups,omitempty"`
}

// CatalogEntry represents a permission entry in the catalog.
type CatalogEntry struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	SubCategory  string   `json:"sub_category"`
	Code         string   `json:"code"`
	BitValue     int64    `json:"bit_value"`
	FeatureFlags []string `json:"feature_flags,omitempty"`
}

// GroupCatalogEntry represents a permission group entry in the catalog.
type GroupCatalogEntry struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	Code          string   `json:"code"`
	CategoryCode  string   `json:"category_code"`
	Bitmask       int64    `json:"bitmask"`
	PermissionIDs []string `json:"permission_ids"`
}

// Internal models for the package

// PermissionRequest is an alias for CreateRequest for backward compatibility.
type PermissionRequest = CreateRequest

// PermissionResponse is an alias for CreateResponse for backward compatibility.
type PermissionResponse = CreateResponse

// PermissionEntry is an alias for CreateResponseEntry for backward compatibility.
type PermissionEntry = CreateResponseEntry

// ServicePermissions is an alias for ServiceCatalog for backward compatibility.
type ServicePermissions = ServiceCatalog

// PermissionCatalogEntry is an alias for CatalogEntry for backward compatibility.
type PermissionCatalogEntry = CatalogEntry

// Standard API models - these match the structure used by all services
// since permission create and catalog fetch APIs are always the same.
// These are used internally by the permissions package when making HTTP calls.

// StandardCreateRequest represents the standard request structure for creating permissions.
// This matches the structure used by all services.
type StandardCreateRequest struct {
	Name         string   `json:"name" binding:"required"`
	Service      string   `json:"service" binding:"required"`
	Category     string   `json:"category" binding:"required"`
	SubCategory  string   `json:"sub_category" binding:"required"`
	FeatureFlags []string `json:"feature_flags,omitempty"`
	Description  string   `json:"description" binding:"required"`
}

// StandardCreateResponse represents the standard response structure from creating permissions.
// This matches the structure returned by the sentinel service.
type StandardCreateResponse struct {
	Permissions []StandardCreateResponseEntry `json:"permissions"`
}

// StandardCreateResponseEntry represents a single permission entry in the create response.
type StandardCreateResponseEntry struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Service  string `json:"service"`
	BitValue int64  `json:"bit_value"`
}

// StandardCatalogResponse represents the standard catalog response structure.
// This matches the structure returned by the sentinel service.
type StandardCatalogResponse struct {
	Services map[string]StandardServiceCatalog `json:"services"`
}

// StandardServiceCatalog represents permissions for a service in the catalog.
type StandardServiceCatalog struct {
	Permissions map[string]StandardCatalogEntry      `json:"permissions"`
	Groups      map[string]StandardGroupCatalogEntry `json:"groups,omitempty"`
}

// StandardCatalogEntry represents a permission entry in the catalog.
type StandardCatalogEntry struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	SubCategory  string   `json:"sub_category"`
	Code         string   `json:"code"`
	BitValue     int64    `json:"bit_value"`
	FeatureFlags []string `json:"feature_flags,omitempty"`
}

// StandardGroupCatalogEntry represents a permission group entry in the catalog.
type StandardGroupCatalogEntry struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	Code          string   `json:"code"`
	CategoryCode  string   `json:"category_code"`
	Bitmask       int64    `json:"bitmask"`
	PermissionIDs []string `json:"permission_ids"`
}
