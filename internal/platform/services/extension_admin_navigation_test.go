package platformservices_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/refext"
)

func TestExtensionService_ListWorkspaceAdminNavigation(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "web-analytics")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "error-tracking")

	service := platformservices.NewExtensionService(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
	)

	items, err := service.ListWorkspaceAdminNavigation(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, items, 3)

	byHref := make(map[string]platformservices.ResolvedExtensionAdminNavigationItem, len(items))
	for _, item := range items {
		byHref[item.Href] = item
	}

	analytics := byHref["/extensions/web-analytics"]
	assert.Equal(t, "Analytics", analytics.Section)
	assert.Equal(t, "Web Analytics", analytics.Title)
	assert.Equal(t, "/extensions/web-analytics", analytics.Href)
	assert.Equal(t, "analytics", analytics.ActivePage)

	errorTrackingApplications := byHref["/extensions/error-tracking/applications"]
	assert.Equal(t, "Error Tracking", errorTrackingApplications.Section)
	assert.Equal(t, "Applications", errorTrackingApplications.Title)
	assert.Equal(t, "/extensions/error-tracking/applications", errorTrackingApplications.Href)
	assert.Equal(t, "applications", errorTrackingApplications.ActivePage)

	errorTrackingIssues := byHref["/extensions/error-tracking/issues"]
	assert.Equal(t, "Error Tracking", errorTrackingIssues.Section)
	assert.Equal(t, "Issues", errorTrackingIssues.Title)
	assert.Equal(t, "/extensions/error-tracking/issues", errorTrackingIssues.Href)
	assert.Equal(t, "issues", errorTrackingIssues.ActivePage)
}

func TestExtensionService_HasActiveExtensionInWorkspace(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := platformservices.NewExtensionService(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
	)

	enabled, err := service.HasActiveExtensionInWorkspace(ctx, workspace.ID, "web-analytics")
	require.NoError(t, err)
	assert.False(t, enabled)

	refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "web-analytics")

	enabled, err = service.HasActiveExtensionInWorkspace(ctx, workspace.ID, "web-analytics")
	require.NoError(t, err)
	assert.True(t, enabled)
}

func TestExtensionService_ListInstanceAdminNavigation(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	service := platformservices.NewExtensionService(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
	)

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
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
					Name:          "settings",
					Class:         platformdomain.ExtensionEndpointClassAdminPage,
					MountPath:     "/extensions/enterprise-access",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthSession,
					ServiceTarget: "enterprise-access.admin.settings",
				},
				{
					Name:          "health",
					Class:         platformdomain.ExtensionEndpointClassHealth,
					MountPath:     "/extensions/enterprise-access/health",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
					ServiceTarget: "enterprise-access.runtime.health",
				},
			},
			AdminNavigation: []platformdomain.ExtensionAdminNavigationItem{
				{
					Name:       "enterprise-access",
					Section:    "Identity",
					Title:      "Enterprise Access",
					Endpoint:   "settings",
					ActivePage: "enterprise-access",
				},
			},
		},
		BundleBase64: base64.StdEncoding.EncodeToString([]byte(`{"manifest":{}}`)),
	})
	require.NoError(t, err)
	installed.Status = platformdomain.ExtensionStatusActive
	require.NoError(t, store.Extensions().UpdateInstalledExtension(ctx, installed))

	items, err := service.ListInstanceAdminNavigation(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "/extensions/enterprise-access", items[0].Href)
	assert.Equal(t, "Identity", items[0].Section)
	assert.Equal(t, "Enterprise Access", items[0].Title)
}
