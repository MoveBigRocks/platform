package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extensionruntime "github.com/movebigrocks/platform/internal/extensionhost/runtime"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/container"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestCreateAPIRouter_ServesAnalyticsScriptFromInstalledExtension(t *testing.T) {
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
	c, err := container.New(cfg, container.Options{
		Version:   "test",
		GitCommit: "test-commit",
		BuildDate: "2026-03-13",
	})
	require.NoError(t, err)
	defer func() {
		_ = c.Stop(0)
	}()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, c.Store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(c.Store.Extensions(), c.Store.Workspaces(), c.Store.Queues(), c.Store.Forms(), c.Store.Rules(), c.Store)
	runtimeDir := newShortRuntimeDir(t)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_web_analytics",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindProduct,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_web_analytics",
				PackageKey:      "demandops/web-analytics",
				TargetVersion:   "000001",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP,
				OCIReference: "ghcr.io/test/web-analytics-runtime:test",
				Digest:       "sha256:test",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "analytics-script",
					Class:            platformdomain.ExtensionEndpointClassPublicAsset,
					MountPath:        "/js/analytics.js",
					Methods:          []string{"GET", "HEAD"},
					Auth:             platformdomain.ExtensionEndpointAuthPublic,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingNone,
					ServiceTarget:    "analytics.asset.script",
				},
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/extensions/web-analytics/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
		Migrations: serviceBackedTestMigrations(),
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	engine := gin.New()
	engine.GET("/js/analytics.js", func(ctx *gin.Context) {
		ctx.Header("Content-Type", "application/javascript; charset=utf-8")
		ctx.String(http.StatusOK, "window.__mbrAnalyticsEndpoint = '/api/analytics/event';")
	})
	engine.GET("/extensions/web-analytics/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	stop := startUnixSocketTestServer(t, runtimeDir, installed.Manifest.PackageKey(), engine)
	defer stop()

	router := createAPIRouter(cfg, c, nil, nil, nil, extensionruntime.NewRegistryForRuntimeDir(runtimeDir))

	req := httptest.NewRequest(http.MethodGet, "/js/analytics.js", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/javascript")
	assert.Contains(t, w.Body.String(), "/api/analytics/event")
}
