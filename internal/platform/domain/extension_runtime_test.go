package platformdomain

import "testing"

func TestNewExtensionPackageRegistration(t *testing.T) {
	t.Parallel()

	registration, err := NewExtensionPackageRegistration(ExtensionManifest{
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
				Methods:          []string{"GET"},
				Auth:             ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "analytics.runtime.health",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected registration to be created, got %v", err)
	}
	if registration.PackageKey != "demandops/web-analytics" {
		t.Fatalf("unexpected package key %q", registration.PackageKey)
	}
	if registration.Status != ExtensionSchemaRegistrationPending {
		t.Fatalf("unexpected registration status %q", registration.Status)
	}
}

func TestNewExtensionSchemaMigration(t *testing.T) {
	t.Parallel()

	migration, err := NewExtensionSchemaMigration(
		"DemandOps/Web-Analytics",
		"Ext-DemandOps-Web-Analytics",
		"000001",
		"ABC123",
	)
	if err != nil {
		t.Fatalf("expected migration to be created, got %v", err)
	}
	if migration.PackageKey != "demandops/web-analytics" {
		t.Fatalf("unexpected package key %q", migration.PackageKey)
	}
	if migration.SchemaName != "ext_demandops_web_analytics" {
		t.Fatalf("unexpected schema name %q", migration.SchemaName)
	}
	if migration.ChecksumSHA256 != "abc123" {
		t.Fatalf("unexpected checksum %q", migration.ChecksumSHA256)
	}
}

func TestExtensionPackageRegistrationLifecycleAndValidation(t *testing.T) {
	t.Parallel()

	registration := &ExtensionPackageRegistration{
		PackageKey:             " DemandOps/Web-Analytics ",
		SchemaName:             " Ext-DemandOps-Web-Analytics ",
		RuntimeClass:           ExtensionRuntimeClass(" SERVICE_BACKED "),
		StorageClass:           ExtensionStorageClass(" OWNED_SCHEMA "),
		InstalledBundleVersion: " 1.2.3 ",
		DesiredSchemaVersion:   " 2026.03.13 ",
		CurrentSchemaVersion:   " 2026.03.12 ",
		Status:                 ExtensionSchemaRegistrationStatus(" READY "),
		LastError:              " ignored ",
	}

	registration.Normalize()
	if registration.PackageKey != "demandops/web-analytics" || registration.SchemaName != "ext_demandops_web_analytics" {
		t.Fatalf("unexpected normalized registration identity: %#v", registration)
	}
	if registration.RuntimeClass != ExtensionRuntimeClassServiceBacked || registration.StorageClass != ExtensionStorageClassOwnedSchema {
		t.Fatalf("unexpected normalized runtime/storage: %#v", registration)
	}
	if registration.Status != ExtensionSchemaRegistrationReady || registration.LastError != "ignored" {
		t.Fatalf("unexpected normalized status/error: %#v", registration)
	}
	if err := registration.Validate(); err != nil {
		t.Fatalf("expected normalized registration to validate, got %v", err)
	}

	registration.MarkSchemaReady(" 2026.03.13 ")
	if registration.Status != ExtensionSchemaRegistrationReady || registration.CurrentSchemaVersion != "2026.03.13" || registration.LastError != "" {
		t.Fatalf("expected ready registration, got %#v", registration)
	}

	registration.MarkSchemaFailed(" migration failed ")
	if registration.Status != ExtensionSchemaRegistrationFailed || registration.LastError != "migration failed" {
		t.Fatalf("expected failed registration, got %#v", registration)
	}
}

func TestExtensionPackageRegistrationInvalidScenarios(t *testing.T) {
	t.Parallel()

	invalid := &ExtensionPackageRegistration{}
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected empty registration to fail validation")
	}

	manifest := validServiceBackedOwnedSchemaManifest()
	manifest.RuntimeClass = ExtensionRuntimeClassBundle
	if _, err := NewExtensionPackageRegistration(manifest); err == nil {
		t.Fatal("expected bundle runtime registration to fail")
	}

	manifest = validServiceBackedOwnedSchemaManifest()
	manifest.StorageClass = ExtensionStorageClassSharedPrimitivesOnly
	if _, err := NewExtensionPackageRegistration(manifest); err == nil {
		t.Fatal("expected shared_primitives_only registration to fail")
	}

	if normalizeExtensionSchemaRegistrationStatus(" UNKNOWN ") != "" {
		t.Fatal("expected invalid registration status to normalize to empty")
	}
}

func TestExtensionSchemaMigrationValidationFailures(t *testing.T) {
	t.Parallel()

	migration := &ExtensionSchemaMigration{
		PackageKey:     " DemandOps/Web-Analytics ",
		SchemaName:     " Ext-DemandOps-Web-Analytics ",
		Version:        " 000001 ",
		ChecksumSHA256: " ABC123 ",
	}
	migration.Normalize()
	if migration.PackageKey != "demandops/web-analytics" || migration.SchemaName != "ext_demandops_web_analytics" || migration.ChecksumSHA256 != "abc123" {
		t.Fatalf("unexpected normalized migration: %#v", migration)
	}
	if err := migration.Validate(); err != nil {
		t.Fatalf("expected normalized migration to validate, got %v", err)
	}

	if _, err := NewExtensionSchemaMigration("", "", "", ""); err == nil {
		t.Fatal("expected empty migration constructor to fail")
	}
}

func validServiceBackedOwnedSchemaManifest() ExtensionManifest {
	return ExtensionManifest{
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
				Methods:          []string{"GET"},
				Auth:             ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "analytics.runtime.health",
			},
		},
	}
}
