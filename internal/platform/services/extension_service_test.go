package platformservices

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestExtensionService_InstallActivateAndCustomize(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "ats",
		Name:          "Applicant Tracking",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindProduct,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		Permissions:   []string{"case:read", "case:write", "contact:read"},
		Queues: []platformdomain.ExtensionQueueSeed{
			{Slug: "backend-engineer", Name: "Backend Engineer"},
		},
		PublicRoutes: []platformdomain.ExtensionRoute{
			{PathPrefix: "/careers", AssetPath: "templates/careers/index.html"},
		},
		Commands: []platformdomain.ExtensionCommand{
			{Name: "ats.jobs.publish", Description: "Publish a job"},
		},
		AgentSkills: []platformdomain.ExtensionAgentSkill{
			{Name: "publish-job", Description: "Guide an agent through publishing a job", AssetPath: "agent-skills/publish-job.md"},
		},
		CustomizableAssets: []string{"templates/careers/index.html"},
	}

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_123",
		Manifest:      manifest,
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Careers</body></html>"),
			},
			{
				Path:        "agent-skills/publish-job.md",
				ContentType: "text/markdown",
				Content:     []byte("# Publish Job\n"),
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, installed)
	assert.Equal(t, platformdomain.ExtensionStatusInstalled, installed.Status)
	assert.Equal(t, platformdomain.ExtensionValidationValid, installed.ValidationStatus)

	assets, err := service.ListExtensionAssets(ctx, installed.ID)
	require.NoError(t, err)
	require.Len(t, assets, 2)
	assert.True(t, assets[0].IsCustomizable || assets[1].IsCustomizable)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)
	assert.Equal(t, platformdomain.ExtensionHealthHealthy, activated.HealthStatus)

	queues, err := store.Queues().ListWorkspaceQueues(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, queues, 1)
	assert.Equal(t, "backend-engineer", queues[0].Slug)

	updatedAsset, err := service.UpdateExtensionAsset(
		ctx,
		installed.ID,
		"templates/careers/index.html",
		[]byte("<html><body>Acme Careers</body></html>"),
		"text/html",
	)
	require.NoError(t, err)
	assert.Contains(t, string(updatedAsset.Content), "Acme Careers")

	updated, err := service.UpdateExtensionConfig(ctx, installed.ID, map[string]interface{}{
		"brand":     "Acme",
		"published": true,
		"stages":    []interface{}{"applied", "screening"},
	})
	require.NoError(t, err)

	brand, ok := updated.Config.GetString("brand")
	require.True(t, ok)
	assert.Equal(t, "Acme", brand)
	published, ok := updated.Config.GetBool("published")
	require.True(t, ok)
	assert.True(t, published)
	stages, ok := updated.Config.GetStrings("stages")
	require.True(t, ok)
	assert.Equal(t, []string{"applied", "screening"}, stages)
}

func TestExtensionService_UpgradePreservesCustomizableAssetsAndActiveState(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "ats",
		Name:          "Applicant Tracking",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindProduct,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		Permissions:   []string{"case:read", "case:write"},
		PublicRoutes: []platformdomain.ExtensionRoute{
			{PathPrefix: "/careers", AssetPath: "templates/careers/index.html"},
		},
		Commands: []platformdomain.ExtensionCommand{
			{Name: "ats.jobs.publish", Description: "Publish a job"},
		},
		CustomizableAssets: []string{"templates/careers/index.html"},
	}

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_123",
		Manifest:      manifest,
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Careers v1</body></html>"),
			},
		},
	})
	require.NoError(t, err)

	_, err = service.UpdateExtensionAsset(ctx, installed.ID, "templates/careers/index.html", []byte("<html><body>Acme Careers</body></html>"), "text/html")
	require.NoError(t, err)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	require.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)

	upgradedManifest := manifest
	upgradedManifest.Version = "1.1.0"
	upgradedManifest.Description = "Upgraded ATS bundle"

	upgraded, err := service.UpgradeExtension(ctx, UpgradeExtensionParams{
		ExtensionID:   installed.ID,
		InstalledByID: "user_456",
		Manifest:      upgradedManifest,
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Careers v2 default</body></html>"),
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", upgraded.Version)
	assert.Equal(t, "lic_123", upgraded.LicenseToken)
	assert.Equal(t, platformdomain.ExtensionStatusActive, upgraded.Status)
	assert.Equal(t, platformdomain.ExtensionHealthHealthy, upgraded.HealthStatus)

	assets, err := service.ListExtensionAssets(ctx, upgraded.ID)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, "templates/careers/index.html", assets[0].Path)
	assert.Contains(t, string(assets[0].Content), "Acme Careers")
}

func TestExtensionService_InstallInvalidAssetTopologyMarksFailed(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "web-analytics",
		Name:          "Web Analytics",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindProduct,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		PublicRoutes: []platformdomain.ExtensionRoute{
			{PathPrefix: "/analytics", AssetPath: "templates/analytics/index.html"},
		},
	}

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_456",
		Manifest:     manifest,
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/analytics/other.html",
				ContentType: "text/html",
				Content:     []byte("<html></html>"),
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusFailed, installed.Status)
	assert.Equal(t, platformdomain.ExtensionValidationInvalid, installed.ValidationStatus)
	assert.Contains(t, installed.ValidationMessage, "missing asset")

	_, err = service.ActivateExtension(ctx, installed.ID)
	require.Error(t, err)
}

func TestExtensionService_ActivateSeedsFormsAndAutomationRules(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	schema := shareddomain.NewTypedSchema()
	schema.Set("type", "object")
	schema.Set("required", []any{"full_name", "email"})
	schema.Set("properties", map[string]any{
		"full_name": map[string]any{"type": "string", "title": "Full name"},
		"email":     map[string]any{"type": "string", "title": "Email", "format": "email"},
	})

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "ats",
		Name:          "Applicant Tracking",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindProduct,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		WorkspacePlan: platformdomain.ExtensionWorkspacePlan{
			Mode: platformdomain.ExtensionWorkspaceProvisionDedicated,
			Name: "Hiring",
			Slug: "hiring",
		},
		Forms: []platformdomain.ExtensionFormSeed{
			{
				Slug:              "job-application",
				Name:              "Job Application",
				Description:       "Apply for an open role",
				Status:            string(servicedomain.FormStatusActive),
				IsPublic:          true,
				AllowMultiple:     true,
				CollectEmail:      true,
				AutoCreateCase:    true,
				AutoCasePriority:  "normal",
				AutoCaseType:      "web",
				AutoTags:          []string{"ats", "candidate", "applied"},
				SubmissionMessage: "Thanks for applying",
				Schema:            schema,
			},
		},
		AutomationRules: []platformdomain.ExtensionAutomationSeed{
			{
				Key:                  "ext.demandops.ats.application_form",
				Title:                "ATS application form",
				Description:          "Tag ATS cases for review",
				IsActive:             true,
				Priority:             50,
				MaxExecutionsPerHour: 100,
				MaxExecutionsPerDay:  1000,
				Conditions: shareddomain.TypedSchemaFromMap(map[string]any{
					"operator": "and",
					"conditions": []any{
						map[string]any{
							"type":     string(shareddomain.ConditionTypeField),
							"field":    "contact_email",
							"operator": string(shareddomain.OpContains),
							"value":    "example.com",
						},
					},
				}),
				Actions: shareddomain.TypedSchemaFromMap(map[string]any{
					"actions": []any{
						map[string]any{
							"type":  string(shareddomain.ActionTypeAddTag),
							"value": "ats-review",
						},
					},
				}),
			},
		},
		Events: platformdomain.ExtensionEventCatalog{
			Publishes: []platformdomain.ExtensionEventDefinition{
				{Type: "ext.demandops.ats.application_received", SchemaVersion: 1},
			},
			Subscribes: []string{"case.created", "form.submitted"},
		},
		PublicRoutes: []platformdomain.ExtensionRoute{
			{PathPrefix: "/careers", AssetPath: "templates/careers/index.html"},
		},
		CustomizableAssets: []string{"templates/careers/index.html"},
	}

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_ats",
		Manifest:      manifest,
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Careers</body></html>"),
			},
		},
	})
	require.NoError(t, err)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)

	forms, err := store.Forms().ListWorkspaceFormSchemas(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, forms, 1)
	assert.Equal(t, "job-application", forms[0].Slug)
	assert.True(t, forms[0].IsPublic)
	assert.Equal(t, []string{"ats", "candidate", "applied"}, forms[0].AutoTags)

	rules, err := store.Rules().ListWorkspaceRules(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "ext.demandops.ats.application_form", rules[0].SystemRuleKey)
	assert.True(t, rules[0].IsSystem)
	assert.True(t, rules[0].IsActive)
	assert.Equal(t, 1, len(rules[0].Actions.Actions))
	assert.Equal(t, string(shareddomain.ActionTypeAddTag), rules[0].Actions.Actions[0].Type)
	assert.Equal(t, "ats-review", rules[0].Actions.Actions[0].Value.AsString())
}

func TestExtensionService_ListWorkspaceAdminNavigationAndWidgets(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	manifest := platformdomain.ExtensionManifest{
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
				Name:             "ats-admin-dashboard",
				Class:            platformdomain.ExtensionEndpointClassAdminPage,
				MountPath:        "/admin/extensions/ats",
				Methods:          []string{"GET"},
				Auth:             platformdomain.ExtensionEndpointAuthSession,
				WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingFromSession,
				AssetPath:        "templates/admin/dashboard.html",
			},
		},
		AdminNavigation: []platformdomain.ExtensionAdminNavigationItem{
			{
				Name:       "ats-dashboard",
				Section:    "Extensions",
				Title:      "ATS",
				Icon:       "briefcase-business",
				Endpoint:   "ats-admin-dashboard",
				ActivePage: "ats",
			},
		},
		DashboardWidgets: []platformdomain.ExtensionDashboardWidget{
			{
				Name:        "ats-overview",
				Title:       "ATS Workspace",
				Description: "Open the hiring dashboard and review candidate workflows.",
				Icon:        "briefcase-business",
				Endpoint:    "ats-admin-dashboard",
			},
		},
	}

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_ats",
		Manifest:     manifest,
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/admin/dashboard.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>ATS</body></html>"),
			},
		},
	})
	require.NoError(t, err)

	_, err = service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	nav, err := service.ListWorkspaceAdminNavigation(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, nav, 1)
	assert.Equal(t, "Extensions", nav[0].Section)
	assert.Equal(t, "ATS", nav[0].Title)
	assert.Equal(t, "/admin/extensions/ats", nav[0].Href)
	assert.Equal(t, "ats", nav[0].ActivePage)

	widgets, err := service.ListWorkspaceDashboardWidgets(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, widgets, 1)
	assert.Equal(t, "ATS Workspace", widgets[0].Title)
	assert.Equal(t, "/admin/extensions/ats", widgets[0].Href)
}

func TestExtensionService_AllowsFirstPartyPrivilegedWorkspaceScopedExtensions(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_identity",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "enterprise-access",
			Name:          "Enterprise Access",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindIdentity,
			Scope:         platformdomain.ExtensionScopeWorkspace,
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
					Name:          "health",
					Class:         platformdomain.ExtensionEndpointClassHealth,
					MountPath:     "/admin/extensions/enterprise-access/health",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
					ServiceTarget: "enterprise-access.runtime.health",
				},
			},
		},
		BundleBase64: base64.StdEncoding.EncodeToString([]byte(`{"manifest":{}}`)),
	})
	require.NoError(t, err)
	require.NotNil(t, installed)
	assert.Equal(t, "enterprise-access", installed.Slug)
	assert.Equal(t, platformdomain.ExtensionStatusInstalled, installed.Status)
}

func TestExtensionService_RejectsUnsupportedPrivilegedInstallPolicies(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	instanceInstalled, err := service.InstallExtension(ctx, InstallExtensionParams{
		LicenseToken: "lic_identity_instance",
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
					Name:          "health",
					Class:         platformdomain.ExtensionEndpointClassHealth,
					MountPath:     "/admin/extensions/enterprise-access/health",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
					ServiceTarget: "enterprise-access.runtime.health",
				},
			},
		},
		BundleBase64: base64.StdEncoding.EncodeToString([]byte(`{"manifest":{}}`)),
	})
	require.NoError(t, err)
	require.NotNil(t, instanceInstalled)
	assert.Equal(t, "", instanceInstalled.WorkspaceID)
	assert.Equal(t, platformdomain.ExtensionScopeInstance, instanceInstalled.Manifest.Scope)

	_, err = service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_identity_instance_with_workspace",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "enterprise-access-alt",
			Name:          "Enterprise Access",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindIdentity,
			Scope:         platformdomain.ExtensionScopeInstance,
			Risk:          platformdomain.ExtensionRiskPrivileged,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_enterprise_access_alt",
				PackageKey:      "demandops/enterprise-access-alt",
				TargetVersion:   "1.0.0",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "health",
					Class:         platformdomain.ExtensionEndpointClassHealth,
					MountPath:     "/admin/extensions/enterprise-access-alt/health",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
					ServiceTarget: "enterprise-access.runtime.health",
				},
			},
		},
		BundleBase64: base64.StdEncoding.EncodeToString([]byte(`{"manifest":{}}`)),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace_id is not allowed")

	_, err = service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_connector",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "slack-alerts",
			Name:          "Slack Alerts",
			Version:       "1.0.0",
			Publisher:     "Custom Builder",
			Kind:          platformdomain.ExtensionKindConnector,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskPrivileged,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_custom_builder_slack_alerts",
				PackageKey:      "custom-builder/slack-alerts",
				TargetVersion:   "1.0.0",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "health",
					Class:         platformdomain.ExtensionEndpointClassHealth,
					MountPath:     "/admin/extensions/slack-alerts/health",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
					ServiceTarget: "slack-alerts.runtime.health",
				},
			},
		},
		BundleBase64: base64.StdEncoding.EncodeToString([]byte(`{"manifest":{}}`)),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trusted first-party publishers")

	_, err = service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_connector_standard",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "slack-alerts",
			Name:          "Slack Alerts",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindConnector,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_slack_alerts",
				PackageKey:      "demandops/slack-alerts",
				TargetVersion:   "1.0.0",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "health",
					Class:         platformdomain.ExtensionEndpointClassHealth,
					MountPath:     "/admin/extensions/slack-alerts/health",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
					ServiceTarget: "slack-alerts.runtime.health",
				},
			},
		},
		BundleBase64: base64.StdEncoding.EncodeToString([]byte(`{"manifest":{}}`)),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must declare risk=privileged")
}

func TestExtensionService_InstallInvalidEndpointAssetMarksFailed(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	manifest := platformdomain.ExtensionManifest{
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
				Name:      "tracking-script",
				Class:     platformdomain.ExtensionEndpointClassPublicAsset,
				MountPath: "/js/analytics.js",
				AssetPath: "public/analytics.js",
			},
		},
	}

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_endpoint",
		Manifest:     manifest,
		Assets: []ExtensionAssetInput{
			{
				Path:        "public/other.js",
				ContentType: "application/javascript",
				Content:     []byte("console.log('ok')"),
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusFailed, installed.Status)
	assert.Equal(t, platformdomain.ExtensionValidationInvalid, installed.ValidationStatus)
	assert.Contains(t, installed.ValidationMessage, "missing asset for endpoint")
}

func TestExtensionService_RejectsInvalidPublishedEventName(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	_, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_invalid_event",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "ats",
			Name:          "Applicant Tracking",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindProduct,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Events: platformdomain.ExtensionEventCatalog{
				Publishes: []platformdomain.ExtensionEventDefinition{
					{Type: "application.received"},
				},
			},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "published event")
}

func TestExtensionService_RejectsServiceBackedSourceInstallWithoutMigrations(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	_, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_backed",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_web_analytics",
				PackageKey:      "demandops/web-analytics",
				TargetVersion:   "2026.03.13",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/web-analytics:1.0.0",
				Digest:       "sha256:abc123",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/admin/extensions/web-analytics/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
	})
	require.Error(t, err)
	apiErr, ok := err.(*apierrors.APIError)
	require.True(t, ok)
	require.Contains(t, apiErr.Details, "validation_errors")
	validationErrors, ok := apiErr.Details["validation_errors"].([]apierrors.ValidationError)
	require.True(t, ok)
	require.Len(t, validationErrors, 1)
	assert.Equal(t, "migrations", validationErrors[0].Field)
}

func TestExtensionService_AllowsServiceBackedSourceInstallWithMigrations(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_backed",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_web_analytics",
				PackageKey:      "demandops/web-analytics",
				TargetVersion:   "2026.03.13",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/web-analytics:1.0.0",
				Digest:       "sha256:abc123",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/admin/extensions/web-analytics/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
		Migrations: []ExtensionMigrationInput{
			{
				Path:    "000001_init.up.sql",
				Content: []byte("create table ${SCHEMA_NAME}.events (id text);"),
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, installed)
	assert.Equal(t, platformdomain.ExtensionStatusInstalled, installed.Status)

	bundle, err := store.Extensions().GetExtensionBundle(ctx, installed.ID)
	require.NoError(t, err)
	var payload struct {
		Migrations []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"migrations"`
	}
	require.NoError(t, json.Unmarshal(bundle, &payload))
	require.Len(t, payload.Migrations, 1)
	assert.Equal(t, "000001_init.up.sql", payload.Migrations[0].Path)
	decoded, err := base64.StdEncoding.DecodeString(payload.Migrations[0].Content)
	require.NoError(t, err)
	assert.Contains(t, string(decoded), "create table ${SCHEMA_NAME}.events")
}

func TestExtensionService_CheckExtensionHealthUsesConfiguredRuntime(t *testing.T) {
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
		WithExtensionHealthRuntime(mockExtensionHealthRuntime{
			status:  platformdomain.ExtensionHealthHealthy,
			message: "runtime healthy",
		}),
	)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_backed",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_web_analytics",
				PackageKey:      "demandops/web-analytics",
				TargetVersion:   "2026.03.13",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/web-analytics:1.0.0",
				Digest:       "sha256:abc123",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/admin/extensions/web-analytics/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
		Migrations: []ExtensionMigrationInput{
			{Path: "000001_init.up.sql", Content: []byte("create table ${SCHEMA_NAME}.events (id text);")},
		},
	})
	require.NoError(t, err)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionHealthHealthy, activated.HealthStatus)
	assert.Equal(t, "runtime healthy", activated.HealthMessage)
	require.NotNil(t, activated.LastHealthCheckAt)
}

func TestExtensionService_CheckExtensionHealthDegradesWhenRuntimeMissing(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_backed",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_web_analytics",
				PackageKey:      "demandops/web-analytics",
				TargetVersion:   "2026.03.13",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/web-analytics:1.0.0",
				Digest:       "sha256:abc123",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/admin/extensions/web-analytics/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
		Migrations: []ExtensionMigrationInput{
			{Path: "000001_init.up.sql", Content: []byte("create table ${SCHEMA_NAME}.events (id text);")},
		},
	})
	require.NoError(t, err)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionHealthDegraded, activated.HealthStatus)
	assert.Equal(t, "service runtime health checks are not configured", activated.HealthMessage)
}

func TestExtensionService_GetExtensionRuntimeDiagnosticsUsesConfiguredRuntime(t *testing.T) {
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
		WithExtensionDiagnosticsRuntime(mockExtensionDiagnosticsRuntime{
			diagnostics: platformdomain.ExtensionRuntimeDiagnostics{
				EventConsumers: []platformdomain.ExtensionRuntimeConsumerState{
					{Name: "case-created", Status: "healthy", Stream: "case-events", ServiceTarget: "consumer.case-created"},
				},
				ScheduledJobs: []platformdomain.ExtensionRuntimeJobState{
					{Name: "maintenance", Status: "healthy", IntervalSeconds: 60, ServiceTarget: "jobs.maintenance"},
				},
			},
		}),
	)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_backed",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_web_analytics",
				PackageKey:      "demandops/web-analytics",
				TargetVersion:   "2026.03.13",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/web-analytics:1.0.0",
				Digest:       "sha256:abc123",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/admin/extensions/web-analytics/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
		Migrations: []ExtensionMigrationInput{
			{Path: "000001_init.up.sql", Content: []byte("create table ${SCHEMA_NAME}.events (id text);")},
		},
	})
	require.NoError(t, err)

	diagnostics, err := service.GetExtensionRuntimeDiagnostics(ctx, installed.ID)
	require.NoError(t, err)
	require.Len(t, diagnostics.EventConsumers, 1)
	require.Len(t, diagnostics.ScheduledJobs, 1)
	assert.Equal(t, "healthy", diagnostics.EventConsumers[0].Status)
	assert.Equal(t, "healthy", diagnostics.ScheduledJobs[0].Status)
}

func TestExtensionService_ActivateExtensionFailsWhenConfiguredRuntimeIsUnavailable(t *testing.T) {
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
		WithExtensionActivationRuntime(mockExtensionActivationRuntime{
			err: assert.AnError,
		}),
	)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_backed",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_web_analytics",
				PackageKey:      "demandops/web-analytics",
				TargetVersion:   "2026.03.13",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/web-analytics:1.0.0",
				Digest:       "sha256:abc123",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/admin/extensions/web-analytics/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
		Migrations: []ExtensionMigrationInput{
			{Path: "000001_init.up.sql", Content: []byte("create table ${SCHEMA_NAME}.events (id text);")},
		},
	})
	require.NoError(t, err)

	_, err = service.ActivateExtension(ctx, installed.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extension runtime activation failed")
}

func TestExtensionService_DeactivateExtensionDrainsConfiguredRuntime(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	runtime := &mockExtensionActivationRuntime{}
	service := NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		WithExtensionActivationRuntime(runtime),
	)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_service_backed",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "web-analytics",
			Name:          "Web Analytics",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_web_analytics",
				PackageKey:      "demandops/web-analytics",
				TargetVersion:   "2026.03.13",
				MigrationEngine: "postgres_sql",
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/web-analytics:1.0.0",
				Digest:       "sha256:abc123",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/admin/extensions/web-analytics/health",
					Methods:          []string{"GET"},
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
		Migrations: []ExtensionMigrationInput{
			{Path: "000001_init.up.sql", Content: []byte("create table ${SCHEMA_NAME}.events (id text);")},
		},
	})
	require.NoError(t, err)

	_, err = service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	deactivated, err := service.DeactivateExtension(ctx, installed.ID, "maintenance window")
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusInactive, deactivated.Status)
	assert.True(t, runtime.deactivateCalled)
	assert.Equal(t, installed.ID, runtime.deactivatedExtensionID)
	assert.Equal(t, "maintenance window", runtime.deactivateReason)
}

func TestExtensionService_UninstallRequiresDeactivationAndSoftDeletesInstallation(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "ats",
		Name:          "Applicant Tracking",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindProduct,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		PublicRoutes: []platformdomain.ExtensionRoute{
			{PathPrefix: "/careers", AssetPath: "templates/careers/index.html"},
		},
		CustomizableAssets: []string{"templates/careers/index.html"},
	}

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_123",
		Manifest:      manifest,
		Assets: []ExtensionAssetInput{
			{
				Path:        "templates/careers/index.html",
				ContentType: "text/html",
				Content:     []byte("<html><body>Careers</body></html>"),
			},
		},
	})
	require.NoError(t, err)

	_, err = service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)

	err = service.UninstallExtension(ctx, installed.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be deactivated before uninstall")

	_, err = service.DeactivateExtension(ctx, installed.ID, "ready to remove")
	require.NoError(t, err)

	err = service.UninstallExtension(ctx, installed.ID)
	require.NoError(t, err)

	_, err = service.GetInstalledExtension(ctx, installed.ID)
	require.Error(t, err)

	extensions, err := service.ListWorkspaceExtensions(ctx, workspace.ID)
	require.NoError(t, err)
	assert.Empty(t, extensions)

	assets, err := service.ListExtensionAssets(ctx, installed.ID)
	require.NoError(t, err)
	assert.Empty(t, assets)
}

type mockExtensionHealthRuntime struct {
	status  platformdomain.ExtensionHealthStatus
	message string
	err     error
}

func (m mockExtensionHealthRuntime) CheckInstalledExtensionHealth(context.Context, *platformdomain.InstalledExtension) (platformdomain.ExtensionHealthStatus, string, error) {
	return m.status, m.message, m.err
}

type mockExtensionActivationRuntime struct {
	err                    error
	deactivateErr          error
	deactivateCalled       bool
	deactivatedExtensionID string
	deactivateReason       string
}

func (m mockExtensionActivationRuntime) EnsureInstalledExtensionRuntime(context.Context, *platformdomain.InstalledExtension) error {
	return m.err
}

func (m *mockExtensionActivationRuntime) DeactivateInstalledExtensionRuntime(_ context.Context, extension *platformdomain.InstalledExtension, reason string) error {
	m.deactivateCalled = true
	if extension != nil {
		m.deactivatedExtensionID = extension.ID
	}
	m.deactivateReason = reason
	return m.deactivateErr
}

type mockExtensionDiagnosticsRuntime struct {
	diagnostics platformdomain.ExtensionRuntimeDiagnostics
	err         error
}

func (m mockExtensionDiagnosticsRuntime) GetInstalledExtensionRuntimeDiagnostics(context.Context, *platformdomain.InstalledExtension) (platformdomain.ExtensionRuntimeDiagnostics, error) {
	return m.diagnostics, m.err
}
