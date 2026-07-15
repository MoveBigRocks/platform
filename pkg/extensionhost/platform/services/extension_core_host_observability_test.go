package platformservices

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/platform/pkg/eventbus"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

type fakeCoreHostCaseStore struct {
	workspaceID string
	issueID     string
	contactID   string
	result      *servicedomain.Case
}

func (f *fakeCoreHostCaseStore) GetCaseByIssueAndContact(_ context.Context, workspaceID, issueID, contactID string) (*servicedomain.Case, error) {
	f.workspaceID, f.issueID, f.contactID = workspaceID, issueID, contactID
	return f.result, nil
}

type fakeCoreHostWorkspaces struct{ values []*platformdomain.Workspace }

func (f *fakeCoreHostWorkspaces) ListWorkspaces(context.Context) ([]*platformdomain.Workspace, error) {
	return f.values, nil
}
func (f *fakeCoreHostWorkspaces) GetWorkspacesByIDs(context.Context, []string) ([]*platformdomain.Workspace, error) {
	return f.values, nil
}

type fakeCoreHostPublisher struct {
	stream eventbus.Stream
	event  eventbus.Event
}

func (f *fakeCoreHostPublisher) PublishEvent(_ context.Context, stream eventbus.Stream, event eventbus.Event) error {
	f.stream, f.event = stream, event
	return nil
}

func TestCoreHostIssueCaseOperationsUseTargetWorkspace(t *testing.T) {
	cases := &fakeCaseService{}
	tenant := &fakeTenantRunner{}
	service := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeInstanceExtension("case:read", "case:write")},
		Cases:      cases, Tenant: tenant,
		CaseStore: &fakeCoreHostCaseStore{result: &servicedomain.Case{CaseIdentity: servicedomain.CaseIdentity{ID: "case-1", WorkspaceID: "ws-7"}}},
	})

	if err := service.LinkIssueToCase(context.Background(), "ext-1", "case-1", runtimehost.LinkIssueToCaseInput{
		WorkspaceID: "ws-7", IssueID: "issue-1", ProjectID: "project-1",
	}); err != nil {
		t.Fatalf("LinkIssueToCase: %v", err)
	}
	if tenant.tenantWorkspace != "ws-7" || cases.linkedCaseID != "case-1" || cases.linkedIssue != "issue-1" || cases.linkedProject != "project-1" {
		t.Fatalf("link was not scoped and forwarded: tenant=%q cases=%+v", tenant.tenantWorkspace, cases)
	}

	if err := service.UnlinkIssueFromCase(context.Background(), "ext-1", "case-1", runtimehost.UnlinkIssueFromCaseInput{
		WorkspaceID: "ws-7", IssueID: "issue-1",
	}); err != nil {
		t.Fatalf("UnlinkIssueFromCase: %v", err)
	}
	if cases.unlinkedCaseID != "case-1" || cases.unlinkedIssue != "issue-1" {
		t.Fatalf("unlink was not forwarded: %+v", cases)
	}
}

func TestCoreHostCrossWorkspaceReadsRequireInstanceScope(t *testing.T) {
	workspaces := &fakeCoreHostWorkspaces{values: []*platformdomain.Workspace{{ID: "ws-1", Name: "One"}}}
	tenant := &fakeTenantRunner{}
	instanceService := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeInstanceExtension("workspace:read")},
		Workspaces: workspaces, Tenant: tenant,
	})

	got, err := instanceService.ListWorkspaces(context.Background(), "ext-1")
	if err != nil || len(got) != 1 || got[0].ID != "ws-1" {
		t.Fatalf("ListWorkspaces: got=%v err=%v", got, err)
	}
	if tenant.adminCalls != 1 {
		t.Fatalf("expected admin-scoped query, got %d calls", tenant.adminCalls)
	}

	workspaceService := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("workspace:read")},
		Workspaces: workspaces, Tenant: &fakeTenantRunner{},
	})
	if _, err := workspaceService.ListWorkspaces(context.Background(), "ext-1"); !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("workspace-scoped extension must not list all workspaces: %v", err)
	}
}

func TestCoreHostPublishEventEnforcesScopeAndPublishesRegisteredType(t *testing.T) {
	publisher := &fakeCoreHostPublisher{}
	tenant := &fakeTenantRunner{}
	service := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeInstanceExtension("event:publish")},
		Outbox:     publisher, Tenant: tenant,
	})
	input := runtimehost.PublishEventInput{
		WorkspaceID: "ws-9", EventType: "issue.created", Data: map[string]any{"IssueID": "issue-1"},
	}
	if err := service.PublishEvent(context.Background(), "ext-1", input); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}
	if tenant.tenantWorkspace != "ws-9" || publisher.stream != eventbus.StreamIssueEvents || publisher.event == nil {
		t.Fatalf("event was not scoped and published: tenant=%q stream=%v event=%v", tenant.tenantWorkspace, publisher.stream, publisher.event)
	}
	encoded, err := json.Marshal(publisher.event)
	if err != nil {
		t.Fatalf("marshal published event: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode published event: %v", err)
	}
	if payload["WorkspaceID"] != "ws-9" || payload["IssueID"] != "issue-1" {
		t.Fatalf("unexpected event payload: %s", encoded)
	}

	input.Data["WorkspaceID"] = "ws-other"
	if err := service.PublishEvent(context.Background(), "ext-1", input); !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("payload workspace mismatch must be forbidden: %v", err)
	}
	delete(input.Data, "WorkspaceID")
	input.Data["workspace_id"] = "ws-other"
	if err := service.PublishEvent(context.Background(), "ext-1", input); !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("snake-case payload workspace mismatch must be forbidden: %v", err)
	}
}
