package platformdomain

import (
	"testing"
	"time"
)

func TestBuildAvailableContextsAdditionalBranches(t *testing.T) {
	t.Parallel()

	invalidRole := InstanceRole("owner")
	user := &User{ID: "user_1", InstanceRole: &invalidRole}
	if _, err := user.BuildAvailableContexts(nil, nil, nil); err == nil {
		t.Fatal("expected invalid instance role to fail")
	}

	member := &User{ID: "user_2"}
	expiredAt := time.Now().Add(-time.Hour)
	contexts, err := member.BuildAvailableContexts(
		nil,
		[]*UserWorkspaceRole{
			nil,
			{WorkspaceID: "ws_inactive", Role: WorkspaceRoleAdmin, ExpiresAt: &expiredAt},
			{WorkspaceID: "ws_active", Role: WorkspaceRoleMember},
		},
		map[string]*Workspace{
			"ws_active": {ID: "ws_active", Name: "Acme", Slug: "acme"},
		},
	)
	if err != nil {
		t.Fatalf("build contexts: %v", err)
	}
	if len(contexts) != 1 || contexts[0].Type != ContextTypeWorkspace || contexts[0].WorkspaceID == nil || *contexts[0].WorkspaceID != "ws_active" {
		t.Fatalf("unexpected regular user contexts: %#v", contexts)
	}

	contexts, err = member.BuildAvailableContexts(
		nil,
		[]*UserWorkspaceRole{{WorkspaceID: "ws_unknown", Role: WorkspaceRoleViewer}},
		map[string]*Workspace{},
	)
	if err != nil {
		t.Fatalf("build contexts without workspace metadata: %v", err)
	}
	if len(contexts) != 1 || contexts[0].WorkspaceID != nil {
		t.Fatalf("expected nil workspace metadata context, got %#v", contexts)
	}
}

func TestDefaultContextPriorityBranches(t *testing.T) {
	t.Parallel()

	workspaceOwner := Context{Type: ContextTypeWorkspace, Role: string(WorkspaceRoleOwner)}
	workspaceAdmin := Context{Type: ContextTypeWorkspace, Role: string(WorkspaceRoleAdmin)}
	workspaceMember := Context{Type: ContextTypeWorkspace, Role: string(WorkspaceRoleMember)}
	workspaceViewer := Context{Type: ContextTypeWorkspace, Role: string(WorkspaceRoleViewer)}
	instance := Context{Type: ContextTypeInstance, Role: string(InstanceRoleAdmin)}

	super := &User{InstanceRole: ptr(InstanceRoleSuperAdmin)}
	if got := super.DefaultContext([]Context{workspaceOwner, instance}); got.Type != ContextTypeInstance {
		t.Fatalf("expected super admin to prefer instance context, got %#v", got)
	}

	regular := &User{}
	if got := regular.DefaultContext([]Context{workspaceViewer, workspaceOwner}); got.Role != string(WorkspaceRoleOwner) {
		t.Fatalf("expected owner priority, got %#v", got)
	}
	if got := regular.DefaultContext([]Context{workspaceViewer, workspaceAdmin}); got.Role != string(WorkspaceRoleAdmin) {
		t.Fatalf("expected admin priority, got %#v", got)
	}
	if got := regular.DefaultContext([]Context{workspaceViewer, workspaceMember}); got.Role != string(WorkspaceRoleMember) {
		t.Fatalf("expected member priority, got %#v", got)
	}
	if got := regular.DefaultContext([]Context{workspaceViewer}); got.Role != string(WorkspaceRoleViewer) {
		t.Fatalf("expected workspace fallback, got %#v", got)
	}
	if got := regular.DefaultContext([]Context{instance}); got.Type != ContextTypeInstance {
		t.Fatalf("expected instance fallback, got %#v", got)
	}
	if got := regular.DefaultContext([]Context{{Role: "unknown"}}); got.Role != "unknown" {
		t.Fatalf("expected first-context fallback, got %#v", got)
	}
	if got := regular.DefaultContext(nil); got != (Context{}) {
		t.Fatalf("expected zero context for empty input, got %#v", got)
	}
}

func TestInstanceContextAndReconcileBranches(t *testing.T) {
	t.Parallel()

	if _, err := NewInstanceContext(InstanceRole("viewer")); err == nil {
		t.Fatal("expected invalid instance role to fail")
	}

	instance, err := NewInstanceContext(InstanceRole(" ADMIN "))
	if err != nil {
		t.Fatalf("new instance context: %v", err)
	}
	if instance.Role != string(InstanceRoleAdmin) {
		t.Fatalf("expected canonical admin role, got %#v", instance)
	}

	workspaceID := "ws_1"
	contexts := []Context{
		{Type: ContextTypeWorkspace, WorkspaceID: &workspaceID, Role: string(WorkspaceRoleAdmin)},
		{Type: ContextTypeInstance, Role: string(InstanceRoleAdmin)},
	}
	user := &User{}
	session := &Session{CurrentContext: contexts[0]}
	session.ReconcileContexts(user, contexts)
	if session.CurrentContext != contexts[0] {
		t.Fatalf("expected reconcile to preserve valid current context, got %#v", session.CurrentContext)
	}

	session = &Session{CurrentContext: Context{Type: ContextTypeWorkspace, WorkspaceID: ptr("missing")}}
	session.ReconcileContexts(user, nil)
	if len(session.AvailableContexts) != 0 || session.CurrentContext.WorkspaceID == nil || *session.CurrentContext.WorkspaceID != "missing" {
		t.Fatalf("expected empty reconcile to leave current context unchanged, got %#v", session)
	}
}
