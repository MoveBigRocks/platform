package routing

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/pkg/logger"
)

// SubdomainMux routes requests to different Gin engines based on subdomain
// Enables multi-tenant subdomain routing: team1.movebigrocks.com, team2.movebigrocks.com, etc.
type SubdomainMux struct {
	routers        map[string]*gin.Engine // Static subdomain → router mapping
	tenantResolver TenantResolver         // Dynamic tenant subdomain handler
	defaultRouter  *gin.Engine            // Fallback router (apex domain)
	logger         *logger.Logger
}

// TenantResolver dynamically resolves tenant subdomains to routers
// This allows {tenant-slug}.movebigrocks.com to route to tenant-specific workspace
type TenantResolver interface {
	// ResolveTenant returns a router for the given tenant slug
	// Returns nil if tenant doesn't exist or subdomain is reserved
	ResolveTenant(subdomain string) *gin.Engine
}

// NewSubdomainMux creates a new subdomain multiplexer
func NewSubdomainMux(log *logger.Logger, tenantResolver TenantResolver) *SubdomainMux {
	return &SubdomainMux{
		routers:        make(map[string]*gin.Engine),
		tenantResolver: tenantResolver,
		logger:         log,
	}
}

// Register associates a static subdomain with a Gin engine
// Example: mux.Register("admin", adminRouter) → admin.movebigrocks.com
// Reserved subdomains: admin, api, www, app, cdn, mail, smtp
func (m *SubdomainMux) Register(subdomain string, router *gin.Engine) {
	m.routers[subdomain] = router
}

// SetDefault sets the fallback router for apex domain (movebigrocks.com)
func (m *SubdomainMux) SetDefault(router *gin.Engine) {
	m.defaultRouter = router
}

// ServeHTTP implements http.Handler to route requests based on subdomain
func (m *SubdomainMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	subdomain := extractSubdomain(r.Host)

	// Get request ID for correlation (if present)
	requestID := r.Header.Get("X-Request-ID")
	log := m.logger
	if requestID != "" {
		log = log.WithRequestID(requestID)
	}

	// Log routing decision with full context
	log.Debugw("SubdomainMux routing request",
		"host", r.Host,
		"subdomain", subdomain,
		"path", r.URL.Path,
		"method", r.Method,
	)

	// 1. Check static subdomain routes (admin, api, etc.)
	if router, ok := m.routers[subdomain]; ok {
		log.Debugw("Routing to static subdomain router",
			"router", subdomain,
		)
		router.ServeHTTP(w, r)
		return
	}

	// 2. Try dynamic tenant resolution (tenant-slug.movebigrocks.com)
	if subdomain != "" && m.tenantResolver != nil {
		if tenantRouter := m.tenantResolver.ResolveTenant(subdomain); tenantRouter != nil {
			log.Debugw("Routing to tenant workspace",
				"tenant_subdomain", subdomain,
			)
			tenantRouter.ServeHTTP(w, r)
			return
		}
	}

	// 3. Fallback to default router (apex domain)
	if m.defaultRouter != nil {
		log.Debugw("Routing to default router",
			"subdomain_extracted", subdomain,
		)
		m.defaultRouter.ServeHTTP(w, r)
		return
	}

	// No router found - return 404
	log.Warnw("No router found for request",
		"host", r.Host,
		"subdomain", subdomain,
		"registered_routers", m.getRouterNames(),
		"has_default", false,
	)
	http.NotFound(w, r)
}

// getRouterNames returns list of registered router names for debugging
func (m *SubdomainMux) getRouterNames() []string {
	names := make([]string, 0, len(m.routers))
	for name := range m.routers {
		names = append(names, name)
	}
	return names
}

// extractSubdomain extracts the subdomain from a host string
// Examples:
//   - "admin.movebigrocks.com" → "admin"
//   - "team1.movebigrocks.com" → "team1"
//   - "api.staging.movebigrocks.com" → "api"
//   - "localhost:8080" → ""
//   - "movebigrocks.com" → ""
func extractSubdomain(host string) string {
	// Remove port if present
	if idx := strings.IndexByte(host, ':'); idx != -1 {
		host = host[:idx]
	}

	// Handle localhost (no subdomain)
	if host == "localhost" || host == "127.0.0.1" {
		return ""
	}

	// Extract subdomain (first part before first dot)
	if idx := strings.IndexByte(host, '.'); idx != -1 {
		return host[:idx]
	}

	// No subdomain (apex domain or single-label hostname)
	return ""
}

// IsReservedSubdomain checks if a subdomain is reserved for system use
// Prevents tenants from claiming admin, api, www, etc.
func IsReservedSubdomain(subdomain string) bool {
	reserved := map[string]bool{
		"admin":      true,
		"api":        true,
		"www":        true,
		"app":        true,
		"cdn":        true,
		"mail":       true,
		"smtp":       true,
		"staging":    true,
		"production": true,
		"dev":        true,
		"test":       true,
		"demo":       true,
		"beta":       true,
		"alpha":      true,
		"status":     true,
		"help":       true,
		"support":    true,
		"blog":       true,
		"docs":       true,
		"ftp":        true,
		"webmail":    true,
		"assets":     true,
		"static":     true,
		"media":      true,
		"images":     true,
		"files":      true,
	}
	return reserved[strings.ToLower(subdomain)]
}
