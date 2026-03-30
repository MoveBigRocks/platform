package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

type ExtensionRuntimeStore struct {
	db *SqlxDB
}

const (
	extensionPackageRegistrationsTable   = "core_extension_runtime.extension_package_registrations"
	extensionSchemaMigrationHistoryTable = "core_extension_runtime.schema_migration_history"
)

func NewExtensionRuntimeStore(db *SqlxDB) *ExtensionRuntimeStore {
	return &ExtensionRuntimeStore{db: db}
}

func (s *ExtensionRuntimeStore) UpsertExtensionPackageRegistration(ctx context.Context, registration *platformdomain.ExtensionPackageRegistration) error {
	model, err := s.mapRegistrationToModel(registration)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`INSERT INTO %s (
		package_key, schema_name, runtime_class, storage_class, installed_bundle_version,
		desired_schema_version, current_schema_version, status, last_error, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT (package_key) DO UPDATE SET
		schema_name = excluded.schema_name,
		runtime_class = excluded.runtime_class,
		storage_class = excluded.storage_class,
		installed_bundle_version = excluded.installed_bundle_version,
		desired_schema_version = excluded.desired_schema_version,
		current_schema_version = excluded.current_schema_version,
		status = excluded.status,
		last_error = excluded.last_error,
		updated_at = excluded.updated_at`, extensionPackageRegistrationsTable)

	_, err = s.db.Get(ctx).ExecContext(
		ctx,
		query,
		model.PackageKey,
		model.SchemaName,
		model.RuntimeClass,
		model.StorageClass,
		model.InstalledBundleVersion,
		model.DesiredSchemaVersion,
		model.CurrentSchemaVersion,
		model.Status,
		model.LastError,
		model.CreatedAt,
		model.UpdatedAt,
	)
	return TranslateSqlxError(err, "extension_package_registrations")
}

func (s *ExtensionRuntimeStore) GetExtensionPackageRegistration(ctx context.Context, packageKey string) (*platformdomain.ExtensionPackageRegistration, error) {
	var model models.ExtensionPackageRegistration
	query := fmt.Sprintf(`SELECT * FROM %s WHERE package_key = ?`, extensionPackageRegistrationsTable)
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, packageKey); err != nil {
		return nil, TranslateSqlxError(err, "extension_package_registrations")
	}
	return s.mapModelToRegistration(&model), nil
}

func (s *ExtensionRuntimeStore) ListExtensionPackageRegistrations(ctx context.Context) ([]*platformdomain.ExtensionPackageRegistration, error) {
	var rows []models.ExtensionPackageRegistration
	query := fmt.Sprintf(`SELECT * FROM %s ORDER BY package_key ASC`, extensionPackageRegistrationsTable)
	if err := s.db.Get(ctx).SelectContext(ctx, &rows, query); err != nil {
		return nil, TranslateSqlxError(err, "extension_package_registrations")
	}

	result := make([]*platformdomain.ExtensionPackageRegistration, len(rows))
	for i := range rows {
		result[i] = s.mapModelToRegistration(&rows[i])
	}
	return result, nil
}

func (s *ExtensionRuntimeStore) CreateExtensionSchemaMigration(ctx context.Context, migration *platformdomain.ExtensionSchemaMigration) error {
	model, err := s.mapMigrationToModel(migration)
	if err != nil {
		return err
	}
	normalizePersistedUUID(&model.ID)

	query := fmt.Sprintf(`INSERT INTO %s (
		id, package_key, schema_name, version, checksum_sha256, applied_at
	) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?)
	RETURNING id`, extensionSchemaMigrationHistoryTable)

	err = s.db.Get(ctx).QueryRowxContext(
		ctx,
		query,
		model.ID,
		model.PackageKey,
		model.SchemaName,
		model.Version,
		model.ChecksumSHA256,
		model.AppliedAt,
	).Scan(&model.ID)
	migration.ID = model.ID
	return TranslateSqlxError(err, "schema_migration_history")
}

func (s *ExtensionRuntimeStore) ListExtensionSchemaMigrations(ctx context.Context, packageKey string) ([]*platformdomain.ExtensionSchemaMigration, error) {
	var rows []models.ExtensionSchemaMigration
	query := fmt.Sprintf(`SELECT * FROM %s WHERE package_key = ? ORDER BY applied_at ASC, version ASC`, extensionSchemaMigrationHistoryTable)
	if err := s.db.Get(ctx).SelectContext(ctx, &rows, query, packageKey); err != nil {
		return nil, TranslateSqlxError(err, "schema_migration_history")
	}

	result := make([]*platformdomain.ExtensionSchemaMigration, len(rows))
	for i := range rows {
		result[i] = s.mapModelToMigration(&rows[i])
	}
	return result, nil
}

func (s *ExtensionRuntimeStore) mapRegistrationToModel(registration *platformdomain.ExtensionPackageRegistration) (*models.ExtensionPackageRegistration, error) {
	if registration == nil {
		return nil, fmt.Errorf("registration is required")
	}
	if err := registration.Validate(); err != nil {
		return nil, err
	}
	return &models.ExtensionPackageRegistration{
		PackageKey:             registration.PackageKey,
		SchemaName:             registration.SchemaName,
		RuntimeClass:           string(registration.RuntimeClass),
		StorageClass:           string(registration.StorageClass),
		InstalledBundleVersion: registration.InstalledBundleVersion,
		DesiredSchemaVersion:   registration.DesiredSchemaVersion,
		CurrentSchemaVersion:   registration.CurrentSchemaVersion,
		Status:                 string(registration.Status),
		LastError:              nullString(registration.LastError),
		CreatedAt:              registration.CreatedAt,
		UpdatedAt:              registration.UpdatedAt,
	}, nil
}

func (s *ExtensionRuntimeStore) mapMigrationToModel(migration *platformdomain.ExtensionSchemaMigration) (*models.ExtensionSchemaMigration, error) {
	if migration == nil {
		return nil, fmt.Errorf("migration is required")
	}
	if err := migration.Validate(); err != nil {
		return nil, err
	}
	return &models.ExtensionSchemaMigration{
		ID:             migration.ID,
		PackageKey:     migration.PackageKey,
		SchemaName:     migration.SchemaName,
		Version:        migration.Version,
		ChecksumSHA256: migration.ChecksumSHA256,
		AppliedAt:      migration.AppliedAt,
	}, nil
}

func (s *ExtensionRuntimeStore) mapModelToRegistration(model *models.ExtensionPackageRegistration) *platformdomain.ExtensionPackageRegistration {
	return &platformdomain.ExtensionPackageRegistration{
		PackageKey:             model.PackageKey,
		SchemaName:             model.SchemaName,
		RuntimeClass:           platformdomain.ExtensionRuntimeClass(model.RuntimeClass),
		StorageClass:           platformdomain.ExtensionStorageClass(model.StorageClass),
		InstalledBundleVersion: model.InstalledBundleVersion,
		DesiredSchemaVersion:   model.DesiredSchemaVersion,
		CurrentSchemaVersion:   model.CurrentSchemaVersion,
		Status:                 platformdomain.ExtensionSchemaRegistrationStatus(model.Status),
		LastError:              strings.TrimSpace(model.LastError.String),
		CreatedAt:              model.CreatedAt,
		UpdatedAt:              model.UpdatedAt,
	}
}

func (s *ExtensionRuntimeStore) mapModelToMigration(model *models.ExtensionSchemaMigration) *platformdomain.ExtensionSchemaMigration {
	return &platformdomain.ExtensionSchemaMigration{
		ID:             model.ID,
		PackageKey:     model.PackageKey,
		SchemaName:     model.SchemaName,
		Version:        model.Version,
		ChecksumSHA256: model.ChecksumSHA256,
		AppliedAt:      model.AppliedAt,
	}
}

func nullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
