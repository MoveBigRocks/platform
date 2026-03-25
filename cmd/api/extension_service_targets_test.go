package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/container"
	sqlstore "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/platform/extensionruntime"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/refext"
)

func TestExtensionServiceTargetRegistry_Dispatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := &extensionruntime.Registry{}
	registry.Register("test.echo", func(c *gin.Context) {
		c.JSON(http.StatusAccepted, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/hook", nil)

	ok := registry.Dispatch("test.echo", c)
	require.True(t, ok)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":true`)
}

func TestExtensionServiceTargetRegistry_Probe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := &extensionruntime.Registry{}
	registry.Register("test.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "message": "ready"})
	})

	result, err := registry.Probe("test.health", http.MethodGet, "/extensions/test/health", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Contains(t, string(result.Body), `"status":"healthy"`)
}

func TestExtensionServiceHealthRuntime_CheckInstalledExtensionHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := &extensionruntime.Registry{}
	registry.Register("analytics.runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "message": "analytics ready"})
	})

	runtime := extensionruntime.NewRuntime(registry)
	extension := &platformdomain.InstalledExtension{
		Manifest: platformdomain.ExtensionManifest{
			RuntimeClass: platformdomain.ExtensionRuntimeClassServiceBacked,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/extensions/web-analytics/health",
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
	}

	status, message, err := runtime.CheckInstalledExtensionHealth(context.Background(), extension)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionHealthHealthy, status)
	assert.Contains(t, message, "analytics ready")
}

func TestServeResolvedExtensionServiceRoute_DispatchesInstalledServiceEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_dispatch",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "analytics-ingest",
					Class:         platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:     "/api/ext/test/event",
					Methods:       []string{"POST"},
					Auth:          platformdomain.ExtensionEndpointAuthPublic,
					ServiceTarget: "test.analytics.ingest",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("test.analytics.ingest", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"handled": true})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ext/test/event", nil)

	serveResolvedExtensionServiceRoute(c, extensionService, registry, nil, nil)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"handled":true`)
}

func TestServeResolvedExtensionServiceRoute_ReturnsServiceUnavailableForUnknownTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_missing_dispatch",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "analytics-ingest",
					Class:         platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:     "/api/ext/test/event",
					Methods:       []string{"POST"},
					Auth:          platformdomain.ExtensionEndpointAuthPublic,
					ServiceTarget: "test.analytics.ingest",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ext/test/event", nil)

	serveResolvedExtensionServiceRoute(c, extensionService, &extensionruntime.Registry{}, nil, nil)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestServeResolvedExtensionServiceRoute_PropagatesRouteParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_route_params",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "error-tracking",
			Name:          "Error Tracking",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "sentry-envelope-project",
					Class:         platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:     "/api/:projectNumber/envelope",
					Methods:       []string{"POST"},
					Auth:          platformdomain.ExtensionEndpointAuthPublic,
					ServiceTarget: "error-tracking.ingest.envelope.project",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("error-tracking.ingest.envelope.project", func(c *gin.Context) {
		c.JSON(http.StatusAccepted, gin.H{"projectNumber": c.Param("projectNumber")})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/777/envelope", nil)

	serveResolvedExtensionServiceRoute(c, extensionService, registry, nil, nil)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), `"projectNumber":"777"`)
}

func TestServeResolvedAdminExtensionServiceRoute_DispatchesInstanceScopedServiceEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		LicenseToken: "lic_enterprise_access",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "enterprise-access",
			Name:          "Enterprise Access",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindIdentity,
			Scope:         platformdomain.ExtensionScopeInstance,
			Risk:          platformdomain.ExtensionRiskPrivileged,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_enterprise_access",
				PackageKey:      "demandops/enterprise-access",
				TargetVersion:   "1.0.0",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "settings",
					Class:            platformdomain.ExtensionEndpointClassAdminPage,
					MountPath:        "/extensions/enterprise-access",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthSession,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "enterprise-access.admin.settings",
				},
				{
					Name:             "health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/extensions/enterprise-access/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "enterprise-access.runtime.health",
				},
			},
		},
		BundleBase64: base64.StdEncoding.EncodeToString([]byte(`{"manifest":{}}`)),
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("enterprise-access.admin.settings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "scope": "instance"})
	})
	registry.Register("enterprise-access.runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/extensions/enterprise-access", nil)
	c.Set("auth_context", &platformdomain.AuthContext{
		Principal:     &platformdomain.User{ID: "user_admin", Email: "admin@example.com"},
		PrincipalType: platformdomain.PrincipalTypeUser,
		AuthMethod:    platformdomain.AuthMethodSession,
		InstanceRole:  instanceRolePtr(platformdomain.InstanceRoleAdmin),
	})

	ok := serveResolvedAdminExtensionServiceRoute(c, extensionService, registry, "", nil)
	require.True(t, ok)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"scope":"instance"`)
}

func TestServeResolvedAdminExtensionServiceRoute_DispatchesWorkspaceScopedEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_admin_service_dispatch",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "ops-pack",
			Name:          "Ops Pack",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "refresh-dashboard",
					Class:            platformdomain.ExtensionEndpointClassAdminAction,
					MountPath:        "/extensions/ops/actions/:actionName",
					Methods:          []string{"POST"},
					Auth:             platformdomain.ExtensionEndpointAuthSession,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingFromSession,
					ServiceTarget:    "ops.dashboard.action",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("ops.dashboard.action", func(c *gin.Context) {
		c.JSON(http.StatusAccepted, gin.H{"action": c.Param("actionName")})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/extensions/ops/actions/refresh", nil)
	c.Set("auth_context", &platformdomain.AuthContext{
		Principal:     &platformdomain.User{ID: "user_operator", Email: "ops@example.com"},
		PrincipalType: platformdomain.PrincipalTypeUser,
		AuthMethod:    platformdomain.AuthMethodSession,
		WorkspaceID:   workspace.ID,
	})
	c.Set("workspace_id", workspace.ID)

	handled := serveResolvedAdminExtensionServiceRoute(c, extensionService, registry, workspace.ID, nil)

	require.True(t, handled)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), `"action":"refresh"`)
}

func TestServeResolvedAdminExtensionServiceRoute_BlocksInstanceScopedEndpointFromWorkspaceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		LicenseToken: "lic_enterprise_access_block",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "enterprise-access",
			Name:          "Enterprise Access",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindIdentity,
			Scope:         platformdomain.ExtensionScopeInstance,
			Risk:          platformdomain.ExtensionRiskPrivileged,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_enterprise_access",
				PackageKey:      "demandops/enterprise-access",
				TargetVersion:   "1.0.0",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "settings",
					Class:            platformdomain.ExtensionEndpointClassAdminPage,
					MountPath:        "/extensions/enterprise-access",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthSession,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "enterprise-access.admin.settings",
				},
				{
					Name:             "health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/extensions/enterprise-access/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "enterprise-access.runtime.health",
				},
			},
		},
		BundleBase64: base64.StdEncoding.EncodeToString([]byte(`{"manifest":{}}`)),
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("enterprise-access.admin.settings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/extensions/enterprise-access", nil)
	c.Set("auth_context", &platformdomain.AuthContext{
		Principal:     &platformdomain.User{ID: "user_operator", Email: "ops@example.com"},
		PrincipalType: platformdomain.PrincipalTypeUser,
		AuthMethod:    platformdomain.AuthMethodSession,
		WorkspaceID:   workspace.ID,
	})
	c.Set("workspace_id", workspace.ID)

	handled := serveResolvedAdminExtensionServiceRoute(c, extensionService, registry, workspace.ID, nil)

	require.True(t, handled)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestServeResolvedAdminExtensionServiceRoute_BlocksInternalOnlyEndpointOverHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		LicenseToken: "lic_internal_only",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "enterprise-access",
			Name:          "Enterprise Access",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindIdentity,
			Scope:         platformdomain.ExtensionScopeInstance,
			Risk:          platformdomain.ExtensionRiskPrivileged,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_enterprise_access",
				PackageKey:      "demandops/enterprise-access",
				TargetVersion:   "1.0.0",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/extensions/enterprise-access/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "enterprise-access.runtime.health",
				},
			},
		},
		BundleBase64: base64.StdEncoding.EncodeToString([]byte(`{"manifest":{}}`)),
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("enterprise-access.runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/extensions/enterprise-access/health", nil)
	c.Set("auth_context", &platformdomain.AuthContext{
		Principal:     &platformdomain.User{ID: "user_admin", Email: "admin@example.com"},
		PrincipalType: platformdomain.PrincipalTypeUser,
		AuthMethod:    platformdomain.AuthMethodSession,
		InstanceRole:  instanceRolePtr(platformdomain.InstanceRoleAdmin),
	})

	handled := serveResolvedAdminExtensionServiceRoute(c, extensionService, registry, "", nil)

	require.True(t, handled)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestServeResolvedExtensionServiceRoute_EnforcesMaxBodySize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_body_limit",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "analytics-ingest",
					Class:            platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:        "/api/ext/body-limit",
					Methods:          []string{"POST"},
					Auth:             platformdomain.ExtensionEndpointAuthPublic,
					MaxBodyBytes:     4,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingNone,
					ServiceTarget:    "test.analytics.ingest",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("test.analytics.ingest", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"handled": true})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ext/body-limit", bytes.NewBufferString("12345"))

	serveResolvedExtensionServiceRoute(c, extensionService, registry, nil, nil)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.NotContains(t, w.Body.String(), `"handled":true`)
}

func TestServeResolvedExtensionServiceRoute_EnforcesRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_rate_limit",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "analytics-ingest-rate-limited",
					Class:            platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:        "/api/ext/rate-limit",
					Methods:          []string{"POST"},
					Auth:             platformdomain.ExtensionEndpointAuthPublic,
					RateLimitPerMin:  1,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingNone,
					ServiceTarget:    "test.analytics.ingest",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("test.analytics.ingest", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"handled": true})
	})

	firstWriter := httptest.NewRecorder()
	firstCtx, _ := gin.CreateTestContext(firstWriter)
	firstCtx.Request = httptest.NewRequest(http.MethodPost, "/api/ext/rate-limit", bytes.NewBufferString("{}"))
	serveResolvedExtensionServiceRoute(firstCtx, extensionService, registry, nil, nil)
	require.Equal(t, http.StatusCreated, firstWriter.Code)

	secondWriter := httptest.NewRecorder()
	secondCtx, _ := gin.CreateTestContext(secondWriter)
	secondCtx.Request = httptest.NewRequest(http.MethodPost, "/api/ext/rate-limit", bytes.NewBufferString("{}"))
	serveResolvedExtensionServiceRoute(secondCtx, extensionService, registry, nil, nil)

	assert.Equal(t, http.StatusTooManyRequests, secondWriter.Code)
}

func TestServeResolvedExtensionServiceRoute_ValidatesSignedWebhookAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_signed_webhook",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "ops-pack",
			Name:          "Ops Pack",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_ops_pack",
				PackageKey:      "demandops/ops-pack",
				TargetVersion:   "1.0.0",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "signed-webhook",
					Class:            platformdomain.ExtensionEndpointClassWebhook,
					MountPath:        "/api/ext/signed-webhook",
					Methods:          []string{"POST"},
					Auth:             platformdomain.ExtensionEndpointAuthSignedWebhook,
					MaxBodyBytes:     1024,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingNone,
					ServiceTarget:    "ops.signed.webhook",
				},
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/extensions/ops-pack/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "ops.runtime.health",
				},
			},
		},
		Migrations: []platformservices.ExtensionMigrationInput{{Path: "000001_init.sql", Content: []byte("select 1;")}},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("ops.signed.webhook", func(c *gin.Context) {
		body, readErr := io.ReadAll(c.Request.Body)
		require.NoError(t, readErr)
		c.JSON(http.StatusAccepted, gin.H{"body": string(body)})
	})
	registry.Register("ops.runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	cfg := &config.Config{}
	cfg.Auth.JWTSecret = "test-signed-webhook-secret"

	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	body := []byte(`{"ok":true}`)
	signature := computeSignedWebhookSignature(cfg.Auth.JWTSecret, timestamp, body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ext/signed-webhook", bytes.NewReader(body))
	c.Request.Header.Set(extensionWebhookTimestampHeader, timestamp)
	c.Request.Header.Set(extensionWebhookSignatureHeader, signature)

	serveResolvedExtensionServiceRoute(c, extensionService, registry, cfg, nil)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), `"body":"{\"ok\":true}"`)
}

func instanceRolePtr(role platformdomain.InstanceRole) *platformdomain.InstanceRole {
	return &role
}

func TestExtensionServiceTargetRegistry_DispatchesAnalyticsAdminPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testutil.SetupTestEnv(t)

	cfg := testutil.NewTestConfig(t)

	cntr, err := container.New(cfg, container.Options{
		Version:   "test",
		GitCommit: "test-commit",
		BuildDate: "2026-03-13",
	})
	require.NoError(t, err)
	defer func() {
		_ = cntr.Stop(2 * time.Second)
	}()

	registry := extensionruntime.NewRegistry(cntr)

	w := httptest.NewRecorder()
	ginCtx, router := gin.CreateTestContext(w)
	router.SetHTMLTemplate(template.Must(template.New("analytics_properties.html").Parse(`{{define "analytics_properties.html"}}{{.AnalyticsBasePath}}|analytics-admin{{end}}`)))
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/extensions/web-analytics", nil)
	ginCtx.Set("name", "Test User")
	ginCtx.Set("email", "test@example.com")

	ok := registry.Dispatch("analytics.admin.properties", ginCtx)

	require.True(t, ok)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "/extensions/web-analytics")
	assert.Contains(t, w.Body.String(), "analytics-admin")
}

func TestExtensionServiceTargetRegistry_DispatchesErrorTrackingApplicationsPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testDSN, cleanupDB := testutil.SetupTestPostgresDatabase(t)
	defer cleanupDB()

	tmpDir := t.TempDir()
	t.Setenv("DATABASE_DSN", testDSN)
	t.Setenv("STORAGE_PATH", tmpDir)
	t.Setenv("FILESYSTEM_PATH", tmpDir)
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-chars-long-for-testing")
	t.Setenv("ENVIRONMENT", "test")
	t.Setenv("EMAIL_BACKEND", "mock")
	t.Setenv("STORAGE_TYPE", "filesystem")
	t.Setenv("TRACING_ENABLED", "false")
	t.Setenv("ENABLE_METRICS", "false")
	t.Setenv("CLAMAV_ADDR", "")

	cfg := testutil.NewTestConfig(t)
	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{DSN: cfg.Database.EffectiveDSN()})
	require.NoError(t, err)
	store, err := sqlstore.NewStore(db)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	backgroundCtx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(backgroundCtx, workspace))
	refext.InstallAndActivateReferenceExtension(t, backgroundCtx, store, workspace.ID, "error-tracking")
	project := observabilitydomain.NewProject(workspace.ID, "", "Backend API", "backend-api", "go")
	require.NoError(t, store.Projects().CreateProject(backgroundCtx, project))

	cntr, err := container.New(cfg, container.Options{
		Version:   "test",
		GitCommit: "test-commit",
		BuildDate: "2026-03-14",
	})
	require.NoError(t, err)
	defer func() {
		_ = cntr.Stop(2 * time.Second)
	}()

	registry := extensionruntime.NewRegistry(cntr)

	w := httptest.NewRecorder()
	ginCtx, router := gin.CreateTestContext(w)
	router.SetHTMLTemplate(template.Must(template.New("applications.html").Parse(`{{define "applications.html"}}{{.ApplicationsBasePath}}|{{len .Applications}}{{end}}`)))
	workspaceID := workspace.ID
	workspaceName := workspace.Name
	workspaceSlug := workspace.ShortCode
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/extensions/error-tracking/applications", nil)
	ginCtx.Set("name", "Test User")
	ginCtx.Set("email", "test@example.com")
	ginCtx.Set("session", &platformdomain.Session{
		CurrentContext: platformdomain.Context{
			Type:          platformdomain.ContextTypeWorkspace,
			WorkspaceID:   &workspaceID,
			WorkspaceName: &workspaceName,
			WorkspaceSlug: &workspaceSlug,
			Role:          "owner",
		},
	})

	ok := registry.Dispatch("error-tracking.admin.applications", ginCtx)

	require.True(t, ok)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "/extensions/error-tracking/applications")
	assert.Contains(t, w.Body.String(), "|1")
}

func TestExtensionServiceTargetRegistry_DispatchesErrorTrackingIssuesPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testDSN, cleanupDB := testutil.SetupTestPostgresDatabase(t)
	defer cleanupDB()

	tmpDir := t.TempDir()
	t.Setenv("DATABASE_DSN", testDSN)
	t.Setenv("STORAGE_PATH", tmpDir)
	t.Setenv("FILESYSTEM_PATH", tmpDir)
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-chars-long-for-testing")
	t.Setenv("ENVIRONMENT", "test")
	t.Setenv("EMAIL_BACKEND", "mock")
	t.Setenv("STORAGE_TYPE", "filesystem")
	t.Setenv("TRACING_ENABLED", "false")
	t.Setenv("ENABLE_METRICS", "false")
	t.Setenv("CLAMAV_ADDR", "")

	cfg := testutil.NewTestConfig(t)
	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{DSN: cfg.Database.EffectiveDSN()})
	require.NoError(t, err)
	store, err := sqlstore.NewStore(db)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	backgroundCtx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(backgroundCtx, workspace))
	refext.InstallAndActivateReferenceExtension(t, backgroundCtx, store, workspace.ID, "error-tracking")

	project := observabilitydomain.NewProject(workspace.ID, "", "Backend API", "backend-api", "go")
	require.NoError(t, store.Projects().CreateProject(backgroundCtx, project))
	issue := &observabilitydomain.Issue{
		ID:          "issue_extension_runtime",
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		ShortID:     "ERR-123",
		Title:       "NullPointerException",
		Culprit:     "service.handler",
		Status:      observabilitydomain.IssueStatusUnresolved,
		Level:       "error",
		Fingerprint: "ext-runtime-issue",
		FirstSeen:   time.Now().Add(-1 * time.Hour),
		LastSeen:    time.Now(),
	}
	require.NoError(t, store.Issues().CreateIssue(backgroundCtx, issue))

	cntr, err := container.New(cfg, container.Options{
		Version:   "test",
		GitCommit: "test-commit",
		BuildDate: "2026-03-14",
	})
	require.NoError(t, err)
	defer func() {
		_ = cntr.Stop(2 * time.Second)
	}()

	registry := extensionruntime.NewRegistry(cntr)

	w := httptest.NewRecorder()
	ginCtx, router := gin.CreateTestContext(w)
	router.SetHTMLTemplate(template.Must(template.New("issues.html").Parse(`{{define "issues.html"}}{{.IssuesBasePath}}|{{len .Issues}}{{end}}`)))
	workspaceID := workspace.ID
	workspaceName := workspace.Name
	workspaceSlug := workspace.ShortCode
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/extensions/error-tracking/issues", nil)
	ginCtx.Set("name", "Test User")
	ginCtx.Set("email", "test@example.com")
	ginCtx.Set("session", &platformdomain.Session{
		CurrentContext: platformdomain.Context{
			Type:          platformdomain.ContextTypeWorkspace,
			WorkspaceID:   &workspaceID,
			WorkspaceName: &workspaceName,
			WorkspaceSlug: &workspaceSlug,
			Role:          "owner",
		},
	})

	ok := registry.Dispatch("error-tracking.admin.issues", ginCtx)

	require.True(t, ok)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "/extensions/error-tracking/issues")
	assert.Contains(t, w.Body.String(), "|1")
}
