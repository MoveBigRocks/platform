# Milestone 1 Readiness

**Updated:** 2026-03-28
**Purpose:** This is the living proof-synthesis artifact for Milestone 1. [`milestone-1-scope.md`](../milestone-1-scope.md) remains the target. This document records what is currently proven in-repo, what is only partially evidenced, and what still needs explicit closure.

## Status Legend

- `Proven` means the repo has matching behavior, tests, and supporting docs.
- `Partially evidenced` means the shape exists, but the proof chain is incomplete or fragmented.
- `Open` means the milestone claim is still materially missing or not yet credible from current repo evidence.

## Current Summary

- The core product shape is credible in this repo: multi-team operational work, forms, queues, conversations, cases, knowledge, concept specs, and installable extensions all have concrete implementation and test coverage.
- Milestone 1 is reopened under an expanded "complete and mature platform" bar. The runtime, extension, and release foundations are strong, but the repo does not yet prove the full operator-complete service-desk loop implied by the public product promise.
- The milestone target has been deliberately expanded to include two public beta packs, `sales-pipeline` and `community-feature-requests`, alongside the four core first-party packs.
- Live public-bundle publication evidence exists for the full in-scope public pack set, including ATS `v0.8.24`, error tracking `v0.8.21`, web analytics `v0.8.21`, `sales-pipeline` beta `v0.1.0`, and `community-feature-requests` beta `v0.1.0`; those emitted files are now durably archived in [`docs/evidence/public-bundle-publication/`](./evidence/public-bundle-publication/) and cross-checked in the milestone proof workflow against [`docs/evidence/public-bundle-publication-runs.json`](./evidence/public-bundle-publication-runs.json) plus the generated publication plan.
- Hosted sandboxes are deferred to [`docs/RFCs/RFC-0013-hosted-sandbox-control-plane.md`](./RFCs/RFC-0013-hosted-sandbox-control-plane.md) and are not part of the current Milestone 1 CLI contract or proof loop.
- The workflow-proof standard has now been tightened in [`docs/testing-strategy.md`](./testing-strategy.md) and [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md).
- Command-driven operational workflows now have end-to-end automated proof and machine-readable proof artifacts archived by the milestone proof loop.
- The full `go test -tags=integration ./...` sweep is green and now hard-gated in CI.
- The reusable `Test` workflow now provisions and verifies the same canonical
  sibling repos the first-party source tests depend on, so those checks fail
  closed in everyday CI instead of only inside the milestone-proof job.
- The committed proof workflow now materializes the canonical sibling repos it
  depends on, and proof mode fails closed when those checkouts are absent
  instead of silently skipping first-party source validation.
- The committed proof workflow now pins those sibling repos to the exact SHAs
  recorded in [`docs/evidence/canonical-workspace-refs.json`](./evidence/canonical-workspace-refs.json),
  and the proof bundle archives verification that the exercised workspace
  matched those refs.
- The proof bundle now also archives the input manifests for canonical
  workspace refs and publication evidence, so the downloaded artifact is more
  self-contained and less dependent on repo-relative paths.
- The previously closed command-driven workflow gap is still closed, but Milestone 1 itself is no longer considered closed until the broader product-complete operational loops are implemented and proven.

## Proof Matrix

| Area | Status | Repo Evidence | Missing Evidence / Remaining Gap |
| --- | --- | --- | --- |
| Foundational operational base: forms, queues, conversations, cases, knowledge, concept specs, audit-oriented behavior | `Proven` | [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md), [`internal/service/services/conversation_service.go`](../internal/service/services/conversation_service.go), [`internal/service/services/case_service_test.go`](../internal/service/services/case_service_test.go), [`internal/service/services/form_spec_service_test.go`](../internal/service/services/form_spec_service_test.go), [`internal/knowledge/services/knowledge_service_test.go`](../internal/knowledge/services/knowledge_service_test.go), [`internal/knowledge/services/concept_spec_service_test.go`](../internal/knowledge/services/concept_spec_service_test.go), [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh) | The primitives and several high-risk workflows are real. What remains open is the broader product-complete operator loop, not the existence of the base platform. |
| Operator-complete case loop: manual create, assign, priority, note, reply, handoff, status, attachment handling | `Partially evidenced` | [`internal/service/services/case_service.go`](../internal/service/services/case_service.go), [`internal/platform/handlers/workspace_api_handler.go`](../internal/platform/handlers/workspace_api_handler.go), [`docs/AGENT_CLI.md`](./AGENT_CLI.md), [`cmd/mbr/cases.go`](../cmd/mbr/cases.go), [`internal/service/resolvers/case_operator_workflow_integration_test.go`](../internal/service/resolvers/case_operator_workflow_integration_test.go), [`internal/infrastructure/stores/sql/case_communication_postgres_test.go`](../internal/infrastructure/stores/sql/case_communication_postgres_test.go) | Manual create, assign, unassign, reprioritize, internal note, reply, handoff, and lifecycle status transitions are now proven through supported product surfaces. Case routing also now validates real team targets, and the supported status mutation now follows resolve/close/reopen lifecycle semantics. The remaining case-loop gap is attachment-bearing case flows. |
| Conversation loop: public intake, reply, handoff, escalate, queue parity, provenance | `Open` | [`internal/service/services/conversation_service.go`](../internal/service/services/conversation_service.go), [`cmd/mbr/conversations.go`](../cmd/mbr/conversations.go), [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md), [`docs/CUSTOMER_FAQ.md`](./CUSTOMER_FAQ.md) | Conversations are central to the public promise, but the workflow-proof matrix still does not prove conversation reply, handoff, escalation, or any supported public conversation intake surface. |
| Attachment workflows: manual case upload, inbound-email attachment ingest, ATS resume path | `Partially evidenced` | [`internal/service/handlers/attachment_upload_handler.go`](../internal/service/handlers/attachment_upload_handler.go), [`internal/service/handlers/attachment_upload_handler_test.go`](../internal/service/handlers/attachment_upload_handler_test.go), [`internal/service/handlers/postmark_webhooks.go`](../internal/service/handlers/postmark_webhooks.go), [`internal/service/services/attachment_service.go`](../internal/service/services/attachment_service.go) | Upload and ingest code exists, but Milestone 1 now requires workflow-proof rows and proof-bundle artifacts for attachment-bearing operational flows. |
| Sanctioned extension-to-core case action contract | `Open` | [`internal/shared/events/commands.go`](../internal/shared/events/commands.go), [`internal/workers/manager.go`](../internal/workers/manager.go), [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md) | The architecture promises sanctioned core-action requests, but `case-commands` still has no production consumer or workflow proof. |
| Agent surface: `mbr` CLI contract, JSON ergonomics, stored context, agent-facing docs | `Proven` | [`docs/AGENT_CLI.md`](./AGENT_CLI.md), [`internal/clispec/spec.go`](../internal/clispec/spec.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`scripts/check-cli-contract-docs.sh`](../scripts/check-cli-contract-docs.sh), [`scripts/build-cli-release.sh`](../scripts/build-cli-release.sh), [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml), [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md) | External confirmation still comes from the next tagged GitHub release run, but the CLI contract and operator surface are concrete in-repo. |
| Extension runtime lifecycle: install, validate, configure, activate, monitor, deactivate, uninstall | `Proven` | [`internal/platform/services/extension_service.go`](../internal/platform/services/extension_service.go), [`internal/platform/services/extension_service_test.go`](../internal/platform/services/extension_service_test.go), [`internal/graphql/schema/schema.graphql`](../internal/graphql/schema/schema.graphql), [`cmd/mbr/extensions.go`](../cmd/mbr/extensions.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md`](./INSTANCE_AND_EXTENSION_LIFECYCLE.md) | The uninstall flow now includes dry-run planning, optional pre-uninstall deactivation, export bundles, and schema-cleanup guidance. |
| First-party packs: core launch packs plus beta public packs | `Proven` | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go), [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go), [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go), [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md), [`MoveBigRocks/extensions/ats/runtime/service_test.go`](https://github.com/MoveBigRocks/extensions/blob/e957fb2272f77d24d3bb4a907ad372fb93175e30/ats/runtime/service_test.go), [`MoveBigRocks/extensions/tools/ats-scenario-proof/main.go`](https://github.com/MoveBigRocks/extensions/blob/e957fb2272f77d24d3bb4a907ad372fb93175e30/tools/ats-scenario-proof/main.go) | The core and beta pack set validates on the shared runtime, ATS has owned-schema lifecycle proof and scenario evidence, and live publication evidence now exists for the full in-scope public set. |
| Public bundle catalog and publication pipeline | `Proven` | [`MoveBigRocks/extensions/catalog/public-bundles.json`](https://github.com/MoveBigRocks/extensions/blob/e957fb2272f77d24d3bb4a907ad372fb93175e30/catalog/public-bundles.json), [`MoveBigRocks/extensions/scripts/validate-first-party.sh`](https://github.com/MoveBigRocks/extensions/blob/e957fb2272f77d24d3bb4a907ad372fb93175e30/scripts/validate-first-party.sh), [`MoveBigRocks/extensions/tools/publication-evidence/main.go`](https://github.com/MoveBigRocks/extensions/blob/e957fb2272f77d24d3bb4a907ad372fb93175e30/tools/publication-evidence/main.go), [`MoveBigRocks/extensions/.github/workflows/public-bundles.yml`](https://github.com/MoveBigRocks/extensions/blob/e957fb2272f77d24d3bb4a907ad372fb93175e30/.github/workflows/public-bundles.yml), [`docs/evidence/public-bundle-publication-runs.json`](./evidence/public-bundle-publication-runs.json), [`docs/evidence/public-bundle-publication/`](./evidence/public-bundle-publication/), [`docs/evidence/canonical-workspace-refs.json`](./evidence/canonical-workspace-refs.json), [`scripts/fetch-publication-evidence.sh`](../scripts/fetch-publication-evidence.sh), [`scripts/verify-publication-evidence.sh`](../scripts/verify-publication-evidence.sh), [`scripts/verify-canonical-workspace-refs.sh`](../scripts/verify-canonical-workspace-refs.sh), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) | Catalog validation, publication planning, durable archived evidence, and live publish runs are present for the in-scope public bundle set. The proof workflow now pins the canonical sibling repos it needs, verifies the exercised workspace against those refs, verifies the emitted publication evidence against the checked-in manifest and publication plan, cross-checks live downloads against the archived copies, and archives the manifest inputs it used. |
| Operational workflow proof: inbound email, case replies, notifications, and rule-driven delivery | `Proven` | [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md), [`internal/service/handlers/postmark_webhooks_integration_test.go`](../internal/service/handlers/postmark_webhooks_integration_test.go), [`internal/service/handlers/email_command_handler_test.go`](../internal/service/handlers/email_command_handler_test.go), [`internal/service/handlers/form_public_handler_test.go`](../internal/service/handlers/form_public_handler_test.go), [`internal/service/handlers/notification_command_handler_test.go`](../internal/service/handlers/notification_command_handler_test.go), [`internal/automation/services/action_handlers_notification_integration_test.go`](../internal/automation/services/action_handlers_notification_integration_test.go), [`internal/knowledge/services/knowledge_service_test.go`](../internal/knowledge/services/knowledge_service_test.go), [`internal/workers/manager_test.go`](../internal/workers/manager_test.go), [`internal/infrastructure/container/container_integration_test.go`](../internal/infrastructure/container/container_integration_test.go), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh) | The repo now proves the subscribed bus and outbox path, worker-manager/container wiring, and failure-visible retry state for `email-commands` and `notification-commands`, with machine-readable proof artifacts archived for those flows. |
| Release, migration, and verification story | `Proven` | [`docs/testing-strategy.md`](./testing-strategy.md), [`docs/RELEASE_ARTIFACT_CONTRACT.md`](./RELEASE_ARTIFACT_CONTRACT.md), [`migrations/postgres/README.md`](../migrations/postgres/README.md), [`.github/workflows/_build.yml`](../.github/workflows/_build.yml), [`.github/workflows/_test.yml`](../.github/workflows/_test.yml), [`.github/workflows/milestone-proof.yml`](../.github/workflows/milestone-proof.yml), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md), [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md), [`docs/evidence/canonical-workspace-refs.json`](./evidence/canonical-workspace-refs.json) | CLI release and public-bundle publication evidence remain strong, the full integration sweep is hard-gated, the reusable test workflow now provisions the pinned canonical sibling repos instead of letting first-party source tests skip, and the milestone proof archives both the command-driven workflow artifacts and the integration sweep log. |

## Sync Changes In This Update

- Expanded the milestone target to include `sales-pipeline` and `community-feature-requests` as in-scope public beta packs instead of leaving them as undocumented scope drift.
- Closed the remaining release-facing evidence gap by tagging and running the public bundle workflow for the full in-scope public set, then archiving the emitted publication evidence inside the milestone proof bundle.
- Adopted a workflow-proof model for operational capabilities and added [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md) as the inventory of milestone-facing workflow evidence.
- Closed the command-driven email and notification workflow gaps with end-to-end tests, archived workflow artifacts, and a green hard-gated integration sweep.
- Synced [`milestone-1-scope.md`](../milestone-1-scope.md) to the shipped repo shape and CLI namespace so the scope matches the public first-party extensions repo and the `mbr forms ...` command surface.
- Reopened Milestone 1 under a stricter product-complete bar so the milestone now explicitly requires the operator-complete case loop, conversation workflow proof, attachment workflow proof, and a real sanctioned case-action contract for extensions and agents.

## Next Closure Checkpoints

The next closure sequence is now recorded in
[`docs/WORKFLOW_PROOF_CLOSURE_PLAN.md`](./WORKFLOW_PROOF_CLOSURE_PLAN.md).
Milestone 1 should not move back to closed status until that expanded
product-complete loop is proven.

## Live Publication Evidence

- ATS `v0.8.24`: [run 23688333389](https://github.com/MoveBigRocks/extensions/actions/runs/23688333389)
- Error Tracking `v0.8.21`: [run 23688148347](https://github.com/MoveBigRocks/extensions/actions/runs/23688148347)
- Web Analytics `v0.8.21`: [run 23688148371](https://github.com/MoveBigRocks/extensions/actions/runs/23688148371)
- Sales Pipeline beta `v0.1.0`: [run 23683709265](https://github.com/MoveBigRocks/extensions/actions/runs/23683709265)
- Community Feature Requests beta `v0.1.0`: [run 23683709269](https://github.com/MoveBigRocks/extensions/actions/runs/23683709269)

Those runs emit `*.publication-evidence.json` artifacts that are now preserved
under [`docs/evidence/public-bundle-publication/`](./evidence/public-bundle-publication/),
fetched from the checked-in run manifest in CI, verified against the generated
publication plan, and archived into the milestone proof bundle under
`public-bundle-publication/release-evidence/`. Local reruns can rely on the
checked-in archive automatically, use the same manifest to refetch live
artifacts, or provide a pre-downloaded evidence directory directly.

## Ongoing Release Confirmation

- Future tagged runs of [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml) and [`MoveBigRocks/extensions/.github/workflows/public-bundles.yml`](https://github.com/MoveBigRocks/extensions/blob/main/.github/workflows/public-bundles.yml) remain normal operational release confirmation, but they are no longer Milestone 1 blockers.

## Update Rule

When milestone-relevant behavior changes, update this document together with the implementation, tests, [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md), and any operator-facing docs. The scope doc should stay ambitious; this file should stay honest about what the repo actually proves today.
