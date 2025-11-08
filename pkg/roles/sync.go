package roles

import (
	"context"
	"fmt"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/http"
	"github.com/milan604/core-lab/pkg/logger"
	"github.com/milan604/core-lab/pkg/permissions"
)

// Sync validates role definitions by checking if role IDs exist in Sentinel
// This is the main function that validates role IDs
// Similar to permissions.Bootstrap, it creates HTTP client internally and calls Sentinel APIs
func Sync(ctx context.Context, definitions []Definition, cfg *config.Config, log logger.LogManager) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if len(definitions) == 0 {
		log.WarnFCtx(ctx, "No role definitions provided, skipping roles validation")
		return nil
	}

	log.InfoFCtx(ctx, "Starting roles validation for %d role definitions...", len(definitions))

	// Step 1: Verify role definitions
	validatedRoles := make([]*Definition, 0, len(definitions))
	for i := range definitions {
		roleDef := &definitions[i]
		if !roleDef.IsValid() {
			log.WarnFCtx(ctx, "Skipping invalid role definition: %s", roleDef.Name)
			continue
		}
		validatedRoles = append(validatedRoles, roleDef)
	}

	log.InfoFCtx(ctx, "Verified %d role definitions", len(validatedRoles))

	// Step 2: Create HTTP client for Sentinel service (similar to permissions package)
	sentinelURL := cfg.GetString("SentinelServiceEndpoint")

	// Create HTTP client with service token authentication (similar to permissions package)
	// This uses http.NewClientWithServiceToken internally, which handles service token authentication
	httpClient, err := http.NewClientWithServiceToken(log, cfg)
	if err != nil {
		log.ErrorFCtx(ctx, "Failed to create HTTP client with service token: %v", err)
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Step 3: Validate role IDs in Sentinel (bulk validation)
	roleIDs := make([]string, 0, len(validatedRoles))
	for _, roleDef := range validatedRoles {
		roleIDs = append(roleIDs, roleDef.RoleID)
	}

	if err := validateRoleIDs(ctx, roleIDs, sentinelURL, httpClient, log); err != nil {
		log.ErrorFCtx(ctx, "Failed to validate roles in Sentinel: %v", err)
		return fmt.Errorf("failed to validate roles: %w", err)
	}

	log.InfoFCtx(ctx, "Roles validation completed successfully. Validated %d roles", len(validatedRoles))

	// Assign permissions to roles
	for _, roleDef := range validatedRoles {
		if err := assignPermissionsToRole(ctx, roleDef.RoleID, roleDef.Permissions, sentinelURL, httpClient, log); err != nil {
			log.ErrorFCtx(ctx, "Failed to assign permissions to role %s in Sentinel: %v", roleDef.RoleID, err)
			return fmt.Errorf("failed to assign permissions to role %s: %w", roleDef.RoleID, err)
		}
	}

	log.InfoFCtx(ctx, "Default permissions assigned to native roles successfully")

	return nil
}

// validateRoleIDs validates that role IDs exist in Sentinel using bulk API
func validateRoleIDs(ctx context.Context, roleIDs []string, sentinelURL string, httpClient *http.Client, log logger.LogManager) error {
	if len(roleIDs) == 0 {
		return nil
	}

	// Request structure for bulk role lookup
	type GetRolesByIDsRequest struct {
		RoleIDs []string `json:"role_ids" binding:"required,min=1,dive,uuid"`
	}

	// Response structure
	type RoleResponse struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Native      bool   `json:"native"`
		Status      string `json:"status"`
	}

	type GetRolesByIDsResponse []RoleResponse

	request := GetRolesByIDsRequest{
		RoleIDs: roleIDs,
	}

	var response GetRolesByIDsResponse
	url := sentinelURL + "/api/v1/roles/bulk"

	if err := httpClient.PostJSON(ctx, url, request, &response); err != nil {
		log.ErrorFCtx(ctx, "Failed to get roles from Sentinel: %v", err)
		return fmt.Errorf("sentinel service get roles: %w", err)
	}

	// Check if all role IDs were found
	foundRoleIDs := make(map[string]bool)
	for _, role := range response {
		foundRoleIDs[role.ID] = true
	}

	var missingRoles []string
	for _, roleID := range roleIDs {
		if !foundRoleIDs[roleID] {
			missingRoles = append(missingRoles, roleID)
		}
	}

	if len(missingRoles) > 0 {
		return fmt.Errorf("roles not found in Sentinel: %v", missingRoles)
	}

	log.InfoFCtx(ctx, "Successfully validated %d roles in Sentinel", len(roleIDs))
	return nil
}

// getPermissionsByCode gets permission IDs from Sentinel using permission codes
func getPermissionsByCode(ctx context.Context, codes []string, sentinelURL string, httpClient *http.Client, log logger.LogManager) ([]string, error) {
	if len(codes) == 0 {
		return []string{}, nil
	}

	// Request structure
	type GetPermissionsByCodesRequest struct {
		Codes []string `json:"codes" binding:"required,min=1"`
	}

	// Response structure
	type PermissionResponse struct {
		ID   string `json:"id"`
		Code string `json:"code"`
	}

	type GetPermissionsByCodesResponse []PermissionResponse

	request := GetPermissionsByCodesRequest{
		Codes: codes,
	}

	var response GetPermissionsByCodesResponse
	url := sentinelURL + "/api/v1/permissions/by-codes"

	if err := httpClient.PostJSON(ctx, url, request, &response); err != nil {
		log.ErrorFCtx(ctx, "Failed to get permissions from Sentinel: %v", err)
		return nil, fmt.Errorf("sentinel service get permissions: %w", err)
	}

	// Extract permission IDs
	permissionIDs := make([]string, 0, len(response))
	for _, perm := range response {
		permissionIDs = append(permissionIDs, perm.ID)
	}

	log.InfoFCtx(ctx, "Retrieved %d permission IDs from Sentinel", len(permissionIDs))
	return permissionIDs, nil
}

// assignPermissionsToRole assigns permissions to a role in Sentinel
func assignPermissionsToRole(ctx context.Context, roleID string, permissionRefs []permissions.Reference, sentinelURL string, httpClient *http.Client, log logger.LogManager) error {
	if len(permissionRefs) == 0 {
		log.InfoFCtx(ctx, "No permissions to assign to role %s", roleID)
		return nil
	}

	// Convert permission references to codes
	codes := make([]string, 0, len(permissionRefs))
	for _, ref := range permissionRefs {
		code := permissions.GenerateCode(ref.Service, ref.Category, ref.SubCategory)
		codes = append(codes, code)
	}

	// Get permission IDs from codes
	permissionIDs, err := getPermissionsByCode(ctx, codes, sentinelURL, httpClient, log)
	if err != nil {
		return fmt.Errorf("failed to get permissions by code: %w", err)
	}

	if len(permissionIDs) == 0 {
		log.WarnFCtx(ctx, "No permission IDs found for role %s", roleID)
		return nil
	}

	// Request structure
	type AssignPermissionsToRoleRequestBody struct {
		PermissionIDs []string `json:"permissions" binding:"required"`
	}

	// Response structure
	type AssignPermissionsToRoleResponse struct {
		Message string `json:"message"`
	}

	request := AssignPermissionsToRoleRequestBody{
		PermissionIDs: permissionIDs,
	}

	var response []AssignPermissionsToRoleResponse
	url := sentinelURL + "/api/v1/roles/" + roleID + "/permissions"

	if err := httpClient.PostJSON(ctx, url, request, &response); err != nil {
		log.ErrorFCtx(ctx, "Failed to assign permissions to role %s: %v", roleID, err)
		return fmt.Errorf("failed to assign permissions to role: %w", err)
	}

	log.InfoFCtx(ctx, "Successfully assigned %d permissions to role %s", len(permissionIDs), roleID)
	return nil
}
