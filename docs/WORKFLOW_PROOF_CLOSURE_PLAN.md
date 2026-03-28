# Workflow Proof Closure Plan

This document is the execution plan for closing the operational workflow gaps
tracked in [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md).

It exists to answer a simple question: what must change, in what order, before
the repo can honestly claim that milestone-facing operational workflows work
end to end.

## Status

Reopened on 2026-03-28 under the expanded product-complete Milestone 1 bar.

The earlier command-driven closure work remains done:

- entrypoint and integration fixtures were repaired so the full integration
  sweep is green
- `email-commands` and `notification-commands` now have production consumers
  plus workflow proof
- Postmark reply threading is proven through the real webhook path
- milestone proof archives machine-readable operational workflow artifacts
- CI now hard-gates `go test -tags=integration ./...`

That work is now treated as the foundation for the broader product-complete
closure sequence below.

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

The reopened closure work breaks into four linked workstreams:

1. Runtime correctness: the missing consumers, persistence, and parser behavior.
2. Product surface completeness: the supported CLI, API, and admin paths must
   expose the full operator loop.
3. Workflow proof: end-to-end tests that exercise the real production path.
4. Signal integrity: CI and milestone proof must report the real state of the
   repo and archive the relevant artifacts.

## Execution Order

### Phase 0: Preserve The Honest Baseline

Goal: keep the already repaired integration and proof baseline honest while the
expanded milestone target is being implemented.

Changes:

- Keep the current hard-gated integration sweep green.
- Do not mark Milestone 1 closed while the expanded product-complete rows remain
  open.
- Keep readiness and proof docs aligned with the reopened target.

Acceptance criteria:

- `go test -tags=integration ./... -count=1` remains green.
- The reopened gaps are reflected in the scope, readiness, proof, and workflow
  matrix docs.

Evidence:

- green full integration test log
- synced milestone and workflow-proof docs

### Phase 1: Complete The Case Operator Loop

Goal: make case operations complete on the supported product surface rather than
only rich in the service layer.

Progress on 2026-03-28:

- `mbr cases` now exposes manual create, assign, unassign, set priority,
  add-note, and reply on the supported CLI surface.
- Operator workflow proof now exists for manual case creation plus
  assign/unassign, reprioritization, and internal note flows via
  [`internal/service/resolvers/case_operator_workflow_integration_test.go`](../internal/service/resolvers/case_operator_workflow_integration_test.go).
- Supported case handoff and lifecycle status transitions are now also proven
  through the same resolver workflow proof, with archived artifacts for
  `case-operator-handoff.json` and `case-operator-status-transition.json`.
- The milestone-proof artifact set now includes
  `workflow-proof/case-operator-manual-create.json`,
  `workflow-proof/case-operator-work-management.json`, and
  `workflow-proof/case-operator-reply.json`,
  `workflow-proof/case-operator-handoff.json`, and
  `workflow-proof/case-operator-status-transition.json`.
- The case thread persistence path now durably preserves `from_agent_id` for
  agent-authored communications, which the new operator workflow proof asserts.
- The supported assignment and handoff paths now validate real routing targets
  instead of accepting phantom team IDs, and the supported status mutation now
  follows real case lifecycle semantics for resolve, close, and reopen.

Changes:

- Expose manual case creation, assignment, unassignment, set priority, add
  internal note, reply, handoff, and status transitions through the supported
  CLI and keep API/admin parity explicit.
- Ensure the supported surfaces present the case thread and queue state
  coherently after those actions.
- Add workflow-proof rows and machine-readable artifacts for the operator case
  loop.

Acceptance criteria:

- `mbr` exposes the full in-scope case operator loop.
- Equivalent admin/API paths remain supported and tested.
- The milestone proof bundle archives workflow artifacts for manual create and
  case work-management actions, not only reply/send paths.

Evidence:

- CLI and API tests for the expanded case command set
- workflow tests for case create, assignment, priority change, internal note,
  and queue-visible follow-through
- archived workflow artifacts for those case loops

Remaining Phase 1 work:

- none inside the non-attachment operator loop; the remaining case-surface gap
  is the attachment-bearing loop tracked in Phase 2

### Phase 2: Complete Attachment-Bearing Workflows

Goal: make attachments part of the real supported work loop instead of an
unproven support surface.

Progress on 2026-03-28:

- Manual case attachment upload is now proven through the supported upload
  entrypoint, durable metadata persistence, and the supported case query
  surface via
  [`internal/service/handlers/attachment_upload_handler_integration_test.go`](../internal/service/handlers/attachment_upload_handler_integration_test.go).
- Inbound email attachment ingest is now proven through the real Postmark
  webhook path, including durable audit visibility for rejected attachments, via
  [`internal/service/handlers/postmark_webhooks_integration_test.go`](../internal/service/handlers/postmark_webhooks_integration_test.go).
- The milestone-proof artifact set now includes
  `workflow-proof/case-operator-attachment-upload.json` and
  `workflow-proof/inbound-email-attachments.json`.
- Attachment IDs are now durable at creation time, successful uploads persist
  the real S3 storage key, inbound webhook ingest persists attachment metadata,
  and inbound case threading links those attachment records back to the case and
  communication surfaces operators use.

Changes:

- Add workflow-proof rows for manual case attachment upload, inbound email
  attachment ingest, and the ATS resume path.
- Ensure attachment linkage is visible from the case or candidate record and is
  represented in proof artifacts.
- Preserve failure visibility for scanning or storage failures.

Acceptance criteria:

- Supported attachment uploads are visible as durable linked records.
- Inbound email attachments survive the real webhook path with auditable success
  and failure states.
- ATS resume and portfolio uploads remain part of the milestone proof chain.

Evidence:

- workflow tests and archived artifacts for manual and inbound attachment flows
- ATS proof artifact updated to include attachment-bearing candidate evidence

Remaining Phase 2 work:

- ATS resume and portfolio uploads still need equivalent workflow proof so the
  attachment workstream can be marked fully closed

### Phase 3: Complete Conversation Workflows And Public Intake

Goal: make supervised conversation a real core product loop instead of mainly a
public promise plus isolated service behavior.

Changes:

- Add workflow-proof rows for conversation reply, handoff, and escalation with
  queue parity and provenance preservation.
- Implement and prove one supported public conversation intake surface owned by
  core, such as the website widget or equivalent public conversation adapter.
- Ensure the public intake path lands in a core-owned conversation record and
  can continue through the same operator surfaces.

Acceptance criteria:

- A supported public conversation intake path exists and is proven.
- Conversation reply, handoff, and escalation each have milestone-proof rows and
  archived artifacts.
- Escalation produces a linked case without losing conversation provenance.

Evidence:

- workflow tests plus archived artifacts for conversation reply, handoff, and
  escalation
- workflow test plus archived artifact for public conversation intake

### Phase 4: Implement The Sanctioned Case Action Contract

Goal: make the architectural promise about extension and agent initiated core
case actions true.

Progress on 2026-03-28:

- `case-commands` is now the chosen Milestone 1 sanctioned case-action
  mechanism for case creation, and it now has a production consumer in
  [`internal/service/handlers/case_command_handler.go`](../internal/service/handlers/case_command_handler.go).
- The worker manager and container startup path now wire `case-commands`
  alongside the other production command streams via
  [`internal/workers/manager_test.go`](../internal/workers/manager_test.go) and
  [`internal/infrastructure/container/container_integration_test.go`](../internal/infrastructure/container/container_integration_test.go).
- The repo now proves a real producer -> consumer -> durable case result flow
  plus a failure-visible retry path via
  [`internal/service/handlers/case_command_handler_test.go`](../internal/service/handlers/case_command_handler_test.go),
  archived as `workflow-proof/case-command-create.json` and
  `workflow-proof/case-command-failure-visible.json`.
- The contract now carries queue-routing metadata and emits a durable
  `case.created_from_command` response event after successful case creation.

Changes:

- Choose the sanctioned case-action mechanism for Milestone 1 and finish it.
  This can be `case-commands` if that remains the preferred contract, or an
  explicit replacement if the stream contract changes.
- Implement a production consumer and the durable result path.
- Add at least one real extension-or-agent initiated case-action workflow row.

Acceptance criteria:

- The architecture no longer promises a placeholder.
- An extension or agent can request an in-scope case action through the
  sanctioned contract and the repo proves it end to end.
- `case-commands` is either proven or removed as the preferred milestone
  mechanism.

Evidence:

- consumer registration, execution, and failure-path tests
- workflow proof and archived artifact for a real producer -> consumer -> case
  result flow

Remaining Phase 4 work:

- None for the case-create contract currently in scope; any additional
  sanctioned case-command verbs should meet the same parity bar before they are
  treated as milestone-ready.

### Phase 5: Replace Simulated Workflow Claims And Extend Proof

Goal: make the expanded product-complete proof bundle reflect the real milestone
surface, not just the earlier scoped subset.

Changes:

- Reclassify synthetic scenarios in the scenario runner as smoke-only where they
  remain useful.
- Replace any remaining milestone-facing simulated workflow claims with real
  workflow tests.
- Extend [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh)
  to generate and archive machine-readable artifacts for the expanded workflow
  matrix, including case management, conversation operations, attachment flows,
  and sanctioned case actions.
- Update [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) with the new
  artifact set and rerun instructions.

Acceptance criteria:

- The proof bundle contains workflow artifacts for every operational workflow
  row still in scope.
- The proof summary points to those artifact directories explicitly.

Evidence:

- updated proof bundle layout
- successful local and CI proof rerun with non-empty workflow artifact outputs

### Phase 6: Reclose Milestone 1 Honestly

Goal: move Milestone 1 back to closed status only after the expanded product bar
is actually met.

Changes:

- Keep the full integration gate hard.
- Require the expanded workflow rows in the merge and milestone proof gates.
- Only then mark the milestone closed again in readiness docs.

Acceptance criteria:

- `go test -tags=integration ./...` passes in CI without `continue-on-error`.
- Affected operational workflow tests run in CI and pass.
- [`docs/MILESTONE_1_READINESS.md`](./MILESTONE_1_READINESS.md) can move the
  reopened product-complete areas from `Open` or `Partially evidenced` to
  `Proven`.

Evidence:

- green CI run with hard-gated integration and workflow coverage
- final milestone proof bundle with operational workflow artifacts

## Dependency Notes

- Phase 1 should land before the proof bundle is expanded, because the expanded
  milestone bar now depends on the operator-complete case loop.
- Phase 2 and Phase 3 can overlap, but both must finish before Milestone 1 can
  honestly claim a mature service-desk replacement slice.
- Phase 4 depends on the chosen sanctioned case-action mechanism.
- Phase 5 should start after Phases 1 through 4 have produced machine-readable
  workflow outputs worth archiving.
- Phase 6 is last on purpose; the milestone should not close again until the
  broader product loops are actually present.

## Definition Of Closed

The expanded product-complete milestone gap is closed only when all of the
following are true:

- the operator-complete case loop is available through the supported CLI, API,
  and admin surfaces and has workflow proof
- conversation reply, handoff, escalation, and a supported public conversation
  intake path are proven
- attachment-bearing operational workflows are proven
- the sanctioned extension-to-core case action contract is real and proven
- scenario-runner simulations are not represented as workflow proof
- the milestone proof archives machine-readable operational workflow artifacts
  for the expanded matrix
- the full integration suite remains green and hard-gated in CI
