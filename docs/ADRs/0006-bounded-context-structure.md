# ADR 0006: Bounded Context Package Structure

**Status:** Accepted

## Context

Move Big Rocks combines multiple product domains:
- Customer support (cases, contacts, email)
- Error monitoring (Sentry-compatible)
- Knowledge resources
- Automation (rules, jobs, workflows)
- Platform (users, workspaces, auth)

Without clear boundaries, these domains become tightly coupled and difficult to maintain.

## Decision

**Organize code by bounded context (domain) with consistent internal structure.**

```
internal/
├── automation/          # Rules, jobs, workflows
│   ├── domain/          # Business entities and logic
│   ├── services/        # Application services
│   └── handlers/        # Event handlers
│
├── knowledge/           # Knowledge resources and retrieval metadata
│   ├── domain/
│   └── services/
│
├── observability/       # Error monitoring (Sentry-compatible)
│   ├── domain/
│   ├── services/
│   └── handlers/
│
├── platform/            # Users, workspaces, auth, admin
│   ├── domain/
│   ├── services/
│   └── handlers/
│
├── service/             # Cases, conversations, contacts, forms, email
│   ├── domain/
│   ├── services/
│   └── handlers/
│
├── shared/              # Shared kernel
│   ├── domain/          # Types used across contexts
│   ├── contracts/       # Service interfaces for cross-context deps
│   └── events/          # Event type definitions
│
├── infrastructure/      # Cross-cutting concerns
│   ├── container/       # Dependency injection
│   ├── stores/          # Database layer
│   │   ├── shared/      # Store interfaces
│   │   └── sql/         # SQL store implementations
│   └── ...
│
└── workers/             # Embedded event processors
```

### Context Internal Structure

| Directory | Purpose |
|-----------|---------|
| `domain/` | Business entities, value objects, domain logic (no I/O) |
| `services/` | Coordination layer - calls stores, domain methods, publishes events |
| `handlers/` | HTTP/event handlers (calls services only) |

### Bounded Context Responsibilities

| Context | Responsibility |
|---------|---------------|
| `automation` | Assignment rules, job scheduling, workflow execution |
| `knowledge` | Knowledge resources, retrieval, provenance, search |
| `observability` | Error ingestion, issue grouping, alerting |
| `platform` | Users, workspaces, teams, permissions, audit |
| `service` | Cases, conversations, contacts, email threading, structured forms |

### Communication Between Contexts

Contexts communicate via:
1. **Events** (async, preferred for side effects) - see ADR-0005
2. **Service interfaces** (sync, for mutations needing validation)

```go
    // Good: Events for async side effects
    s.outbox.Publish(eventbus.StreamCaseEvents, events.CaseCreated{...})

// Good: Service interface for cross-context mutations
type RulesEngine struct {
    caseService contracts.CaseServiceInterface
}

// Bad: Direct import of concrete service
import "github.com/movebigrocks/platform/internal/service/services"  // Creates tight coupling
```

### Allowed Imports

- Any context -> `shared/domain` (shared kernel types)
- Any context -> `shared/contracts` (service interfaces)
- Any context -> `shared/events` (event definitions)
- Any context -> `infrastructure/*` (stores, outbox, etc.)

## Consequences

**Positive:**
- Clear code ownership per context
- Easier onboarding - learn one context at a time
- Safe refactoring - changes isolated to context
- Testable - contexts test in isolation

**Negative:**
- Requires discipline to maintain boundaries
- Some code duplication between contexts
- Event-driven communication adds complexity

## References

- Automation: `internal/automation/`
- Knowledge: `internal/knowledge/`
- Observability: `internal/observability/`
- Platform: `internal/platform/`
- Service: `internal/service/`
- Shared kernel: `internal/shared/`
- Event architecture: ADR-0005
