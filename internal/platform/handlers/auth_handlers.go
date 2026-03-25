package platformhandlers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/platform/handlers/dtos"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	"github.com/movebigrocks/platform/pkg/logger"
)

// AuthHandler handles authentication and context switching
type AuthHandler struct {
	SessionService *platformservices.SessionService // Exported for AdminHandler
	baseURL        string
	adminBaseURL   string
	environment    string          // "development", "staging", "production"
	cookieDomain   string          // Cookie domain (e.g., ".example.com" for cross-subdomain)
	allowedEmails  map[string]bool // Whitelisted admin emails
	cliLogin       *platformservices.CLILoginService
	emailService   *platformservices.AdminEmailService
	logger         *logger.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	sessionService *platformservices.SessionService,
	baseURL string,
	environment string,
	allowedAdminEmails []string,
	cookieDomain string,
) *AuthHandler {
	emailMap := make(map[string]bool)
	for _, email := range allowedAdminEmails {
		emailMap[email] = true
	}

	return &AuthHandler{
		SessionService: sessionService,
		baseURL:        baseURL,
		environment:    environment,
		cookieDomain:   cookieDomain,
		allowedEmails:  emailMap,
		logger:         logger.New().WithField("handler", "auth"),
	}
}

func (h *AuthHandler) WithCLILogin(adminBaseURL string, cliLogin *platformservices.CLILoginService) *AuthHandler {
	h.adminBaseURL = strings.TrimRight(strings.TrimSpace(adminBaseURL), "/")
	h.cliLogin = cliLogin
	return h
}

// WithEmailService sets the email service for magic link delivery
func (h *AuthHandler) WithEmailService(svc *platformservices.AdminEmailService) *AuthHandler {
	h.emailService = svc
	return h
}

// CookieDomain returns the configured cookie domain for use by other handlers
func (h *AuthHandler) CookieDomain() string {
	return h.cookieDomain
}


type cliLoginPollRequest struct {
	PollToken string `json:"pollToken"`
}

// HandleMagicLinkRequest generates a magic link for authentication
// POST /auth/magic-link
func (h *AuthHandler) HandleMagicLinkRequest(c *gin.Context) {
	var req dtos.AuthRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid email address")
		return
	}

	if isMagicLoginHoneypotTriggered(req.Honeypot) {
		h.logger.WithFields(map[string]interface{}{
			"ip":        c.ClientIP(),
			"userAgent": c.Request.UserAgent(),
		}).Warn("Magic link honeypot triggered")

		if h.SessionService != nil {
			if err := h.SessionService.CheckMagicLinkHoneypotRateLimit(c.Request.Context(), magicLoginHoneypotFingerprint(c)); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"message": "If this email is registered, you will receive a magic link",
				})
				return
			}
		}
	}

	// Check rate limit before processing (prevents enumeration attacks)
	if err := h.SessionService.CheckMagicLinkRateLimit(c.Request.Context(), req.Email); err != nil {
		// Return same message as success to prevent enumeration
		c.JSON(http.StatusOK, gin.H{
			"message": "If this email is registered, you will receive a magic link",
		})
		return
	}

	// Check if user exists
	user, err := h.SessionService.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		// For security: don't reveal if email exists or not
		c.JSON(http.StatusOK, gin.H{
			"message": "If this email is registered, you will receive a magic link",
		})
		return
	}

	// Check if user is active and not locked
	if !user.IsActive || user.IsLocked() {
		c.JSON(http.StatusOK, gin.H{
			"message": "If this email is registered, you will receive a magic link",
		})
		return
	}

	// Generate magic link using session service
	magicLink, err := h.SessionService.GenerateMagicLinkToken(user.ID, req.Email)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to generate magic link")
		return
	}

	// Save magic link to store
	if err := h.SessionService.SaveMagicLink(c.Request.Context(), magicLink); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to save magic link")
		return
	}

	// Generate magic link URL
	magicLinkURL := fmt.Sprintf("%s/auth/verify-magic-link?token=%s", h.baseURL, magicLink.Token)

	// Send magic link via email service
	if h.emailService != nil && h.environment != "development" {
		if err := h.emailService.SendMagicLinkEmail(c.Request.Context(), req.Email, magicLinkURL); err != nil {
			h.logger.WithError(err).Warn("Failed to send magic link email", "email", req.Email)
		}
	}

	// SECURITY: Only log magic link in development mode
	if h.environment == "development" {
		h.logger.Info("Magic link generated (DEV ONLY)", "url", magicLinkURL)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Magic link sent! Check your email.",
	})
}

// magicLoginHoneypotFingerprint creates a stable, privacy-preserving source key
// for honeypot-triggered submissions.
func magicLoginHoneypotFingerprint(c *gin.Context) string {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		ip = "unknown"
	}

	userAgent := strings.TrimSpace(c.Request.UserAgent())
	if userAgent == "" {
		userAgent = "unknown"
	}

	hash := sha256.Sum256([]byte(ip + "|" + userAgent))
	return hex.EncodeToString(hash[:])
}

func isMagicLoginHoneypotTriggered(value string) bool {
	return strings.TrimSpace(value) != ""
}

// VerifyMagicLink verifies a magic link token and creates a session
// GET /auth/verify-magic-link?token=xxx
func (h *AuthHandler) VerifyMagicLink(c *gin.Context) {
	token := c.Query("token")

	if token == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid magic link")
		return
	}

	// Get and validate magic link
	magicLink, err := h.SessionService.GetMagicLink(c.Request.Context(), token)
	if err != nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Invalid or expired magic link")
		return
	}

	// Validate magic link using infrastructure auth service
	if err := h.SessionService.ValidateMagicLinkToken(magicLink); err != nil {
		// Log detailed error for debugging, but return generic message to prevent information leakage
		h.logger.WithError(err).Warn("Magic link validation failed", "ip", c.ClientIP())
		middleware.RespondWithError(c, http.StatusUnauthorized, "Invalid or expired magic link")
		return
	}

	// Mark magic link as used (atomic check-and-set to prevent race conditions)
	if err := h.SessionService.MarkMagicLinkUsed(c.Request.Context(), token); err != nil {
		if contracts.IsAlreadyUsed(err) {
			// Token was already used by another concurrent request
			middleware.RespondWithError(c, http.StatusUnauthorized, "Invalid or expired magic link")
			return
		}
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to mark magic link as used")
		return
	}

	// Get user
	user, err := h.SessionService.GetUserByID(c.Request.Context(), magicLink.UserID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to retrieve user")
		return
	}

	// Create session with all available contexts using SessionService
	// CreateSession returns both the session (with TokenHash) and the plaintext token
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()
	session, sessionToken, err := h.SessionService.CreateSession(c.Request.Context(), user, ipAddress, userAgent)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to create session")
		return
	}

	// Set session cookie with secure flag based on environment
	// SameSite=Lax prevents CSRF while allowing normal navigation
	// Note: We use the plaintext token here - only the hash is stored in the database
	// Cookie domain enables cross-subdomain auth (e.g., ".example.com")
	secure := h.environment != "development"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"mbr_session",
		sessionToken, // Use plaintext token (hash is stored in DB)
		int(session.ExpiresAt.Sub(session.CreatedAt).Seconds()),
		"/",
		h.cookieDomain, // Empty for host-only, ".domain.com" for cross-subdomain
		secure,         // Secure: true in production/staging, false only in development
		true,           // HttpOnly
	)

	// Determine redirect based on default context
	redirectURL := h.getRedirectURLForContext(session.CurrentContext)

	c.JSON(http.StatusOK, gin.H{
		"message":  "Authentication successful",
		"redirect": redirectURL,
		"context":  dtos.ToContextResponse(session.CurrentContext),
		"contexts": dtos.ToContextResponseList(session.AvailableContexts),
	})
}

// StartCLILogin creates a pending browser login request for the CLI.
// POST /auth/cli/start
func (h *AuthHandler) StartCLILogin(c *gin.Context) {
	if h.cliLogin == nil {
		middleware.RespondWithError(c, http.StatusNotImplemented, "CLI browser login is not configured")
		return
	}

	start, err := h.cliLogin.Start()
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to start CLI login")
		return
	}

	authorizeURL, err := h.cliAuthorizeURL(start.RequestID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to build CLI login URL")
		return
	}
	adminBaseURL := strings.TrimRight(strings.TrimSpace(h.adminBaseURL), "/")
	if adminBaseURL == "" {
		adminBaseURL = strings.TrimRight(strings.TrimSpace(h.baseURL), "/")
	}

	c.JSON(http.StatusOK, gin.H{
		"requestID":        start.RequestID,
		"pollToken":        start.PollToken,
		"authorizeURL":     authorizeURL,
		"adminBaseURL":     adminBaseURL,
		"adminGraphQLURL":  adminBaseURL + "/graphql",
		"expiresAt":        start.ExpiresAt,
		"expiresInSeconds": int(time.Until(start.ExpiresAt).Seconds()),
		"intervalSeconds":  2,
	})
}

// PollCLILogin polls the pending browser login request for completion.
// POST /auth/cli/poll
func (h *AuthHandler) PollCLILogin(c *gin.Context) {
	if h.cliLogin == nil {
		middleware.RespondWithError(c, http.StatusNotImplemented, "CLI browser login is not configured")
		return
	}

	var req cliLoginPollRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.PollToken) == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "pollToken is required")
		return
	}

	result, err := h.cliLogin.Poll(strings.TrimSpace(req.PollToken))
	if err != nil {
		switch {
		case errors.Is(err, platformservices.ErrCLILoginExpired):
			c.JSON(http.StatusGone, gin.H{"status": "expired"})
		case errors.Is(err, platformservices.ErrCLILoginRequestNotFound):
			middleware.RespondWithError(c, http.StatusNotFound, "CLI login request not found")
		default:
			middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to poll CLI login")
		}
		return
	}

	if result.Status == platformservices.CLILoginStatusPending {
		c.JSON(http.StatusOK, gin.H{
			"status":    string(result.Status),
			"expiresAt": result.ExpiresAt,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       string(result.Status),
		"userID":       result.UserID,
		"sessionToken": result.SessionToken,
		"expiresAt":    result.ExpiresAt,
	})
}

func (h *AuthHandler) cliAuthorizeURL(requestID string) (string, error) {
	adminBaseURL := strings.TrimSpace(h.adminBaseURL)
	if adminBaseURL == "" {
		adminBaseURL = strings.TrimRight(strings.TrimSpace(h.baseURL), "/")
	}
	u, err := url.Parse(adminBaseURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/cli-login"
	q := u.Query()
	q.Set("request_id", requestID)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// SwitchContext allows users to switch between available contexts
// POST /auth/switch-context
// Body: { "type": "instance|workspace", "workspace_id": "optional-workspace-id" }
func (h *AuthHandler) SwitchContext(c *gin.Context) {
	// Get current session from middleware
	sessionValue, exists := c.Get("session")
	if !exists {
		middleware.RespondWithError(c, http.StatusUnauthorized, "No active session")
		return
	}

	session, ok := sessionValue.(*platformdomain.Session)
	if !ok || session == nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Invalid session")
		return
	}

	var req struct {
		Type        string  `json:"type" binding:"required,oneof=instance workspace"`
		WorkspaceID *string `json:"workspace_id,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Validate workspace_id is provided for workspace context
	contextType := platformdomain.ContextType(req.Type)
	if contextType == platformdomain.ContextTypeWorkspace && req.WorkspaceID == nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "workspace_id is required for workspace context")
		return
	}

	// Switch context using SessionService
	if err := h.SessionService.SwitchContext(c.Request.Context(), session, contextType, req.WorkspaceID); err != nil {
		middleware.RespondWithError(c, http.StatusForbidden, "Access denied")
		return
	}

	// Determine redirect URL based on new context
	redirectURL := h.getRedirectURLForContext(session.CurrentContext)

	c.JSON(http.StatusOK, gin.H{
		"message":         "Context switched successfully",
		"current_context": dtos.ToContextResponse(session.CurrentContext),
		"redirect":        redirectURL,
	})
}

// GetCurrentContext returns the user's current session context
// GET /auth/context
func (h *AuthHandler) GetCurrentContext(c *gin.Context) {
	sessionValue, exists := c.Get("session")
	if !exists {
		middleware.RespondWithError(c, http.StatusUnauthorized, "No active session")
		return
	}

	session, ok := sessionValue.(*platformdomain.Session)
	if !ok || session == nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Invalid session")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"current_context":    dtos.ToContextResponse(session.CurrentContext),
		"available_contexts": dtos.ToContextResponseList(session.AvailableContexts),
		"user": gin.H{
			"id":    session.UserID,
			"email": session.Email,
			"name":  session.Name,
		},
	})
}

// Logout invalidates the user's session
// POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// Get session token from cookie
	token, err := c.Cookie("mbr_session")
	if err == nil && token != "" {
		// Hash the token to find and delete the session
		hash := sha256.Sum256([]byte(token))
		tokenHash := hex.EncodeToString(hash[:])

		// Invalidate session by hash
		if err := h.SessionService.DeleteSessionByHash(c.Request.Context(), tokenHash); err != nil {
			// Log error but continue with logout
			h.logger.WithError(err).Warn("Failed to delete session during logout")
		}
	}

	// Clear session cookie with secure flag based on environment
	// SameSite=Lax for consistency with session creation
	// Cookie domain must match the domain used when setting the cookie
	secure := h.environment != "development"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"mbr_session",
		"",
		-1, // MaxAge: -1 deletes the cookie
		"/",
		h.cookieDomain,
		secure,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// Helper: Determine redirect URL based on context
// Admin routes stay at top-level paths; handlers scope data based on current context.
func (h *AuthHandler) getRedirectURLForContext(ctx platformdomain.Context) string {
	return "/dashboard"
}
