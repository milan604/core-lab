package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/milan604/core-lab/pkg/auth"
)

const (
	resolvedTenantIDContextKey tenantContextKey = "resolved_tenant_id"
	tenantIDContextKey         tenantContextKey = "tenant_id"
)

type tenantContextKey string

// TenantDB injects tenant-scoping SQL so services can share one request-layer
// contract for tenant-aware database access.
type TenantDB struct {
	DB *sql.DB
}

func NewTenantDB(db *sql.DB) *TenantDB {
	if db == nil {
		return nil
	}
	return &TenantDB{DB: db}
}

func ContextWithTenantID(ctx context.Context, tenantID string) context.Context {
	return coreauth.ContextWithTenantID(ctx, tenantID)
}

func (db *TenantDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, injectTenantScope(query, tenantIDFromContext(ctx)), args...)
}

func (db *TenantDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.DB.QueryRowContext(ctx, injectTenantScope(query, tenantIDFromContext(ctx)), args...)
}

func (db *TenantDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.DB.ExecContext(ctx, injectTenantScope(query, tenantIDFromContext(ctx)), args...)
}

func (db *TenantDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, tenantScopeStatement(tenantIDFromContext(ctx))); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func (db *TenantDB) Conn(ctx context.Context) (*sql.Conn, error) {
	return db.DB.Conn(ctx)
}

func tenantIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if tenantID, ok := coreauth.TenantIDFromContext(ctx); ok {
		return strings.TrimSpace(tenantID)
	}
	if claims, ok := ctx.Value(string(coreauth.CtxAuthClaims)).(coreauth.Claims); ok && !claims.IsServiceToken() {
		if tenantID := strings.TrimSpace(claims.TenantID()); tenantID != "" {
			return tenantID
		}
	}
	if c, ok := ctx.(*gin.Context); ok {
		if tenantID, ok := coreauth.GetTenantID(c); ok {
			return strings.TrimSpace(tenantID)
		}
		if claims, ok := coreauth.GetClaims(c); ok && !claims.IsServiceToken() {
			return strings.TrimSpace(claims.TenantID())
		}
	}
	return ""
}

func injectTenantScope(query string, tenantID string) string {
	cte := tenantScopeCTE(tenantID)
	insertAt := leadingWithClauseInsertIndex(query)
	if insertAt < 0 {
		return "WITH " + cte + " " + query
	}
	return query[:insertAt] + " " + cte + "," + query[insertAt:]
}

func tenantScopeCTE(tenantID string) string {
	return fmt.Sprintf("_tenant_scope AS (SELECT set_config('app.current_tenant_id', '%s', true))", escapeSQLLiteral(tenantID))
}

func tenantScopeStatement(tenantID string) string {
	return fmt.Sprintf("SELECT set_config('app.current_tenant_id', '%s', true)", escapeSQLLiteral(tenantID))
}

func escapeSQLLiteral(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "'", "''")
}

func leadingWithClauseInsertIndex(query string) int {
	index := skipLeadingTrivia(query, 0)
	if !matchKeyword(query, index, "WITH") {
		return -1
	}

	index += len("WITH")
	next := skipLeadingTrivia(query, index)
	if matchKeyword(query, next, "RECURSIVE") {
		return next + len("RECURSIVE")
	}

	return index
}

func skipLeadingTrivia(query string, start int) int {
	index := start
	for index < len(query) {
		switch {
		case isSQLWhitespace(query[index]):
			index++
		case strings.HasPrefix(query[index:], "--"):
			index += 2
			for index < len(query) && query[index] != '\n' {
				index++
			}
		case strings.HasPrefix(query[index:], "/*"):
			index += 2
			for index+1 < len(query) && query[index:index+2] != "*/" {
				index++
			}
			if index+1 < len(query) {
				index += 2
			}
		default:
			return index
		}
	}
	return index
}

func matchKeyword(query string, start int, keyword string) bool {
	if start < 0 || start+len(keyword) > len(query) {
		return false
	}
	if !strings.EqualFold(query[start:start+len(keyword)], keyword) {
		return false
	}
	if end := start + len(keyword); end < len(query) && isSQLIdentifierChar(query[end]) {
		return false
	}
	return true
}

func isSQLWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f':
		return true
	default:
		return false
	}
}

func isSQLIdentifierChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
