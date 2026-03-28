# Extension Desired-State Reconciliation Plan

This document captures the remaining control-plane gap between the intended Move
Big Rocks extension lifecycle and what the private instance repo can currently
enforce automatically.

## Problem

The current instance-repo model says `extensions/desired-state.yaml` is the
source of truth for which extensions should be installed on a live instance.

That is not yet fully true in production.

Today the private instance repo deploy flow:

- deploys core artifacts pinned in `mbr.instance.yaml`
- deploys service-backed extension runtime binaries from
  `extensions/runtime-manifest.json`
- verifies runtime processes and selected endpoints

It does **not** automatically reconcile `extensions/desired-state.yaml` against
`core_platform.installed_extensions`.

That means runtime processes can advance while the installed bundle version in
the database stays behind. DemandOps hit exactly that drift when the ATS runtime
was updated to `v0.8.25` but the installed ATS bundle remained `0.8.24` until a
manual `mbr extensions upgrade` run corrected it.

## Why This Matters

This is more than one flaky deploy.

It breaks several product promises at once:

- the instance repo is not yet a fully declarative control plane
- `extensions/desired-state.yaml` is intent, not enforced state
- service-backed extension runtimes can drift away from installed bundle state
- verification can fail after a technically healthy core rollout
- operators still need manual, stateful extension surgery after deploys

For Milestone 1 under a "complete and mature platform" bar, that is not good
enough.

## Root Causes

### 1. No canonical reconciler exists

The platform has the primitive lifecycle operations:

- install
- upgrade
- configure
- validate
- activate
- deactivate
- uninstall

But there is no reusable engine that takes declarative desired state and turns
it into a deterministic plan and apply sequence.

### 2. Authority is split across two files

The instance repo currently uses:

- `extensions/desired-state.yaml` for bundle intent
- `extensions/runtime-manifest.json` for runtime artifact deployment

That creates double-entry version management for service-backed packs.

### 3. The desired-state schema is too shallow

The current file can express basic install intent, but not a full operational
target such as:

- extension config values
- secret references for config
- explicit preview workspace rollout
- safe destructive removal intent
- expected runtime compatibility metadata

### 4. The existing CLI path is not enough for automation

`mbr extensions deploy` is useful for operators, but it is not a complete
solution for instance-repo automation:

- bundle-loading logic is currently tied to the CLI package
- instance-scoped installs require instance-admin authority
- a workspace agent token cannot safely reconcile privileged instance-scoped
  packs such as `enterprise-access`

### 5. Verification proves runtime health, not full desired-state convergence

The current production verifier checks runtime processes, sockets, and selected
extension endpoints. That is useful, but it is still narrower than proving:

- the right bundle ref is installed
- the right version is active in the right workspace or instance scope
- the desired config is applied
- no unexpected drift remains

### 6. Auditability is incomplete

`installed_extensions.installed_by_id` exists, but there is no dedicated
extension-reconciliation audit trail today. A mature reconciler needs durable
plan/apply artifacts and attributable actor identity.

## Non-Goals

This plan does **not** treat raw SQL updates to `installed_extensions` as an
acceptable shortcut.

The fix must continue to use the real extension lifecycle so that validation,
schema migration, runtime activation, health checks, and provisioning stay
intact.

## Target End State

The full fix is complete only when all of these are true:

1. The instance repo can declare the desired extension state once.
2. The platform can produce a machine-readable reconcile plan from that state.
3. The platform can apply that plan idempotently with host-safe admin authority.
4. Service-backed runtime artifacts and installed bundle state cannot silently
   diverge.
5. Production verification can fail closed on any remaining drift.
6. Deploy workflows archive plan and apply artifacts as evidence.

## Recommended Design

### A. Make desired state truly authoritative

Keep `extensions/desired-state.yaml` as the human-edited source of truth, but
extend the schema so it can express the full operational target.

At minimum each installed entry should be able to declare:

- `slug`
- `scope`
- `workspace` for workspace-scoped installs
- `ref` for the desired bundle source
- `activate`
- `config`
- `configSecretRefs`
- `previewWorkspace` when rollout should happen in a staging workspace first
- `state` with explicit destructive intent such as `present` or `absent`

For service-backed packs, the same desired state must also own or derive the
runtime artifact expectations so the runtime side is not maintained as an
independent source of truth.

Recommended direction:

- move runtime artifact ownership under the same extension entry
- generate `extensions/runtime-manifest.json` from that declarative state, or
  remove it entirely in favor of generated deploy inputs

### B. Extract a reusable bundle-source package

Move bundle download and decode logic out of `cmd/mbr` into a reusable package
that both the CLI and a host-side reconciler can call.

That package should support the existing source types:

- local directory
- local bundle file
- HTTPS bundle URL
- OCI reference
- marketplace alias

### C. Build a server-aware reconciliation engine

Add a shared reconciliation package in the platform repo that:

- parses desired state
- resolves workspace slugs to IDs
- loads currently installed extensions for instance and workspace scope
- computes a deterministic plan
- applies lifecycle operations in order
- emits plan and result JSON

Plan actions should include:

- `install`
- `upgrade`
- `configure`
- `validate`
- `activate`
- `deactivate`
- `uninstall`
- `noop`
- `drift`

The engine must be idempotent and non-destructive by default.

Important rule:

- deletion from YAML alone must **not** auto-uninstall an extension
- destructive removal should require explicit desired state such as
  `state: absent`

### D. Ship a packaged admin tool with the core release

Add a new host-safe release tool, for example:

- `tools/reconcile-extensions`

This tool should be included in `servicesArtifact` alongside
`tools/create-admin` and `tools/create-agent`.

Required modes:

- `plan`
- `apply`
- `check`

Required outputs:

- machine-readable JSON to stdout or a file
- clear non-zero exit codes on drift or apply failure

This tool should operate directly on platform services under admin context
rather than scraping GraphQL or depending on browser sessions.

### E. Support real admin authority safely

The reconciler must be able to handle both:

- workspace-scoped packs
- instance-scoped packs

That means it cannot rely only on workspace agent tokens.

Recommended path:

- run the packaged reconciler on the host with database-backed admin context
- use a dedicated reconcile actor identity for attribution when available
- record the actor and action set in the result artifact

### F. Treat extension reconciliation as a first-class deploy stage

Extension reconciliation is not slot-local in the same way the core binary is.
Installed extension state lives in the shared database.

So the deploy flow should model extension reconciliation explicitly instead of
pretending it is part of blue-green binary rollout isolation.

Recommended sequence:

1. deploy new core slot
2. restart required extension runtime services
3. confirm core and runtime health
4. run `tools/reconcile-extensions --plan`
5. archive the plan
6. run `tools/reconcile-extensions --apply`
7. archive the result
8. run verification against desired state and runtime state together

If extension changes require preview-first rollout, the desired-state contract
should support that as a separate controlled step before production activation.

## Concrete Work Packages

### Work Package 1: Desired-State Contract

- define the authoritative YAML schema
- document `installed`, `planned`, and explicit removal semantics
- add config and secret-ref support
- define how service-backed runtime refs are represented or derived

### Work Package 2: Reconciliation Engine

- extract reusable bundle acquisition logic from `cmd/mbr`
- implement desired-state parsing and diffing
- resolve workspace slugs and scope constraints
- compare desired bundle refs against installed bundle metadata
- compare desired activation/config state against installed state

### Work Package 3: Packaged Reconciler Tool

- add `tools/reconcile-extensions` to the release artifact
- support `plan`, `apply`, and `check`
- support instance-scoped installs without browser flow
- emit machine-readable artifacts

### Work Package 4: CI and Instance-Repo Integration

- update the instance repo workflow to upload desired-state inputs and invoke
  the reconciler on the host
- archive plan/apply artifacts in Actions
- fail closed on drift

### Work Package 5: Verification Upgrade

Upgrade production verification so it proves:

- desired bundle ref matches installed bundle version
- scope and workspace placement are correct
- activation state is correct
- config was applied where expected
- runtime services and sockets match the reconciled installs

### Work Package 6: Audit and Evidence

- attribute reconciliation actions to a clear actor
- archive plan/apply artifacts per run
- make the verifier consume those artifacts

## Evidence Required Before Calling This Fixed

### Platform tests

- parser tests for desired-state schema
- diff-engine tests for install, upgrade, configure, deactivate, uninstall,
  and noop cases
- integration tests for workspace-scoped reconciliation
- integration tests for instance-scoped reconciliation
- failure tests for invalid workspace, trust failure, validation failure,
  runtime activation failure, and partial apply visibility

### Release proof

- release artifact test proving `tools/reconcile-extensions` is packaged
- CLI or tool contract tests for plan/apply/check JSON outputs

### Instance-repo proof

- workflow artifact containing reconcile plan JSON
- workflow artifact containing reconcile result JSON
- verify step that cross-checks desired state against live installed state

### DemandOps production proof

- one clean DemandOps run where:
  - desired-state changes are applied automatically
  - no manual `mbr extensions upgrade` is needed
  - verify passes on the first run
  - archived plan and result artifacts show zero remaining drift

## Acceptance Criteria

This issue is fully resolved only when:

- a change to `extensions/desired-state.yaml` is enough to drive the live
  installed extension state without manual post-deploy intervention
- service-backed runtime refs and installed bundle refs cannot silently drift
- instance-scoped and workspace-scoped packs both reconcile through supported
  automation
- verification fails closed on any remaining drift
- the resulting deploy evidence is durable and machine-readable

Until then, the instance repo should be treated as having an honest but still
unfinished extension control plane.
