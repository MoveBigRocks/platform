package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
)

// ContextAuthMiddleware handles context-aware authentication
type ContextAuthMiddleware struct {
	sessionService *platformservices.SessionService
	store          shared.Store // For workspace lookups (optional, used by RequireWorkspaceAccessBySlug)
	isDevelopment  bool         // Controls Secure flag on cookies
	cookieDomain   string       // Cookie domain for cross-subdomain auth
}

// NewContextAuthMiddleware creates a new context auth middleware
func NewContextAuthMiddleware(sessionService *platformservices.SessionService) *ContextAuthMiddleware {
	return &ContextAuthMiddleware{
		sessionService: sessionService,
		isDevelopment:  false, // Defaults to production-safe (Secure=true)
	}
}

// WithEnvironment sets whether this is a development environment.
// In development, cookies are set without the Secure flag to allow HTTP.
func (m *ContextAuthMiddleware) WithEnvironment(environment string) *ContextAuthMiddleware {
	m.isDevelopment = environment == "development"
	return m
}

// WithStore adds a store to the middleware for workspace lookups
// Required for RequireWorkspaceAccessBySlug middleware
func (m *ContextAuthMiddleware) WithStore(store shared.Store) *ContextAuthMiddleware {
	m.store = store
	return m
}

// WithCookieDomain sets the cookie domain for cross-subdomain authentication
// e.g., ".example.com" allows cookies to be shared across subdomains
func (m *ContextAuthMiddleware) WithCookieDomain(domain string) *ContextAuthMiddleware {
	m.cookieDomain = domain
	return m
}

// AuthRequired validates the session and sets context variables
// Use this for all protected routes.
func (m *ContextAuthMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session token from cookie
		token, err := c.Cookie("mbr_session")
		if err != nil {
			m.redirectToLogin(c)
			return
		}

		// Validate session
		session, err := m.sessionService.ValidateSession(c.Request.Context(), token)
		if err != nil || !session.IsValid() {
			m.clearCookieAndRedirect(c)
			return
		}

		// Check for idle timeout (7 days - matches session expiry)
		if session.IsIdle(7 * 24 * time.Hour) {
			m.clearCookieAndRedirect(c)
			return
		}

		// Update activity (async to not block request)
		// Check debounce BEFORE spawning goroutine to avoid goroutine churn on high-traffic sites
		if m.sessionService.ShouldUpdateActivity(session.ID) {
			// Pass immutable token hash instead of session pointer to avoid race conditions.
			// The goroutine will re-fetch the session from DB, ensuring it works with
			// a fresh copy rather than sharing a mutable pointer with the request handler.
			requestID := c.GetString("request_id")
			go func(tokenHash, requestID string) {
				defer func() {
					if r := recover(); r != nil {
						// Panic in activity update should not crash the server.
						// This is best-effort work; log and continue.
						slog.Error("panic while updating session activity",
							"request_id", requestID,
							"token_hash", tokenHash,
							"panic", r)
					}
				}()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				// Best-effort activity update: log errors but don't block request
				if err := m.sessionService.UpdateActivityByHash(ctx, tokenHash); err != nil {
					// Log at debug level since this is best-effort and non-critical
					// Errors here are typically transient DB issues that resolve on next request
					slog.Debug("failed to update session activity",
						"request_id", requestID,
						"token_hash", tokenHash,
						"error", err)
				}
			}(session.TokenHash, requestID)
		}

		// Set session and user info in context for downstream handlers
		c.Set("session", session)
		c.Set("user_id", session.UserID)
		c.Set("email", session.Email)
		c.Set("name", session.Name)
		c.Set("current_context", session.CurrentContext)
		c.Set("available_contexts", session.AvailableContexts)

		// Set the normalized AuthContext for downstream handlers that call
		// middleware.GetAuthContext(c).
		authCtx := &platformdomain.AuthContext{
			Principal: &platformdomain.User{
				ID:    session.UserID,
				Email: session.Email,
				Name:  session.Name,
			},
			PrincipalType: platformdomain.PrincipalTypeUser,
			AuthMethod:    platformdomain.AuthMethodSession,
		}
		// Set instance role if in instance context
		if session.IsInstanceContext() {
			instanceRole := platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(session.CurrentContext.Role))
			if !instanceRole.IsOperator() {
				instanceRole = ""
			}
			authCtx.InstanceRole = &instanceRole
			if instanceRole == "" {
				authCtx.InstanceRole = nil
			}
		}
		// Set workspace ID if in workspace context
		if session.IsWorkspaceContext() && session.CurrentContext.WorkspaceID != nil {
			authCtx.WorkspaceID = *session.CurrentContext.WorkspaceID
		}
		c.Set("auth_context", authCtx)

		c.Next()
	}
}

// RequireOperationalAccess allows either instance-operator access or an active workspace context.
// Use this for shared browser pages that can render either cross-workspace or workspace-scoped data.
func (m *ContextAuthMiddleware) RequireOperationalAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, exists := c.Get("session")
		if !exists {
			RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Not authenticated")
			return
		}

		s, ok := session.(*platformdomain.Session)
		if !ok {
			RespondWithErrorAndAbort(c, http.StatusInternalServerError, "Invalid session type")
			return
		}

		if s.IsInstanceContext() {
			userRole := platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(s.CurrentContext.Role))
			if !userRole.IsOperator() {
				m.respondForbidden(c, "Insufficient permissions for this operation")
				return
			}
			s.CurrentContext.Role = string(userRole)
			c.Set("instance_role", s.CurrentContext.Role)
			c.Next()
			return
		}

		if s.IsWorkspaceContext() && s.CurrentContext.WorkspaceID != nil {
			c.Set("workspace_id", *s.CurrentContext.WorkspaceID)
			if s.CurrentContext.WorkspaceSlug != nil {
				c.Set("workspace_slug", *s.CurrentContext.WorkspaceSlug)
			}
			c.Set("workspace_role", s.CurrentContext.Role)
			c.Next()
			return
		}

		m.respondForbidden(c, "Please switch to an accessible operational context")
	}
}

// RequireInstanceAccess ensures the user is in instance admin context
// Use this for admin subdomain routes (instance admin panel)
func (m *ContextAuthMiddleware) RequireInstanceAccess(allowedRoles ...platformdomain.InstanceRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, exists := c.Get("session")
		if !exists {
			RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Not authenticated")
			return
		}

		s, ok := session.(*platformdomain.Session)
		if !ok {
			RespondWithErrorAndAbort(c, http.StatusInternalServerError, "Invalid session type")
			return
		}

		// Check if current context is instance admin
		if !s.IsInstanceContext() {
			m.respondForbidden(c, "Please switch to Instance Admin context to access this page")
			return
		}

		userRole := platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(s.CurrentContext.Role))
		if !userRole.IsOperator() {
			m.respondForbidden(c, "Insufficient permissions for this operation")
			return
		}
		s.CurrentContext.Role = string(userRole)

		// If specific roles are required, check them
		if len(allowedRoles) > 0 {
			normalizedRoles := make([]platformdomain.InstanceRole, 0, len(allowedRoles))
			for _, allowedRole := range allowedRoles {
				normalizedRoles = append(normalizedRoles, platformdomain.CanonicalizeInstanceRole(allowedRole))
			}

			// Super admin bypasses all role checks
			if userRole == platformdomain.InstanceRoleSuperAdmin {
				c.Next()
				return
			}

			// Check if user has one of the allowed roles
			allowed := false
			for _, role := range normalizedRoles {
				if userRole == role {
					allowed = true
					break
				}
			}

			if !allowed {
				m.respondForbidden(c, "Insufficient permissions for this operation")
				return
			}
		}

		// Set instance-specific context variables
		c.Set("instance_role", s.CurrentContext.Role)

		c.Next()
	}
}

// RequireCurrentWorkspaceAccess validates the active workspace context against workspace_id already present in gin.Context.
// Use this for tenant-subdomain routes where middleware has already injected workspace_id.
func (m *ContextAuthMiddleware) RequireCurrentWorkspaceAccess(allowedRoles ...platformdomain.WorkspaceRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, exists := c.Get("session")
		if !exists {
			RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Not authenticated")
			return
		}

		s, ok := session.(*platformdomain.Session)
		if !ok {
			RespondWithErrorAndAbort(c, http.StatusInternalServerError, "Invalid session type")
			return
		}

		workspaceID := c.GetString("workspace_id")
		if workspaceID == "" {
			RespondWithErrorAndAbort(c, http.StatusBadRequest, "workspace_id required")
			return
		}

		if !s.IsWorkspaceContext() || s.CurrentContext.WorkspaceID == nil || *s.CurrentContext.WorkspaceID != workspaceID {
			m.respondForbidden(c, "Context mismatch: please switch to this workspace first")
			return
		}

		if len(allowedRoles) > 0 {
			userRole := platformdomain.WorkspaceRole(s.CurrentContext.Role)
			if userRole != platformdomain.WorkspaceRoleOwner && userRole != platformdomain.WorkspaceRoleAdmin {
				allowed := false
				for _, role := range allowedRoles {
					if userRole == role {
						allowed = true
						break
					}
				}
				if !allowed {
					m.respondForbidden(c, "Insufficient workspace permissions for this operation")
					return
				}
			}
		}

		c.Set("workspace_role", s.CurrentContext.Role)
		c.Next()
	}
}

// Helper functions

func (m *ContextAuthMiddleware) redirectToLogin(c *gin.Context) {
	// Check if this is an HTML request or API request
	if m.isHTMLRequest(c) {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
	} else {
		RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Authentication required")
	}
}

func (m *ContextAuthMiddleware) clearCookieAndRedirect(c *gin.Context) {
	m.clearSessionCookie(c)
	m.redirectToLogin(c)
}

// clearSessionCookie clears the session cookie with proper security settings.
// Uses SameSite=Lax to prevent CSRF, and Secure=true in production.
// Cookie domain must match the domain used when setting the cookie.
func (m *ContextAuthMiddleware) clearSessionCookie(c *gin.Context) {
	secure := !m.isDevelopment
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("mbr_session", "", -1, "/", m.cookieDomain, secure, true)
}

func (m *ContextAuthMiddleware) respondForbidden(c *gin.Context, message string) {
	if m.isHTMLRequest(c) {
		c.HTML(http.StatusForbidden, "403.html", gin.H{
			"message": message,
		})
		c.Abort()
	} else {
		RespondWithErrorAndAbort(c, http.StatusForbidden, message)
	}
}

func (m *ContextAuthMiddleware) isHTMLRequest(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	return strings.HasPrefix(accept, "text/html")
}

// RespondWithErrorAndAbort sends an error response and aborts the request.
func RespondWithErrorAndAbort(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error":   http.StatusText(statusCode),
		"message": message,
	})
	c.Abort()
}
