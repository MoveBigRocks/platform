package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
)

func main() {
	var (
		outPath   string
		version   string
		gitSHA    string
		buildDate string
	)

	flag.StringVar(&outPath, "out", "", "path to write the sandbox lifecycle proof artifact")
	flag.StringVar(&version, "version", "", "runtime version string")
	flag.StringVar(&gitSHA, "git-sha", "", "runtime git commit")
	flag.StringVar(&buildDate, "build-date", "", "runtime build date")
	flag.Parse()

	if outPath == "" {
		fmt.Fprintln(os.Stderr, "--out is required")
		os.Exit(2)
	}

	store := newProofSandboxStore()
	provisioner := &proofProvisioner{
		urlProvisioner: platformservices.URLSandboxProvisioner{RuntimeDomain: "movebigrocks.io"},
	}
	service := platformservices.NewSandboxService(
		store,
		platformservices.SandboxServiceConfig{
			PublicBaseURL: "https://movebigrocks.com",
			RuntimeDomain: "movebigrocks.io",
			TrialTTL:      5 * 24 * time.Hour,
			ExtensionTTL:  30 * 24 * time.Hour,
		},
		platformservices.WithSandboxProvisioner(provisioner),
	)

	ctx := context.Background()
	created, err := service.CreateSandbox(ctx, platformservices.SandboxCreateParams{
		Email: "ops@example.com",
		Name:  "Milestone Proof Sandbox",
	})
	if err != nil {
		failf("create sandbox: %v", err)
	}

	exportBeforeExpiry, err := service.ExportSandbox(ctx, created.Sandbox.ID, created.ManageToken)
	if err != nil {
		failf("export sandbox before expiry: %v", err)
	}

	sandbox, err := service.GetSandbox(ctx, created.Sandbox.ID, created.ManageToken)
	if err != nil {
		failf("load sandbox: %v", err)
	}
	reapAt := time.Now().UTC()
	expiredAt := reapAt.Add(-2 * time.Hour)
	sandbox.ExpiresAt = &expiredAt
	if err := store.UpdateSandbox(ctx, sandbox); err != nil {
		failf("persist forced expiry: %v", err)
	}

	reaped, err := service.ReapExpiredSandboxes(ctx, reapAt)
	if err != nil {
		failf("reap expired sandboxes: %v", err)
	}
	if len(reaped) != 1 {
		failf("expected one reaped sandbox, got %d", len(reaped))
	}

	expiredSandbox, err := service.GetSandbox(ctx, created.Sandbox.ID, created.ManageToken)
	if err != nil {
		failf("reload expired sandbox: %v", err)
	}
	exportAfterExpiry, err := service.ExportSandbox(ctx, created.Sandbox.ID, created.ManageToken)
	if err != nil {
		failf("export sandbox after expiry: %v", err)
	}

	destroyed, err := service.DestroySandbox(ctx, created.Sandbox.ID, created.ManageToken, "proof complete")
	if err != nil {
		failf("destroy sandbox: %v", err)
	}

	artifact := map[string]any{
		"generated_at":            time.Now().UTC().Format(time.RFC3339),
		"version":                 version,
		"git_sha":                 gitSHA,
		"build_date":              buildDate,
		"manage_token_redacted":   redactToken(created.ManageToken),
		"provision_runtime_calls": provisioner.provisionCalls,
		"destroy_runtime_calls":   provisioner.destroyCalls,
		"created":                 summarizeSandbox(created.Sandbox),
		"export_before_expiry": map[string]any{
			"includes":  exportBeforeExpiry.Includes,
			"omissions": exportBeforeExpiry.Omissions,
			"bundle":    exportBeforeExpiry.Bundle,
		},
		"reaped": map[string]any{
			"count":     len(reaped),
			"sandboxes": summarizeSandboxes(reaped),
		},
		"expired": summarizeSandbox(expiredSandbox),
		"export_after_expiry": map[string]any{
			"includes":  exportAfterExpiry.Includes,
			"omissions": exportAfterExpiry.Omissions,
			"bundle":    exportAfterExpiry.Bundle,
		},
		"destroyed": summarizeSandbox(destroyed),
	}

	encoded, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		failf("marshal proof artifact: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		failf("create output directory: %v", err)
	}
	if err := os.WriteFile(outPath, append(encoded, '\n'), 0o644); err != nil {
		failf("write proof artifact: %v", err)
	}

	fmt.Printf("wrote %s\n", outPath)
}

type proofSandboxStore struct {
	byID                 map[string]*platformdomain.Sandbox
	bySlug               map[string]string
	byVerificationHashes map[string]string
}

func newProofSandboxStore() *proofSandboxStore {
	return &proofSandboxStore{
		byID:                 map[string]*platformdomain.Sandbox{},
		bySlug:               map[string]string{},
		byVerificationHashes: map[string]string{},
	}
}

func (s *proofSandboxStore) CreateSandbox(_ context.Context, sandbox *platformdomain.Sandbox) error {
	if sandbox == nil {
		return errors.New("sandbox is required")
	}
	if sandbox.ID == "" {
		sandbox.ID = "sbx_proof"
	}
	s.byID[sandbox.ID] = cloneSandbox(sandbox)
	s.bySlug[sandbox.Slug] = sandbox.ID
	s.byVerificationHashes[sandbox.VerificationTokenHash] = sandbox.ID
	return nil
}

func (s *proofSandboxStore) GetSandbox(_ context.Context, sandboxID string) (*platformdomain.Sandbox, error) {
	if sandbox, ok := s.byID[sandboxID]; ok {
		return cloneSandbox(sandbox), nil
	}
	return nil, nil
}

func (s *proofSandboxStore) GetSandboxBySlug(_ context.Context, slug string) (*platformdomain.Sandbox, error) {
	if sandboxID, ok := s.bySlug[slug]; ok {
		return s.GetSandbox(context.Background(), sandboxID)
	}
	return nil, nil
}

func (s *proofSandboxStore) GetSandboxByVerificationTokenHash(_ context.Context, tokenHash string) (*platformdomain.Sandbox, error) {
	if sandboxID, ok := s.byVerificationHashes[tokenHash]; ok {
		return s.GetSandbox(context.Background(), sandboxID)
	}
	return nil, nil
}

func (s *proofSandboxStore) ListReapableSandboxes(_ context.Context, now time.Time) ([]*platformdomain.Sandbox, error) {
	reapable := make([]*platformdomain.Sandbox, 0)
	for _, sandbox := range s.byID {
		if due, _ := sandbox.ExpiryDue(now); due {
			reapable = append(reapable, cloneSandbox(sandbox))
		}
	}
	return reapable, nil
}

func (s *proofSandboxStore) UpdateSandbox(_ context.Context, sandbox *platformdomain.Sandbox) error {
	if sandbox == nil {
		return errors.New("sandbox is required")
	}
	s.byID[sandbox.ID] = cloneSandbox(sandbox)
	s.bySlug[sandbox.Slug] = sandbox.ID
	s.byVerificationHashes[sandbox.VerificationTokenHash] = sandbox.ID
	return nil
}

type proofProvisioner struct {
	urlProvisioner platformservices.URLSandboxProvisioner
	provisionCalls int
	destroyCalls   int
}

func (p *proofProvisioner) Provision(ctx context.Context, sandbox *platformdomain.Sandbox) (platformservices.SandboxProvisionResult, error) {
	p.provisionCalls++
	return p.urlProvisioner.Provision(ctx, sandbox)
}

func (p *proofProvisioner) Destroy(ctx context.Context, sandbox *platformdomain.Sandbox) error {
	p.destroyCalls++
	return p.urlProvisioner.Destroy(ctx, sandbox)
}

func (p *proofProvisioner) Export(ctx context.Context, sandbox *platformdomain.Sandbox) (platformservices.SandboxExportBundle, error) {
	return p.urlProvisioner.Export(ctx, sandbox)
}

func summarizeSandboxes(sandboxes []*platformdomain.Sandbox) []map[string]any {
	summaries := make([]map[string]any, 0, len(sandboxes))
	for _, sandbox := range sandboxes {
		summaries = append(summaries, summarizeSandbox(sandbox))
	}
	return summaries
}

func summarizeSandbox(sandbox *platformdomain.Sandbox) map[string]any {
	if sandbox == nil {
		return nil
	}
	return map[string]any{
		"id":                     sandbox.ID,
		"slug":                   sandbox.Slug,
		"status":                 sandbox.Status,
		"runtime_url":            sandbox.RuntimeURL,
		"login_url":              sandbox.LoginURL,
		"bootstrap_url":          sandbox.BootstrapURL,
		"activation_deadline_at": sandbox.ActivationDeadlineAt,
		"verified_at":            sandbox.VerifiedAt,
		"expires_at":             sandbox.ExpiresAt,
		"expired_at":             sandbox.ExpiredAt,
		"extended_at":            sandbox.ExtendedAt,
		"destroyed_at":           sandbox.DestroyedAt,
		"last_error":             sandbox.LastError,
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

func redactToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:8] + "...redacted"
}

func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
