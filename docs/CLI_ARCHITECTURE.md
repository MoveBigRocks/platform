# Move Big Rocks CLI Architecture

The `mbr` CLI is the primary operator and agent surface for Move Big Rocks. It needs to stay predictable for machines, pleasant for humans, and easy to extend without collapsing back into a single giant file.

## Goals

- Keep the CLI contract stable and machine-readable.
- Make each command family easy to test in isolation.
- Keep browser/auth/config/runtime concerns explicit.
- Let new milestone slices add commands without growing `cmd/mbr/main.go` further.

## Design

### 1. Thin entrypoint

- `cmd/mbr/main.go` should contain only shared helpers and cross-cutting helpers that are genuinely used across command families.
- Root command dispatch lives in a dedicated registry in `cmd/mbr/root.go`.
- `main()` should stay a trivial entrypoint.

### 2. Explicit runtime seam

- External effects are injected through function variables in `cmd/mbr/runtime.go`.
- This seam covers:
  - CLI config loading
  - GraphQL client construction
  - HTTP client construction
  - browser opening
- Tests override these seams directly instead of standing up the whole runtime.

### 3. Section-oriented command files

Each major command family should have its own file:

- `root.go`
- `runtime.go`
- `usage.go`
- `auth.go`
- `context.go`
- `workspaces.go`
- `teams.go`
- `agents.go`
- `catalog.go`
- `concepts.go`
- `artifacts.go`
- `forms_spec.go`
- `queues.go`
- `conversations.go`
- `contacts.go`
- `attachments.go`
- `forms.go`
- `automation.go`
- `cases.go`
- `knowledge.go`
- `knowledge_worktree.go`
- `extensions.go`
- `sandboxes.go`

Each section file should own:

- argument parsing for that family
- output formatting for that family
- family-specific GraphQL helper calls
- family-specific output structs when practical

### 4. Shared helpers by concern

Keep shared helpers in small, concern-specific files rather than a generic dumping ground:

- config/context resolution helpers
- input loading helpers
- JSON / table output helpers
- bundle-source helpers
- common formatting helpers

For complex write flows, prefer whole-object payload entrypoints such as
`--input-file` or `--input-json` so agents can apply a deliberate, versionable
payload instead of constructing dozens of flags.

### 5. CLI contract remains authoritative

- `internal/clispec/spec.go` remains the canonical command contract.
- `docs/AGENT_CLI.md` is generated from that contract.
- Every command added to code must be added to the spec in the same slice.

### 6. Work-object actions stay explicit

- Commands should act on `case` and `conversation_session` objects, not on raw
  `queue_item` rows.
- `queue_item` is the routing projection that follows from those object-level
  actions.
- Delegated team routing is a first-class CLI use case: agents may hand off
  cases and conversations through `mbr` on behalf of their human principal
  when the underlying auth and policy model allows it.
- Cases should expose their own durable work thread, with linked source or
  follow-up conversations remaining visible as related interaction context.

## Testing Strategy

### Section tests

- Prefer tests that call one command family at a time with a mocked CLI client.
- Example: `queues`, `conversations`, `knowledge`.

### Contract tests

- Keep `mbr spec export` tests as the contract-level guardrail.
- Assert that required milestone commands are present and correctly typed.

### Runtime seam tests

- Use the runtime seam in `runtime.go` to swap in:
  - mock GraphQL clients
  - fake HTTP clients
  - fake browser openers

### Limit end-to-end blast radius

- Avoid tests that depend on the entire CLI monolith for every change.
- New command families should bring focused tests with them.

## Immediate Direction

The next CLI architecture steps are:

1. Keep `root.go` as the command registry and `runtime.go` as the runtime seam.
2. Keep the already-extracted durable-work families on their own seams:
   `queues`, `conversations`, `cases`, and `forms` should continue evolving in
   their own files instead of drifting back into `main.go`.
3. Keep the knowledge family on its dedicated seam:
   `knowledge.go` owns command parsing and formatting, while
   `knowledge_worktree.go` owns local working-copy behavior.
4. Keep `extensions`, `teams`, `catalog`, `concepts`, and `artifacts` on their
   dedicated seams so new capability lands in focused command-family files
   instead of drifting back into `main.go`.
5. Keep `agents`, `auth`, `context`, `workspaces`, and `health` on dedicated seams as the
   remaining platform-entrypoint families move off `main.go`.
6. Introduce `sandboxes.go` as a new family rather than adding the sandbox
   lifecycle to `main.go`.
7. Keep complex mutation families input-object oriented when that improves
   agent ergonomics and makes the resulting change easy to validate and replay.
8. Keep work-object actions explicit: commands should act on cases or
   conversations, with `queue_item` updating as a consequence.

## Near-Term Extraction Order

To keep Milestone 1 execution moving without turning the CLI into a monolith
again, command-family extraction should happen in this order:

1. `extensions.go`
2. `teams.go`
3. `catalog.go`
4. `concepts.go`
5. `artifacts.go`
6. `sandboxes.go`
7. `agents.go`
8. `auth.go`
9. `context.go`
10. `workspaces.go`
11. `health.go`

The rule is simple: if a new milestone slice introduces a new command family or
substantially expands an existing one, land it on a dedicated file with focused
tests rather than extending `main.go`.

Today that means `extensions.go`, `teams.go`, `agents.go`, `catalog.go`,
`concepts.go`, `artifacts.go`, `sandboxes.go`, `auth.go`, `context.go`,
`workspaces.go`, `health.go`, `contacts.go`, `attachments.go`, `forms.go`, and
`automation.go` should stay extracted, and the next structural targets are the
shared helpers and any future command families that still live in `main.go`.

This keeps milestone delivery moving while improving the CLI structure at the same time.
