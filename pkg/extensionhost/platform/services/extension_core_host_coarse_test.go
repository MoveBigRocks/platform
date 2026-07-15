package platformservices

import (
	"context"
	"errors"
	"testing"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

type fakeRuleEngine struct{ evaluations int }

func (f *fakeRuleEngine) EvaluateRulesForCase(_ context.Context, _ *servicedomain.Case, _ string, _ *automationservices.FieldChanges) error {
	f.evaluations++
	return nil
}

func TestCoreHostApplyCaseChangeFiresRulesOnceAcrossRetries(t *testing.T) {
	rules := &fakeRuleEngine{}
	cases := &fakeCaseService{getResult: &servicedomain.Case{
		CaseIdentity: servicedomain.CaseIdentity{ID: "case-1", WorkspaceID: "ws-1"},
	}}
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("case:write", "automation:write")},
		Cases:      cases,
		Rules:      rules,
		Tenant:     &fakeTenantRunner{},
	})
	input := runtimehost.ApplyCaseChangeInput{
		IdempotencyKey: "chg-1",
		Event:          "ats_application_stage_changed",
		Changes:        map[string]any{"stage": "interview"},
	}
	if _, err := svc.ApplyCaseChange(context.Background(), "ext-1", "case-1", input); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if _, err := svc.ApplyCaseChange(context.Background(), "ext-1", "case-1", input); err != nil {
		t.Fatalf("retry apply: %v", err)
	}
	if rules.evaluations != 1 {
		t.Fatalf("rules must fire exactly once across a retry, got %d", rules.evaluations)
	}
}

func TestCoreHostApplyCaseChangeRequiresAutomationPermission(t *testing.T) {
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("case:write")}, // no automation:write
		Cases:      &fakeCaseService{},
		Rules:      &fakeRuleEngine{},
		Tenant:     &fakeTenantRunner{},
	})
	_, err := svc.ApplyCaseChange(context.Background(), "ext-1", "case-1", runtimehost.ApplyCaseChangeInput{IdempotencyKey: "k"})
	if !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("expected forbidden without automation:write, got %v", err)
	}
}

type fakeContactService struct{ created int }

func (f *fakeContactService) CreateContact(_ context.Context, params CreateContactParams) (*platformdomain.Contact, error) {
	f.created++
	return &platformdomain.Contact{ID: "contact-1", WorkspaceID: params.WorkspaceID, Email: params.Email}, nil
}

func ingestInput() runtimehost.IngestApplicationInput {
	return runtimehost.IngestApplicationInput{
		IdempotencyKey: "app-123",
		Contact:        runtimehost.CreateContactInput{Email: "a@b.com"},
		Case:           runtimehost.IngestCaseInput{Subject: "Application"},
	}
}

func TestCoreHostIngestApplicationIsIdempotentAcrossRetries(t *testing.T) {
	contacts := &fakeContactService{}
	cases := &fakeCaseService{createResult: &servicedomain.Case{
		CaseIdentity: servicedomain.CaseIdentity{ID: "case-1", WorkspaceID: "ws-1"},
	}}
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("case:write", "contact:write", "attachment:write")},
		Cases:      cases,
		Contacts:   contacts,
		Tenant:     &fakeTenantRunner{},
	})

	first, err := svc.IngestApplication(context.Background(), "ext-1", ingestInput())
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	retry, err := svc.IngestApplication(context.Background(), "ext-1", ingestInput())
	if err != nil {
		t.Fatalf("retry ingest: %v", err)
	}
	if first.ContactID != retry.ContactID || first.CaseID != retry.CaseID {
		t.Fatalf("a retry must return the same ids: %+v vs %+v", first, retry)
	}
	if contacts.created != 1 {
		t.Fatalf("contact must be created exactly once across a retry, got %d", contacts.created)
	}
}

func TestCoreHostIngestApplicationRequiresAllWritePermissions(t *testing.T) {
	svc := NewExtensionCoreHostService(CoreHostDeps{
		// has case:write but not contact:write / attachment:write
		Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("case:write")},
		Cases:      &fakeCaseService{},
		Contacts:   &fakeContactService{},
		Tenant:     &fakeTenantRunner{},
	})
	_, err := svc.IngestApplication(context.Background(), "ext-1", ingestInput())
	if !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("expected forbidden without contact:write and attachment:write, got %v", err)
	}
}
