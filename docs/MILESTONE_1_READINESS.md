# Milestone 1 Readiness

**Updated:** 2026-03-24  
**Purpose:** This is the living proof-synthesis artifact for Milestone 1. [`milestone-1-scope.md`](../milestone-1-scope.md) remains the target. This document records what is currently proven in-repo, what is only partially evidenced, and what still needs explicit closure.

## Status Legend

- `Proven` means the repo has matching behavior, tests, and supporting docs.
- `Partially evidenced` means the shape exists, but the proof chain is incomplete or fragmented.
- `Open` means the milestone claim is still materially missing or not yet credible from current repo evidence.

## Current Summary

- The core product shape is credible in this repo: multi-team operational work, forms, queues, conversations, cases, knowledge, concept specs, sandbox lifecycle, and installable extensions all have concrete implementation and test coverage.
- The lifecycle miss found in the first review pass was extension uninstall. That is now closed with service, GraphQL, CLI, contract, guided removal export planning, and test coverage.
- The proof-coherence gap is now closed with a runnable milestone proof script, a CI workflow that exercises it, and explicit docs for first-party pack readiness and cross-platform CLI release artifacts.
- No repo-local Milestone 1 blocker remains open. The remaining external confirmation is the next tagged GitHub run that publishes signed CLI release archives from the new workflow.

## Proof Matrix

| Area | Status | Repo Evidence | Missing Evidence / Remaining Gap |
| --- | --- | --- | --- |
| Core operational base: forms, queues, conversations, cases, knowledge, concept specs, audit-oriented behavior | `Proven` | [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md), [`internal/service/services/conversation_service.go`](../internal/service/services/conversation_service.go), [`internal/service/services/case_service_test.go`](../internal/service/services/case_service_test.go), [`internal/service/services/form_spec_service_test.go`](../internal/service/services/form_spec_service_test.go), [`internal/knowledge/services/knowledge_service_test.go`](../internal/knowledge/services/knowledge_service_test.go), [`internal/knowledge/services/concept_spec_service_test.go`](../internal/knowledge/services/concept_spec_service_test.go) | The proof exists, but it is spread across many files rather than summarized in one milestone-facing scenario. |
| Agent surface: `mbr` CLI contract, JSON ergonomics, stored context, agent-facing docs | `Proven` | [`docs/AGENT_CLI.md`](./AGENT_CLI.md), [`internal/clispec/spec.go`](../internal/clispec/spec.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`scripts/check-cli-contract-docs.sh`](../scripts/check-cli-contract-docs.sh), [`scripts/build-cli-release.sh`](../scripts/build-cli-release.sh), [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml), [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md) | External confirmation still comes from the next tagged GitHub release run, but the release path and verification contract now exist in-repo. |
| Extension runtime lifecycle: install, validate, configure, activate, monitor, deactivate, uninstall | `Proven` | [`internal/platform/services/extension_service.go`](../internal/platform/services/extension_service.go), [`internal/platform/services/extension_service_test.go`](../internal/platform/services/extension_service_test.go), [`internal/graphql/schema/schema.graphql`](../internal/graphql/schema/schema.graphql), [`cmd/mbr/extensions.go`](../cmd/mbr/extensions.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md`](./INSTANCE_AND_EXTENSION_LIFECYCLE.md) | The uninstall flow now includes dry-run planning, optional pre-uninstall deactivation, export bundles, and schema-cleanup guidance. |
| First-party packs: ATS, enterprise access, error tracking, web analytics | `Proven` | [`internal/platform/services/extension_reference_ats_test.go`](../internal/platform/services/extension_reference_ats_test.go), [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go), [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go), [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go), [`extensions/first-party`](../extensions/first-party), [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md) | The launch-readiness summary for the first-party pack set now lives in-repo instead of only in scattered tests. |
| Hosted sandbox lifecycle and bootstrap path | `Proven` | [`internal/platform/services/sandbox_service.go`](../internal/platform/services/sandbox_service.go), [`internal/platform/services/sandbox_service_test.go`](../internal/platform/services/sandbox_service_test.go), [`internal/platform/handlers/sandbox_public.go`](../internal/platform/handlers/sandbox_public.go), [`cmd/api/routers_sandbox_test.go`](../cmd/api/routers_sandbox_test.go), [`cmd/mbr/sandboxes.go`](../cmd/mbr/sandboxes.go), [`cmd/mbr/sandboxes_test.go`](../cmd/mbr/sandboxes_test.go), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) | The milestone proof loop now ties sandbox create, export, expiry, CLI flows, and extension evidence together in one rerunnable artifact. |
| Release, migration, and verification story | `Proven` | [`docs/testing-strategy.md`](./testing-strategy.md), [`docs/RELEASE_ARTIFACT_CONTRACT.md`](./RELEASE_ARTIFACT_CONTRACT.md), [`migrations/postgres/README.md`](../migrations/postgres/README.md), [`docs/doc-reconciliation-checklist.md`](./doc-reconciliation-checklist.md), [`.github/workflows/_build.yml`](../.github/workflows/_build.yml), [`.github/workflows/_test.yml`](../.github/workflows/_test.yml), [`.github/workflows/milestone-proof.yml`](../.github/workflows/milestone-proof.yml), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) | The repo now has a specific launch-readiness automation and proof runbook rather than only a collection of lower-level CI checks. |

## Closed In This Update

- Added extension uninstall to the real lifecycle instead of only documenting it:
  [`internal/platform/services/extension_service.go`](../internal/platform/services/extension_service.go),
  [`internal/platform/resolvers/resolver.go`](../internal/platform/resolvers/resolver.go),
  [`internal/graph/root.go`](../internal/graph/root.go),
  [`internal/graphql/schema/schema.graphql`](../internal/graphql/schema/schema.graphql),
  [`cmd/mbr/extensions.go`](../cmd/mbr/extensions.go),
  [`internal/clispec/spec.go`](../internal/clispec/spec.go),
  [`internal/platform/services/extension_service_test.go`](../internal/platform/services/extension_service_test.go),
  [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go).
- Added this readiness document so milestone proof is captured in-repo instead of only inferred from scattered code and docs.
- Added a guided extension removal workflow that previews removal impact, exports a removal bundle, supports optional pre-uninstall deactivation, and records schema cleanup guidance:
  [`cmd/mbr/extensions.go`](../cmd/mbr/extensions.go),
  [`cmd/mbr/main.go`](../cmd/mbr/main.go),
  [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go),
  [`docs/AGENT_CLI.md`](./AGENT_CLI.md).
- Added explicit cross-platform CLI release evidence:
  [`scripts/build-cli-release.sh`](../scripts/build-cli-release.sh),
  [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml),
  [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md).
- Added a runnable milestone proof loop and first-party pack readiness synthesis:
  [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh),
  [`.github/workflows/milestone-proof.yml`](../.github/workflows/milestone-proof.yml),
  [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md),
  [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md).

## Residual External Confirmation

- The next tagged GitHub run of [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml) is the last non-local confirmation step for published signatures and release attachments. The repo-local release path, manifest, checksums, and signing automation are now present and testable.

## Update Rule

When milestone-relevant behavior changes, update this document together with the implementation, tests, and any operator-facing docs. The scope doc should stay ambitious; this file should stay honest about what the repo actually proves today.
