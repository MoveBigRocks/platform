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
6. Sandbox lifecycle artifact capture:
   - render create, export, expiry reaping, post-expiry export, and destroy evidence into the proof bundle
7. CLI sandbox lifecycle artifact capture:
   - execute `mbr sandboxes create`, `show`, `extend`, `export`, and `destroy` against a proof server
   - archive the command outputs, request log, and exported bundle
8. Public first-party catalog validation:
   - build a host `mbr`
   - run `MoveBigRocks/extensions/scripts/validate-first-party.sh`
   - archive the validator log and `public-bundles.json` snapshot
9. ATS scenario proof:
   - run `MoveBigRocks/extensions/go test ./ats/runtime ./cmd/ats-runtime ./tools/ats-scenario-proof`
   - archive the ATS scenario JSON proving create job, publish, submit, review, move stage, close, and reopen
10. Public bundle publication planning:
   - run `MoveBigRocks/extensions/go run ./tools/publication-evidence --mode plan`
   - archive the generated publication-plan JSON from the public bundle catalog and current manifests
   - when tagged publish runs exist, copy the workflow-generated `*.publication-evidence.json` files into the proof bundle
11. Cross-platform CLI packaging evidence:
   - `bash scripts/build-cli-release.sh`
12. CLI release artifact validation:
   - `bash scripts/verify-cli-release.sh dist/milestone-proof/cli-release`

## Evidence Expansion Required Before Close

1. Public bundle publication evidence:
   - run tagged publishes for ATS, error tracking, web analytics, `sales-pipeline` beta, and `community-feature-requests` beta so the workflow emits `*.publication-evidence.json`
   - archive those JSON artifacts inside the milestone proof bundle so the milestone has registry-visible proof for every public pack in scope

## Outputs

The current proof run writes:

- `dist/milestone-proof/summary.md`
- `dist/milestone-proof/sandbox-bootstrap/`
- `dist/milestone-proof/sandbox-lifecycle/`
- `dist/milestone-proof/cli-sandbox/`
- `dist/milestone-proof/ats-scenario/`
- `dist/milestone-proof/public-bundle-publication/`
- `dist/milestone-proof/extensions-validation/`
- `dist/milestone-proof/cli-release/`
- `dist/milestone-proof/cli-release/verification.json`

Those outputs together are the concrete proof bundle tying together:

- milestone readiness status
- first-party pack readiness
- sandbox and extension lifecycle evidence
- cross-platform CLI release evidence

## Related Evidence

- [`docs/MILESTONE_1_READINESS.md`](./MILESTONE_1_READINESS.md)
- [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md)
- [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md)
