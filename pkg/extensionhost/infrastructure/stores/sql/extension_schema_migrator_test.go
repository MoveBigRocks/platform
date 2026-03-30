package sql_test

import (
	"context"
	stdsql "database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestExtensionSchemaMigrator_ActivateAppliesAndReusesPackageSchema(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	concrete, ok := store.(*sqlstore.Store)
	require.True(t, ok)

	service := platformservices.NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		platformservices.WithExtensionSchemaRuntime(concrete.ExtensionSchemaMigrator()),
	)

	ctx := context.Background()
	workspaceOne := testutil.NewIsolatedWorkspace(t)
	workspaceTwo := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceOne))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceTwo))

	install := func(workspaceID, licenseToken string) *platformdomain.InstalledExtension {
		installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
			WorkspaceID:  workspaceID,
			LicenseToken: licenseToken,
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
					TargetVersion:   "000002",
					MigrationEngine: "postgres_sql",
				},
				Endpoints: []platformdomain.ExtensionEndpoint{
					{
						Name:          "runtime-health",
						Class:         platformdomain.ExtensionEndpointClassHealth,
						MountPath:     "/extensions/web-analytics/health",
						Methods:       []string{"GET"},
						Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
						ServiceTarget: "web-analytics.runtime.health",
					},
				},
				Runtime: platformdomain.ExtensionRuntimeSpec{
					Protocol:     "unix_socket_http",
					OCIReference: "registry.example.com/mbr/web-analytics:1.0.0",
					Digest:       "sha256:abc123",
				},
			},
			Migrations: []platformservices.ExtensionMigrationInput{
				{
					Path: "000001_init.up.sql",
					Content: []byte(`
CREATE TABLE ${SCHEMA_NAME}.events (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`),
				},
				{
					Path: "000002_add_name.up.sql",
					Content: []byte(`
ALTER TABLE ${SCHEMA_NAME}.events
    ADD COLUMN name TEXT;`),
				},
			},
		})
		require.NoError(t, err)
		return installed
	}

	first := install(workspaceOne.ID, "lic_schema_one")
	activated, err := service.ActivateExtension(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)

	registration, err := store.ExtensionRuntime().GetExtensionPackageRegistration(ctx, "demandops/web-analytics")
	require.NoError(t, err)
	assert.Equal(t, "ext_demandops_web_analytics", registration.SchemaName)
	assert.Equal(t, "000002", registration.CurrentSchemaVersion)
	assert.Equal(t, platformdomain.ExtensionSchemaRegistrationReady, registration.Status)

	history, err := store.ExtensionRuntime().ListExtensionSchemaMigrations(ctx, "demandops/web-analytics")
	require.NoError(t, err)
	require.Len(t, history, 2)

	rawDB, err := concrete.GetSQLDB()
	require.NoError(t, err)
	assertColumnExists(t, rawDB, "ext_demandops_web_analytics", "events", "name")

	second := install(workspaceTwo.ID, "lic_schema_two")
	_, err = service.ActivateExtension(ctx, second.ID)
	require.NoError(t, err)

	history, err = store.ExtensionRuntime().ListExtensionSchemaMigrations(ctx, "demandops/web-analytics")
	require.NoError(t, err)
	assert.Len(t, history, 2)
}

func TestExtensionSchemaMigrator_RejectsChecksumDrift(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	concrete, ok := store.(*sqlstore.Store)
	require.True(t, ok)

	service := platformservices.NewExtensionService(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
	)

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_schema_drift",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "error-tracking",
			Name:          "Error Tracking",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_error_tracking",
				PackageKey:      "demandops/error-tracking",
				TargetVersion:   "000001",
				MigrationEngine: "postgres_sql",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "runtime-health",
					Class:         platformdomain.ExtensionEndpointClassHealth,
					MountPath:     "/extensions/error-tracking/health",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
					ServiceTarget: "error-tracking.runtime.health",
				},
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/error-tracking:1.0.0",
				Digest:       "sha256:def456",
			},
		},
		Migrations: []platformservices.ExtensionMigrationInput{
			{
				Path: "000001_init.up.sql",
				Content: []byte(`
CREATE TABLE ${SCHEMA_NAME}.issues (
    id TEXT PRIMARY KEY
);`),
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, concrete.ExtensionSchemaMigrator().EnsureInstalledExtensionSchema(ctx, installed))

	bundlePayload, err := store.Extensions().GetExtensionBundle(ctx, installed.ID)
	require.NoError(t, err)

	var bundle struct {
		Manifest   map[string]any `json:"manifest"`
		Migrations []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"migrations"`
	}
	require.NoError(t, json.Unmarshal(bundlePayload, &bundle))
	require.Len(t, bundle.Migrations, 1)
	bundle.Migrations[0].Content = base64.StdEncoding.EncodeToString([]byte(`
CREATE TABLE ${SCHEMA_NAME}.issues (
    id TEXT PRIMARY KEY,
    fingerprint TEXT
);`))
	mutatedPayload, err := json.Marshal(bundle)
	require.NoError(t, err)

	rawDB, err := concrete.GetSQLDB()
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `UPDATE core_platform.installed_extensions SET bundle_payload = $1 WHERE id = $2`, mutatedPayload, installed.ID)
	require.NoError(t, err)

	err = concrete.ExtensionSchemaMigrator().EnsureInstalledExtensionSchema(ctx, installed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum drift")

	registration, regErr := store.ExtensionRuntime().GetExtensionPackageRegistration(ctx, "demandops/error-tracking")
	require.NoError(t, regErr)
	assert.Equal(t, platformdomain.ExtensionSchemaRegistrationFailed, registration.Status)
	assert.True(t, strings.Contains(registration.LastError, "checksum drift"))
}

func TestExtensionSchemaMigrator_AcceptsLegacyRawSQLMigrationBodies(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	concrete, ok := store.(*sqlstore.Store)
	require.True(t, ok)

	service := platformservices.NewExtensionService(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
	)

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_schema_legacy",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "error-tracking",
			Name:          "Error Tracking",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
			StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
			Schema: platformdomain.ExtensionSchemaManifest{
				Name:            "ext_demandops_error_tracking",
				PackageKey:      "demandops/error-tracking",
				TargetVersion:   "000001",
				MigrationEngine: "postgres_sql",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "runtime-health",
					Class:         platformdomain.ExtensionEndpointClassHealth,
					MountPath:     "/extensions/error-tracking/health",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
					ServiceTarget: "error-tracking.runtime.health",
				},
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/error-tracking:1.0.0",
				Digest:       "sha256:def456",
			},
		},
		Migrations: []platformservices.ExtensionMigrationInput{
			{
				Path: "000001_init.up.sql",
				Content: []byte(`
CREATE TABLE ${SCHEMA_NAME}.issues (
    id TEXT PRIMARY KEY
);`),
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, concrete.ExtensionSchemaMigrator().EnsureInstalledExtensionSchema(ctx, installed))

	bundlePayload, err := store.Extensions().GetExtensionBundle(ctx, installed.ID)
	require.NoError(t, err)

	var bundle struct {
		Manifest   map[string]any `json:"manifest"`
		Migrations []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"migrations"`
	}
	require.NoError(t, json.Unmarshal(bundlePayload, &bundle))
	require.Len(t, bundle.Migrations, 1)
	legacySQL, err := base64.StdEncoding.DecodeString(bundle.Migrations[0].Content)
	require.NoError(t, err)
	bundle.Migrations[0].Content = string(legacySQL)
	mutatedPayload, err := json.Marshal(bundle)
	require.NoError(t, err)

	rawDB, err := concrete.GetSQLDB()
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `UPDATE core_platform.installed_extensions SET bundle_payload = $1 WHERE id = $2`, mutatedPayload, installed.ID)
	require.NoError(t, err)

	err = concrete.ExtensionSchemaMigrator().EnsureInstalledExtensionSchema(ctx, installed)
	require.NoError(t, err)
}

func TestExtensionSchemaMigrator_AcceptsLegacyMigrationHistoryMarkers(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	concrete, ok := store.(*sqlstore.Store)
	require.True(t, ok)

	service := platformservices.NewExtensionService(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
	)

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_schema_legacy_markers",
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
				TargetVersion:   "000002",
				MigrationEngine: "postgres_sql",
			},
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "runtime-health",
					Class:         platformdomain.ExtensionEndpointClassHealth,
					MountPath:     "/extensions/web-analytics/health",
					Methods:       []string{"GET"},
					Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
					ServiceTarget: "web-analytics.runtime.health",
				},
			},
			Runtime: platformdomain.ExtensionRuntimeSpec{
				Protocol:     "unix_socket_http",
				OCIReference: "registry.example.com/mbr/web-analytics:1.0.0",
				Digest:       "sha256:abc123",
			},
		},
		Migrations: []platformservices.ExtensionMigrationInput{
			{
				Path: "000001_init.up.sql",
				Content: []byte(`
CREATE TABLE ${SCHEMA_NAME}.events (
    id TEXT PRIMARY KEY
);`),
			},
			{
				Path: "000002_rls.up.sql",
				Content: []byte(`
ALTER TABLE ${SCHEMA_NAME}.events
    ENABLE ROW LEVEL SECURITY;`),
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, concrete.ExtensionSchemaMigrator().EnsureInstalledExtensionSchema(ctx, installed))

	rawDB, err := concrete.GetSQLDB()
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		UPDATE core_extension_runtime.schema_migration_history
		SET checksum_sha256 = CASE version
			WHEN '000001' THEN 'init'
			WHEN '000002' THEN 'rls_skipped'
			ELSE checksum_sha256
		END
		WHERE package_key = 'demandops/web-analytics'
	`)
	require.NoError(t, err)

	err = concrete.ExtensionSchemaMigrator().EnsureInstalledExtensionSchema(ctx, installed)
	require.NoError(t, err)
}

func TestExtensionSchemaMigrator_ReferenceExtensionBundlesApply(t *testing.T) {
	testCases := []struct {
		name       string
		packageKey string
		schema     string
		tables     []string
		policies   []string
	}{
		{
			name:       "web-analytics",
			packageKey: "demandops/web-analytics",
			schema:     "ext_demandops_web_analytics",
			tables:     []string{"properties", "hostname_rules", "goals", "events", "sessions", "salts"},
			policies:   []string{"properties_tenant_isolation", "events_tenant_isolation", "sessions_tenant_isolation"},
		},
		{
			name:       "error-tracking",
			packageKey: "demandops/error-tracking",
			schema:     "ext_demandops_error_tracking",
			tables:     []string{"projects", "issues", "error_events", "alerts", "project_stats", "issue_stats", "git_repos"},
			policies:   []string{"projects_tenant_isolation", "error_events_tenant_isolation", "git_repos_tenant_isolation"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store, cleanup := testutil.SetupTestPostgresStore(t)
			defer cleanup()

			concrete, ok := store.(*sqlstore.Store)
			require.True(t, ok)

			service := platformservices.NewExtensionServiceWithOptions(
				store.Extensions(),
				store.Workspaces(),
				store.Queues(),
				store.Forms(),
				store.Rules(),
				store,
				platformservices.WithExtensionSchemaRuntime(concrete.ExtensionSchemaMigrator()),
			)

			ctx := context.Background()
			workspace := testutil.NewIsolatedWorkspace(t)
			require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

			params := loadReferenceExtensionInstallParams(t, tc.name, workspace.ID)
			installed, err := service.InstallExtension(ctx, params)
			require.NoError(t, err)

			activated, err := service.ActivateExtension(ctx, installed.ID)
			require.NoError(t, err)
			assert.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)

			registration, err := store.ExtensionRuntime().GetExtensionPackageRegistration(ctx, tc.packageKey)
			require.NoError(t, err)
			assert.Equal(t, tc.schema, registration.SchemaName)
			assert.Equal(t, params.Manifest.Schema.TargetVersion, registration.CurrentSchemaVersion)
			assert.Equal(t, platformdomain.ExtensionSchemaRegistrationReady, registration.Status)

			rawDB, err := concrete.GetSQLDB()
			require.NoError(t, err)
			for _, tableName := range tc.tables {
				assertTableExists(t, rawDB, tc.schema, tableName)
			}
			for _, policyName := range tc.policies {
				assertPolicyExists(t, rawDB, tc.schema, policyName)
			}
		})
	}
}

func assertColumnExists(t *testing.T, db *stdsql.DB, schemaName, tableName, columnName string) {
	t.Helper()

	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2 AND column_name = $3
		)
	`, schemaName, tableName, columnName).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "expected column %s.%s.%s to exist", schemaName, tableName, columnName)
}

func assertTableExists(t *testing.T, db *stdsql.DB, schemaName, tableName string) {
	t.Helper()

	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2
		)
	`, schemaName, tableName).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "expected table %s.%s to exist", schemaName, tableName)
}

func assertPolicyExists(t *testing.T, db *stdsql.DB, schemaName, policyName string) {
	t.Helper()

	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_policies
			WHERE schemaname = $1 AND policyname = $2
		)
	`, schemaName, policyName).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "expected policy %s.%s to exist", schemaName, policyName)
}

func loadReferenceExtensionInstallParams(t *testing.T, extensionName, workspaceID string) platformservices.InstallExtensionParams {
	t.Helper()

	root := referenceExtensionRoot(t, extensionName)
	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	require.NoError(t, err)

	var manifest platformdomain.ExtensionManifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))

	migrations := loadReferenceExtensionMigrations(t, filepath.Join(root, "migrations"))
	return platformservices.InstallExtensionParams{
		WorkspaceID:  workspaceID,
		LicenseToken: "lic_" + strings.ReplaceAll(extensionName, "-", "_"),
		Manifest:     manifest,
		Migrations:   migrations,
	}
}

func loadReferenceExtensionMigrations(t *testing.T, root string) []platformservices.ExtensionMigrationInput {
	t.Helper()

	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	migrations := make([]platformservices.ExtensionMigrationInput, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, entry.Name()))
		require.NoError(t, err)
		migrations = append(migrations, platformservices.ExtensionMigrationInput{
			Path:    entry.Name(),
			Content: content,
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Path < migrations[j].Path
	})
	require.NotEmpty(t, migrations, "expected ATS extension migrations in %s", root)
	return migrations
}

func referenceExtensionRoot(t *testing.T, extensionName string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	root, err := testutil.ResolveWorkspaceSiblingDir(repoRoot, filepath.Join("extensions", extensionName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("reference extension checkout not available for %s", extensionName)
		}
		require.NoError(t, err)
	}
	return root
}
