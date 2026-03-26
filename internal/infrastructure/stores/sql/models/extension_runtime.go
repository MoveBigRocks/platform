package models

import (
	"database/sql"
	"time"
)

type ExtensionPackageRegistration struct {
	PackageKey             string         `db:"package_key"`
	SchemaName             string         `db:"schema_name"`
	RuntimeClass           string         `db:"runtime_class"`
	StorageClass           string         `db:"storage_class"`
	InstalledBundleVersion string         `db:"installed_bundle_version"`
	DesiredSchemaVersion   string         `db:"desired_schema_version"`
	CurrentSchemaVersion   string         `db:"current_schema_version"`
	Status                 string         `db:"status"`
	LastError              sql.NullString `db:"last_error"`
	CreatedAt              time.Time      `db:"created_at"`
	UpdatedAt              time.Time      `db:"updated_at"`
}

type ExtensionSchemaMigration struct {
	ID             string    `db:"id"`
	PackageKey     string    `db:"package_key"`
	SchemaName     string    `db:"schema_name"`
	Version        string    `db:"version"`
	ChecksumSHA256 string    `db:"checksum_sha256"`
	AppliedAt      time.Time `db:"applied_at"`
}
