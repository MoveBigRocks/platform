package sql_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/lib/pq"

	sqlstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

// TestCoreRLSEnforcedUnderNonBypassingRole proves the full enforcement path on
// the real migrated schema. The application role owns the schema and does not
// bypass row-level security, mirroring production. With
// deploy/rls/enable-core-rls.sql applied, tenant data is visible only when the
// store sets the workspace context, is invisible with no context, and is fully
// visible through WithAdminContext (which assumes the mbr_admin bypass role).
// It also exercises deploy/rls/disable-core-rls.sql as the rollback path. This
// is the gate that must pass before production activation: it confirms
// isolation AND that the normal store paths still work.
func TestCoreRLSEnforcedUnderNonBypassingRole(t *testing.T) {
	testDSN, cleanupDB := testutil.SetupTestPostgresDatabase(t)
	defer cleanupDB()

	dbName := databaseName(t, testDSN)

	// Raw superuser connection to the fresh database. It does NOT run
	// migrations, so it never becomes the owner of the schema objects; that is
	// left to the application role below.
	suDB, err := sql.Open("postgres", testDSN)
	if err != nil {
		t.Fatalf("open superuser connection: %v", err)
	}
	defer suDB.Close()
	ctx := context.Background()

	// Provision the bypass role and the non-bypassing application login role,
	// and let the application role create the schema.
	appRole := "mbr_rls_app_" + uniqueRoleSuffix(testDSN)
	seedRLSRoles(t, suDB, appRole, dbName)
	defer dropRole(suDB, appRole)

	// Open the store AS the application role. NewDBWithConfig runs the
	// migrations, so the application role owns every schema object and is
	// subject to FORCE row-level security, exactly as in production.
	appStore, closeApp := openStoreAsRole(t, testDSN, appRole)
	defer closeApp()

	// Validate the rollback script, then the activation script, against the real
	// schema. Run as the superuser so ownership is irrelevant to the test setup.
	if _, err := suDB.ExecContext(ctx, readRepoFile(t, "deploy/rls/disable-core-rls.sql")); err != nil {
		t.Fatalf("apply disable-core-rls.sql: %v", err)
	}
	if got := rlsForced(t, suDB, "core_platform", "contacts"); got {
		t.Fatal("disable-core-rls.sql did not clear FORCE row security on contacts")
	}
	if _, err := suDB.ExecContext(ctx, readRepoFile(t, "deploy/rls/enable-core-rls.sql")); err != nil {
		t.Fatalf("apply enable-core-rls.sql: %v", err)
	}
	if got := rlsForced(t, suDB, "core_platform", "contacts"); !got {
		t.Fatal("enable-core-rls.sql did not set FORCE row security on contacts")
	}

	// Seed two workspaces of contacts as the superuser (which bypasses RLS).
	wsA := seedWorkspaceWithContacts(t, suDB, "alpha", "aa", 3)
	wsB := seedWorkspaceWithContacts(t, suDB, "bravo", "bb", 2)

	countContacts := func(setup func(ctx context.Context) error) int {
		t.Helper()
		var n int
		err := appStore.WithTransaction(ctx, func(txCtx context.Context) error {
			if setup != nil {
				if err := setup(txCtx); err != nil {
					return err
				}
			}
			return appStore.SqlxDB().Get(txCtx).GetContext(txCtx, &n, `SELECT count(*) FROM core_platform.contacts`)
		})
		if err != nil {
			t.Fatalf("count contacts: %v", err)
		}
		return n
	}

	if got := countContacts(func(c context.Context) error { return appStore.SetTenantContext(c, wsA) }); got != 3 {
		t.Fatalf("workspace A context: expected 3 contacts, got %d", got)
	}
	if got := countContacts(func(c context.Context) error { return appStore.SetTenantContext(c, wsB) }); got != 2 {
		t.Fatalf("workspace B context: expected 2 contacts, got %d", got)
	}
	if got := countContacts(nil); got != 0 {
		t.Fatalf("no context: expected 0 contacts (fail-safe), got %d", got)
	}

	// WithAdminContext switches to mbr_admin and sees every workspace. This also
	// proves the store resolved the admin role at construction.
	var adminCount int
	if err := appStore.WithAdminContext(ctx, func(txCtx context.Context) error {
		return appStore.SqlxDB().Get(txCtx).GetContext(txCtx, &adminCount, `SELECT count(*) FROM core_platform.contacts`)
	}); err != nil {
		t.Fatalf("admin context count: %v", err)
	}
	if adminCount != 5 {
		t.Fatalf("admin context: expected 5 contacts across workspaces, got %d", adminCount)
	}

	// A write is confined the same way: insert under A's context, then confirm B
	// does not see it and A does.
	if err := appStore.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := appStore.SetTenantContext(txCtx, wsA); err != nil {
			return err
		}
		_, err := appStore.SqlxDB().Get(txCtx).ExecContext(txCtx,
			`INSERT INTO core_platform.contacts (workspace_id, email) VALUES ($1, $2)`, wsA, "written@example.com")
		return err
	}); err != nil {
		t.Fatalf("insert under workspace A: %v", err)
	}
	if got := countContacts(func(c context.Context) error { return appStore.SetTenantContext(c, wsB) }); got != 2 {
		t.Fatalf("after cross-workspace write, workspace B should still see 2, got %d", got)
	}
	if got := countContacts(func(c context.Context) error { return appStore.SetTenantContext(c, wsA) }); got != 4 {
		t.Fatalf("workspace A should see its own new row (4), got %d", got)
	}
}

// seedRLSRoles provisions the mbr_admin bypass role and a non-bypassing login
// role that will own the schema (granted CREATE on the database and public
// schema so the migration runner can build it), plus membership in mbr_admin.
func seedRLSRoles(t *testing.T, db *sql.DB, appRole, dbName string) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='mbr_admin') THEN CREATE ROLE mbr_admin BYPASSRLS; ELSE ALTER ROLE mbr_admin BYPASSRLS; END IF; END $$;`,
		fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD 'rls_test_pw' NOBYPASSRLS`, appRole),
		fmt.Sprintf(`GRANT mbr_admin TO %s`, appRole),
		fmt.Sprintf(`GRANT CREATE ON DATABASE %s TO %s`, dbName, appRole),
		fmt.Sprintf(`GRANT CREATE, USAGE ON SCHEMA public TO %s`, appRole),
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed rls role stmt %q: %v", s, err)
		}
	}
}

func dropRole(db *sql.DB, appRole string) {
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, fmt.Sprintf(`REVOKE mbr_admin FROM %s`, appRole))
	_, _ = db.ExecContext(ctx, fmt.Sprintf(`DROP OWNED BY %s`, appRole))
	_, _ = db.ExecContext(ctx, fmt.Sprintf(`DROP ROLE IF EXISTS %s`, appRole))
}

// seedWorkspaceWithContacts inserts a workspace and n contacts and returns the
// workspace id. It runs on a superuser connection, which bypasses RLS.
func seedWorkspaceWithContacts(t *testing.T, db *sql.DB, slug, shortCode string, n int) string {
	t.Helper()
	ctx := context.Background()
	var wsID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO core_platform.workspaces (name, slug, short_code) VALUES ($1, $2, $3) RETURNING id`,
		slug, slug, shortCode,
	).Scan(&wsID); err != nil {
		t.Fatalf("seed workspace %s: %v", slug, err)
	}
	for i := 0; i < n; i++ {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO core_platform.contacts (workspace_id, email) VALUES ($1, $2)`,
			wsID, fmt.Sprintf("%s-%d@example.com", slug, i),
		); err != nil {
			t.Fatalf("seed contact %d for %s: %v", i, slug, err)
		}
	}
	return wsID
}

// rlsForced reports whether a table has FORCE row-level security set.
func rlsForced(t *testing.T, db *sql.DB, schema, table string) bool {
	t.Helper()
	var forced bool
	if err := db.QueryRowContext(context.Background(),
		`SELECT c.relforcerowsecurity FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1 AND c.relname = $2`, schema, table,
	).Scan(&forced); err != nil {
		t.Fatalf("read force row security for %s.%s: %v", schema, table, err)
	}
	return forced
}

// openStoreAsRole opens a store connected as the given login role. The migration
// runner builds the schema, so this role owns it.
func openStoreAsRole(t *testing.T, adminDSN, role string) (*sqlstore.Store, func()) {
	t.Helper()
	parsed, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	parsed.User = url.UserPassword(role, "rls_test_pw")
	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{DSN: parsed.String()})
	if err != nil {
		t.Fatalf("open store as role %s: %v", role, err)
	}
	store, err := sqlstore.NewStore(db)
	if err != nil {
		db.Close()
		t.Fatalf("create store as role %s: %v", role, err)
	}
	return store, func() {
		_ = store.Close()
		_ = db.Close()
	}
}

func databaseName(t *testing.T, dsn string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	return strings.TrimPrefix(parsed.Path, "/")
}

func uniqueRoleSuffix(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "x"
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	name = strings.ReplaceAll(name, "-", "")
	return strings.ToLower(name)
}

// readRepoFile reads a file relative to the module root, located by walking up
// from this test file until go.mod is found.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller for repo root")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", filepath.Dir(thisFile))
		}
		dir = parent
	}
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}
