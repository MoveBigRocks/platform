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
7. ATS scenario proof:
   - run `MoveBigRocks/extensions/go test ./ats/runtime ./cmd/ats-runtime ./tools/ats-scenario-proof`
   - archive the ATS scenario JSON proving create job, publish, submit, review, move stage, close, and reopen
8. Public bundle publication planning:
   - run `MoveBigRocks/extensions/go run ./tools/publication-evidence --mode plan`
   - archive the generated publication-plan JSON from the public bundle catalog and current manifests
   - archive the workflow-generated `*.publication-evidence.json` files inside `public-bundle-publication/release-evidence/` when `FIRST_PARTY_PUBLICATION_EVIDENCE_DIR` is supplied
9. Operational workflow proof:
   - archive machine-readable JSON artifacts for inbound-new-email case creation, case reply send, inbound reply threading, public form notification delivery, rule-driven email delivery, and knowledge-review notifications
10. Cross-platform CLI packaging evidence:
   - `bash scripts/build-cli-release.sh`
11. CLI release artifact validation:
   - `bash scripts/verify-cli-release.sh dist/milestone-proof/cli-release`

## What The Proof Run Does Not Yet Cover

The proof bundle still does not claim anything about command streams that are
outside the Milestone 1 workflow set. In particular, `case-commands` still has
no production consumer and must not be treated as proven. External-provider
reliability on the public internet also remains outside what the archived proof
artifacts can establish.

## Current Live Publication Evidence

- ATS `v0.8.23`: [run 23683709259](https://github.com/MoveBigRocks/extensions/actions/runs/23683709259)
- Error Tracking `v0.8.20`: [run 23683710893](https://github.com/MoveBigRocks/extensions/actions/runs/23683710893)
- Web Analytics `v0.8.20`: [run 23683711231](https://github.com/MoveBigRocks/extensions/actions/runs/23683711231)
- Sales Pipeline beta `v0.1.0`: [run 23683709265](https://github.com/MoveBigRocks/extensions/actions/runs/23683709265)
- Community Feature Requests beta `v0.1.0`: [run 23683709269](https://github.com/MoveBigRocks/extensions/actions/runs/23683709269)

When `FIRST_PARTY_PUBLICATION_EVIDENCE_DIR` is supplied to the proof script, the
bundle also archives the emitted `*.publication-evidence.json` files from those
runs inside `public-bundle-publication/release-evidence/`.

## Outputs

The current proof run writes:

- `dist/milestone-proof/summary.md`
- `dist/milestone-proof/runtime-bootstrap/`
- `dist/milestone-proof/ats-scenario/`
- `dist/milestone-proof/workflow-proof/`
- `dist/milestone-proof/public-bundle-publication/`
- `dist/milestone-proof/public-bundle-publication/release-evidence/` when
  publication evidence files are provided to the proof script
- `dist/milestone-proof/extensions-validation/`
- `dist/milestone-proof/cli-release/`
- `dist/milestone-proof/cli-release/verification.json`

Those outputs currently provide the concrete proof bundle for:

- milestone readiness status
- first-party pack readiness
- runtime discovery, extension lifecycle, and publication evidence
- milestone-scoped operational workflow evidence
- cross-platform CLI release evidence

## Related Evidence

- [`docs/MILESTONE_1_READINESS.md`](./MILESTONE_1_READINESS.md)
- [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md)
- [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md)
- [`docs/testing-strategy.md`](./testing-strategy.md)
- [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md)
- [`docs/WORKFLOW_PROOF_CLOSURE_PLAN.md`](./WORKFLOW_PROOF_CLOSURE_PLAN.md)
