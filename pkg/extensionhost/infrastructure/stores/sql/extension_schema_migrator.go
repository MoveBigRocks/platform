package sql

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/lib/pq"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

type ExtensionSchemaMigrator struct {
	db           *SqlxDB
	extensions   *ExtensionStore
	runtimeStore *ExtensionRuntimeStore
}

func NewExtensionSchemaMigrator(db *SqlxDB, extensions *ExtensionStore, runtimeStore *ExtensionRuntimeStore) *ExtensionSchemaMigrator {
	return &ExtensionSchemaMigrator{
		db:           db,
		extensions:   extensions,
		runtimeStore: runtimeStore,
	}
}

func (m *ExtensionSchemaMigrator) EnsureInstalledExtensionSchema(ctx context.Context, extension *platformdomain.InstalledExtension) error {
	if extension == nil {
		return fmt.Errorf("extension is required")
	}
	if m == nil || m.db == nil || m.extensions == nil || m.runtimeStore == nil {
		return fmt.Errorf("extension schema migrator is not configured")
	}
	if m.db.driver != "postgres" {
		return fmt.Errorf("extension schema migrator requires postgres")
	}
	if extension.Manifest.RuntimeClass != platformdomain.ExtensionRuntimeClassServiceBacked ||
		extension.Manifest.StorageClass != platformdomain.ExtensionStorageClassOwnedSchema {
		return nil
	}

	registration, err := platformdomain.NewExtensionPackageRegistration(extension.Manifest)
	if err != nil {
		return err
	}

	migrations, err := m.loadExtensionSchemaMigrations(ctx, extension)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		return fmt.Errorf("service-backed extension bundle contains no schema migrations")
	}
	if migrations[len(migrations)-1].Version != registration.DesiredSchemaVersion {
		return fmt.Errorf(
			"schema target version %s does not match highest bundled migration %s",
			registration.DesiredSchemaVersion,
			migrations[len(migrations)-1].Version,
		)
	}

	startingCurrentVersion := ""
	startingCreatedAt := registration.CreatedAt
	err = m.db.Transaction(ctx, func(txCtx context.Context) error {
		existing, getErr := m.runtimeStore.GetExtensionPackageRegistration(txCtx, registration.PackageKey)
		switch {
		case getErr == nil:
			registration.CreatedAt = existing.CreatedAt
			registration.CurrentSchemaVersion = existing.CurrentSchemaVersion
		case errors.Is(getErr, shared.ErrNotFound):
			// First install for this package key on this instance.
		default:
			return getErr
		}
		startingCurrentVersion = registration.CurrentSchemaVersion
		startingCreatedAt = registration.CreatedAt

		if err := m.acquirePackageLock(txCtx, registration.PackageKey); err != nil {
			return err
		}

		registration.Status = platformdomain.ExtensionSchemaRegistrationPending
		registration.LastError = ""
		if err := m.runtimeStore.UpsertExtensionPackageRegistration(txCtx, registration); err != nil {
			return err
		}

		if err := m.ensureSchemaExists(txCtx, registration.SchemaName); err != nil {
			return err
		}

		applied, err := m.readAppliedSchemaMigrations(txCtx, registration.PackageKey)
		if err != nil {
			return err
		}

		for _, migration := range migrations {
			if checksum, ok := applied[migration.Version]; ok {
				if checksum != migration.Checksum {
					if !isLegacyPlaceholderChecksum(checksum) {
						return fmt.Errorf("extension schema migration checksum drift for version %s", migration.Version)
					}
					// Backfill legacy placeholder checksums in-place: the
					// migration is considered already applied (the schema
					// objects exist from the original run) but the stored
					// value is a semantic placeholder ("init", "rls_skipped")
					// written by an older platform that predated real sha256
					// ledger entries. Replace with the real checksum so the
					// normal drift check protects future upgrades.
					if err := m.runtimeStore.BackfillExtensionSchemaMigrationChecksum(txCtx, registration.PackageKey, migration.Version, migration.Checksum); err != nil {
						return err
					}
				}
				registration.CurrentSchemaVersion = migration.Version
				continue
			}

			if err := m.applyExtensionSchemaMigration(txCtx, registration.SchemaName, migration); err != nil {
				return err
			}

			record, err := platformdomain.NewExtensionSchemaMigration(
				registration.PackageKey,
				registration.SchemaName,
				migration.Version,
				migration.Checksum,
			)
			if err != nil {
				return err
			}
			if err := m.runtimeStore.CreateExtensionSchemaMigration(txCtx, record); err != nil {
				return err
			}
			registration.CurrentSchemaVersion = migration.Version
		}

		registration.MarkSchemaReady(registration.CurrentSchemaVersion)
		registration.DesiredSchemaVersion = extension.Manifest.Schema.TargetVersion
		registration.InstalledBundleVersion = extension.Manifest.Version
		return m.runtimeStore.UpsertExtensionPackageRegistration(txCtx, registration)
	})
	if err != nil {
		registration.CreatedAt = startingCreatedAt
		registration.CurrentSchemaVersion = startingCurrentVersion
		registration.MarkSchemaFailed(err.Error())
		registration.DesiredSchemaVersion = extension.Manifest.Schema.TargetVersion
		registration.InstalledBundleVersion = extension.Manifest.Version
		_ = m.runtimeStore.UpsertExtensionPackageRegistration(ctx, registration)
		return err
	}

	return nil
}

func (m *ExtensionSchemaMigrator) loadExtensionSchemaMigrations(ctx context.Context, extension *platformdomain.InstalledExtension) ([]sqlMigration, error) {
	if extension == nil {
		return nil, fmt.Errorf("extension is required")
	}

	providedPayload := cloneBundlePayload(extension.BundlePayload)
	var (
		providedMigrations []sqlMigration
		providedErr        error
	)
	if len(providedPayload) > 0 {
		providedMigrations, providedErr = decodeExtensionSchemaMigrations(providedPayload)
		if providedErr != nil {
			providedErr = fmt.Errorf("decode provided extension bundle: %w", providedErr)
		}
	}

	return m.loadStoredExtensionSchemaMigrations(ctx, extension.ID, providedPayload, providedMigrations, providedErr)
}

func (m *ExtensionSchemaMigrator) loadStoredExtensionSchemaMigrations(ctx context.Context, extensionID string, providedPayload []byte, providedMigrations []sqlMigration, providedErr error) ([]sqlMigration, error) {
	bundlePayload, err := m.extensions.GetExtensionBundle(ctx, extensionID)
	if err != nil {
		if providedErr != nil {
			return nil, fmt.Errorf("%v; load stored extension bundle: %w", providedErr, err)
		}
		return nil, err
	}
	migrations, err := decodeExtensionSchemaMigrations(bundlePayload)
	if err != nil {
		if len(providedMigrations) > 0 {
			if err := m.extensions.UpdateExtensionBundle(ctx, extensionID, providedPayload); err != nil {
				return nil, fmt.Errorf("repair stored extension bundle: %w", err)
			}
			return providedMigrations, nil
		}
		if providedErr != nil {
			return nil, fmt.Errorf("%v; decode stored extension bundle: %w", providedErr, err)
		}
		return nil, fmt.Errorf("decode stored extension bundle: %w", err)
	}
	return migrations, nil
}

func (m *ExtensionSchemaMigrator) acquirePackageLock(ctx context.Context, packageKey string) error {
	_, err := m.db.Get(ctx).ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, packageKey)
	if err != nil {
		return fmt.Errorf("acquire extension schema advisory lock: %w", err)
	}
	return nil
}

func (m *ExtensionSchemaMigrator) ensureSchemaExists(ctx context.Context, schemaName string) error {
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pq.QuoteIdentifier(schemaName))
	if _, err := m.db.Get(ctx).ExecContext(ctx, query); err != nil {
		return fmt.Errorf("ensure schema %s: %w", schemaName, err)
	}
	return nil
}

// legacyPlaceholderChecksums enumerates the semantic labels that older
// platform releases stored in core_extension_runtime.schema_migration_history
// before the ledger switched to content-derived sha256 hashes. Rows that
// carry these values represent migrations that were successfully applied
// against the extension schema; the placeholder is not a real drift signal.
// The migrator upgrades these rows in-place on first contact with a newer
// platform so the normal drift check remains load-bearing going forward.
var legacyPlaceholderChecksums = map[string]struct{}{
	"init":        {},
	"rls_skipped": {},
}

func isLegacyPlaceholderChecksum(value string) bool {
	_, ok := legacyPlaceholderChecksums[strings.TrimSpace(value)]
	return ok
}

func (m *ExtensionSchemaMigrator) readAppliedSchemaMigrations(ctx context.Context, packageKey string) (map[string]string, error) {
	history, err := m.runtimeStore.ListExtensionSchemaMigrations(ctx, packageKey)
	if err != nil {
		return nil, err
	}

	applied := make(map[string]string, len(history))
	for _, record := range history {
		applied[record.Version] = record.ChecksumSHA256
	}
	return applied, nil
}

func (m *ExtensionSchemaMigrator) applyExtensionSchemaMigration(ctx context.Context, schemaName string, migration sqlMigration) error {
	if err := validateExtensionMigrationContainment(migration.SQL); err != nil {
		return fmt.Errorf("extension schema migration %s: %w", migration.Path, err)
	}
	sqlText := strings.ReplaceAll(migration.SQL, "${SCHEMA_NAME}", pq.QuoteIdentifier(schemaName))
	if _, err := m.db.Get(ctx).ExecContext(ctx, sqlText); err != nil {
		return fmt.Errorf("apply extension schema migration %s: %w", migration.Path, err)
	}
	return nil
}

// protectedSchemaTarget matches a write or DDL statement that targets a core,
// public, or catalog schema. Extension bundles own their ${SCHEMA_NAME} schema
// and may reference core tables for foreign keys or reads, but must never
// create, alter, drop, truncate, or write rows into a schema they do not own.
var protectedSchemaTarget = regexp.MustCompile(
	`(?:DROP|ALTER|TRUNCATE)\s+(?:TABLE|INDEX|VIEW|MATERIALIZED\s+VIEW|SEQUENCE|SCHEMA|TYPE|FUNCTION|TRIGGER|CONSTRAINT|POLICY)?\s*(?:IF\s+EXISTS\s+)?"?(?:CORE_[A-Z0-9_]+|PUBLIC|PG_[A-Z0-9_]+|INFORMATION_SCHEMA)"?\.` +
		`|(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM)\s+(?:ONLY\s+)?"?(?:CORE_[A-Z0-9_]+|PUBLIC|PG_[A-Z0-9_]+|INFORMATION_SCHEMA)"?\.`,
)

// forbiddenMigrationOps are operations no owned-schema extension migration
// legitimately needs, and which a malicious bundle could use to escalate
// privilege, disable tenant controls, or run host commands.
var forbiddenMigrationOps = []string{
	"ALTER SYSTEM",
	"CREATE ROLE", "DROP ROLE", "ALTER ROLE",
	"CREATE USER", "DROP USER", "ALTER USER",
	"SET ROLE", "RESET ROLE",
	"CREATE EXTENSION", "DROP EXTENSION",
	"SECURITY DEFINER",
	"FROM PROGRAM", "TO PROGRAM",
}

var grantOrRevoke = regexp.MustCompile(`(?:^|[^A-Z_])(GRANT|REVOKE)\s`)

// validateExtensionMigrationContainment rejects bundle migration SQL that would
// operate outside the extension's own schema in a dangerous way. It is a
// defense-in-depth check on top of bundle signing: it stops the obvious
// destructive and privilege-escalating statements (for example
// "DROP TABLE core_service.cases" or "ALTER TABLE core_platform.workspaces
// DISABLE ROW LEVEL SECURITY"), while still allowing foreign-key references to
// and reads from core tables. It does not replace per-extension database roles,
// which remain the complete isolation mechanism.
func validateExtensionMigrationContainment(sqlText string) error {
	normalized := strings.ToUpper(stripSQLNoise(sqlText))
	for _, op := range forbiddenMigrationOps {
		if strings.Contains(normalized, op) {
			return fmt.Errorf("contains a forbidden operation %q", op)
		}
	}
	if grantOrRevoke.MatchString(normalized) {
		return fmt.Errorf("changes privileges with GRANT or REVOKE")
	}
	if loc := protectedSchemaTarget.FindString(normalized); loc != "" {
		return fmt.Errorf("writes to a protected schema: %s", strings.TrimSpace(loc))
	}
	return nil
}

// stripSQLNoise removes comments and string, quoted-identifier, and
// dollar-quoted literals so that keyword scanning cannot be fooled by keywords
// that appear inside comments or string data.
func stripSQLNoise(sqlText string) string {
	var out strings.Builder
	runes := []rune(sqlText)
	for i := 0; i < len(runes); {
		switch {
		case runes[i] == '-' && i+1 < len(runes) && runes[i+1] == '-':
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
		case runes[i] == '/' && i+1 < len(runes) && runes[i+1] == '*':
			i += 2
			for i < len(runes) && !(runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '/') {
				i++
			}
			i += 2
			out.WriteByte(' ')
		case runes[i] == '\'':
			i++
			for i < len(runes) {
				if runes[i] == '\'' {
					if i+1 < len(runes) && runes[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			out.WriteByte(' ')
		case runes[i] == '$':
			if tag, closed := dollarTag(runes, i); tag != "" {
				end := indexRunes(runes, closed, tag)
				if end < 0 {
					return out.String()
				}
				i = end + len(tag)
				out.WriteByte(' ')
				continue
			}
			out.WriteRune(runes[i])
			i++
		default:
			out.WriteRune(runes[i])
			i++
		}
	}
	return out.String()
}

// dollarTag returns the opening dollar-quote tag starting at i (for example
// "$$" or "$body$") and the index just past it, or "" if i does not start one.
func dollarTag(runes []rune, i int) (string, int) {
	if runes[i] != '$' {
		return "", i
	}
	j := i + 1
	for j < len(runes) && (runes[j] == '_' || isIdentRune(runes[j])) {
		j++
	}
	if j < len(runes) && runes[j] == '$' {
		return string(runes[i : j+1]), j + 1
	}
	return "", i
}

func indexRunes(runes []rune, from int, tag string) int {
	tagRunes := []rune(tag)
	for i := from; i+len(tagRunes) <= len(runes); i++ {
		if string(runes[i:i+len(tagRunes)]) == tag {
			return i
		}
	}
	return -1
}

func isIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func decodeExtensionSchemaMigrations(bundlePayload []byte) ([]sqlMigration, error) {
	var bundle struct {
		Migrations []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"migrations"`
	}
	if err := json.Unmarshal(bundlePayload, &bundle); err != nil {
		return nil, fmt.Errorf("decode extension bundle payload: %w", err)
	}

	migrations := make([]sqlMigration, 0, len(bundle.Migrations))
	seenVersions := make(map[string]struct{}, len(bundle.Migrations))
	for _, migration := range bundle.Migrations {
		body := []byte(migration.Content)
		if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(migration.Content)); err == nil {
			body = decoded
		}
		version := migrationVersion(migration.Path)
		if version == "" {
			return nil, fmt.Errorf("extension migration %s is missing a numeric version prefix", migration.Path)
		}
		if _, exists := seenVersions[version]; exists {
			return nil, fmt.Errorf("duplicate extension migration version %s", version)
		}
		seenVersions[version] = struct{}{}
		migrations = append(migrations, sqlMigration{
			Version:  version,
			Path:     migration.Path,
			SQL:      string(body),
			Checksum: checksumSQL(body),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}
