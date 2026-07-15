package platformservices

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	shared "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
)

type fakeExtensionResolver struct {
	ext *platformdomain.InstalledExtension
	err error
}

func (f *fakeExtensionResolver) GetInstalledExtension(context.Context, string) (*platformdomain.InstalledExtension, error) {
	return f.ext, f.err
}

type fakeCaseService struct {
	created      serviceapp.CreateCaseParams
	createResult *servicedomain.Case
	getResult    *servicedomain.Case
	getErr       error
}

func (f *fakeCaseService) CreateCase(_ context.Context, params serviceapp.CreateCaseParams) (*servicedomain.Case, error) {
	f.created = params
	return f.createResult, nil
}

func (f *fakeCaseService) GetCaseInWorkspace(_ context.Context, _, _ string) (*servicedomain.Case, error) {
	return f.getResult, f.getErr
}

func (f *fakeCaseService) UpdateCase(_ context.Context, _ *servicedomain.Case) error { return nil }

func (f *fakeCaseService) HandoffCase(_ context.Context, _ string, _ serviceapp.CaseHandoffParams) error {
	return nil
}

func (f *fakeCaseService) MarkCaseResolved(_ context.Context, _ string, _ time.Time) error {
	return nil
}

type fakeTenantRunner struct {
	tenantWorkspace string
	ledger          map[string][]byte
}

func (f *fakeTenantRunner) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (f *fakeTenantRunner) SetTenantContext(_ context.Context, workspaceID string) error {
	f.tenantWorkspace = workspaceID
	return nil
}

func (f *fakeTenantRunner) GetHostOperationResult(_ context.Context, _, _, operation, key string) ([]byte, bool, error) {
	v, ok := f.ledger[operation+"|"+key]
	return v, ok, nil
}

func (f *fakeTenantRunner) PutHostOperationResult(_ context.Context, _, _, operation, key string, result []byte) error {
	if f.ledger == nil {
		f.ledger = map[string][]byte{}
	}
	f.ledger[operation+"|"+key] = result
	return nil
}

func activeWorkspaceExtension(permissions ...string) *platformdomain.InstalledExtension {
	return &platformdomain.InstalledExtension{
		WorkspaceID: "ws-1",
		Status:      platformdomain.ExtensionStatusActive,
		Manifest: platformdomain.ExtensionManifest{
			Scope:       platformdomain.ExtensionScopeWorkspace,
			Permissions: permissions,
		},
	}
}

func activeInstanceExtension(permissions ...string) *platformdomain.InstalledExtension {
	return &platformdomain.InstalledExtension{
		Status: platformdomain.ExtensionStatusActive,
		Manifest: platformdomain.ExtensionManifest{
			Scope:       platformdomain.ExtensionScopeInstance,
			Permissions: permissions,
		},
	}
}

// TestResolveExtensionScopeEnforcement pins down the cross-tenant boundary: a
// workspace-scoped extension can only ever act in its own workspace, while an
// instance-scoped extension must name the workspace it wants to act in and may
// only then reach across workspaces.
func TestResolveExtensionScopeEnforcement(t *testing.T) {
	ctx := context.Background()
	ws := NewExtensionCoreHostService(CoreHostDeps{Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("workspace:read", "case:write")}})
	inst := NewExtensionCoreHostService(CoreHostDeps{Extensions: &fakeExtensionResolver{ext: activeInstanceExtension("workspace:read", "case:write")}})

	// Workspace-scoped: an empty or matching target resolves to its own workspace.
	if _, got, err := ws.resolveExtensionForWorkspace(ctx, "ext-1", "case:write", ""); err != nil || got != "ws-1" {
		t.Fatalf("workspace-scoped empty target: got=%q err=%v", got, err)
	}
	if _, got, err := ws.resolveExtensionForWorkspace(ctx, "ext-1", "case:write", "ws-1"); err != nil || got != "ws-1" {
		t.Fatalf("workspace-scoped own target: got=%q err=%v", got, err)
	}
	// Workspace-scoped: naming a foreign workspace is refused (no cross-tenant reach).
	if _, _, err := ws.resolveExtensionForWorkspace(ctx, "ext-1", "case:write", "ws-2"); !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("workspace-scoped foreign target must be forbidden, got %v", err)
	}
	// Workspace-scoped: a cross-workspace operation is refused outright.
	if _, err := ws.resolveExtensionForCrossWorkspace(ctx, "ext-1", "workspace:read"); !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("workspace-scoped cross-workspace must be forbidden, got %v", err)
	}

	// Instance-scoped: a single-workspace op must name the target workspace.
	if _, _, err := inst.resolveExtensionForWorkspace(ctx, "ext-1", "case:write", ""); err == nil {
		t.Fatalf("instance-scoped without a target must error")
	}
	// Instance-scoped: may act in any named workspace.
	if _, got, err := inst.resolveExtensionForWorkspace(ctx, "ext-1", "case:write", "ws-7"); err != nil || got != "ws-7" {
		t.Fatalf("instance-scoped named target: got=%q err=%v", got, err)
	}
	// Instance-scoped: a cross-workspace op is allowed with the permission.
	if _, err := inst.resolveExtensionForCrossWorkspace(ctx, "ext-1", "workspace:read"); err != nil {
		t.Fatalf("instance-scoped cross-workspace must be allowed, got %v", err)
	}
	// The permission gate still applies regardless of scope.
	if _, err := inst.resolveExtensionForCrossWorkspace(ctx, "ext-1", "contact:write"); !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("missing permission must be forbidden, got %v", err)
	}
}

func TestCoreHostCreateCaseAppliesWorkspaceScopeAndTenantContext(t *testing.T) {
	cases := &fakeCaseService{createResult: &servicedomain.Case{
		CaseIdentity: servicedomain.CaseIdentity{ID: "case-1", WorkspaceID: "ws-1"},
		Subject:      "Boom",
	}}
	tenant := &fakeTenantRunner{}
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("case:write")},
		Cases:      cases,
		Tenant:     tenant,
	})

	out, err := svc.CreateCase(context.Background(), "ext-1", runtimehost.CreateCaseInput{
		Subject:  "Boom",
		Priority: "high",
		// A caller-supplied workspace must never override the extension's scope.
	})
	if err != nil {
		t.Fatalf("CreateCase: %v", err)
	}
	if out.ID != "case-1" {
		t.Fatalf("expected returned case id case-1, got %q", out.ID)
	}
	if cases.created.WorkspaceID != "ws-1" {
		t.Fatalf("case must be created in the extension workspace ws-1, got %q", cases.created.WorkspaceID)
	}
	if tenant.tenantWorkspace != "ws-1" {
		t.Fatalf("tenant context must be set to ws-1, got %q", tenant.tenantWorkspace)
	}
}

func TestCoreHostCreateCaseRequiresPermission(t *testing.T) {
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("case:read")}, // no case:write
		Cases:      &fakeCaseService{},
		Tenant:     &fakeTenantRunner{},
	})
	_, err := svc.CreateCase(context.Background(), "ext-1", runtimehost.CreateCaseInput{Subject: "x"})
	if !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("expected forbidden without case:write, got %v", err)
	}
}

func TestCoreHostCreateCaseRejectsInactiveExtension(t *testing.T) {
	ext := activeWorkspaceExtension("case:write")
	ext.Status = platformdomain.ExtensionStatusInactive
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: ext},
		Cases:      &fakeCaseService{},
		Tenant:     &fakeTenantRunner{},
	})
	_, err := svc.CreateCase(context.Background(), "ext-1", runtimehost.CreateCaseInput{Subject: "x"})
	if !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("expected forbidden for inactive extension, got %v", err)
	}
}

func TestCoreHostGetCaseMapsNotFound(t *testing.T) {
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("case:read")},
		Cases:      &fakeCaseService{getErr: shared.ErrNotFound},
		Tenant:     &fakeTenantRunner{},
	})
	_, err := svc.GetCase(context.Background(), "ext-1", "missing")
	if !errors.Is(err, ErrCoreHostNotFound) {
		t.Fatalf("expected ErrCoreHostNotFound, got %v", err)
	}
}
