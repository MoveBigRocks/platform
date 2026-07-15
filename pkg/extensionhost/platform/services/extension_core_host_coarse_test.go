package platformservices

import (
	"context"
	"errors"
	"testing"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

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
