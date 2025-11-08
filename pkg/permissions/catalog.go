package permissions

import (
	"strings"
)

// Reference represents a permission reference with service, category, and subcategory.
type Reference struct {
	Service     string
	Category    string
	SubCategory string
}

// Code generates a permission code from the reference.
func (r Reference) Code() string {
	return GenerateCode(r.Service, r.Category, r.SubCategory)
}

// Definition represents a permission definition with its reference and metadata.
type Definition struct {
	Reference   Reference
	Name        string
	Description string
}

// Catalog manages a collection of permission definitions.
type Catalog struct {
	definitions []Definition
	byName      map[string]Definition
	byCode      map[string]Definition
}

// NewCatalog creates a new permission catalog from definitions.
func NewCatalog(definitions []Definition) *Catalog {
	catalog := &Catalog{
		definitions: definitions,
		byName:      make(map[string]Definition, len(definitions)),
		byCode:      make(map[string]Definition, len(definitions)),
	}

	for _, def := range definitions {
		catalog.byName[def.Name] = def
		catalog.byCode[def.Reference.Code()] = def
	}

	return catalog
}

// All returns all permission definitions.
func (c *Catalog) All() []Definition {
	return c.definitions
}

// ByName retrieves a permission definition by name.
func (c *Catalog) ByName(name string) (Definition, bool) {
	def, ok := c.byName[name]
	return def, ok
}

// ByCode retrieves a permission definition by code.
func (c *Catalog) ByCode(code string) (Definition, bool) {
	def, ok := c.byCode[code]
	return def, ok
}

// Codes returns all permission codes.
func (c *Catalog) Codes() []string {
	codes := make([]string, 0, len(c.definitions))
	for _, def := range c.definitions {
		codes = append(codes, def.Reference.Code())
	}
	return codes
}

// Count returns the number of permissions in the catalog.
func (c *Catalog) Count() int {
	return len(c.definitions)
}

// GenerateCode generates a permission code from service, category, and subcategory.
func GenerateCode(service, category, subCategory string) string {
	short := func(s string) string {
		ns := normalize(s)
		if len(ns) == 0 {
			return ""
		}
		if len(ns) >= 3 {
			return strings.ToUpper(ns[:3])
		}
		return strings.ToUpper(ns)
	}
	parts := make([]string, 0, 3)
	if v := short(service); v != "" {
		parts = append(parts, v)
	}
	if v := short(category); v != "" {
		parts = append(parts, v)
	}
	if v := short(subCategory); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, "-")
}

func normalize(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
}
