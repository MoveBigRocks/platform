# First-Party Pack Readiness

This document is the launch-readiness summary for the first-party Milestone 1
extension set.

## Quality Bar

A first-party pack is considered launch-ready when it has:

- a canonical checked-in source in the public first-party extensions repo or the
  controlled first-party repo, depending on its release posture
- no duplicate mirror manifests left behind in `platform`
- install and activation proof in automated tests
- evidence for its public or admin runtime surface where applicable
- a clear scope and risk profile consistent with the manifest

## Pack Matrix

| Pack | Scope | Core Proof | Runtime / Surface Proof | Launch Note |
| --- | --- | --- | --- | --- |
| ATS | Workspace product pack with careers-site and application flow | [`internal/platform/services/extension_reference_ats_test.go`](../internal/platform/services/extension_reference_ats_test.go) | [`internal/platform/services/extension_runtime_test.go`](../internal/platform/services/extension_runtime_test.go) | Targeted as part of the free public first-party bundle set. |
| Enterprise Access | Instance-scoped identity and privileged admin pack | [`internal/platform/services/extension_reference_ats_test.go`](../internal/platform/services/extension_reference_ats_test.go) | [`internal/platform/services/extension_admin_navigation_test.go`](../internal/platform/services/extension_admin_navigation_test.go) | Separately controlled first-party privileged pack, not part of the free public bundle set. |
| Error Tracking | Workspace operational pack with Sentry-compatible ingest and admin pages | [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | Targeted as part of the free public first-party bundle set. |
| Web Analytics | Workspace operational pack with analytics script and admin page | [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | Targeted as part of the free public first-party bundle set. |

## Pack Locations

- ATS: public first-party source in
  [`MoveBigRocks/extensions/ats`](https://github.com/MoveBigRocks/extensions/tree/main/ats)
- Enterprise Access: controlled first-party source in `packs/enterprise-access`
- Error Tracking: public first-party source in
  [`MoveBigRocks/extensions/error-tracking`](https://github.com/MoveBigRocks/extensions/tree/main/error-tracking)
- Web Analytics: public first-party source in
  [`MoveBigRocks/extensions/web-analytics`](https://github.com/MoveBigRocks/extensions/tree/main/web-analytics)

## Proof Loop

These packs are part of the standard milestone readiness run:

- [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh)
- [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md)

## Distribution Note

The current public distribution target is:

- free public signed bundles for ATS, error tracking, and web analytics,
  published from the public first-party extensions repo at
  [`MoveBigRocks/extensions`](https://github.com/MoveBigRocks/extensions)
- a separately controlled first-party path for enterprise access
