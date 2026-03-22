package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

// AdminContextMiddleware provides RLS bypass for instance admin operations.
// This allows admin handlers to query across all workspaces.
type AdminContextMiddleware struct {
	store stores.Store
}

// NewAdminContextMiddleware creates a new admin context middleware
func NewAdminContextMiddleware(store stores.Store) *AdminContextMiddleware {
	return &AdminContextMiddleware{
		store: store,
	}
}

// WithAdminContext wraps the request handler in an admin context that bypasses RLS.
// Use this middleware for admin routes that need to query data across all workspaces.
//
// SECURITY: Only apply this to routes that are already protected by RequireInstanceAccess().
// The admin context uses the mbr_admin role which has BYPASSRLS privilege.
//
// Usage:
//
//	adminRoutes := router.Group("/admin")
//	adminRoutes.Use(authMiddleware.AuthRequired())
//	adminRoutes.Use(authMiddleware.RequireInstanceAccess())
//	adminRoutes.Use(adminContextMiddleware.WithAdminContext())
func (m *AdminContextMiddleware) WithAdminContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Wrap the entire request in an admin context transaction
		err := m.store.WithAdminContext(c.Request.Context(), func(adminCtx context.Context) error {
			// Replace request context with admin context
			c.Request = c.Request.WithContext(adminCtx)

			// Process request within admin context
			c.Next()

			// Check if handler aborted with an error
			if len(c.Errors) > 0 {
				return c.Errors[0].Err
			}

			return nil
		})

		if err != nil {
			// Only set error response if not already handled by handler
			if !c.IsAborted() {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Internal Server Error",
					"message": "Failed to establish admin context",
				})
				c.Abort()
			}
		}
	}
}

// WithOperationalContext applies admin RLS bypass only when the current session is in instance context.
// Workspace-scoped requests continue without bypass and rely on explicit workspace filters.
func (m *AdminContextMiddleware) WithOperationalContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionVal, exists := c.Get("session")
		if !exists {
			c.Next()
			return
		}

		session, ok := sessionVal.(*platformdomain.Session)
		if !ok || !session.IsInstanceContext() {
			c.Next()
			return
		}

		m.WithAdminContext()(c)
	}
}
