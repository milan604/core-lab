package roles

// Catalog represents a collection of role definitions with their permissions
type Catalog struct {
	definitions []Definition
}

// NewCatalog creates a new roles catalog from definitions
func NewCatalog(definitions []Definition) *Catalog {
	return &Catalog{
		definitions: definitions,
	}
}

// Definitions returns all role definitions in the catalog
func (c *Catalog) Definitions() []Definition {
	return c.definitions
}

// GetRoleByID returns a role definition by role ID
func (c *Catalog) GetRoleByID(roleID string) *Definition {
	for i := range c.definitions {
		if c.definitions[i].RoleID == roleID {
			return &c.definitions[i]
		}
	}
	return nil
}

// GetAllRoleIDs returns all role IDs in the catalog
func (c *Catalog) GetAllRoleIDs() []string {
	ids := make([]string, 0, len(c.definitions))
	for _, def := range c.definitions {
		ids = append(ids, def.RoleID)
	}
	return ids
}

// Count returns the number of role definitions in the catalog
func (c *Catalog) Count() int {
	return len(c.definitions)
}
