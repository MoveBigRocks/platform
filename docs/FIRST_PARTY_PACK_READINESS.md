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
| ATS | Core launch pack, public | Workspace product pack with careers site, owned-schema recruiting workflow state, and application flow | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`internal/platform/services/extension_runtime_test.go`](../internal/platform/services/extension_runtime_test.go), [`MoveBigRocks/extensions/ats/runtime/service_test.go`](https://github.com/MoveBigRocks/extensions/blob/main/ats/runtime/service_test.go), [`MoveBigRocks/extensions/tools/ats-scenario-proof/main.go`](https://github.com/MoveBigRocks/extensions/blob/main/tools/ats-scenario-proof/main.go) | None for Milestone 1. Live publication evidence exists and is archived in the milestone proof bundle. |
| Enterprise Access | Core launch pack, controlled privileged | Instance-scoped identity and privileged admin pack | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go) | [`internal/platform/services/extension_admin_navigation_test.go`](../internal/platform/services/extension_admin_navigation_test.go) | Controlled distribution path is intentional for Milestone 1. |
| Error Tracking | Core launch pack, public | Workspace operational pack with Sentry-compatible ingest and admin pages | [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | None for Milestone 1. Live publication evidence exists and is archived in the milestone proof bundle. |
| Web Analytics | Core launch pack, public | Workspace operational pack with analytics script and admin page | [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | None for Milestone 1. Live publication evidence exists and is archived in the milestone proof bundle. |
| Sales Pipeline | Public beta pack | Workspace product pack for opportunity intake, stage movement, and dedicated sales workspace provisioning | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go), [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | None for Milestone 1. Beta install guidance is explicit and live publication evidence exists. |
| Community Feature Requests | Public beta pack | Workspace product pack for public idea capture, voting, triage, and roadmap review | [`internal/platform/services/first_party_extension_packages_test.go`](../internal/platform/services/first_party_extension_packages_test.go), [`cmd/mbr/extension_contract_test.go`](../cmd/mbr/extension_contract_test.go) | [`internal/platform/services/extension_validation_test.go`](../internal/platform/services/extension_validation_test.go) | None for Milestone 1. Beta install guidance is explicit and live publication evidence exists. |

## Pack Locations

- ATS: public first-party source in
  [`MoveBigRocks/extensions/ats`](https://github.com/MoveBigRocks/extensions/tree/main/ats)
- Enterprise Access: controlled first-party source in `packs/enterprise-access`
- Error Tracking: public first-party source in
  [`MoveBigRocks/extensions/error-tracking`](https://github.com/MoveBigRocks/extensions/tree/main/error-tracking)
- Sales Pipeline: public first-party beta source in
  [`MoveBigRocks/extensions/sales-pipeline`](https://github.com/MoveBigRocks/extensions/tree/main/sales-pipeline)
- Community Feature Requests: public first-party beta source in
  [`MoveBigRocks/extensions/community-feature-requests`](https://github.com/MoveBigRocks/extensions/tree/main/community-feature-requests)
- Web Analytics: public first-party source in
  [`MoveBigRocks/extensions/web-analytics`](https://github.com/MoveBigRocks/extensions/tree/main/web-analytics)

## Proof Loop

These packs are part of the standard milestone readiness run:

- [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh)
- [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md)

The pack proof tests load ATS, the beta public packs, enterprise access, and
the SDK sample pack from their canonical sibling repos instead of from
duplicated `platform` fixtures. The milestone proof now pulls in the
extensions-side ATS scenario, catalog validation steps, and a publication plan
generated from the public bundle catalog. The current closure proof also
archives the workflow-generated publication artifacts and digests from live
public-bundle runs.

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
