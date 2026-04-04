package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extensionruntime "github.com/movebigrocks/platform/internal/extensionhost/runtime"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/container"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
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

func TestServeResolvedExtensionServiceRoute_AppliesResolvedWorkspaceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_workspace_binding",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "careers",
			Name:          "Careers",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "public-apply",
					Class:         platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:     "/careers/applications",
					Methods:       []string{"POST"},
					Auth:          platformdomain.ExtensionEndpointAuthPublic,
					ServiceTarget: "careers.public.apply",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	registry := &extensionruntime.Registry{}
	registry.Register("careers.public.apply", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"workspaceID": c.GetString("workspace_id")})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/careers/applications", nil)

	serveResolvedExtensionServiceRoute(c, extensionService, registry, nil, nil)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"workspaceID":"`+workspace.ID+`"`)
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

func TestServeResolvedAdminExtensionServiceRoute_InstanceAdminQueryWorkspaceSelectsInstall(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceOne := testutil.NewIsolatedWorkspace(t)
	workspaceTwo := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceOne))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceTwo))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	install := func(workspaceID, target string) {
		installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
			WorkspaceID:  workspaceID,
			LicenseToken: "lic_" + workspaceID,
			Manifest: platformdomain.ExtensionManifest{
				SchemaVersion: 1,
				Slug:          "sales-pipeline",
				Name:          "Sales Pipeline",
				Version:       "1.0.0",
				Publisher:     "DemandOps",
				Kind:          platformdomain.ExtensionKindOperational,
				Scope:         platformdomain.ExtensionScopeWorkspace,
				Risk:          platformdomain.ExtensionRiskStandard,
				Endpoints: []platformdomain.ExtensionEndpoint{
					{
						Name:             "dashboard",
						Class:            platformdomain.ExtensionEndpointClassAdminPage,
						MountPath:        "/extensions/sales-pipeline",
						Methods:          []string{"GET"},
						Auth:             platformdomain.ExtensionEndpointAuthSession,
						WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingFromSession,
						ServiceTarget:    "sales.dashboard." + target,
					},
				},
			},
		})
		require.NoError(t, err)
		_, err = extensionService.ActivateExtension(ctx, installed.ID)
		require.NoError(t, err)
	}

	install(workspaceOne.ID, "one")
	install(workspaceTwo.ID, "two")

	registry := &extensionruntime.Registry{}
	registry.Register("sales.dashboard.one", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"target": "one", "workspaceID": c.GetString("workspace_id")})
	})
	registry.Register("sales.dashboard.two", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"target": "two", "workspaceID": c.GetString("workspace_id")})
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/extensions/sales-pipeline?workspace="+workspaceTwo.ID, nil)
	c.Set("auth_context", &platformdomain.AuthContext{
		Principal:     &platformdomain.User{ID: "user_admin", Email: "admin@example.com"},
		PrincipalType: platformdomain.PrincipalTypeUser,
		AuthMethod:    platformdomain.AuthMethodSession,
		InstanceRole:  instanceRolePtr(platformdomain.InstanceRoleAdmin),
	})

	handled := serveResolvedAdminExtensionServiceRoute(c, extensionService, registry, resolvedAdminRouteWorkspaceID(c), nil)

	require.True(t, handled)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"target":"two"`)
	assert.Contains(t, w.Body.String(), `"workspaceID":"`+workspaceTwo.ID+`"`)
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

	runtimeDir := newShortRuntimeDir(t)
	cfg := testutil.NewTestConfig(t)
	cfg.ExtensionRuntimeDir = runtimeDir

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
	cleanup := startUnixSocketTestServer(t, runtimeDir, "demandops/web-analytics", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(r.URL.Path + "|" + r.Header.Get("X-MBR-User-Name")))
	}))
	defer cleanup()

	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/extensions/web-analytics", nil)
	ginCtx.Set("name", "Test User")
	ginCtx.Set("email", "test@example.com")

	extension := &platformdomain.InstalledExtension{
		ID:   "ext_web_analytics",
		Slug: "web-analytics",
		Manifest: platformdomain.ExtensionManifest{
			Publisher: "DemandOps",
			Slug:      "web-analytics",
			Schema: platformdomain.ExtensionSchemaManifest{
				PackageKey: "demandops/web-analytics",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP,
			},
		},
	}
	endpoint := platformdomain.ExtensionEndpoint{
		Name:          "analytics-admin-properties",
		Class:         platformdomain.ExtensionEndpointClassAdminPage,
		MountPath:     "/extensions/web-analytics",
		ServiceTarget: "analytics.admin.properties",
	}

	require.NoError(t, registry.DispatchEndpoint(extension, endpoint, ginCtx))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "/extensions/web-analytics")
	assert.Contains(t, w.Body.String(), "Test User")
}

func TestExtensionServiceTargetRegistry_DispatchesErrorTrackingApplicationsPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testutil.NewTestConfig(t)
	runtimeDir := newShortRuntimeDir(t)
	cfg.ExtensionRuntimeDir = runtimeDir

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
	cleanup := startUnixSocketTestServer(t, runtimeDir, "demandops/error-tracking", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(r.URL.Path + "|" + r.Header.Get("X-MBR-Workspace-ID")))
	}))
	defer cleanup()

	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	workspaceID := "ws_applications"
	workspaceName := "Engineering"
	workspaceSlug := "engineering"
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/extensions/error-tracking/applications", nil)
	ginCtx.Set("name", "Test User")
	ginCtx.Set("email", "test@example.com")
	ginCtx.Set("workspace_id", workspaceID)
	ginCtx.Set("session", &platformdomain.Session{
		CurrentContext: platformdomain.Context{
			Type:          platformdomain.ContextTypeWorkspace,
			WorkspaceID:   &workspaceID,
			WorkspaceName: &workspaceName,
			WorkspaceSlug: &workspaceSlug,
			Role:          "owner",
		},
	})

	extension := &platformdomain.InstalledExtension{
		ID:          "ext_error_tracking",
		Slug:        "error-tracking",
		WorkspaceID: workspaceID,
		Manifest: platformdomain.ExtensionManifest{
			Publisher: "DemandOps",
			Slug:      "error-tracking",
			Schema: platformdomain.ExtensionSchemaManifest{
				PackageKey: "demandops/error-tracking",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP,
			},
		},
	}
	endpoint := platformdomain.ExtensionEndpoint{
		Name:          "error-tracking-admin-applications",
		Class:         platformdomain.ExtensionEndpointClassAdminPage,
		MountPath:     "/extensions/error-tracking/applications",
		ServiceTarget: "error-tracking.admin.applications",
	}

	require.NoError(t, registry.DispatchEndpoint(extension, endpoint, ginCtx))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "/extensions/error-tracking/applications")
	assert.Contains(t, w.Body.String(), workspaceID)
}

func TestExtensionServiceTargetRegistry_DispatchesErrorTrackingIssuesPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testutil.NewTestConfig(t)
	runtimeDir := newShortRuntimeDir(t)
	cfg.ExtensionRuntimeDir = runtimeDir

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
	cleanup := startUnixSocketTestServer(t, runtimeDir, "demandops/error-tracking", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(r.URL.Path + "|" + r.Header.Get("X-MBR-User-Email")))
	}))
	defer cleanup()

	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	workspaceID := "ws_issues"
	workspaceName := "Engineering"
	workspaceSlug := "engineering"
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/extensions/error-tracking/issues", nil)
	ginCtx.Set("name", "Test User")
	ginCtx.Set("email", "test@example.com")
	ginCtx.Set("workspace_id", workspaceID)
	ginCtx.Set("session", &platformdomain.Session{
		CurrentContext: platformdomain.Context{
			Type:          platformdomain.ContextTypeWorkspace,
			WorkspaceID:   &workspaceID,
			WorkspaceName: &workspaceName,
			WorkspaceSlug: &workspaceSlug,
			Role:          "owner",
		},
	})

	extension := &platformdomain.InstalledExtension{
		ID:          "ext_error_tracking",
		Slug:        "error-tracking",
		WorkspaceID: workspaceID,
		Manifest: platformdomain.ExtensionManifest{
			Publisher: "DemandOps",
			Slug:      "error-tracking",
			Schema: platformdomain.ExtensionSchemaManifest{
				PackageKey: "demandops/error-tracking",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP,
			},
		},
	}
	endpoint := platformdomain.ExtensionEndpoint{
		Name:          "error-tracking-admin-issues",
		Class:         platformdomain.ExtensionEndpointClassAdminPage,
		MountPath:     "/extensions/error-tracking/issues",
		ServiceTarget: "error-tracking.admin.issues",
	}

	require.NoError(t, registry.DispatchEndpoint(extension, endpoint, ginCtx))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "/extensions/error-tracking/issues")
	assert.Contains(t, w.Body.String(), "test@example.com")
}
