package sql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

type ExtensionStore struct {
	db *SqlxDB
}

const installedExtensionSelectColumns = `
	id, workspace_id, slug, name, publisher, version, description, license_token,
	bundle_sha256, bundle_size, manifest_json, config_json, status,
	validation_status, validation_message, health_status, health_message,
	installed_by_id, installed_at, activated_at, deactivated_at,
	validated_at, last_health_check_at, updated_at, deleted_at`

func NewExtensionStore(db *SqlxDB) *ExtensionStore {
	return &ExtensionStore{db: db}
}

func (s *ExtensionStore) CreateInstalledExtension(ctx context.Context, extension *platformdomain.InstalledExtension) error {
	model, err := s.mapInstallationToModel(extension)
	if err != nil {
		return fmt.Errorf("map installed extension: %w", err)
	}
	normalizePersistedUUID(&model.ID)

	query := `INSERT INTO core_platform.installed_extensions (
		id, workspace_id, slug, name, publisher, version, description, license_token,
		bundle_sha256, bundle_size, bundle_payload, manifest_json, config_json, status,
		validation_status, validation_message, health_status, health_message,
		installed_by_id, installed_at, activated_at, deactivated_at,
		validated_at, last_health_check_at, updated_at
	) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(
		ctx,
		query,
		model.ID,
		model.WorkspaceID,
		model.Slug,
		model.Name,
		model.Publisher,
		model.Version,
		model.Description,
		model.LicenseToken,
		model.BundleSHA256,
		model.BundleSize,
		model.BundlePayload,
		model.ManifestJSON,
		model.ConfigJSON,
		model.Status,
		model.ValidationStatus,
		model.ValidationMessage,
		model.HealthStatus,
		model.HealthMessage,
		nullableLegacyUUIDPtrValue(model.InstalledByID),
		model.InstalledAt,
		model.ActivatedAt,
		model.DeactivatedAt,
		model.ValidatedAt,
		model.LastHealthCheckAt,
		model.UpdatedAt,
	).Scan(&model.ID)
	extension.ID = model.ID
	return TranslateSqlxError(err, "installed_extensions")
}

func (s *ExtensionStore) GetInstalledExtension(ctx context.Context, extensionID string) (*platformdomain.InstalledExtension, error) {
	var model models.InstalledExtension
	query := `SELECT ` + installedExtensionSelectColumns + ` FROM core_platform.installed_extensions WHERE id = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, extensionID); err != nil {
		return nil, TranslateSqlxError(err, "installed_extensions")
	}
	return s.mapModelToInstallation(&model), nil
}

func (s *ExtensionStore) GetInstalledExtensionBySlug(ctx context.Context, workspaceID, slug string) (*platformdomain.InstalledExtension, error) {
	var model models.InstalledExtension
	query := `SELECT ` + installedExtensionSelectColumns + ` FROM core_platform.installed_extensions WHERE workspace_id = ? AND slug = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, workspaceID, slug); err != nil {
		return nil, TranslateSqlxError(err, "installed_extensions")
	}
	return s.mapModelToInstallation(&model), nil
}

func (s *ExtensionStore) GetInstanceExtensionBySlug(ctx context.Context, slug string) (*platformdomain.InstalledExtension, error) {
	var model models.InstalledExtension
	query := `SELECT ` + installedExtensionSelectColumns + ` FROM core_platform.installed_extensions WHERE workspace_id IS NULL AND slug = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, slug); err != nil {
		return nil, TranslateSqlxError(err, "installed_extensions")
	}
	return s.mapModelToInstallation(&model), nil
}

func (s *ExtensionStore) ListWorkspaceExtensions(ctx context.Context, workspaceID string) ([]*platformdomain.InstalledExtension, error) {
	var rows []models.InstalledExtension
	query := `SELECT ` + installedExtensionSelectColumns + ` FROM core_platform.installed_extensions WHERE workspace_id = ? AND deleted_at IS NULL ORDER BY installed_at DESC`
	if err := s.db.Get(ctx).SelectContext(ctx, &rows, query, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "installed_extensions")
	}

	result := make([]*platformdomain.InstalledExtension, len(rows))
	for i := range rows {
		result[i] = s.mapModelToInstallation(&rows[i])
	}
	return result, nil
}

func (s *ExtensionStore) ListInstanceExtensions(ctx context.Context) ([]*platformdomain.InstalledExtension, error) {
	var rows []models.InstalledExtension
	query := `SELECT ` + installedExtensionSelectColumns + ` FROM core_platform.installed_extensions WHERE workspace_id IS NULL AND deleted_at IS NULL ORDER BY installed_at DESC`
	if err := s.db.Get(ctx).SelectContext(ctx, &rows, query); err != nil {
		return nil, TranslateSqlxError(err, "installed_extensions")
	}

	result := make([]*platformdomain.InstalledExtension, len(rows))
	for i := range rows {
		result[i] = s.mapModelToInstallation(&rows[i])
	}
	return result, nil
}

func (s *ExtensionStore) ListAllExtensions(ctx context.Context) ([]*platformdomain.InstalledExtension, error) {
	var rows []models.InstalledExtension
	query := `SELECT ` + installedExtensionSelectColumns + ` FROM core_platform.installed_extensions WHERE deleted_at IS NULL ORDER BY installed_at DESC`
	if err := s.db.Get(ctx).SelectContext(ctx, &rows, query); err != nil {
		return nil, TranslateSqlxError(err, "installed_extensions")
	}

	result := make([]*platformdomain.InstalledExtension, len(rows))
	for i := range rows {
		result[i] = s.mapModelToInstallation(&rows[i])
	}
	return result, nil
}

func (s *ExtensionStore) UpdateInstalledExtension(ctx context.Context, extension *platformdomain.InstalledExtension) error {
	model, err := s.mapInstallationToModel(extension)
	if err != nil {
		return fmt.Errorf("map installed extension: %w", err)
	}

	query := `UPDATE core_platform.installed_extensions SET
		slug = ?, name = ?, publisher = ?, version = ?, description = ?,
		license_token = ?, bundle_sha256 = ?, bundle_size = ?, bundle_payload = ?, manifest_json = ?, config_json = ?,
		status = ?, validation_status = ?, validation_message = ?, health_status = ?, health_message = ?,
		installed_by_id = ?, installed_at = ?, activated_at = ?, deactivated_at = ?,
		validated_at = ?, last_health_check_at = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`

	result, err := s.db.Get(ctx).ExecContext(
		ctx,
		query,
		model.Slug,
		model.Name,
		model.Publisher,
		model.Version,
		model.Description,
		model.LicenseToken,
		model.BundleSHA256,
		model.BundleSize,
		model.BundlePayload,
		model.ManifestJSON,
		model.ConfigJSON,
		model.Status,
		model.ValidationStatus,
		model.ValidationMessage,
		model.HealthStatus,
		model.HealthMessage,
		nullableLegacyUUIDPtrValue(model.InstalledByID),
		model.InstalledAt,
		model.ActivatedAt,
		model.DeactivatedAt,
		model.ValidatedAt,
		model.LastHealthCheckAt,
		time.Now(),
		model.ID,
	)
	if err != nil {
		return TranslateSqlxError(err, "installed_extensions")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *ExtensionStore) GetExtensionBundle(ctx context.Context, extensionID string) ([]byte, error) {
	var payload []byte
	query := `SELECT bundle_payload FROM core_platform.installed_extensions WHERE id = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &payload, query, extensionID); err != nil {
		return nil, TranslateSqlxError(err, "installed_extensions")
	}
	return payload, nil
}

func (s *ExtensionStore) DeleteInstalledExtension(ctx context.Context, extensionID string) error {
	now := time.Now()
	query := `UPDATE core_platform.installed_extensions SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, now, now, extensionID)
	if err != nil {
		return TranslateSqlxError(err, "installed_extensions")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	_, _ = s.db.Get(ctx).ExecContext(ctx, `UPDATE core_platform.extension_assets SET deleted_at = ?, updated_at = ? WHERE extension_id = ? AND deleted_at IS NULL`, now, now, extensionID)
	return nil
}

func (s *ExtensionStore) ReplaceExtensionAssets(ctx context.Context, extensionID string, assets []*platformdomain.ExtensionAsset) error {
	if _, err := s.db.Get(ctx).ExecContext(ctx, `DELETE FROM core_platform.extension_assets WHERE extension_id = ?`, extensionID); err != nil {
		return TranslateSqlxError(err, "extension_assets")
	}
	for _, asset := range assets {
		model := s.mapAssetToModel(asset)
		normalizePersistedUUID(&model.ID)
		query := `INSERT INTO core_platform.extension_assets (
			id, extension_id, path, kind, content_type, content, is_customizable, checksum, size, created_at, updated_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`
		if err := s.db.Get(ctx).QueryRowxContext(
			ctx,
			query,
			model.ID,
			model.ExtensionID,
			model.Path,
			model.Kind,
			model.ContentType,
			model.Content,
			model.IsCustomizable,
			model.Checksum,
			model.Size,
			model.CreatedAt,
			model.UpdatedAt,
		).Scan(&model.ID); err != nil {
			return TranslateSqlxError(err, "extension_assets")
		}
		asset.ID = model.ID
	}
	return nil
}

func (s *ExtensionStore) ListExtensionAssets(ctx context.Context, extensionID string) ([]*platformdomain.ExtensionAsset, error) {
	var rows []models.ExtensionAsset
	query := `SELECT * FROM core_platform.extension_assets WHERE extension_id = ? AND deleted_at IS NULL ORDER BY path ASC`
	if err := s.db.Get(ctx).SelectContext(ctx, &rows, query, extensionID); err != nil {
		return nil, TranslateSqlxError(err, "extension_assets")
	}

	result := make([]*platformdomain.ExtensionAsset, len(rows))
	for i := range rows {
		result[i] = s.mapModelToAsset(&rows[i])
	}
	return result, nil
}

func (s *ExtensionStore) GetExtensionAsset(ctx context.Context, extensionID, assetPath string) (*platformdomain.ExtensionAsset, error) {
	var row models.ExtensionAsset
	query := `SELECT * FROM core_platform.extension_assets WHERE extension_id = ? AND path = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &row, query, extensionID, assetPath); err != nil {
		return nil, TranslateSqlxError(err, "extension_assets")
	}
	return s.mapModelToAsset(&row), nil
}

func (s *ExtensionStore) UpdateExtensionAsset(ctx context.Context, asset *platformdomain.ExtensionAsset) error {
	model := s.mapAssetToModel(asset)
	query := `UPDATE core_platform.extension_assets SET
		path = ?, kind = ?, content_type = ?, content = ?, is_customizable = ?,
		checksum = ?, size = ?, updated_at = ?
		WHERE id = ? AND extension_id = ? AND deleted_at IS NULL`
	result, err := s.db.Get(ctx).ExecContext(
		ctx,
		query,
		model.Path,
		model.Kind,
		model.ContentType,
		model.Content,
		model.IsCustomizable,
		model.Checksum,
		model.Size,
		time.Now(),
		model.ID,
		model.ExtensionID,
	)
	if err != nil {
		return TranslateSqlxError(err, "extension_assets")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *ExtensionStore) mapInstallationToModel(extension *platformdomain.InstalledExtension) (*models.InstalledExtension, error) {
	manifestJSON, err := marshalJSONString(extension.Manifest, "manifest_json")
	if err != nil {
		return nil, err
	}
	configJSON, err := marshalJSONString(extension.Config, "config_json")
	if err != nil {
		return nil, err
	}

	return &models.InstalledExtension{
		ID:                extension.ID,
		WorkspaceID:       nullableStringPtr(extension.WorkspaceID),
		Slug:              extension.Slug,
		Name:              extension.Name,
		Publisher:         extension.Publisher,
		Version:           extension.Version,
		Description:       extension.Description,
		LicenseToken:      extension.LicenseToken,
		BundleSHA256:      extension.BundleSHA256,
		BundleSize:        extension.BundleSize,
		BundlePayload:     cloneBundlePayload(extension.BundlePayload),
		ManifestJSON:      manifestJSON,
		ConfigJSON:        configJSON,
		Status:            string(extension.Status),
		ValidationStatus:  string(extension.ValidationStatus),
		ValidationMessage: extension.ValidationMessage,
		HealthStatus:      string(extension.HealthStatus),
		HealthMessage:     extension.HealthMessage,
		InstalledByID:     nullableStringPtr(extension.InstalledByID),
		InstalledAt:       extension.InstalledAt,
		ActivatedAt:       extension.ActivatedAt,
		DeactivatedAt:     extension.DeactivatedAt,
		ValidatedAt:       extension.ValidatedAt,
		LastHealthCheckAt: extension.LastHealthCheckAt,
		UpdatedAt:         extension.UpdatedAt,
		DeletedAt:         extension.DeletedAt,
	}, nil
}

func (s *ExtensionStore) mapModelToInstallation(model *models.InstalledExtension) *platformdomain.InstalledExtension {
	var manifest platformdomain.ExtensionManifest
	if model.ManifestJSON != "" {
		unmarshalJSONField(model.ManifestJSON, &manifest, "installed_extensions", "manifest_json")
	}

	config := shareddomain.NewTypedCustomFields()
	if model.ConfigJSON != "" {
		unmarshalJSONField(model.ConfigJSON, &config, "installed_extensions", "config_json")
	}

	return &platformdomain.InstalledExtension{
		ID:                model.ID,
		WorkspaceID:       derefStringPtr(model.WorkspaceID),
		Slug:              model.Slug,
		Name:              model.Name,
		Publisher:         model.Publisher,
		Version:           model.Version,
		Description:       model.Description,
		LicenseToken:      model.LicenseToken,
		BundleSHA256:      model.BundleSHA256,
		BundleSize:        model.BundleSize,
		BundlePayload:     cloneBundlePayload(model.BundlePayload),
		Manifest:          manifest,
		Config:            config,
		Status:            platformdomain.ExtensionStatus(model.Status),
		ValidationStatus:  platformdomain.ExtensionValidationStatus(model.ValidationStatus),
		ValidationMessage: model.ValidationMessage,
		HealthStatus:      platformdomain.ExtensionHealthStatus(model.HealthStatus),
		HealthMessage:     model.HealthMessage,
		InstalledByID:     derefStringPtr(model.InstalledByID),
		InstalledAt:       model.InstalledAt,
		ActivatedAt:       model.ActivatedAt,
		DeactivatedAt:     model.DeactivatedAt,
		ValidatedAt:       model.ValidatedAt,
		LastHealthCheckAt: model.LastHealthCheckAt,
		UpdatedAt:         model.UpdatedAt,
		DeletedAt:         model.DeletedAt,
	}
}

func cloneBundlePayload(payload []byte) []byte {
	if payload == nil {
		return []byte{}
	}
	cloned := make([]byte, len(payload))
	copy(cloned, payload)
	return cloned
}

func nullableStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func derefStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (s *ExtensionStore) mapAssetToModel(asset *platformdomain.ExtensionAsset) *models.ExtensionAsset {
	return &models.ExtensionAsset{
		ID:             asset.ID,
		ExtensionID:    asset.ExtensionID,
		Path:           asset.Path,
		Kind:           string(asset.Kind),
		ContentType:    asset.ContentType,
		Content:        asset.Content,
		IsCustomizable: asset.IsCustomizable,
		Checksum:       asset.Checksum,
		Size:           asset.Size,
		CreatedAt:      asset.CreatedAt,
		UpdatedAt:      asset.UpdatedAt,
		DeletedAt:      asset.DeletedAt,
	}
}

func (s *ExtensionStore) mapModelToAsset(model *models.ExtensionAsset) *platformdomain.ExtensionAsset {
	return &platformdomain.ExtensionAsset{
		ID:             model.ID,
		ExtensionID:    model.ExtensionID,
		Path:           model.Path,
		Kind:           platformdomain.ExtensionAssetKind(model.Kind),
		ContentType:    model.ContentType,
		Content:        model.Content,
		IsCustomizable: model.IsCustomizable,
		Checksum:       model.Checksum,
		Size:           model.Size,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
		DeletedAt:      model.DeletedAt,
	}
}
