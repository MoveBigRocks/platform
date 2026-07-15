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
