package middleware

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/tenant"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
)

// TenantContextMiddleware sets up tenant context for database queries.
// This must be applied AFTER authentication middleware that sets workspace_id.
//
// For SQLite, tenant isolation is enforced at the application level via
// workspace_id checks in queries. This middleware stores the tenant context
// and wraps requests in transactions for consistency.
type TenantContextMiddleware struct {
	store  stores.Store
	logger *slog.Logger
}

// NewTenantContextMiddleware creates a new tenant context middleware
func NewTenantContextMiddleware(store stores.Store, logger *slog.Logger) *TenantContextMiddleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &TenantContextMiddleware{
		store:  store,
		logger: logger,
	}
}

// SetTenantContext extracts workspace_id from gin context and sets up tenant isolation.
// This enables application-level tenant isolation via workspace_id filtering.
//
// The request is wrapped in a database transaction for consistency.
// For SQLite, tenant isolation is enforced by query-level workspace_id filters.
//
// Usage:
//
//	router.Use(authMiddleware.RequireCurrentWorkspaceAccess())
//	router.Use(tenantMiddleware.SetTenantContext())  // Must come after workspace_id is set
func (m *TenantContextMiddleware) SetTenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		workspaceID := c.GetString("workspace_id")

		if workspaceID == "" {
			// No workspace context (public route or instance admin)
			// RLS will return empty results for tenant-scoped tables (fail-safe)
			c.Next()
			return
		}

		// Create tenant context
		tenantCtx := tenant.NewRLSContext(workspaceID)

		// Store tenant context in gin context for convenience (before transaction)
		c.Set("tenant_context", tenantCtx)

		// Wrap the entire request in a transaction for consistency
		err := m.store.WithTransaction(c.Request.Context(), func(txCtx context.Context) error {
			// Set tenant context (a no-op for SQLite, but the same call path is used everywhere)
			if err := m.store.SetTenantContext(txCtx, workspaceID); err != nil {
				m.logger.Error("failed to set tenant context",
					"workspace_id", workspaceID,
					"error", err,
				)
				return err
			}

			// Update request context with both tenant context and transaction
			ctx := tenant.WithContext(txCtx, tenantCtx)
			c.Request = c.Request.WithContext(ctx)

			// Process request within transaction
			c.Next()

			// Check if handler aborted with an error
			if len(c.Errors) > 0 {
				// Return the first error to trigger rollback
				return c.Errors[0].Err
			}

			return nil
		})

		if err != nil {
			// Only set error response if not already handled by handler
			if !c.IsAborted() {
				RespondWithError(c, 500, "Failed to set tenant context")
				c.Abort()
			}
			return
		}
	}
}

// RequireTenantContext ensures a tenant context is set before proceeding.
// Use this for routes that absolutely require tenant isolation.
func (m *TenantContextMiddleware) RequireTenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantCtx := tenant.FromContext(c.Request.Context())
		if tenantCtx == nil || tenantCtx.WorkspaceID() == "" {
			RespondWithError(c, 400, "Workspace context required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RespondWithError sends a JSON error response.
func RespondWithError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error":  message,
		"status": statusCode,
	})
	c.Abort()
}
