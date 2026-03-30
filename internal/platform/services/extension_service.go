package platformservices

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

type ExtensionService struct {
	extensionStore     shared.ExtensionStore
	workspaceStore     shared.WorkspaceStore
	queueStore         shared.QueueStore
	formStore          shared.FormStore
	ruleStore          shared.RuleStore
	schemaRuntime      ExtensionSchemaRuntime
	activationRuntime  ExtensionActivationRuntime
	bundleVerifier     ExtensionBundleVerifier
	healthRuntime      ExtensionHealthRuntime
	diagnosticsRuntime ExtensionDiagnosticsRuntime
	privilegedRuntime  PrivilegedExtensionRuntime
	privilegedPolicy   PrivilegedExtensionPolicy
	artifacts          extensionArtifactService
	tx                 contracts.TransactionRunner
	logger             *logger.Logger
}

type ExtensionSchemaRuntime interface {
	EnsureInstalledExtensionSchema(ctx context.Context, extension *platformdomain.InstalledExtension) error
}

type ExtensionActivationRuntime interface {
	EnsureInstalledExtensionRuntime(ctx context.Context, extension *platformdomain.InstalledExtension) error
}

type ExtensionDeactivationRuntime interface {
	DeactivateInstalledExtensionRuntime(ctx context.Context, extension *platformdomain.InstalledExtension, reason string) error
}

type ExtensionHealthRuntime interface {
	CheckInstalledExtensionHealth(ctx context.Context, extension *platformdomain.InstalledExtension) (platformdomain.ExtensionHealthStatus, string, error)
}

type ExtensionDiagnosticsRuntime interface {
	GetInstalledExtensionRuntimeDiagnostics(ctx context.Context, extension *platformdomain.InstalledExtension) (platformdomain.ExtensionRuntimeDiagnostics, error)
}

type PrivilegedExtensionRuntime interface {
	PrepareInstall(ctx context.Context, manifest platformdomain.ExtensionManifest, workspaceID string) error
}

type PrivilegedExtensionPolicy interface {
	ValidateInstall(ctx context.Context, manifest platformdomain.ExtensionManifest, workspaceID string) error
}

type extensionArtifactService interface {
	Commit(ctx context.Context, params artifactservices.CommitParams) (*artifactservices.CommitResult, error)
	Write(ctx context.Context, params artifactservices.WriteParams) (*artifactservices.WriteResult, error)
	Read(ctx context.Context, params artifactservices.ReadParams) ([]byte, error)
	List(ctx context.Context, repository artifactservices.RepositoryRef, prefix string) ([]string, error)
	History(ctx context.Context, repository artifactservices.RepositoryRef, relativePath string, limit int) ([]artifactservices.Revision, error)
	Diff(ctx context.Context, repository artifactservices.RepositoryRef, relativePath, fromRef, toRef string) (string, string, string, error)
}

type ExtensionServiceOption func(*ExtensionService)

func WithExtensionSchemaRuntime(runtime ExtensionSchemaRuntime) ExtensionServiceOption {
	return func(service *ExtensionService) {
		service.schemaRuntime = runtime
	}
}

func WithExtensionBundleVerifier(verifier ExtensionBundleVerifier) ExtensionServiceOption {
	return func(service *ExtensionService) {
		service.bundleVerifier = verifier
	}
}

func WithExtensionActivationRuntime(runtime ExtensionActivationRuntime) ExtensionServiceOption {
	return func(service *ExtensionService) {
		service.activationRuntime = runtime
	}
}

func WithExtensionHealthRuntime(runtime ExtensionHealthRuntime) ExtensionServiceOption {
	return func(service *ExtensionService) {
		service.healthRuntime = runtime
	}
}

func WithExtensionDiagnosticsRuntime(runtime ExtensionDiagnosticsRuntime) ExtensionServiceOption {
	return func(service *ExtensionService) {
		service.diagnosticsRuntime = runtime
	}
}

func WithPrivilegedExtensionRuntime(runtime PrivilegedExtensionRuntime) ExtensionServiceOption {
	return func(service *ExtensionService) {
		service.privilegedRuntime = runtime
	}
}

func WithPrivilegedExtensionPolicy(policy PrivilegedExtensionPolicy) ExtensionServiceOption {
	return func(service *ExtensionService) {
		service.privilegedPolicy = policy
	}
}

func WithExtensionArtifactService(artifacts extensionArtifactService) ExtensionServiceOption {
	return func(service *ExtensionService) {
		service.artifacts = artifacts
	}
}

type ExtensionAssetInput struct {
	Path           string
	ContentType    string
	Content        []byte
	IsCustomizable bool
}

type ExtensionMigrationInput struct {
	Path    string
	Content []byte
}

type InstallExtensionParams struct {
	WorkspaceID   string
	InstalledByID string
	LicenseToken  string
	BundleBase64  string
	Manifest      platformdomain.ExtensionManifest
	Assets        []ExtensionAssetInput
	Migrations    []ExtensionMigrationInput
}

type UpgradeExtensionParams struct {
	ExtensionID   string
	InstalledByID string
	LicenseToken  string
	BundleBase64  string
	Manifest      platformdomain.ExtensionManifest
	Assets        []ExtensionAssetInput
	Migrations    []ExtensionMigrationInput
}

func NewExtensionService(
	extensionStore shared.ExtensionStore,
	workspaceStore shared.WorkspaceStore,
	queueStore shared.QueueStore,
	formStore shared.FormStore,
	ruleStore shared.RuleStore,
	tx contracts.TransactionRunner,
) *ExtensionService {
	return NewExtensionServiceWithOptions(
		extensionStore,
		workspaceStore,
		queueStore,
		formStore,
		ruleStore,
		tx,
	)
}

func NewExtensionServiceWithOptions(
	extensionStore shared.ExtensionStore,
	workspaceStore shared.WorkspaceStore,
	queueStore shared.QueueStore,
	formStore shared.FormStore,
	ruleStore shared.RuleStore,
	tx contracts.TransactionRunner,
	options ...ExtensionServiceOption,
) *ExtensionService {
	service := &ExtensionService{
		extensionStore:   extensionStore,
		workspaceStore:   workspaceStore,
		queueStore:       queueStore,
		formStore:        formStore,
		ruleStore:        ruleStore,
		tx:               tx,
		logger:           logger.New().WithField("service", "extension"),
		privilegedPolicy: defaultPrivilegedExtensionPolicy(),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *ExtensionService) SetHealthRuntime(runtime ExtensionHealthRuntime) {
	s.healthRuntime = runtime
}

func (s *ExtensionService) SetDiagnosticsRuntime(runtime ExtensionDiagnosticsRuntime) {
	s.diagnosticsRuntime = runtime
}

func (s *ExtensionService) SetActivationRuntime(runtime ExtensionActivationRuntime) {
	s.activationRuntime = runtime
}

func (s *ExtensionService) SetPrivilegedRuntime(runtime PrivilegedExtensionRuntime) {
	s.privilegedRuntime = runtime
}

func (s *ExtensionService) GetInstalledExtension(ctx context.Context, extensionID string) (*platformdomain.InstalledExtension, error) {
	if strings.TrimSpace(extensionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("extension_id", "required"))
	}
	extension, err := s.extensionStore.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, apierrors.NotFoundError("extension", extensionID)
	}
	return extension, nil
}

func (s *ExtensionService) ListWorkspaceExtensions(ctx context.Context, workspaceID string) ([]*platformdomain.InstalledExtension, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	extensions, err := s.extensionStore.ListWorkspaceExtensions(ctx, workspaceID)
	if err != nil {
		return nil, apierrors.DatabaseError("list extensions", err)
	}
	return extensions, nil
}

func (s *ExtensionService) ListInstanceExtensions(ctx context.Context) ([]*platformdomain.InstalledExtension, error) {
	if s.extensionStore == nil {
		return nil, nil
	}
	extensions, err := s.extensionStore.ListInstanceExtensions(ctx)
	if err != nil {
		return nil, apierrors.DatabaseError("list instance extensions", err)
	}
	return extensions, nil
}

func (s *ExtensionService) HasActiveExtension(ctx context.Context, slug string) (bool, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return false, apierrors.NewValidationErrors(apierrors.NewValidationError("slug", "required"))
	}
	if s.workspaceStore == nil {
		if s.extensionStore == nil {
			return false, nil
		}
		instanceExtension, err := s.extensionStore.GetInstanceExtensionBySlug(ctx, slug)
		if err == nil && instanceExtension != nil && instanceExtension.Status == platformdomain.ExtensionStatusActive {
			return true, nil
		}
		return false, nil
	}

	if s.extensionStore != nil {
		instanceExtension, err := s.extensionStore.GetInstanceExtensionBySlug(ctx, slug)
		if err == nil && instanceExtension != nil && instanceExtension.Status == platformdomain.ExtensionStatusActive {
			return true, nil
		}
	}

	workspaces, err := s.workspaceStore.ListWorkspaces(ctx)
	if err != nil {
		return false, apierrors.DatabaseError("list workspaces", err)
	}

	for _, workspace := range workspaces {
		extension, err := s.extensionStore.GetInstalledExtensionBySlug(ctx, workspace.ID, slug)
		if err != nil || extension == nil {
			continue
		}
		if extension.Status == platformdomain.ExtensionStatusActive {
			return true, nil
		}
	}

	return false, nil
}

func (s *ExtensionService) InstallExtension(ctx context.Context, params InstallExtensionParams) (*platformdomain.InstalledExtension, error) {
	params.WorkspaceID = strings.TrimSpace(params.WorkspaceID)
	params.LicenseToken = strings.TrimSpace(params.LicenseToken)
	params.Manifest.Normalize()
	if err := s.validateInstallPolicy(ctx, params.Manifest, params.WorkspaceID); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "extension policy validation failed")
	}
	switch params.Manifest.Scope {
	case platformdomain.ExtensionScopeWorkspace:
		if params.WorkspaceID == "" {
			return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
		}
		if s.workspaceStore != nil {
			workspace, err := s.workspaceStore.GetWorkspace(ctx, params.WorkspaceID)
			if err != nil || workspace == nil {
				return nil, apierrors.NotFoundError("workspace", params.WorkspaceID)
			}
		}
		if existing, err := s.extensionStore.GetInstalledExtensionBySlug(ctx, params.WorkspaceID, params.Manifest.Slug); err == nil && existing != nil {
			return nil, apierrors.Newf(apierrors.ErrorTypeConflict, "extension %s is already installed in workspace", params.Manifest.Slug)
		}
	case platformdomain.ExtensionScopeInstance:
		if params.WorkspaceID != "" {
			return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "not_allowed_for_instance_scoped_extension"))
		}
		if existing, err := s.extensionStore.GetInstanceExtensionBySlug(ctx, params.Manifest.Slug); err == nil && existing != nil {
			return nil, apierrors.Newf(apierrors.ErrorTypeConflict, "extension %s is already installed on this instance", params.Manifest.Slug)
		}
	default:
		return nil, apierrors.Newf(apierrors.ErrorTypeValidation, "unsupported extension scope %q", params.Manifest.Scope)
	}
	if err := validateInstallTransport(params); err != nil {
		return nil, err
	}

	bundle, err := s.bundleBytes(params)
	if err != nil {
		return nil, err
	}
	if s.bundleVerifier != nil {
		if err := s.bundleVerifier.VerifyBundle(ctx, params.Manifest, params.LicenseToken, bundle); err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "extension bundle trust verification failed")
		}
	}

	installation, err := platformdomain.NewInstalledExtension(
		params.WorkspaceID,
		params.InstalledByID,
		params.LicenseToken,
		params.Manifest,
		bundle,
	)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "extension manifest validation failed")
	}

	var assets []*platformdomain.ExtensionAsset

	if s.tx != nil {
		err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := s.extensionStore.CreateInstalledExtension(txCtx, installation); err != nil {
				return apierrors.DatabaseError("create extension", err)
			}
			builtAssets, err := s.buildAssets(installation, params.Assets)
			if err != nil {
				return err
			}
			assets = builtAssets
			if err := s.extensionStore.ReplaceExtensionAssets(txCtx, installation.ID, assets); err != nil {
				return apierrors.DatabaseError("create extension assets", err)
			}
			return nil
		})
	} else {
		if err := s.extensionStore.CreateInstalledExtension(ctx, installation); err != nil {
			return nil, apierrors.DatabaseError("create extension", err)
		}
		assets, err = s.buildAssets(installation, params.Assets)
		if err != nil {
			return nil, err
		}
		if err := s.extensionStore.ReplaceExtensionAssets(ctx, installation.ID, assets); err != nil {
			return nil, apierrors.DatabaseError("create extension assets", err)
		}
	}
	if err != nil {
		return nil, err
	}

	valid, message := s.validateInstallation(ctx, installation, assets)
	installation.MarkValidation(valid, message)
	if err := s.extensionStore.UpdateInstalledExtension(ctx, installation); err != nil {
		return nil, apierrors.DatabaseError("update extension validation", err)
	}
	return installation, nil
}

func (s *ExtensionService) ActivateExtension(ctx context.Context, extensionID string) (*platformdomain.InstalledExtension, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	assets, err := s.extensionStore.ListExtensionAssets(ctx, extension.ID)
	if err != nil {
		return nil, apierrors.DatabaseError("list extension assets", err)
	}
	valid, message := s.validateInstallation(ctx, extension, assets)
	extension.MarkValidation(valid, message)
	if !valid {
		if updateErr := s.extensionStore.UpdateInstalledExtension(ctx, extension); updateErr != nil {
			return nil, apierrors.DatabaseError("update extension validation", updateErr)
		}
		return nil, apierrors.Newf(apierrors.ErrorTypeValidation, "extension validation failed: %s", message)
	}
	if err := s.ensureManagedArtifactSurfaces(ctx, extension, assets); err != nil {
		return nil, err
	}
	if s.schemaRuntime != nil {
		if err := s.schemaRuntime.EnsureInstalledExtensionSchema(ctx, extension); err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "extension schema migration failed")
		}
	}
	if s.activationRuntime != nil {
		if err := s.activationRuntime.EnsureInstalledExtensionRuntime(ctx, extension); err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "extension runtime activation failed")
		}
	}

	if s.tx != nil {
		err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			if extension.Manifest.Scope == platformdomain.ExtensionScopeWorkspace {
				if err := s.provisionQueues(txCtx, extension); err != nil {
					return err
				}
				if err := s.provisionForms(txCtx, extension); err != nil {
					return err
				}
				if err := s.provisionAutomationRules(txCtx, extension); err != nil {
					return err
				}
			}
			extension.Activate()
			return s.extensionStore.UpdateInstalledExtension(txCtx, extension)
		})
		if err != nil {
			return nil, err
		}
	} else {
		if extension.Manifest.Scope == platformdomain.ExtensionScopeWorkspace {
			if err := s.provisionQueues(ctx, extension); err != nil {
				return nil, err
			}
			if err := s.provisionForms(ctx, extension); err != nil {
				return nil, err
			}
			if err := s.provisionAutomationRules(ctx, extension); err != nil {
				return nil, err
			}
		}
		extension.Activate()
		if err := s.extensionStore.UpdateInstalledExtension(ctx, extension); err != nil {
			return nil, apierrors.DatabaseError("activate extension", err)
		}
	}
	return s.CheckExtensionHealth(ctx, extension.ID)
}

func (s *ExtensionService) UpgradeExtension(ctx context.Context, params UpgradeExtensionParams) (*platformdomain.InstalledExtension, error) {
	if strings.TrimSpace(params.ExtensionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("extension_id", "required"))
	}

	existing, err := s.GetInstalledExtension(ctx, params.ExtensionID)
	if err != nil {
		return nil, err
	}

	params.Manifest.Normalize()
	if params.Manifest.Slug != existing.Slug {
		return nil, apierrors.Newf(apierrors.ErrorTypeValidation, "extension slug cannot change during upgrade")
	}
	if err := s.validateInstallPolicy(ctx, params.Manifest, existing.WorkspaceID); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "extension policy validation failed")
	}

	installParams := InstallExtensionParams{
		WorkspaceID:   existing.WorkspaceID,
		InstalledByID: params.InstalledByID,
		LicenseToken:  params.LicenseToken,
		BundleBase64:  params.BundleBase64,
		Manifest:      params.Manifest,
		Assets:        params.Assets,
		Migrations:    params.Migrations,
	}
	if strings.TrimSpace(installParams.LicenseToken) == "" {
		installParams.LicenseToken = existing.LicenseToken
	}
	if err := validateInstallTransport(installParams); err != nil {
		return nil, err
	}

	bundle, err := s.bundleBytes(installParams)
	if err != nil {
		return nil, err
	}
	if s.bundleVerifier != nil {
		if err := s.bundleVerifier.VerifyBundle(ctx, params.Manifest, installParams.LicenseToken, bundle); err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "extension bundle trust verification failed")
		}
	}

	existingAssets, err := s.extensionStore.ListExtensionAssets(ctx, existing.ID)
	if err != nil {
		return nil, apierrors.DatabaseError("list extension assets", err)
	}

	upgraded := *existing
	upgraded.Manifest = params.Manifest
	upgraded.Name = params.Manifest.Name
	upgraded.Publisher = params.Manifest.Publisher
	upgraded.Version = params.Manifest.Version
	upgraded.Description = params.Manifest.Description
	upgraded.LicenseToken = installParams.LicenseToken
	upgraded.BundleSHA256 = checksumBytes(bundle)
	upgraded.BundleSize = int64(len(bundle))
	upgraded.BundlePayload = cloneInstallBundlePayload(bundle)
	upgraded.UpdatedAt = existing.UpdatedAt
	if upgraded.Config.IsEmpty() {
		config := params.Manifest.DefaultConfig
		if config.IsEmpty() {
			config = shareddomain.NewTypedCustomFields()
		}
		upgraded.Config = config
	}

	assets, err := s.buildAssets(&upgraded, params.Assets)
	if err != nil {
		return nil, err
	}
	assets = preserveCustomizableAssets(existingAssets, assets)

	previousStatus := existing.Status
	previousHealthMessage := existing.HealthMessage
	resetUpgradedExtensionState(&upgraded)

	if s.tx != nil {
		err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := s.extensionStore.UpdateInstalledExtension(txCtx, &upgraded); err != nil {
				return apierrors.DatabaseError("update extension", err)
			}
			if err := s.extensionStore.ReplaceExtensionAssets(txCtx, upgraded.ID, assets); err != nil {
				return apierrors.DatabaseError("replace extension assets", err)
			}
			return nil
		})
	} else {
		if err := s.extensionStore.UpdateInstalledExtension(ctx, &upgraded); err != nil {
			return nil, apierrors.DatabaseError("update extension", err)
		}
		if err := s.extensionStore.ReplaceExtensionAssets(ctx, upgraded.ID, assets); err != nil {
			return nil, apierrors.DatabaseError("replace extension assets", err)
		}
	}
	if err != nil {
		return nil, err
	}

	valid, message := s.validateInstallation(ctx, &upgraded, assets)
	upgraded.MarkValidation(valid, message)
	if !valid {
		if updateErr := s.extensionStore.UpdateInstalledExtension(ctx, &upgraded); updateErr != nil {
			return nil, apierrors.DatabaseError("update extension validation", updateErr)
		}
		return nil, apierrors.Newf(apierrors.ErrorTypeValidation, "extension validation failed: %s", message)
	}

	if previousStatus == platformdomain.ExtensionStatusActive {
		if err := s.ensureManagedArtifactSurfaces(ctx, &upgraded, assets); err != nil {
			return nil, err
		}
		if err := s.provisionQueues(ctx, &upgraded); err != nil {
			return nil, err
		}
		if err := s.provisionForms(ctx, &upgraded); err != nil {
			return nil, err
		}
		if err := s.provisionAutomationRules(ctx, &upgraded); err != nil {
			return nil, err
		}
		if s.schemaRuntime != nil {
			if err := s.schemaRuntime.EnsureInstalledExtensionSchema(ctx, &upgraded); err != nil {
				return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "extension schema migration failed")
			}
		}
		if s.activationRuntime != nil {
			if err := s.activationRuntime.EnsureInstalledExtensionRuntime(ctx, &upgraded); err != nil {
				return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "extension runtime activation failed")
			}
		}
		upgraded.Activate()
	} else if previousStatus == platformdomain.ExtensionStatusInactive {
		upgraded.Deactivate(previousHealthMessage)
	}

	if err := s.extensionStore.UpdateInstalledExtension(ctx, &upgraded); err != nil {
		return nil, apierrors.DatabaseError("update upgraded extension", err)
	}
	if upgraded.Status == platformdomain.ExtensionStatusActive {
		return s.CheckExtensionHealth(ctx, upgraded.ID)
	}
	return &upgraded, nil
}

func (s *ExtensionService) DeactivateExtension(ctx context.Context, extensionID, reason string) (*platformdomain.InstalledExtension, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	extension.Deactivate(reason)
	if err := s.extensionStore.UpdateInstalledExtension(ctx, extension); err != nil {
		return nil, apierrors.DatabaseError("deactivate extension", err)
	}
	if runtime, ok := s.activationRuntime.(ExtensionDeactivationRuntime); ok && runtime != nil {
		if err := runtime.DeactivateInstalledExtensionRuntime(ctx, extension, reason); err != nil {
			s.logger.Warn("Failed to drain extension runtime during deactivation",
				"extension_id", extension.ID,
				"slug", extension.Slug,
				"error", err,
			)
		}
	}
	return extension, nil
}

func (s *ExtensionService) UninstallExtension(ctx context.Context, extensionID string) error {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return err
	}
	if extension.Status == platformdomain.ExtensionStatusActive {
		return apierrors.Newf(apierrors.ErrorTypeValidation, "extension %s must be deactivated before uninstall", extension.Slug)
	}

	deleteInstalled := func(runCtx context.Context) error {
		if err := s.extensionStore.DeleteInstalledExtension(runCtx, extensionID); err != nil {
			return apierrors.DatabaseError("uninstall extension", err)
		}
		return nil
	}
	if s.tx != nil {
		return s.tx.WithTransaction(ctx, deleteInstalled)
	}
	return deleteInstalled(ctx)
}

func (s *ExtensionService) ValidateExtension(ctx context.Context, extensionID string) (*platformdomain.InstalledExtension, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	assets, err := s.extensionStore.ListExtensionAssets(ctx, extension.ID)
	if err != nil {
		return nil, apierrors.DatabaseError("list extension assets", err)
	}
	valid, message := s.validateInstallation(ctx, extension, assets)
	extension.MarkValidation(valid, message)
	if err := s.extensionStore.UpdateInstalledExtension(ctx, extension); err != nil {
		return nil, apierrors.DatabaseError("validate extension", err)
	}
	return extension, nil
}

func (s *ExtensionService) CheckExtensionHealth(ctx context.Context, extensionID string) (*platformdomain.InstalledExtension, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}

	status, message := s.resolveExtensionHealth(ctx, extension)
	extension.RecordHealth(status, message)
	if err := s.extensionStore.UpdateInstalledExtension(ctx, extension); err != nil {
		return nil, apierrors.DatabaseError("update extension health", err)
	}
	return extension, nil
}

func (s *ExtensionService) GetExtensionRuntimeDiagnostics(ctx context.Context, extensionID string) (platformdomain.ExtensionRuntimeDiagnostics, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return platformdomain.ExtensionRuntimeDiagnostics{}, err
	}
	if s.diagnosticsRuntime == nil {
		return platformdomain.ExtensionRuntimeDiagnostics{}, nil
	}
	diagnostics, err := s.diagnosticsRuntime.GetInstalledExtensionRuntimeDiagnostics(ctx, extension)
	if err != nil {
		return platformdomain.ExtensionRuntimeDiagnostics{}, err
	}
	return diagnostics, nil
}

func (s *ExtensionService) UpdateExtensionConfig(ctx context.Context, extensionID string, config map[string]interface{}) (*platformdomain.InstalledExtension, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	typedConfig := shareddomain.NewTypedCustomFields()
	for key, value := range config {
		setTypedConfigValue(&typedConfig, key, value)
	}
	extension.UpdateConfig(typedConfig)
	if err := s.extensionStore.UpdateInstalledExtension(ctx, extension); err != nil {
		return nil, apierrors.DatabaseError("update extension config", err)
	}
	return extension, nil
}

func (s *ExtensionService) ListExtensionAssets(ctx context.Context, extensionID string) ([]*platformdomain.ExtensionAsset, error) {
	if strings.TrimSpace(extensionID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("extension_id", "required"))
	}
	assets, err := s.extensionStore.ListExtensionAssets(ctx, extensionID)
	if err != nil {
		return nil, apierrors.DatabaseError("list extension assets", err)
	}
	return assets, nil
}

func (s *ExtensionService) UpdateExtensionAsset(ctx context.Context, extensionID, assetPath string, content []byte, contentType string) (*platformdomain.ExtensionAsset, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	if surfaceName, ok := managedArtifactSurfaceForAssetPath(extension.Manifest, assetPath); ok {
		return nil, apierrors.Newf(apierrors.ErrorTypeValidation, "asset %s is managed by artifact surface %s; use artifact publication instead", assetPath, surfaceName)
	}
	asset, err := s.extensionStore.GetExtensionAsset(ctx, extensionID, assetPath)
	if err != nil {
		return nil, apierrors.NotFoundError("extension_asset", assetPath)
	}
	if !asset.IsCustomizable {
		return nil, apierrors.Newf(apierrors.ErrorTypeAuthorization, "asset %s is not customizable", assetPath)
	}
	asset.UpdateContent(content, contentType)
	if err := s.extensionStore.UpdateExtensionAsset(ctx, asset); err != nil {
		return nil, apierrors.DatabaseError("update extension asset", err)
	}

	assets, err := s.extensionStore.ListExtensionAssets(ctx, extension.ID)
	if err != nil {
		return nil, apierrors.DatabaseError("list extension assets", err)
	}
	valid, message := s.validateInstallation(ctx, extension, assets)
	extension.MarkValidation(valid, message)
	if err := s.extensionStore.UpdateInstalledExtension(ctx, extension); err != nil {
		return nil, apierrors.DatabaseError("update extension validation", err)
	}
	return asset, nil
}

func (s *ExtensionService) ListExtensionArtifactFiles(ctx context.Context, extensionID, surface string) ([]*platformdomain.ExtensionArtifactFile, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	surfaceSpec, err := extensionArtifactSurfaceSpec(extension.Manifest, surface)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid extension artifact surface")
	}
	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	files, err := s.artifacts.List(ctx, extensionArtifactRepository(extension), extensionArtifactSurfaceRoot(extension, surfaceSpec.Name))
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "list extension artifacts")
	}
	result := make([]*platformdomain.ExtensionArtifactFile, 0, len(files))
	prefix := extensionArtifactSurfaceRoot(extension, surfaceSpec.Name)
	for _, file := range files {
		file = strings.TrimPrefix(file, prefix)
		file = strings.TrimPrefix(file, "/")
		if file == "" {
			continue
		}
		result = append(result, &platformdomain.ExtensionArtifactFile{
			Surface: surfaceSpec.Name,
			Path:    file,
		})
	}
	return result, nil
}

func (s *ExtensionService) GetExtensionArtifactContent(ctx context.Context, extensionID, surface, relativePath, ref string) (string, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return "", err
	}
	surfaceSpec, err := extensionArtifactSurfaceSpec(extension.Manifest, surface)
	if err != nil {
		return "", apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid extension artifact surface")
	}
	if s.artifacts == nil {
		return "", apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	data, err := s.artifacts.Read(ctx, artifactservices.ReadParams{
		Repository:   extensionArtifactRepository(extension),
		RelativePath: extensionArtifactPath(extension, surfaceSpec.Name, relativePath),
		Ref:          strings.TrimSpace(ref),
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", apierrors.NotFoundError("extension artifact", relativePath)
		}
		return "", apierrors.Wrap(err, apierrors.ErrorTypeInternal, "read extension artifact")
	}
	return string(data), nil
}

func (s *ExtensionService) GetExtensionArtifactHistory(ctx context.Context, extensionID, surface, relativePath string, limit int) ([]artifactservices.Revision, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	surfaceSpec, err := extensionArtifactSurfaceSpec(extension.Manifest, surface)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid extension artifact surface")
	}
	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	revisions, err := s.artifacts.History(ctx, extensionArtifactRepository(extension), extensionArtifactPath(extension, surfaceSpec.Name, relativePath), limit)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "load extension artifact history")
	}
	return revisions, nil
}

func (s *ExtensionService) GetExtensionArtifactDiff(ctx context.Context, extensionID, surface, relativePath, fromRevision, toRevision string) (*platformdomain.ExtensionArtifactDiff, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	surfaceSpec, err := extensionArtifactSurfaceSpec(extension.Manifest, surface)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid extension artifact surface")
	}
	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	pathRef := extensionArtifactPath(extension, surfaceSpec.Name, relativePath)
	fromRef, toRef, patch, err := s.artifacts.Diff(ctx, extensionArtifactRepository(extension), pathRef, fromRevision, toRevision)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "load extension artifact diff")
	}
	return &platformdomain.ExtensionArtifactDiff{
		Surface:      surfaceSpec.Name,
		Path:         strings.TrimSpace(relativePath),
		FromRevision: fromRef,
		ToRevision:   toRef,
		Patch:        patch,
	}, nil
}

func (s *ExtensionService) PublishExtensionArtifact(ctx context.Context, extensionID, surface, relativePath string, content []byte, actorID string) (*platformdomain.ExtensionArtifactPublication, error) {
	extension, err := s.GetInstalledExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	surfaceSpec, err := extensionArtifactSurfaceSpec(extension.Manifest, surface)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid extension artifact surface")
	}
	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	writeResult, err := s.artifacts.Write(ctx, artifactservices.WriteParams{
		Repository:    extensionArtifactRepository(extension),
		RelativePath:  extensionArtifactPath(extension, surfaceSpec.Name, relativePath),
		Content:       content,
		CommitMessage: extensionArtifactCommitMessage(extension, surfaceSpec.Name, relativePath),
		ActorID:       strings.TrimSpace(actorID),
	})
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "publish extension artifact")
	}
	return &platformdomain.ExtensionArtifactPublication{
		Surface:     surfaceSpec.Name,
		Path:        strings.TrimSpace(relativePath),
		RevisionRef: writeResult.Ref,
	}, nil
}

func (s *ExtensionService) bundleBytes(params InstallExtensionParams) ([]byte, error) {
	if strings.TrimSpace(params.BundleBase64) != "" {
		bundle, err := base64.StdEncoding.DecodeString(strings.TrimSpace(params.BundleBase64))
		if err != nil {
			return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("bundleBase64", "must be valid base64"))
		}
		return bundle, nil
	}

	payload := struct {
		Manifest platformdomain.ExtensionManifest `json:"manifest"`
		Assets   []struct {
			Path           string `json:"path"`
			ContentType    string `json:"contentType,omitempty"`
			IsCustomizable bool   `json:"isCustomizable,omitempty"`
			Content        string `json:"content,omitempty"`
		} `json:"assets,omitempty"`
		Migrations []struct {
			Path    string `json:"path"`
			Content string `json:"content,omitempty"`
		} `json:"migrations,omitempty"`
	}{
		Manifest: params.Manifest,
		Assets: make([]struct {
			Path           string `json:"path"`
			ContentType    string `json:"contentType,omitempty"`
			IsCustomizable bool   `json:"isCustomizable,omitempty"`
			Content        string `json:"content,omitempty"`
		}, 0, len(params.Assets)),
		Migrations: make([]struct {
			Path    string `json:"path"`
			Content string `json:"content,omitempty"`
		}, 0, len(params.Migrations)),
	}
	for _, asset := range params.Assets {
		payload.Assets = append(payload.Assets, struct {
			Path           string `json:"path"`
			ContentType    string `json:"contentType,omitempty"`
			IsCustomizable bool   `json:"isCustomizable,omitempty"`
			Content        string `json:"content,omitempty"`
		}{
			Path:           asset.Path,
			ContentType:    asset.ContentType,
			IsCustomizable: asset.IsCustomizable,
			Content:        base64.StdEncoding.EncodeToString(asset.Content),
		})
	}
	for _, migration := range params.Migrations {
		payload.Migrations = append(payload.Migrations, struct {
			Path    string `json:"path"`
			Content string `json:"content,omitempty"`
		}{
			Path:    strings.TrimSpace(migration.Path),
			Content: base64.StdEncoding.EncodeToString(migration.Content),
		})
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "failed to serialize extension bundle")
	}
	return data, nil
}

func (s *ExtensionService) buildAssets(extension *platformdomain.InstalledExtension, inputs []ExtensionAssetInput) ([]*platformdomain.ExtensionAsset, error) {
	assets := make([]*platformdomain.ExtensionAsset, 0, len(inputs))
	seenPaths := make(map[string]struct{}, len(inputs))
	customizablePaths := make(map[string]struct{}, len(extension.Manifest.CustomizableAssets))
	for _, assetPath := range extension.Manifest.CustomizableAssets {
		customizablePaths[strings.TrimSpace(assetPath)] = struct{}{}
	}

	for _, input := range inputs {
		path := strings.TrimSpace(input.Path)
		if path == "" {
			return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("assets.path", "required"))
		}
		if _, exists := seenPaths[path]; exists {
			return nil, apierrors.Newf(apierrors.ErrorTypeValidation, "duplicate asset path %s", path)
		}
		seenPaths[path] = struct{}{}
		_, listedAsCustomizable := customizablePaths[path]
		asset, err := platformdomain.NewExtensionAsset(
			extension.ID,
			path,
			input.ContentType,
			input.Content,
			input.IsCustomizable || listedAsCustomizable,
		)
		if err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid extension asset")
		}
		assets = append(assets, asset)
	}

	return assets, nil
}

func preserveCustomizableAssets(existing, next []*platformdomain.ExtensionAsset) []*platformdomain.ExtensionAsset {
	if len(existing) == 0 || len(next) == 0 {
		return next
	}
	existingByPath := make(map[string]*platformdomain.ExtensionAsset, len(existing))
	for _, asset := range existing {
		if asset == nil {
			continue
		}
		existingByPath[asset.Path] = asset
	}
	for _, asset := range next {
		if asset == nil || !asset.IsCustomizable {
			continue
		}
		previous, ok := existingByPath[asset.Path]
		if !ok || !previous.IsCustomizable {
			continue
		}
		asset.UpdateContent(previous.Content, previous.ContentType)
	}
	return next
}

func resetUpgradedExtensionState(extension *platformdomain.InstalledExtension) {
	extension.Status = platformdomain.ExtensionStatusInstalled
	extension.ValidationStatus = platformdomain.ExtensionValidationUnknown
	extension.ValidationMessage = ""
	extension.HealthStatus = platformdomain.ExtensionHealthInactive
	extension.HealthMessage = "extension installed but not active"
	extension.ActivatedAt = nil
	extension.DeactivatedAt = nil
	extension.ValidatedAt = nil
	extension.UpdatedAt = time.Now()
}

func checksumBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func (s *ExtensionService) provisionQueues(ctx context.Context, extension *platformdomain.InstalledExtension) error {
	for _, seed := range extension.Manifest.Queues {
		queue, err := buildSeededQueue(extension, seed)
		if err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid seeded queue")
		}
		if _, err := s.queueStore.GetQueueBySlug(ctx, extension.WorkspaceID, queue.Slug); err == nil {
			continue
		} else if !errors.Is(err, shared.ErrNotFound) {
			return apierrors.DatabaseError("lookup seeded queue", err)
		}
		if err := s.queueStore.CreateQueue(ctx, queue); err != nil {
			return apierrors.DatabaseError("create seeded queue", err)
		}
	}
	return nil
}

func (s *ExtensionService) provisionForms(ctx context.Context, extension *platformdomain.InstalledExtension) error {
	if s.formStore == nil {
		return nil
	}

	existingForms, err := s.formStore.ListWorkspaceFormSchemas(ctx, extension.WorkspaceID)
	if err != nil {
		return apierrors.DatabaseError("list workspace forms", err)
	}
	existingBySlug := make(map[string]struct{}, len(existingForms))
	for _, form := range existingForms {
		existingBySlug[form.Slug] = struct{}{}
	}

	for _, seed := range extension.Manifest.Forms {
		if _, exists := existingBySlug[seed.Slug]; exists {
			continue
		}

		form := buildSeededForm(extension, seed)

		if err := s.formStore.CreateFormSchema(ctx, form); err != nil {
			return apierrors.DatabaseError("create seeded form", err)
		}
		existingBySlug[seed.Slug] = struct{}{}
	}

	return nil
}

func (s *ExtensionService) provisionAutomationRules(ctx context.Context, extension *platformdomain.InstalledExtension) error {
	if s.ruleStore == nil {
		return nil
	}

	existingRules, err := s.ruleStore.ListWorkspaceRules(ctx, extension.WorkspaceID)
	if err != nil {
		return apierrors.DatabaseError("list workspace rules", err)
	}
	existingByKey := make(map[string]struct{}, len(existingRules))
	for _, rule := range existingRules {
		if strings.TrimSpace(rule.SystemRuleKey) == "" {
			continue
		}
		existingByKey[rule.SystemRuleKey] = struct{}{}
	}

	for _, seed := range extension.Manifest.AutomationRules {
		if _, exists := existingByKey[seed.Key]; exists {
			continue
		}

		rule, err := buildSeededAutomationRule(extension, seed)
		if err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "invalid seeded automation rule")
		}

		if err := s.ruleStore.CreateRule(ctx, rule); err != nil {
			return apierrors.DatabaseError("create seeded automation rule", err)
		}
		existingByKey[seed.Key] = struct{}{}
	}

	return nil
}

func (s *ExtensionService) validateInstallation(ctx context.Context, extension *platformdomain.InstalledExtension, assets []*platformdomain.ExtensionAsset) (bool, string) {
	if err := extension.Manifest.Validate(); err != nil {
		return false, err.Error()
	}
	if err := s.validateRuntimeTopology(ctx, extension); err != nil {
		return false, err.Error()
	}
	assetPaths := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		assetPaths[asset.Path] = struct{}{}
	}

	for _, route := range extension.Manifest.PublicRoutes {
		if route.AssetPath != "" && !hasAssetOrPrefix(assetPaths, route.AssetPath) {
			return false, fmt.Sprintf("missing asset for public route %s", route.AssetPath)
		}
	}
	for _, route := range extension.Manifest.AdminRoutes {
		if route.AssetPath != "" && !hasAssetOrPrefix(assetPaths, route.AssetPath) {
			return false, fmt.Sprintf("missing asset for admin route %s", route.AssetPath)
		}
	}
	for _, endpoint := range extension.Manifest.Endpoints {
		if endpoint.AssetPath == "" {
			continue
		}
		if !hasAssetOrPrefix(assetPaths, endpoint.AssetPath) {
			return false, fmt.Sprintf("missing asset for endpoint %s (%s)", endpoint.Name, endpoint.AssetPath)
		}
	}
	for _, surface := range extension.Manifest.ArtifactSurfaces {
		if surface.SeedAssetPath != "" && !hasAssetOrPrefix(assetPaths, surface.SeedAssetPath) {
			return false, fmt.Sprintf("missing seed assets for artifact surface %s (%s)", surface.Name, surface.SeedAssetPath)
		}
	}
	for _, assetPath := range extension.Manifest.CustomizableAssets {
		if _, ok := assetPaths[assetPath]; !ok {
			return false, fmt.Sprintf("missing customizable asset %s", assetPath)
		}
	}
	for _, skill := range extension.Manifest.AgentSkills {
		if _, ok := assetPaths[skill.AssetPath]; !ok {
			return false, fmt.Sprintf("missing agent skill asset %s", skill.AssetPath)
		}
	}
	if s.workspaceStore != nil && extension.Manifest.Scope == platformdomain.ExtensionScopeWorkspace {
		if workspace, err := s.workspaceStore.GetWorkspace(ctx, extension.WorkspaceID); err != nil || workspace == nil {
			return false, "workspace not found"
		}
	}
	if err := s.validateEventSubscriptions(ctx, extension); err != nil {
		return false, err.Error()
	}
	for _, seed := range extension.Manifest.AutomationRules {
		conditions, err := decodeTypedConditions(seed.Conditions)
		if err != nil {
			return false, fmt.Sprintf("invalid automation conditions for %s: %v", seed.Key, err)
		}
		actions, err := decodeTypedActions(seed.Actions)
		if err != nil {
			return false, fmt.Sprintf("invalid automation actions for %s: %v", seed.Key, err)
		}
		if err := automationservices.ValidateRuleActions(conditions, actions); err != nil {
			return false, fmt.Sprintf("invalid automation rule %s: %v", seed.Key, err)
		}
	}
	return true, "manifest and installed assets validated"
}

func decodeTypedConditions(schema shareddomain.TypedSchema) (automationdomain.TypedConditions, error) {
	if schema.IsEmpty() {
		return automationdomain.TypedConditions{
			Operator:   shareddomain.LogicalAnd,
			Conditions: []automationdomain.TypedCondition{},
		}, nil
	}

	var typed automationdomain.TypedConditions
	data, err := json.Marshal(schema.ToMap())
	if err != nil {
		return automationdomain.TypedConditions{}, err
	}
	if err := json.Unmarshal(data, &typed); err != nil {
		return automationdomain.TypedConditions{}, err
	}
	if typed.Operator == "" {
		typed.Operator = shareddomain.LogicalAnd
	}
	if typed.Conditions == nil {
		typed.Conditions = []automationdomain.TypedCondition{}
	}
	return typed, nil
}

func decodeTypedActions(schema shareddomain.TypedSchema) (automationdomain.TypedActions, error) {
	if schema.IsEmpty() {
		return automationdomain.TypedActions{Actions: []automationdomain.TypedAction{}}, nil
	}

	var typed automationdomain.TypedActions
	data, err := json.Marshal(schema.ToMap())
	if err != nil {
		return automationdomain.TypedActions{}, err
	}
	if err := json.Unmarshal(data, &typed); err != nil {
		return automationdomain.TypedActions{}, err
	}
	if typed.Actions == nil {
		typed.Actions = []automationdomain.TypedAction{}
	}
	return typed, nil
}

func seededBy(extension *platformdomain.InstalledExtension) string {
	if strings.TrimSpace(extension.InstalledByID) != "" {
		return extension.InstalledByID
	}
	return "system"
}

func setTypedConfigValue(fields *shareddomain.TypedCustomFields, key string, value interface{}) {
	switch typed := value.(type) {
	case []interface{}:
		stringsValue := make([]string, 0, len(typed))
		for _, item := range typed {
			if str, ok := item.(string); ok {
				stringsValue = append(stringsValue, str)
			}
		}
		fields.SetStrings(key, stringsValue)
	default:
		fields.SetAny(key, value)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func hasAssetOrPrefix(assetPaths map[string]struct{}, assetPath string) bool {
	assetPath = strings.TrimSpace(assetPath)
	if assetPath == "" {
		return false
	}
	if _, ok := assetPaths[assetPath]; ok {
		return true
	}
	prefix := strings.TrimSuffix(assetPath, "/") + "/"
	for existingPath := range assetPaths {
		if strings.HasPrefix(existingPath, prefix) {
			return true
		}
	}
	return false
}

func extensionArtifactRepository(extension *platformdomain.InstalledExtension) artifactservices.RepositoryRef {
	if extension != nil && strings.TrimSpace(extension.WorkspaceID) != "" {
		return artifactservices.WorkspaceRepository(extension.WorkspaceID)
	}
	return artifactservices.InstanceRepository()
}

func extensionArtifactSurfaceRoot(extension *platformdomain.InstalledExtension, surface string) string {
	return path.Join("extensions", extension.Slug, "surfaces", surface)
}

func extensionArtifactPath(extension *platformdomain.InstalledExtension, surface, relativePath string) string {
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return extensionArtifactSurfaceRoot(extension, surface)
	}
	return path.Join(extensionArtifactSurfaceRoot(extension, surface), relativePath)
}

func extensionArtifactSurfaceSpec(manifest platformdomain.ExtensionManifest, surface string) (platformdomain.ExtensionArtifactSurface, error) {
	surface = strings.TrimSpace(surface)
	for _, candidate := range manifest.ArtifactSurfaces {
		if candidate.Name == surface {
			return candidate, nil
		}
	}
	return platformdomain.ExtensionArtifactSurface{}, fmt.Errorf("artifact surface %s not found", surface)
}

func extensionArtifactCommitMessage(extension *platformdomain.InstalledExtension, surface, relativePath string) string {
	return fmt.Sprintf("publish extension artifact %s %s/%s", extension.Slug, surface, strings.TrimSpace(relativePath))
}

func managedArtifactSurfaceForAssetPath(manifest platformdomain.ExtensionManifest, assetPath string) (string, bool) {
	assetPath = strings.TrimSpace(assetPath)
	if assetPath == "" {
		return "", false
	}
	for _, surface := range manifest.ArtifactSurfaces {
		if surface.SeedAssetPath == "" {
			continue
		}
		if assetPath == surface.SeedAssetPath || strings.HasPrefix(assetPath, strings.TrimSuffix(surface.SeedAssetPath, "/")+"/") {
			return surface.Name, true
		}
	}
	return "", false
}

func seedSurfaceRelativePath(seedAssetPath, assetPath string) (string, bool) {
	seedAssetPath = strings.TrimSpace(seedAssetPath)
	assetPath = strings.TrimSpace(assetPath)
	if seedAssetPath == "" || assetPath == "" {
		return "", false
	}
	if assetPath == seedAssetPath {
		return path.Base(assetPath), true
	}
	prefix := strings.TrimSuffix(seedAssetPath, "/") + "/"
	if !strings.HasPrefix(assetPath, prefix) {
		return "", false
	}
	relative := strings.TrimPrefix(assetPath, prefix)
	if strings.TrimSpace(relative) == "" {
		return "", false
	}
	return relative, true
}

func (s *ExtensionService) ensureManagedArtifactSurfaces(ctx context.Context, extension *platformdomain.InstalledExtension, assets []*platformdomain.ExtensionAsset) error {
	if extension == nil || s.artifacts == nil || len(extension.Manifest.ArtifactSurfaces) == 0 {
		return nil
	}
	repository := extensionArtifactRepository(extension)
	for _, surface := range extension.Manifest.ArtifactSurfaces {
		if strings.TrimSpace(surface.SeedAssetPath) == "" {
			continue
		}
		files := make([]artifactservices.CommitFile, 0)
		for _, asset := range assets {
			if asset == nil {
				continue
			}
			relative, ok := seedSurfaceRelativePath(surface.SeedAssetPath, asset.Path)
			if !ok {
				continue
			}
			target := extensionArtifactPath(extension, surface.Name, relative)
			if _, err := s.artifacts.Read(ctx, artifactservices.ReadParams{
				Repository:   repository,
				RelativePath: target,
			}); err == nil {
				continue
			} else if !errors.Is(err, fs.ErrNotExist) {
				return apierrors.Wrap(err, apierrors.ErrorTypeInternal, "read extension artifact seed state")
			}
			files = append(files, artifactservices.CommitFile{
				RelativePath: target,
				Content:      asset.Content,
			})
		}
		if len(files) == 0 {
			continue
		}
		if _, err := s.artifacts.Commit(ctx, artifactservices.CommitParams{
			Repository:    repository,
			Files:         files,
			CommitMessage: fmt.Sprintf("seed extension artifact surface %s for %s", surface.Name, extension.Slug),
			ActorID:       firstNonEmptyString(extension.InstalledByID, "system"),
		}); err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeInternal, "seed extension artifact surface")
		}
	}
	return nil
}

type defaultPrivilegedPolicy struct {
	allowedPublishers map[string]struct{}
}

func defaultPrivilegedExtensionPolicy() PrivilegedExtensionPolicy {
	return &defaultPrivilegedPolicy{
		allowedPublishers: map[string]struct{}{
			"demandops": {},
		},
	}
}

func (s *ExtensionService) validateInstallPolicy(ctx context.Context, manifest platformdomain.ExtensionManifest, workspaceID string) error {
	if manifest.RequiresPrivilegedInstallPolicy() {
		if s.privilegedPolicy == nil {
			return fmt.Errorf("privileged extensions require an install policy")
		}
		if err := s.privilegedPolicy.ValidateInstall(ctx, manifest, workspaceID); err != nil {
			return err
		}
		if s.privilegedRuntime != nil {
			if err := s.privilegedRuntime.PrepareInstall(ctx, manifest, workspaceID); err != nil {
				return err
			}
		}
		return nil
	}
	return manifest.ValidateGenericInstallPolicy()
}

func (p *defaultPrivilegedPolicy) ValidateInstall(_ context.Context, manifest platformdomain.ExtensionManifest, workspaceID string) error {
	return manifest.ValidatePrivilegedInstallPolicy(workspaceID, p.publisherAllowed)
}

func (p *defaultPrivilegedPolicy) publisherAllowed(publisher string) bool {
	if p == nil {
		return false
	}
	_, ok := p.allowedPublishers[strings.ToLower(strings.TrimSpace(publisher))]
	return ok
}

func validateInstallTransport(params InstallExtensionParams) error {
	if params.Manifest.RuntimeClass != platformdomain.ExtensionRuntimeClassServiceBacked {
		return nil
	}
	if strings.TrimSpace(params.BundleBase64) != "" {
		return nil
	}
	if len(params.Migrations) == 0 {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("migrations", "service-backed extensions installed from source must include migrations"))
	}
	return nil
}

func (s *ExtensionService) resolveExtensionHealth(ctx context.Context, extension *platformdomain.InstalledExtension) (platformdomain.ExtensionHealthStatus, string) {
	if status, message, ok := extension.BaseHealthStatus(); ok {
		return status, message
	}
	if s.healthRuntime == nil {
		return platformdomain.ExtensionHealthDegraded, "service runtime health checks are not configured"
	}

	status, message, err := s.healthRuntime.CheckInstalledExtensionHealth(ctx, extension)
	if err != nil {
		return platformdomain.ExtensionHealthFailed, err.Error()
	}
	if status == "" {
		status = platformdomain.ExtensionHealthUnknown
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = platformdomain.DefaultExtensionHealthMessage(status)
	}
	return status, message
}

func cloneInstallBundlePayload(bundle []byte) []byte {
	if bundle == nil {
		return []byte{}
	}
	cloned := make([]byte, len(bundle))
	copy(cloned, bundle)
	return cloned
}
