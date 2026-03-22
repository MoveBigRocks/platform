# RFC: Multi-Layer Workspace Isolation

**Status**: Implemented
**Created**: 2026-01-10
**Updated**: 2026-01-10
**Author**: Security Review
**Priority**: P1 - Security Critical

## Implementation Status

Phase 1 (Resolver Layer) has been fully implemented:
- All GraphQL resolvers now validate workspace ownership using `ValidateWorkspaceOwnership()`
- Helper function added to `internal/graph/shared/context.go`
- Error messages return "not found" to prevent enumeration

Files modified:
- `internal/service/resolvers/resolver.go` - Case, Communication mutations
- `internal/observability/resolvers/resolver.go` - Issue, ErrorEvent, Project queries/mutations
- `internal/platform/resolvers/resolver.go` - Agent, Workspace queries/mutations

Phase 2-4 (Service/Store defense-in-depth) remain optional hardening layers on top of the core workspace-isolation model.

---

## Original Problem Statement

The GraphQL API previously allowed authenticated agents/users to access resources from workspaces other than their own by providing resource IDs directly. This bypassed workspace isolation.

### Affected Endpoints

All GraphQL queries/mutations that accept resource IDs:
- `Case(id)`, `Issue(id)`, `Project(id)`, `ErrorEvent(id)`, `Agent(id)`
- `UpdateCaseStatus`, `AssignCase`, `AddCommunication`
- `UpdateIssueStatus`, `LinkIssueToCase`, `UnlinkIssueFromCase`

### Root Cause

1. GraphQL resolvers only check **permissions** (e.g., `PermissionCaseRead`), not **workspace ownership**
2. Service layer methods accept only resource ID, not workspace ID
3. Store layer queries by ID without workspace filtering

## Solution: Defense-in-Depth

Implement workspace validation at multiple application layers. Each layer provides an independent security barrier.

```
┌─────────────────────────────────────────────────────────────┐
│ Layer 1: GraphQL Resolver                                   │
│ - Extract workspace from AuthContext                        │
│ - Validate resource.WorkspaceID == authCtx.WorkspaceID      │
│ - Return "not found" (not "access denied") on mismatch      │
├─────────────────────────────────────────────────────────────┤
│ Layer 2: Service                                            │
│ - Workspace-scoped methods: GetCaseInWorkspace(ws, id)      │
│ - Validates workspace match internally                      │
├─────────────────────────────────────────────────────────────┤
│ Layer 3: Store                                              │
│ - SQL queries include workspace_id in WHERE clause          │
│ - SELECT ... WHERE id = ? AND workspace_id = ?              │
├─────────────────────────────────────────────────────────────┤
│ Layer 4: PostgreSQL Database                                │
│ - Composite indexes on (workspace_id, id) for performance   │
│ - Foreign key constraints ensure referential integrity      │
└─────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Resolver Layer (Immediate - Day 1-2)

Add workspace validation to ALL affected GraphQL resolvers.

### Pattern to Apply

```go
func (r *Resolver) Case(ctx context.Context, id string) (*CaseResolver, error) {
    authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseRead)
    if err != nil {
        return nil, err
    }

    caseObj, err := r.caseService.GetCase(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get case: %w", err)
    }

    // NEW: Validate workspace ownership
    if caseObj.WorkspaceID != authCtx.WorkspaceID {
        return nil, fmt.Errorf("case not found") // Don't reveal existence
    }

    return &CaseResolver{case_: caseObj, r: r}, nil
}
```

### Files to Modify

**1. `internal/service/resolvers/resolver.go`**

| Method | Line | Action |
|--------|------|--------|
| `Case()` | 50-61 | Add workspace check after GetCase |
| `AddCommunication()` | 129-171 | Check case.WorkspaceID before modification |
| `UpdateCaseStatus()` | 174-190 | Add workspace check |
| `AssignCase()` | 193-214 | Add workspace check |

**2. `internal/observability/resolvers/resolver.go`**

| Method | Line | Action |
|--------|------|--------|
| `Issue()` | 42-54 | Add workspace check (need to get project first) |
| `ErrorEvent()` | 105-117 | Add workspace check (via issue → project) |
| `Project()` | 124-136 | Add workspace check directly |
| `UpdateIssueStatus()` | 165-182 | Add workspace check |
| `LinkIssueToCase()` | 185-202 | Verify BOTH issue and case are in same workspace |
| `UnlinkIssueFromCase()` | 205-225 | Add workspace check |

**3. `internal/platform/resolvers/resolver.go`**

| Method | Line | Action |
|--------|------|--------|
| `Agent()` | 63-75 | Add workspace check |
| `UpdateAgent()` | 186-215 | Add workspace check |
| `SuspendAgent()` | 217-240 | Add workspace check |
| `ActivateAgent()` | 242-265 | Add workspace check |
| `RevokeAgent()` | 267-294 | Add workspace check |
| `CreateAgentToken()` | 296-326 | Verify agent belongs to workspace |
| `RevokeAgentToken()` | 328-354 | Verify token's agent belongs to workspace |

### Helper Function

Add a helper to reduce boilerplate:

```go
// internal/graph/shared/workspace.go

// ValidateWorkspaceOwnership checks if a resource belongs to the authenticated workspace.
// Returns a "not found" error to avoid leaking resource existence.
func ValidateWorkspaceOwnership(resourceWorkspaceID, authWorkspaceID, resourceType string) error {
    if resourceWorkspaceID != authWorkspaceID {
        return fmt.Errorf("%s not found", resourceType)
    }
    return nil
}
```

---

## Phase 2: Service Layer (Day 3-4)

Add workspace-scoped variants to service methods.

### New Method Signatures

```go
// CaseService
func (s *CaseService) GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*Case, error) {
    c, err := s.GetCase(ctx, caseID)
    if err != nil {
        return nil, err
    }
    if c.WorkspaceID != workspaceID {
        return nil, apierrors.NotFoundError("case", caseID)
    }
    return c, nil
}

// IssueService
func (s *IssueService) GetIssueInWorkspace(ctx context.Context, workspaceID, issueID string) (*Issue, error)

// ProjectService
func (s *ProjectService) GetProjectInWorkspace(ctx context.Context, workspaceID, projectID string) (*Project, error)

// AgentService
func (s *AgentService) GetAgentInWorkspace(ctx context.Context, workspaceID, agentID string) (*Agent, error)
```

### Files to Modify

- `internal/service/services/case_service.go`
- `internal/observability/services/issue_service.go`
- `internal/observability/services/project_service.go`
- `internal/platform/services/agent_service.go`

---

## Phase 3: Store Layer (Day 5-7)

Add workspace filtering directly into SQL queries for defense-in-depth.

### New Store Methods

```go
// internal/infrastructure/stores/sql/case_store.go

func (s *CaseStore) GetCaseByWorkspace(ctx context.Context, workspaceID, caseID string) (*Case, error) {
    query := `SELECT * FROM cases WHERE id = ? AND workspace_id = ?`
    var model models.Case
    err := s.db.Get(ctx).GetContext(ctx, &model, query, caseID, workspaceID)
    if err != nil {
        return nil, TranslateSqlxError(err, "cases")
    }
    return s.mapToDomain(&model), nil
}
```

### Database Index Optimization

Add composite indexes for efficient workspace-scoped queries:

```sql
-- migrations/xxx_add_workspace_composite_indexes.sql
CREATE INDEX IF NOT EXISTS idx_cases_workspace_id ON cases(workspace_id, id);
CREATE INDEX IF NOT EXISTS idx_issues_workspace_id ON issues(workspace_id, id);
CREATE INDEX IF NOT EXISTS idx_projects_workspace_id ON projects(workspace_id, id);
CREATE INDEX IF NOT EXISTS idx_agents_workspace_id ON agents(workspace_id, id);
```

---

## Phase 4: Integration Tests (Concurrent)

### Test Structure

```go
// internal/service/resolvers/resolver_isolation_test.go

func TestCrossWorkspaceAccessDenied(t *testing.T) {
    // Setup: Create two workspaces
    wsA := createTestWorkspace(t, "workspace-a")
    wsB := createTestWorkspace(t, "workspace-b")

    // Setup: Create case in Workspace A
    caseA := createTestCase(t, wsA.ID)

    // Setup: Create agent in Workspace B with case read permission
    agentB := createTestAgent(t, wsB.ID, []string{PermissionCaseRead})

    // Test: Agent B queries Case A via GraphQL
    query := `query { case(id: "%s") { id subject } }`
    resp := executeGraphQL(t, agentB.Token, fmt.Sprintf(query, caseA.ID))

    // Assert: Should fail with "not found"
    require.Len(t, resp.Errors, 1)
    assert.Contains(t, resp.Errors[0].Message, "not found")
    assert.Nil(t, resp.Data)
}

func TestCrossWorkspaceMutationDenied(t *testing.T) {
    // Similar setup...

    // Test: Agent B tries to add communication to Case A
    mutation := `mutation { addCommunication(input: {caseId: "%s", body: "test"}) { id } }`
    resp := executeGraphQL(t, agentB.Token, fmt.Sprintf(mutation, caseA.ID))

    // Assert: Should fail
    require.Len(t, resp.Errors, 1)

    // Verify: Case A should have no new communications
    comms := getCaseCommunications(t, caseA.ID)
    assert.Len(t, comms, 0)
}
```

### Test Files to Create

- `internal/service/resolvers/resolver_isolation_test.go`
- `internal/observability/resolvers/resolver_isolation_test.go`
- `internal/platform/resolvers/resolver_isolation_test.go`

---

## Implementation Order

| Priority | Phase | Effort | Blocks Deployment? |
|----------|-------|--------|-------------------|
| **1** | Phase 1: Resolver validation | 4-6 hours | Yes - vulnerability |
| **2** | Phase 4: Integration tests | 4 hours | Yes - verify fix |
| **3** | Phase 2: Service layer | 4 hours | No - defense-in-depth |
| **4** | Phase 3: Store layer | 6 hours | No - defense-in-depth |

---

## Security Considerations

1. **Error message consistency**: Always return "not found" rather than "access denied" to prevent information leakage about resource existence in other workspaces.

2. **Logging**: Log workspace mismatch attempts for security monitoring:
   ```go
   if caseObj.WorkspaceID != authCtx.WorkspaceID {
       logger.Warn("Cross-workspace access attempt",
           "requested_case", id,
           "case_workspace", caseObj.WorkspaceID,
           "auth_workspace", authCtx.WorkspaceID,
           "principal", authCtx.Principal.GetID())
       return nil, fmt.Errorf("case not found")
   }
   ```

3. **Compound resources**: For `LinkIssueToCase`, verify BOTH the issue AND case belong to the authenticated workspace.

---

## Acceptance Criteria

- [ ] All GraphQL resolvers validate workspace ownership
- [ ] Service layer has workspace-scoped Get methods
- [ ] Store layer queries include workspace_id in WHERE clause
- [ ] Integration tests verify cross-workspace access is blocked
- [ ] Error messages don't reveal existence of cross-workspace resources
- [ ] Security logging captures access attempts

---

## Related ADRs

- ADR-0003: Multi-tenant Isolation
- ADR-0009: Layered Architecture
