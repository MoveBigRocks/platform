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
- The milestone proof now archives live public-bundle publication evidence for the full in-scope public pack set, including ATS `v0.8.23`, error tracking `v0.8.20`, web analytics `v0.8.20`, `sales-pipeline` beta `v0.1.0`, and `community-feature-requests` beta `v0.1.0`.
- Milestone 1 now satisfies the current scope and can be treated as closed against this definition of done.

## Proof Matrix

| Area | Status | Repo Evidence | Missing Evidence / Remaining Gap |
| --- | --- | --- | --- |
| Core operational base: forms, queues, conversations, cases, knowledge, concept specs, audit-oriented behavior | `Proven` | [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md), [`internal/service/services/conversation_service.go`](../internal/service/services/conversation_service.go), [`internal/service/services/case_service_test.go`](../internal/service/services/case_service_test.go), [`internal/service/services/form_spec_service_test.go`](../internal/service/services/form_spec_service_test.go), [`internal/knowledge/services/knowledge_service_test.go`](../internal/knowledge/services/knowledge_service_test.go), [`internal/knowledge/services/concept_spec_service_test.go`](../internal/knowledge/services/concept_spec_service_test.go) | The proof exists, but it is spread across many files rather than summarized in one milestone-facing scenario. |
| Agent surface: `mbr` CLI contract, JSON ergonomics, stored context, agent-facing docs | `Proven` | [`docs/AGENT_CLI.md`](./AGENT_CLI.md), [`internal/clispec/spec.go`](../internal/clispec/spec.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`scripts/check-cli-contract-docs.sh`](../scripts/check-cli-contract-docs.sh), [`scripts/build-cli-release.sh`](../scripts/build-cli-release.sh), [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml), [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md) | External confirmation still comes from the next tagged GitHub release run, but the CLI contract and operator surface are concrete in-repo. |
| Extension runtime lifecycle: install, validate, configure, activate, monitor, deactivate, uninstall | `Proven` | [`internal/platform/services/extension_service.go`](../internal/platform/services/extension_service.go), [`internal/platform/services/extension_service_test.go`](../internal/platform/services/extension_service_test.go), [`internal/graphql/schema/schema.graphql`](../internal/graphql/schema/schema.graphql), [`cmd/mbr/extensions.go`](../cmd/mbr/extensions.go), [`cmd/mbr/main_test.go`](../cmd/mbr/main_test.go), [`docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md`](./INSTANCE_AND_EXTENSION_LIFECYCLE.md) | The uninstall flow now includes dry-run planning, optional pre-uninstall deactivation, export bundles, and schema-cleanup guidance. |
| First-party packs: core launch packs plus beta public packs | `Proven` | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go), [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go), [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go), [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md), [`MoveBigRocks/extensions/ats/runtime/service_test.go`](https://github.com/MoveBigRocks/extensions/blob/main/ats/runtime/service_test.go), [`MoveBigRocks/extensions/tools/ats-scenario-proof/main.go`](https://github.com/MoveBigRocks/extensions/blob/main/tools/ats-scenario-proof/main.go) | The core and beta pack set validates on the shared runtime, ATS has owned-schema lifecycle proof and scenario evidence, and live publication evidence now exists for the full in-scope public set. |
| Public bundle catalog and publication pipeline | `Proven` | [`MoveBigRocks/extensions/catalog/public-bundles.json`](https://github.com/MoveBigRocks/extensions/blob/main/catalog/public-bundles.json), [`MoveBigRocks/extensions/scripts/validate-first-party.sh`](https://github.com/MoveBigRocks/extensions/blob/main/scripts/validate-first-party.sh), [`MoveBigRocks/extensions/tools/publication-evidence/main.go`](https://github.com/MoveBigRocks/extensions/blob/main/tools/publication-evidence/main.go), [`MoveBigRocks/extensions/.github/workflows/public-bundles.yml`](https://github.com/MoveBigRocks/extensions/blob/main/.github/workflows/public-bundles.yml), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) | Catalog validation, publication planning, live publish runs, and archived publication evidence are now all present for the in-scope public bundle set. |
| Hosted sandbox lifecycle and bootstrap path | `Proven` | [`internal/platform/services/sandbox_service.go`](../internal/platform/services/sandbox_service.go), [`internal/platform/services/sandbox_service_test.go`](../internal/platform/services/sandbox_service_test.go), [`internal/platform/handlers/sandbox_public.go`](../internal/platform/handlers/sandbox_public.go), [`internal/platform/handlers/runtime_bootstrap.go`](../internal/platform/handlers/runtime_bootstrap.go), [`cmd/api/routers_sandbox_test.go`](../cmd/api/routers_sandbox_test.go), [`cmd/mbr/sandboxes.go`](../cmd/mbr/sandboxes.go), [`cmd/mbr/sandboxes_test.go`](../cmd/mbr/sandboxes_test.go), [`tools/runtime-bootstrap-proof/main.go`](../tools/runtime-bootstrap-proof/main.go), [`tools/sandbox-lifecycle-proof/main.go`](../tools/sandbox-lifecycle-proof/main.go), [`tools/cli-sandbox-proof/main.go`](../tools/cli-sandbox-proof/main.go), [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md) | The platform now proves the hosted sandbox lifecycle end to end through the public API, the `mbr sandboxes` CLI, bootstrap discovery, expiry reaping, richer lifecycle/configuration export bundles, and archived proof artifacts. |
| Release, migration, and verification story | `Proven` | [`docs/testing-strategy.md`](./testing-strategy.md), [`docs/RELEASE_ARTIFACT_CONTRACT.md`](./RELEASE_ARTIFACT_CONTRACT.md), [`migrations/postgres/README.md`](../migrations/postgres/README.md), [`.github/workflows/_build.yml`](../.github/workflows/_build.yml), [`.github/workflows/_test.yml`](../.github/workflows/_test.yml), [`.github/workflows/milestone-proof.yml`](../.github/workflows/milestone-proof.yml), [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md), [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md) | The milestone proof validates CLI release artifacts, first-party bundle validation, publication planning, and archived live publication evidence. |

## Sync Changes In This Update

- Expanded the milestone target to include `sales-pipeline` and `community-feature-requests` as in-scope public beta packs instead of leaving them as undocumented scope drift.
- Closed the remaining release-facing evidence gap by tagging and running the public bundle workflow for the full in-scope public set, then archiving the emitted publication evidence inside the milestone proof bundle.
- Synchronized the scope, readiness, pack-readiness, and proof docs so they all describe the same milestone target and the same closure work.

## Live Publication Evidence

- ATS `v0.8.23`: [run 23683709259](https://github.com/MoveBigRocks/extensions/actions/runs/23683709259)
- Error Tracking `v0.8.20`: [run 23683710893](https://github.com/MoveBigRocks/extensions/actions/runs/23683710893)
- Web Analytics `v0.8.20`: [run 23683711231](https://github.com/MoveBigRocks/extensions/actions/runs/23683711231)
- Sales Pipeline beta `v0.1.0`: [run 23683709265](https://github.com/MoveBigRocks/extensions/actions/runs/23683709265)
- Community Feature Requests beta `v0.1.0`: [run 23683709269](https://github.com/MoveBigRocks/extensions/actions/runs/23683709269)

The emitted `*.publication-evidence.json` artifacts from those runs are archived
in the current milestone proof bundle under
`public-bundle-publication/release-evidence/`.

## Ongoing Release Confirmation

- Future tagged runs of [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml) and [`MoveBigRocks/extensions/.github/workflows/public-bundles.yml`](https://github.com/MoveBigRocks/extensions/blob/main/.github/workflows/public-bundles.yml) remain normal operational release confirmation, but they are no longer Milestone 1 blockers.

## Update Rule

When milestone-relevant behavior changes, update this document together with the implementation, tests, and any operator-facing docs. The scope doc should stay ambitious; this file should stay honest about what the repo actually proves today.
