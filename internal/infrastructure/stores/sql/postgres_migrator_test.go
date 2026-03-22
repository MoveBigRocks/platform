package sql

import (
	"slices"
	"testing"
	"testing/fstest"

	migrationsfs "github.com/movebigrocks/platform/migrations"
)

func TestLoadMigrationsFromPathsOrdersByVersion(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"postgres/000003_final.up.sql": {Data: []byte("select 3;")},
		"000001_init.up.sql":           {Data: []byte("select 1;")},
		"postgres/000002_mid.up.sql":   {Data: []byte("select 2;")},
	}

	migrations, err := loadMigrationsFromPaths(fsys, []string{
		"postgres/000003_final.up.sql",
		"000001_init.up.sql",
		"postgres/000002_mid.up.sql",
	})
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	if len(migrations) != 3 {
		t.Fatalf("unexpected migration count: %d", len(migrations))
	}
	if migrations[0].Version != "000001" || migrations[1].Version != "000002" || migrations[2].Version != "000003" {
		t.Fatalf("unexpected migration order: %#v", migrations)
	}
	if migrations[0].Checksum == "" || migrations[1].Checksum == "" || migrations[2].Checksum == "" {
		t.Fatal("expected checksums to be populated")
	}
}

func TestLoadPostgresMigrationsUsesEmbeddedManifest(t *testing.T) {
	t.Parallel()

	migrations, err := loadPostgresMigrations(fstest.MapFS{
		"postgres/000001_core_platform.up.sql":          {Data: []byte("select 1;")},
		"postgres/000002_core_integrity.up.sql":         {Data: []byte("select 2;")},
		"postgres/000003_core_store_readiness.up.sql":   {Data: []byte("select 3;")},
		"postgres/000004_core_extension_runtime.up.sql": {Data: []byte("select 4;")},
		"postgres/000005_core_extension_bundles.up.sql": {Data: []byte("select 5;")},
		"postgres/not-a-migration.txt":                  {Data: []byte("ignore me")},
	})
	if err != nil {
		t.Fatalf("load embedded manifest: %v", err)
	}

	if len(migrations) != 5 {
		t.Fatalf("unexpected embedded migration count: got %d want %d", len(migrations), 5)
	}
	if migrations[0].Version != "000001" || migrations[4].Version != "000005" {
		t.Fatalf("unexpected migration order: %#v", migrations)
	}
}

func TestLoadPostgresMigrationsMatchesResetBaseline(t *testing.T) {
	t.Parallel()

	migrations, err := loadPostgresMigrations(migrationsfs.FS)
	if err != nil {
		t.Fatalf("load embedded migrations: %v", err)
	}

	got := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		got = append(got, migration.Path)
	}

	want := []string{
		"postgres/000001_core_infra.up.sql",
		"postgres/000002_core_platform.up.sql",
		"postgres/000003_core_auth.up.sql",
		"postgres/000004_core_service.up.sql",
		"postgres/000005_core_email.up.sql",
		"postgres/000006_core_forms.up.sql",
		"postgres/000007_core_automation.up.sql",
		"postgres/000008_core_knowledge_resources.up.sql",
		"postgres/000009_core_access_audit.up.sql",
		"postgres/000010_core_agents.up.sql",
		"postgres/000011_core_rls.up.sql",
		"postgres/000012_core_oauth.up.sql",
		"postgres/000013_core_extension_runtime.up.sql",
		"postgres/000014_core_concept_specs.up.sql",
		"postgres/000015_core_sandboxes.up.sql",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("unexpected embedded baseline:\n got: %#v\nwant: %#v", got, want)
	}
}
