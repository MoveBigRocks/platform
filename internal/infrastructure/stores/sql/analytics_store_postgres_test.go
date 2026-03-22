package sql_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"
	sqlstore "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestAnalyticsStoreUsesExtensionOwnedPostgresSchema(t *testing.T) {
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

	concrete := store

	_, err = sqlstore.NewAnalyticsDB(testDSN)
	require.ErrorContains(t, err, "analytics extension schema")

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

	installed, err := service.InstallExtension(ctx, loadReferenceExtensionInstallParams(t, "web-analytics", workspace.ID))
	require.NoError(t, err)
	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	require.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)

	analyticsDB, err := sqlstore.NewAnalyticsDB(testDSN)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, analyticsDB.Close())
	}()

	analyticsStore := sqlstore.NewAnalyticsStore(analyticsDB)

	property, err := analyticsdomain.NewProperty(workspace.ID, "example.com", "UTC")
	require.NoError(t, err)
	require.NoError(t, analyticsStore.CreateProperty(ctx, property))

	rawDB, err := concrete.GetSQLDB()
	require.NoError(t, err)

	var extensionInstallID string
	err = rawDB.QueryRowContext(ctx, `
		SELECT extension_install_id
		FROM ext_demandops_web_analytics.properties
		WHERE id = $1
	`, property.ID).Scan(&extensionInstallID)
	require.NoError(t, err)
	assert.Equal(t, installed.ID, extensionInstallID)

	salt, err := analyticsdomain.NewSalt()
	require.NoError(t, err)
	require.NoError(t, analyticsStore.InsertSalt(ctx, salt))

	now := time.Now().UTC()
	require.NoError(t, analyticsStore.InsertSession(ctx, &analyticsdomain.Session{
		SessionID:      sqlstore.GenerateSessionID(),
		PropertyID:     property.ID,
		VisitorID:      101,
		EntryPage:      "/",
		ExitPage:       "/",
		ReferrerSource: "Direct",
		CountryCode:    "NL",
		Browser:        "Safari",
		OS:             "macOS",
		DeviceType:     "desktop",
		StartedAt:      now,
		LastActivity:   now,
		Pageviews:      1,
		IsBounce:       1,
	}))
	require.NoError(t, analyticsStore.InsertEvent(ctx, &analyticsdomain.AnalyticsEvent{
		PropertyID:     property.ID,
		VisitorID:      101,
		Name:           "pageview",
		Pathname:       "/",
		ReferrerSource: "Direct",
		CountryCode:    "NL",
		Browser:        "Safari",
		OS:             "macOS",
		DeviceType:     "desktop",
		Timestamp:      now,
	}))

	metrics, err := analyticsStore.GetMetrics(ctx, property.ID, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 1, metrics.UniqueVisitors)
	assert.Equal(t, 1, metrics.TotalVisits)
	assert.Equal(t, 1, metrics.TotalPageviews)
}
