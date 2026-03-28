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

Run this whenever behavior changes are introduced.

1. **Truth source check**
   - Keep the primary behavioral description in `docs/ARCHITECTURE.md`.
   - Put decision rationale in `docs/ADRs/` or `docs/RFCs/`.
   - Prefer code-level contracts and generated specs over prose when they
     exist.

2. **Change impact sweep**
   - Verify all behavior claims against the implementation in `internal/`,
     `pkg/`, and `cmd/`.
   - Confirm event names, stream names, auth rules, tenancy rules, and runtime
     contracts still match code.

3. **Test sweep**
   - Confirm updated or added behavior is covered by existing unit tests.
   - Add or adjust integration coverage when store-level or cross-service
     behavior changed.
   - Update scenario or end-to-end coverage when the user-facing workflow
     changed.

4. **Docs integrity sweep**
   - Remove dead docs from canonical lists instead of leaving stale references
     in place.
   - Replace removed docs with updates to `docs/ARCHITECTURE.md`,
     `docs/ADRs/`, or `docs/RFCs/` when the material is still important.
   - Run `make docs-check` to catch broken local links before merge.

5. **Merge gate**
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
