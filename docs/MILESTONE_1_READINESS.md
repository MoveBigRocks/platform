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
- Repo-local blockers still remain open before Milestone 1 can honestly be called closed: ATS is still below the written owned-schema workflow scope, the runtime bootstrap endpoint is not yet implemented, sandbox expiry and richer export evidence are incomplete, and the platform proof loop still does not archive live public-bundle publication metadata and digests.
- The milestone is closeable with focused implementation and evidence work, but it should be treated as `not yet closed` until the gaps below are resolved.

## Proof Matrix

| Area | Status | Repo Evidence | Missing Evidence / Remaining Gap |
| --- | --- | --- | --- |
| Core operational base: forms, queues, conversations, cases, knowledge, concept specs, audit-oriented behavior | `Proven` | [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md), [`internal/service/services/conversation_service.go`](../internal/service/services/conversation_service.go), [`internal/service/services/case_service_test.go`](../internal/service/services/case_service_test.go), [`internal/service/services/form_spec_service_test.go`](../internal/service/services/form_spec_service_test.go), [`internal/knowledge/services/knowledge_service_test.go`](../internal/knowledge/services/knowledge_service_test.go), [`internal/knowledge/services/concept_spec_service_test.go`](../internal/knowledge/services/concept_spec_service_test.go) | The proof exists, but it is spread across many files rather than summarized in one milestone-facing scenario. |
| Agent surface: `mbr` CLI contract, JSON ergonomics, stored context, agent-facing docs | `Proven` | [`docs/AGENT_CLI.md`](./AGENT_CLI.md), [`internal/clispec/spec.go`](../internal/clispec/spec.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`scripts/check-cli-contract-docs.sh`](../scripts/check-cli-contract-docs.sh), [`scripts/build-cli-release.sh`](../scripts/build-cli-release.sh), [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml), [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md) | External confirmation still comes from the next tagged GitHub release run, but the CLI contract and operator surface are concrete in-repo. |
| Extension runtime lifecycle: install, validate, configure, activate, monitor, deactivate, uninstall | `Proven` | [`internal/platform/services/extension_service.go`](../internal/platform/services/extension_service.go), [`internal/platform/services/extension_service_test.go`](../internal/platform/services/extension_service_test.go), [`internal/graphql/schema/schema.graphql`](../internal/graphql/schema/schema.graphql), [`cmd/mbr/extensions.go`](../cmd/mbr/extensions.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md`](./INSTANCE_AND_EXTENSION_LIFECYCLE.md) | The uninstall flow now includes dry-run planning, optional pre-uninstall deactivation, export bundles, and schema-cleanup guidance. |
| First-party packs: core launch packs plus beta public packs | `Partially evidenced` | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go), [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go), [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go), [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md) | The core and beta pack set loads and validates on the shared runtime, but ATS still falls short of the written owned-schema workflow scope, and public publication evidence for the full bundle set is not yet captured in the milestone proof. |
| Public bundle catalog and publication pipeline | `Partially evidenced` | [`MoveBigRocks/extensions/catalog/public-bundles.json`](https://github.com/MoveBigRocks/extensions/blob/main/catalog/public-bundles.json), [`MoveBigRocks/extensions/scripts/validate-first-party.sh`](https://github.com/MoveBigRocks/extensions/blob/main/scripts/validate-first-party.sh), [`MoveBigRocks/extensions/.github/workflows/public-bundles.yml`](https://github.com/MoveBigRocks/extensions/blob/main/.github/workflows/public-bundles.yml), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) | The milestone proof now archives the public bundle catalog snapshot and validator output, but live publication metadata and digests are still not captured in the proof bundle. |
| Hosted sandbox lifecycle and bootstrap path | `Partially evidenced` | [`internal/platform/services/sandbox_service.go`](../internal/platform/services/sandbox_service.go), [`internal/platform/services/sandbox_service_test.go`](../internal/platform/services/sandbox_service_test.go), [`internal/platform/handlers/sandbox_public.go`](../internal/platform/handlers/sandbox_public.go), [`cmd/api/routers_sandbox_test.go`](../cmd/api/routers_sandbox_test.go), [`cmd/mbr/sandboxes.go`](../cmd/mbr/sandboxes.go), [`cmd/mbr/sandboxes_test.go`](../cmd/mbr/sandboxes_test.go), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) | The platform proves create, show, extend, export, and destroy flows, but `/.well-known/mbr-instance.json` is still missing, expiry and reaping are not yet proven as a real lifecycle, and sandbox export still omits richer runtime data and configuration artifacts. |
| Release, migration, and verification story | `Partially evidenced` | [`docs/testing-strategy.md`](./testing-strategy.md), [`docs/RELEASE_ARTIFACT_CONTRACT.md`](./RELEASE_ARTIFACT_CONTRACT.md), [`migrations/postgres/README.md`](../migrations/postgres/README.md), [`.github/workflows/_build.yml`](../.github/workflows/_build.yml), [`.github/workflows/_test.yml`](../.github/workflows/_test.yml), [`.github/workflows/milestone-proof.yml`](../.github/workflows/milestone-proof.yml), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md), [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md) | The proof runbook and CI entry point exist, and the CLI release artifacts are now validated in-repo, but the milestone proof still needs extensions-side evidence ingestion and live publication confirmation. |

## Sync Changes In This Update

- Expanded the milestone target to include `sales-pipeline` and `community-feature-requests` as in-scope public beta packs instead of leaving them as undocumented scope drift.
- Reopened milestone close-out status where repo-local gaps are still real: ATS scope parity, bootstrap discovery, sandbox expiry and export, and public bundle evidence capture.
- Synchronized the scope, readiness, pack-readiness, and proof docs so they all describe the same milestone target and the same closure work.

## Gap Closure Plan

1. Implement `/.well-known/mbr-instance.json`, then add automated proof that it returns runtime discovery metadata, CLI install metadata, docs URLs, and sandbox policy data.
2. Close the sandbox lifecycle gaps by implementing auto-expiry or a reaper flow, enriching export output beyond handoff-only metadata, and archiving that evidence as part of the milestone proof bundle.
3. Bring ATS up to the written scope by proving extension-owned recruiting workflow state and the create, publish, review, stage-move, close, and reopen lifecycle in automated tests and proof artifacts.
4. Capture public bundle publication evidence for ATS, error tracking, web analytics, `sales-pipeline` beta, and `community-feature-requests` beta from [`MoveBigRocks/extensions/.github/workflows/public-bundles.yml`](https://github.com/MoveBigRocks/extensions/blob/main/.github/workflows/public-bundles.yml), then link that evidence from this document and [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md).

## Residual External Confirmation

- After the repo-local gaps above are closed, the remaining external confirmation steps will be the next tagged GitHub run of [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml) and the next tagged or public runs of [`MoveBigRocks/extensions/.github/workflows/public-bundles.yml`](https://github.com/MoveBigRocks/extensions/blob/main/.github/workflows/public-bundles.yml).
- Until then, this milestone should be treated as internally credible but not yet fully closed.

## Update Rule

When milestone-relevant behavior changes, update this document together with the implementation, tests, and any operator-facing docs. The scope doc should stay ambitious; this file should stay honest about what the repo actually proves today.
