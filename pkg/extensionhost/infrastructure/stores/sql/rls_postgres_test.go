package sql_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

// TestRowLevelSecurityPolicyDeniesCrossWorkspace proves that the RLS helper and
// policy shape from migration 000011 (public.current_workspace_id reading the
// app.current_workspace_id session variable) actually deny cross-workspace
// reads when the querying role does not bypass row-level security. This is the
// security model that Store.SetTenantContext feeds: it issues exactly the
// set_config call asserted below. The assertions run under SET LOCAL ROLE
// because the migration/setup role bypasses RLS.
func TestRowLevelSecurityPolicyDeniesCrossWorkspace(t *testing.T) {
	dsn, cleanup := testutil.SetupTestPostgresDatabase(t)
	defer cleanup()

	db, err := sqlx.Connect("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()

	const (
		wsA = "11111111-1111-1111-1111-111111111111"
		wsB = "22222222-2222-2222-2222-222222222222"
	)

	setup := []string{
		// current_workspace_id() as defined by migration 000011. Recreated here
		// so the test exercises the exact RLS predicate function without
		// depending on the full migration set being applied to the bare test DB.
		`CREATE OR REPLACE FUNCTION public.current_workspace_id()
			RETURNS UUID LANGUAGE plpgsql STABLE SECURITY DEFINER AS $$
			BEGIN
				RETURN NULLIF(current_setting('app.current_workspace_id', true), '')::uuid;
			END;
			$$`,
		`DROP ROLE IF EXISTS mbr_rls_probe`,
		`CREATE ROLE mbr_rls_probe NOLOGIN`,
		`GRANT mbr_rls_probe TO CURRENT_USER`,
		`CREATE SCHEMA rls_probe_schema`,
		`GRANT USAGE ON SCHEMA rls_probe_schema TO mbr_rls_probe`,
		`CREATE TABLE rls_probe_schema.items (workspace_id uuid NOT NULL, note text NOT NULL)`,
		`GRANT SELECT, INSERT ON rls_probe_schema.items TO mbr_rls_probe`,
		`ALTER TABLE rls_probe_schema.items ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE rls_probe_schema.items FORCE ROW LEVEL SECURITY`,
		`CREATE POLICY tenant_isolation ON rls_probe_schema.items FOR ALL USING (workspace_id = public.current_workspace_id())`,
	}
	for _, stmt := range setup {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup failed for %q: %v", stmt, err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DROP SCHEMA IF EXISTS rls_probe_schema CASCADE`)
		_, _ = db.Exec(`DROP OWNED BY mbr_rls_probe`)
		_, _ = db.Exec(`DROP ROLE IF EXISTS mbr_rls_probe`)
	})

	// Seed rows for two workspaces as the setup role, which bypasses RLS.
	_, err = db.Exec(`INSERT INTO rls_probe_schema.items (workspace_id, note) VALUES ($1, 'a'), ($2, 'b')`, wsA, wsB)
	require.NoError(t, err)

	// countAs runs a select under the non-bypassing probe role, optionally
	// setting the workspace context exactly as Store.SetTenantContext does.
	countAs := func(t *testing.T, workspaceID string) int {
		t.Helper()
		tx, err := db.Beginx()
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		_, err = tx.Exec(`SET LOCAL ROLE mbr_rls_probe`)
		require.NoError(t, err)
		if workspaceID != "" {
			_, err = tx.Exec(`SELECT set_config('app.current_workspace_id', $1, true)`, workspaceID)
			require.NoError(t, err)
		}
		var n int
		require.NoError(t, tx.Get(&n, `SELECT count(*) FROM rls_probe_schema.items`))
		return n
	}

	require.Equal(t, 1, countAs(t, wsA), "workspace A must see only its own row")
	require.Equal(t, 1, countAs(t, wsB), "workspace B must see only its own row")
	require.Equal(t, 0, countAs(t, ""), "no workspace context must return no rows (fail-safe)")
}
