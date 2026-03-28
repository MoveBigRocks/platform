# Workflow Proof Closure Plan

This document is the execution plan for closing the operational workflow gaps
tracked in [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md).

It exists to answer a simple question: what must change, in what order, before
the repo can honestly claim that milestone-facing operational workflows work
end to end.

## Status

Closed on 2026-03-28.

The phases below were completed in order:

- entrypoint and integration fixtures were repaired so the full integration
  sweep is green
- `email-commands` and `notification-commands` now have production consumers
  plus workflow proof
- Postmark reply threading is proven through the real webhook path
- milestone proof archives machine-readable operational workflow artifacts
- CI now hard-gates `go test -tags=integration ./...`

## Closure Principles

- Do not mark a workflow `Proven` until the production path is implemented and
  exercised end to end.
- Do not treat "event published" as successful completion when no consumer is
  registered.
- Do not treat scenario-runner simulations as workflow proof.
- Do not hard-gate CI on the full integration sweep until the known broken
  integration fixtures are repaired.
- Every closure phase must end with automated proof and, where relevant,
  milestone-proof artifacts.

## Workstreams

The closure work breaks into three linked workstreams:

1. Runtime correctness: the missing consumers, persistence, and parser behavior.
2. Workflow proof: end-to-end tests that exercise the real production path.
3. Signal integrity: CI and milestone proof must report the real state of the
   repo and archive the relevant artifacts.

## Execution Order

### Phase 0: Restore CI Baseline

Goal: make the current integration suite credible enough to use as a real
signal while later workflow work is landing.

Changes:

- Repair broken integration fixtures in `internal/platform/services`,
  especially tests that use non-UUID workspace IDs.
- Keep soft-gating until the package-level integration failures are resolved,
  but ensure summaries and workflow outputs continue to distinguish
  hard-gated versus soft-gated checks.

Acceptance criteria:

- `go test -tags=integration ./internal/platform/services -count=1` passes.
- The remaining `go test -tags=integration ./...` failures, if any, are known,
  enumerated, and reduced to a short list rather than a broad fixture problem.

Evidence:

- green package-level integration test log for `./internal/platform/services`
- updated CI summary outputs in [`.github/workflows/_test.yml`](../.github/workflows/_test.yml)

### Phase 1: Close `email-commands` Runtime Gaps

Goal: make outbound email workflows real instead of "published and forgotten".

Changes:

- Implement a production `email-commands` consumer and register it in the
  embedded worker manager.
- Persist an `outbound_emails` record before or as part of command handling for
  case replies, form notifications, and rule-driven emails.
- Correlate provider message IDs and provider status updates back to the stored
  outbound email.
- Ensure user-facing responses reflect queued-versus-sent truth rather than
  implying delivery when only persistence succeeded.

Acceptance criteria:

- A case reply results in a durable outbound email record plus a processed email
  command.
- A form notification results in a durable outbound email record plus a
  processed email command.
- A rule `send_email` action results in a durable outbound email record plus a
  processed email command.
- Delivery and bounce handlers can update the same outbound email record by
  provider message ID.

Evidence:

- consumer registration and execution tests for `email-commands`
- end-to-end workflow tests for case reply, form notification, and rule email
- archived workflow artifact(s) showing outbound email creation, send result,
  and provider correlation

### Phase 2: Fix Inbound Reply Threading

Goal: make real provider payloads thread back to the correct case.

Changes:

- Update the Postmark parser to populate `InReplyTo`, `References`, canonical
  sender email, and sender display name from real webhook payloads.
- Add reply-thread workflow proof using the actual webhook handler plus worker.
- Confirm case reopen/open transitions and duplicate-case avoidance.

Acceptance criteria:

- A Postmark reply payload with `In-Reply-To` and `References` updates the
  correct existing case.
- The same test proves no duplicate case is created.
- Subject-only fallback remains covered for cases where headers are absent.

Evidence:

- webhook-to-worker end-to-end test using real Postmark-shaped payloads
- archived workflow artifact showing inbound email record, matched case ID, and
  resulting case status

### Phase 3: Close `notification-commands` Runtime Gaps

Goal: make non-email notifications real instead of aspirational contracts.

Changes:

- Implement a production `notification-commands` consumer and register it in
  the worker manager.
- Persist a durable notification side effect or notification-delivery record
  that the system can query after processing.
- Route knowledge-review notifications through that consumer path.

Acceptance criteria:

- A knowledge review action emits a notification command that is consumed and
  results in durable notification state.
- The resulting notification can be queried from the system of record.

Evidence:

- consumer registration and execution tests for `notification-commands`
- workflow proof for knowledge-review notifications
- archived workflow artifact showing notification command consumption and stored
  delivery state

### Phase 4: Replace Simulated Workflow Claims

Goal: stop relying on tests and scenarios that manually write the final state.

Changes:

- Reclassify synthetic scenarios in the scenario runner as smoke-only where they
  remain useful.
- Replace milestone-facing simulated email/form scenarios with real workflow
  tests that drive production entrypoints and consumers.
- Remove or downgrade any doc language that still describes simulation as
  end-to-end proof.

Acceptance criteria:

- No milestone-facing workflow is marked `Proven` based only on scenario-runner
  simulation.
- Each row in [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md) has
  a concrete automated test path attached to it.

Evidence:

- updated scenario-runner descriptions
- workflow tests linked from the matrix for every scoped operational row

### Phase 5: Extend Milestone Proof

Goal: make milestone proof archive operational workflow artifacts, not just
package tests and release evidence.

Changes:

- Extend [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh)
  to generate and archive machine-readable artifacts for:
  - inbound-new-email case creation
  - case reply send flow
  - inbound reply threading
  - form submission notification delivery
  - knowledge review notification delivery
- Update [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) with the new
  artifact set and rerun instructions.

Acceptance criteria:

- The proof bundle contains workflow artifacts for every operational workflow
  row still in scope.
- The proof summary points to those artifact directories explicitly.

Evidence:

- updated proof bundle layout
- successful local and CI proof rerun with non-empty workflow artifact outputs

### Phase 6: Harden The Gate

Goal: promote the full integration and workflow suite from soft signal to real
gate once the above work is stable.

Changes:

- Remove soft-gating from the integration sweep when the suite is consistently
  green.
- Add workflow tests for scoped operational flows to the merge gate.
- Only then consider Milestone 1 closed again in readiness docs.

Acceptance criteria:

- `go test -tags=integration ./...` passes in CI without `continue-on-error`.
- Affected operational workflow tests run in CI and pass.
- [`docs/MILESTONE_1_READINESS.md`](./MILESTONE_1_READINESS.md) can move the
  operational workflow area from `Open` to `Proven`.

Evidence:

- green CI run with hard-gated integration and workflow coverage
- final milestone proof bundle with operational workflow artifacts

## Dependency Notes

- Phase 1 must land before Phase 2 can be considered complete, because reply
  threading must round-trip against real outbound records and provider message
  IDs.
- Phase 3 can proceed in parallel with Phase 2 once the notification side
  effect contract is chosen.
- Phase 5 should start after Phases 1 through 3 have produced machine-readable
  workflow outputs worth archiving.
- Phase 6 is last on purpose; making CI stricter before runtime and workflow
  gaps are fixed would create noise instead of confidence.

## Definition Of Closed

The operational workflow gap is closed only when all of the following are true:

- `email-commands` has a real registered consumer plus workflow proof.
- `notification-commands` has a real registered consumer plus workflow proof.
- inbound reply threading is proven through a real provider-shaped webhook path.
- scenario-runner simulations are no longer represented as workflow proof.
- the milestone proof archives machine-readable operational workflow artifacts.
- the full integration suite is green enough to be hard-gated in CI.

All of those conditions are now satisfied on `main`.
