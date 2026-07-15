package platformservices

import (
	"context"
	"encoding/json"

	"github.com/movebigrocks/extension-sdk/bundletrust"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

type ExtensionBundleVerifier interface {
	VerifyBundle(ctx context.Context, manifest platformdomain.ExtensionManifest, licenseToken string, bundle []byte) error
}

type ExtensionBundleTrustVerifier struct {
	verifier *bundletrust.Verifier
}

type bundleLicenseClaim = bundletrust.LicenseClaim

func NewExtensionBundleTrustVerifier(instanceID string, requireVerification bool, trustedPublishers map[string]map[string]string) (*ExtensionBundleTrustVerifier, error) {
	verifier, err := bundletrust.NewVerifier(instanceID, requireVerification, trustedPublishers)
	if err != nil {
		return nil, err
	}
	return &ExtensionBundleTrustVerifier{verifier: verifier}, nil
}

func (v *ExtensionBundleTrustVerifier) VerifyBundle(ctx context.Context, manifest platformdomain.ExtensionManifest, licenseToken string, bundle []byte) error {
	if v == nil {
		return nil
	}
	return v.verifier.VerifyBundle(ctx, bundletrust.ManifestIdentity{
		Publisher: manifest.Publisher,
		Slug:      manifest.Slug,
		Version:   manifest.Version,
	}, licenseToken, bundle)
}

func canonicalSignedBundlePayload(manifestRaw, assetsRaw, migrationsRaw json.RawMessage, license bundleLicenseClaim) ([]byte, error) {
	return bundletrust.CanonicalSignedBundlePayload(manifestRaw, assetsRaw, migrationsRaw, license)
}

func checksumSHA256Hex(value []byte) string {
	return bundletrust.ChecksumSHA256Hex(value)
}
