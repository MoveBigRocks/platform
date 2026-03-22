package platformservices

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestExtensionService_ResolvePublicAssetRoute(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_public_runtime",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "ats",
			Name:          "Applicant Tracking",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindProduct,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			PublicRoutes: []platformdomain.ExtensionRoute{
				{PathPrefix: "/careers", AssetPath: "public/careers"},
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:      "careers-home",
					Class:     platformdomain.ExtensionEndpointClassPublicPage,
					MountPath: "/careers",
					AssetPath: "templates/careers/index.html",
				},
			},
		},
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Careers home</body></html>"),
			},
			{
				Path:        "public/careers/app.css",
				ContentType: "text/css",
				Content:     []byte("body { color: #123456; }"),
			},
		},
	})
	require.NoError(t, err)
	_, err = service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	home, err := service.ResolvePublicAssetRoute(ctx, "/careers")
	require.NoError(t, err)
	require.NotNil(t, home)
	assert.Equal(t, "public_page", home.Source)
	assert.Equal(t, "templates/careers/index.html", home.Asset.Path)

	css, err := service.ResolvePublicAssetRoute(ctx, "/careers/app.css")
	require.NoError(t, err)
	require.NotNil(t, css)
	assert.Equal(t, "public_route", css.Source)
	assert.Equal(t, "public/careers/app.css", css.Asset.Path)
}

func TestExtensionService_ResolveAdminAssetRouteByWorkspace(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceOne := testutil.NewIsolatedWorkspace(t)
	workspaceTwo := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceOne))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceTwo))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installForWorkspace := func(workspaceID, title string) string {
		installed, err := service.InstallExtension(ctx, InstallExtensionParams{
			WorkspaceID:  workspaceID,
			LicenseToken: "lic_" + workspaceID,
			Manifest: platformdomain.ExtensionManifest{
				SchemaVersion: 1,
				Slug:          "ats",
				Name:          "Applicant Tracking",
				Version:       "1.0.0",
				Publisher:     "DemandOps",
				Kind:          platformdomain.ExtensionKindProduct,
				Scope:         platformdomain.ExtensionScopeWorkspace,
				Risk:          platformdomain.ExtensionRiskStandard,
				AdminRoutes: []platformdomain.ExtensionRoute{
					{PathPrefix: "/admin/extensions/ats", AssetPath: "templates/admin/dashboard.html"},
				},
				Endpoints: []platformdomain.ExtensionEndpoint{
					{
						Name:      "ats-admin-dashboard",
						Class:     platformdomain.ExtensionEndpointClassAdminPage,
						MountPath: "/admin/extensions/ats",
						AssetPath: "templates/admin/dashboard.html",
					},
				},
			},
			Assets: []ExtensionAssetInput{
				{
					Path:        "templates/admin/dashboard.html",
					ContentType: "text/html",
					Content:     []byte(title),
				},
			},
		})
		require.NoError(t, err)
		_, err = service.ActivateExtension(ctx, installed.ID)
		require.NoError(t, err)
		return installed.ID
	}

	installForWorkspace(workspaceOne.ID, "<html><body>Workspace One ATS</body></html>")
	installForWorkspace(workspaceTwo.ID, "<html><body>Workspace Two ATS</body></html>")

	first, err := service.ResolveAdminAssetRoute(ctx, workspaceOne.ID, "/admin/extensions/ats")
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Contains(t, string(first.Asset.Content), "Workspace One ATS")

	second, err := service.ResolveAdminAssetRoute(ctx, workspaceTwo.ID, "/admin/extensions/ats")
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Contains(t, string(second.Asset.Content), "Workspace Two ATS")
}

func TestExtensionService_ResolveManagedArtifactRoute(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		WithExtensionArtifactService(artifactservices.NewGitService(t.TempDir())),
	)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_managed_artifact",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "ats",
			Name:          "Applicant Tracking",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindProduct,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			ArtifactSurfaces: []platformdomain.ExtensionArtifactSurface{
				{
					Name:          "website",
					SeedAssetPath: "templates/careers",
				},
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:            "careers-home",
					Class:           platformdomain.ExtensionEndpointClassPublicPage,
					MountPath:       "/careers",
					ArtifactSurface: "website",
					ArtifactPath:    "index.html",
				},
				{
					Name:            "careers-apply",
					Class:           platformdomain.ExtensionEndpointClassPublicPage,
					MountPath:       "/careers/apply",
					ArtifactSurface: "website",
					ArtifactPath:    "apply.html",
				},
			},
		},
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Careers home</body></html>"),
			},
			{
				Path:        "templates/careers/apply.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Apply now</body></html>"),
			},
		},
	})
	require.NoError(t, err)
	_, err = service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	home, err := service.ResolvePublicAssetRoute(ctx, "/careers")
	require.NoError(t, err)
	require.NotNil(t, home)
	assert.Equal(t, "public_page", home.Source)
	assert.Contains(t, string(home.Asset.Content), "Careers home")

	apply, err := service.ResolvePublicAssetRoute(ctx, "/careers/apply")
	require.NoError(t, err)
	require.NotNil(t, apply)
	assert.Contains(t, string(apply.Asset.Content), "Apply now")
}

func TestExtensionService_InstallRejectsConflictingPublicPathsAcrossWorkspaces(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceOne := testutil.NewIsolatedWorkspace(t)
	workspaceTwo := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceOne))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceTwo))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	first, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspaceOne.ID,
		LicenseToken: "lic_one",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "ats-one",
			Name:          "Applicant Tracking",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindProduct,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			PublicRoutes: []platformdomain.ExtensionRoute{
				{PathPrefix: "/careers", AssetPath: "templates/careers/index.html"},
			},
		},
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>One</body></html>"),
			},
		},
	})
	require.NoError(t, err)
	_, err = service.ActivateExtension(ctx, first.ID)
	require.NoError(t, err)

	second, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspaceTwo.ID,
		LicenseToken: "lic_two",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "ats-two",
			Name:          "Applicant Tracking",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindProduct,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			PublicRoutes: []platformdomain.ExtensionRoute{
				{PathPrefix: "/careers", AssetPath: "templates/careers/index.html"},
			},
		},
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Two</body></html>"),
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionValidationInvalid, second.ValidationStatus)
	assert.Contains(t, second.ValidationMessage, "conflicts with active extension")

	_, err = service.ActivateExtension(ctx, second.ID)
	require.Error(t, err)
}

func TestExtensionService_InstallRejectsAdminPathOutsideExtensionNamespace(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_bad_admin_path",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "custom-ops",
			Name:          "Custom Ops",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			AdminRoutes: []platformdomain.ExtensionRoute{
				{PathPrefix: "/analytics/custom", AssetPath: "templates/admin/dashboard.html"},
			},
		},
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/admin/dashboard.html",
				ContentType: "text/html",
				Content:     []byte("<html></html>"),
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionValidationInvalid, installed.ValidationStatus)
	assert.Contains(t, installed.ValidationMessage, "must be mounted under /admin/extensions")
}

func TestExtensionService_ResolvePublicServiceRoute(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_public",
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
					MountPath:     "/api/analytics/event",
					Methods:       []string{"POST"},
					Auth:          platformdomain.ExtensionEndpointAuthPublic,
					ServiceTarget: "analytics.ingest.event",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	resolved, err := service.ResolvePublicServiceRoute(ctx, "POST", "/api/analytics/event")
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, "analytics.ingest.event", resolved.Endpoint.ServiceTarget)
	assert.Equal(t, platformdomain.ExtensionEndpointClassPublicIngest, resolved.Endpoint.Class)

	notAllowed, err := service.ResolvePublicServiceRoute(ctx, "GET", "/api/analytics/event")
	require.NoError(t, err)
	assert.Nil(t, notAllowed)
}

func TestExtensionService_ResolvePublicServiceRouteForServiceBackedAsset(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_asset",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindProduct,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "analytics-script",
					Class:         platformdomain.ExtensionEndpointClassPublicAsset,
					MountPath:     "/js/analytics.js",
					Methods:       []string{"GET", "HEAD"},
					Auth:          platformdomain.ExtensionEndpointAuthPublic,
					ServiceTarget: "analytics.asset.script",
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	resolved, err := service.ResolvePublicServiceRoute(ctx, "GET", "/js/analytics.js")
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, "analytics.asset.script", resolved.Endpoint.ServiceTarget)
	assert.Equal(t, platformdomain.ExtensionEndpointClassPublicAsset, resolved.Endpoint.Class)
}

func TestExtensionService_ResolvePublicServiceRouteWithPathParams(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_error_tracking",
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
	_, err = service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	resolved, err := service.ResolvePublicServiceRoute(ctx, "POST", "/api/123456/envelope")
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, "error-tracking.ingest.envelope.project", resolved.Endpoint.ServiceTarget)
	assert.Equal(t, "123456", resolved.RouteParams["projectNumber"])

	notAllowed, err := service.ResolvePublicServiceRoute(ctx, "POST", "/api/123456/store")
	require.NoError(t, err)
	assert.Nil(t, notAllowed)
}

func TestExtensionService_ResolveAdminServiceRouteByWorkspace(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceOne := testutil.NewIsolatedWorkspace(t)
	workspaceTwo := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceOne))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceTwo))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	install := func(workspaceID, target string) {
		installed, err := service.InstallExtension(ctx, InstallExtensionParams{
			WorkspaceID:  workspaceID,
			LicenseToken: "lic_" + workspaceID,
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
						Name:             "dashboard-refresh",
						Class:            platformdomain.ExtensionEndpointClassAdminAction,
						MountPath:        "/admin/extensions/ops/actions/refresh",
						Methods:          []string{"POST"},
						Auth:             platformdomain.ExtensionEndpointAuthSession,
						WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingFromSession,
						ServiceTarget:    "ops.dashboard." + target,
					},
				},
			},
		})
		require.NoError(t, err)
		_, err = service.ActivateExtension(ctx, installed.ID)
		require.NoError(t, err)
	}

	install(workspaceOne.ID, "one")
	install(workspaceTwo.ID, "two")

	first, err := service.ResolveAdminServiceRoute(ctx, workspaceOne.ID, "POST", "/admin/extensions/ops/actions/refresh")
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, "ops.dashboard.one", first.Endpoint.ServiceTarget)

	second, err := service.ResolveAdminServiceRoute(ctx, workspaceTwo.ID, "POST", "/admin/extensions/ops/actions/refresh")
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, "ops.dashboard.two", second.Endpoint.ServiceTarget)
}
