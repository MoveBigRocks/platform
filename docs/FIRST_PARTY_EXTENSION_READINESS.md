# First-Party Extension Readiness

This document is the launch-readiness summary for the Milestone 1 first-party
extension set, including the in-scope public beta extensions.

## Quality Bar

A first-party extension is considered launch-ready when it has:

- a canonical checked-in source in the first-party extensions repo
- install and activation proof in automated tests
- evidence for its public or admin runtime surface where applicable
- a clear scope and risk profile consistent with the manifest
- an explicit launch posture: core launch extension or public beta extension

## Extension Matrix

| Extension | Launch Posture | Scope | Core Proof | Runtime / Surface Proof | Remaining Gap |
| --- | --- | --- | --- | --- | --- |
| ATS | Core launch extension | Workspace product extension with careers site, owned-schema recruiting workflow state, and application flow | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`internal/platform/services/extension_runtime_test.go`](../internal/platform/services/extension_runtime_test.go), [`MoveBigRocks/extensions/ats/runtime/service_test.go`](https://github.com/MoveBigRocks/extensions/blob/a0aa6945763ad1559834a3639658f31b7fe25ea6/ats/runtime/service_test.go), [`MoveBigRocks/extensions/tools/ats-scenario-proof/main.go`](https://github.com/MoveBigRocks/extensions/blob/a0aa6945763ad1559834a3639658f31b7fe25ea6/tools/ats-scenario-proof/main.go) | None for Milestone 1. The uploaded-resume path is proven end to end, live publication evidence exists for `v0.8.25`, and that evidence is archived in the milestone proof bundle. |
| Enterprise Access | Core launch extension | Instance-scoped identity extension with first-party admin routes and owned-schema provider state | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go) | [`internal/platform/services/extension_admin_navigation_test.go`](../internal/platform/services/extension_admin_navigation_test.go), [`MoveBigRocks/extensions/enterprise-access/manifest.json`](https://github.com/MoveBigRocks/extensions/blob/a0aa6945763ad1559834a3639658f31b7fe25ea6/enterprise-access/manifest.json) | None for Milestone 1. The canonical first-party source is in `MoveBigRocks/extensions` and validates from that same source tree in proof and CI. |
| Error Tracking | Core launch extension | Workspace operational extension with Sentry-compatible ingest and admin pages | [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | None for Milestone 1. Live publication evidence exists, is archived in-repo, and is verified in the milestone proof bundle. |
| Web Analytics | Core launch extension | Workspace operational extension with analytics script and admin page | [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | None for Milestone 1. Live publication evidence exists, is archived in-repo, and is verified in the milestone proof bundle. |
| Sales Pipeline | Public beta extension | Workspace product extension for opportunity intake, stage movement, and dedicated sales workspace provisioning | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go), [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | None for Milestone 1. Beta install guidance is explicit and live publication evidence exists, is archived in-repo, and is verified in the milestone proof bundle. |
| Community Feature Requests | Public beta extension | Workspace product extension for public idea capture, voting, triage, and roadmap review | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go) | None for Milestone 1. Beta install guidance is explicit and live publication evidence exists, is archived in-repo, and is verified in the milestone proof bundle. |

## Extension Locations

- ATS: public first-party source in
  [`MoveBigRocks/extensions/ats`](https://github.com/MoveBigRocks/extensions/tree/a0aa6945763ad1559834a3639658f31b7fe25ea6/ats)
- Enterprise Access: first-party source in
  [`MoveBigRocks/extensions/enterprise-access`](https://github.com/MoveBigRocks/extensions/tree/a0aa6945763ad1559834a3639658f31b7fe25ea6/enterprise-access)
- Error Tracking: public first-party source in
  [`MoveBigRocks/extensions/error-tracking`](https://github.com/MoveBigRocks/extensions/tree/a0aa6945763ad1559834a3639658f31b7fe25ea6/error-tracking)
- Sales Pipeline: public first-party beta source in
  [`MoveBigRocks/extensions/sales-pipeline`](https://github.com/MoveBigRocks/extensions/tree/a0aa6945763ad1559834a3639658f31b7fe25ea6/sales-pipeline)
- Community Feature Requests: public first-party beta source in
  [`MoveBigRocks/extensions/community-feature-requests`](https://github.com/MoveBigRocks/extensions/tree/a0aa6945763ad1559834a3639658f31b7fe25ea6/community-feature-requests)
- Web Analytics: public first-party source in
  [`MoveBigRocks/extensions/web-analytics`](https://github.com/MoveBigRocks/extensions/tree/a0aa6945763ad1559834a3639658f31b7fe25ea6/web-analytics)

## Proof Loop

These extensions are part of the standard milestone readiness run:

- [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh)
- [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md)

The extension proof tests load ATS, Enterprise Access, the public beta
extensions, and the SDK sample extension from their canonical sibling repos.
The milestone proof pulls in the extensions-side ATS scenario, catalog
validation steps, and a publication plan generated from the public bundle
catalog. It keeps a checked-in archive of the workflow-generated publication
artifacts and digests under
[`docs/evidence/public-bundle-publication/`](./evidence/public-bundle-publication/),
verifies them against the manifest and publication plan, and cross-checks live
downloads against those archived copies in CI. Proof mode fails closed when
those canonical sibling repos are missing, and the committed milestone-proof CI
workflow materializes `extensions` and `extension-sdk` before it runs that
validation. Those sibling repos are pinned in
[`docs/evidence/canonical-workspace-refs.json`](./evidence/canonical-workspace-refs.json)
and verified inside the proof bundle before the first-party extension checks run.
Local reruns can materialize that same pinned sibling workspace with
[`scripts/bootstrap-canonical-workspace.sh`](../scripts/bootstrap-canonical-workspace.sh)
instead of assembling it manually.
The proof bundle also archives that manifest and the publication-evidence
manifest so the artifact stays self-contained.

## Live Publication Evidence

- ATS `v0.8.25`: [run 23693072851](https://github.com/MoveBigRocks/extensions/actions/runs/23693072851)
- Error Tracking `v0.8.21`: [run 23688148347](https://github.com/MoveBigRocks/extensions/actions/runs/23688148347)
- Web Analytics `v0.8.21`: [run 23688148371](https://github.com/MoveBigRocks/extensions/actions/runs/23688148371)
- Sales Pipeline beta `v0.1.0`: [run 23683709265](https://github.com/MoveBigRocks/extensions/actions/runs/23683709265)
- Community Feature Requests beta `v0.1.0`: [run 23683709269](https://github.com/MoveBigRocks/extensions/actions/runs/23683709269)

## Distribution Note

The public distribution target is:

- free public signed bundles for ATS, error tracking, and web analytics,
  plus beta public bundles for sales-pipeline and community-feature-requests,
  published from the public first-party extensions repo at
  [`MoveBigRocks/extensions`](https://github.com/MoveBigRocks/extensions)
- Enterprise Access source in that same first-party extensions repo, installable
  through the first-party extension lifecycle
