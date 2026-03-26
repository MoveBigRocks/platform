package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/container"
	internal_metrics "github.com/movebigrocks/platform/internal/infrastructure/metrics"
	internal_middleware "github.com/movebigrocks/platform/internal/infrastructure/middleware"
	"github.com/movebigrocks/platform/internal/infrastructure/routing"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	"github.com/movebigrocks/platform/internal/platform/extensionruntime"

	servicehandlers "github.com/movebigrocks/platform/internal/service/handlers"

	"github.com/movebigrocks/platform/pkg/logger"

	_ "github.com/movebigrocks/platform/docs" // swagger docs
)

// Version info injected via ldflags at build time
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// @title Move Big Rocks API
// @version 1.0
// @description Multi-venture Operations Platform combining error monitoring and customer support
// @termsOfService https://movebigrocks.com/terms

// @contact.name API Support
// @contact.email support@movebigrocks.com

// @license.name Proprietary
// @license.url https://movebigrocks.com/license

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token

func main() {
	log := logger.New().WithField("service", "api")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Production safety checks
	if cfg.Server.Environment == "production" {
		if cfg.Auth.JWTSecret == "" || cfg.Auth.JWTSecret == "change-me-in-production" {
			log.Error("JWT secret must be set in production")
			os.Exit(1)
		}
		// ClamAV is recommended when attachment storage is configured in production
		// to prevent malware from being uploaded via email or forms
		if cfg.Storage.Attachments.Bucket != "" && cfg.Integrations.ClamAVAddr == "" {
			log.Warn("ClamAV not configured - attachments will not be scanned for malware. Set CLAMAV_ADDR to enable scanning.")
		}
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize container with all services
	c, err := container.New(cfg, container.Options{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	})
	if err != nil {
		log.Error("Failed to initialize application", "error", err)
		os.Exit(1)
	}

	// Initialize metrics
	initializeMetrics(context.Background(), c.Store, c.Logger)

	serviceTargets := extensionruntime.NewRegistry(c)
	var runtime *extensionruntime.Runtime
	if c.Platform != nil && c.Platform.Extension != nil {
		runtime = extensionruntime.NewRuntime(
			serviceTargets,
			extensionruntime.WithBackgroundRuntimeDeps(c.EventBus, c.Store.Extensions(), c.Store.Workspaces(), c.Logger),
		)
		c.Platform.Extension.SetActivationRuntime(runtime)
		c.Platform.Extension.SetHealthRuntime(runtime)
		c.Platform.Extension.SetDiagnosticsRuntime(runtime)
		c.Platform.Extension.SetPrivilegedRuntime(runtime)
		if err := runtime.Start(context.Background()); err != nil {
			log.Error("Failed to bootstrap extension runtime", "error", err)
			os.Exit(1)
		}
	}

	// Create handlers
	postmarkHandlers := servicehandlers.NewPostmarkWebhookHandlers(
		c.Platform.Workspace,
		c.Service.Email,
		c.Service.Attachment,
		cfg.Email.PostmarkWebhookSecret,
		c.EventBus,
		c.Logger,
	)

	publicFormHandler := servicehandlers.NewFormPublicHandler(c.Automation.Form)

	// Create routers
	adminRouter := createAdminRouter(cfg, c, cfg.Admin.Emails, serviceTargets)
	apiRouter := createAPIRouter(cfg, c, postmarkHandlers, publicFormHandler, serviceTargets)
	publicRouter := createPublicRouter(cfg, c.Platform.Session, c.Platform.Extension, c.Platform.CLILogin, c.Platform.Sandbox, cfg.Admin.Emails, Version, GitCommit, BuildDate)

	// Workspace router factory
	workspaceRouterFactory := routing.NewDefaultWorkspaceRouterFactory(
		c.Store,
		c.Service.Case,
		c.Platform.Extension,
		c.Platform.Session,
		c.Logger,
		cfg.Server.Environment,
	)

	// Tenant resolver
	tenantResolver := routing.NewWorkspaceTenantResolver(c.Store, c.Logger, workspaceRouterFactory)
	defer tenantResolver.Stop()

	// Subdomain multiplexer
	mux := routing.NewSubdomainMux(c.Logger, tenantResolver)
	mux.Register("admin", adminRouter)
	mux.Register("api", apiRouter)
	mux.SetDefault(publicRouter)

	c.Logger.Info("Subdomain routing enabled")
	c.Logger.Info("  - admin.example.com → Instance administration")
	c.Logger.Info("  - api.example.com → API endpoints")
	c.Logger.Info("  - {workspace}.example.com → Workspace-scoped API (dynamic)")
	c.Logger.Info("  - app.example.com → Public runtime surfaces")

	// Start background services with startup health validation
	ctx := context.Background()
	if err := c.StartWithHealthCheck(ctx); err != nil {
		c.Logger.Error("Failed to start background services", "error", err)
		os.Exit(1)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:              os.Getenv("HOST") + ":" + cfg.Server.Port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Start server
	go func() {
		c.Logger.Info("Starting server", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.Logger.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	c.Logger.Info("Shutting down server...")
	if runtime != nil {
		runtime.Stop()
	}

	// Drain in-flight HTTP requests FIRST, before closing any backend services.
	// This prevents requests from hitting a closed database or event bus.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		c.Logger.Error("Server forced to shutdown", "error", err)
	}
	c.Logger.Info("HTTP server drained")

	// Now stop container services (workers, outbox, event bus, database)
	if err := c.Stop(10 * time.Second); err != nil {
		c.Logger.Error("Error stopping services", "error", err)
	}
	if err := serviceTargets.Close(); err != nil {
		c.Logger.Error("Error stopping extension service targets", "error", err)
	}
	internal_middleware.StopGlobalRateLimiterCleanup()

	c.Logger.Info("Server shutdown complete")
}

// initializeMetrics initializes Prometheus gauges with current system state
func initializeMetrics(ctx context.Context, store stores.Store, log *logger.Logger) {
	if log == nil {
		log = logger.NewNop()
	}
	log.Info("Initializing metrics...")

	// Count and set active workspaces
	workspaces, err := store.Workspaces().ListWorkspaces(ctx)
	if err != nil {
		log.Warn("Failed to count workspaces for metrics", "error", err)
	} else {
		// Count only accessible workspaces
		activeCount := 0
		for _, workspace := range workspaces {
			if workspace.IsAccessible() {
				activeCount++
			}
		}

		// Set active workspaces gauge
		internal_metrics.ActiveWorkspaces.Set(float64(activeCount))
		log.Info("Metrics: Initialized active workspaces", "count", activeCount)
	}

	// NOTE: Worker metrics are now reported directly by each worker process
	// via the worker-up metric (see internal/workers/metrics.go)

	// Initialize connection gauges
	internal_metrics.WebSocketConnections.Set(0)
	internal_metrics.PortalSessions.Set(0)

	log.Info("Metrics initialized")
}
