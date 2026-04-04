package extensionreconcile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/platform/extensionbundle"
	"github.com/movebigrocks/platform/internal/platform/extensiondesiredstate"
	domain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

type fakeBundleLoader struct {
	payloads map[string]extensionbundle.SourcePayload
}

func (l fakeBundleLoader) ReadSource(_ context.Context, source, _ string) (extensionbundle.SourcePayload, error) {
	payload, ok := l.payloads[source]
	if !ok {
		return extensionbundle.SourcePayload{}, fmt.Errorf("unexpected source %s", source)
	}
	return payload, nil
}

type fakeInventory struct {
	installed []*domain.InstalledExtension
}

func (i fakeInventory) ListAllExtensions(context.Context) ([]*domain.InstalledExtension, error) {
	return i.installed, nil
}

type fakeWorkspaceLookup struct {
	bySlug map[string]*domain.Workspace
}

func (l fakeWorkspaceLookup) GetWorkspaceBySlug(_ context.Context, slug string) (*domain.Workspace, error) {
	workspace, ok := l.bySlug[slug]
	if !ok {
		return nil, fmt.Errorf("workspace %s not found", slug)
	}
	return workspace, nil
}

type fakeLifecycle struct{}

func (fakeLifecycle) InstallExtension(context.Context, platformservices.InstallExtensionParams) (*domain.InstalledExtension, error) {
	return nil, nil
}
func (fakeLifecycle) UpgradeExtension(context.Context, platformservices.UpgradeExtensionParams) (*domain.InstalledExtension, error) {
	return nil, nil
}
func (fakeLifecycle) UpdateExtensionConfig(context.Context, string, map[string]any) (*domain.InstalledExtension, error) {
	return nil, nil
}
func (fakeLifecycle) ValidateExtension(context.Context, string) (*domain.InstalledExtension, error) {
	return nil, nil
}
func (fakeLifecycle) ActivateExtension(context.Context, string) (*domain.InstalledExtension, error) {
	return nil, nil
}
func (fakeLifecycle) DeactivateExtension(context.Context, string, string) (*domain.InstalledExtension, error) {
	return nil, nil
}
func (fakeLifecycle) UninstallExtension(context.Context, string) error {
	return nil
}
func (fakeLifecycle) CheckExtensionHealth(_ context.Context, extensionID string) (*domain.InstalledExtension, error) {
	return &domain.InstalledExtension{
		ID:           extensionID,
		HealthStatus: domain.ExtensionHealthHealthy,
	}, nil
}

type recordingLifecycle struct {
	calls []string
}

func (l *recordingLifecycle) InstallExtension(context.Context, platformservices.InstallExtensionParams) (*domain.InstalledExtension, error) {
	return nil, nil
}

func (l *recordingLifecycle) UpgradeExtension(_ context.Context, params platformservices.UpgradeExtensionParams) (*domain.InstalledExtension, error) {
	l.calls = append(l.calls, "upgrade")
	return &domain.InstalledExtension{
		ID:               params.ExtensionID,
		WorkspaceID:      "ws_default",
		Slug:             params.Manifest.Slug,
		Version:          params.Manifest.Version,
		BundleSHA256:     checksumHex(mustDecodeBase64(params.BundleBase64)),
		Manifest:         params.Manifest,
		Status:           domain.ExtensionStatusInactive,
		ValidationStatus: domain.ExtensionValidationValid,
	}, nil
}

func (l *recordingLifecycle) UpdateExtensionConfig(context.Context, string, map[string]any) (*domain.InstalledExtension, error) {
	return nil, nil
}

func (l *recordingLifecycle) ValidateExtension(context.Context, string) (*domain.InstalledExtension, error) {
	return nil, nil
}

func (l *recordingLifecycle) ActivateExtension(_ context.Context, extensionID string) (*domain.InstalledExtension, error) {
	l.calls = append(l.calls, "activate")
	return &domain.InstalledExtension{
		ID:           extensionID,
		HealthStatus: domain.ExtensionHealthHealthy,
		Status:       domain.ExtensionStatusActive,
	}, nil
}

func (l *recordingLifecycle) DeactivateExtension(context.Context, string, string) (*domain.InstalledExtension, error) {
	return nil, nil
}

func (l *recordingLifecycle) UninstallExtension(context.Context, string) error {
	return nil
}

func (l *recordingLifecycle) CheckExtensionHealth(_ context.Context, extensionID string) (*domain.InstalledExtension, error) {
	return &domain.InstalledExtension{
		ID:           extensionID,
		HealthStatus: domain.ExtensionHealthHealthy,
		Status:       domain.ExtensionStatusActive,
	}, nil
}

func TestPlanInstallsAndActivatesMissingExtension(t *testing.T) {
	engine := NewEngine(
		fakeBundleLoader{payloads: map[string]extensionbundle.SourcePayload{
			"oci://ats": payloadForManifest(t, domain.ExtensionManifest{
				Slug:         "ats",
				Name:         "ATS",
				Version:      "1.0.0",
				Publisher:    "DemandOps",
				Kind:         domain.ExtensionKindProduct,
				Scope:        domain.ExtensionScopeWorkspace,
				Risk:         domain.ExtensionRiskStandard,
				RuntimeClass: domain.ExtensionRuntimeClassBundle,
				StorageClass: domain.ExtensionStorageClassSharedPrimitivesOnly,
			}),
		}},
		fakeInventory{},
		fakeWorkspaceLookup{bySlug: map[string]*domain.Workspace{
			"default": {ID: "ws_default", Slug: "default"},
		}},
		fakeLifecycle{},
	)

	doc := mustDesiredState(t, `
extensions:
  installed:
    - slug: ats
      ref: oci://ats
      scope: workspace
      workspace: default
`)
	plan, err := engine.Plan(t.Context(), doc, "extensions/desired-state.yaml")
	require.NoError(t, err)
	require.Len(t, plan.Operations, 3)
	assert.Equal(t, OperationInstall, plan.Operations[0].Action)
	assert.Equal(t, OperationValidate, plan.Operations[1].Action)
	assert.Equal(t, OperationActivate, plan.Operations[2].Action)
}

func TestPlanUpgradesConfiguresAndDeactivatesWhenDesiredStateChanges(t *testing.T) {
	fields := shareddomain.NewTypedCustomFields()
	fields.SetString("region", "us")
	engine := NewEngine(
		fakeBundleLoader{payloads: map[string]extensionbundle.SourcePayload{
			"oci://ats": payloadForManifest(t, domain.ExtensionManifest{
				Slug:         "ats",
				Name:         "ATS",
				Version:      "2.0.0",
				Publisher:    "DemandOps",
				Kind:         domain.ExtensionKindProduct,
				Scope:        domain.ExtensionScopeWorkspace,
				Risk:         domain.ExtensionRiskStandard,
				RuntimeClass: domain.ExtensionRuntimeClassBundle,
				StorageClass: domain.ExtensionStorageClassSharedPrimitivesOnly,
			}),
		}},
		fakeInventory{installed: []*domain.InstalledExtension{
			{
				ID:               "ext_123",
				WorkspaceID:      "ws_default",
				Slug:             "ats",
				Version:          "1.0.0",
				BundleSHA256:     "different",
				Manifest:         domain.ExtensionManifest{Scope: domain.ExtensionScopeWorkspace},
				Config:           fields,
				Status:           domain.ExtensionStatusActive,
				ValidationStatus: domain.ExtensionValidationValid,
			},
		}},
		fakeWorkspaceLookup{bySlug: map[string]*domain.Workspace{
			"default": {ID: "ws_default", Slug: "default"},
		}},
		fakeLifecycle{},
	)

	doc := mustDesiredState(t, `
extensions:
  installed:
    - slug: ats
      ref: oci://ats
      scope: workspace
      workspace: default
      activate: false
      config:
        region: eu
`)
	plan, err := engine.Plan(t.Context(), doc, "")
	require.NoError(t, err)
	require.Len(t, plan.Operations, 4)
	assert.Equal(t, OperationUpgrade, plan.Operations[0].Action)
	assert.Equal(t, OperationConfigure, plan.Operations[1].Action)
	assert.Equal(t, OperationValidate, plan.Operations[2].Action)
	assert.Equal(t, OperationDeactivate, plan.Operations[3].Action)
}

func TestPlanMarksUnmanagedExtensionsAsDrift(t *testing.T) {
	engine := NewEngine(
		fakeBundleLoader{payloads: map[string]extensionbundle.SourcePayload{
			"oci://ats": payloadForManifest(t, domain.ExtensionManifest{
				Slug:         "ats",
				Name:         "ATS",
				Version:      "1.0.0",
				Publisher:    "DemandOps",
				Kind:         domain.ExtensionKindProduct,
				Scope:        domain.ExtensionScopeWorkspace,
				Risk:         domain.ExtensionRiskStandard,
				RuntimeClass: domain.ExtensionRuntimeClassBundle,
				StorageClass: domain.ExtensionStorageClassSharedPrimitivesOnly,
			}),
		}},
		fakeInventory{installed: []*domain.InstalledExtension{
			{
				ID:          "ext_extra",
				WorkspaceID: "ws_default",
				Slug:        "error-tracking",
				Version:     "1.0.0",
				Manifest:    domain.ExtensionManifest{Scope: domain.ExtensionScopeWorkspace},
				Status:      domain.ExtensionStatusActive,
			},
		}},
		fakeWorkspaceLookup{bySlug: map[string]*domain.Workspace{
			"default": {ID: "ws_default", Slug: "default"},
		}},
		fakeLifecycle{},
	)

	doc := mustDesiredState(t, `
extensions:
  installed:
    - slug: ats
      ref: oci://ats
      scope: workspace
      workspace: default
`)
	plan, err := engine.Plan(t.Context(), doc, "")
	require.NoError(t, err)
	require.Len(t, plan.Operations, 4)
	assert.Equal(t, OperationDrift, plan.Operations[3].Action)
	assert.Equal(t, "error-tracking", plan.Operations[3].Slug)
}

func TestRenderRuntimeManifestForServiceBackedExtension(t *testing.T) {
	engine := NewEngine(
		fakeBundleLoader{payloads: map[string]extensionbundle.SourcePayload{
			"oci://ats": payloadForManifest(t, domain.ExtensionManifest{
				Slug:         "ats",
				Name:         "ATS",
				Version:      "1.0.0",
				Publisher:    "DemandOps",
				Kind:         domain.ExtensionKindProduct,
				Scope:        domain.ExtensionScopeWorkspace,
				Risk:         domain.ExtensionRiskStandard,
				RuntimeClass: domain.ExtensionRuntimeClassServiceBacked,
				StorageClass: domain.ExtensionStorageClassOwnedSchema,
				Schema: domain.ExtensionSchemaManifest{
					Name:          "ats",
					PackageKey:    "demandops/ats",
					TargetVersion: "1",
				},
				Runtime: domain.ExtensionRuntimeSpec{
					Protocol:     domain.ExtensionRuntimeProtocolUnixSocketHTTP,
					OCIReference: "ghcr.io/movebigrocks/mbr-ext-ats-runtime:v1.0.0",
				},
			}),
		}},
		fakeInventory{},
		fakeWorkspaceLookup{bySlug: map[string]*domain.Workspace{
			"default": {ID: "ws_default", Slug: "default"},
		}},
		fakeLifecycle{},
	)

	doc := mustDesiredState(t, `
extensions:
  installed:
    - slug: ats
      ref: oci://ats
      scope: workspace
      workspace: default
`)
	manifest, err := engine.RenderRuntimeManifest(t.Context(), doc)
	require.NoError(t, err)
	require.Len(t, manifest.Runtimes, 1)
	assert.Equal(t, "ats", manifest.Runtimes[0].Slug)
	assert.Equal(t, "demandops/ats", manifest.Runtimes[0].PackageKey)
	assert.Equal(t, "ghcr.io/movebigrocks/mbr-ext-ats-runtime:v1.0.0", manifest.Runtimes[0].Artifact)
	assert.Equal(t, "ats-runtime", manifest.Runtimes[0].Service)
	assert.Equal(t, "demandops_ats.sock", manifest.Runtimes[0].Socket)
}

func TestApplyRefreshesSchemaBackedBundleBeforeActivation(t *testing.T) {
	lifecycle := &recordingLifecycle{}
	payload := payloadForManifest(t, domain.ExtensionManifest{
		Slug:         "ats",
		Name:         "ATS",
		Version:      "1.0.0",
		Publisher:    "DemandOps",
		Kind:         domain.ExtensionKindProduct,
		Scope:        domain.ExtensionScopeWorkspace,
		Risk:         domain.ExtensionRiskStandard,
		RuntimeClass: domain.ExtensionRuntimeClassServiceBacked,
		StorageClass: domain.ExtensionStorageClassOwnedSchema,
		Schema: domain.ExtensionSchemaManifest{
			Name:          "ats",
			PackageKey:    "demandops/ats",
			TargetVersion: "1",
		},
		Runtime: domain.ExtensionRuntimeSpec{
			Protocol:     domain.ExtensionRuntimeProtocolUnixSocketHTTP,
			OCIReference: "ghcr.io/movebigrocks/mbr-ext-ats-runtime:v1.0.0",
		},
	})
	engine := NewEngine(
		fakeBundleLoader{payloads: map[string]extensionbundle.SourcePayload{
			"oci://ats": payload,
		}},
		fakeInventory{installed: []*domain.InstalledExtension{
			{
				ID:               "ext_ats",
				WorkspaceID:      "ws_default",
				Slug:             "ats",
				Version:          "1.0.0",
				BundleSHA256:     checksumHex(payload.Bytes),
				Manifest:         domain.ExtensionManifest{Scope: domain.ExtensionScopeWorkspace},
				Status:           domain.ExtensionStatusInactive,
				ValidationStatus: domain.ExtensionValidationValid,
			},
		}},
		fakeWorkspaceLookup{bySlug: map[string]*domain.Workspace{
			"default": {ID: "ws_default", Slug: "default"},
		}},
		lifecycle,
	)

	doc := mustDesiredState(t, `
extensions:
  installed:
    - slug: ats
      ref: oci://ats
      scope: workspace
      workspace: default
`)

	result, err := engine.Apply(t.Context(), doc, "extensions/desired-state.yaml")
	require.NoError(t, err)
	require.NotNil(t, result.Check)
	assert.Equal(t, []string{"upgrade", "activate"}, lifecycle.calls)
}

func payloadForManifest(t *testing.T, manifest domain.ExtensionManifest) extensionbundle.SourcePayload {
	t.Helper()
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	var manifestMap map[string]any
	require.NoError(t, json.Unmarshal(raw, &manifestMap))

	bundle := extensionbundle.File{
		Manifest: manifestMap,
		Assets:   []extensionbundle.Asset{},
	}
	data, err := json.Marshal(bundle)
	require.NoError(t, err)
	return extensionbundle.SourcePayload{
		Kind:   extensionbundle.SourceKindOCI,
		Bundle: bundle,
		Bytes:  data,
	}
}

func mustDesiredState(t *testing.T, raw string) extensiondesiredstate.Document {
	t.Helper()
	doc, err := extensiondesiredstate.Parse([]byte(raw))
	require.NoError(t, err)
	return doc
}

func mustDecodeBase64(value string) []byte {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		panic(fmt.Sprintf("decode base64: %v", err))
	}
	return data
}
