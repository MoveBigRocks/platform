package platformdomain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type SandboxStatus string

const (
	SandboxStatusPendingVerification SandboxStatus = "pending_verification"
	SandboxStatusProvisioning        SandboxStatus = "provisioning"
	SandboxStatusReady               SandboxStatus = "ready"
	SandboxStatusExpired             SandboxStatus = "expired"
	SandboxStatusFailed              SandboxStatus = "failed"
	SandboxStatusDestroyed           SandboxStatus = "destroyed"
)

type Sandbox struct {
	ID                      string
	Slug                    string
	Name                    string
	RequestedEmail          string
	Status                  SandboxStatus
	RuntimeURL              string
	LoginURL                string
	BootstrapURL            string
	VerificationTokenHash   string
	ManageTokenHash         string
	VerificationRequestedAt time.Time
	VerifiedAt              *time.Time
	ActivationDeadlineAt    time.Time
	ExpiresAt               *time.Time
	ExpiredAt               *time.Time
	ExtendedAt              *time.Time
	DestroyedAt             *time.Time
	LastError               string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func NewSandbox(slug, name, email, verificationTokenHash, manageTokenHash string, now time.Time, activationTTL time.Duration) *Sandbox {
	if activationTTL <= 0 {
		activationTTL = 24 * time.Hour
	}
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		cleanName = strings.TrimSpace(slug)
	}
	return &Sandbox{
		Slug:                    strings.TrimSpace(slug),
		Name:                    cleanName,
		RequestedEmail:          strings.ToLower(strings.TrimSpace(email)),
		Status:                  SandboxStatusPendingVerification,
		VerificationTokenHash:   strings.TrimSpace(verificationTokenHash),
		ManageTokenHash:         strings.TrimSpace(manageTokenHash),
		VerificationRequestedAt: now.UTC(),
		ActivationDeadlineAt:    now.UTC().Add(activationTTL),
		CreatedAt:               now.UTC(),
		UpdatedAt:               now.UTC(),
	}
}

func (s *Sandbox) MarkProvisioning(now time.Time) error {
	if s == nil {
		return errors.New("sandbox is required")
	}
	if s.Status != SandboxStatusPendingVerification && s.Status != SandboxStatusFailed {
		return errors.New("sandbox cannot enter provisioning from current status")
	}
	s.Status = SandboxStatusProvisioning
	s.LastError = ""
	s.UpdatedAt = now.UTC()
	return nil
}

func (s *Sandbox) MarkReady(runtimeURL, loginURL, bootstrapURL string, now time.Time, trialTTL time.Duration) error {
	if s == nil {
		return errors.New("sandbox is required")
	}
	if s.Status != SandboxStatusProvisioning {
		return errors.New("sandbox must be provisioning before it can become ready")
	}
	if trialTTL <= 0 {
		trialTTL = 5 * 24 * time.Hour
	}
	verifiedAt := now.UTC()
	expiresAt := verifiedAt.Add(trialTTL)
	s.Status = SandboxStatusReady
	s.RuntimeURL = strings.TrimSpace(runtimeURL)
	s.LoginURL = strings.TrimSpace(loginURL)
	s.BootstrapURL = strings.TrimSpace(bootstrapURL)
	s.VerifiedAt = &verifiedAt
	s.ExpiresAt = &expiresAt
	s.LastError = ""
	s.UpdatedAt = verifiedAt
	return nil
}

func (s *Sandbox) MarkFailed(message string, now time.Time) error {
	if s == nil {
		return errors.New("sandbox is required")
	}
	if s.Status == SandboxStatusDestroyed || s.Status == SandboxStatusExpired {
		return errors.New("expired or destroyed sandbox cannot fail")
	}
	s.Status = SandboxStatusFailed
	s.LastError = strings.TrimSpace(message)
	s.UpdatedAt = now.UTC()
	return nil
}

func (s *Sandbox) ExpiryDue(now time.Time) (bool, string) {
	if s == nil {
		return false, ""
	}
	at := now.UTC()
	switch s.Status {
	case SandboxStatusReady:
		if s.ExpiresAt != nil && !at.Before(s.ExpiresAt.UTC()) {
			return true, "sandbox trial expired"
		}
	case SandboxStatusPendingVerification, SandboxStatusProvisioning, SandboxStatusFailed:
		if !at.Before(s.ActivationDeadlineAt.UTC()) {
			return true, "sandbox activation window expired"
		}
	}
	return false, ""
}

func (s *Sandbox) MarkExpired(reason string, now time.Time) error {
	if s == nil {
		return errors.New("sandbox is required")
	}
	if s.Status == SandboxStatusDestroyed {
		return errors.New("destroyed sandbox cannot expire")
	}
	if s.Status == SandboxStatusExpired {
		return errors.New("sandbox already expired")
	}
	expiredAt := now.UTC()
	message := strings.TrimSpace(reason)
	if message == "" {
		message = "sandbox expired"
	}
	s.Status = SandboxStatusExpired
	s.ExpiredAt = &expiredAt
	s.LastError = message
	s.UpdatedAt = expiredAt
	return nil
}

func (s *Sandbox) Extend(now time.Time, duration time.Duration) error {
	if s == nil {
		return errors.New("sandbox is required")
	}
	if s.Status != SandboxStatusReady {
		return errors.New("only ready sandboxes can be extended")
	}
	if duration <= 0 {
		return errors.New("extension duration must be positive")
	}
	if s.ExpiresAt == nil {
		return errors.New("sandbox expiry is required")
	}
	base := *s.ExpiresAt
	if now.UTC().After(base) {
		base = now.UTC()
	}
	extendedAt := now.UTC()
	expiresAt := base.Add(duration)
	s.ExtendedAt = &extendedAt
	s.ExpiresAt = &expiresAt
	s.UpdatedAt = extendedAt
	return nil
}

func (s *Sandbox) Destroy(reason string, now time.Time) error {
	if s == nil {
		return errors.New("sandbox is required")
	}
	if s.Status == SandboxStatusDestroyed {
		return errors.New("sandbox already destroyed")
	}
	destroyedAt := now.UTC()
	s.Status = SandboxStatusDestroyed
	s.DestroyedAt = &destroyedAt
	s.LastError = strings.TrimSpace(reason)
	s.UpdatedAt = destroyedAt
	return nil
}

func HashSandboxToken(token string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(hash[:])
}

func GenerateSandboxToken(prefix string) (plaintext, hash string, err error) {
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", err
	}
	plaintext = prefix + "_" + hex.EncodeToString(randomBytes)
	return plaintext, HashSandboxToken(plaintext), nil
}
