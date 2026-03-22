package platformdomain

import (
	"fmt"
	"strings"
	"time"
)

type ExtensionSchemaRegistrationStatus string

const (
	ExtensionSchemaRegistrationPending ExtensionSchemaRegistrationStatus = "pending"
	ExtensionSchemaRegistrationReady   ExtensionSchemaRegistrationStatus = "ready"
	ExtensionSchemaRegistrationFailed  ExtensionSchemaRegistrationStatus = "failed"
)

type ExtensionPackageRegistration struct {
	PackageKey             string
	SchemaName             string
	RuntimeClass           ExtensionRuntimeClass
	StorageClass           ExtensionStorageClass
	InstalledBundleVersion string
	DesiredSchemaVersion   string
	CurrentSchemaVersion   string
	Status                 ExtensionSchemaRegistrationStatus
	LastError              string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func NewExtensionPackageRegistration(manifest ExtensionManifest) (*ExtensionPackageRegistration, error) {
	manifest.Normalize()
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	if manifest.RuntimeClass != ExtensionRuntimeClassServiceBacked {
		return nil, fmt.Errorf("only service-backed extensions can register owned schemas")
	}
	if manifest.StorageClass != ExtensionStorageClassOwnedSchema {
		return nil, fmt.Errorf("only owned_schema extensions can register owned schemas")
	}

	now := time.Now()
	return &ExtensionPackageRegistration{
		PackageKey:             manifest.PackageKey(),
		SchemaName:             manifest.Schema.Name,
		RuntimeClass:           manifest.RuntimeClass,
		StorageClass:           manifest.StorageClass,
		InstalledBundleVersion: manifest.Version,
		DesiredSchemaVersion:   manifest.Schema.TargetVersion,
		Status:                 ExtensionSchemaRegistrationPending,
		CreatedAt:              now,
		UpdatedAt:              now,
	}, nil
}

func (r *ExtensionPackageRegistration) Normalize() {
	r.PackageKey = normalizeExtensionPackageKey(r.PackageKey)
	r.SchemaName = normalizeExtensionSchemaName(r.SchemaName)
	r.RuntimeClass = normalizeRuntimeClass(r.RuntimeClass)
	r.StorageClass = normalizeStorageClass(r.StorageClass, r.RuntimeClass)
	r.InstalledBundleVersion = strings.TrimSpace(r.InstalledBundleVersion)
	r.DesiredSchemaVersion = strings.TrimSpace(r.DesiredSchemaVersion)
	r.CurrentSchemaVersion = strings.TrimSpace(r.CurrentSchemaVersion)
	r.Status = normalizeExtensionSchemaRegistrationStatus(r.Status)
	r.LastError = strings.TrimSpace(r.LastError)
}

func (r *ExtensionPackageRegistration) Validate() error {
	r.Normalize()

	var problems []string
	if r.PackageKey == "" {
		problems = append(problems, "package_key is required")
	}
	if r.SchemaName == "" {
		problems = append(problems, "schema_name is required")
	}
	if r.RuntimeClass != ExtensionRuntimeClassServiceBacked {
		problems = append(problems, "runtime_class must be service_backed")
	}
	if r.StorageClass != ExtensionStorageClassOwnedSchema {
		problems = append(problems, "storage_class must be owned_schema")
	}
	if r.InstalledBundleVersion == "" {
		problems = append(problems, "installed_bundle_version is required")
	}
	if r.DesiredSchemaVersion == "" {
		problems = append(problems, "desired_schema_version is required")
	}
	if r.Status == "" {
		problems = append(problems, "status is required")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, ", "))
	}
	return nil
}

func (r *ExtensionPackageRegistration) MarkSchemaReady(currentVersion string) {
	r.CurrentSchemaVersion = strings.TrimSpace(currentVersion)
	r.Status = ExtensionSchemaRegistrationReady
	r.LastError = ""
	r.UpdatedAt = time.Now()
}

func (r *ExtensionPackageRegistration) MarkSchemaFailed(message string) {
	r.Status = ExtensionSchemaRegistrationFailed
	r.LastError = strings.TrimSpace(message)
	r.UpdatedAt = time.Now()
}

type ExtensionSchemaMigration struct {
	ID             string
	PackageKey     string
	SchemaName     string
	Version        string
	ChecksumSHA256 string
	AppliedAt      time.Time
}

func NewExtensionSchemaMigration(packageKey, schemaName, version, checksumSHA256 string) (*ExtensionSchemaMigration, error) {
	migration := &ExtensionSchemaMigration{
		PackageKey:     packageKey,
		SchemaName:     schemaName,
		Version:        version,
		ChecksumSHA256: checksumSHA256,
		AppliedAt:      time.Now(),
	}
	if err := migration.Validate(); err != nil {
		return nil, err
	}
	return migration, nil
}

func (m *ExtensionSchemaMigration) Normalize() {
	m.PackageKey = normalizeExtensionPackageKey(m.PackageKey)
	m.SchemaName = normalizeExtensionSchemaName(m.SchemaName)
	m.Version = strings.TrimSpace(m.Version)
	m.ChecksumSHA256 = strings.TrimSpace(strings.ToLower(m.ChecksumSHA256))
}

func (m *ExtensionSchemaMigration) Validate() error {
	m.Normalize()

	var problems []string
	if m.PackageKey == "" {
		problems = append(problems, "package_key is required")
	}
	if m.SchemaName == "" {
		problems = append(problems, "schema_name is required")
	}
	if m.Version == "" {
		problems = append(problems, "version is required")
	}
	if m.ChecksumSHA256 == "" {
		problems = append(problems, "checksum_sha256 is required")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, ", "))
	}
	return nil
}

func normalizeExtensionSchemaRegistrationStatus(value ExtensionSchemaRegistrationStatus) ExtensionSchemaRegistrationStatus {
	switch ExtensionSchemaRegistrationStatus(strings.TrimSpace(strings.ToLower(string(value)))) {
	case ExtensionSchemaRegistrationPending, ExtensionSchemaRegistrationReady, ExtensionSchemaRegistrationFailed:
		return ExtensionSchemaRegistrationStatus(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}
