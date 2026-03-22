package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/container"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestCreateAPIRouter_ServesErrorTrackingEnvelopeFromInstalledExtension(t *testing.T) {
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
	installed, err := extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_error_tracking",
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
					Name:          "sentry-envelope",
					Class:         platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:     "/api/envelope",
					Methods:       []string{"POST"},
					Auth:          platformdomain.ExtensionEndpointAuthPublic,
					ServiceTarget: "error-tracking.ingest.envelope",
				},
				{
					Name:          "sentry-envelope-project",
					Class:         platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:     "/api/:projectNumber/envelope",
					Methods:       []string{"POST"},
					Auth:          platformdomain.ExtensionEndpointAuthPublic,
					ServiceTarget: "error-tracking.ingest.envelope.project",
				},
				{
					Name:          "sentry-envelope-v1",
					Class:         platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:     "/1/envelope",
					Methods:       []string{"POST"},
					Auth:          platformdomain.ExtensionEndpointAuthPublic,
					ServiceTarget: "error-tracking.ingest.envelope",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	router := createAPIRouter(cfg, c, nil, nil, nil)

	envelope := strings.NewReader("{\"event_id\":\"00000000000000000000000000000001\"}\n{\"type\":\"event\"}\n{\"event_id\":\"00000000000000000000000000000001\",\"message\":\"boom\",\"platform\":\"javascript\"}\n")
	req := httptest.NewRequest(http.MethodPost, "/api/777/envelope", envelope)
	req.Header.Set("Content-Type", "application/x-sentry-envelope")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid sentry auth header")
}
