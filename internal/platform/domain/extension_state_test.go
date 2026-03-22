package platformdomain

import (
	"encoding/json"
	"testing"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

func TestExtensionManifestJSONRoundTrip(t *testing.T) {
	config := shareddomain.NewTypedCustomFields()
	config.SetString("theme", "ops")

	manifest := ExtensionManifest{
		SchemaVersion: 2,
		Slug:          "web-analytics",
		Name:          "Web Analytics",
		Version:       "1.2.3",
		Publisher:     "DemandOps",
		Kind:          ExtensionKindOperational,
		Scope:         ExtensionScopeWorkspace,
		Risk:          ExtensionRiskStandard,
		RuntimeClass:  ExtensionRuntimeClassServiceBacked,
		StorageClass:  ExtensionStorageClassOwnedSchema,
		Schema: ExtensionSchemaManifest{
			Name:            "ext_demandops_web_analytics",
			PackageKey:      "demandops/web-analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: ExtensionRuntimeSpec{
			Protocol: ExtensionRuntimeProtocolInProcessHTTP,
		},
		Permissions:     []string{"case:read"},
		Queues:          []ExtensionQueueSeed{{Slug: "jobs", Name: "Jobs"}},
		Forms:           []ExtensionFormSeed{{Slug: "apply", Name: "Apply", AutoTags: []string{"candidate"}}},
		AutomationRules: []ExtensionAutomationSeed{{Key: "analytics-follow-up", Title: "Follow Up"}},
		PublicRoutes:    []ExtensionRoute{{PathPrefix: "/ext/web-analytics", AssetPath: "public/app.js"}},
		AdminRoutes:     []ExtensionRoute{{PathPrefix: "/admin/extensions/web-analytics", AssetPath: "admin/index.html"}},
		Endpoints: []ExtensionEndpoint{
			{
				Name:             "admin-home",
				Class:            ExtensionEndpointClassAdminPage,
				MountPath:        "/admin/extensions/web-analytics",
				Methods:          []string{"GET"},
				Auth:             ExtensionEndpointAuthSession,
				WorkspaceBinding: ExtensionWorkspaceBindingFromSession,
				ServiceTarget:    "analytics.admin.home",
			},
			{
				Name:             "runtime-health",
				Class:            ExtensionEndpointClassHealth,
				MountPath:        "/admin/extensions/web-analytics/health",
				Methods:          []string{"GET"},
				Auth:             ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "analytics.runtime.health",
			},
		},
		AdminNavigation:    []ExtensionAdminNavigationItem{{Name: "properties", Title: "Properties", Endpoint: "admin-home"}},
		DashboardWidgets:   []ExtensionDashboardWidget{{Name: "traffic", Title: "Traffic", Endpoint: "admin-home"}},
		Events:             ExtensionEventCatalog{Publishes: []ExtensionEventDefinition{{Type: "analytics.event.created", SchemaVersion: 1}}, Subscribes: []string{"case.created"}},
		EventConsumers:     []ExtensionEventConsumer{{Name: "consumer", Stream: "analytics-events", EventTypes: []string{"case.created"}, ServiceTarget: "analytics.consume"}},
		ScheduledJobs:      []ExtensionScheduledJob{{Name: "cleanup", IntervalSeconds: 300, ServiceTarget: "analytics.cleanup"}},
		Commands:           []ExtensionCommand{{Name: "web-analytics.rebuild", Description: "Rebuild"}},
		AgentSkills:        []ExtensionAgentSkill{{Name: "review-traffic", AssetPath: "agent-skills/review-traffic.md"}},
		CustomizableAssets: []string{"templates/report.html"},
		DefaultConfig:      config,
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	var decoded ExtensionManifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if decoded.Slug != manifest.Slug || len(decoded.Endpoints) != 2 || len(decoded.Commands) != 1 || len(decoded.AgentSkills) != 1 {
		t.Fatalf("unexpected decoded manifest: %#v", decoded)
	}
	if theme, ok := decoded.DefaultConfig.GetString("theme"); !ok || theme != "ops" {
		t.Fatalf("expected default config to round-trip, got %#v", decoded.DefaultConfig)
	}
}

func TestInstalledExtensionAndAssetLifecycle(t *testing.T) {
	manifest := ExtensionManifest{
		Slug:      "ats",
		Name:      "ATS",
		Version:   "1.0.0",
		Publisher: "DemandOps",
		Kind:      ExtensionKindProduct,
		Scope:     ExtensionScopeWorkspace,
		Risk:      ExtensionRiskStandard,
	}

	extension, err := NewInstalledExtension("ws_1", "user_1", "license_123", manifest, []byte("bundle"))
	if err != nil {
		t.Fatalf("new installed extension: %v", err)
	}
	if extension.IsInstanceScoped() {
		t.Fatal("workspace extension should not be instance scoped")
	}

	extension.Activate()
	if extension.Status != ExtensionStatusActive || extension.HealthStatus != ExtensionHealthHealthy {
		t.Fatalf("expected active extension, got %#v", extension)
	}

	extension.RecordHealth(ExtensionHealthDegraded, "degraded")
	if extension.HealthStatus != ExtensionHealthDegraded || extension.HealthMessage != "degraded" || extension.LastHealthCheckAt == nil {
		t.Fatalf("expected health update, got %#v", extension)
	}

	config := shareddomain.NewTypedCustomFields()
	config.SetBool("enabled", true)
	extension.UpdateConfig(config)
	if enabled, ok := extension.Config.GetBool("enabled"); !ok || !enabled {
		t.Fatalf("expected config to update, got %#v", extension.Config)
	}

	extension.MarkValidation(false, "")
	if extension.Status != ExtensionStatusFailed || extension.ValidationStatus != ExtensionValidationInvalid || extension.HealthStatus != ExtensionHealthFailed {
		t.Fatalf("expected invalid extension state, got %#v", extension)
	}

	extension.Deactivate("manual")
	if extension.Status != ExtensionStatusInactive || extension.HealthStatus != ExtensionHealthInactive || extension.HealthMessage != "manual" {
		t.Fatalf("expected inactive extension state, got %#v", extension)
	}

	asset, err := NewExtensionAsset("ext_1", "/templates/report.html", "text/html", []byte("hello"), true)
	if err != nil {
		t.Fatalf("new extension asset: %v", err)
	}
	if asset.Path != "templates/report.html" || asset.Kind != ExtensionAssetKindTemplate || asset.Size != int64(len([]byte("hello"))) {
		t.Fatalf("unexpected asset state: %#v", asset)
	}

	asset.UpdateContent([]byte("updated"), "text/plain")
	if string(asset.Content) != "updated" || asset.ContentType != "text/plain" || asset.Kind != ExtensionAssetKindTemplate {
		t.Fatalf("expected asset update to preserve kind and update content, got %#v", asset)
	}

	if DefaultExtensionHealthMessage(ExtensionHealthHealthy) == "" || DefaultExtensionHealthMessage(ExtensionHealthUnknown) == "" {
		t.Fatal("expected default health messages for known and unknown states")
	}
}
