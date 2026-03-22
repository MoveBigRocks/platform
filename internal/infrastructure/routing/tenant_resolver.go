package routing

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	"github.com/movebigrocks/platform/pkg/logger"
)

// WorkspaceTenantResolver dynamically resolves tenant subdomains to workspace routers
// Caches workspace lookups to avoid hitting store on every request
type WorkspaceTenantResolver struct {
	store           stores.Store
	logger          *logger.Logger
	routerFactory   WorkspaceRouterFactory
	cache           map[string]*cachedWorkspaceRouter
	cacheMu         sync.RWMutex
	cacheTTL        time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// WorkspaceRouterFactory creates routers for workspace subdomains
// This allows injecting workspace-specific context into the router
type WorkspaceRouterFactory interface {
	// CreateWorkspaceRouter creates a Gin router for the given workspace
	CreateWorkspaceRouter(workspaceID, workspaceSlug string) *gin.Engine
}

// cachedWorkspaceRouter holds a cached workspace router with expiry
type cachedWorkspaceRouter struct {
	router      *gin.Engine
	workspaceID string
	expiresAt   time.Time
}

// NewWorkspaceTenantResolver creates a new tenant resolver with caching
func NewWorkspaceTenantResolver(
	store stores.Store,
	logger *logger.Logger,
	routerFactory WorkspaceRouterFactory,
) *WorkspaceTenantResolver {
	resolver := &WorkspaceTenantResolver{
		store:           store,
		logger:          logger,
		routerFactory:   routerFactory,
		cache:           make(map[string]*cachedWorkspaceRouter),
		cacheTTL:        5 * time.Minute, // Cache workspace lookups for 5 minutes
		cleanupInterval: 10 * time.Minute,
		stopCleanup:     make(chan struct{}),
	}

	// Start cache cleanup goroutine
	go resolver.cleanupExpiredCache()

	return resolver
}

// ResolveTenant implements TenantResolver interface
// Returns router for tenant subdomain or nil if not found/reserved
func (r *WorkspaceTenantResolver) ResolveTenant(subdomain string) *gin.Engine {
	// Check if subdomain is reserved
	if IsReservedSubdomain(subdomain) {
		return nil
	}

	// Check cache first
	r.cacheMu.RLock()
	cached, exists := r.cache[subdomain]
	r.cacheMu.RUnlock()

	if exists && time.Now().Before(cached.expiresAt) {
		r.logger.Debugw("Workspace router cache hit",
			"subdomain", subdomain,
			"workspace_id", cached.workspaceID,
		)
		return cached.router
	}

	// Cache miss - lookup workspace
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	workspace, err := r.store.Workspaces().GetWorkspaceBySlug(ctx, subdomain)
	if err != nil {
		r.logger.Debugw("Workspace not found for subdomain",
			"subdomain", subdomain,
			"error", err,
		)
		return nil
	}

	// Check if workspace is accessible
	if !workspace.IsAccessible() {
		r.logger.Warnw("Workspace not accessible",
			"subdomain", subdomain,
			"workspace_id", workspace.ID,
			"is_active", workspace.IsActive,
			"is_suspended", workspace.IsSuspended,
		)
		return r.createUnavailableRouter(workspace.Name)
	}

	// Create workspace router
	router := r.routerFactory.CreateWorkspaceRouter(workspace.ID, workspace.Slug)

	// Cache the router
	r.cacheMu.Lock()
	r.cache[subdomain] = &cachedWorkspaceRouter{
		router:      router,
		workspaceID: workspace.ID,
		expiresAt:   time.Now().Add(r.cacheTTL),
	}
	r.cacheMu.Unlock()

	r.logger.Infow("Created workspace router",
		"subdomain", subdomain,
		"workspace_id", workspace.ID,
		"workspace_name", workspace.Name,
	)

	return router
}

// InvalidateCache removes a tenant from the cache
// Call this when workspace slug changes or workspace is deleted
func (r *WorkspaceTenantResolver) InvalidateCache(subdomain string) {
	r.cacheMu.Lock()
	delete(r.cache, subdomain)
	r.cacheMu.Unlock()

	r.logger.Infow("Invalidated workspace router cache",
		"subdomain", subdomain,
	)
}

// cleanupExpiredCache periodically removes expired cache entries
func (r *WorkspaceTenantResolver) cleanupExpiredCache() {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cacheMu.Lock()
			now := time.Now()
			removed := 0
			for subdomain, cached := range r.cache {
				if now.After(cached.expiresAt) {
					delete(r.cache, subdomain)
					removed++
				}
			}
			r.cacheMu.Unlock()

			if removed > 0 {
				r.logger.Debugw("Cleaned up expired workspace router cache",
					"removed_count", removed,
				)
			}

		case <-r.stopCleanup:
			return
		}
	}
}

// Stop stops the cache cleanup goroutine
func (r *WorkspaceTenantResolver) Stop() {
	close(r.stopCleanup)
}

// createUnavailableRouter creates a router that shows "workspace unavailable" message
func (r *WorkspaceTenantResolver) createUnavailableRouter(workspaceName string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":     "Workspace Unavailable",
			"message":   "This workspace is currently unavailable. Please contact support if this issue persists.",
			"workspace": workspaceName,
		})
	})

	return router
}
