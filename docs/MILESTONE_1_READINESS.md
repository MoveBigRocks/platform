# Milestone 1 Readiness

**Updated:** 2026-03-28
**Purpose:** This is the living proof-synthesis artifact for Milestone 1. [`milestone-1-scope.md`](../milestone-1-scope.md) remains the target. This document records what is currently proven in-repo, what is only partially evidenced, and what still needs explicit closure.

## Status Legend

- `Proven` means the repo has matching behavior, tests, and supporting docs.
- `Partially evidenced` means the shape exists, but the proof chain is incomplete or fragmented.
- `Open` means the milestone claim is still materially missing or not yet credible from current repo evidence.

## Current Summary

- The core product shape is credible in this repo: multi-team operational work, forms, queues, conversations, cases, knowledge, concept specs, and installable extensions all have concrete implementation and test coverage.
- The milestone target has been deliberately expanded to include two public beta packs, `sales-pipeline` and `community-feature-requests`, alongside the four core first-party packs.
- Live public-bundle publication evidence exists for the full in-scope public pack set, including ATS `v0.8.23`, error tracking `v0.8.20`, web analytics `v0.8.20`, `sales-pipeline` beta `v0.1.0`, and `community-feature-requests` beta `v0.1.0`, and the milestone proof can archive the emitted files when `FIRST_PARTY_PUBLICATION_EVIDENCE_DIR` is supplied.
- Hosted sandboxes are deferred to [`docs/RFCs/RFC-0013-hosted-sandbox-control-plane.md`](./RFCs/RFC-0013-hosted-sandbox-control-plane.md) and are not part of the current Milestone 1 CLI contract or proof loop.
- The workflow-proof standard has now been tightened in [`docs/testing-strategy.md`](./testing-strategy.md) and [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md).
- Command-driven operational workflows now have end-to-end automated proof and machine-readable proof artifacts archived by the milestone proof loop.
- The full `go test -tags=integration ./...` sweep is green and now hard-gated in CI.
- Milestone 1 is closed against the current workflow-proof standard for the scoped operational capabilities.

## Proof Matrix

| Area | Status | Repo Evidence | Missing Evidence / Remaining Gap |
| --- | --- | --- | --- |
| Core operational base: forms, queues, conversations, cases, knowledge, concept specs, audit-oriented behavior | `Proven` | [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md), [`internal/service/services/conversation_service.go`](../internal/service/services/conversation_service.go), [`internal/service/services/case_service_test.go`](../internal/service/services/case_service_test.go), [`internal/service/services/form_spec_service_test.go`](../internal/service/services/form_spec_service_test.go), [`internal/knowledge/services/knowledge_service_test.go`](../internal/knowledge/services/knowledge_service_test.go), [`internal/knowledge/services/concept_spec_service_test.go`](../internal/knowledge/services/concept_spec_service_test.go), [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh) | The product shape is backed by real workflow proof for inbound email, case replies, public-form notifications, rule-driven email, and knowledge-review notifications. |
| Agent surface: `mbr` CLI contract, JSON ergonomics, stored context, agent-facing docs | `Proven` | [`docs/AGENT_CLI.md`](./AGENT_CLI.md), [`internal/clispec/spec.go`](../internal/clispec/spec.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`scripts/check-cli-contract-docs.sh`](../scripts/check-cli-contract-docs.sh), [`scripts/build-cli-release.sh`](../scripts/build-cli-release.sh), [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml), [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md) | External confirmation still comes from the next tagged GitHub release run, but the CLI contract and operator surface are concrete in-repo. |
| Extension runtime lifecycle: install, validate, configure, activate, monitor, deactivate, uninstall | `Proven` | [`internal/platform/services/extension_service.go`](../internal/platform/services/extension_service.go), [`internal/platform/services/extension_service_test.go`](../internal/platform/services/extension_service_test.go), [`internal/graphql/schema/schema.graphql`](../internal/graphql/schema/schema.graphql), [`cmd/mbr/extensions.go`](../cmd/mbr/extensions.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md`](./INSTANCE_AND_EXTENSION_LIFECYCLE.md) | The uninstall flow now includes dry-run planning, optional pre-uninstall deactivation, export bundles, and schema-cleanup guidance. |
| First-party packs: core launch packs plus beta public packs | `Proven` | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go), [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go), [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go), [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md), [`MoveBigRocks/extensions/ats/runtime/service_test.go`](https://github.com/MoveBigRocks/extensions/blob/main/ats/runtime/service_test.go), [`MoveBigRocks/extensions/tools/ats-scenario-proof/main.go`](https://github.com/MoveBigRocks/extensions/blob/main/tools/ats-scenario-proof/main.go) | The core and beta pack set validates on the shared runtime, ATS has owned-schema lifecycle proof and scenario evidence, and live publication evidence now exists for the full in-scope public set. |
| Public bundle catalog and publication pipeline | `Proven` | [`MoveBigRocks/extensions/catalog/public-bundles.json`](https://github.com/MoveBigRocks/extensions/blob/main/catalog/public-bundles.json), [`MoveBigRocks/extensions/scripts/validate-first-party.sh`](https://github.com/MoveBigRocks/extensions/blob/main/scripts/validate-first-party.sh), [`MoveBigRocks/extensions/tools/publication-evidence/main.go`](https://github.com/MoveBigRocks/extensions/blob/main/tools/publication-evidence/main.go), [`MoveBigRocks/extensions/.github/workflows/public-bundles.yml`](https://github.com/MoveBigRocks/extensions/blob/main/.github/workflows/public-bundles.yml), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) | Catalog validation, publication planning, and live publish runs are present for the in-scope public bundle set. Proof reruns can additionally archive the emitted publication evidence files when `FIRST_PARTY_PUBLICATION_EVIDENCE_DIR` is provided. |
| Operational workflow proof: inbound email, case replies, notifications, and rule-driven delivery | `Proven` | [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md), [`internal/service/handlers/postmark_webhooks_integration_test.go`](../internal/service/handlers/postmark_webhooks_integration_test.go), [`internal/service/handlers/email_command_handler_test.go`](../internal/service/handlers/email_command_handler_test.go), [`internal/service/handlers/form_public_handler_test.go`](../internal/service/handlers/form_public_handler_test.go), [`internal/automation/services/action_handlers_notification_integration_test.go`](../internal/automation/services/action_handlers_notification_integration_test.go), [`internal/knowledge/services/knowledge_service_test.go`](../internal/knowledge/services/knowledge_service_test.go), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh) | The repo now proves the worker-backed workflow for `email-commands` and `notification-commands` and archives machine-readable proof artifacts for those flows. |
| Release, migration, and verification story | `Proven` | [`docs/testing-strategy.md`](./testing-strategy.md), [`docs/RELEASE_ARTIFACT_CONTRACT.md`](./RELEASE_ARTIFACT_CONTRACT.md), [`migrations/postgres/README.md`](../migrations/postgres/README.md), [`.github/workflows/_build.yml`](../.github/workflows/_build.yml), [`.github/workflows/_test.yml`](../.github/workflows/_test.yml), [`.github/workflows/milestone-proof.yml`](../.github/workflows/milestone-proof.yml), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md), [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md) | CLI release and public-bundle publication evidence remain strong, the integration sweep is now hard-gated, and the milestone proof archives command-driven operational workflow artifacts. |

## Sync Changes In This Update

- Expanded the milestone target to include `sales-pipeline` and `community-feature-requests` as in-scope public beta packs instead of leaving them as undocumented scope drift.
- Closed the remaining release-facing evidence gap by tagging and running the public bundle workflow for the full in-scope public set, then archiving the emitted publication evidence inside the milestone proof bundle.
- Adopted a workflow-proof model for operational capabilities and added [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md) as the inventory of milestone-facing workflow evidence.
- Closed the command-driven email and notification workflow gaps with end-to-end tests, archived workflow artifacts, and a green hard-gated integration sweep.

## Next Closure Checkpoints

The workflow closure sequence is now complete and recorded in
[`docs/WORKFLOW_PROOF_CLOSURE_PLAN.md`](./WORKFLOW_PROOF_CLOSURE_PLAN.md).

## Live Publication Evidence

- ATS `v0.8.23`: [run 23683709259](https://github.com/MoveBigRocks/extensions/actions/runs/23683709259)
- Error Tracking `v0.8.20`: [run 23683710893](https://github.com/MoveBigRocks/extensions/actions/runs/23683710893)
- Web Analytics `v0.8.20`: [run 23683711231](https://github.com/MoveBigRocks/extensions/actions/runs/23683711231)
- Sales Pipeline beta `v0.1.0`: [run 23683709265](https://github.com/MoveBigRocks/extensions/actions/runs/23683709265)
- Community Feature Requests beta `v0.1.0`: [run 23683709269](https://github.com/MoveBigRocks/extensions/actions/runs/23683709269)

Those runs emit `*.publication-evidence.json` artifacts that can be archived in
the milestone proof bundle under
`public-bundle-publication/release-evidence/` when
`FIRST_PARTY_PUBLICATION_EVIDENCE_DIR` is supplied.

## Ongoing Release Confirmation

- Future tagged runs of [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml) and [`MoveBigRocks/extensions/.github/workflows/public-bundles.yml`](https://github.com/MoveBigRocks/extensions/blob/main/.github/workflows/public-bundles.yml) remain normal operational release confirmation, but they are no longer Milestone 1 blockers.

## Update Rule

When milestone-relevant behavior changes, update this document together with the implementation, tests, [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md), and any operator-facing docs. The scope doc should stay ambitious; this file should stay honest about what the repo actually proves today.
