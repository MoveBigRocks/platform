package platformservices

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestExtensionService_InstallsTrustedSignedBundle(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	verifier, err := NewExtensionBundleTrustVerifier("inst_acme_123", true, map[string]map[string]string{
		"DemandOps": {
			"demandops-main": base64.StdEncoding.EncodeToString(publicKey),
		},
	})
	require.NoError(t, err)

	service := NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		WithExtensionBundleVerifier(verifier),
	)

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "ats",
		Name:          "Applicant Tracking",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindProduct,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		PublicRoutes: []platformdomain.ExtensionRoute{
			{PathPrefix: "/careers", AssetPath: "templates/careers/index.html"},
		},
	}
	assets := []ExtensionAssetInput{
		{
			Path:        "templates/careers/index.html",
			ContentType: "text/html",
			Content:     []byte("<html><body>Careers</body></html>"),
		},
	}
	licenseToken := "lic_signed"
	bundleBase64 := signedBundleBase64(t, manifest, assets, nil, bundleLicenseClaim{
		InstanceID:  "inst_acme_123",
		Publisher:   "DemandOps",
		Slug:        "ats",
		Version:     "1.0.0",
		TokenSHA256: checksumSHA256Hex([]byte(licenseToken)),
	}, "demandops-main", privateKey)

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: licenseToken,
		BundleBase64: bundleBase64,
		Manifest:     manifest,
		Assets:       assets,
	})
	require.NoError(t, err)
	require.NotNil(t, installed)
	assert.Equal(t, platformdomain.ExtensionStatusInstalled, installed.Status)
}

func TestExtensionService_RejectsBundleForWrongInstance(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	verifier, err := NewExtensionBundleTrustVerifier("inst_acme_123", true, map[string]map[string]string{
		"DemandOps": {
			"demandops-main": base64.StdEncoding.EncodeToString(publicKey),
		},
	})
	require.NoError(t, err)

	service := NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		WithExtensionBundleVerifier(verifier),
	)

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "ats",
		Name:          "Applicant Tracking",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindProduct,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
	}
	licenseToken := "lic_signed"
	bundleBase64 := signedBundleBase64(t, manifest, nil, nil, bundleLicenseClaim{
		InstanceID:  "inst_other",
		Publisher:   "DemandOps",
		Slug:        "ats",
		Version:     "1.0.0",
		TokenSHA256: checksumSHA256Hex([]byte(licenseToken)),
	}, "demandops-main", privateKey)

	_, err = service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: licenseToken,
		BundleBase64: bundleBase64,
		Manifest:     manifest,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trust verification failed")
	assert.Contains(t, err.Error(), "not valid for instance")
}

func signedBundleBase64(
	t *testing.T,
	manifest platformdomain.ExtensionManifest,
	assets []ExtensionAssetInput,
	migrations []ExtensionMigrationInput,
	license bundleLicenseClaim,
	keyID string,
	privateKey ed25519.PrivateKey,
) string {
	t.Helper()

	assetPayload := make([]map[string]any, 0, len(assets))
	for _, asset := range assets {
		item := map[string]any{
			"path":    asset.Path,
			"content": string(asset.Content),
		}
		if asset.ContentType != "" {
			item["contentType"] = asset.ContentType
		}
		if asset.IsCustomizable {
			item["isCustomizable"] = asset.IsCustomizable
		}
		assetPayload = append(assetPayload, item)
	}

	migrationPayload := make([]map[string]any, 0, len(migrations))
	for _, migration := range migrations {
		migrationPayload = append(migrationPayload, map[string]any{
			"path":    migration.Path,
			"content": string(migration.Content),
		})
	}

	manifestRaw, err := json.Marshal(manifest)
	require.NoError(t, err)
	assetsRaw, err := json.Marshal(assetPayload)
	require.NoError(t, err)
	migrationsRaw, err := json.Marshal(migrationPayload)
	require.NoError(t, err)

	payload, err := canonicalSignedBundlePayload(manifestRaw, assetsRaw, migrationsRaw, license)
	require.NoError(t, err)

	signature := ed25519.Sign(privateKey, payload)
	envelope := map[string]any{
		"manifest":   json.RawMessage(manifestRaw),
		"assets":     json.RawMessage(assetsRaw),
		"migrations": json.RawMessage(migrationsRaw),
		"trust": map[string]any{
			"keyID":     keyID,
			"algorithm": "ed25519",
			"signature": base64.StdEncoding.EncodeToString(signature),
			"license":   license,
		},
	}
	rawBundle, err := json.Marshal(envelope)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(rawBundle)
}
