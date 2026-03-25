package sql_test

import (
	"context"
	"testing"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestExtensionRuntimeStore_UpsertAndGetRegistration(t *testing.T) {
	t.Parallel()

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	registration, err := platformdomain.NewExtensionPackageRegistration(platformdomain.ExtensionManifest{
		Slug:         "web-analytics",
		Name:         "Web Analytics",
		Version:      "1.2.3",
		Publisher:    "DemandOps",
		Kind:         platformdomain.ExtensionKindOperational,
		Scope:        platformdomain.ExtensionScopeWorkspace,
		Risk:         platformdomain.ExtensionRiskStandard,
		RuntimeClass: platformdomain.ExtensionRuntimeClassServiceBacked,
		StorageClass: platformdomain.ExtensionStorageClassOwnedSchema,
		Schema: platformdomain.ExtensionSchemaManifest{
			Name:            "ext_demandops_web_analytics",
			PackageKey:      "demandops/web-analytics",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: platformdomain.ExtensionRuntimeSpec{
			Protocol:     "unix_socket_http",
			OCIReference: "registry.example.com/mbr/web-analytics:1.2.3",
			Digest:       "sha256:abc123",
		},
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:          "health",
				Class:         platformdomain.ExtensionEndpointClassHealth,
				MountPath:     "/extensions/web-analytics/health",
				Methods:       []string{"GET"},
				Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
				ServiceTarget: "web-analytics.runtime.health",
			},
		},
	})
	if err != nil {
		t.Fatalf("create registration: %v", err)
	}

	ctx := context.Background()
	if err := store.ExtensionRuntime().UpsertExtensionPackageRegistration(ctx, registration); err != nil {
		t.Fatalf("upsert registration: %v", err)
	}

	registration.MarkSchemaReady("2026.03.13")
	if err := store.ExtensionRuntime().UpsertExtensionPackageRegistration(ctx, registration); err != nil {
		t.Fatalf("update registration: %v", err)
	}

	got, err := store.ExtensionRuntime().GetExtensionPackageRegistration(ctx, "demandops/web-analytics")
	if err != nil {
		t.Fatalf("get registration: %v", err)
	}
	if got.CurrentSchemaVersion != "2026.03.13" {
		t.Fatalf("unexpected current schema version %q", got.CurrentSchemaVersion)
	}
	if got.Status != platformdomain.ExtensionSchemaRegistrationReady {
		t.Fatalf("unexpected registration status %q", got.Status)
	}
}

func TestExtensionRuntimeStore_CreateAndListMigrations(t *testing.T) {
	t.Parallel()

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()
	registration, err := platformdomain.NewExtensionPackageRegistration(platformdomain.ExtensionManifest{
		Slug:         "error-tracking",
		Name:         "Error Tracking",
		Version:      "2.0.0",
		Publisher:    "DemandOps",
		Kind:         platformdomain.ExtensionKindOperational,
		Scope:        platformdomain.ExtensionScopeWorkspace,
		Risk:         platformdomain.ExtensionRiskStandard,
		RuntimeClass: platformdomain.ExtensionRuntimeClassServiceBacked,
		StorageClass: platformdomain.ExtensionStorageClassOwnedSchema,
		Schema: platformdomain.ExtensionSchemaManifest{
			Name:            "ext_demandops_error_tracking",
			PackageKey:      "demandops/error-tracking",
			TargetVersion:   "2026.03.13",
			MigrationEngine: "postgres_sql",
		},
		Runtime: platformdomain.ExtensionRuntimeSpec{
			Protocol:     "unix_socket_http",
			OCIReference: "registry.example.com/mbr/error-tracking:2.0.0",
			Digest:       "sha256:def456",
		},
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:          "health",
				Class:         platformdomain.ExtensionEndpointClassHealth,
				MountPath:     "/extensions/error-tracking/health",
				Methods:       []string{"GET"},
				Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
				ServiceTarget: "error-tracking.runtime.health",
			},
		},
	})
	if err != nil {
		t.Fatalf("create registration: %v", err)
	}
	if err := store.ExtensionRuntime().UpsertExtensionPackageRegistration(ctx, registration); err != nil {
		t.Fatalf("upsert registration: %v", err)
	}

	first, err := platformdomain.NewExtensionSchemaMigration(
		"demandops/error-tracking",
		"ext_demandops_error_tracking",
		"000001",
		"abc123",
	)
	if err != nil {
		t.Fatalf("create migration: %v", err)
	}
	second, err := platformdomain.NewExtensionSchemaMigration(
		"demandops/error-tracking",
		"ext_demandops_error_tracking",
		"000002",
		"def456",
	)
	if err != nil {
		t.Fatalf("create migration: %v", err)
	}

	if err := store.ExtensionRuntime().CreateExtensionSchemaMigration(ctx, first); err != nil {
		t.Fatalf("create first migration: %v", err)
	}
	if err := store.ExtensionRuntime().CreateExtensionSchemaMigration(ctx, second); err != nil {
		t.Fatalf("create second migration: %v", err)
	}

	migrations, err := store.ExtensionRuntime().ListExtensionSchemaMigrations(ctx, "demandops/error-tracking")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("unexpected migration count %d", len(migrations))
	}
	if migrations[0].Version != "000001" || migrations[1].Version != "000002" {
		t.Fatalf("unexpected migration versions %#v", migrations)
	}
}
