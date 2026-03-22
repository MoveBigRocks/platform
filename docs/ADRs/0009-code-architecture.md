# ADR 0009: Code Architecture Patterns

**Status:** Accepted

## Context

As the codebase grows, we need consistent patterns for:
- Separating HTTP handling from business logic
- Organizing domain logic within entities
- Predictable request flow through layers

## Decision

### Layered Architecture

Four layers with downward-only dependencies:

```
┌─────────────────────────────────────────┐
│           HANDLER LAYER                 │  HTTP concerns only
│   internal/*/handlers/                  │
├─────────────────────────────────────────┤
│           SERVICE LAYER                 │  Orchestration, validation
│   internal/*/services/                  │
├─────────────────────────────────────────┤
│           DOMAIN LAYER                  │  Business logic, entities
│   internal/*/domain/                    │
├─────────────────────────────────────────┤
│           STORE LAYER                   │  Persistence abstraction
│   internal/infrastructure/stores/       │
└─────────────────────────────────────────┘
```

**Layer Responsibilities:**

| Layer | Responsibility |
|-------|----------------|
| Handler | Parse HTTP, call services, format responses |
| Service | Validate inputs, orchestrate domain, persist, publish events |
| Domain | Business logic, state transitions, calculations |
| Store | Database operations, model mapping |

**Dependency Rules:**
- Handlers -> Services -> Domain
- Stores -> Domain
- Domain imports nothing (except stdlib)

### Rich Domain Models

Domain entities contain business logic, not just data:

```go
// Good: Business logic in domain
func (c *Case) MarkResolved(resolvedAt time.Time) error {
    if c.Status == CaseStatusResolved || c.Status == CaseStatusClosed {
        return fmt.Errorf("case is already %s", c.Status)
    }
    c.Status = CaseStatusResolved
    c.ResolvedAt = &resolvedAt
    return nil
}

// Service is thin orchestrator
func (s *CaseService) MarkCaseResolved(ctx context.Context, caseID string) error {
    return s.withCase(ctx, caseID, func(c *Case) error {
        return c.MarkResolved(time.Now())  // Delegates to domain
    })
}
```

**Domain models contain:**
- Validation rules (`Validate()`)
- State transitions (`MarkResolved()`, `Assign()`)
- Calculations (`IsOverdue()`)

**Services contain:**
- Input validation
- Orchestrating domain operations
- Calling stores
- Publishing events

### Request Flow Pattern

```
HTTP Request
    │
    ▼
HANDLER: Extract params, call service, format response
    │
    ▼
SERVICE: Validate, load entity, call domain, persist, publish events
    │
    ▼
DOMAIN: Validate operation, perform transition, return error if invalid
    │
    ▼
STORE: Map models, execute SQL, return result
```

### Service Helper Pattern

For mutations, use the `withEntity` pattern:

```go
func (s *CaseService) withCase(ctx context.Context, id string, mutate func(c *Case) error) error {
    caseObj, err := s.caseStore.GetCase(ctx, id)
    if err != nil {
        return err
    }

    if err := mutate(caseObj); err != nil {
        return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "case mutation failed")
    }

    return s.caseStore.UpdateCase(ctx, caseObj)
}
```

### Error Handling

```go
// Domain returns plain errors
func (c *Case) Assign(userID string) error {
    if userID == "" {
        return fmt.Errorf("user_id is required")
    }
    // ...
}

// Service wraps with error types
if err := mutate(entity); err != nil {
    return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "operation failed")
}

// Handler maps to HTTP status
if err != nil {
    middleware.RespondWithError(c, http.StatusBadRequest, err.Error())
    return
}
```

## Consequences

**Positive:**
- Clear separation of concerns
- Each layer testable independently
- Domain logic is portable
- Consistent patterns across modules

**Negative:**
- More files for simple features
- Learning curve for new developers

## References

- Service-layer services: `internal/service/services/`
- Service domain: `internal/service/domain/`
- Case store: `internal/infrastructure/stores/sql/case_store.go`
