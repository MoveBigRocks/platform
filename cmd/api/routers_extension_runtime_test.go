package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestCreatePublicRouter_ServesExtensionAssetBackedPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	extensionService := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_router",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "ats",
			Name:          "Applicant Tracking",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindProduct,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:      "careers-home",
					Class:     platformdomain.ExtensionEndpointClassPublicPage,
					MountPath: "/careers",
					AssetPath: "templates/careers/index.html",
				},
			},
		},
		Assets: []platformservices.ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Careers</body></html>"),
			},
		},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	cfg := testutil.NewTestConfig(t)
	router := createPublicRouter(cfg, nil, extensionService, nil, nil, nil, "test", "abc123", "2026-03-13")

	req := httptest.NewRequest(http.MethodGet, "/careers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.NotEmpty(t, w.Header().Get("ETag"))
	assert.Contains(t, w.Body.String(), "Careers")

	headReq := httptest.NewRequest(http.MethodGet, "/careers", nil)
	headReq.Header.Set("If-None-Match", w.Header().Get("ETag"))
	headW := httptest.NewRecorder()
	router.ServeHTTP(headW, headReq)
	assert.Equal(t, http.StatusNotModified, headW.Code)
}

func TestCreatePublicRouter_DoesNotServeStandaloneMarketingPages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testutil.NewTestConfig(t)
	router := createPublicRouter(cfg, nil, nil, nil, nil, nil, "test", "abc123", "2026-03-13")

	for _, path := range []string{"/", "/core", "/docs", "/signup"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, path)
	}
}

func TestCreatePublicRouter_DoesNotServeSiteOwnedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testutil.NewTestConfig(t)
	router := createPublicRouter(cfg, nil, nil, nil, nil, nil, "test", "abc123", "2026-03-13")

	for _, path := range []string{
		"/site.md",
		"/llms.txt",
		"/core.md",
		"/docs.md",
		"/docs/cli.md",
		"/.well-known/mbr-agent.json",
		"/releases/latest.json",
		"/install.sh",
		"/install.ps1",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, path)
	}
}
