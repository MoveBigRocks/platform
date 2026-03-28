# Milestone 1 Proof

This document is the rerun guide for the Milestone 1 launch-proof loop.

## Entry Points

- Local:
  - `make milestone-proof`
  - or `bash scripts/run-milestone-1-proof.sh --out dist/milestone-proof`
- CI:
  - [`.github/workflows/milestone-proof.yml`](../.github/workflows/milestone-proof.yml)

## What The Proof Run Covers Today

1. Core operational behavior:
   - `./internal/service/services`
   - `./internal/knowledge/services`
2. Sandbox lifecycle, extension lifecycle, and first-party pack behavior:
   - `./internal/platform/services`
   - `./cmd/api`
3. Agent and operator CLI behavior:
   - `./cmd/mbr`
4. CLI contract and doc reconciliation:
   - `bash scripts/check-cli-contract-docs.sh`
5. Runtime bootstrap artifact capture:
   - render `/.well-known/mbr-instance.json` into the proof bundle
6. Public first-party catalog validation:
   - build a host `mbr`
   - run `MoveBigRocks/extensions/scripts/validate-first-party.sh`
   - archive the validator log and `public-bundles.json` snapshot
7. Cross-platform CLI packaging evidence:
   - `bash scripts/build-cli-release.sh`
8. CLI release artifact validation:
   - `bash scripts/verify-cli-release.sh dist/milestone-proof/cli-release`

## Evidence Expansion Required Before Close

1. Sandbox lifecycle closure:
   - prove auto-expiry or reaper behavior, not only manual destroy
   - archive richer export evidence rather than only a handoff-style export manifest
2. Public bundle publication evidence:
   - capture OCI ref, digest, and release-workflow metadata for ATS, error tracking, web analytics, `sales-pipeline` beta, and `community-feature-requests` beta
3. ATS scenario proof:
   - add one automated scenario that proves create job, publish, submit, review, move stage, close, and reopen

## Outputs

The current proof run writes:

- `dist/milestone-proof/summary.md`
- `dist/milestone-proof/sandbox-bootstrap/`
- `dist/milestone-proof/extensions-validation/`
- `dist/milestone-proof/cli-release/`
- `dist/milestone-proof/cli-release/verification.json`

Milestone closure should expand that output set to also include:

- `dist/milestone-proof/sandbox-lifecycle/`
- `dist/milestone-proof/public-bundles/`
- `dist/milestone-proof/ats-scenario/`

Those outputs together are the concrete proof bundle tying together:

- milestone readiness status
- first-party pack readiness
- sandbox and extension lifecycle evidence
- cross-platform CLI release evidence

## Related Evidence

- [`docs/MILESTONE_1_READINESS.md`](./MILESTONE_1_READINESS.md)
- [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md)
- [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md)
