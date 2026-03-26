package routing

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	platformhandlers "github.com/movebigrocks/platform/internal/platform/handlers"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/pkg/logger"
)

// DefaultWorkspaceRouterFactory creates workspace-specific routers
// Each tenant subdomain gets its own router with workspace context automatically injected
type DefaultWorkspaceRouterFactory struct {
	store                   stores.Store
	caseService             *serviceapp.CaseService
	extensionService        *platformservices.ExtensionService
	sessionService          *platformservices.SessionService
	tenantContextMiddleware *middleware.TenantContextMiddleware
	logger                  *logger.Logger
	environment             string
}

// NewDefaultWorkspaceRouterFactory creates a new workspace router factory
func NewDefaultWorkspaceRouterFactory(
	store stores.Store,
	caseService *serviceapp.CaseService,
	extensionService *platformservices.ExtensionService,
	sessionService *platformservices.SessionService,
	logger *logger.Logger,
	environment string,
) *DefaultWorkspaceRouterFactory {
	// Create tenant context middleware with store (uses transaction-based isolation)
	tenantMiddleware := middleware.NewTenantContextMiddleware(store, slog.Default())

	return &DefaultWorkspaceRouterFactory{
		store:                   store,
		caseService:             caseService,
		extensionService:        extensionService,
		sessionService:          sessionService,
		tenantContextMiddleware: tenantMiddleware,
		logger:                  logger,
		environment:             environment,
	}
}

// CreateWorkspaceRouter creates a Gin router for a workspace subdomain
// All routes automatically have workspace context injected
func (f *DefaultWorkspaceRouterFactory) CreateWorkspaceRouter(workspaceID, workspaceSlug string) *gin.Engine {
	// Create router without default middleware
	router := gin.New()

	// Add middleware (order matters!)
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.MaxBodySize(middleware.DefaultMaxBodySize)) // 1MB limit for workspace API
	router.Use(middleware.ValidatePathParams())                       // Prevent path traversal
	router.Use(middleware.InputValidation())                          // Content-Type validation
	router.Use(middleware.RequestID())
	router.Use(gin.Logger())
	router.Use(middleware.Recovery())

	// CRITICAL: Inject workspace context middleware
	// This makes workspaceID available to all routes via c.Get("workspace_id")
	router.Use(func(c *gin.Context) {
		c.Set("workspace_id", workspaceID)
		c.Set("workspace_slug", workspaceSlug)
		c.Next()
	})

	// Set tenant context for tenant isolation
	// This must come after workspace_id is set in the context
	if f.tenantContextMiddleware != nil {
		router.Use(f.tenantContextMiddleware.SetTenantContext())
	}

	// Create context-aware auth middleware
	contextAuthMiddleware := middleware.NewContextAuthMiddleware(f.sessionService)

	workspaceAPIHandler := platformhandlers.NewWorkspaceAPIHandler(f.caseService)

	// Public routes (no auth required)
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":          "Workspace HTML routes are not exposed on tenant subdomains",
			"workspace_id":   workspaceID,
			"workspace_slug": workspaceSlug,
		})
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "ok",
			"workspace_id":   workspaceID,
			"workspace_slug": workspaceSlug,
		})
	})

	// Workspace API routes (require workspace authentication)
	api := router.Group("/api")
	api.Use(contextAuthMiddleware.AuthRequired())
	api.Use(contextAuthMiddleware.RequireCurrentWorkspaceAccess())
	{
		// Case Management APIs
		api.GET("/cases", workspaceAPIHandler.ListCases)
		api.GET("/cases/:id", workspaceAPIHandler.GetCase)
		api.POST("/cases", workspaceAPIHandler.CreateCase)
		api.PUT("/cases/:id", workspaceAPIHandler.UpdateCase)
		api.DELETE("/cases/:id", workspaceAPIHandler.DeleteCase)

	}

	f.logger.Infow("Created workspace router",
		"workspace_id", workspaceID,
		"workspace_slug", workspaceSlug,
	)

	return router
}
