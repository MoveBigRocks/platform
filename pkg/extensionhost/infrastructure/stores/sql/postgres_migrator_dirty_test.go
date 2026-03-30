package sql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestNewDBWithConfigRejectsDirtyPostgresMigrationLedger(t *testing.T) {
	testDSN, cleanup := testutil.SetupTestPostgresDatabase(t)
	defer cleanup()

	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{
		DSN: testDSN,
	})
	require.NoError(t, err)

	rawDB, err := db.GetSQLDB()
	require.NoError(t, err)
	_, err = rawDB.ExecContext(context.Background(), `
		UPDATE public.schema_migrations
		SET dirty = TRUE
		WHERE version = (SELECT version FROM public.schema_migrations ORDER BY version DESC LIMIT 1)
	`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = sqlstore.NewDBWithConfig(sqlstore.DBConfig{
		DSN: testDSN,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dirty")
}
