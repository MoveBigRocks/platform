package platformservices

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

type ExtensionBundleVerifier interface {
	VerifyBundle(ctx context.Context, manifest platformdomain.ExtensionManifest, licenseToken string, bundle []byte) error
}

type ExtensionBundleTrustVerifier struct {
	instanceID           string
	requireVerification  bool
	trustedPublisherKeys map[string]map[string]ed25519.PublicKey
}

type bundleTrustEnvelope struct {
	KeyID     string             `json:"keyID"`
	Algorithm string             `json:"algorithm,omitempty"`
	Signature string             `json:"signature"`
	License   bundleLicenseClaim `json:"license"`
}

type bundleLicenseClaim struct {
	InstanceID  string `json:"instanceID"`
	Publisher   string `json:"publisher,omitempty"`
	Slug        string `json:"slug"`
	Version     string `json:"version"`
	TokenSHA256 string `json:"tokenSHA256"`
}

type rawBundleEnvelope struct {
	Manifest   json.RawMessage      `json:"manifest"`
	Assets     json.RawMessage      `json:"assets,omitempty"`
	Migrations json.RawMessage      `json:"migrations,omitempty"`
	Trust      *bundleTrustEnvelope `json:"trust,omitempty"`
}

func NewExtensionBundleTrustVerifier(instanceID string, requireVerification bool, trustedPublishers map[string]map[string]string) (*ExtensionBundleTrustVerifier, error) {
	keys := map[string]map[string]ed25519.PublicKey{}
	for publisher, keyMap := range trustedPublishers {
		publisher = strings.TrimSpace(publisher)
		if publisher == "" {
			return nil, fmt.Errorf("trusted publisher name cannot be empty")
		}
		keys[publisher] = map[string]ed25519.PublicKey{}
		for keyID, encodedKey := range keyMap {
			keyID = strings.TrimSpace(keyID)
			if keyID == "" {
				return nil, fmt.Errorf("trusted publisher key id cannot be empty for %s", publisher)
			}
			publicKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
			if err != nil {
				return nil, fmt.Errorf("decode trusted publisher key %s/%s: %w", publisher, keyID, err)
			}
			if len(publicKey) != ed25519.PublicKeySize {
				return nil, fmt.Errorf("trusted publisher key %s/%s must be %d bytes", publisher, keyID, ed25519.PublicKeySize)
			}
			keys[publisher][keyID] = ed25519.PublicKey(publicKey)
		}
	}

	return &ExtensionBundleTrustVerifier{
		instanceID:           strings.TrimSpace(instanceID),
		requireVerification:  requireVerification,
		trustedPublisherKeys: keys,
	}, nil
}

func (v *ExtensionBundleTrustVerifier) VerifyBundle(_ context.Context, manifest platformdomain.ExtensionManifest, licenseToken string, bundle []byte) error {
	if v == nil {
		return nil
	}
	publisherKeys := v.trustedPublisherKeys[strings.TrimSpace(manifest.Publisher)]
	shouldVerify := v.requireVerification || len(publisherKeys) > 0
	if !shouldVerify {
		return nil
	}
	if strings.TrimSpace(v.instanceID) == "" {
		return fmt.Errorf("instance_id is required for extension bundle verification")
	}
	if len(bundle) == 0 {
		return fmt.Errorf("bundle payload is required for extension bundle verification")
	}

	var envelope rawBundleEnvelope
	if err := json.Unmarshal(bundle, &envelope); err != nil {
		return fmt.Errorf("decode signed bundle: %w", err)
	}
	if envelope.Trust == nil {
		return fmt.Errorf("signed bundle metadata is required")
	}
	if envelope.Trust.Algorithm != "" && !strings.EqualFold(envelope.Trust.Algorithm, "ed25519") {
		return fmt.Errorf("unsupported bundle signature algorithm %q", envelope.Trust.Algorithm)
	}
	if len(publisherKeys) == 0 {
		return fmt.Errorf("publisher %s is not trusted on this instance", manifest.Publisher)
	}

	keyID := strings.TrimSpace(envelope.Trust.KeyID)
	if keyID == "" {
		return fmt.Errorf("bundle trust metadata is missing keyID")
	}
	publicKey, ok := publisherKeys[keyID]
	if !ok {
		return fmt.Errorf("publisher %s key %s is not trusted on this instance", manifest.Publisher, keyID)
	}

	license := envelope.Trust.License
	if strings.TrimSpace(license.InstanceID) != v.instanceID {
		return fmt.Errorf("bundle license is not valid for instance %s", v.instanceID)
	}
	if license.Publisher != "" && strings.TrimSpace(license.Publisher) != strings.TrimSpace(manifest.Publisher) {
		return fmt.Errorf("bundle license publisher does not match manifest publisher")
	}
	if strings.TrimSpace(license.Slug) != strings.TrimSpace(manifest.Slug) {
		return fmt.Errorf("bundle license slug does not match manifest slug")
	}
	if strings.TrimSpace(license.Version) != strings.TrimSpace(manifest.Version) {
		return fmt.Errorf("bundle license version does not match manifest version")
	}
	if tokenHash := checksumSHA256Hex([]byte(strings.TrimSpace(licenseToken))); !strings.EqualFold(strings.TrimSpace(license.TokenSHA256), tokenHash) {
		return fmt.Errorf("bundle license token does not match the provided token")
	}

	payload, err := canonicalSignedBundlePayload(envelope.Manifest, envelope.Assets, envelope.Migrations, license)
	if err != nil {
		return fmt.Errorf("build signed bundle payload: %w", err)
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(envelope.Trust.Signature))
	if err != nil {
		return fmt.Errorf("decode bundle signature: %w", err)
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("bundle signature verification failed")
	}
	return nil
}

func canonicalSignedBundlePayload(manifestRaw, assetsRaw, migrationsRaw json.RawMessage, license bundleLicenseClaim) ([]byte, error) {
	manifestValue, err := decodeBundleSection(manifestRaw, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	assetsValue, err := decodeBundleSection(assetsRaw, []any{})
	if err != nil {
		return nil, fmt.Errorf("decode assets: %w", err)
	}
	migrationsValue, err := decodeBundleSection(migrationsRaw, []any{})
	if err != nil {
		return nil, fmt.Errorf("decode migrations: %w", err)
	}

	payload := map[string]any{
		"assets":     assetsValue,
		"license":    license,
		"manifest":   manifestValue,
		"migrations": migrationsValue,
	}
	return json.Marshal(payload)
}

func decodeBundleSection(raw json.RawMessage, defaultValue any) (any, error) {
	if len(raw) == 0 {
		return defaultValue, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value == nil {
		return defaultValue, nil
	}
	return value, nil
}

func checksumSHA256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
