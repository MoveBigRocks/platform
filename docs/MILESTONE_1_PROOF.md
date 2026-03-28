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
5. Cross-platform CLI packaging evidence:
   - `bash scripts/build-cli-release.sh`

## Evidence Expansion Required Before Close

1. Public bundle catalog validation:
   - run [`MoveBigRocks/extensions/scripts/validate-first-party.sh`](https://github.com/MoveBigRocks/extensions/blob/main/scripts/validate-first-party.sh) from the milestone proof using a freshly built `mbr` binary
   - archive the validation output and the exact [`MoveBigRocks/extensions/catalog/public-bundles.json`](https://github.com/MoveBigRocks/extensions/blob/main/catalog/public-bundles.json) snapshot in the proof bundle
2. CLI release artifact integrity:
   - validate `cli-release/release-manifest.json` with `jq`
   - verify `checksums.txt` against the emitted archives
3. Runtime bootstrap discovery:
   - exercise `/.well-known/mbr-instance.json`
   - archive the returned runtime-discovery payload as proof that agents can bootstrap from a live instance or sandbox
4. Sandbox lifecycle closure:
   - prove auto-expiry or reaper behavior, not only manual destroy
   - archive richer export evidence rather than only a handoff-style export manifest
5. Public bundle publication evidence:
   - capture OCI ref, digest, and release-workflow metadata for ATS, error tracking, web analytics, `sales-pipeline` beta, and `community-feature-requests` beta
6. ATS scenario proof:
   - add one automated scenario that proves create job, publish, submit, review, move stage, close, and reopen

## Outputs

The current proof run writes:

- `dist/milestone-proof/summary.md`
- `dist/milestone-proof/cli-release/`

Milestone closure should expand that output set to also include:

- `dist/milestone-proof/extensions-validation/`
- `dist/milestone-proof/sandbox-bootstrap/`
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
