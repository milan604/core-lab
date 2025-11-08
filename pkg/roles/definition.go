package roles

import (
	"github.com/milan604/core-lab/pkg/permissions"
)

// Definition represents a role definition with its permissions
// This uses RoleID instead of Name to identify roles
type Definition struct {
	// RoleID is the ID of the role (UUID from Sentinel service)
	RoleID string

	// Name is the name of the role (e.g., "Admin", "Manager", "User")
	Name string

	// Permissions is the list of permission references assigned to this role
	Permissions []permissions.Reference
}

// PermissionCount returns the number of permissions assigned to this role
func (d *Definition) PermissionCount() int {
	return len(d.Permissions)
}

// IsValid checks if the role definition is valid
func (d *Definition) IsValid() bool {
	return d.RoleID != ""
}
