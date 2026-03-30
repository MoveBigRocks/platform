package platformhandlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/middleware"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	"github.com/movebigrocks/platform/pkg/logger"
	"github.com/movebigrocks/platform/web"
)

// AdminHandler handles admin panel routes including Grafana proxy
type AdminHandler struct {
	grafanaURL    string
	authHandler   *AuthHandler
	statsService  *platformservices.AdminStatsService
	emailService  *platformservices.AdminEmailService
	nonceService  *platformservices.NonceService
	grafanaSecret string
	baseURL       string
	isDevelopment bool
	appConfig     *config.Config
	logger        *logger.Logger
}

const (
	grafanaAccessCookieName = "mbr_grafana_access"
	grafanaAccessCookieTTL  = 30 * time.Minute
	magicLinkHoneypotField  = "honeypot"
)

// NewAdminHandler creates a new admin handler.
// The nonceService parameter manages form nonces to prevent duplicate submissions.
// If nonceService is nil, nonce validation will be disabled (not recommended for production).
func NewAdminHandler(grafanaURL string, authHandler *AuthHandler, statsService *platformservices.AdminStatsService, emailService *platformservices.AdminEmailService, nonceService *platformservices.NonceService, baseURL string, isDevelopment bool, grafanaSecret string, appConfig *config.Config) *AdminHandler {
	if grafanaURL == "" {
		grafanaURL = "http://127.0.0.1:3000"
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	// Ensure grafana access secret is never empty so proxy gating remains active.
	if grafanaSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic("crypto/rand failure: cannot generate grafana proxy secret: " + err.Error())
		}
		grafanaSecret = hex.EncodeToString(b)
	}
	return &AdminHandler{
		grafanaURL:    grafanaURL,
		authHandler:   authHandler,
		statsService:  statsService,
		emailService:  emailService,
		nonceService:  nonceService,
		grafanaSecret: grafanaSecret,
		appConfig:     appConfig,
		baseURL:       baseURL,
		isDevelopment: isDevelopment,
		logger:        logger.New().WithField("handler", "admin"),
	}
}

func (h *AdminHandler) createGrafanaAccessToken(now time.Time) string {
	expiresAt := now.Add(grafanaAccessCookieTTL).Unix()
	payload := strconv.FormatInt(expiresAt, 10)

	mac := hmac.New(sha256.New, []byte(h.grafanaSecret))
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)

	return payload + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (h *AdminHandler) validateGrafanaAccessToken(token string, now time.Time) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}

	expiresAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || now.Unix() > expiresAt {
		return false
	}

	gotSig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.grafanaSecret))
	mac.Write([]byte(parts[0]))
	expectedSig := mac.Sum(nil)

	return subtle.ConstantTimeCompare(gotSig, expectedSig) == 1
}

func (h *AdminHandler) issueGrafanaAccessCookie(c *gin.Context) {
	token := h.createGrafanaAccessToken(time.Now())
	secure := !h.isDevelopment

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		grafanaAccessCookieName,
		token,
		int(grafanaAccessCookieTTL.Seconds()),
		"/",
		h.authHandler.CookieDomain(),
		secure,
		true,
	)
}

func (h *AdminHandler) hasGrafanaAccess(c *gin.Context) bool {
	token, err := c.Cookie(grafanaAccessCookieName)
	if err != nil || token == "" {
		return false
	}
	return h.validateGrafanaAccessToken(token, time.Now())
}

// generateFormNonce creates a new single-use form nonce using the NonceService
func (h *AdminHandler) generateFormNonce() (string, error) {
	if h.nonceService == nil {
		return "", nil // Nonce validation disabled
	}
	return h.nonceService.Generate()
}

// validateAndConsumeNonce checks if a nonce is valid and marks it as used
func (h *AdminHandler) validateAndConsumeNonce(nonce string) bool {
	if h.nonceService == nil {
		return true // Nonce validation disabled - always pass
	}
	return h.nonceService.ValidateAndConsume(nonce)
}

// adminBaseURL constructs the admin subdomain URL from the base URL.
// e.g., https://app.example.com -> https://admin.example.com
func (h *AdminHandler) adminBaseURL() string {
	parsed, err := url.Parse(h.baseURL)
	if err != nil {
		return h.baseURL // fallback
	}
	// Insert "admin." prefix to the host
	if !strings.HasPrefix(parsed.Host, "admin.") {
		parsed.Host = "admin." + parsed.Host
	}
	return parsed.String()
}

// isValidReturnURL validates that a return_to URL is safe to redirect to.
// Prevents open redirect attacks by only allowing:
// - Relative paths (starting with /)
// - Absolute URLs to the configured base domain and its subdomains
func (h *AdminHandler) isValidReturnURL(returnTo string) bool {
	// Allow relative paths (but not protocol-relative URLs like //evil.com)
	if strings.HasPrefix(returnTo, "/") && !strings.HasPrefix(returnTo, "//") {
		return true
	}

	// Parse absolute URLs
	parsed, err := url.Parse(returnTo)
	if err != nil {
		return false
	}

	// Must be http or https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	// Must match the configured base domain or one of its subdomains.
	host := strings.ToLower(parsed.Hostname())
	baseHost := h.allowedBaseHostname()
	if baseHost != "" && (host == baseHost || strings.HasSuffix(host, "."+baseHost)) {
		return true
	}
	// Allow localhost for development
	if h.isDevelopment && (strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1")) {
		return true
	}

	return false
}

func (h *AdminHandler) allowedBaseHostname() string {
	parsed, err := url.Parse(h.baseURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "admin.") {
		return strings.TrimPrefix(host, "admin.")
	}
	if strings.HasPrefix(host, "api.") {
		return strings.TrimPrefix(host, "api.")
	}
	return host
}

// RegisterAdminRoutes sets up admin panel routes including Grafana reverse proxy.
// If template loading fails, it returns an error but the health endpoint will still work.
func (h *AdminHandler) RegisterAdminRoutes(
	router *gin.Engine,
	contextAuthMiddleware *middleware.ContextAuthMiddleware,
	adminContextMiddleware *middleware.AdminContextMiddleware,
	featureMiddleware ...gin.HandlerFunc,
) error {
	tmpl, err := ParseAdminTemplates()
	if err != nil {
		return fmt.Errorf("parse embedded templates: %w", err)
	}
	router.SetHTMLTemplate(tmpl)
	h.logger.Info("Admin templates loaded from embedded FS")

	// Serve static assets from embedded FS
	staticFS, _ := fs.Sub(web.Static, "static")
	router.StaticFS("/static", http.FS(staticFS))

	// Public routes (authentication) - no /admin prefix since we're on admin subdomain
	router.GET("/login", h.ShowLogin)
	router.POST("/login", h.HandleLogin)
	router.GET("/verify-magic-link", h.VerifyMagicLinkWeb)
	router.GET("/cli-login", h.CompleteCLILogin)

	// Public Grafana health check (for metrics page to check availability before loading)
	router.GET("/grafana-status", h.GrafanaHealthCheck)

	// Grafana reverse proxy (public - Grafana handles its own auth via anonymous access)
	// This must be outside the protected group because Grafana's JavaScript dynamically loads chunks
	// and those requests may not include the mbr_session cookie properly.
	// Security: Grafana binds to localhost and is only reachable via admin subdomain proxy.
	router.Any("/grafana/*proxyPath", h.GrafanaProxy())

	// Dashboard routes support either instance-admin or active workspace context.
	dashboard := router.Group("")
	dashboard.Use(contextAuthMiddleware.AuthRequired())
	dashboard.Use(contextAuthMiddleware.RequireOperationalAccess())
	dashboard.Use(adminContextMiddleware.WithOperationalContext())
	for _, mw := range featureMiddleware {
		dashboard.Use(mw)
	}
	{
		dashboard.GET("/", h.ShowDashboard)
		dashboard.GET("/dashboard", h.ShowDashboard)
	}

	// Logout is valid regardless of active context.
	authOnly := router.Group("")
	authOnly.Use(contextAuthMiddleware.AuthRequired())
	{
		authOnly.GET("/logout", h.Logout)
	}

	// Protected instance-only routes.
	protected := router.Group("")
	protected.Use(contextAuthMiddleware.AuthRequired())
	protected.Use(contextAuthMiddleware.RequireInstanceAccess())
	{
		protected.GET("/metrics", h.ShowMetrics)
		protected.GET("/api-tokens", h.ShowAPITokens)
		protected.GET("/instance", h.ShowInstance)
	}

	return nil
}

// ShowLogin renders the admin login page
func (h *AdminHandler) ShowLogin(c *gin.Context) {
	// Generate a form nonce for this page load
	nonce, err := h.generateFormNonce()
	if err != nil {
		h.logger.WithError(err).Warn("Failed to generate form nonce")
		// Still render the page, validation will fail on submit
	}

	// Preserve return_to parameter for OAuth flow redirect after login
	returnTo := c.Query("return_to")

	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title":     "Admin Login",
		"FormNonce": nonce,
		"ReturnTo":  returnTo,
	})
}

// HandleLogin processes the magic link login request
func (h *AdminHandler) HandleLogin(c *gin.Context) {
	email := c.PostForm("email")
	formNonce := c.PostForm("form_nonce")
	returnTo := c.PostForm("return_to")
	honeypot := c.PostForm(magicLinkHoneypotField)

	// Helper to generate a new nonce for error/success responses
	newNonce := func() string {
		nonce, err := h.generateFormNonce()
		if err != nil {
			// crypto/rand failure is catastrophic - log it
			// The form will fail validation on submit (empty nonce rejected)
			h.logger.WithError(err).Error("Failed to generate form nonce - form submission will fail")
		}
		return nonce
	}

	// SECURITY: hidden honeypot triggers additional telemetry + stricter rate limiting.
	// Keep response generic to avoid bot human-awareness.
	if isMagicLoginHoneypotTriggered(honeypot) {
		h.logger.WithField("ip", c.ClientIP()).Warn("Admin magic-link honeypot triggered")
		if h.authHandler.SessionService != nil {
			if err := h.authHandler.SessionService.CheckMagicLinkHoneypotRateLimit(c.Request.Context(), magicLoginHoneypotFingerprint(c)); err != nil {
				c.HTML(http.StatusOK, "login.html", gin.H{
					"Title":     "Admin Login",
					"Success":   "If this email is registered, you'll receive a magic link shortly.",
					"FormNonce": newNonce(),
				})
				return
			}
		}
	}

	// Validate form nonce FIRST - prevents duplicate submissions and replay attacks
	if !h.validateAndConsumeNonce(formNonce) {
		// Invalid or already-used nonce - this could be a page reload/back button
		c.HTML(http.StatusOK, "login.html", gin.H{
			"Title":     "Admin Login",
			"Error":     "This form has already been submitted. Please try again.",
			"FormNonce": newNonce(),
		})
		return
	}

	if email == "" {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"Title":     "Admin Login",
			"Error":     "Email address is required",
			"FormNonce": newNonce(),
		})
		return
	}

	// SECURITY: Always show the same message to prevent email enumeration
	// Process the request silently - only send email if user exists and has access
	successMessage := "If this email is registered, you'll receive a magic link shortly."

	// Apply rate limiting for web login flow as well (same generic response to avoid enumeration).
	if h.authHandler.SessionService != nil {
		if err := h.authHandler.SessionService.CheckMagicLinkRateLimit(c.Request.Context(), email); err != nil {
			c.HTML(http.StatusOK, "login.html", gin.H{
				"Title":     "Admin Login",
				"Success":   successMessage,
				"FormNonce": newNonce(),
			})
			return
		}
	}

	// Get user by email
	user, err := h.authHandler.SessionService.GetUserByEmail(c.Request.Context(), email)
	if err != nil || !user.CanAccessAdminPanel() {
		// User doesn't exist or doesn't have admin access - show same message
		c.HTML(http.StatusOK, "login.html", gin.H{
			"Title":     "Admin Login",
			"Success":   successMessage,
			"FormNonce": newNonce(),
		})
		return
	}

	// Generate magic link
	magicLink, err := h.authHandler.SessionService.GenerateMagicLinkToken(user.ID, email)
	if err != nil {
		// Log error but show same message to user
		h.logger.WithError(err).Warn("Failed to generate magic link", "email", email)
		c.HTML(http.StatusOK, "login.html", gin.H{
			"Title":     "Admin Login",
			"Success":   successMessage,
			"FormNonce": newNonce(),
		})
		return
	}

	// Save magic link
	if err := h.authHandler.SessionService.SaveMagicLink(c.Request.Context(), magicLink); err != nil {
		// Log error but show same message to user
		h.logger.WithError(err).Warn("Failed to save magic link", "email", email)
		c.HTML(http.StatusOK, "login.html", gin.H{
			"Title":     "Admin Login",
			"Success":   successMessage,
			"FormNonce": newNonce(),
		})
		return
	}

	// Generate magic link URL (uses admin subdomain, no /admin prefix in path)
	magicLinkURL := fmt.Sprintf("%s/verify-magic-link?token=%s", h.adminBaseURL(), magicLink.Token)
	if returnTo != "" {
		magicLinkURL += "&return_to=" + url.QueryEscape(returnTo)
	}

	// Send magic link via email
	if h.emailService != nil {
		if err := h.emailService.SendMagicLinkEmail(c.Request.Context(), email, magicLinkURL); err != nil {
			// Log the error but don't expose it to the user (security)
			h.logger.WithError(err).Warn("Failed to send magic link email", "email", email)
			// SECURITY: Only log magic link in development mode
			if h.isDevelopment {
				h.logger.Info("Admin magic link (email failed, DEV ONLY)", "url", magicLinkURL)
			}
		}
	} else if h.isDevelopment {
		// Development mode only - log the link
		h.logger.Info("Admin magic link (DEV ONLY)", "url", magicLinkURL)
	}

	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title":     "Admin Login",
		"Success":   successMessage,
		"FormNonce": newNonce(),
	})
}

// VerifyMagicLinkWeb verifies the magic link token and creates an admin session (web flow)
func (h *AdminHandler) VerifyMagicLinkWeb(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"Title": "Admin Login",
			"Error": "Invalid magic link",
		})
		return
	}

	// Get and validate magic link
	magicLink, err := h.authHandler.SessionService.GetMagicLink(c.Request.Context(), token)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"Title": "Admin Login",
			"Error": "Invalid or expired magic link",
		})
		return
	}

	// Validate magic link
	if err := h.authHandler.SessionService.ValidateMagicLinkToken(magicLink); err != nil {
		// Log detailed error for debugging, but return generic message to prevent information leakage
		h.logger.Warn("Magic link validation failed", "ip", c.ClientIP(), "error", err.Error())
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"Title": "Admin Login",
			"Error": "Invalid or expired magic link",
		})
		return
	}

	// Mark magic link as used (atomic check-and-set to prevent race conditions)
	if err := h.authHandler.SessionService.MarkMagicLinkUsed(c.Request.Context(), token); err != nil {
		if contracts.IsAlreadyUsed(err) {
			// Token was already used by another concurrent request
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{
				"Title": "Admin Login",
				"Error": "Invalid or expired magic link",
			})
			return
		}
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"Title": "Admin Login",
			"Error": "Failed to mark magic link as used",
		})
		return
	}

	// Get user
	user, err := h.authHandler.SessionService.GetUserByID(c.Request.Context(), magicLink.UserID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"Title": "Admin Login",
			"Error": "Failed to retrieve user",
		})
		return
	}

	// Create session with all available contexts using SessionService
	// CreateSession returns both the session (with TokenHash) and the plaintext token
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()
	session, sessionToken, err := h.authHandler.SessionService.CreateSession(c.Request.Context(), user, ipAddress, userAgent)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"Title": "Admin Login",
			"Error": "Failed to create session",
		})
		return
	}

	// Set session cookie with the plaintext token (hash is stored in DB)
	// Cookie domain enables cross-subdomain auth (e.g., ".example.com")
	maxAge := int(session.ExpiresAt.Sub(session.CreatedAt).Seconds())
	secure := !h.isDevelopment
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("mbr_session", sessionToken, maxAge, "/", h.authHandler.CookieDomain(), secure, true)

	// Check for return_to parameter (used for OAuth flow redirect after login)
	returnTo := c.Query("return_to")
	if returnTo != "" {
		// Security: validate the return_to URL to prevent open redirect attacks
		if h.isValidReturnURL(returnTo) {
			c.Redirect(http.StatusFound, returnTo)
			return
		}
		// Invalid return_to URL, log and fall through to dashboard
		h.logger.Warn("Invalid return_to URL in magic link", "return_to", returnTo)
	}

	// Redirect to dashboard
	c.Redirect(http.StatusFound, "/dashboard")
}

// CompleteCLILogin finalizes a CLI browser login request once the browser has
// an authenticated Move Big Rocks admin session.
func (h *AdminHandler) CompleteCLILogin(c *gin.Context) {
	requestID := strings.TrimSpace(c.Query("request_id"))
	if requestID == "" {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"Title": "CLI Login",
			"Error": "Missing CLI login request ID",
		})
		return
	}

	if h.authHandler == nil || h.authHandler.SessionService == nil || h.authHandler.cliLogin == nil {
		c.HTML(http.StatusServiceUnavailable, "login.html", gin.H{
			"Title": "CLI Login",
			"Error": "CLI browser login is not configured",
		})
		return
	}

	session, err := h.sessionFromCookie(c)
	if err != nil || session == nil {
		returnTo := c.Request.URL.RequestURI()
		c.Redirect(http.StatusFound, "/login?return_to="+url.QueryEscape(returnTo))
		return
	}

	if !sessionHasInstanceAccess(session) {
		c.Data(http.StatusForbidden, "text/html; charset=utf-8", []byte(`<!doctype html>
<html>
  <head><meta charset="utf-8"><title>CLI Login</title></head>
  <body>
    <h1>CLI login requires instance access</h1>
    <p>This browser account does not currently have an instance admin or operator context.</p>
  </body>
</html>`))
		return
	}

	user, err := h.authHandler.SessionService.GetUserByID(c.Request.Context(), session.UserID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"Title": "CLI Login",
			"Error": "Failed to load the authenticated user",
		})
		return
	}

	cliSession, cliSessionToken, err := h.authHandler.SessionService.CreateSession(
		c.Request.Context(),
		user,
		c.ClientIP(),
		c.Request.UserAgent()+" mbr-cli-browser-login",
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"Title": "CLI Login",
			"Error": "Failed to create CLI session",
		})
		return
	}
	if sessionHasInstanceAccess(cliSession) {
		if err := h.authHandler.SessionService.SwitchContext(c.Request.Context(), cliSession, platformdomain.ContextTypeInstance, nil); err != nil {
			c.HTML(http.StatusInternalServerError, "login.html", gin.H{
				"Title": "CLI Login",
				"Error": "Failed to switch CLI session into instance context",
			})
			return
		}
	}

	if err := h.authHandler.cliLogin.Authorize(requestID, user.ID, cliSessionToken); err != nil {
		statusCode := http.StatusBadRequest
		message := "CLI login request is invalid or expired"
		if errors.Is(err, platformservices.ErrCLILoginRequestNotFound) {
			statusCode = http.StatusNotFound
		} else if errors.Is(err, platformservices.ErrCLILoginExpired) {
			statusCode = http.StatusGone
		}
		c.HTML(statusCode, "login.html", gin.H{
			"Title": "CLI Login",
			"Error": message,
		})
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html>
<html>
  <head><meta charset="utf-8"><title>CLI Login Complete</title></head>
  <body>
    <h1>CLI login complete</h1>
    <p>You can close this window and return to your terminal.</p>
  </body>
</html>`))
}

func (h *AdminHandler) sessionFromCookie(c *gin.Context) (*platformdomain.Session, error) {
	token, err := c.Cookie("mbr_session")
	if err != nil || strings.TrimSpace(token) == "" {
		return nil, err
	}
	return h.authHandler.SessionService.ValidateSession(c.Request.Context(), token)
}

func sessionHasInstanceAccess(session *platformdomain.Session) bool {
	if session == nil {
		return false
	}
	for _, ctx := range session.AvailableContexts {
		if ctx.Type != platformdomain.ContextTypeInstance {
			continue
		}
		if platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(ctx.Role)).IsOperator() {
			return true
		}
	}
	return false
}

// ShowDashboard renders the admin dashboard
func (h *AdminHandler) ShowDashboard(c *gin.Context) {
	if workspaceID, _, ok := currentWorkspaceScope(c); ok && workspaceID != "" {
		c.Redirect(http.StatusFound, "/cases")
		return
	}

	// Get dashboard stats directly from service
	stats, err := h.statsService.GetDashboardStats(c.Request.Context())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "dashboard.html", gin.H{
			"Error": "Failed to load dashboard stats",
		})
		return
	}

	// Track partial load failures so the dashboard can show a warning
	var loadErrors []string

	// Get recent activity
	recentActivity, err := h.statsService.GetRecentActivity(c.Request.Context(), 10)
	if err != nil {
		recentActivity = []platformservices.RecentActivity{}
		loadErrors = append(loadErrors, "recent activity")
	}

	// Get system health
	systemHealth, err := h.statsService.GetSystemHealth(c.Request.Context())
	if err != nil {
		systemHealth = []platformservices.SystemHealth{}
		loadErrors = append(loadErrors, "system health")
	}

	// Get quick stats
	quickStats, err := h.statsService.GetQuickStats(c.Request.Context())
	if err != nil {
		quickStats = []platformservices.QuickStat{}
		loadErrors = append(loadErrors, "quick stats")
	}

	// Get user info from context (set by auth middleware)
	_ = GetContextValues(c)

	pageData := buildAdminTemplateContext(c, "dashboard", "Dashboard", "Platform overview and key metrics")
	pageData["Stats"] = stats
	pageData["RecentActivity"] = recentActivity
	pageData["SystemHealth"] = systemHealth
	pageData["QuickStats"] = quickStats
	if len(loadErrors) > 0 {
		pageData["PartialLoadError"] = "Some dashboard data failed to load: " + strings.Join(loadErrors, ", ")
	}
	c.HTML(http.StatusOK, "dashboard.html", pageData)
}

// ShowMetrics renders the metrics page with Grafana dashboards
func (h *AdminHandler) ShowMetrics(c *gin.Context) {
	// Get user info from context (set by auth middleware)
	_ = GetContextValues(c)
	// Issue short-lived access cookie so Grafana proxy can remain unauthenticated
	// while still being reachable only through authenticated admin panel navigation.
	h.issueGrafanaAccessCookie(c)

	pageData := buildAdminTemplateContext(c, "metrics", "System Metrics", "Real-time technical performance metrics powered by Grafana")
	c.HTML(http.StatusOK, "metrics.html", pageData)
}

// ShowAPITokens renders the API tokens management page
func (h *AdminHandler) ShowAPITokens(c *gin.Context) {
	_ = GetContextValues(c)

	pageData := buildAdminTemplateContext(c, "api_tokens", "Agent Tokens", "Create and manage agent tokens for the CLI and automated workflows")
	c.HTML(http.StatusOK, "api_tokens.html", pageData)
}

// ShowInstance renders the instance inspector page showing current configuration.
func (h *AdminHandler) ShowInstance(c *gin.Context) {
	_ = GetContextValues(c)

	pageData := buildAdminTemplateContext(c, "instance", "Instance", "Current configuration overview")

	if h.appConfig != nil {
		cfg := h.appConfig
		pageData["Config"] = map[string]interface{}{
			// General
			"Environment": cfg.Server.Environment,
			"Domain":      cfg.Server.Domain,
			"BaseURL":     cfg.Server.BaseURL,
			"AdminURL":    cfg.Server.AdminBaseURL,
			"APIURL":      cfg.Server.APIBaseURL,

			// Auth
			"SessionExpiry":    cfg.Auth.JWTExpiry.String(),
			"MagicLinkTTL":     cfg.Auth.MagicLinkTTL.String(),
			"SignupEnabled":    cfg.Features.EnableSignup,
			"MagicLinkEnabled": cfg.Features.EnableMagicLink,

			// Email
			"EmailBackend": cfg.Email.Backend,
			"FromEmail":    cfg.Email.FromEmail,
			"FromName":     cfg.Email.FromName,

			// Storage
			"StorageType":       cfg.Storage.Type,
			"S3Region":          cfg.Storage.Operational.Region,
			"S3Bucket":          cfg.Storage.Operational.Bucket,
			"EncryptionEnabled": cfg.Storage.EncryptionEnabled,

			// Security
			"RateLimitEnabled": cfg.Security.RateLimitEnabled,
			"RateLimitPerMin":  cfg.Security.RateLimitPerMin,
			"DefaultUserRole":  cfg.Security.DefaultUserRole,

			// Limits
			"MaxUploadSize":  cfg.Limits.MaxUploadSize / (1024 * 1024),
			"MaxRequestSize": cfg.Limits.MaxRequestSize / (1024 * 1024),
			"MetricsEnabled": cfg.Limits.EnableMetrics,

			// Integrations
			"HasOpenAI":        cfg.Integrations.OpenAIAPIKey != "",
			"HasAnthropic":     cfg.Integrations.AnthropicAPIKey != "",
			"HasElasticsearch": cfg.Integrations.ElasticsearchURL != "",
			"HasClamAV":        cfg.Integrations.ClamAVAddr != "",

			// Cache
			"CacheEnabled": cfg.Cache.Enabled,
			"CacheTTL":     cfg.Cache.TTL.String(),

			// Database
			"DatabaseDriver": cfg.Database.EffectiveDriver(),
			"DatabaseTarget": cfg.Database.RedactedDSN(),
		}
	}

	c.HTML(http.StatusOK, "instance.html", pageData)
}

// Logout logs out the admin user
func (h *AdminHandler) Logout(c *gin.Context) {
	// Get token from cookie (ignore error: no cookie means already logged out)
	token, _ := c.Cookie("mbr_session")
	if token != "" {
		// Hash the token to find and delete the session
		hash := sha256.Sum256([]byte(token))
		tokenHash := hex.EncodeToString(hash[:])

		// Best-effort session cleanup: log error but continue to clear cookie
		if err := h.authHandler.SessionService.DeleteSessionByHash(c.Request.Context(), tokenHash); err != nil {
			h.logger.WithError(err).Warn("Failed to delete session from store during logout")
		}
	}

	// Clear session cookie
	cookie := &http.Cookie{
		Name:     "mbr_session",
		Value:    "",
		Domain:   h.authHandler.CookieDomain(),
		Path:     "/",
		MaxAge:   -1, // Delete cookie
		HttpOnly: true,
		Secure:   !h.isDevelopment,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(c.Writer, cookie)

	c.Redirect(http.StatusFound, "/login")
}

// GrafanaHealthCheck checks if Grafana is available (public endpoint for metrics page)
func (h *AdminHandler) GrafanaHealthCheck(c *gin.Context) {
	if !h.hasGrafanaAccess(c) {
		middleware.RespondWithError(c, http.StatusForbidden, "access denied")
		return
	}

	if h.grafanaURL == "" {
		middleware.RespondWithError(c, http.StatusServiceUnavailable, "grafana not configured")
		return
	}

	// Make a quick request to Grafana health endpoint
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(h.grafanaURL + "/api/health")
	if err != nil {
		middleware.RespondWithError(c, http.StatusServiceUnavailable, "grafana not reachable")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		c.JSON(http.StatusOK, gin.H{"status": "available"})
	} else {
		middleware.RespondWithError(c, http.StatusServiceUnavailable, "grafana unhealthy")
	}
}

// GrafanaProxy creates a reverse proxy handler for Grafana
func (h *AdminHandler) GrafanaProxy() gin.HandlerFunc {
	// Parse Grafana URL - validated at startup, log error and return 502 if invalid
	target, err := url.Parse(h.grafanaURL)
	if err != nil {
		return func(c *gin.Context) {
			middleware.RespondWithError(c, http.StatusBadGateway, "grafana proxy misconfigured")
		}
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize the director to handle the proxy path properly
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host

		// Keep /grafana prefix - Grafana needs it when GF_SERVER_SERVE_FROM_SUB_PATH=true
	}

	// Allow iframe embedding by removing security headers set by global middleware
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")
		return nil
	}

	return func(c *gin.Context) {
		if !h.hasGrafanaAccess(c) {
			middleware.RespondWithError(c, http.StatusForbidden, "access denied")
			return
		}
		c.Writer.Header().Del("X-Frame-Options")
		c.Writer.Header().Del("Content-Security-Policy")
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
