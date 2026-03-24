# Milestone 1 Proof

This document is the rerun guide for the Milestone 1 launch-proof loop.

## Entry Points

- Local:
  - `make milestone-proof`
  - or `bash scripts/run-milestone-1-proof.sh --out dist/milestone-proof`
- CI:
  - [`.github/workflows/milestone-proof.yml`](../.github/workflows/milestone-proof.yml)

## What The Proof Run Covers

1. Core operational behavior:
   - `./internal/service/services`
   - `./internal/knowledge/services`
2. Sandbox lifecycle, extension lifecycle, and first-party pack behavior:
   - `./internal/platform/services`
   - `./cmd/api`
3. Agent and operator CLI behavior:
   - `./cmd/mbr`
4. CLI contract/doc reconciliation:
   - `bash scripts/check-cli-contract-docs.sh`
5. Cross-platform CLI packaging evidence:
   - `bash scripts/build-cli-release.sh`

## Outputs

The proof run writes:

- `dist/milestone-proof/summary.md`
- `dist/milestone-proof/cli-release/`

Those outputs are the concrete proof bundle tying together:

- milestone readiness status
- first-party pack readiness
- sandbox and extension lifecycle evidence
- cross-platform CLI release evidence

## Related Evidence

- [`docs/MILESTONE_1_READINESS.md`](./MILESTONE_1_READINESS.md)
- [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md)
- [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md)
