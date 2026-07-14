# Testing Strategy

## Purpose

Move Big Rocks needs workflow proof, not just implementation proof.

A capability is not "done" because:

- a domain/service method passes
- a handler stores a record
- a command event is published
- a scenario runner script simulates the final state

A capability is only credible when the real production path completes through
the entrypoint, parser/adapter, store, outbox, worker/consumer, and durable
side effect that users depend on.

## The Failure Mode This Strategy Prevents

Tests can prove the shape of a feature without proving the user-visible
workflow. This strategy exists to catch that failure mode, which shows up as:

- tests prove that email or notification commands are queued, but not that a
  consumer exists and executes them
- tests prove inbound case matching when `InReplyTo` and `References` are set
  manually, but not that the real provider parser populates them
- scenario tests often simulate success by writing final state directly instead
  of traversing the production path

These gaps make capability claims look stronger than they are. The
workflow-proof model keeps evidence honest by requiring the real production path
to complete.

## Capability Proof Model

Use these proof levels consistently:

| Level | What it proves | What it does **not** prove |
| --- | --- | --- |
| `Component` | Domain logic, validation, pure helpers, single-service behavior | Cross-service or user-visible workflow completion |
| `Storage / Integration` | PostgreSQL persistence, tenancy, migrations, store queries | Handler, worker, provider, and outbox wiring |
| `Adapter / Handler` | HTTP/webhook/form entrypoint parsing and transaction behavior | Downstream worker execution or provider side effects |
| `Consumer / Worker` | A command or event has a registered consumer and is processed | End-to-end round trip from real entrypoint |
| `Workflow` | The production path completes end-to-end for a scoped user workflow | External-provider production reliability across the internet |
| `Operational proof` | Workflow proof plus archived artifact(s) in milestone/release evidence | Broader product claims outside the archived workflow set |

A capability is credible for release only when all required proof levels for its
workflow are present in CI, and release-critical flows are represented in the
proof bundle.

If a workflow stops at "event queued" or "submission stored", it is only
`Partially evidenced`.

## Test Types

### Unit Tests

Use for:

- domain models
- validation rules
- pure transformation helpers
- small policy evaluators

### Store / Integration Tests

Use for:

- real PostgreSQL behavior
- tenancy and RLS behavior
- migration coverage
- query semantics such as case lookup by message ID or subject

### Handler / Adapter Tests

Use for:

- HTTP handlers
- webhook parsing
- public form submission entrypoints
- GraphQL or CLI entry adapters where they contain workflow logic

These tests should use the real handler plus real storage. They should not be
counted as workflow proof unless they also verify downstream worker or provider
effects.

### Worker / Consumer Tests

Use for:

- event bus subscription registration
- outbox-dispatched command processing
- consumer idempotency
- durable side effects produced by workers

Every production command stream must have at least one consumer test.

### Workflow Tests

These are the minimum proof gate for user-visible behavior.

A workflow test must drive the same production path the feature uses in real
operation. For example:

- webhook -> store pending record -> event bus/outbox -> worker -> case update
- admin reply -> case communication -> outbound email record -> send command ->
  provider send -> provider status update
- public form -> submission -> form worker -> case creation -> notification send

Workflow tests may still use mock providers for systems we do not own, but they
must not skip any internal step we do own.

### External / Provider Contract Tests

Use sparingly for:

- Postmark/SendGrid/SES request and response shape checks
- third-party SDK ingestion compatibility

These are helpful confirmation tests, but they do not replace internal workflow
proof.

## Scenario Runner Repositioning

The scenario runner remains useful, but its role changes.

It should be treated as:

- synthetic-data generation
- exploratory or smoke validation
- broad regression exercises across many entities

It must **not** be described as end-to-end proof when it simulates final state
directly instead of traversing the real production path.

When scenario steps write the result state manually, the scenario is a
simulation, not a workflow proof.

## Stream Parity Rule

For every production command stream, the repo must have all of the following:

1. A producer test proving the command/event is emitted with correct payload.
2. A consumer registration test proving a real worker subscribes to the stream.
3. A consumer execution test proving the worker performs the durable side
   effect.
4. A round-trip workflow test proving the full user flow reaches that consumer.
5. A failure-path test proving retry, failure state, or rollback is visible and
   durable.

If a stream has producers but no consumers, that is a release blocker for any
workflow that depends on it.

## Workflow Evidence

User-visible workflow claims need automated end-to-end evidence, not just
component tests. A workflow is credible only when an automated test drives its
full production path, from the real entrypoint through storage, outbox, and
worker/consumer to the durable side effect.

Evidence should cover complete product loops, not only the narrowest
command-driven subset. At minimum this includes:

- the operator case loop, including manual create and work management,
  handoff/status, and attachment-bearing case flows
- conversation reply, handoff, escalation, and public intake
- attachment-bearing operational flows, including first-party extension intake
  paths such as ATS candidate resumes
- sanctioned extension-or-agent initiated case actions when the architecture
  promises them

## CI And Proof Gates

### Merge Gate

The merge gate requires:

1. `make docs-check`
2. unit and store/integration tests
3. the full integration sweep (`go test -tags=integration ./...`) hard-gated in
   CI
4. workflow tests for affected scoped capabilities
5. command-stream parity checks for any changed producer/consumer paths
6. build verification

### Milestone Proof Gate

Milestone proof must not rely only on package tests plus publication artifacts.
For milestone-critical workflows it should archive machine-readable outputs that
prove:

- the real entrypoint was exercised
- the worker/consumer ran
- the durable side effect exists
- the resulting state is queryable from the system of record
- command failure or retry state remains visible when the workflow cannot
  complete

## Ongoing Guardrails

1. Keep user-visible workflows aligned with their automated end-to-end evidence.
2. Add or update workflow artifacts whenever an entrypoint, consumer, or
   durable side effect changes.
3. Do not add new producer streams without a real consumer, consumer test, and
   workflow proof.
4. Treat scenario-runner flows as smoke coverage only unless they traverse the
   real production path.
5. Keep the full integration sweep green so CI can continue to hard-gate it.
6. For any proof that depends on canonical sibling repositories, make the
   committed CI workflow materialize those repos and make proof mode fail when
   they are absent.

## Running Tests

```bash
# All tests
go test ./...

# Current CI-style sweep
go test -v -short -race ./...
go test -v -tags=integration ./...

# Coverage
go test ./... -cover

# Scenario runner (synthetic smoke, not workflow proof by itself)
./scenario-runner --clean --scenarios
```

## Documentation Reconciliation Checklist

Run this whenever behavior changes are introduced.

1. **Truth source check**
   - Keep the primary behavioral description in `docs/ARCHITECTURE.md`.
   - Put decision rationale in `docs/ADRs/` or `docs/RFCs/`.
   - Prefer code-level contracts and generated specs over prose when they
     exist.

2. **Workflow evidence check**
   - If the change affects a user-visible workflow, add or update its automated
     end-to-end evidence.
   - Do not treat a workflow as credible without listing its automated evidence.

3. **Change impact sweep**
   - Verify all behavior claims against the implementation in `internal/`,
     `pkg/`, and `cmd/`.
   - Confirm event names, stream names, auth rules, tenancy rules, and runtime
     contracts still match code.
   - Confirm every newly produced command stream has a registered consumer.

4. **Test sweep**
   - Confirm updated or added behavior is covered by unit tests.
   - Add or adjust integration coverage when store-level or cross-service
     behavior changed.
   - Add workflow proof when the user-facing workflow changed.
   - Do not treat simulated scenarios as workflow proof.

5. **Docs integrity sweep**
   - Remove dead docs from canonical lists instead of leaving stale references
     in place.
   - Replace removed docs with updates to `docs/ARCHITECTURE.md`,
     `docs/ADRs/`, or `docs/RFCs/` when the material is still important.
   - Run `make docs-check` to catch broken local links before merge.

6. **Merge gate**
   - `make docs-check`
   - `go test ./...`
   - `go test -v -tags=integration ./...`
   - affected workflow tests

## Integration Test Tag

Tests that need database/resources can use `//go:build integration`.

- Unit-only: `go test ./...`
- Integration-tagged: `go test -tags=integration ./...`

Use additional workflow-specific test organization only when it materially
improves signal or runtime, not as a substitute for defining the workflow proof
itself.
