# ADR 0003: Multi-Tenant Isolation

**Status:** Accepted (Updated January 2026)

## Context

Move Big Rocks supports multiple brands on a single instance. Each brand (workspace) needs:
- Complete data isolation (GDPR, security)
- Independent configuration (branding, workflows)
- Separate user access controls
- Zero cross-workspace data leakage

## Decision

**Use workspace_id column on all tables with 4-layer defense-in-depth enforcement.**

### Database Schema

All tables include `workspace_id` with foreign key to workspaces:

```sql
CREATE TABLE cases (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    -- other fields...
);

CREATE INDEX idx_cases_workspace ON cases(workspace_id);
```

### 4-Layer Defense-in-Depth Architecture

Workspace isolation is enforced at every layer of the application. If any single layer is bypassed (through a bug or attack), the other layers provide protection.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Layer 1: Authentication Middleware                                          │
│  - Extract workspace_id from session/token                                   │
│  - Validate token authenticity and expiration                                │
│  - Inject authenticated context into request                                 │
└─────────────────────────────────────────────────────────────────────────────┘
       │
┌──────▼──────────────────────────────────────────────────────────────────────┐
│  Layer 2: GraphQL Resolver / REST Handler                                    │
│  - Validate permissions via RequirePermission()                              │
│  - Fetch resource by ID                                                      │
│  - Compare resource.WorkspaceID with authContext.WorkspaceID                 │
│  - Return "not found" error if mismatch (prevents enumeration)               │
└─────────────────────────────────────────────────────────────────────────────┘
       │
┌──────▼──────────────────────────────────────────────────────────────────────┐
│  Layer 3: Service Layer                                                      │
│  - GetResourceInWorkspace(workspaceID, resourceID) methods                   │
│  - Validates workspace ownership before returning resource                   │
│  - Returns "not found" for both non-existent AND wrong-workspace resources   │
└─────────────────────────────────────────────────────────────────────────────┘
       │
┌──────▼──────────────────────────────────────────────────────────────────────┐
│  Layer 4: Store/Database Layer                                               │
│  - Workspace-scoped query methods with workspace_id in WHERE clause          │
│  - SQL: SELECT * FROM table WHERE id = ? AND workspace_id = ?                │
│  - Returns ErrNotFound if no matching row                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Security Principle: Consistent Error Responses

**CRITICAL:** All layers return a generic "not found" error when workspace validation fails. This prevents information disclosure attacks:

```go
// BAD - reveals resource exists in different workspace
if resource.WorkspaceID != authContext.WorkspaceID {
    return errors.New("access denied")  // WRONG - reveals existence
}

// GOOD - hides resource existence across workspaces
if resource.WorkspaceID != authContext.WorkspaceID {
    return apierrors.NotFoundError("resource", resourceID)  // CORRECT
}
```

This prevents attackers from enumerating resources across workspaces by observing different error responses.

### GraphQL Resolver Pattern

All resolvers validate workspace ownership after fetching resources:

```go
func (r *Resolver) Case(ctx context.Context, id string) (*CaseResolver, error) {
    authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseRead)
    if err != nil {
        return nil, err
    }

    caseObj, err := r.caseService.GetCase(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("case not found")
    }

    // Layer 2: Validate workspace ownership
    if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx.WorkspaceID); err != nil {
        return nil, fmt.Errorf("case not found")  // Same error as "not found"
    }

    return &CaseResolver{case_: caseObj, r: r}, nil
}
```

### Service Layer Pattern

Services provide workspace-scoped retrieval methods for defense-in-depth:

```go
// GetCaseInWorkspace retrieves a case with workspace validation.
// Returns NotFoundError if the case doesn't exist or belongs to a different workspace.
func (cs *CaseService) GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*Case, error) {
    caseObj, err := cs.caseStore.GetCase(ctx, caseID)
    if err != nil {
        return nil, apierrors.NotFoundError("case", caseID)
    }

    // Layer 3: Validate workspace ownership
    if caseObj.WorkspaceID != workspaceID {
        return nil, apierrors.NotFoundError("case", caseID)
    }

    return caseObj, nil
}
```

### Store Layer Pattern

Store methods with workspace filtering in SQL for ultimate protection:

```go
// GetCaseInWorkspace retrieves a case only if it belongs to the specified workspace.
func (s *CaseStore) GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*Case, error) {
    query := `SELECT * FROM cases WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
    err := s.db.GetContext(ctx, &model, query, caseID, workspaceID)
    if err != nil {
        return nil, TranslateSqlxError(err, "cases")  // Returns ErrNotFound
    }
    return model.ToDomain(), nil
}
```

### S3 Isolation

Attachments use workspace-prefixed paths:

```
s3://mbr-attachments/
├── {workspace-1}/attachments/...
└── {workspace-2}/attachments/...
```

### Event Isolation

All events include workspace context:

```go
type CaseCreatedEvent struct {
    WorkspaceID string
    CaseID      string
    // ...
}
```

### Admin Bypass for Cross-Workspace Operations

Admin operations (auto-close workers, notifications) use explicit admin context:

```go
func (s *Store) WithAdminContext(ctx context.Context, fn func(context.Context) error) error
```

**SECURITY:** Only use admin context for legitimate cross-tenant administrative operations.

## Consequences

**Positive:**
- Cost efficiency: Single instance for all brands
- Complete data isolation via 4-layer enforcement
- GDPR compliance: Delete workspace = delete all data
- Flexible: Users can belong to multiple workspaces
- Defense-in-depth: Any single layer failure doesn't cause breach
- Consistent error responses prevent enumeration attacks

**Negative:**
- Requires discipline in all code paths
- Must test cross-workspace isolation
- More validation code at each layer

## Testing Requirements

All workspace-scoped operations must be tested for:

1. **Positive case:** Resource accessed by authenticated workspace owner
2. **Negative case:** Resource access attempt by different workspace returns "not found"
3. **Non-existent case:** Non-existent resource returns same "not found" error

```go
func TestCaseWorkspaceIsolation(t *testing.T) {
    // Create case in workspace A
    caseA := createCase(t, workspaceA)

    // Verify workspace A can access
    result, err := service.GetCaseInWorkspace(ctx, workspaceA, caseA.ID)
    require.NoError(t, err)
    assert.Equal(t, caseA.ID, result.ID)

    // Verify workspace B cannot access (returns same error as non-existent)
    _, err = service.GetCaseInWorkspace(ctx, workspaceB, caseA.ID)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not found")

    // Verify non-existent returns same error
    _, err = service.GetCaseInWorkspace(ctx, workspaceA, "non-existent-id")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not found")
}
```

## References

- Workspace middleware: `internal/infrastructure/middleware/`
- Store layer: `internal/infrastructure/stores/sql/`
- GraphQL context helpers: `internal/graph/shared/context.go`
- Service layer: `internal/*/services/`
