# Documentation Reconciliation Checklist

Use this checklist whenever behavior changes are introduced.

## 1. Truth source check
- Primary behavior source: `docs/ARCHITECTURE.md`
- Decision rationale: `docs/ADRs/` + `docs/RFCs/`
- AI/context path: `.ai-context`

## 2. Change impact sweep
For each changed behavior:
- Confirm API and runtime path in `internal/*`, `pkg/*`, `cmd/*`.
- Confirm event names, stream names, and contracts in code match docs.
- Confirm tenancy/rate-limiting/worker behavior claims align with implementation.

## 3. Tests sweep
For each behavioral change:
- Find existing test coverage in unit tests.
- Add/adjust integration test (`-tags=integration`) where store-level or cross-service behavior changed.
- If end-to-end workflows are impacted, update scenario tests.

## 4. Docs integrity sweep
- Remove dead documentation files from canonical lists.
- Replace removed docs with ADR/RFC references or architecture doc updates.
- Run `make docs-check` and fail on broken local links before merge.

## 5. Merge gate
- `make docs-check`
- `go test ./...`
- `go test -v -tags=integration ./...`
