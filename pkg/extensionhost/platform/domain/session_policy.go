package platformdomain

import "fmt"

// BuildAvailableContexts derives the session contexts that a user may access from loaded domain objects.
func (u *User) BuildAvailableContexts(
	allWorkspaces []*Workspace,
	workspaceRoles []*UserWorkspaceRole,
	workspaceByID map[string]*Workspace,
) ([]Context, error) {
	contexts := make([]Context, 0, len(workspaceRoles)+1)

	if u.InstanceRole != nil {
		instanceContext, err := NewInstanceContext(*u.InstanceRole)
		if err != nil {
			return nil, fmt.Errorf("invalid instance role for user %s: %s", u.ID, *u.InstanceRole)
		}
		contexts = append(contexts, instanceContext)
	}

	if u.IsSuperAdmin() {
		for _, workspace := range allWorkspaces {
			contexts = append(contexts, NewWorkspaceContext(workspace, string(InstanceRoleSuperAdmin)))
		}
		return contexts, nil
	}

	for _, role := range workspaceRoles {
		if role == nil || !role.IsActive() {
			continue
		}
		contexts = append(contexts, NewWorkspaceContext(workspaceByID[role.WorkspaceID], string(role.Role)))
	}

	return contexts, nil
}

// NewInstanceContext builds an instance-admin session context from an instance role.
func NewInstanceContext(role InstanceRole) (Context, error) {
	canonical := CanonicalizeInstanceRole(role)
	if !canonical.IsOperator() {
		return Context{}, fmt.Errorf("invalid instance role: %s", role)
	}
	return Context{
		Type: ContextTypeInstance,
		Role: string(canonical),
	}, nil
}

// NewWorkspaceContext builds a workspace session context from workspace metadata and effective role.
func NewWorkspaceContext(workspace *Workspace, role string) Context {
	context := Context{
		Type: ContextTypeWorkspace,
		Role: role,
	}
	if workspace == nil {
		return context
	}

	workspaceID := workspace.ID
	context.WorkspaceID = &workspaceID
	if workspace.Name != "" {
		workspaceName := workspace.Name
		context.WorkspaceName = &workspaceName
	}
	if workspace.Slug != "" {
		workspaceSlug := workspace.Slug
		context.WorkspaceSlug = &workspaceSlug
	}
	return context
}

// DefaultContext selects the default context for a user.
func (u *User) DefaultContext(contexts []Context) Context {
	if len(contexts) == 0 {
		return Context{}
	}

	if u.IsSuperAdmin() {
		if ctx, ok := FindContext(contexts, ContextTypeInstance, nil); ok {
			return ctx
		}
	}

	priority := []string{
		string(WorkspaceRoleOwner),
		string(WorkspaceRoleAdmin),
		string(WorkspaceRoleMember),
	}

	for _, wantedRole := range priority {
		for _, ctx := range contexts {
			if ctx.Type == ContextTypeWorkspace && ctx.Role == wantedRole {
				return ctx
			}
		}
	}

	for _, ctx := range contexts {
		if ctx.Type == ContextTypeWorkspace {
			return ctx
		}
	}

	if ctx, ok := FindContext(contexts, ContextTypeInstance, nil); ok {
		return ctx
	}

	return contexts[0]
}

// FindContext returns a matching session context from an available set.
func FindContext(contexts []Context, contextType ContextType, workspaceID *string) (Context, bool) {
	for _, ctx := range contexts {
		if ctx.Type != contextType {
			continue
		}
		if contextType == ContextTypeInstance {
			return ctx, true
		}
		if contextType == ContextTypeWorkspace &&
			workspaceID != nil &&
			ctx.WorkspaceID != nil &&
			*ctx.WorkspaceID == *workspaceID {
			return ctx, true
		}
	}
	return Context{}, false
}

// HasContext reports whether a context remains available to the session.
func HasContext(contexts []Context, target Context) bool {
	_, ok := FindContext(contexts, target.Type, target.WorkspaceID)
	return ok
}

// ReconcileContexts replaces the available contexts and falls back to the user's default if needed.
func (s *Session) ReconcileContexts(user *User, contexts []Context) {
	s.AvailableContexts = contexts
	if len(contexts) == 0 {
		return
	}
	if !HasContext(contexts, s.CurrentContext) {
		s.CurrentContext = user.DefaultContext(contexts)
	}
}
