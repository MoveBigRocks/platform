package platformdomain

import (
	"strings"
	"testing"
)

func TestExtensionManifestValidateEndpoints(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:      "analytics-pack",
		Name:      "Analytics Pack",
		Version:   "1.0.0",
		Publisher: "DemandOps",
		Kind:      ExtensionKindProduct,
		Scope:     ExtensionScopeWorkspace,
		Risk:      ExtensionRiskStandard,
		Endpoints: []ExtensionEndpoint{
			{
				Name:      "tracking-script",
				Class:     ExtensionEndpointClassPublicAsset,
				MountPath: "/js/analytics.js",
				AssetPath: "public/analytics.js",
			},
			{
				Name:          "capture",
				Class:         ExtensionEndpointClassPublicIngest,
				MountPath:     "/ingest/ext/web-analytics/event",
				Methods:       []string{"post"},
				ServiceTarget: "analytics.capture",
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected manifest to validate, got error: %v", err)
	}
}

func TestExtensionManifestValidateArtifactSurfaceEndpoints(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:      "ats",
		Name:      "Applicant Tracking",
		Version:   "1.0.0",
		Publisher: "DemandOps",
		Kind:      ExtensionKindProduct,
		Scope:     ExtensionScopeWorkspace,
		Risk:      ExtensionRiskStandard,
		ArtifactSurfaces: []ExtensionArtifactSurface{
			{
				Name:          "website",
				SeedAssetPath: "templates/careers",
			},
		},
		Endpoints: []ExtensionEndpoint{
			{
				Name:            "careers-home",
				Class:           ExtensionEndpointClassPublicPage,
				MountPath:       "/careers",
				ArtifactSurface: "website",
				ArtifactPath:    "index.html",
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected artifact-backed manifest to validate, got error: %v", err)
	}
}

func TestExtensionManifestValidateRejectsInvalidEndpoint(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:      "broken-pack",
		Name:      "Broken Pack",
		Version:   "1.0.0",
		Publisher: "DemandOps",
		Kind:      ExtensionKindProduct,
		Scope:     ExtensionScopeWorkspace,
		Risk:      ExtensionRiskStandard,
		Endpoints: []ExtensionEndpoint{
			{
				Name:      "health",
				Class:     ExtensionEndpointClassHealth,
				MountPath: "/health",
				Auth:      ExtensionEndpointAuthPublic,
			},
		},
	}

	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected manifest validation to fail")
	}
}

func TestExtensionManifestNormalizeDefaultsRuntimeAndStorage(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:      "analytics-pack",
		Name:      "Analytics Pack",
		Version:   "1.0.0",
		Publisher: "DemandOps",
		Kind:      ExtensionKindProduct,
		Scope:     ExtensionScopeWorkspace,
		Risk:      ExtensionRiskStandard,
	}

	manifest.Normalize()

	if manifest.RuntimeClass != ExtensionRuntimeClassBundle {
		t.Fatalf("expected bundle runtime default, got %q", manifest.RuntimeClass)
	}
	if manifest.StorageClass != ExtensionStorageClassSharedPrimitivesOnly {
		t.Fatalf("expected shared_primitives_only storage default, got %q", manifest.StorageClass)
	}
}

func TestExtensionManifestValidateServiceBackedRequirements(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "web-analytics",
		Name:         "Web Analytics",
		Version:      "1.2.3",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindOperational,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		StorageClass: ExtensionStorageClassOwnedSchema,
		Schema: ExtensionSchemaManifest{
			Name:            "Ext-DemandOps-Web-Analytics",
			PackageKey:      "DemandOps/Web-Analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "POSTGRES_SQL",
		},
		Runtime: ExtensionRuntimeSpec{
			Protocol: ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []ExtensionEndpoint{
			{
				Name:             "capture",
				Class:            ExtensionEndpointClassPublicIngest,
				MountPath:        "/ingest/ext/web-analytics/event",
				Methods:          []string{"post"},
				Auth:             ExtensionEndpointAuthPublic,
				WorkspaceBinding: ExtensionWorkspaceBindingNone,
				ServiceTarget:    "analytics.capture",
			},
			{
				Name:             "runtime-health",
				Class:            ExtensionEndpointClassHealth,
				MountPath:        "/admin/extensions/web-analytics/health",
				Methods:          []string{"get"},
				Auth:             ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "analytics.runtime.health",
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected valid service-backed manifest, got %v", err)
	}
	if manifest.Schema.Name != "ext_demandops_web_analytics" {
		t.Fatalf("expected normalized schema name, got %q", manifest.Schema.Name)
	}
	if manifest.Schema.PackageKey != "demandops/web-analytics" {
		t.Fatalf("expected normalized package key, got %q", manifest.Schema.PackageKey)
	}
	if manifest.PackageKey() != "demandops/web-analytics" {
		t.Fatalf("expected manifest package key, got %q", manifest.PackageKey())
	}
}

func TestExtensionManifestValidateRejectsServiceBackedPackageKeyMismatch(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "web-analytics",
		Name:         "Web Analytics",
		Version:      "1.2.3",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindOperational,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		StorageClass: ExtensionStorageClassOwnedSchema,
		Schema: ExtensionSchemaManifest{
			Name:            "ext_demandops_web_analytics",
			PackageKey:      "other-publisher/web-analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: ExtensionRuntimeSpec{
			Protocol: ExtensionRuntimeProtocolInProcessHTTP,
		},
	}

	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected package key mismatch to fail validation")
	}
}

func TestExtensionManifestValidateRejectsServiceBackedManifestWithoutHealthEndpoint(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "web-analytics",
		Name:         "Web Analytics",
		Version:      "1.2.3",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindOperational,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		StorageClass: ExtensionStorageClassOwnedSchema,
		Schema: ExtensionSchemaManifest{
			Name:            "ext_demandops_web_analytics",
			PackageKey:      "demandops/web-analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: ExtensionRuntimeSpec{
			Protocol: ExtensionRuntimeProtocolInProcessHTTP,
		},
	}

	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected missing service-backed health endpoint to fail validation")
	}
}

func TestExtensionManifestValidateAcceptsAdminNavigationForAdminPageEndpoints(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "web-analytics",
		Name:         "Web Analytics",
		Version:      "1.2.3",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindOperational,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		StorageClass: ExtensionStorageClassOwnedSchema,
		Schema: ExtensionSchemaManifest{
			Name:            "ext_demandops_web_analytics",
			PackageKey:      "demandops/web-analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: ExtensionRuntimeSpec{
			Protocol: ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []ExtensionEndpoint{
			{
				Name:             "admin-home",
				Class:            ExtensionEndpointClassAdminPage,
				MountPath:        "/admin/extensions/web-analytics",
				Methods:          []string{"get"},
				Auth:             ExtensionEndpointAuthSession,
				WorkspaceBinding: ExtensionWorkspaceBindingFromSession,
				ServiceTarget:    "analytics.admin.properties",
			},
			{
				Name:             "runtime-health",
				Class:            ExtensionEndpointClassHealth,
				MountPath:        "/admin/extensions/web-analytics/health",
				Methods:          []string{"get"},
				Auth:             ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "analytics.runtime.health",
			},
		},
		AdminNavigation: []ExtensionAdminNavigationItem{
			{
				Name:     "properties",
				Section:  "Analytics",
				Title:    "Web Analytics",
				Icon:     "bar-chart-3",
				Endpoint: "admin-home",
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected admin navigation to validate, got %v", err)
	}
}

func TestExtensionManifestValidateAcceptsEventConsumersAndScheduledJobs(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "web-analytics",
		Name:         "Web Analytics",
		Version:      "1.2.3",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindOperational,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		StorageClass: ExtensionStorageClassOwnedSchema,
		Schema: ExtensionSchemaManifest{
			Name:            "ext_demandops_web_analytics",
			PackageKey:      "demandops/web-analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: ExtensionRuntimeSpec{
			Protocol: ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []ExtensionEndpoint{
			{
				Name:             "runtime-health",
				Class:            ExtensionEndpointClassHealth,
				MountPath:        "/admin/extensions/web-analytics/health",
				Methods:          []string{"get"},
				Auth:             ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "analytics.runtime.health",
			},
		},
		Events: ExtensionEventCatalog{
			Subscribes: []string{"case.created"},
		},
		EventConsumers: []ExtensionEventConsumer{
			{
				Name:          "case-created-consumer",
				Stream:        "case-events",
				EventTypes:    []string{"case.created"},
				ServiceTarget: "analytics.consumer.case-created",
			},
		},
		ScheduledJobs: []ExtensionScheduledJob{
			{
				Name:            "maintenance",
				IntervalSeconds: 300,
				ServiceTarget:   "analytics.job.maintenance",
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected event consumers and scheduled jobs to validate, got %v", err)
	}
}

func TestExtensionManifestValidateRejectsConsumerWithoutSubscribedEvent(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "web-analytics",
		Name:         "Web Analytics",
		Version:      "1.2.3",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindOperational,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		StorageClass: ExtensionStorageClassOwnedSchema,
		Schema: ExtensionSchemaManifest{
			Name:            "ext_demandops_web_analytics",
			PackageKey:      "demandops/web-analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: ExtensionRuntimeSpec{
			Protocol: ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []ExtensionEndpoint{
			{
				Name:             "runtime-health",
				Class:            ExtensionEndpointClassHealth,
				MountPath:        "/admin/extensions/web-analytics/health",
				Methods:          []string{"get"},
				Auth:             ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "analytics.runtime.health",
			},
		},
		EventConsumers: []ExtensionEventConsumer{
			{
				Name:          "missing-subscription",
				Stream:        "case-events",
				EventTypes:    []string{"case.created"},
				ServiceTarget: "analytics.consumer.case-created",
			},
		},
	}

	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected consumer validation to fail when eventTypes are not declared in events.subscribes")
	}
}

func TestExtensionManifestValidateRejectsAdminNavigationWithoutAdminPageEndpoint(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "web-analytics",
		Name:         "Web Analytics",
		Version:      "1.2.3",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindOperational,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		StorageClass: ExtensionStorageClassOwnedSchema,
		Schema: ExtensionSchemaManifest{
			Name:            "ext_demandops_web_analytics",
			PackageKey:      "demandops/web-analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: ExtensionRuntimeSpec{
			Protocol: ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []ExtensionEndpoint{
			{
				Name:             "runtime-health",
				Class:            ExtensionEndpointClassHealth,
				MountPath:        "/admin/extensions/web-analytics/health",
				Methods:          []string{"get"},
				Auth:             ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "analytics.runtime.health",
			},
		},
		AdminNavigation: []ExtensionAdminNavigationItem{
			{
				Name:     "properties",
				Title:    "Web Analytics",
				Endpoint: "missing-admin-page",
			},
		},
	}

	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected admin navigation without admin page endpoint to fail validation")
	}
}

func TestExtensionManifestValidateRequiresRuntimeArtifactForUnixSocketProtocol(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "web-analytics",
		Name:         "Web Analytics",
		Version:      "1.2.3",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindOperational,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		StorageClass: ExtensionStorageClassOwnedSchema,
		Schema: ExtensionSchemaManifest{
			Name:            "ext_demandops_web_analytics",
			PackageKey:      "demandops/web-analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: ExtensionRuntimeSpec{
			Protocol: ExtensionRuntimeProtocolUnixSocketHTTP,
		},
		Endpoints: []ExtensionEndpoint{
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
	}

	err := manifest.Validate()
	if err == nil {
		t.Fatalf("expected unix_socket_http protocol without artifact metadata to fail validation")
	}
	if !strings.Contains(err.Error(), "runtime.ociReference") {
		t.Fatalf("expected missing oci reference validation error, got %v", err)
	}
}

func TestExtensionManifestValidateRejectsUnnamespacedCommands(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "ats",
		Name:         "Applicant Tracking",
		Version:      "1.0.0",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindProduct,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassBundle,
		StorageClass: ExtensionStorageClassSharedPrimitivesOnly,
		Commands: []ExtensionCommand{
			{Name: "jobs.publish"},
		},
	}

	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected unnamespaced command validation to fail")
	}
}

func TestExtensionManifestValidateAcceptsNamespacedCommandsAndUniqueSkills(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:         "ats",
		Name:         "Applicant Tracking",
		Version:      "1.0.0",
		Publisher:    "DemandOps",
		Kind:         ExtensionKindProduct,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskStandard,
		RuntimeClass: ExtensionRuntimeClassBundle,
		StorageClass: ExtensionStorageClassSharedPrimitivesOnly,
		Commands: []ExtensionCommand{
			{Name: "ats.jobs.publish"},
			{Name: "ats.jobs.close"},
		},
		AgentSkills: []ExtensionAgentSkill{
			{Name: "publish-job", AssetPath: "agent-skills/publish-job.md"},
			{Name: "review-candidates", AssetPath: "agent-skills/review-candidates.md"},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected manifest to validate, got %v", err)
	}
}

func TestExtensionInstallPolicies(t *testing.T) {
	t.Parallel()

	product := ExtensionManifest{
		Kind:  ExtensionKindProduct,
		Scope: ExtensionScopeWorkspace,
		Risk:  ExtensionRiskStandard,
	}
	if product.RequiresPrivilegedInstallPolicy() {
		t.Fatal("expected product extension to use generic install policy")
	}
	if err := product.ValidateGenericInstallPolicy(); err != nil {
		t.Fatalf("expected generic install policy to pass, got %v", err)
	}

	identity := ExtensionManifest{
		Kind:         ExtensionKindIdentity,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskPrivileged,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		Publisher:    "DemandOps",
	}
	if !identity.RequiresPrivilegedInstallPolicy() {
		t.Fatal("expected identity extension to require privileged install policy")
	}
	if err := identity.ValidatePrivilegedInstallPolicy("ws_1", func(publisher string) bool {
		return strings.EqualFold(strings.TrimSpace(publisher), "demandops")
	}); err != nil {
		t.Fatalf("expected privileged install policy to pass, got %v", err)
	}
}

func TestInstalledExtensionBaseHealthStatus(t *testing.T) {
	t.Parallel()

	extension := &InstalledExtension{
		Status: ExtensionStatusInactive,
	}
	status, message, ok := extension.BaseHealthStatus()
	if !ok || status != ExtensionHealthInactive || message != "extension inactive" {
		t.Fatalf("expected inactive base health, got status=%q message=%q ok=%v", status, message, ok)
	}

	extension = &InstalledExtension{
		Status:           ExtensionStatusActive,
		ValidationStatus: ExtensionValidationInvalid,
	}
	status, message, ok = extension.BaseHealthStatus()
	if !ok || status != ExtensionHealthFailed || message != "extension validation failed" {
		t.Fatalf("expected validation failure base health, got status=%q message=%q ok=%v", status, message, ok)
	}

	extension = &InstalledExtension{
		Status:           ExtensionStatusActive,
		ValidationStatus: ExtensionValidationValid,
		Manifest: ExtensionManifest{
			RuntimeClass: ExtensionRuntimeClassServiceBacked,
		},
	}
	status, message, ok = extension.BaseHealthStatus()
	if ok || status != "" || message != "" {
		t.Fatalf("expected service-backed extension to require runtime health, got status=%q message=%q ok=%v", status, message, ok)
	}
}

func TestExtensionNormalizationHelpers(t *testing.T) {
	t.Parallel()

	if got := normalizeStringList([]string{" read ", "", "write", "read"}); len(got) != 2 || got[0] != "read" || got[1] != "write" {
		t.Fatalf("unexpected normalized string list %#v", got)
	}
	if got := normalizeExtensionRuleKey(" Follow Up Rule "); got != "follow_up_rule" {
		t.Fatalf("unexpected normalized rule key %q", got)
	}
	if got := normalizePathList([]string{" ./templates/report.html ", "/public/app.js", "templates/report.html"}); len(got) != 2 || got[0] != "templates/report.html" || got[1] != "public/app.js" {
		t.Fatalf("unexpected normalized path list %#v", got)
	}
	if !isValidPublishedExtensionEventType("ext.demandops.analytics.created") {
		t.Fatal("expected published extension event type to validate")
	}
	if isValidPublishedExtensionEventType("case.created") {
		t.Fatal("expected non-extension event type to fail validation")
	}
	if inferAssetKind("public/app.js") != ExtensionAssetKindStatic || inferAssetKind("misc/notes.txt") != ExtensionAssetKindOther {
		t.Fatal("expected asset kind inference to cover static and other kinds")
	}
}

func TestExtensionManifestValidatePublishedExtensionEvents(t *testing.T) {
	t.Parallel()

	manifest := ExtensionManifest{
		Slug:      "analytics-pack",
		Name:      "Analytics Pack",
		Version:   "1.0.0",
		Publisher: "DemandOps",
		Kind:      ExtensionKindOperational,
		Scope:     ExtensionScopeWorkspace,
		Risk:      ExtensionRiskStandard,
		Events: ExtensionEventCatalog{
			Publishes: []ExtensionEventDefinition{
				{Type: "ext.demandops.analytics_pack.created", SchemaVersion: 1},
				{Type: "case.created", SchemaVersion: 1},
			},
		},
	}

	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "published event case.created") {
		t.Fatalf("expected invalid published event type to fail validation, got %v", err)
	}
}
