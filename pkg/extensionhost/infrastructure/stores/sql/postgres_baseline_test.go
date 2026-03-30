package sql_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestPostgresBaselineExcludesExtensionOwnedObservabilityTables(t *testing.T) {
	testDSN, cleanupDatabase := testutil.SetupTestPostgresDatabase(t)
	defer cleanupDatabase()

	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{DSN: testDSN})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	store, err := sqlstore.NewStore(db)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	rawDB, err := store.GetSQLDB()
	require.NoError(t, err)

	ctx := context.Background()
	for _, tableName := range []string{
		"projects",
		"issues",
		"error_events",
		"alerts",
		"project_stats",
		"issue_stats",
		"git_repos",
	} {
		t.Run(tableName, func(t *testing.T) {
			var regclass sql.NullString
			err := rawDB.QueryRowContext(ctx, `SELECT to_regclass($1)`, "public."+tableName).Scan(&regclass)
			require.NoError(t, err)
			assert.False(t, regclass.Valid, "expected %s to be absent from public baseline", tableName)
		})
	}
}

func TestPostgresBaselineCreatesCoreTablesInBoundedContextSchemas(t *testing.T) {
	testDSN, cleanupDatabase := testutil.SetupTestPostgresDatabase(t)
	defer cleanupDatabase()

	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{DSN: testDSN})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	store, err := sqlstore.NewStore(db)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	rawDB, err := store.GetSQLDB()
	require.NoError(t, err)

	ctx := context.Background()
	for _, tableName := range []string{
		"core_infra.outbox_events",
		"core_identity.users",
		"core_platform.workspaces",
		"core_service.case_queues",
		"core_service.queue_items",
		"core_service.service_catalog_nodes",
		"core_service.conversation_sessions",
		"core_service.cases",
		"core_service.form_specs",
		"core_service.form_access_tokens",
		"core_automation.rules",
		"core_knowledge.knowledge_resources",
		"core_knowledge.case_knowledge_resource_links",
		"core_governance.audit_logs",
		"core_extension_runtime.extension_package_registrations",
	} {
		t.Run(tableName, func(t *testing.T) {
			var regclass sql.NullString
			err := rawDB.QueryRowContext(ctx, `SELECT to_regclass($1)`, tableName).Scan(&regclass)
			require.NoError(t, err)
			assert.True(t, regclass.Valid, "expected %s to exist in the PostgreSQL baseline", tableName)
		})
	}
}

func TestPostgresBaselineExcludesRemovedKnowledgeAndFormTables(t *testing.T) {
	testDSN, cleanupDatabase := testutil.SetupTestPostgresDatabase(t)
	defer cleanupDatabase()

	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{DSN: testDSN})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	store, err := sqlstore.NewStore(db)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	rawDB, err := store.GetSQLDB()
	require.NoError(t, err)

	ctx := context.Background()
	// form_submissions now exists as the core form_submissions table;
	// form_schemas, form_api_tokens, and form_analytics were old web
	// form tables removed in earlier migrations and remain absent.
	for _, tableName := range []string{
		"core_service.form_schemas",
		"core_service.form_api_tokens",
		"core_service.form_analytics",
		"core_knowledge.knowledge_bases",
		"core_knowledge.article_categories",
		"core_knowledge.articles",
		"core_knowledge.case_kb_articles",
		"core_knowledge.ai_generation_requests",
	} {
		t.Run(tableName, func(t *testing.T) {
			var regclass sql.NullString
			err := rawDB.QueryRowContext(ctx, `SELECT to_regclass($1)`, tableName).Scan(&regclass)
			require.NoError(t, err)
			assert.False(t, regclass.Valid, "expected %s to remain absent from the PostgreSQL baseline", tableName)
		})
	}
}

func TestPostgresBaselineCreatesFinalInstalledExtensionsShape(t *testing.T) {
	testDSN, cleanupDatabase := testutil.SetupTestPostgresDatabase(t)
	defer cleanupDatabase()

	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{DSN: testDSN})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	store, err := sqlstore.NewStore(db)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	rawDB, err := store.GetSQLDB()
	require.NoError(t, err)

	ctx := context.Background()

	var workspaceNullable string
	err = rawDB.QueryRowContext(ctx, `
		SELECT is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'core_platform'
		  AND table_name = 'installed_extensions'
		  AND column_name = 'workspace_id'
	`).Scan(&workspaceNullable)
	require.NoError(t, err)
	assert.Equal(t, "YES", workspaceNullable)

	var bundlePayloadType string
	err = rawDB.QueryRowContext(ctx, `
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = 'core_platform'
		  AND table_name = 'installed_extensions'
		  AND column_name = 'bundle_payload'
	`).Scan(&bundlePayloadType)
	require.NoError(t, err)
	assert.Equal(t, "bytea", bundlePayloadType)

	for _, indexName := range []string{
		"core_platform.idx_installed_extensions_workspace_slug_active",
		"core_platform.idx_installed_extensions_instance_slug_active",
	} {
		t.Run(indexName, func(t *testing.T) {
			var regclass sql.NullString
			err := rawDB.QueryRowContext(ctx, `SELECT to_regclass($1)`, indexName).Scan(&regclass)
			require.NoError(t, err)
			assert.True(t, regclass.Valid, "expected %s to exist in the PostgreSQL baseline", indexName)
		})
	}
}
