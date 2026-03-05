package postgres

import (
	"fmt"

	"gorm.io/gorm"
)

// SetTenantContext sets the PostgreSQL session variable `app.current_tenant_id`
// for the current database transaction/connection. This is required for
// Row Level Security (RLS) policies to filter queries by tenant.
//
// Usage in a request handler:
//
//	tenantID, ok := auth.EnforceTenantScope(c, requestedTenantID)
//	if !ok { return }
//	tx := postgres.SetTenantContext(db.Client, tenantID)
//	tx.Where("...").Find(&results)
//
// For service tokens or admin operations where no tenant scoping is needed,
// pass an empty string to reset the session variable.
func SetTenantContext(db *gorm.DB, tenantID string) *gorm.DB {
	if tenantID == "" {
		// Reset to empty — RLS policies with current_setting(..., true) will return ''
		// which won't match any tenant_id, effectively denying access.
		// Use the platform role for admin/cross-tenant operations instead.
		return db.Exec("SET LOCAL app.current_tenant_id = ''")
	}
	return db.Exec(fmt.Sprintf("SET LOCAL app.current_tenant_id = '%s'", tenantID))
}

// WithTenantScope starts a transaction with the tenant context set.
// All queries within the returned *gorm.DB will be scoped to the tenant.
//
// Usage:
//
//	tx := postgres.WithTenantScope(db.Client, tenantID)
//	defer tx.Rollback()
//	tx.Where("status = ?", "active").Find(&items)
//	tx.Commit()
func WithTenantScope(db *gorm.DB, tenantID string) *gorm.DB {
	tx := db.Begin()
	if tx.Error != nil {
		return tx
	}
	SetTenantContext(tx, tenantID)
	return tx
}
