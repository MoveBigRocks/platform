package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	automationresolvers "github.com/movebigrocks/platform/internal/automation/resolvers"
	"github.com/movebigrocks/platform/internal/graph"
	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/container"
	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	v1routes "github.com/movebigrocks/platform/internal/infrastructure/routes/v1"
	"github.com/movebigrocks/platform/internal/infrastructure/tracing"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/platform/extensionruntime"
	platformhandlers "github.com/movebigrocks/platform/internal/platform/handlers"
	platformresolvers "github.com/movebigrocks/platform/internal/platform/resolvers"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	servicehandlers "github.com/movebigrocks/platform/internal/service/handlers"
	serviceresolvers "github.com/movebigrocks/platform/internal/service/resolvers"
	"github.com/movebigrocks/platform/pkg/logger"
)

func configureTrustedProxies(router *gin.Engine, cfg *config.Config, log *logger.Logger, routerName string) {
	if cfg == nil {
		return
	}
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		if log != nil {
			log.Warn("Failed to set trusted proxies; falling back to safe default",
				"router", routerName,
				"error", err,
			)
		}
		_ = router.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	}
}

// createAdminRouter creates the instance admin router (admin.example.com)
func createAdminRouter(
	cfg *config.Config,
	c *container.Container,
	adminEmails []string,
	serviceTargets *extensionruntime.Registry,
) *gin.Engine {
	log := logger.New().WithField("router", "admin")

	router := gin.New()
	configureTrustedProxies(router, cfg, log, "admin")

	// Disable automatic redirects that interfere with Grafana proxy
	router.RedirectTrailingSlash = false
	router.RedirectFixedPath = false

	// Admin middleware
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.MaxBodySize(middleware.LargeMaxBodySize)) // 25MB limit for admin payloads including attachment uploads
	router.Use(middleware.ValidatePathParams())                     // Prevent path traversal
	router.Use(middleware.InputValidation())                        // Content-Type validation
	router.Use(middleware.RequestID())
	router.Use(tracing.Middleware("mbr-admin"))
	router.Use(gin.Logger())
	router.Use(middleware.Recovery())

	// Create auth handler
	authHandler := platformhandlers.NewAuthHandler(
		c.Platform.Session,
		cfg.Server.BaseURL,
		cfg.Server.Environment,
		adminEmails,
		cfg.Auth.CookieDomain,
	).WithCLILogin(cfg.Server.AdminBaseURL, c.Platform.CLILogin)

	// Create context-aware auth middleware (with store for workspace slug lookups)
	contextAuthMiddleware := middleware.NewContextAuthMiddleware(c.Platform.Session).
		WithStore(c.Store).
		WithEnvironment(cfg.Server.Environment).
		WithCookieDomain(cfg.Auth.CookieDomain)

	// Create admin email service for magic links
	var emailAPIKey string
	switch cfg.Email.Backend {
	case "sendgrid":
		emailAPIKey = cfg.Email.SendGridAPIKey
	case "postmark":
		emailAPIKey = cfg.Email.PostmarkServerToken
	default:
		emailAPIKey = "" // mock provider doesn't need a key
	}
	adminEmailService := platformservices.NewAdminEmailService(platformservices.AdminEmailConfig{
		Provider: cfg.Email.Backend,
		APIKey:   emailAPIKey,
		From:     cfg.Email.FromEmail,
		FromName: cfg.Email.FromName,
	})

	// Create nonce service for CSRF protection
	nonceService := platformservices.NewNonceService(
		platformservices.WithNonceLogger(log),
	)

	// Create admin handler
	adminHandler := platformhandlers.NewAdminHandler(
		cfg.Integrations.GrafanaURL,
		authHandler,
		c.Platform.Stats,
		adminEmailService,
		nonceService,
		cfg.Server.BaseURL,
		cfg.Server.Environment == "development",
		cfg.Auth.JWTSecret,
		cfg,
	)

	// Create admin context middleware for RLS bypass on cross-tenant queries.
	adminContextMiddleware := middleware.NewAdminContextMiddleware(c.Store)

	// Create admin management handler using container services
	adminManagementHandler := platformhandlers.NewAdminManagementHandler(
		c.Platform.Workspace,
		c.Platform.User,
		c.Platform.Stats,
		c.Platform.Extension,
		c.Service.Case,
		c.Automation.Rule,
		c.Automation.Form,
		c.Observability.Issue,
		c.Observability.Project,
	)
	adminFeatureMiddleware := adminManagementHandler.FeatureContextMiddleware()
	attachmentUploadHandler := servicehandlers.NewAttachmentUploadHandler(c.Service.Attachment, c.Store.Cases())
	adminServiceTargets := serviceTargets
	if adminServiceTargets == nil {
		adminServiceTargets = extensionruntime.NewRegistry(c)
	}

	// Health check endpoint (public, unauthenticated)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "admin"})
	})

	// Register admin panel routes (login, dashboard, etc.)
	if err := adminHandler.RegisterAdminRoutes(router, contextAuthMiddleware, adminContextMiddleware); err != nil {
		log.Error("Failed to register admin routes", "error", err)
		// Templates failed to load - register a fallback error page
		// Health endpoint still works for monitoring, but all other routes show error
		router.NoRoute(func(c *gin.Context) {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusServiceUnavailable, `<!DOCTYPE html>
<html><head><title>Admin Panel Unavailable</title></head>
<body style="font-family: system-ui; max-width: 600px; margin: 50px auto; padding: 20px;">
<h1>⚠️ Admin Panel Temporarily Unavailable</h1>
<p>The admin panel failed to start due to a template error. The API and other services are still running.</p>
<p>Check the server logs for details.</p>
<p><a href="/health">Health Check</a></p>
</body></html>`)
		})
		return router
	}

	adminExtensionRoutes := router.Group("/admin/extensions")
	adminExtensionRoutes.Use(contextAuthMiddleware.AuthRequired())
	adminExtensionRoutes.Use(contextAuthMiddleware.RequireOperationalAccess())
	adminExtensionRoutes.Use(adminFeatureMiddleware)
	adminExtensionRoutes.Any("/*extensionPath", func(ctx *gin.Context) {
		workspaceID := ctx.GetString("workspace_id")
		if serveResolvedAdminExtensionServiceRoute(ctx, c.Platform.Extension, adminServiceTargets, workspaceID, cfg) {
			return
		}
		serveResolvedExtensionRoute(ctx, c.Platform.Extension, workspaceID, true)
	})

	// Auth API endpoints (public)
	router.POST("/auth/magic-link", authHandler.HandleMagicLinkRequest)
	router.POST("/auth/logout", authHandler.Logout)

	// Protected auth routes
	authProtected := router.Group("/auth")
	authProtected.Use(contextAuthMiddleware.AuthRequired())
	{
		authProtected.GET("/context", authHandler.GetCurrentContext)
		authProtected.POST("/switch-context", authHandler.SwitchContext)
	}

	// Shared operational pages work in either instance or workspace context.
	sharedPages := router.Group("")
	sharedPages.Use(contextAuthMiddleware.AuthRequired())
	sharedPages.Use(contextAuthMiddleware.RequireOperationalAccess())
	sharedPages.Use(adminContextMiddleware.WithOperationalContext())
	sharedPages.Use(adminFeatureMiddleware)
	{
		sharedPages.GET("/cases", adminManagementHandler.ShowCases)
		sharedPages.GET("/cases/:id", adminManagementHandler.ShowCaseDetail)
		sharedPages.GET("/forms", adminManagementHandler.ShowForms)
	}

	// Protected instance operations pages for operator+ roles.
	// Admin context middleware enables queries across all workspaces (bypasses RLS).
	protected := router.Group("")
	protected.Use(contextAuthMiddleware.AuthRequired())
	protected.Use(contextAuthMiddleware.RequireInstanceAccess())
	protected.Use(adminContextMiddleware.WithAdminContext())
	protected.Use(adminFeatureMiddleware)
	{
		protected.GET("/workspaces", adminManagementHandler.ShowWorkspaces)

		// Case action routes
		protected.POST("/cases/:id/assign", adminManagementHandler.AssignCase)
		protected.POST("/cases/:id/unassign", adminManagementHandler.UnassignCase)
		protected.POST("/cases/:id/priority", adminManagementHandler.SetCasePriority)
		protected.POST("/cases/:id/status", adminManagementHandler.SetCaseStatus)
		protected.POST("/cases/:id/resolve", adminManagementHandler.ResolveCase)
		protected.POST("/cases/:id/close", adminManagementHandler.CloseCase)
		protected.POST("/cases/:id/reopen", adminManagementHandler.ReopenCase)
		protected.POST("/cases/:id/note", adminManagementHandler.AddCaseNote)
		protected.POST("/cases/:id/reply", adminManagementHandler.ReplyCaseToCustomer)
		protected.POST("/cases/:id/tag", adminManagementHandler.AddCaseTag)
		protected.POST("/cases/:id/tag/:tag", adminManagementHandler.RemoveCaseTag) // POST keeps SSR form handling simple
		protected.POST("/cases/:id/category", adminManagementHandler.SetCaseCategory)

		// Automation pages
		protected.GET("/rules", adminManagementHandler.ShowRules)
		protected.GET("/rules/new", adminManagementHandler.ShowRuleEdit)
		protected.GET("/rules/:id", adminManagementHandler.ShowRuleEdit)
		protected.GET("/forms/new", adminManagementHandler.ShowFormEdit)
		protected.GET("/forms/:id", adminManagementHandler.ShowFormEdit)
		protected.GET("/forms/:id/builder", adminManagementHandler.ShowFormBuilder)
		protected.GET("/forms/builder", adminManagementHandler.ShowFormBuilder) // New form via builder

	}

	// Protected user management pages - admin role required (supports users)
	userPages := router.Group("")
	userPages.Use(contextAuthMiddleware.AuthRequired())
	userPages.Use(contextAuthMiddleware.RequireInstanceAccess(platformdomain.InstanceRoleAdmin))
	userPages.Use(adminContextMiddleware.WithAdminContext())
	userPages.Use(adminFeatureMiddleware)
	{
		userPages.GET("/users", adminManagementHandler.ShowUsers)
	}

	// Admin action routes (internal SSR support surfaces; no public contract)
	// These are form/action endpoints backing admin browser UI workflows.
	adminActionAPI := router.Group("/admin/actions")
	adminActionAPI.Use(contextAuthMiddleware.AuthRequired())
	adminActionAPI.Use(contextAuthMiddleware.RequireInstanceAccess())
	adminActionAPI.Use(adminContextMiddleware.WithAdminContext())
	adminActionAPI.Use(adminFeatureMiddleware)
	{
		// Workspace APIs
		adminActionAPI.GET("/workspaces/:id", adminManagementHandler.GetWorkspace)
		adminActionAPI.POST("/workspaces", adminManagementHandler.CreateWorkspace)
		adminActionAPI.PUT("/workspaces/:id", adminManagementHandler.UpdateWorkspace)
		adminActionAPI.DELETE("/workspaces/:id", adminManagementHandler.DeleteWorkspace)

		// Rule APIs
		adminActionAPI.POST("/rules", adminManagementHandler.CreateRule)
		adminActionAPI.PUT("/rules/:id", adminManagementHandler.UpdateRule)
		adminActionAPI.DELETE("/rules/:id", adminManagementHandler.DeleteRule)

		// Form APIs
		adminActionAPI.POST("/forms", adminManagementHandler.CreateForm)
		adminActionAPI.PUT("/forms/:id", adminManagementHandler.UpdateForm)
		adminActionAPI.DELETE("/forms/:id", adminManagementHandler.DeleteForm)

		// Attachment APIs
		adminActionAPI.POST("/attachments", attachmentUploadHandler.HandleAdminUpload)

	}

	// User management action routes (admin role required)
	adminUserAPI := router.Group("/admin/actions")
	adminUserAPI.Use(contextAuthMiddleware.AuthRequired())
	adminUserAPI.Use(contextAuthMiddleware.RequireInstanceAccess(platformdomain.InstanceRoleAdmin))
	adminUserAPI.Use(adminContextMiddleware.WithAdminContext())
	{
		adminUserAPI.GET("/users/:id", adminManagementHandler.GetUser)
		adminUserAPI.GET("/users/:id/workspaces", adminManagementHandler.GetUserWorkspaces)
		adminUserAPI.POST("/users", adminManagementHandler.CreateUser)
		adminUserAPI.PUT("/users/:id", adminManagementHandler.UpdateUser)
		adminUserAPI.PATCH("/users/:id/status", adminManagementHandler.UpdateUserStatus)
		adminUserAPI.DELETE("/users/:id", adminManagementHandler.DeleteUser)
	}

	// GraphQL API for admin workflows (session authenticated).
	// Browser pages such as API tokens and admin tooling call this endpoint.
	// The path is explicitly admin-internal: /admin/graphql.
	// Using new graph-gophers/graphql-go implementation.
	gqlResolver := graph.NewRootResolver(graph.Config{
		Service: &serviceresolvers.Config{
			QueueService:        c.Service.Queue,
			ConversationService: c.Service.Conversation,
			CatalogService:      c.Service.Catalog,
			FormSpecService:     c.Service.FormSpecs,
			ConceptService:      c.Service.Concepts,
			KnowledgeService:    c.Service.Knowledge,
			CaseService:         c.Service.Case,
			UserService:         c.Platform.User,
			ContactService:      c.Platform.Contact,
			AgentService:        c.Platform.Agent,
			ExtensionChecker:    c.Platform.Extension,
		},
		Platform: &platformresolvers.Config{
			UserService:      c.Platform.User,
			WorkspaceService: c.Platform.Workspace,
			AgentService:     c.Platform.Agent,
			ContactService:   c.Platform.Contact,
			ExtensionService: c.Platform.Extension,
		},
		Automation: &automationresolvers.Config{
			RuleService:      c.Automation.Rule,
			FormService:      c.Automation.Form,
			WorkspaceService: c.Platform.Workspace,
		},
	})
	gqlServer := graph.MustParseSchema(gqlResolver)

	// GraphQL endpoint for admin portal (session auth, instance access required)
	gqlProtected := router.Group("")
	gqlProtected.Use(contextAuthMiddleware.AuthRequired())
	gqlProtected.Use(contextAuthMiddleware.RequireInstanceAccess())
	gqlProtected.POST("/admin/graphql", graph.GinHandler(gqlServer))

	// GraphQL playground (development only)
	if cfg.Server.Environment != "production" {
		router.GET("/admin/graphql", gin.WrapH(graph.NewPlaygroundHandler("/admin/graphql")))
	}

	return router
}

// createAPIRouter creates the API router (api.example.com)
func createAPIRouter(
	cfg *config.Config,
	c *container.Container,
	postmarkHandlers *servicehandlers.PostmarkWebhookHandlers,
	publicFormHandler *servicehandlers.FormPublicHandler,
	serviceTargets *extensionruntime.Registry,
) *gin.Engine {
	router := gin.New()
	configureTrustedProxies(router, cfg, logger.New().WithField("router", "api"), "api")

	// API middleware
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.MaxBodySize(middleware.LargeMaxBodySize)) // 25MB limit for authenticated uploads
	router.Use(middleware.ValidatePathParams())                     // Prevent path traversal
	router.Use(middleware.InputValidation())                        // Content-Type validation
	router.Use(middleware.RequestID())
	router.Use(tracing.Middleware("mbr-services"))
	router.Use(middleware.PrometheusMetrics())
	router.Use(gin.Logger())
	router.Use(middleware.Recovery())

	authHandler := platformhandlers.NewAuthHandler(
		c.Platform.Session,
		cfg.Server.BaseURL,
		cfg.Server.Environment,
		nil,
		cfg.Auth.CookieDomain,
	).WithCLILogin(cfg.Server.AdminBaseURL, c.Platform.CLILogin)

	// Liveness check - simple check that the service is running
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Readiness check - checks if service can handle requests
	router.GET("/health/ready", func(ctx *gin.Context) {
		if c.HealthChecker == nil {
			ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
			return
		}
		response := c.HealthChecker.CheckReadiness(ctx.Request.Context())
		statusCode := http.StatusOK
		if response.Status == "unhealthy" {
			statusCode = http.StatusServiceUnavailable
		}
		ctx.JSON(statusCode, response)
	})

	// Detailed health check - checks all components
	router.GET("/health", func(ctx *gin.Context) {
		if c.HealthChecker == nil {
			ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
			return
		}
		response := c.HealthChecker.Check(ctx.Request.Context())
		statusCode := http.StatusOK
		if response.Status == "unhealthy" {
			statusCode = http.StatusServiceUnavailable
		} else if response.Status == "degraded" {
			statusCode = http.StatusOK // Still return 200 for degraded, but include status
		}
		ctx.JSON(statusCode, response)
	})

	// Expose Prometheus metrics (protected - requires token or localhost)
	router.GET("/metrics", middleware.MetricsAuth(cfg.Limits.MetricsToken), gin.WrapH(promhttp.Handler()))

	// Swagger API documentation (development only)
	if cfg.Server.Environment != "production" {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// API v1 routes (Postmark webhooks, public forms, etc.)
	v1 := router.Group("/v1")
	v1Router := v1routes.NewRouter()
	v1Router.SetPostmarkHandlers(postmarkHandlers)
	v1Router.SetPublicFormHandler(publicFormHandler)
	v1Router.RegisterRoutes(v1)

	// CLI browser login bootstrap endpoints. These stay unauthenticated because
	// the browser session is completed on the admin subdomain, and the CLI polls
	// using the high-entropy one-time poll token issued at start.
	router.POST("/auth/cli/start", authHandler.StartCLILogin)
	router.POST("/auth/cli/poll", authHandler.PollCLILogin)

	principalAuth := middleware.NewPrincipalAuthMiddleware(c.Store.Agents())

	if serviceTargets == nil {
		serviceTargets = extensionruntime.NewRegistry(c)
	}

	router.NoRoute(func(ctx *gin.Context) {
		serveResolvedExtensionServiceRoute(ctx, c.Platform.Extension, serviceTargets, cfg, principalAuth)
	})

	// GraphQL API (for agents and programmatic access)
	// Using new graph-gophers/graphql-go implementation
	apiGqlResolver := graph.NewRootResolver(graph.Config{
		Service: &serviceresolvers.Config{
			QueueService:        c.Service.Queue,
			ConversationService: c.Service.Conversation,
			CatalogService:      c.Service.Catalog,
			FormSpecService:     c.Service.FormSpecs,
			ConceptService:      c.Service.Concepts,
			KnowledgeService:    c.Service.Knowledge,
			CaseService:         c.Service.Case,
			UserService:         c.Platform.User,
			ContactService:      c.Platform.Contact,
			AgentService:        c.Platform.Agent,
			ExtensionChecker:    c.Platform.Extension,
		},
		Platform: &platformresolvers.Config{
			UserService:      c.Platform.User,
			WorkspaceService: c.Platform.Workspace,
			AgentService:     c.Platform.Agent,
			ContactService:   c.Platform.Contact,
			ExtensionService: c.Platform.Extension,
		},
		Automation: &automationresolvers.Config{
			RuleService:      c.Automation.Rule,
			FormService:      c.Automation.Form,
			WorkspaceService: c.Platform.Workspace,
		},
	})
	apiGqlServer := graph.MustParseSchema(apiGqlResolver)

	attachmentUploadHandler := servicehandlers.NewAttachmentUploadHandler(c.Service.Attachment, c.Store.Cases())

	// Attachment upload endpoint for agents and non-interactive tooling.
	router.POST("/attachments", principalAuth.AuthenticateAgent(), attachmentUploadHandler.HandleAgentUpload)

	// GraphQL endpoint for agents and command-line tooling.
	router.POST("/graphql", principalAuth.AuthenticateAgent(), graph.GinHandler(apiGqlServer))

	// GraphQL playground (development only, no auth required)
	if cfg.Server.Environment != "production" {
		router.GET("/graphql", gin.WrapH(graph.NewPlaygroundHandler("/graphql")))
	}

	return router
}

// createPublicRouter creates the public runtime router (app.example.com)
func createPublicRouter(
	cfg *config.Config,
	sessionService *platformservices.SessionService,
	extensionService *platformservices.ExtensionService,
	cliLoginService *platformservices.CLILoginService,
	sandboxService *platformservices.SandboxService,
	adminEmails []string,
	version, gitCommit, buildDate string,
) *gin.Engine {
	router := gin.New()
	configureTrustedProxies(router, cfg, logger.New().WithField("router", "public"), "public")

	// Public middleware
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.MaxBodySize(middleware.DefaultMaxBodySize)) // 1MB limit for public API
	router.Use(middleware.ValidatePathParams())                       // Prevent path traversal
	router.Use(middleware.InputValidation())                          // Content-Type validation
	router.Use(middleware.RequestID())
	router.Use(tracing.Middleware("mbr-public"))
	router.Use(gin.Logger())
	router.Use(middleware.Recovery())

	// Create auth handler
	authHandler := platformhandlers.NewAuthHandler(
		sessionService,
		cfg.Server.BaseURL,
		cfg.Server.Environment,
		adminEmails,
		cfg.Auth.CookieDomain,
	).WithCLILogin(cfg.Server.AdminBaseURL, cliLoginService)

	// Create context-aware auth middleware
	contextAuthMiddleware := middleware.NewContextAuthMiddleware(sessionService).
		WithEnvironment(cfg.Server.Environment).
		WithCookieDomain(cfg.Auth.CookieDomain)
	sandboxHandler := platformhandlers.NewSandboxPublicHandler(sandboxService)

	router.POST("/api/public/sandboxes", sandboxHandler.CreateSandbox)
	router.GET("/api/public/sandboxes/:id", sandboxHandler.GetSandbox)
	router.GET("/api/public/sandboxes/:id/export", sandboxHandler.ExportSandbox)
	router.POST("/api/public/sandboxes/:id/extend", sandboxHandler.ExtendSandbox)
	router.DELETE("/api/public/sandboxes/:id", sandboxHandler.DestroySandbox)
	router.GET("/sandbox/verify", sandboxHandler.VerifySandbox)

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service":    "Move Big Rocks Platform",
			"status":     "ok",
			"version":    version,
			"git_commit": gitCommit,
			"build_date": buildDate,
		})
	})

	// Prometheus metrics (protected - requires token or localhost)
	router.GET("/metrics", middleware.MetricsAuth(cfg.Limits.MetricsToken), gin.WrapH(promhttp.Handler()))

	// Auth endpoints (magic link)
	router.POST("/auth/magic-link", authHandler.HandleMagicLinkRequest)
	router.GET("/auth/verify-magic-link", authHandler.VerifyMagicLink)
	router.POST("/auth/cli/start", authHandler.StartCLILogin)
	router.POST("/auth/cli/poll", authHandler.PollCLILogin)
	router.POST("/auth/logout", authHandler.Logout)

	// Protected auth routes
	authProtected := router.Group("/auth")
	authProtected.Use(contextAuthMiddleware.AuthRequired())
	{
		authProtected.GET("/context", authHandler.GetCurrentContext)
		authProtected.POST("/switch-context", authHandler.SwitchContext)
	}

	router.NoRoute(func(c *gin.Context) {
		serveResolvedExtensionRoute(c, extensionService, "", false)
	})

	return router
}

func serveResolvedExtensionRoute(
	ctx *gin.Context,
	extensionService *platformservices.ExtensionService,
	workspaceID string,
	admin bool,
) {
	if extensionService == nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	if ctx.Request.Method != http.MethodGet && ctx.Request.Method != http.MethodHead {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	var (
		resolved *platformservices.ResolvedExtensionAssetRoute
		err      error
	)
	if admin {
		resolved, err = extensionService.ResolveAdminAssetRoute(ctx.Request.Context(), workspaceID, ctx.Request.URL.Path)
	} else {
		resolved, err = extensionService.ResolvePublicAssetRoute(ctx.Request.Context(), ctx.Request.URL.Path)
	}
	if err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if resolved == nil || resolved.Asset == nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	if resolved.Asset.Checksum != "" {
		etag := `"` + resolved.Asset.Checksum + `"`
		ctx.Header("ETag", etag)
		if match := ctx.GetHeader("If-None-Match"); match == etag {
			ctx.Status(http.StatusNotModified)
			return
		}
	}

	if resolved.Asset.Kind == platformdomain.ExtensionAssetKindTemplate || resolved.Asset.ContentType == "text/html" {
		ctx.Header("Cache-Control", "no-store")
	} else {
		ctx.Header("Cache-Control", "public, max-age=300")
	}

	if ctx.Request.Method == http.MethodHead {
		ctx.Data(http.StatusOK, resolved.Asset.ContentType, nil)
		return
	}
	ctx.Data(http.StatusOK, resolved.Asset.ContentType, resolved.Asset.Content)
}
