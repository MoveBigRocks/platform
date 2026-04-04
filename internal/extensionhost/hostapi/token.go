package hostapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

const DefaultTokenTTL = 5 * time.Minute

type TokenClaims struct {
	ExtensionID string `json:"extensionId"`
	Slug        string `json:"slug"`
	PackageKey  string `json:"packageKey"`
	ExpiresAt   int64  `json:"exp"`
}

func IssueToken(secret string, extension *platformdomain.InstalledExtension, ttl time.Duration) (string, error) {
	if extension == nil {
		return "", fmt.Errorf("extension is required")
	}
	secret = signingSecret(secret)
	if secret == "" {
		return "", fmt.Errorf("host token signing secret is not configured")
	}
	if ttl <= 0 {
		ttl = DefaultTokenTTL
	}
	claims := TokenClaims{
		ExtensionID: strings.TrimSpace(extension.ID),
		Slug:        strings.TrimSpace(extension.Slug),
		PackageKey:  strings.TrimSpace(extension.Manifest.PackageKey()),
		ExpiresAt:   time.Now().UTC().Add(ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode host token claims: %w", err)
	}
	signature := sign(secret, payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func VerifyToken(secret string, token string) (*TokenClaims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("host token is required")
	}
	secret = signingSecret(secret)
	if secret == "" {
		return nil, fmt.Errorf("host token signing secret is not configured")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid host token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid host token")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid host token")
	}
	if !hmac.Equal(signature, sign(secret, payload)) {
		return nil, fmt.Errorf("invalid host token")
	}
	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("invalid host token")
	}
	if strings.TrimSpace(claims.ExtensionID) == "" || strings.TrimSpace(claims.PackageKey) == "" {
		return nil, fmt.Errorf("invalid host token")
	}
	if claims.ExpiresAt == 0 || time.Now().UTC().After(time.Unix(claims.ExpiresAt, 0)) {
		return nil, fmt.Errorf("host token has expired")
	}
	return &claims, nil
}

func signingSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	return "mbr-extension-host:" + secret
}

func sign(secret string, payload []byte) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}
