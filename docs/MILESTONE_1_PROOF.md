# Milestone 1 Proof

This document is the rerun guide for the Milestone 1 launch-proof loop.

## Entry Points

- Local:
  - `make milestone-proof`
  - or `bash scripts/run-milestone-1-proof.sh --out dist/milestone-proof`
- CI:
  - [`.github/workflows/milestone-proof.yml`](../.github/workflows/milestone-proof.yml)

## Workspace Contract

The milestone proof is intentionally a multi-repo proof, because part of the
Milestone 1 contract is that `platform` validates and exercises the canonical
first-party sources rather than stale mirrored fixtures.

- CI now checks out `platform`, `extensions`, `extension-sdk`, and `packs`
  into one workspace before it runs the proof script.
- Those sibling repos are pinned by
  [`docs/evidence/canonical-workspace-refs.json`](./evidence/canonical-workspace-refs.json)
  instead of floating on whatever happens to be current on `main`.
- Local reruns should use the same workspace shape, or explicitly point
  `MBR_WORKSPACE_ROOT`, `FIRST_PARTY_EXTENSIONS_ROOT`, `EXTENSION_SDK_ROOT`,
  and `PACKS_ROOT` at equivalent checkouts.
- When `MBR_WORKSPACE_ROOT` is set, the proof script now resolves all sibling
  repo defaults from that workspace root consistently, including `extensions`.
- The proof script exports `MBR_REQUIRE_WORKSPACE_REFS=true`, so first-party
  tests fail closed when those canonical sibling repos are missing instead of
  silently skipping.
- If the workspace does not already provide a top-level `go.work`, or if the
  available `go.work` does not actually include the `platform` checkout being
  proven, the proof script bootstraps a temporary one inside the proof bundle
  so the exercised `platform` repo and its pinned sibling modules resolve
  correctly together.
- The proof script also verifies that the checked-out sibling repos exactly
  match the pinned refs in the manifest and archives that verification result.

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
   - keep a durable checked-in copy of the real `*.publication-evidence.json` files under [`docs/evidence/public-bundle-publication/`](./evidence/public-bundle-publication/)
   - in CI, download the live `*.publication-evidence.json` artifacts listed in [`docs/evidence/public-bundle-publication-runs.json`](./evidence/public-bundle-publication-runs.json), compare them byte-for-byte with the checked-in archive, and archive them inside `public-bundle-publication/release-evidence/`
   - for local reruns, `make milestone-proof` uses the checked-in archive automatically; you can still override it with `FIRST_PARTY_PUBLICATION_EVIDENCE_DIR` or fetch live evidence with `FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST=docs/evidence/public-bundle-publication-runs.json`
   - structurally verify the evidence bundle against both the checked-in run manifest and the generated publication plan via [`scripts/verify-publication-evidence.sh`](../scripts/verify-publication-evidence.sh)
9. Operational workflow proof:
   - archive machine-readable JSON artifacts for inbound-new-email case creation, case reply send, inbound reply threading, public form notification delivery, rule-driven email delivery, knowledge-review notifications, and failure-visible command artifacts for `email-commands` and `notification-commands`
10. Full integration sweep:
   - run `go test -tags=integration ./...`
   - archive the integration log inside the proof bundle
    - run in the same canonical multi-repo workspace shape that CI uses for
      first-party source validation
11. Cross-platform CLI packaging evidence:
   - `bash scripts/build-cli-release.sh`
12. CLI release artifact validation:
   - `bash scripts/verify-cli-release.sh dist/milestone-proof/cli-release`

## Canonical Workspace Refs

- Manifest:
  [`docs/evidence/canonical-workspace-refs.json`](./evidence/canonical-workspace-refs.json)
- Verification script:
  [`scripts/verify-canonical-workspace-refs.sh`](../scripts/verify-canonical-workspace-refs.sh)
- Archived manifest artifact:
  `dist/milestone-proof/workspace-refs/canonical-workspace-refs.json`
- Proof artifact:
  `dist/milestone-proof/workspace-refs/canonical-workspace-refs-verification.json`

## What The Proof Run Does Not Yet Cover

The proof bundle still does not claim anything about command streams that are
outside the Milestone 1 workflow set. In particular, `case-commands` still has
no production consumer and must not be treated as proven. External-provider
reliability on the public internet also remains outside what the archived proof
artifacts can establish.

## Current Live Publication Evidence

- ATS `v0.8.24`: [run 23688333389](https://github.com/MoveBigRocks/extensions/actions/runs/23688333389)
- Error Tracking `v0.8.21`: [run 23688148347](https://github.com/MoveBigRocks/extensions/actions/runs/23688148347)
- Web Analytics `v0.8.21`: [run 23688148371](https://github.com/MoveBigRocks/extensions/actions/runs/23688148371)
- Sales Pipeline beta `v0.1.0`: [run 23683709265](https://github.com/MoveBigRocks/extensions/actions/runs/23683709265)
- Community Feature Requests beta `v0.1.0`: [run 23683709269](https://github.com/MoveBigRocks/extensions/actions/runs/23683709269)

The GitHub proof workflow now downloads and archives those evidence files by
default, and it now cross-checks them against the checked-in archive under
[`docs/evidence/public-bundle-publication/`](./evidence/public-bundle-publication/).
Local reruns work from that durable archive automatically and can still use
either `FIRST_PARTY_PUBLICATION_EVIDENCE_DIR` or
`FIRST_PARTY_PUBLICATION_EVIDENCE_MANIFEST` when you want to override it.
Local reruns still need the canonical sibling repos or equivalent explicit
paths, because the first-party proof rows now fail closed instead of skipping.

## Outputs

The current proof run writes:

- `dist/milestone-proof/summary.md`
- `dist/milestone-proof/integration-go-test.log`
- `dist/milestone-proof/runtime-bootstrap/`
- `dist/milestone-proof/ats-scenario/`
- `dist/milestone-proof/workspace-refs/`
- `dist/milestone-proof/workspace-refs/canonical-workspace-refs.json`
- `dist/milestone-proof/workspace-refs/canonical-workspace-refs-verification.json`
- `dist/milestone-proof/workflow-proof/`
- `dist/milestone-proof/public-bundle-publication/`
- `dist/milestone-proof/public-bundle-publication/publication-evidence-manifest.json`
- `dist/milestone-proof/public-bundle-publication/release-evidence/`
- `dist/milestone-proof/public-bundle-publication/evidence-verification.json`
- `dist/milestone-proof/extensions-validation/`
- `dist/milestone-proof/cli-release/`
- `dist/milestone-proof/cli-release/verification.json`

Those outputs currently provide the concrete proof bundle for:

- milestone readiness status
- first-party pack readiness
- runtime discovery, extension lifecycle, and publication evidence
- durable publication evidence archiving plus manifest/plan verification
- archived input manifests for canonical workspace refs and publication evidence
- canonical sibling-repo ref verification for deterministic multi-repo proof
- milestone-scoped operational workflow evidence
- command failure visibility for the scoped operational streams
- cross-platform CLI release evidence

## Related Evidence

- [`docs/MILESTONE_1_READINESS.md`](./MILESTONE_1_READINESS.md)
- [`docs/FIRST_PARTY_PACK_READINESS.md`](./FIRST_PARTY_PACK_READINESS.md)
- [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md)
- [`docs/testing-strategy.md`](./testing-strategy.md)
- [`docs/WORKFLOW_PROOF_MATRIX.md`](./WORKFLOW_PROOF_MATRIX.md)
- [`docs/WORKFLOW_PROOF_CLOSURE_PLAN.md`](./WORKFLOW_PROOF_CLOSURE_PLAN.md)
