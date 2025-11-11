package permissions

// PermissionSource represents a source of permission data that can be converted to a Catalog.
// This interface allows the converter to work with any external permission structure.
type PermissionSource interface {
	GetName() string
	GetDescription() string
	GetService() string
	GetCategory() string
	GetAction() string
}

// FromDefinitions converts a slice of permission definitions to a Catalog.
// This is a generic converter that works with any permission definition structure.
func FromDefinitions(defs []Definition) *Catalog {
	return NewCatalog(defs)
}

// FromSource converts a map of permission sources to a Catalog.
// This is a generic converter that works with any permission source structure.
func FromSource[T PermissionSource](sources map[string]T) *Catalog {
	definitions := make([]Definition, 0, len(sources))

	for _, source := range sources {
		definitions = append(definitions, Definition{
			Reference: Reference{
				Service:     source.GetService(),
				Category:    source.GetCategory(),
				Action: source.GetAction(),
			},
			Name:        source.GetName(),
			Description: source.GetDescription(),
		})
	}

	return NewCatalog(definitions)
}

// FromSlice converts a slice of permission sources to a Catalog.
// This is a generic converter that works with any permission source structure.
func FromSlice[T PermissionSource](sources []T) *Catalog {
	definitions := make([]Definition, 0, len(sources))

	for _, source := range sources {
		definitions = append(definitions, Definition{
			Reference: Reference{
				Service:     source.GetService(),
				Category:    source.GetCategory(),
				Action: source.GetAction(),
			},
			Name:        source.GetName(),
			Description: source.GetDescription(),
		})
	}

	return NewCatalog(definitions)
}
