// Package tenant provides multi-tenant context management for database operations.
//
// This package abstracts the tenant isolation mechanism, allowing the application
// to enforce tenant isolation without changing handler or service code.
//
// Current implementation: application-level isolation using workspace_id filtering.
// The API mirrors the PostgreSQL/RLS shape while SQLite uses query-level filtering.
package tenant

import (
	"context"
	"database/sql"
)

// contextKey is the type for tenant context keys to avoid collisions
type contextKey string

const (
	// TenantContextKey is the context key for storing tenant context
	TenantContextKey contextKey = "tenant_context"
)

// Context represents the current tenant context for a request.
// This abstraction allows future migration to schema-per-tenant
// without changing application code.
type Context interface {
	// WorkspaceID returns the current workspace identifier
	WorkspaceID() string

	// Apply configures the database connection for this tenant.
	// For RLS: sets session variable
	// For schema-per-tenant: would set search_path
	Apply(db *sql.DB) error

	// ApplyTx configures a transaction for this tenant.
	// Uses SET LOCAL which scopes to the current transaction only.
	ApplyTx(tx *sql.Tx) error

	// Validate checks if a resource belongs to this tenant.
	// Returns true if the resource's workspace ID matches the current context.
	Validate(resourceWorkspaceID string) bool
}

// RLSContext implements Context for tenant isolation.
// For SQLite, tenant isolation is enforced at the application level
// via workspace_id checks in queries.
type RLSContext struct {
	workspaceID string
}

// NewRLSContext creates a new RLS-based tenant context
func NewRLSContext(workspaceID string) *RLSContext {
	return &RLSContext{workspaceID: workspaceID}
}

// WorkspaceID returns the current workspace identifier
func (c *RLSContext) WorkspaceID() string {
	return c.workspaceID
}

// Apply sets the tenant context for database operations.
// For SQLite, this is a no-op as tenant isolation is enforced at the query level.
func (c *RLSContext) Apply(db *sql.DB) error {
	// No-op for SQLite - tenant isolation is enforced by query-level workspace_id filters
	return nil
}

// ApplyTx sets the tenant context for a transaction.
// For SQLite, this is a no-op as tenant isolation is enforced at the query level.
func (c *RLSContext) ApplyTx(tx *sql.Tx) error {
	// No-op for SQLite - tenant isolation is enforced by query-level workspace_id filters
	return nil
}

// Validate checks if a resource belongs to this tenant
func (c *RLSContext) Validate(resourceWorkspaceID string) bool {
	return c.workspaceID == resourceWorkspaceID
}

// FromContext extracts the tenant context from a Go context.
// Returns nil if no tenant context is set.
func FromContext(ctx context.Context) Context {
	if tc, ok := ctx.Value(TenantContextKey).(Context); ok {
		return tc
	}
	return nil
}

// WithContext adds a tenant context to a Go context
func WithContext(ctx context.Context, tc Context) context.Context {
	return context.WithValue(ctx, TenantContextKey, tc)
}

// ============================================================================
// Implementation Notes
// ============================================================================
//
// SQLite does not support session variables or RLS.
// Tenant isolation is enforced at the application level:
// - all queries include workspace_id in WHERE clauses
// - the Context interface keeps the same shape across runtimes
// - Apply/ApplyTx methods are no-ops for SQLite
// ============================================================================
