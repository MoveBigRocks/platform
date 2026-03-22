# Testing Strategy

## Philosophy

We aim for **50%+ test coverage** but accept less for now. Coverage improves through a risk-based approach:

1. Storage layer
2. Domain logic
3. Integration workflows via scenario runner

## Testing Principles

### Use Real Storage, Not Mocks

Tests use a real PostgreSQL test database where appropriate. We only mock external systems we do not own (email providers, third-party APIs).

### Avoid Over-Engineering Tests

- Do not over-test thin HTTP wrappers.
- Do not create heavy mock graphs for internal interfaces.
- Prefer behavior tests over implementation-detail tests.

## Test Types

### Unit Tests

Domain models, business rules, utilities.

### Store/Integration Tests

Storage behavior with PostgreSQL.

### E2E Scenario Tests

Full workflow exercises through the scenario runner:

```bash
./scenario-runner --clean --workspaces=1 --cases=10 --scenarios
```

## Running Tests

```bash
# All tests
go test ./...

# Verbose short-mode CI equivalent
go test -v -short -race ./...

# Coverage
go test ./... -cover

# Integration
go test -v -tags=integration ./...

# E2E scenarios
./scenario-runner --clean --scenarios
```

## CI Integration

Tests run on every push and PR. Coverage is reported but not hard-gated.

## Documentation Reconciliation Checklist

Run this when behavior changes are made:

1. **Docs updates**
   - Keep behavioral description in `docs/ARCHITECTURE.md`.
   - Put decision rationale updates in `docs/ADRs/` or `docs/RFCs/`.
   - Remove stale references to deleted docs (`MIGRATIONS.md`, `DOMAIN_MODEL.md`, `EVENTS.md`, `TYPE_SAFE_EVENTS_IMPLEMENTATION.md`).

2. **Code truth check**
   - Verify all behavior claims against implementation in:
     - `internal/`
     - `pkg/`
     - `cmd/`
   - Prefer constants/types from code (`pkg/eventbus/interface.go`, store interfaces, ADR-indexed config docs) over prose.

3. **Test parity check**
   - Confirm updated/added behavior is covered by tests in:
     - existing unit tests
     - relevant integration tests (`-tags=integration`)
     - scenario/e2e coverage where applicable

4. **Docs link integrity**
   - Run `make docs-check` to catch stale links to removed files.

5. **Command sequence before merge**
   - `make docs-check`
   - `go test ./...`
   - `go test -v -tags=integration ./...`
   - `go test -v -short -race ./...` (optional if local/runtime budget allows)

## Integration Test Tag

Tests that need database/resources can use `//go:build integration`:

- Unit-only: `go test ./...`
- Integration-tagged: `go test -tags=integration ./...`

### Template

```go
//go:build integration

package mypackage

import (
    "testing"
)

func TestMyFeature(t *testing.T) {
    // setup PostgreSQL-backed store and validate behavior
}
```
