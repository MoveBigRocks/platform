# ADR 0005: Event-Driven Architecture

**Status:** Accepted

## Context

Move Big Rocks has multiple bounded contexts that need to integrate without tight coupling:
- Error Monitoring <-> Customer Support
- Email Operations -> Customer Support
- Automation -> All Contexts

Direct service calls create tight coupling and circular dependency risks.

## Decision

**Cross-context integration uses events via the outbox pattern.**

### Architecture

```
┌─────────────┐     ┌──────────────────────────────────┐
│   Service   │────►│  Outbox.Publish()                │
└─────────────┘     │  1. Validate event               │
                    │  2. Save to outbox_events (durable)│
                    └──────────────────────────────────┘
                                    │
                    ┌───────────────▼──────────────────┐
                    │  Outbox Worker (polls DB)        │
                    │  - Fetch pending events          │
                    │  - Call eventbus.Dispatch()      │
                    │  - Mark as published/retry       │
                    └───────────────┬──────────────────┘
                                    │
                    ┌───────────────▼──────────────────┐
                    │  EventBus (in-memory)            │
                    │  - handlers map[stream][]Handler │
                    │  - Dispatch(stream, payload)     │
                    │  - NO database interaction       │
                    └──────────────────────────────────┘
```

**Key Principle:** The Outbox owns durability, the EventBus owns dispatch. Clean separation of concerns.

### In-Memory EventBus

The EventBus runtime is a pure in-memory dispatcher with no database interaction:

```go
type InMemoryBus struct {
    handlers map[string][]Handler // stream -> handlers
    mu       sync.RWMutex
}

// Subscribe registers a handler for a stream
func (b *InMemoryBus) Subscribe(stream Stream, handler Handler)

// Dispatch sends an event to all registered handlers
func (b *InMemoryBus) Dispatch(ctx context.Context, stream Stream, eventType EventType, payload []byte) error
```

In production wiring, container initialization uses `eventbus.NewInMemoryBus()` in `internal/infrastructure/container/container.go`.
The filesystem-backed implementation in `pkg/eventbus/eventbus_filesystem.go` is used for isolated file-backed test scenarios.

### Outbox Service

The outbox service handles durability and retry logic:

```go
// Publish saves event to database - worker dispatches later
func (s *Service) PublishEvent(ctx context.Context, stream Stream, event Event) error {
    // 1. Validate event
    // 2. Save to outbox_events table
    // 3. Return (worker handles dispatch)
}
```

The outbox worker polls for pending events and dispatches them:
- Poll interval: 5 seconds (configurable)
- Exponential backoff for retries
- Max retries before marking as failed
- Dead letter queue for failed events

### Event Streams

| Stream | Purpose |
|--------|---------|
| `issue-events` | Issue created, updated, resolved |
| `case-events` | Case created, updated, resolved, cases bulk resolved |
| `audit-events` | Audit record lifecycle |
| `error-events` | Error ingestion and processing events |
| `alert-events` | Alert triggered |
| `email-events` | Email received, sent |
| `job-events` | Job lifecycle events (started, completed, failed) |
| `permission-events` | Permission checks and role updates |
| `system-events` | System lifecycle and integrity events |
| `metrics` | Telemetry counters for workers/collectors |
| `analytics` | Derived analytics and aggregation events |
| `form-events` | Form submission and form lifecycle events |
| `email-commands` | Request to send email |
| `case-commands` | Request to create case |
| `notification-commands` | Request to send notification |

### Publishing Events

```go
type AlertService struct {
    outbox OutboxPublisher
}

func (s *AlertService) TriggerAlert(alert *Alert) error {
    s.store.Alerts().CreateAlert(alert)
    s.outbox.PublishEvent(ctx, eventbus.StreamAlertEvents, AlertTriggeredEvent{...})
    return nil  // Handler processes async
}
```

### Event Handlers

Handlers are registered at startup and invoked by the outbox worker:

```go
func (m *Manager) handleCaseEvents(ctx context.Context, payload []byte) error {
    var event events.CaseEvent
    json.Unmarshal(payload, &event)

    switch event.Type {
    case events.CaseCreated:
        return m.onCaseCreated(ctx, event)
    case events.CaseResolved:
        return m.onCaseResolved(ctx, event)
    }
    return nil
}
```

### Service Interfaces for Synchronous Operations

When mutations require validation or immediate feedback:

```go
type RulesEngine struct {
    caseService contracts.CaseServiceInterface
}

func (re *RulesEngine) executeAction(...) {
    re.caseService.AssignCase(ctx, caseID, userID, teamID)
}
```

## Guidelines

**Use events for:**
- Cross-context side effects
- Notifications
- Audit logging
- Background processing

**Use service interfaces for:**
- Cross-context mutations needing validation
- Synchronous operations

**Event conventions:**
- Past tense names (CaseCreated, not CreateCase)
- Include workspace_id
- Handlers must be idempotent

## Consequences

**Positive:**
- Loose coupling between contexts
- Clean separation: outbox (durability) vs eventbus (dispatch)
- Easy testing (mock OutboxPublisher)
- Audit trail in outbox_events table
- No complex database hooks in eventbus

**Negative:**
- Eventual consistency (poll interval latency)
- Requires idempotent handlers

## References

- In-Memory EventBus runtime: `pkg/eventbus/bus.go`
- Filesystem EventBus (test scenarios): `pkg/eventbus/eventbus_filesystem.go`
- Outbox Service: `internal/infrastructure/outbox/outbox.go`
- Worker Manager: `internal/workers/manager.go`
- Event types: `internal/shared/events/`
- Service interfaces: `internal/shared/contracts/`
