package platformservices

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

type SandboxCreateParams struct {
	Email string
	Name  string
}

type SandboxCreateResult struct {
	Sandbox         *platformdomain.Sandbox
	ManageToken     string
	VerificationURL string
}

type SandboxProvisionResult struct {
	RuntimeURL   string
	LoginURL     string
	BootstrapURL string
}

type SandboxExportBundle struct {
	Includes  []string
	Omissions []string
	Data      map[string]any
}

type SandboxExportResult struct {
	ExportVersion string         `json:"export_version"`
	GeneratedAt   time.Time      `json:"generated_at"`
	FileName      string         `json:"file_name"`
	ContentType   string         `json:"content_type"`
	Includes      []string       `json:"includes"`
	Omissions     []string       `json:"omissions,omitempty"`
	Bundle        map[string]any `json:"bundle"`
}

type SandboxProvisioner interface {
	Provision(ctx context.Context, sandbox *platformdomain.Sandbox) (SandboxProvisionResult, error)
	Destroy(ctx context.Context, sandbox *platformdomain.Sandbox) error
	Export(ctx context.Context, sandbox *platformdomain.Sandbox) (SandboxExportBundle, error)
}

type SandboxServiceConfig struct {
	PublicBaseURL    string
	RuntimeDomain    string
	ActivationTTL    time.Duration
	TrialTTL         time.Duration
	ExtensionTTL     time.Duration
	VerificationPath string
}

type SandboxServiceOption func(*SandboxService)

type SandboxService struct {
	store       shared.SandboxStore
	config      SandboxServiceConfig
	provisioner SandboxProvisioner
}

type SandboxBootstrapPolicy struct {
	Available             bool
	ActivationWindowHours int
	DefaultTrialDays      int
	ExtensionDays         int
	VerificationPath      string
}

type URLSandboxProvisioner struct {
	RuntimeDomain string
}

func (p URLSandboxProvisioner) Provision(_ context.Context, sandbox *platformdomain.Sandbox) (SandboxProvisionResult, error) {
	runtimeDomain := strings.TrimSpace(p.RuntimeDomain)
	if runtimeDomain == "" {
		return SandboxProvisionResult{}, fmt.Errorf("runtime domain is required")
	}
	base := fmt.Sprintf("https://%s.%s", sandbox.Slug, runtimeDomain)
	return SandboxProvisionResult{
		RuntimeURL:   base,
		LoginURL:     base + "/login",
		BootstrapURL: base + "/.well-known/mbr-instance.json",
	}, nil
}

func (p URLSandboxProvisioner) Destroy(_ context.Context, _ *platformdomain.Sandbox) error {
	return nil
}

func (p URLSandboxProvisioner) Export(_ context.Context, sandbox *platformdomain.Sandbox) (SandboxExportBundle, error) {
	if sandbox == nil {
		return SandboxExportBundle{}, fmt.Errorf("sandbox is required")
	}
	return SandboxExportBundle{
		Includes: []string{
			"sandbox_metadata",
			"runtime_urls",
			"runtime_configuration",
			"promotion_handoff",
		},
		Omissions: []string{
			"runtime_database_dump",
			"runtime_secrets",
		},
		Data: map[string]any{
			"runtime": map[string]any{
				"url":           sandbox.RuntimeURL,
				"login_url":     sandbox.LoginURL,
				"bootstrap_url": sandbox.BootstrapURL,
			},
			"runtime_configuration": map[string]any{
				"runtime_domain":       strings.TrimSpace(p.RuntimeDomain),
				"bootstrap_discovery":  "well_known_json",
				"delivery_model":       "vendor_operated_preview",
				"teardown_on_expiry":   true,
				"export_before_delete": true,
			},
			"promotion": map[string]any{
				"recommended_path": "self_host",
				"notes": []string{
					"Use the sandbox export as a handoff bundle for a real instance repo or hosted trial close-out.",
					"Runtime URLs and configuration metadata are preserved here even after the preview runtime is torn down.",
				},
			},
		},
	}, nil
}

func WithSandboxProvisioner(provisioner SandboxProvisioner) SandboxServiceOption {
	return func(s *SandboxService) {
		if provisioner != nil {
			s.provisioner = provisioner
		}
	}
}

func NewSandboxService(store shared.SandboxStore, cfg SandboxServiceConfig, opts ...SandboxServiceOption) *SandboxService {
	if cfg.ActivationTTL <= 0 {
		cfg.ActivationTTL = 24 * time.Hour
	}
	if cfg.TrialTTL <= 0 {
		cfg.TrialTTL = 5 * 24 * time.Hour
	}
	if cfg.ExtensionTTL <= 0 {
		cfg.ExtensionTTL = 30 * 24 * time.Hour
	}
	if strings.TrimSpace(cfg.VerificationPath) == "" {
		cfg.VerificationPath = "/sandbox/verify"
	}
	svc := &SandboxService{
		store:  store,
		config: cfg,
	}
	if strings.TrimSpace(cfg.RuntimeDomain) != "" {
		svc.provisioner = URLSandboxProvisioner{RuntimeDomain: cfg.RuntimeDomain}
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func (s *SandboxService) BootstrapPolicy() SandboxBootstrapPolicy {
	if s == nil {
		return SandboxBootstrapPolicy{}
	}
	return SandboxBootstrapPolicy{
		Available:             s.provisioner != nil,
		ActivationWindowHours: int(s.config.ActivationTTL / time.Hour),
		DefaultTrialDays:      int(s.config.TrialTTL / (24 * time.Hour)),
		ExtensionDays:         int(s.config.ExtensionTTL / (24 * time.Hour)),
		VerificationPath:      s.config.VerificationPath,
	}
}

func (s *SandboxService) CreateSandbox(ctx context.Context, params SandboxCreateParams) (*SandboxCreateResult, error) {
	email := strings.ToLower(strings.TrimSpace(params.Email))
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if s.provisioner == nil {
		return nil, fmt.Errorf("sandbox provisioner is not configured")
	}
	verificationToken, verificationHash, err := platformdomain.GenerateSandboxToken("sbv")
	if err != nil {
		return nil, fmt.Errorf("generate verification token: %w", err)
	}
	manageToken, manageHash, err := platformdomain.GenerateSandboxToken("sbm")
	if err != nil {
		return nil, fmt.Errorf("generate manage token: %w", err)
	}
	now := time.Now().UTC()
	slug, err := s.generateUniqueSlug(ctx)
	if err != nil {
		return nil, err
	}
	sandbox := platformdomain.NewSandbox(slug, params.Name, email, verificationHash, manageHash, now, s.config.ActivationTTL)
	if err := s.store.CreateSandbox(ctx, sandbox); err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	if _, err := s.provisionSandbox(ctx, sandbox); err != nil {
		return nil, err
	}
	return &SandboxCreateResult{
		Sandbox:         sandbox,
		ManageToken:     manageToken,
		VerificationURL: s.verificationURL(verificationToken),
	}, nil
}

func (s *SandboxService) GetSandbox(ctx context.Context, sandboxID, manageToken string) (*platformdomain.Sandbox, error) {
	sandbox, err := s.store.GetSandbox(ctx, strings.TrimSpace(sandboxID))
	if err != nil {
		return nil, fmt.Errorf("load sandbox: %w", err)
	}
	if sandbox == nil {
		return nil, fmt.Errorf("sandbox not found")
	}
	if err := s.authorizeSandbox(sandbox, manageToken); err != nil {
		return nil, err
	}
	return sandbox, nil
}

func (s *SandboxService) VerifySandbox(ctx context.Context, verificationToken string) (*platformdomain.Sandbox, error) {
	tokenHash := platformdomain.HashSandboxToken(verificationToken)
	sandbox, err := s.store.GetSandboxByVerificationTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("load sandbox by verification token: %w", err)
	}
	if sandbox == nil {
		return nil, fmt.Errorf("sandbox not found")
	}
	switch sandbox.Status {
	case platformdomain.SandboxStatusReady, platformdomain.SandboxStatusProvisioning:
		return sandbox, nil
	case platformdomain.SandboxStatusExpired:
		return nil, fmt.Errorf("sandbox expired")
	case platformdomain.SandboxStatusDestroyed:
		return nil, fmt.Errorf("sandbox already destroyed")
	}
	return s.provisionSandbox(ctx, sandbox)
}

func (s *SandboxService) ExtendSandbox(ctx context.Context, sandboxID, manageToken string) (*platformdomain.Sandbox, error) {
	sandbox, err := s.GetSandbox(ctx, sandboxID, manageToken)
	if err != nil {
		return nil, err
	}
	if err := sandbox.Extend(time.Now().UTC(), s.config.ExtensionTTL); err != nil {
		return nil, err
	}
	if err := s.store.UpdateSandbox(ctx, sandbox); err != nil {
		return nil, fmt.Errorf("extend sandbox: %w", err)
	}
	return sandbox, nil
}

func (s *SandboxService) DestroySandbox(ctx context.Context, sandboxID, manageToken, reason string) (*platformdomain.Sandbox, error) {
	sandbox, err := s.GetSandbox(ctx, sandboxID, manageToken)
	if err != nil {
		return nil, err
	}
	if err := sandbox.Destroy(reason, time.Now().UTC()); err != nil {
		return nil, err
	}
	if s.provisioner != nil {
		if err := s.provisioner.Destroy(ctx, sandbox); err != nil {
			return nil, fmt.Errorf("destroy sandbox runtime: %w", err)
		}
	}
	if err := s.store.UpdateSandbox(ctx, sandbox); err != nil {
		return nil, fmt.Errorf("destroy sandbox: %w", err)
	}
	return sandbox, nil
}

func (s *SandboxService) ReapExpiredSandboxes(ctx context.Context, now time.Time) ([]*platformdomain.Sandbox, error) {
	if s == nil {
		return nil, fmt.Errorf("sandbox service is required")
	}
	if s.store == nil {
		return nil, fmt.Errorf("sandbox store is not configured")
	}
	candidates, err := s.store.ListReapableSandboxes(ctx, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("list reapable sandboxes: %w", err)
	}
	reaped := make([]*platformdomain.Sandbox, 0, len(candidates))
	for _, sandbox := range candidates {
		if sandbox == nil {
			continue
		}
		due, reason := sandbox.ExpiryDue(now)
		if !due {
			continue
		}
		if strings.TrimSpace(sandbox.RuntimeURL) != "" && s.provisioner != nil {
			if err := s.provisioner.Destroy(ctx, sandbox); err != nil {
				return nil, fmt.Errorf("destroy expired sandbox runtime %s: %w", sandbox.ID, err)
			}
		}
		if err := sandbox.MarkExpired(reason, now); err != nil {
			return nil, fmt.Errorf("expire sandbox %s: %w", sandbox.ID, err)
		}
		if err := s.store.UpdateSandbox(ctx, sandbox); err != nil {
			return nil, fmt.Errorf("persist expired sandbox %s: %w", sandbox.ID, err)
		}
		reaped = append(reaped, sandbox)
	}
	return reaped, nil
}

func (s *SandboxService) ExportSandbox(ctx context.Context, sandboxID, manageToken string) (*SandboxExportResult, error) {
	sandbox, err := s.GetSandbox(ctx, sandboxID, manageToken)
	if err != nil {
		return nil, err
	}

	generatedAt := time.Now().UTC()
	fileName := fmt.Sprintf("mbr-sandbox-%s-export.json", sandbox.Slug)
	bundle := SandboxExportBundle{
		Includes: []string{"sandbox_metadata"},
		Data:     map[string]any{},
	}
	if s.provisioner != nil {
		exported, err := s.provisioner.Export(ctx, sandbox)
		if err != nil {
			return nil, fmt.Errorf("export sandbox: %w", err)
		}
		bundle = exported
	}

	if bundle.Data == nil {
		bundle.Data = map[string]any{}
	}
	ensureSandboxExportIncludes(&bundle, "sandbox_metadata", "runtime_configuration", "lifecycle_snapshot", "cli_handoff", "public_bundle_catalog", "promotion_handoff")
	ensureSandboxExportOmissions(&bundle, "runtime_database_dump", "runtime_secrets")
	mergeSandboxExportMap(bundle.Data, "runtime_configuration", map[string]any{
		"activation_window_hours": s.BootstrapPolicy().ActivationWindowHours,
		"default_trial_days":      s.BootstrapPolicy().DefaultTrialDays,
		"extension_days":          s.BootstrapPolicy().ExtensionDays,
		"verification_path":       s.BootstrapPolicy().VerificationPath,
		"status":                  sandbox.Status,
		"expired_at":              sandbox.ExpiredAt,
		"destroyed_at":            sandbox.DestroyedAt,
	})
	bundle.Data["lifecycle"] = map[string]any{
		"status":                 sandbox.Status,
		"activation_deadline_at": sandbox.ActivationDeadlineAt,
		"verified_at":            sandbox.VerifiedAt,
		"expires_at":             sandbox.ExpiresAt,
		"expired_at":             sandbox.ExpiredAt,
		"extended_at":            sandbox.ExtendedAt,
		"destroyed_at":           sandbox.DestroyedAt,
		"last_error":             sandbox.LastError,
	}
	bundle.Data["cli"] = map[string]any{
		"login_command": fmt.Sprintf("mbr auth login --url %s", sandbox.RuntimeURL),
		"show_url":      sandboxManageURL(s.config.PublicBaseURL, sandbox.ID),
		"extend_url":    sandboxActionURL(s.config.PublicBaseURL, sandbox.ID, "extend"),
		"export_url":    sandboxActionURL(s.config.PublicBaseURL, sandbox.ID, "export"),
		"destroy_url":   sandboxManageURL(s.config.PublicBaseURL, sandbox.ID),
	}
	bundle.Data["public_bundle_catalog"] = []map[string]any{
		{"slug": "ats", "channel": "launch"},
		{"slug": "error-tracking", "channel": "launch"},
		{"slug": "web-analytics", "channel": "launch"},
		{"slug": "sales-pipeline", "channel": "beta"},
		{"slug": "community-feature-requests", "channel": "beta"},
	}
	bundle.Data["sandbox"] = map[string]any{
		"id":                     sandbox.ID,
		"slug":                   sandbox.Slug,
		"name":                   sandbox.Name,
		"requested_email":        sandbox.RequestedEmail,
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
		"created_at":             sandbox.CreatedAt,
		"updated_at":             sandbox.UpdatedAt,
	}

	return &SandboxExportResult{
		ExportVersion: "mbr-sandbox-export-v1",
		GeneratedAt:   generatedAt,
		FileName:      fileName,
		ContentType:   "application/json",
		Includes:      bundle.Includes,
		Omissions:     bundle.Omissions,
		Bundle:        bundle.Data,
	}, nil
}

func (s *SandboxService) verificationURL(token string) string {
	base := strings.TrimRight(strings.TrimSpace(s.config.PublicBaseURL), "/")
	if base == "" {
		return s.config.VerificationPath + "?token=" + token
	}
	return base + s.config.VerificationPath + "?token=" + token
}

func (s *SandboxService) authorizeSandbox(sandbox *platformdomain.Sandbox, manageToken string) error {
	if sandbox == nil {
		return fmt.Errorf("sandbox not found")
	}
	if strings.TrimSpace(manageToken) == "" {
		return fmt.Errorf("sandbox manage token is required")
	}
	if platformdomain.HashSandboxToken(manageToken) != sandbox.ManageTokenHash {
		return fmt.Errorf("sandbox not found")
	}
	return nil
}

func (s *SandboxService) generateUniqueSlug(ctx context.Context) (string, error) {
	for i := 0; i < 16; i++ {
		slug := randomSandboxSlug()
		existing, err := s.store.GetSandboxBySlug(ctx, slug)
		if err != nil {
			return "", fmt.Errorf("check sandbox slug: %w", err)
		}
		if existing == nil {
			return slug, nil
		}
	}
	return "", fmt.Errorf("failed to allocate unique sandbox slug")
}

func (s *SandboxService) provisionSandbox(ctx context.Context, sandbox *platformdomain.Sandbox) (*platformdomain.Sandbox, error) {
	if sandbox == nil {
		return nil, fmt.Errorf("sandbox is required")
	}
	if s.provisioner == nil {
		return nil, fmt.Errorf("sandbox provisioner is not configured")
	}
	if sandbox.Status == platformdomain.SandboxStatusReady || sandbox.Status == platformdomain.SandboxStatusProvisioning {
		return sandbox, nil
	}
	if sandbox.Status == platformdomain.SandboxStatusExpired {
		return nil, fmt.Errorf("sandbox expired")
	}
	if sandbox.Status == platformdomain.SandboxStatusDestroyed {
		return nil, fmt.Errorf("sandbox already destroyed")
	}

	now := time.Now().UTC()
	if err := sandbox.MarkProvisioning(now); err != nil {
		return nil, err
	}
	if err := s.store.UpdateSandbox(ctx, sandbox); err != nil {
		return nil, fmt.Errorf("mark sandbox provisioning: %w", err)
	}

	provisioned, err := s.provisioner.Provision(ctx, sandbox)
	if err != nil {
		if markErr := sandbox.MarkFailed(err.Error(), now); markErr != nil {
			return nil, markErr
		}
		if updateErr := s.store.UpdateSandbox(ctx, sandbox); updateErr != nil {
			return nil, fmt.Errorf("update failed sandbox: %w", updateErr)
		}
		return sandbox, nil
	}
	if err := sandbox.MarkReady(provisioned.RuntimeURL, provisioned.LoginURL, provisioned.BootstrapURL, now, s.config.TrialTTL); err != nil {
		return nil, err
	}
	if err := s.store.UpdateSandbox(ctx, sandbox); err != nil {
		return nil, fmt.Errorf("mark sandbox ready: %w", err)
	}
	return sandbox, nil
}

var sandboxAdjectives = []string{
	"amber", "brisk", "calm", "clear", "ember", "focal", "magic", "steady",
}

var sandboxNouns = []string{
	"badger", "canvas", "dumpling", "harbor", "lantern", "meadow", "rocket", "signal",
}

func randomSandboxSlug() string {
	adj := sandboxAdjectives[randomIndex(len(sandboxAdjectives))]
	noun := sandboxNouns[randomIndex(len(sandboxNouns))]
	number := 10 + randomIndex(90)
	return fmt.Sprintf("%s-%s-%d", adj, noun, number)
}

func ensureSandboxExportIncludes(bundle *SandboxExportBundle, values ...string) {
	if bundle == nil {
		return
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(bundle.Includes, value) {
			continue
		}
		bundle.Includes = append(bundle.Includes, value)
	}
}

func ensureSandboxExportOmissions(bundle *SandboxExportBundle, values ...string) {
	if bundle == nil {
		return
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(bundle.Omissions, value) {
			continue
		}
		bundle.Omissions = append(bundle.Omissions, value)
	}
}

func mergeSandboxExportMap(data map[string]any, key string, values map[string]any) {
	if data == nil {
		return
	}
	existing, _ := data[key].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}
	for mapKey, value := range values {
		existing[mapKey] = value
	}
	data[key] = existing
}

func sandboxManageURL(baseURL, sandboxID string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return "/api/public/sandboxes/" + strings.TrimSpace(sandboxID)
	}
	return base + "/api/public/sandboxes/" + strings.TrimSpace(sandboxID)
}

func sandboxActionURL(baseURL, sandboxID, action string) string {
	base := sandboxManageURL(baseURL, sandboxID)
	action = strings.Trim(strings.TrimSpace(action), "/")
	if action == "" {
		return base
	}
	return base + "/" + action
}

func randomIndex(length int) int {
	if length <= 1 {
		return 0
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return int(time.Now().UnixNano() % int64(length))
	}
	return int(binary.BigEndian.Uint64(buf[:]) % uint64(length))
}
