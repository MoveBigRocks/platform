package sql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/movebigrocks/platform/migrations"
)

type sqlMigration struct {
	Version  string
	Path     string
	SQL      string
	Checksum string
}

func runPostgresMigrations(db *sql.DB) error {
	migrationFiles, err := loadPostgresMigrations(migrations.FS)
	if err != nil {
		return err
	}

	if err := ensureCoreMigrationLedger(db); err != nil {
		return err
	}

	applied, dirtyVersion, err := readAppliedCoreMigrations(db)
	if err != nil {
		return err
	}
	if dirtyVersion != "" {
		return fmt.Errorf("postgres migrations are dirty at version %s", dirtyVersion)
	}

	for _, migration := range migrationFiles {
		if checksum, ok := applied[migration.Version]; ok {
			if checksum != migration.Checksum {
				return fmt.Errorf("postgres migration checksum drift for version %s", migration.Version)
			}
			continue
		}

		if err := applyPostgresMigration(db, migration); err != nil {
			return err
		}
		log.Printf("Applied postgres migration %s (%s)", migration.Version, migration.Path)
	}

	return nil
}

func loadPostgresMigrations(fsys fs.FS) ([]sqlMigration, error) {
	entries, err := fs.ReadDir(fsys, "postgres")
	if err != nil {
		return nil, fmt.Errorf("read postgres migrations directory: %w", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		paths = append(paths, filepath.ToSlash(filepath.Join("postgres", entry.Name())))
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no postgres migrations found")
	}

	return loadMigrationsFromPaths(fsys, paths)
}

func loadMigrationsFromPaths(fsys fs.FS, paths []string) ([]sqlMigration, error) {
	migrationFiles := make([]sqlMigration, 0, len(paths))
	for _, path := range paths {
		sqlBytes, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", path, err)
		}

		migrationFiles = append(migrationFiles, sqlMigration{
			Version:  migrationVersion(path),
			Path:     path,
			SQL:      string(sqlBytes),
			Checksum: checksumSQL(sqlBytes),
		})
	}

	slices.SortFunc(migrationFiles, func(a, b sqlMigration) int {
		return strings.Compare(a.Version, b.Version)
	})

	return migrationFiles, nil
}

func migrationVersion(path string) string {
	base := filepath.Base(path)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) == 0 {
		return base
	}
	return parts[0]
}

func checksumSQL(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func ensureCoreMigrationLedger(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS public.schema_migrations (
			version VARCHAR(32) PRIMARY KEY,
			checksum_sha256 VARCHAR(64) NOT NULL,
			dirty BOOLEAN NOT NULL DEFAULT FALSE,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("ensure core migration ledger: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE public.schema_migrations ADD COLUMN IF NOT EXISTS dirty BOOLEAN NOT NULL DEFAULT FALSE`); err != nil {
		return fmt.Errorf("ensure core migration dirty flag: %w", err)
	}
	return nil
}

func readAppliedCoreMigrations(db *sql.DB) (map[string]string, string, error) {
	rows, err := db.Query(`SELECT version, checksum_sha256, dirty FROM public.schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, "", fmt.Errorf("list applied core migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]string)
	dirtyVersion := ""
	for rows.Next() {
		var version string
		var checksum string
		var dirty bool
		if err := rows.Scan(&version, &checksum, &dirty); err != nil {
			return nil, "", fmt.Errorf("scan applied core migration: %w", err)
		}
		if dirty {
			dirtyVersion = version
			continue
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate applied core migrations: %w", err)
	}
	return applied, dirtyVersion, nil
}

func applyPostgresMigration(db *sql.DB, migration sqlMigration) error {
	if _, err := db.Exec(
		`INSERT INTO public.schema_migrations (version, checksum_sha256, dirty) VALUES ($1, $2, TRUE)
		 ON CONFLICT (version) DO UPDATE SET checksum_sha256 = EXCLUDED.checksum_sha256, dirty = EXCLUDED.dirty`,
		migration.Version,
		migration.Checksum,
	); err != nil {
		return fmt.Errorf("mark migration %s dirty: %w", migration.Path, err)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", migration.Path, err)
	}

	if _, err := tx.Exec(migration.SQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration %s: %w", migration.Path, err)
	}
	if _, err := tx.Exec(
		`UPDATE public.schema_migrations
		 SET checksum_sha256 = $2, dirty = FALSE, applied_at = NOW()
		 WHERE version = $1`,
		migration.Version,
		migration.Checksum,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %s: %w", migration.Path, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", migration.Path, err)
	}
	return nil
}
