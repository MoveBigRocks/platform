package servicedomain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// FormAPIToken represents an API token for programmatic form submissions
type FormAPIToken struct {
	ID           string
	WorkspaceID  string
	FormID       string
	Token        string
	Name         string
	IsActive     bool
	ExpiresAt    *time.Time
	AllowedHosts []string
	LastUsedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GenerateFormAPIToken creates a high-entropy token suitable for one-time
// display to an API client. Only HashFormAPIToken's output should be persisted.
func GenerateFormAPIToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate form API token: %w", err)
	}
	return "fat_" + hex.EncodeToString(randomBytes), nil
}

// HashFormAPIToken returns the stable SHA-256 lookup hash for a plaintext form
// API token. Form tokens are random credentials, so a fast hash is appropriate.
func HashFormAPIToken(plaintext string) string {
	hash := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(hash[:])
}
