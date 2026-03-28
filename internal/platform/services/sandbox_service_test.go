package platformservices

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

type sandboxMemoryStore struct {
	byID                 map[string]*platformdomain.Sandbox
	bySlug               map[string]string
	byVerificationHashes map[string]string
}

func newSandboxMemoryStore() *sandboxMemoryStore {
	return &sandboxMemoryStore{
		byID:                 map[string]*platformdomain.Sandbox{},
		bySlug:               map[string]string{},
		byVerificationHashes: map[string]string{},
	}
}

func (s *sandboxMemoryStore) CreateSandbox(_ context.Context, sandbox *platformdomain.Sandbox) error {
	if sandbox == nil {
		return errors.New("sandbox is required")
	}
	if sandbox.ID == "" {
		sandbox.ID = "sbx_test"
	}
	s.byID[sandbox.ID] = cloneSandbox(sandbox)
	s.bySlug[sandbox.Slug] = sandbox.ID
	s.byVerificationHashes[sandbox.VerificationTokenHash] = sandbox.ID
	return nil
}

func (s *sandboxMemoryStore) GetSandbox(_ context.Context, sandboxID string) (*platformdomain.Sandbox, error) {
	if sandbox, ok := s.byID[sandboxID]; ok {
		return cloneSandbox(sandbox), nil
	}
	return nil, nil
}

func (s *sandboxMemoryStore) GetSandboxBySlug(_ context.Context, slug string) (*platformdomain.Sandbox, error) {
	if sandboxID, ok := s.bySlug[slug]; ok {
		return s.GetSandbox(context.Background(), sandboxID)
	}
	return nil, nil
}

func (s *sandboxMemoryStore) GetSandboxByVerificationTokenHash(_ context.Context, tokenHash string) (*platformdomain.Sandbox, error) {
	if sandboxID, ok := s.byVerificationHashes[tokenHash]; ok {
		return s.GetSandbox(context.Background(), sandboxID)
	}
	return nil, nil
}

func (s *sandboxMemoryStore) ListReapableSandboxes(_ context.Context, now time.Time) ([]*platformdomain.Sandbox, error) {
	reapable := make([]*platformdomain.Sandbox, 0)
	for _, sandbox := range s.byID {
		if due, _ := sandbox.ExpiryDue(now); due {
			reapable = append(reapable, cloneSandbox(sandbox))
		}
	}
	return reapable, nil
}

func (s *sandboxMemoryStore) UpdateSandbox(_ context.Context, sandbox *platformdomain.Sandbox) error {
	if sandbox == nil {
		return errors.New("sandbox is required")
	}
	s.byID[sandbox.ID] = cloneSandbox(sandbox)
	s.bySlug[sandbox.Slug] = sandbox.ID
	s.byVerificationHashes[sandbox.VerificationTokenHash] = sandbox.ID
	return nil
}

type sandboxRecordingProvisioner struct {
	provisionCalls int
	destroyCalls   int
}

func (p *sandboxRecordingProvisioner) Provision(_ context.Context, sandbox *platformdomain.Sandbox) (SandboxProvisionResult, error) {
	p.provisionCalls++
	base := "https://" + sandbox.Slug + ".movebigrocks.io"
	return SandboxProvisionResult{
		RuntimeURL:   base,
		LoginURL:     base + "/login",
		BootstrapURL: base + "/.well-known/mbr-instance.json",
	}, nil
}

func (p *sandboxRecordingProvisioner) Destroy(_ context.Context, _ *platformdomain.Sandbox) error {
	p.destroyCalls++
	return nil
}

func (p *sandboxRecordingProvisioner) Export(_ context.Context, sandbox *platformdomain.Sandbox) (SandboxExportBundle, error) {
	return SandboxExportBundle{
		Includes: []string{"sandbox_metadata", "runtime_urls"},
		Data: map[string]any{
			"runtime": map[string]any{
				"url": sandbox.RuntimeURL,
			},
		},
	}, nil
}

func TestSandboxServiceCreateVerifyExtendDestroy(t *testing.T) {
	t.Parallel()

	store := newSandboxMemoryStore()
	provisioner := &sandboxRecordingProvisioner{}
	service := NewSandboxService(
		store,
		SandboxServiceConfig{
			PublicBaseURL: "https://movebigrocks.com",
			RuntimeDomain: "movebigrocks.io",
			TrialTTL:      5 * 24 * time.Hour,
			ExtensionTTL:  30 * 24 * time.Hour,
		},
		WithSandboxProvisioner(provisioner),
	)

	result, err := service.CreateSandbox(t.Context(), SandboxCreateParams{
		Email: "Ops@Example.com",
		Name:  "Agent Trial",
	})
	if err != nil {
		t.Fatalf("create sandbox: %v", err)
	}
	if result.Sandbox.Status != platformdomain.SandboxStatusReady {
		t.Fatalf("unexpected create status %q", result.Sandbox.Status)
	}
	if result.Sandbox.RequestedEmail != "ops@example.com" {
		t.Fatalf("unexpected requested email %q", result.Sandbox.RequestedEmail)
	}
	if result.Sandbox.RuntimeURL == "" {
		t.Fatalf("expected runtime URL")
	}
	if result.ManageToken == "" {
		t.Fatalf("expected manage token")
	}
	if result.VerificationURL == "" {
		t.Fatalf("expected verification URL")
	}
	if provisioner.provisionCalls != 1 {
		t.Fatalf("expected one provision call during create, got %d", provisioner.provisionCalls)
	}

	verificationToken := result.VerificationURL[strings.LastIndex(result.VerificationURL, "=")+1:]
	verified, err := service.VerifySandbox(t.Context(), verificationToken)
	if err != nil {
		t.Fatalf("verify sandbox: %v", err)
	}
	if verified.Status != platformdomain.SandboxStatusReady {
		t.Fatalf("unexpected verified status %q", verified.Status)
	}
	if provisioner.provisionCalls != 1 {
		t.Fatalf("expected verify to reuse ready sandbox, got %d provision calls", provisioner.provisionCalls)
	}

	loaded, err := service.GetSandbox(t.Context(), verified.ID, result.ManageToken)
	if err != nil {
		t.Fatalf("get sandbox: %v", err)
	}
	if loaded.RuntimeURL == "" {
		t.Fatalf("expected runtime URL")
	}
	previousExpiry := *loaded.ExpiresAt

	extended, err := service.ExtendSandbox(t.Context(), loaded.ID, result.ManageToken)
	if err != nil {
		t.Fatalf("extend sandbox: %v", err)
	}
	if !extended.ExpiresAt.After(previousExpiry) {
		t.Fatalf("expected extended expiry")
	}

	destroyed, err := service.DestroySandbox(t.Context(), loaded.ID, result.ManageToken, "expired")
	if err != nil {
		t.Fatalf("destroy sandbox: %v", err)
	}
	if destroyed.Status != platformdomain.SandboxStatusDestroyed {
		t.Fatalf("unexpected destroyed status %q", destroyed.Status)
	}
	if provisioner.destroyCalls != 1 {
		t.Fatalf("expected one destroy call, got %d", provisioner.destroyCalls)
	}
}

func TestSandboxServiceGetSandboxRejectsWrongManageToken(t *testing.T) {
	t.Parallel()

	store := newSandboxMemoryStore()
	service := NewSandboxService(
		store,
		SandboxServiceConfig{
			PublicBaseURL: "https://movebigrocks.com",
			RuntimeDomain: "movebigrocks.io",
		},
		WithSandboxProvisioner(&sandboxRecordingProvisioner{}),
	)
	result, err := service.CreateSandbox(t.Context(), SandboxCreateParams{
		Email: "ops@example.com",
	})
	if err != nil {
		t.Fatalf("create sandbox: %v", err)
	}

	_, err = service.GetSandbox(t.Context(), result.Sandbox.ID, "wrong-token")
	if err == nil {
		t.Fatalf("expected invalid token error")
	}
}

func TestSandboxServiceExportSandbox(t *testing.T) {
	t.Parallel()

	store := newSandboxMemoryStore()
	service := NewSandboxService(
		store,
		SandboxServiceConfig{
			PublicBaseURL: "https://movebigrocks.com",
			RuntimeDomain: "movebigrocks.io",
		},
		WithSandboxProvisioner(&sandboxRecordingProvisioner{}),
	)
	result, err := service.CreateSandbox(t.Context(), SandboxCreateParams{
		Email: "ops@example.com",
	})
	if err != nil {
		t.Fatalf("create sandbox: %v", err)
	}
	exported, err := service.ExportSandbox(t.Context(), result.Sandbox.ID, result.ManageToken)
	if err != nil {
		t.Fatalf("export sandbox: %v", err)
	}
	if exported.ExportVersion != "mbr-sandbox-export-v1" {
		t.Fatalf("unexpected export version %q", exported.ExportVersion)
	}
	if exported.FileName == "" {
		t.Fatalf("expected export filename")
	}
	if exported.Bundle["sandbox"] == nil {
		t.Fatalf("expected sandbox bundle data")
	}
	if exported.Bundle["runtime_configuration"] == nil {
		t.Fatalf("expected runtime configuration snapshot")
	}
	if exported.Bundle["lifecycle"] == nil {
		t.Fatalf("expected lifecycle snapshot")
	}
	if exported.Bundle["cli"] == nil {
		t.Fatalf("expected cli handoff data")
	}
	if exported.Bundle["public_bundle_catalog"] == nil {
		t.Fatalf("expected public bundle catalog snapshot")
	}
	if !containsString(exported.Includes, "runtime_configuration") {
		t.Fatalf("expected runtime configuration include, got %#v", exported.Includes)
	}
	if !containsString(exported.Omissions, "runtime_database_dump") {
		t.Fatalf("expected runtime database omission, got %#v", exported.Omissions)
	}
}

func TestSandboxServiceReapExpiredSandboxes(t *testing.T) {
	t.Parallel()

	store := newSandboxMemoryStore()
	provisioner := &sandboxRecordingProvisioner{}
	service := NewSandboxService(
		store,
		SandboxServiceConfig{
			PublicBaseURL: "https://movebigrocks.com",
			RuntimeDomain: "movebigrocks.io",
			TrialTTL:      5 * 24 * time.Hour,
			ExtensionTTL:  30 * 24 * time.Hour,
		},
		WithSandboxProvisioner(provisioner),
	)
	result, err := service.CreateSandbox(t.Context(), SandboxCreateParams{
		Email: "ops@example.com",
		Name:  "Trial Sandbox",
	})
	if err != nil {
		t.Fatalf("create sandbox: %v", err)
	}

	sandbox, err := service.GetSandbox(t.Context(), result.Sandbox.ID, result.ManageToken)
	if err != nil {
		t.Fatalf("load sandbox: %v", err)
	}
	past := time.Now().UTC().Add(-2 * time.Hour)
	sandbox.ExpiresAt = &past
	if err := store.UpdateSandbox(t.Context(), sandbox); err != nil {
		t.Fatalf("update sandbox expiry: %v", err)
	}

	reaped, err := service.ReapExpiredSandboxes(t.Context(), time.Now().UTC())
	if err != nil {
		t.Fatalf("reap expired sandboxes: %v", err)
	}
	if len(reaped) != 1 {
		t.Fatalf("expected one reaped sandbox, got %d", len(reaped))
	}
	if reaped[0].Status != platformdomain.SandboxStatusExpired {
		t.Fatalf("unexpected reaped status %q", reaped[0].Status)
	}
	if reaped[0].ExpiredAt == nil {
		t.Fatalf("expected expired timestamp")
	}
	if reaped[0].LastError != "sandbox trial expired" {
		t.Fatalf("unexpected expiry reason %q", reaped[0].LastError)
	}
	if provisioner.destroyCalls != 1 {
		t.Fatalf("expected one runtime destroy call, got %d", provisioner.destroyCalls)
	}

	exported, err := service.ExportSandbox(t.Context(), result.Sandbox.ID, result.ManageToken)
	if err != nil {
		t.Fatalf("export expired sandbox: %v", err)
	}
	lifecycle, ok := exported.Bundle["lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("expected lifecycle export payload, got %#v", exported.Bundle["lifecycle"])
	}
	if lifecycle["status"] != platformdomain.SandboxStatusExpired {
		t.Fatalf("expected expired lifecycle status, got %#v", lifecycle["status"])
	}
}

func cloneSandbox(sandbox *platformdomain.Sandbox) *platformdomain.Sandbox {
	if sandbox == nil {
		return nil
	}
	clone := *sandbox
	if sandbox.VerifiedAt != nil {
		verifiedAt := *sandbox.VerifiedAt
		clone.VerifiedAt = &verifiedAt
	}
	if sandbox.ExpiresAt != nil {
		expiresAt := *sandbox.ExpiresAt
		clone.ExpiresAt = &expiresAt
	}
	if sandbox.ExpiredAt != nil {
		expiredAt := *sandbox.ExpiredAt
		clone.ExpiredAt = &expiredAt
	}
	if sandbox.ExtendedAt != nil {
		extendedAt := *sandbox.ExtendedAt
		clone.ExtendedAt = &extendedAt
	}
	if sandbox.DestroyedAt != nil {
		destroyedAt := *sandbox.DestroyedAt
		clone.DestroyedAt = &destroyedAt
	}
	return &clone
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
