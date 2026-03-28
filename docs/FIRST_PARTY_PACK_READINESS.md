# First-Party Pack Readiness

This document is the launch-readiness summary for the Milestone 1 first-party
extension set, including the in-scope public beta packs.

## Quality Bar

A first-party pack is considered launch-ready when it has:

- a canonical checked-in source in the public first-party extensions repo or the
  controlled first-party repo, depending on its release posture
- no stale first-party bundle mirrors or pack fixtures left behind in `platform`
- install and activation proof in automated tests
- evidence for its public or admin runtime surface where applicable
- a clear scope and risk profile consistent with the manifest
- an explicit launch posture: core launch pack, controlled privileged pack, or
  public beta pack

## Pack Matrix

| Pack | Launch Posture | Scope | Core Proof | Runtime / Surface Proof | Remaining Gap |
| --- | --- | --- | --- | --- | --- |
| ATS | Core launch pack, public | Workspace product pack with careers site, owned-schema recruiting workflow state, and application flow | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`internal/platform/services/extension_runtime_test.go`](../internal/platform/services/extension_runtime_test.go), [`MoveBigRocks/extensions/ats/runtime/service_test.go`](https://github.com/MoveBigRocks/extensions/blob/e957fb2272f77d24d3bb4a907ad372fb93175e30/ats/runtime/service_test.go), [`MoveBigRocks/extensions/tools/ats-scenario-proof/main.go`](https://github.com/MoveBigRocks/extensions/blob/e957fb2272f77d24d3bb4a907ad372fb93175e30/tools/ats-scenario-proof/main.go) | None for Milestone 1. Live publication evidence exists, is durably archived in-repo, and is verified in the milestone proof bundle. |
| Enterprise Access | Core launch pack, controlled privileged | Instance-scoped identity and privileged admin pack | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go) | [`internal/platform/services/extension_admin_navigation_test.go`](../internal/platform/services/extension_admin_navigation_test.go) | Controlled distribution path is intentional for Milestone 1. |
| Error Tracking | Core launch pack, public | Workspace operational pack with Sentry-compatible ingest and admin pages | [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | None for Milestone 1. Live publication evidence exists, is durably archived in-repo, and is verified in the milestone proof bundle. |
| Web Analytics | Core launch pack, public | Workspace operational pack with analytics script and admin page | [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | None for Milestone 1. Live publication evidence exists, is durably archived in-repo, and is verified in the milestone proof bundle. |
| Sales Pipeline | Public beta pack | Workspace product pack for opportunity intake, stage movement, and dedicated sales workspace provisioning | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go), [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | None for Milestone 1. Beta install guidance is explicit and live publication evidence exists, is durably archived in-repo, and is verified in the milestone proof bundle. |
| Community Feature Requests | Public beta pack | Workspace product pack for public idea capture, voting, triage, and roadmap review | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go) | None for Milestone 1. Beta install guidance is explicit and live publication evidence exists, is durably archived in-repo, and is verified in the milestone proof bundle. |

## Pack Locations

- ATS: public first-party source in
  [`MoveBigRocks/extensions/ats`](https://github.com/MoveBigRocks/extensions/tree/e957fb2272f77d24d3bb4a907ad372fb93175e30/ats)
- Enterprise Access: controlled first-party source in `packs/enterprise-access`
- Error Tracking: public first-party source in
  [`MoveBigRocks/extensions/error-tracking`](https://github.com/MoveBigRocks/extensions/tree/e957fb2272f77d24d3bb4a907ad372fb93175e30/error-tracking)
- Sales Pipeline: public first-party beta source in
  [`MoveBigRocks/extensions/sales-pipeline`](https://github.com/MoveBigRocks/extensions/tree/e957fb2272f77d24d3bb4a907ad372fb93175e30/sales-pipeline)
- Community Feature Requests: public first-party beta source in
  [`MoveBigRocks/extensions/community-feature-requests`](https://github.com/MoveBigRocks/extensions/tree/e957fb2272f77d24d3bb4a907ad372fb93175e30/community-feature-requests)
- Web Analytics: public first-party source in
  [`MoveBigRocks/extensions/web-analytics`](https://github.com/MoveBigRocks/extensions/tree/e957fb2272f77d24d3bb4a907ad372fb93175e30/web-analytics)

## Proof Loop

These packs are part of the standard milestone readiness run:

- [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh)
- [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md)

The pack proof tests load ATS, the beta public packs, enterprise access, and
the SDK sample pack from their canonical sibling repos instead of from
duplicated `platform` fixtures. The milestone proof now pulls in the
extensions-side ATS scenario, catalog validation steps, and a publication plan
generated from the public bundle catalog. The current closure proof also keeps
a checked-in archive of the workflow-generated publication artifacts and
digests under [`docs/evidence/public-bundle-publication/`](./evidence/public-bundle-publication/),
verifies them against the manifest and publication plan, and cross-checks live
downloads against those archived copies in CI. Proof mode now fails closed when
those canonical sibling repos are missing, and the committed milestone-proof CI
workflow materializes `extensions`, `extension-sdk`, and `packs` before it runs
that validation. Those sibling repos are also pinned in
[`docs/evidence/canonical-workspace-refs.json`](./evidence/canonical-workspace-refs.json)
and verified inside the proof bundle before the first-party pack checks run.
Local reruns can materialize that same pinned sibling workspace with
[`scripts/bootstrap-canonical-workspace.sh`](../scripts/bootstrap-canonical-workspace.sh)
instead of assembling it manually.
The proof bundle now also archives that manifest and the publication-evidence
manifest so the artifact stays self-contained.

## Live Publication Evidence

- ATS `v0.8.24`: [run 23688333389](https://github.com/MoveBigRocks/extensions/actions/runs/23688333389)
- Error Tracking `v0.8.21`: [run 23688148347](https://github.com/MoveBigRocks/extensions/actions/runs/23688148347)
- Web Analytics `v0.8.21`: [run 23688148371](https://github.com/MoveBigRocks/extensions/actions/runs/23688148371)
- Sales Pipeline beta `v0.1.0`: [run 23683709265](https://github.com/MoveBigRocks/extensions/actions/runs/23683709265)
- Community Feature Requests beta `v0.1.0`: [run 23683709269](https://github.com/MoveBigRocks/extensions/actions/runs/23683709269)

## Distribution Note

The current public distribution target is:

- free public signed bundles for ATS, error tracking, and web analytics,
  plus beta public bundles for sales-pipeline and community-feature-requests,
  published from the public first-party extensions repo at
  [`MoveBigRocks/extensions`](https://github.com/MoveBigRocks/extensions)
- a separately controlled first-party path for enterprise access
