package models

import "time"

type InstalledExtension struct {
	ID                string     `db:"id"`
	WorkspaceID       *string    `db:"workspace_id"`
	Slug              string     `db:"slug"`
	Name              string     `db:"name"`
	Publisher         string     `db:"publisher"`
	Version           string     `db:"version"`
	Description       string     `db:"description"`
	LicenseToken      string     `db:"license_token"`
	BundleSHA256      string     `db:"bundle_sha256"`
	BundleSize        int64      `db:"bundle_size"`
	BundlePayload     []byte     `db:"bundle_payload"`
	ManifestJSON      string     `db:"manifest_json"`
	ConfigJSON        string     `db:"config_json"`
	Status            string     `db:"status"`
	ValidationStatus  string     `db:"validation_status"`
	ValidationMessage string     `db:"validation_message"`
	HealthStatus      string     `db:"health_status"`
	HealthMessage     string     `db:"health_message"`
	InstalledByID     *string    `db:"installed_by_id"`
	InstalledAt       time.Time  `db:"installed_at"`
	ActivatedAt       *time.Time `db:"activated_at"`
	DeactivatedAt     *time.Time `db:"deactivated_at"`
	ValidatedAt       *time.Time `db:"validated_at"`
	LastHealthCheckAt *time.Time `db:"last_health_check_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
	DeletedAt         *time.Time `db:"deleted_at"`
}

func (InstalledExtension) TableName() string {
	return "installed_extensions"
}

type ExtensionAsset struct {
	ID             string     `db:"id"`
	ExtensionID    string     `db:"extension_id"`
	Path           string     `db:"path"`
	Kind           string     `db:"kind"`
	ContentType    string     `db:"content_type"`
	Content        []byte     `db:"content"`
	IsCustomizable bool       `db:"is_customizable"`
	Checksum       string     `db:"checksum"`
	Size           int64      `db:"size"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

func (ExtensionAsset) TableName() string {
	return "extension_assets"
}
