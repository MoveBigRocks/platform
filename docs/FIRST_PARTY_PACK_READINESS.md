# First-Party Pack Readiness

This document is the launch-readiness summary for the first-party Milestone 1
extension set.

## Quality Bar

A first-party pack is considered launch-ready when it has:

- a checked-in manifest and asset bundle under [`extensions/first-party`](../extensions/first-party)
- install and activation proof in automated tests
- evidence for its public or admin runtime surface where applicable
- a clear scope and risk profile consistent with the manifest

## Pack Matrix

| Pack | Scope | Core Proof | Runtime / Surface Proof | Launch Note |
| --- | --- | --- | --- | --- |
| ATS | Workspace product pack with careers-site and application flow | [`internal/platform/services/extension_reference_ats_test.go`](../internal/platform/services/extension_reference_ats_test.go) | [`internal/platform/services/extension_runtime_test.go`](../internal/platform/services/extension_runtime_test.go) | Proven as the reference pack for workspace-level operational extensions. |
| Enterprise Access | Instance-scoped identity and privileged admin pack | [`internal/platform/services/extension_reference_ats_test.go`](../internal/platform/services/extension_reference_ats_test.go) | [`internal/platform/services/extension_admin_navigation_test.go`](../internal/platform/services/extension_admin_navigation_test.go) | Proven as the instance-scoped privileged first-party pack. |
| Error Tracking | Workspace operational pack with Sentry-compatible ingest and admin pages | [`cmd/api/error_tracking_extraction_test.go`](../cmd/api/error_tracking_extraction_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | Proven for public ingest routing and first-party admin UI dispatch. |
| Web Analytics | Workspace operational pack with analytics script and admin page | [`cmd/api/analytics_extraction_test.go`](../cmd/api/analytics_extraction_test.go) | [`cmd/api/extension_service_targets_test.go`](../cmd/api/extension_service_targets_test.go) | Proven for runtime script delivery and first-party admin UI dispatch. |

## Pack Locations

- ATS: [`extensions/first-party/ats`](../extensions/first-party/ats)
- Enterprise Access: [`extensions/first-party/enterprise-access`](../extensions/first-party/enterprise-access)
- Error Tracking: [`extensions/first-party/error-tracking`](../extensions/first-party/error-tracking)
- Web Analytics: [`extensions/first-party/web-analytics`](../extensions/first-party/web-analytics)

## Proof Loop

These packs are part of the standard milestone readiness run:

- [`scripts/run-milestone-1-proof.sh`](../scripts/run-milestone-1-proof.sh)
- [`docs/MILESTONE_1_PROOF.md`](./MILESTONE_1_PROOF.md)
