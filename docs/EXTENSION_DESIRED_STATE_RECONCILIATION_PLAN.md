# Extension Desired-State Reconciliation

This document records the now-implemented extension desired-state reconciliation
model for private Move Big Rocks instance repos.

Status as of March 28, 2026:

- implemented in core release `v0.13.0`
- shipped in `servicesArtifact` as `tools/reconcile-extensions`
- integrated into the DemandOps instance repo deploy and verify workflows
- proven on `mbr.demandops.com` by production run `23695306450`

## Problem This Closed

The original gap was that instance repos declared extension intent in
`extensions/desired-state.yaml`, but production deploys only rolled forward
service-backed runtime binaries and did not automatically reconcile
`core_platform.installed_extensions`.

That made three things drift-prone:

- desired bundle intent
- deployed service-backed runtime binaries
- installed extension rows in PostgreSQL

The concrete failure we hit was ATS runtime drift: the runtime version moved
ahead of the installed ATS bundle version until a manual `mbr extensions
upgrade` corrected it.

## What Is Implemented

### 1. Desired state is authoritative

Private instance repos now treat `extensions/desired-state.yaml` as the single
human-edited declaration of extension intent.

Supported entry fields include:

- `slug`
- `scope`
- `workspace`
- `ref`
- `activate`
- `state`
- `config`
- `configSecretRefs`

`state: absent` is the explicit destructive path. Deleting an entry alone does
not silently uninstall it.

### 2. Runtime manifest is generated, not hand-maintained

Service-backed runtime deployment no longer relies on a separately curated
checked-in `extensions/runtime-manifest.json`.

The runtime manifest is now generated from desired state by:

- `tools/reconcile-extensions render-runtime-manifest`

That generated manifest becomes the deploy input for runtime artifact pulls and
runtime verification.

### 3. Shared bundle source loading exists

Bundle acquisition is shared between the CLI and the reconciler through the
platform bundle-loading packages instead of being trapped inside `cmd/mbr`.

Supported source types remain:

- local directory
- local bundle file
- HTTPS bundle URL
- OCI reference
- marketplace alias

### 4. A real reconciliation engine exists

The platform now ships a deterministic desired-state engine that:

- parses desired state
- resolves workspace slugs to IDs
- loads currently installed instance and workspace extensions
- computes a machine-readable plan
- applies lifecycle operations through the real extension service
- checks for remaining drift

Plan and apply actions include:

- `install`
- `upgrade`
- `configure`
- `validate`
- `activate`
- `deactivate`
- `uninstall`
- `noop`
- `drift`

### 5. A packaged host-side admin tool exists

Core release artifacts now include:

- `mbr-server`
- `tools/create-admin`
- `tools/create-agent`
- `tools/reconcile-extensions`

The reconciler supports:

- `render-runtime-manifest`
- `plan`
- `apply`
- `check`

It runs under database-backed admin context on the host, so it can reconcile
both workspace-scoped and instance-scoped extensions without relying on browser
sessions or workspace agent tokens.

### 6. Reconciliation is part of deploy and verify

Instance-repo deploy now:

1. pulls the pinned core artifacts
2. generates a runtime manifest from desired state
3. pulls the required service-backed runtime artifacts
4. deploys the new core slot
5. runs `deploy/reconcile-extensions.sh apply-all` on the host
6. downloads `plan.json`, `apply.json`, `check.json`, and the generated
   `runtime-manifest.json`
7. uploads those files as deploy artifacts

Verification now:

1. runs `deploy/reconcile-extensions.sh check` on the host
2. uploads verification reconciliation artifacts
3. cross-checks runtime services, sockets, runtime protocol rows, and endpoints
   against the generated runtime manifest
4. fails closed on remaining drift

## Evidence

### Platform evidence

Implemented in `platform` and released in `v0.13.0` from commit `fb389d3`.

Relevant shipped areas:

- `internal/platform/extensionbundle`
- `internal/platform/extensiondesiredstate`
- `internal/platform/extensionreconcile`
- `cmd/reconcile-extensions`
- `.github/workflows/_build.yml`

### DemandOps production proof

DemandOps run:

- [Production Deploy run 23695306450](https://github.com/DemandOps/mbr-prod/actions/runs/23695306450)

Observed live result:

- API health serving build commit `fb389d3`
- admin health passing
- generated runtime manifest deployed for ATS, Web Analytics, and Error Tracking
- deploy reconciliation artifact set archived
- verify reconciliation artifact set archived
- verify passed on the first run with no manual extension follow-up

Local artifact copy used during proof review:

- `/tmp/demandops-run-23695306450-artifacts/extension-reconcile-production/plan.json`
- `/tmp/demandops-run-23695306450-artifacts/extension-reconcile-production/apply.json`
- `/tmp/demandops-run-23695306450-artifacts/extension-reconcile-production/check.json`
- `/tmp/demandops-run-23695306450-artifacts/extension-reconcile-production/runtime-manifest.json`
- `/tmp/demandops-run-23695306450-artifacts/extension-reconcile-verify-production/check.json`
- `/tmp/demandops-run-23695306450-artifacts/extension-reconcile-verify-production/runtime-manifest.json`

Those artifacts showed:

- deploy `plan.json` with only `noop` operations for the current desired state
- deploy `apply.json` clean
- deploy `check.json` clean
- verify `check.json` clean

## Contract For Instance Repos

The required operating model for private instance repos is now:

1. edit `extensions/desired-state.yaml`
2. pin the desired core release in `mbr.instance.yaml`
3. push to `main`
4. let deploy and verify reconcile the live extension state automatically
5. inspect uploaded reconciliation artifacts if verification fails

Manual `mbr extensions ...` commands remain valid repair tools, but they are no
longer the normal production control plane.

## What This Does Not Claim

This closes the control-plane reconciliation gap. It does not claim that every
possible custom extension workflow is frictionless.

Still true:

- preview-first rollout is a policy choice the instance repo can layer on top
- custom extension authoring and review still require the documented security
  gates
- GitHub Actions workflow-action maintenance remains normal CI upkeep, but it
  is no longer a blocker or open warning against the shipped reconciliation
  path
