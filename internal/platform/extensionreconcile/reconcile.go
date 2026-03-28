package extensionreconcile

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	domain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/platform/extensionbundle"
	"github.com/movebigrocks/platform/internal/platform/extensiondesiredstate"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	publicruntime "github.com/movebigrocks/platform/pkg/extensionsruntime"
)

type BundleLoader interface {
	ReadSource(ctx context.Context, source, licenseToken string) (extensionbundle.SourcePayload, error)
}

type Inventory interface {
	ListAllExtensions(ctx context.Context) ([]*domain.InstalledExtension, error)
}

type WorkspaceLookup interface {
	GetWorkspaceBySlug(ctx context.Context, slug string) (*domain.Workspace, error)
}

type Lifecycle interface {
	InstallExtension(ctx context.Context, params platformservices.InstallExtensionParams) (*domain.InstalledExtension, error)
	UpgradeExtension(ctx context.Context, params platformservices.UpgradeExtensionParams) (*domain.InstalledExtension, error)
	UpdateExtensionConfig(ctx context.Context, extensionID string, config map[string]any) (*domain.InstalledExtension, error)
	ValidateExtension(ctx context.Context, extensionID string) (*domain.InstalledExtension, error)
	ActivateExtension(ctx context.Context, extensionID string) (*domain.InstalledExtension, error)
	DeactivateExtension(ctx context.Context, extensionID string, reason string) (*domain.InstalledExtension, error)
	UninstallExtension(ctx context.Context, extensionID string) error
	CheckExtensionHealth(ctx context.Context, extensionID string) (*domain.InstalledExtension, error)
}

type Engine struct {
	Bundles    BundleLoader
	Inventory  Inventory
	Workspaces WorkspaceLookup
	Lifecycle  Lifecycle
	Actor      string
	EnvLookup  func(string) (string, bool)
}

type OperationKind string

const (
	OperationInstall    OperationKind = "install"
	OperationUpgrade    OperationKind = "upgrade"
	OperationConfigure  OperationKind = "configure"
	OperationValidate   OperationKind = "validate"
	OperationActivate   OperationKind = "activate"
	OperationDeactivate OperationKind = "deactivate"
	OperationUninstall  OperationKind = "uninstall"
	OperationNoop       OperationKind = "noop"
	OperationDrift      OperationKind = "drift"
)

type Operation struct {
	Action          OperationKind `json:"action"`
	Slug            string        `json:"slug"`
	Scope           string        `json:"scope"`
	WorkspaceSlug   string        `json:"workspaceSlug,omitempty"`
	WorkspaceID     string        `json:"workspaceID,omitempty"`
	ExtensionID     string        `json:"extensionID,omitempty"`
	State           string        `json:"state"`
	Ref             string        `json:"ref,omitempty"`
	Reason          string        `json:"reason,omitempty"`
	ExistingVersion string        `json:"existingVersion,omitempty"`
	DesiredVersion  string        `json:"desiredVersion,omitempty"`
	DesiredActive   bool          `json:"desiredActive"`
	ConfigKeys      []string      `json:"configKeys,omitempty"`
}

type RuntimeManifest struct {
	Runtimes []RuntimeManifestEntry `json:"runtimes"`
}

type RuntimeManifestEntry struct {
	Slug       string `json:"slug"`
	PackageKey string `json:"packageKey"`
	Artifact   string `json:"artifact"`
	Binary     string `json:"binary"`
	Service    string `json:"service"`
	Socket     string `json:"socket"`
}

type Plan struct {
	Actor            string          `json:"actor"`
	DesiredStatePath string          `json:"desiredStatePath,omitempty"`
	GeneratedAt      time.Time       `json:"generatedAt"`
	RuntimeManifest  RuntimeManifest `json:"runtimeManifest"`
	Operations       []Operation     `json:"operations"`
}

type OperationResult struct {
	Action        OperationKind `json:"action"`
	Slug          string        `json:"slug"`
	Scope         string        `json:"scope"`
	WorkspaceSlug string        `json:"workspaceSlug,omitempty"`
	ExtensionID   string        `json:"extensionID,omitempty"`
	Status        string        `json:"status"`
	Message       string        `json:"message,omitempty"`
}

type CheckResult struct {
	Actor            string          `json:"actor"`
	DesiredStatePath string          `json:"desiredStatePath,omitempty"`
	CheckedAt        time.Time       `json:"checkedAt"`
	RuntimeManifest  RuntimeManifest `json:"runtimeManifest"`
	Clean            bool            `json:"clean"`
	Issues           []Operation     `json:"issues"`
}

type ApplyResult struct {
	Actor            string            `json:"actor"`
	DesiredStatePath string            `json:"desiredStatePath,omitempty"`
	AppliedAt        time.Time         `json:"appliedAt"`
	RuntimeManifest  RuntimeManifest   `json:"runtimeManifest"`
	Results          []OperationResult `json:"results"`
	Clean            bool              `json:"clean"`
	Check            *CheckResult      `json:"check,omitempty"`
}

type DefaultBundleLoader struct {
	Config extensionbundle.ResolverConfig
}

func (l DefaultBundleLoader) ReadSource(ctx context.Context, source, licenseToken string) (extensionbundle.SourcePayload, error) {
	return extensionbundle.ReadSource(ctx, source, licenseToken, l.Config)
}

func NewEngine(bundles BundleLoader, inventory Inventory, workspaces WorkspaceLookup, lifecycle Lifecycle) *Engine {
	return &Engine{
		Bundles:    bundles,
		Inventory:  inventory,
		Workspaces: workspaces,
		Lifecycle:  lifecycle,
		Actor:      "system:reconcile-extensions",
		EnvLookup:  os.LookupEnv,
	}
}

func (e *Engine) Plan(ctx context.Context, doc extensiondesiredstate.Document, desiredStatePath string) (Plan, error) {
	eval, err := e.evaluate(ctx, doc, desiredStatePath)
	if err != nil {
		return Plan{}, err
	}
	return eval.plan, nil
}

func (e *Engine) RenderRuntimeManifest(ctx context.Context, doc extensiondesiredstate.Document) (RuntimeManifest, error) {
	eval, err := e.evaluate(ctx, doc, "")
	if err != nil {
		return RuntimeManifest{}, err
	}
	return eval.plan.RuntimeManifest, nil
}

func (e *Engine) Check(ctx context.Context, doc extensiondesiredstate.Document, desiredStatePath string) (CheckResult, error) {
	eval, err := e.evaluate(ctx, doc, desiredStatePath)
	if err != nil {
		return CheckResult{}, err
	}

	issues := make([]Operation, 0)
	for _, op := range eval.plan.Operations {
		if op.Action != OperationNoop {
			issues = append(issues, op)
		}
	}

	for _, desired := range eval.resolved {
		if desired.entry.DesiredState() != extensiondesiredstate.StatePresent || !desired.entry.DesiredActive() {
			continue
		}
		current := eval.current[desired.key]
		if current == nil {
			continue
		}
		checked, err := e.Lifecycle.CheckExtensionHealth(ctx, current.ID)
		if err != nil {
			issues = append(issues, Operation{
				Action:        OperationDrift,
				Slug:          desired.entry.Slug,
				Scope:         desired.entry.Scope,
				WorkspaceSlug: desired.entry.Workspace,
				WorkspaceID:   desired.workspaceID,
				ExtensionID:   current.ID,
				State:         desired.entry.DesiredState(),
				Reason:        fmt.Sprintf("health check failed: %v", err),
				DesiredActive: true,
			})
			continue
		}
		if checked.HealthStatus != domain.ExtensionHealthHealthy {
			issues = append(issues, Operation{
				Action:          OperationDrift,
				Slug:            desired.entry.Slug,
				Scope:           desired.entry.Scope,
				WorkspaceSlug:   desired.entry.Workspace,
				WorkspaceID:     desired.workspaceID,
				ExtensionID:     checked.ID,
				State:           desired.entry.DesiredState(),
				ExistingVersion: checked.Version,
				DesiredVersion:  desired.manifest.Version,
				Reason:          fmt.Sprintf("extension health is %s: %s", checked.HealthStatus, strings.TrimSpace(checked.HealthMessage)),
				DesiredActive:   true,
			})
		}
	}

	return CheckResult{
		Actor:            e.actor(),
		DesiredStatePath: desiredStatePath,
		CheckedAt:        time.Now().UTC(),
		RuntimeManifest:  eval.plan.RuntimeManifest,
		Clean:            len(issues) == 0,
		Issues:           issues,
	}, nil
}

func (e *Engine) Apply(ctx context.Context, doc extensiondesiredstate.Document, desiredStatePath string) (ApplyResult, error) {
	eval, err := e.evaluate(ctx, doc, desiredStatePath)
	if err != nil {
		return ApplyResult{}, err
	}

	result := ApplyResult{
		Actor:            e.actor(),
		DesiredStatePath: desiredStatePath,
		AppliedAt:        time.Now().UTC(),
		RuntimeManifest:  eval.plan.RuntimeManifest,
		Results:          make([]OperationResult, 0, len(eval.plan.Operations)),
	}

	current := make(map[string]*domain.InstalledExtension, len(eval.current))
	for key, value := range eval.current {
		current[key] = value
	}

	for _, op := range eval.plan.Operations {
		res := OperationResult{
			Action:        op.Action,
			Slug:          op.Slug,
			Scope:         op.Scope,
			WorkspaceSlug: op.WorkspaceSlug,
			ExtensionID:   op.ExtensionID,
			Status:        "skipped",
		}

		if op.Action == OperationNoop {
			res.Status = "noop"
			res.Message = op.Reason
			result.Results = append(result.Results, res)
			continue
		}
		if op.Action == OperationDrift {
			res.Status = "drift"
			res.Message = op.Reason
			result.Results = append(result.Results, res)
			continue
		}

		key := installKey(op.Scope, op.WorkspaceID, op.Slug)
		resolved, ok := eval.resolved[key]
		if !ok && op.Action != OperationUninstall && op.Action != OperationDeactivate {
			res.Status = "failed"
			res.Message = "missing resolved desired entry"
			result.Results = append(result.Results, res)
			continue
		}
		currentExt := current[key]

		switch op.Action {
		case OperationInstall:
			installed, installErr := e.Lifecycle.InstallExtension(ctx, platformservices.InstallExtensionParams{
				WorkspaceID:   resolved.workspaceID,
				InstalledByID: e.actor(),
				BundleBase64:  base64.StdEncoding.EncodeToString(resolved.payload.Bytes),
				Manifest:      resolved.manifest,
				Assets:        assetInputs(resolved.payload.Bundle.Assets),
				Migrations:    migrationInputs(resolved.payload.Bundle.Migrations),
			})
			if installErr != nil {
				res.Status = "failed"
				res.Message = installErr.Error()
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, installErr
			}
			current[key] = installed
			res.Status = "applied"
			res.ExtensionID = installed.ID
		case OperationUpgrade:
			if currentExt == nil {
				res.Status = "failed"
				res.Message = "current extension missing for upgrade"
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, fmt.Errorf("current extension missing for upgrade")
			}
			upgraded, upgradeErr := e.Lifecycle.UpgradeExtension(ctx, platformservices.UpgradeExtensionParams{
				ExtensionID:   currentExt.ID,
				InstalledByID: e.actor(),
				BundleBase64:  base64.StdEncoding.EncodeToString(resolved.payload.Bytes),
				Manifest:      resolved.manifest,
				Assets:        assetInputs(resolved.payload.Bundle.Assets),
				Migrations:    migrationInputs(resolved.payload.Bundle.Migrations),
			})
			if upgradeErr != nil {
				res.Status = "failed"
				res.Message = upgradeErr.Error()
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, upgradeErr
			}
			current[key] = upgraded
			res.Status = "applied"
			res.ExtensionID = upgraded.ID
		case OperationConfigure:
			if currentExt == nil {
				res.Status = "failed"
				res.Message = "current extension missing for configure"
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, fmt.Errorf("current extension missing for configure")
			}
			updated, updateErr := e.Lifecycle.UpdateExtensionConfig(ctx, currentExt.ID, resolved.config)
			if updateErr != nil {
				res.Status = "failed"
				res.Message = updateErr.Error()
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, updateErr
			}
			current[key] = updated
			res.Status = "applied"
			res.ExtensionID = updated.ID
		case OperationValidate:
			if currentExt == nil {
				res.Status = "failed"
				res.Message = "current extension missing for validate"
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, fmt.Errorf("current extension missing for validate")
			}
			validated, validateErr := e.Lifecycle.ValidateExtension(ctx, currentExt.ID)
			if validateErr != nil {
				res.Status = "failed"
				res.Message = validateErr.Error()
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, validateErr
			}
			current[key] = validated
			res.Status = "applied"
			res.ExtensionID = validated.ID
		case OperationActivate:
			if currentExt == nil {
				res.Status = "failed"
				res.Message = "current extension missing for activate"
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, fmt.Errorf("current extension missing for activate")
			}
			activated, activateErr := e.Lifecycle.ActivateExtension(ctx, currentExt.ID)
			if activateErr != nil {
				res.Status = "failed"
				res.Message = activateErr.Error()
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, activateErr
			}
			current[key] = activated
			res.Status = "applied"
			res.ExtensionID = activated.ID
		case OperationDeactivate:
			if currentExt == nil {
				res.Status = "failed"
				res.Message = "current extension missing for deactivate"
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, fmt.Errorf("current extension missing for deactivate")
			}
			deactivated, deactivateErr := e.Lifecycle.DeactivateExtension(ctx, currentExt.ID, op.Reason)
			if deactivateErr != nil {
				res.Status = "failed"
				res.Message = deactivateErr.Error()
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, deactivateErr
			}
			current[key] = deactivated
			res.Status = "applied"
			res.ExtensionID = deactivated.ID
		case OperationUninstall:
			if currentExt == nil {
				res.Status = "failed"
				res.Message = "current extension missing for uninstall"
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, fmt.Errorf("current extension missing for uninstall")
			}
			uninstallErr := e.Lifecycle.UninstallExtension(ctx, currentExt.ID)
			if uninstallErr != nil {
				res.Status = "failed"
				res.Message = uninstallErr.Error()
				result.Results = append(result.Results, res)
				result.Clean = false
				check, _ := e.Check(ctx, doc, desiredStatePath)
				result.Check = &check
				return result, uninstallErr
			}
			delete(current, key)
			res.Status = "applied"
			res.ExtensionID = currentExt.ID
		}

		result.Results = append(result.Results, res)
	}

	check, checkErr := e.Check(ctx, doc, desiredStatePath)
	if checkErr != nil {
		result.Clean = false
		return result, checkErr
	}
	result.Check = &check
	result.Clean = check.Clean
	return result, nil
}

type evaluation struct {
	plan     Plan
	resolved map[string]resolvedEntry
	current  map[string]*domain.InstalledExtension
}

type resolvedEntry struct {
	entry       extensiondesiredstate.InstalledEntry
	key         string
	workspaceID string
	config      map[string]any
	payload     extensionbundle.SourcePayload
	manifest    domain.ExtensionManifest
}

func (e *Engine) evaluate(ctx context.Context, doc extensiondesiredstate.Document, desiredStatePath string) (evaluation, error) {
	if e == nil {
		return evaluation{}, fmt.Errorf("reconciliation engine is required")
	}
	if e.Inventory == nil {
		return evaluation{}, fmt.Errorf("extension inventory is required")
	}
	if e.Workspaces == nil {
		return evaluation{}, fmt.Errorf("workspace lookup is required")
	}
	if e.Bundles == nil {
		return evaluation{}, fmt.Errorf("bundle loader is required")
	}
	if e.Lifecycle == nil {
		return evaluation{}, fmt.Errorf("extension lifecycle is required")
	}

	resolved, runtimeManifest, err := e.resolveDesiredEntries(ctx, doc)
	if err != nil {
		return evaluation{}, err
	}
	installed, err := e.Inventory.ListAllExtensions(ctx)
	if err != nil {
		return evaluation{}, fmt.Errorf("list installed extensions: %w", err)
	}

	current := make(map[string]*domain.InstalledExtension, len(installed))
	for _, ext := range installed {
		if ext == nil {
			continue
		}
		key := installKey(string(ext.Manifest.Scope), ext.WorkspaceID, ext.Slug)
		current[key] = ext
	}

	ops := make([]Operation, 0)
	matched := make(map[string]struct{}, len(resolved))
	keys := make([]string, 0, len(resolved))
	for key := range resolved {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		desired := resolved[key]
		existing := current[key]
		matched[key] = struct{}{}
		ops = append(ops, e.planForEntry(desired, existing)...)
	}

	unmatched := make([]string, 0)
	for key := range current {
		if _, ok := matched[key]; ok {
			continue
		}
		unmatched = append(unmatched, key)
	}
	sort.Strings(unmatched)
	for _, key := range unmatched {
		existing := current[key]
		if existing == nil {
			continue
		}
		ops = append(ops, Operation{
			Action:          OperationDrift,
			Slug:            existing.Slug,
			Scope:           string(existing.Manifest.Scope),
			WorkspaceSlug:   workspaceSlugForInstalled(existing),
			WorkspaceID:     existing.WorkspaceID,
			ExtensionID:     existing.ID,
			State:           extensiondesiredstate.StatePresent,
			ExistingVersion: existing.Version,
			DesiredActive:   existing.Status == domain.ExtensionStatusActive,
			Reason:          "installed extension is not declared in extensions.installed",
		})
	}

	return evaluation{
		plan: Plan{
			Actor:            e.actor(),
			DesiredStatePath: desiredStatePath,
			GeneratedAt:      time.Now().UTC(),
			RuntimeManifest:  runtimeManifest,
			Operations:       ops,
		},
		resolved: resolved,
		current:  current,
	}, nil
}

func (e *Engine) resolveDesiredEntries(ctx context.Context, doc extensiondesiredstate.Document) (map[string]resolvedEntry, RuntimeManifest, error) {
	resolved := make(map[string]resolvedEntry, len(doc.Extensions.Installed))
	runtimes := make([]RuntimeManifestEntry, 0)

	for _, entry := range doc.Extensions.Installed {
		workspaceID := ""
		if entry.Scope == string(domain.ExtensionScopeWorkspace) {
			workspace, err := e.Workspaces.GetWorkspaceBySlug(ctx, entry.Workspace)
			if err != nil || workspace == nil {
				return nil, RuntimeManifest{}, fmt.Errorf("resolve workspace %q for extension %s: %w", entry.Workspace, entry.Slug, err)
			}
			workspaceID = workspace.ID
		}
		config, err := entry.ResolveConfig(e.envLookup())
		if err != nil {
			return nil, RuntimeManifest{}, err
		}

		resolvedEntry := resolvedEntry{
			entry:       entry,
			key:         installKey(entry.Scope, workspaceID, entry.Slug),
			workspaceID: workspaceID,
			config:      config,
		}

		if entry.DesiredState() == extensiondesiredstate.StatePresent {
			payload, readErr := e.Bundles.ReadSource(ctx, entry.Ref, "")
			if readErr != nil {
				return nil, RuntimeManifest{}, fmt.Errorf("load desired bundle for %s: %w", entry.Slug, readErr)
			}
			manifest, decodeErr := extensionbundle.DecodeManifest(payload.Bundle.Manifest)
			if decodeErr != nil {
				return nil, RuntimeManifest{}, fmt.Errorf("decode desired manifest for %s: %w", entry.Slug, decodeErr)
			}
			if !strings.EqualFold(manifest.Slug, entry.Slug) {
				return nil, RuntimeManifest{}, fmt.Errorf("desired entry %s resolved bundle slug %s", entry.Slug, manifest.Slug)
			}
			if manifest.Scope != domain.ExtensionScope(entry.Scope) {
				return nil, RuntimeManifest{}, fmt.Errorf("desired entry %s scope %s does not match bundle scope %s", entry.Slug, entry.Scope, manifest.Scope)
			}
			if entry.Publisher != "" && !strings.EqualFold(manifest.Publisher, entry.Publisher) {
				return nil, RuntimeManifest{}, fmt.Errorf("desired entry %s publisher %s does not match bundle publisher %s", entry.Slug, entry.Publisher, manifest.Publisher)
			}
			resolvedEntry.payload = payload
			resolvedEntry.manifest = manifest
			runtimeEntry, ok, runtimeErr := runtimeEntryForManifest(manifest)
			if runtimeErr != nil {
				return nil, RuntimeManifest{}, runtimeErr
			}
			if ok {
				runtimes = append(runtimes, runtimeEntry)
			}
		}

		resolved[resolvedEntry.key] = resolvedEntry
	}

	sort.Slice(runtimes, func(i, j int) bool {
		if runtimes[i].Slug == runtimes[j].Slug {
			return runtimes[i].PackageKey < runtimes[j].PackageKey
		}
		return runtimes[i].Slug < runtimes[j].Slug
	})
	return resolved, RuntimeManifest{Runtimes: runtimes}, nil
}

func (e *Engine) planForEntry(desired resolvedEntry, existing *domain.InstalledExtension) []Operation {
	configKeys := sortedKeys(desired.config)
	base := Operation{
		Slug:          desired.entry.Slug,
		Scope:         desired.entry.Scope,
		WorkspaceSlug: desired.entry.Workspace,
		WorkspaceID:   desired.workspaceID,
		State:         desired.entry.DesiredState(),
		Ref:           desired.entry.Ref,
		DesiredActive: desired.entry.DesiredActive(),
		ConfigKeys:    configKeys,
	}
	if existing != nil {
		base.ExtensionID = existing.ID
		base.ExistingVersion = existing.Version
	}
	if desired.entry.DesiredState() == extensiondesiredstate.StateAbsent {
		if existing == nil {
			base.Action = OperationNoop
			base.Reason = "extension already absent"
			return []Operation{base}
		}
		ops := make([]Operation, 0, 2)
		if existing.Status == domain.ExtensionStatusActive {
			deactivate := base
			deactivate.Action = OperationDeactivate
			deactivate.Reason = "extension explicitly marked absent"
			ops = append(ops, deactivate)
		}
		uninstall := base
		uninstall.Action = OperationUninstall
		uninstall.Reason = "extension explicitly marked absent"
		ops = append(ops, uninstall)
		return ops
	}

	base.DesiredVersion = desired.manifest.Version
	ops := make([]Operation, 0, 5)
	configChanged := false
	if existing != nil {
		configChanged = !sameConfig(existing.Config.ToMap(), desired.config)
	}

	switch {
	case existing == nil:
		install := base
		install.Action = OperationInstall
		install.Reason = "extension is not installed"
		ops = append(ops, install)
		if configChangedFromDefault(desired.manifest, desired.config) {
			configure := base
			configure.Action = OperationConfigure
			configure.Reason = "desired config differs from bundle default config"
			ops = append(ops, configure)
		}
		validate := base
		validate.Action = OperationValidate
		validate.Reason = "validate new installation"
		ops = append(ops, validate)
		if desired.entry.DesiredActive() {
			activate := base
			activate.Action = OperationActivate
			activate.Reason = "desired state requires active extension"
			ops = append(ops, activate)
		}
	case checksumHex(desired.payload.Bytes) != existing.BundleSHA256 || existing.Version != desired.manifest.Version:
		upgrade := base
		upgrade.Action = OperationUpgrade
		upgrade.Reason = "installed bundle differs from desired bundle"
		ops = append(ops, upgrade)
		if configChanged {
			configure := base
			configure.Action = OperationConfigure
			configure.Reason = "desired config differs from installed config"
			ops = append(ops, configure)
		}
		validate := base
		validate.Action = OperationValidate
		validate.Reason = "validate upgraded installation"
		ops = append(ops, validate)
		if desired.entry.DesiredActive() && existing.Status != domain.ExtensionStatusActive {
			activate := base
			activate.Action = OperationActivate
			activate.Reason = "desired state requires active extension"
			ops = append(ops, activate)
		}
		if !desired.entry.DesiredActive() && existing.Status == domain.ExtensionStatusActive {
			deactivate := base
			deactivate.Action = OperationDeactivate
			deactivate.Reason = "desired state requires inactive extension"
			ops = append(ops, deactivate)
		}
	default:
		if configChanged {
			configure := base
			configure.Action = OperationConfigure
			configure.Reason = "desired config differs from installed config"
			ops = append(ops, configure)
		}
		if existing.ValidationStatus != domain.ExtensionValidationValid {
			validate := base
			validate.Action = OperationValidate
			validate.Reason = "installed extension validation is not valid"
			ops = append(ops, validate)
		}
		if desired.entry.DesiredActive() && existing.Status != domain.ExtensionStatusActive {
			activate := base
			activate.Action = OperationActivate
			activate.Reason = "desired state requires active extension"
			ops = append(ops, activate)
		}
		if !desired.entry.DesiredActive() && existing.Status == domain.ExtensionStatusActive {
			deactivate := base
			deactivate.Action = OperationDeactivate
			deactivate.Reason = "desired state requires inactive extension"
			ops = append(ops, deactivate)
		}
	}

	if len(ops) == 0 {
		base.Action = OperationNoop
		base.Reason = "installed extension already matches desired state"
		return []Operation{base}
	}
	return ops
}

func assetInputs(items []extensionbundle.Asset) []platformservices.ExtensionAssetInput {
	out := make([]platformservices.ExtensionAssetInput, 0, len(items))
	for _, item := range items {
		out = append(out, platformservices.ExtensionAssetInput{
			Path:           item.Path,
			Content:        []byte(item.Content),
			ContentType:    item.ContentType,
			IsCustomizable: item.IsCustomizable,
		})
	}
	return out
}

func migrationInputs(items []extensionbundle.Migration) []platformservices.ExtensionMigrationInput {
	out := make([]platformservices.ExtensionMigrationInput, 0, len(items))
	for _, item := range items {
		out = append(out, platformservices.ExtensionMigrationInput{
			Path:    item.Path,
			Content: []byte(item.Content),
		})
	}
	return out
}

func runtimeEntryForManifest(manifest domain.ExtensionManifest) (RuntimeManifestEntry, bool, error) {
	if manifest.RuntimeClass != domain.ExtensionRuntimeClassServiceBacked {
		return RuntimeManifestEntry{}, false, nil
	}
	artifact := strings.TrimSpace(manifest.Runtime.OCIReference)
	if artifact == "" {
		return RuntimeManifestEntry{}, false, fmt.Errorf("service-backed extension %s is missing runtime.ociReference", manifest.Slug)
	}
	serviceName := strings.TrimSpace(manifest.Slug) + "-runtime"
	return RuntimeManifestEntry{
		Slug:       manifest.Slug,
		PackageKey: manifest.PackageKey(),
		Artifact:   artifact,
		Binary:     serviceName,
		Service:    serviceName,
		Socket:     filepath.Base(publicruntime.SocketPath("/", manifest.PackageKey())),
	}, true, nil
}

func installKey(scope, workspaceRef, slug string) string {
	scope = strings.TrimSpace(strings.ToLower(scope))
	workspaceRef = strings.TrimSpace(strings.ToLower(workspaceRef))
	slug = strings.TrimSpace(strings.ToLower(slug))
	if scope == string(domain.ExtensionScopeInstance) {
		return scope + "::" + slug
	}
	return scope + "::" + workspaceRef + "::" + slug
}

func workspaceSlugForInstalled(extension *domain.InstalledExtension) string {
	if extension == nil {
		return ""
	}
	return strings.TrimSpace(extension.WorkspaceID)
}

func sameConfig(left, right map[string]any) bool {
	return normalizeJSON(left) == normalizeJSON(right)
}

func configChangedFromDefault(manifest domain.ExtensionManifest, desired map[string]any) bool {
	return !sameConfig(manifest.DefaultConfig.ToMap(), desired)
}

func normalizeJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func sortedKeys(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func checksumHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (e *Engine) actor() string {
	if e == nil || strings.TrimSpace(e.Actor) == "" {
		return "system:reconcile-extensions"
	}
	return strings.TrimSpace(e.Actor)
}

func (e *Engine) envLookup() func(string) (string, bool) {
	if e == nil || e.EnvLookup == nil {
		return os.LookupEnv
	}
	return e.EnvLookup
}
