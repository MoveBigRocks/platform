package sql

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

	bundlePayload, err := m.extensions.GetExtensionBundle(ctx, extension.ID)
	if err != nil {
		return err
	}
	migrations, err := decodeExtensionSchemaMigrations(bundlePayload)
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
					return fmt.Errorf("extension schema migration checksum drift for version %s", migration.Version)
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
	sqlText := strings.ReplaceAll(migration.SQL, "${SCHEMA_NAME}", pq.QuoteIdentifier(schemaName))
	if _, err := m.db.Get(ctx).ExecContext(ctx, sqlText); err != nil {
		return fmt.Errorf("apply extension schema migration %s: %w", migration.Path, err)
	}
	return nil
}

func decodeExtensionSchemaMigrations(bundlePayload []byte) ([]sqlMigration, error) {
	var bundle struct {
		Migrations []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"migrations"`
	}
	if err := json.Unmarshal(bundlePayload, &bundle); err != nil {
		return nil, fmt.Errorf("decode stored extension bundle: %w", err)
	}

	migrations := make([]sqlMigration, 0, len(bundle.Migrations))
	seenVersions := make(map[string]struct{}, len(bundle.Migrations))
	for _, migration := range bundle.Migrations {
		body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(migration.Content))
		if err != nil {
			return nil, fmt.Errorf("decode extension migration %s body: %w", migration.Path, err)
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
