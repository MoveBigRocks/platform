package graph

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/graph/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

// GinHandler wraps the GraphQL server for use with gin, bridging gin context to GraphQL context.
// Supports both agent auth (auth_context) and session auth (session).
func GinHandler(server http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Transfer auth context from gin to Go context
		ctx := c.Request.Context()

		// Check for agent auth (via PrincipalAuthMiddleware)
		if authCtx := middleware.GetAuthContext(c); authCtx != nil {
			ctx = shared.SetAuthContext(ctx, authCtx)
		} else if session := getSession(c); session != nil {
			// Build auth context from session (human user via ContextAuthMiddleware)
			authCtx := buildAuthContextFromSession(c, session)
			if authCtx != nil {
				ctx = shared.SetAuthContext(ctx, authCtx)
			}
		}

		// Create a new request with the enriched context
		req := c.Request.WithContext(ctx)
		server.ServeHTTP(c.Writer, req)
	}
}

// getSession retrieves the session from gin context
func getSession(c *gin.Context) *platformdomain.Session {
	session, exists := c.Get("session")
	if !exists {
		return nil
	}
	s, ok := session.(*platformdomain.Session)
	if !ok {
		return nil
	}
	return s
}

// buildAuthContextFromSession creates an AuthContext from a session for GraphQL use
func buildAuthContextFromSession(c *gin.Context, session *platformdomain.Session) *platformdomain.AuthContext {
	if session == nil {
		return nil
	}

	// Create a user principal from session data
	user := &platformdomain.User{
		ID:    session.UserID,
		Email: session.Email,
		Name:  session.Name,
	}

	// Build permissions based on current context
	var permissions []string
	var workspaceID string
	var instanceRole *platformdomain.InstanceRole

	if session.CurrentContext.Type != "" {
		// Instance admin context
		if session.CurrentContext.Type == platformdomain.ContextTypeInstance {
			// Set instance role so instance-level authorization checks work correctly.
			// Keep only canonical roles that are valid instance-admin scopes.
			canonicalRole := platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(session.CurrentContext.Role))
			if canonicalRole.IsOperator() {
				role := canonicalRole
				instanceRole = &role
			}
			// Instance admins get full permissions
			permissions = []string{
				platformdomain.PermissionCaseRead,
				platformdomain.PermissionCaseWrite,
				platformdomain.PermissionQueueRead,
				platformdomain.PermissionQueueWrite,
				platformdomain.PermissionConversationRead,
				platformdomain.PermissionConversationWrite,
				platformdomain.PermissionKnowledgeRead,
				platformdomain.PermissionKnowledgeWrite,
				platformdomain.PermissionCatalogRead,
				platformdomain.PermissionCatalogWrite,
				platformdomain.PermissionFormRead,
				platformdomain.PermissionFormWrite,
				platformdomain.PermissionIssueRead,
				platformdomain.PermissionIssueWrite,
				platformdomain.PermissionContactRead,
				platformdomain.PermissionContactWrite,
				platformdomain.PermissionApplicationRead,
				platformdomain.PermissionApplicationWrite,
				platformdomain.PermissionExtensionRead,
				platformdomain.PermissionExtensionWrite,
			}
		} else if session.CurrentContext.Type == platformdomain.ContextTypeWorkspace {
			// Workspace context - derive permissions from role
			if session.CurrentContext.WorkspaceID != nil {
				workspaceID = *session.CurrentContext.WorkspaceID
			}
			permissions = derivePermissionsFromRole(session.CurrentContext.Role)
		}
	}

	return &platformdomain.AuthContext{
		Principal:     user,
		PrincipalType: platformdomain.PrincipalTypeUser,
		WorkspaceID:   workspaceID,
		Permissions:   permissions,
		InstanceRole:  instanceRole,
		AuthMethod:    platformdomain.AuthMethodSession,
		RequestID:     c.GetString("request_id"),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
	}
}

// derivePermissionsFromRole returns permissions based on workspace role
func derivePermissionsFromRole(role string) []string {
	switch platformdomain.WorkspaceRole(role) {
	case platformdomain.WorkspaceRoleOwner, platformdomain.WorkspaceRoleAdmin:
		return []string{
			platformdomain.PermissionCaseRead,
			platformdomain.PermissionCaseWrite,
			platformdomain.PermissionQueueRead,
			platformdomain.PermissionQueueWrite,
			platformdomain.PermissionConversationRead,
			platformdomain.PermissionConversationWrite,
			platformdomain.PermissionKnowledgeRead,
			platformdomain.PermissionKnowledgeWrite,
			platformdomain.PermissionCatalogRead,
			platformdomain.PermissionCatalogWrite,
			platformdomain.PermissionFormRead,
			platformdomain.PermissionFormWrite,
			platformdomain.PermissionIssueRead,
			platformdomain.PermissionIssueWrite,
			platformdomain.PermissionContactRead,
			platformdomain.PermissionContactWrite,
			platformdomain.PermissionApplicationRead,
			platformdomain.PermissionApplicationWrite,
			platformdomain.PermissionExtensionRead,
			platformdomain.PermissionExtensionWrite,
		}
	case platformdomain.WorkspaceRoleMember:
		return []string{
			platformdomain.PermissionCaseRead,
			platformdomain.PermissionCaseWrite,
			platformdomain.PermissionQueueRead,
			platformdomain.PermissionQueueWrite,
			platformdomain.PermissionConversationRead,
			platformdomain.PermissionConversationWrite,
			platformdomain.PermissionKnowledgeRead,
			platformdomain.PermissionKnowledgeWrite,
			platformdomain.PermissionCatalogRead,
			platformdomain.PermissionCatalogWrite,
			platformdomain.PermissionFormRead,
			platformdomain.PermissionFormWrite,
			platformdomain.PermissionIssueRead,
			platformdomain.PermissionContactRead,
			platformdomain.PermissionExtensionRead,
		}
	case platformdomain.WorkspaceRoleViewer:
		return []string{
			platformdomain.PermissionCaseRead,
			platformdomain.PermissionQueueRead,
			platformdomain.PermissionConversationRead,
			platformdomain.PermissionKnowledgeRead,
			platformdomain.PermissionCatalogRead,
			platformdomain.PermissionFormRead,
			platformdomain.PermissionIssueRead,
			platformdomain.PermissionContactRead,
			platformdomain.PermissionExtensionRead,
		}
	default:
		return []string{}
	}
}
