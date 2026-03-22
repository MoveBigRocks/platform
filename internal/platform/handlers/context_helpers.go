package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

// ContextValues holds validated context values from middleware.
// Use GetContextValues() to extract these from the gin.Context.
type ContextValues struct {
	UserID    string
	UserName  string
	UserEmail string
	Session   *platformdomain.Session
}

// GetContextValues extracts and validates all common context values.
// Returns ContextValues with whatever values are available (nil/empty for missing).
// Use this for handlers that can tolerate missing context values (e.g., template rendering).
func GetContextValues(c *gin.Context) *ContextValues {
	cv := &ContextValues{}

	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok {
			cv.UserID = id
		}
	}

	if userName, exists := c.Get("name"); exists {
		if name, ok := userName.(string); ok {
			cv.UserName = name
		}
	}

	if userEmail, exists := c.Get("email"); exists {
		if email, ok := userEmail.(string); ok {
			cv.UserEmail = email
		}
	}

	if sessionVal, exists := c.Get("session"); exists {
		if session, ok := sessionVal.(*platformdomain.Session); ok {
			cv.Session = session
		}
	}

	return cv
}

// InstanceRole returns the canonicalized instance role from the session context, if any.
func (cv *ContextValues) InstanceRole() platformdomain.InstanceRole {
	if cv == nil || cv.Session == nil {
		return ""
	}

	return platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(cv.Session.CurrentContext.Role))
}

// CanManageUsers returns true when the current role can perform user management.
func (cv *ContextValues) CanManageUsers() bool {
	return cv.InstanceRole().IsAdmin()
}

// UserRole returns the canonicalized instance role string for templates.
func (cv *ContextValues) UserRole() string {
	if role := cv.InstanceRole(); role != "" {
		return string(role)
	}
	return ""
}

// IsWorkspaceContext returns true when the active session context is workspace-scoped.
func (cv *ContextValues) IsWorkspaceContext() bool {
	return cv != nil && cv.Session != nil && cv.Session.IsWorkspaceContext()
}

// WorkspaceContext returns the active workspace context identifiers for templates and handlers.
func (cv *ContextValues) WorkspaceContext() (workspaceID, workspaceName, workspaceSlug string, ok bool) {
	if !cv.IsWorkspaceContext() || cv.Session.CurrentContext.WorkspaceID == nil {
		return "", "", "", false
	}

	workspaceID = *cv.Session.CurrentContext.WorkspaceID
	if cv.Session.CurrentContext.WorkspaceName != nil {
		workspaceName = *cv.Session.CurrentContext.WorkspaceName
	}
	if cv.Session.CurrentContext.WorkspaceSlug != nil {
		workspaceSlug = *cv.Session.CurrentContext.WorkspaceSlug
	}
	return workspaceID, workspaceName, workspaceSlug, true
}

// MustGetUserID extracts user_id from context or aborts with 401.
// Use this for endpoints that require an authenticated user.
// Returns empty string if user_id is missing (after aborting the request).
func MustGetUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		middleware.RespondWithErrorAndAbort(c, http.StatusUnauthorized, "User not authenticated")
		return ""
	}

	id, ok := userID.(string)
	if !ok || id == "" {
		middleware.RespondWithErrorAndAbort(c, http.StatusInternalServerError, "Invalid user context")
		return ""
	}

	return id
}

// RespondWithSuccessOrRedirect handles the common pattern of responding with JSON for API requests
// or redirecting for form submissions based on the Accept header.
// Use this for action handlers that need to support both HTMX/form submissions and API calls.
func RespondWithSuccessOrRedirect(c *gin.Context, message, redirectPath string) {
	if c.Request.Header.Get("Accept") == "application/json" {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": message})
	} else {
		c.Redirect(http.StatusSeeOther, redirectPath)
	}
}
