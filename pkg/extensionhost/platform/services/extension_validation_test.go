package platformservices_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil/refext"
)

func TestExtensionService_InspectExtensionSeededResourcesDetectsDrift(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	manifest, assets, migrations := loadTestExtensionPackage(t, "ats")
	service := newTestExtensionService(t, store)

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_ats",
		Manifest:      manifest,
		Assets:        assets,
		Migrations:    migrations,
	})
	require.NoError(t, err)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	report, err := service.InspectExtensionSeededResources(ctx, activated.ID)
	require.NoError(t, err)
	require.NotEmpty(t, report.Queues)
	require.NotEmpty(t, report.Forms)
	require.NotEmpty(t, report.AutomationRules)

	queue, err := store.Queues().GetQueueBySlug(ctx, workspace.ID, report.Queues[0].Slug)
	require.NoError(t, err)
	require.NoError(t, queue.Rename(queue.Name+" drift", queue.Description))
	require.NoError(t, store.Queues().UpdateQueue(ctx, queue))

	form, err := store.Forms().GetFormBySlug(ctx, workspace.ID, report.Forms[0].Slug)
	require.NoError(t, err)
	form.Theme = "broken-theme"
	require.NoError(t, store.Forms().UpdateFormSchema(ctx, form))

	rules, err := store.Rules().ListWorkspaceRules(ctx, workspace.ID)
	require.NoError(t, err)
	require.NotEmpty(t, rules)
	rules[0].Priority = rules[0].Priority + 10
	require.NoError(t, store.Rules().UpdateRule(ctx, rules[0]))

	report, err = service.InspectExtensionSeededResources(ctx, activated.ID)
	require.NoError(t, err)
	assert.False(t, report.Queues[0].MatchesSeed)
	assert.Contains(t, report.Queues[0].Problems[0], "name mismatch")
	assert.False(t, report.Forms[0].MatchesSeed)
	assert.Contains(t, report.Forms[0].Problems[0], "theme mismatch")
	assert.False(t, report.AutomationRules[0].MatchesSeed)
	assert.Contains(t, report.AutomationRules[0].Problems[0], "priority mismatch")
}

func TestExtensionService_FirstPartyExtensionsExposeCleanProofSurfaces(t *testing.T) {
	testCases := []struct {
		name            string
		expectedNav     int
		expectedWidgets int
		expectedQueues  int
		expectedForms   int
		expectedRules   int
		install         func(t *testing.T, ctx context.Context, store stores.Store, workspaceID string, service *platformservices.ExtensionService) *platformdomain.InstalledExtension
	}{
		{
			name:            "ats",
			expectedNav:     1,
			expectedWidgets: 1,
			expectedQueues:  2,
			expectedForms:   1,
			expectedRules:   2,
			install: func(t *testing.T, ctx context.Context, store stores.Store, workspaceID string, service *platformservices.ExtensionService) *platformdomain.InstalledExtension {
				manifest, assets, migrations := loadTestExtensionPackage(t, "ats")
				installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
					WorkspaceID:   workspaceID,
					InstalledByID: "user_123",
					LicenseToken:  "lic_ats",
					Manifest:      manifest,
					Assets:        assets,
					Migrations:    migrations,
				})
				require.NoError(t, err)
				activated, err := service.ActivateExtension(ctx, installed.ID)
				require.NoError(t, err)
				return activated
			},
		},
		{
			name:            "sales-pipeline",
			expectedNav:     1,
			expectedWidgets: 1,
			expectedQueues:  1,
			expectedForms:   1,
			expectedRules:   1,
			install: func(t *testing.T, ctx context.Context, store stores.Store, workspaceID string, service *platformservices.ExtensionService) *platformdomain.InstalledExtension {
				manifest, assets, migrations := loadTestExtensionPackage(t, "sales-pipeline")
				installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
					WorkspaceID:   workspaceID,
					InstalledByID: "user_123",
					LicenseToken:  "lic_sales",
					Manifest:      manifest,
					Assets:        assets,
					Migrations:    migrations,
				})
				require.NoError(t, err)
				activated, err := service.ActivateExtension(ctx, installed.ID)
				require.NoError(t, err)
				return activated
			},
		},
		{
			name:            "community-feature-requests",
			expectedNav:     1,
			expectedWidgets: 1,
			expectedQueues:  1,
			expectedForms:   0,
			expectedRules:   0,
			install: func(t *testing.T, ctx context.Context, store stores.Store, workspaceID string, service *platformservices.ExtensionService) *platformdomain.InstalledExtension {
				manifest, assets, migrations := loadTestExtensionPackage(t, "community-feature-requests")
				installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
					WorkspaceID:   workspaceID,
					InstalledByID: "user_123",
					LicenseToken:  "lic_community",
					Manifest:      manifest,
					Assets:        assets,
					Migrations:    migrations,
				})
				require.NoError(t, err)
				activated, err := service.ActivateExtension(ctx, installed.ID)
				require.NoError(t, err)
				return activated
			},
		},
		{
			name:            "web-analytics",
			expectedNav:     1,
			expectedWidgets: 1,
			expectedQueues:  0,
			expectedForms:   0,
			expectedRules:   0,
			install: func(t *testing.T, ctx context.Context, store stores.Store, workspaceID string, service *platformservices.ExtensionService) *platformdomain.InstalledExtension {
				return refext.InstallAndActivateReferenceExtension(t, ctx, store, workspaceID, "web-analytics")
			},
		},
		{
			name:            "error-tracking",
			expectedNav:     2,
			expectedWidgets: 0,
			expectedQueues:  0,
			expectedForms:   0,
			expectedRules:   0,
			install: func(t *testing.T, ctx context.Context, store stores.Store, workspaceID string, service *platformservices.ExtensionService) *platformdomain.InstalledExtension {
				return refext.InstallAndActivateReferenceExtension(t, ctx, store, workspaceID, "error-tracking")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store, cleanup := testutil.SetupTestStore(t)
			defer cleanup()

			ctx := context.Background()
			workspace := testutil.NewIsolatedWorkspace(t)
			require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

			service := newTestExtensionService(t, store)
			activated := tc.install(t, ctx, store, workspace.ID, service)

			nav, err := service.ListExtensionResolvedAdminNavigation(ctx, activated.ID)
			require.NoError(t, err)
			require.Len(t, nav, tc.expectedNav)
			for _, item := range nav {
				assert.Equal(t, activated.ID, item.ExtensionID)
				assert.Equal(t, activated.Slug, item.ExtensionSlug)
			}

			widgets, err := service.ListExtensionResolvedDashboardWidgets(ctx, activated.ID)
			require.NoError(t, err)
			require.Len(t, widgets, tc.expectedWidgets)
			for _, widget := range widgets {
				assert.Equal(t, activated.ID, widget.ExtensionID)
				assert.Equal(t, activated.Slug, widget.ExtensionSlug)
			}

			instanceNav, err := service.ListInstanceAdminNavigation(ctx)
			require.NoError(t, err)
			filteredInstanceNav := make([]platformservices.ResolvedExtensionAdminNavigationItem, 0, tc.expectedNav)
			for _, item := range instanceNav {
				if item.ExtensionID == activated.ID {
					filteredInstanceNav = append(filteredInstanceNav, item)
				}
			}
			require.Len(t, filteredInstanceNav, tc.expectedNav)
			for _, item := range filteredInstanceNav {
				assert.Equal(t, activated.WorkspaceID, item.WorkspaceID)
				assert.Contains(t, item.Href, "workspace="+url.QueryEscape(activated.WorkspaceID))
			}

			instanceWidgets, err := service.ListInstanceDashboardWidgets(ctx)
			require.NoError(t, err)
			filteredInstanceWidgets := make([]platformservices.ResolvedExtensionDashboardWidget, 0, tc.expectedWidgets)
			for _, widget := range instanceWidgets {
				if widget.ExtensionID == activated.ID {
					filteredInstanceWidgets = append(filteredInstanceWidgets, widget)
				}
			}
			require.Len(t, filteredInstanceWidgets, tc.expectedWidgets)
			for _, widget := range filteredInstanceWidgets {
				assert.Equal(t, activated.WorkspaceID, widget.WorkspaceID)
				assert.Contains(t, widget.Href, "workspace="+url.QueryEscape(activated.WorkspaceID))
			}

			report, err := service.InspectExtensionSeededResources(ctx, activated.ID)
			require.NoError(t, err)
			require.Len(t, report.Queues, tc.expectedQueues)
			require.Len(t, report.Forms, tc.expectedForms)
			require.Len(t, report.AutomationRules, tc.expectedRules)
			for _, state := range report.Queues {
				assert.True(t, state.MatchesSeed, "queue %s should match seed: %v", state.Slug, state.Problems)
			}
			for _, state := range report.Forms {
				assert.True(t, state.MatchesSeed, "form %s should match seed: %v", state.Slug, state.Problems)
			}
			for _, state := range report.AutomationRules {
				assert.True(t, state.MatchesSeed, "rule %s should match seed: %v", state.Key, state.Problems)
			}
		})
	}
}
