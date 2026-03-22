CREATE SCHEMA IF NOT EXISTS core_extension_runtime;

CREATE TABLE IF NOT EXISTS core_extension_runtime.extension_package_registrations (
    package_key TEXT PRIMARY KEY,
    schema_name TEXT NOT NULL UNIQUE,
    runtime_class TEXT NOT NULL,
    storage_class TEXT NOT NULL,
    installed_bundle_version TEXT NOT NULL,
    desired_schema_version TEXT NOT NULL,
    current_schema_version TEXT,
    status TEXT NOT NULL,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_extension_package_registrations_status
    ON core_extension_runtime.extension_package_registrations(status);

CREATE TABLE IF NOT EXISTS core_extension_runtime.schema_migration_history (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    package_key TEXT NOT NULL REFERENCES core_extension_runtime.extension_package_registrations(package_key) ON DELETE CASCADE,
    schema_name TEXT NOT NULL,
    version TEXT NOT NULL,
    checksum_sha256 VARCHAR(64) NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(package_key, version)
);

CREATE INDEX IF NOT EXISTS idx_schema_migration_history_package
    ON core_extension_runtime.schema_migration_history(package_key, applied_at);
