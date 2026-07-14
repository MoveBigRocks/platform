# Extension Desired-State Reconciliation

This document defines the extension desired-state reconciliation contract for
private Move Big Rocks instance repos.

## Why Reconciliation Exists

Without reconciliation, an instance repo declares extension intent in
`extensions/desired-state.yaml`, but a deploy rolls forward only service-backed
runtime binaries and does not reconcile `core_platform.installed_extensions`.

That leaves three things free to drift:

- desired bundle intent
- deployed service-backed runtime binaries
- installed extension rows in PostgreSQL

The characteristic failure is runtime drift: a runtime version moves ahead of the
installed bundle version until a manual `mbr extensions upgrade` corrects it.
Reconciliation closes that gap by making desired state authoritative.

## How Reconciliation Works

### 1. Desired state is authoritative

Private instance repos treat `extensions/desired-state.yaml` as the single
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

Service-backed runtime deployment does not rely on a separately curated,
checked-in `extensions/runtime-manifest.json`.

The runtime manifest is generated from desired state by:

- `tools/reconcile-extensions render-runtime-manifest`

That generated manifest becomes the deploy input for runtime artifact pulls and
runtime verification.

### 3. Bundle sources are shared between the CLI and the reconciler

Bundle acquisition is shared between the CLI and the reconciler through the
platform bundle-loading packages rather than being trapped inside `cmd/mbr`.

Supported source types are:

- local directory
- local bundle file
- HTTPS bundle URL
- OCI reference
- marketplace alias

### 4. The reconciliation engine

The platform ships a deterministic desired-state engine that:

- parses desired state
- resolves workspace slugs to IDs
- loads installed instance and workspace extensions
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

### 5. The packaged host-side admin tool

Core release artifacts include:

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

Instance-repo deploy:

1. pulls the pinned core artifacts
2. generates a runtime manifest from desired state
3. pulls the required service-backed runtime artifacts
4. deploys the new core slot
5. runs `deploy/reconcile-extensions.sh apply-all` on the host
6. downloads `plan.json`, `apply.json`, `check.json`, and the generated
   `runtime-manifest.json`
7. uploads those files as deploy artifacts

Verification:

1. runs `deploy/reconcile-extensions.sh check` on the host
2. uploads verification reconciliation artifacts
3. cross-checks runtime services, sockets, runtime protocol rows, and endpoints
   against the generated runtime manifest
4. fails closed on remaining drift

## Contract For Instance Repos

The required operating model for private instance repos is:

1. edit `extensions/desired-state.yaml`
2. pin the desired core release in `mbr.instance.yaml`
3. push to `main`
4. let deploy and verify reconcile the live extension state automatically
5. inspect uploaded reconciliation artifacts if verification fails

Manual `mbr extensions ...` commands remain valid repair tools, but they are not
the normal production control plane.

## What This Does Not Claim

Reconciliation covers the control-plane gap between declared and installed
extension state. It does not claim that every possible custom extension workflow
is frictionless.

Still true:

- preview-first rollout is a policy choice the instance repo can layer on top
- custom extension authoring and review still require the documented security
  gates
- GitHub Actions workflow maintenance remains normal CI upkeep, not a blocker
  against reconciliation
