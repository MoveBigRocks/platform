# Instance and Extension Lifecycle

This document defines the intended end-to-end user experience for Move Big Rocks.

It is written from first principles:

- what should a first-time customer have to understand
- what should an agent be able to do without improvising
- where should configuration live
- where should custom code live
- how should an extension move from idea to production safely

The goal is not just to make the architecture technically sound. The goal is to make the whole system feel understandable, predictable, and safe.

## Product UX Rule

Milestone 1 should optimize for this simple default:

1. one public core repo
2. one private instance repo
3. zero extra repos unless the customer is building real custom extension logic

That means:

- a customer can deploy free core with only one private repo
- a customer can buy and install first-party extensions without creating extra repos
- a second repo is needed only for customer-built extension code
- branding, copy, desired state, and deployment policy stay in the instance repo
- real custom logic lives in a separate extension repo

If the setup asks for more than that on day one, it is too complex.

## The Intended User Journey

The ideal Milestone 1 customer journey is:

1. Discover Move Big Rocks through the public core repo and docs.
2. Understand the model quickly:
   - free core first
   - extensions are optional
   - Move Big Rocks is self-hosted
- the CLI is the operator and agent surface
3. Create one private instance repo from the public instance-template repo.
4. Open that repo in Codex or Claude Code.
5. Tell the agent to follow `START_HERE.md`.
6. Give the agent SSH access, DNS, email, and storage credentials.
7. End with a working Move Big Rocks installation.
8. Later:
   - install first-party extensions
   - customize branding and content
   - optionally create a private custom extension repo

That is the right target user experience.

## Agent-First Discovery

The public repo should support a one-URL discovery flow.

That means a user should be able to say:

> Open the Move Big Rocks repo and tell me what I need to do to deploy my own instance and build or install the extensions I need.

To support that, the public repo needs:

- one obvious public agent handoff file
- a small recipe library for common jobs
- a clear distinction between:
  - public core repo
  - private instance repo
  - optional private extension repo
- CLI verbs and docs that use the same language

If the agent cannot infer the correct next step from the public repo and one or two linked docs, the discovery flow is still too weak.

## The Right Repo Model

The repo model should stay simple and explicit.

### 1. Public core repo

Purpose:

- shared source
- docs
- CLI
- release artifacts
- extension runtime contracts

It should not be the deployment control plane for live installations.

### 2. Private instance repo

Purpose:

- desired state for one installation
- deployment workflows
- extension desired state
- branding overrides
- operational policy
- threat-model and review prompts

This is the repo an operator or agent opens to:

- deploy core
- configure the instance
- install or upgrade extensions
- rotate credentials
- monitor health
- apply branding

For most customers, this should be the only private repo they need.

### 3. Optional custom extension repo

Purpose:

- source code for customer-built extension logic

This repo is only needed when the customer is building:

- new business logic
- custom routes
- custom admin workflows
- service-backed extension behavior
- extension-owned data or views

It is not needed for:

- branding
- copy changes
- turning first-party extensions on or off
- filling in desired-state config

## The One-File Agent Handoff

The instance repo should have one canonical handoff file:

- `START_HERE.md`

That file should be enough for an agent to understand:

- what this instance is supposed to become
- which files matter
- which actions are safe
- which actions are not allowed
- what "done" means

The customer should be able to say:

> Open `START_HERE.md` in my Move Big Rocks instance repo and follow it.

The same pattern now applies to the public extension SDK repo:

- one top-level `START_HERE.md`
- one extension manifest
- one local dev path
- one packaging path
- one review path

## The Right Extension Lifecycle

An extension should move through these stages.

## Stage 1: Decide Whether This Is Really an Extension

Ask:

- Is this just branding, content, or desired-state config?
- Or is this new operational behavior?

Keep it in the instance repo if it is:

- branding
- copy
- desired-state changes
- extension config values
- light template overrides with no real new logic

Create a custom extension repo if it is:

- new routes
- new workflows
- new admin screens
- new event consumers
- new service-backed behavior
- new extension-owned entities or data

That boundary matters because it keeps the instance repo small and the upgrade story clean.

## Stage 2: Create the Extension Repo

The extension author should create a new private repo from
[MoveBigRocks/extension-sdk](https://github.com/MoveBigRocks/extension-sdk).

That repo should contain:

- `START_HERE.md`
- `manifest.json`
- `assets/`
- extension code if service-backed
- migrations if service-backed
- local run instructions
- test instructions
- threat-model prompt
- review checklist

This repo should be private by default.

Publishing to a marketplace should be optional.

## Stage 3: Run the Extension Locally

The extension repo should support a simple local loop:

1. run local Move Big Rocks core
2. install the extension from the source directory
3. validate the manifest
4. activate the extension in a local or sandbox workspace
5. exercise the routes, forms, automation, and admin pages

The key local development command pattern should stay simple:

```bash
mbr extensions install ./my-extension --workspace ws_local
```

Public signed bundles can use the same pattern without an instance-bound token.
Keep `--license-token` available for controlled bundle flows such as
instance-bound distribution or a future private catalog.

For service-backed extensions, the local flow should also cover:

- starting the extension runtime locally
- health check verification
- endpoint verification
- event-consumer verification
- migration application against a local Postgres database

## Stage 4: Review and Threat-Model the Extension

Before activation in a real instance, the extension should go through a standard review flow:

1. identify permissions and capabilities requested
2. identify public and admin endpoints
3. identify secrets and external systems involved
4. identify data ownership and storage
5. identify event subscriptions and emitted events
6. identify failure modes and rollback path

This is especially important for:

- service-backed extensions
- extensions with custom endpoints
- extensions that touch email, auth, or external integrations

Milestone 1 should make this review path explicit and agent-friendly, not implicit tribal knowledge.

## Stage 5: Build and Package the Extension

The extension repo should produce a signed bundle.

## One Supported Private-Extension Path

Milestone 1 now has one explicit end-to-end path for private extension work.

1. Create a private repo from
   [MoveBigRocks/extension-sdk](https://github.com/MoveBigRocks/extension-sdk).
2. Implement the extension in that repo, keeping deployment policy and desired
   state in the private instance repo.
3. Run the offline contract pass from the extension repo:

   ```bash
   mbr extensions lint . --write-contract --json
   ```

4. Run the online verification pass against a safe workspace on a real or
   local Move Big Rocks instance:

   ```bash
   mbr extensions verify . --workspace ws_preview --json
   ```

5. During source-first development, install from the source directory and
   exercise the supported lifecycle:

   ```bash
   mbr extensions install . --workspace ws_preview --json
   mbr extensions validate --id ext_123 --json
   mbr extensions activate --id ext_123 --json
   mbr extensions monitor --id ext_123 --json
   ```

6. If the extension is service-backed, verify runtime startup, migrations,
   health, and event-consumer behavior as part of that preview pass.
7. Package and sign the bundle when the preview pass is clean.
8. Add the bundle ref plus config to the instance repo
   `extensions/desired-state.yaml`.
9. Push the instance repo so deploy and verify can reconcile the desired state
   automatically on the target host.
10. Inspect the uploaded reconciliation artifacts if deploy or verify reports
    drift.

That is the supported path agents and humans should be able to follow without
inventing extra glue.

That means:

- validate the manifest
- package assets and metadata
- include migrations when needed
- sign the bundle
- output a local bundle and optionally an OCI-published artifact

The same extension should be installable from:

- a local source directory during development
- a local bundle file
- an OCI ref
- a marketplace alias when a private catalog is in use

The current public first-party default is a signed OCI ref rather than a
marketplace alias.

## Stage 6: Install in a Sandbox Workspace First

Before production activation, the operator should be able to:

- install the extension
- validate it
- configure it
- activate it in a sandbox workspace
- run checks
- inspect health
- deactivate it if needed

This is important because extension deployment should not feel like "go straight to production and hope."

For Milestone 1, the minimum safe preview path should be:

- install into a non-critical sandbox workspace on the live instance
- run `mbr extensions validate`
- run `mbr extensions monitor`
- verify the expected routes and workflows manually or with scripted checks

Use a dedicated sandbox workspace on the live instance for preview and rollout
validation.

## Stage 7: Activate in Production

Production activation should happen from the private instance repo flow.

The operator or agent should:

1. record the desired extension ref in `extensions/desired-state.yaml`
2. let the private instance repo deploy flow generate the runtime manifest,
   deploy required runtimes, and reconcile the installed bundle state
3. verify signature and any required bundle-install credential through the real
   lifecycle operations
4. configure required settings
5. validate the extension
6. activate it in the target workspace or instance
7. verify health and the expected UX

The instance repo should remain the source of truth for what should be running.

Current model:

- the private instance repo deploy flow now auto-reconciles
  `extensions/desired-state.yaml` into `installed_extensions`
- service-backed runtime deployment now derives from generated desired-state
  runtime manifests instead of a separate hand-maintained file
- deploy and verify archive reconciliation artifacts and fail closed on drift

The control-plane record for that implementation lives in
[Extension Desired-State Reconciliation](./EXTENSION_DESIRED_STATE_RECONCILIATION_PLAN.md).

## Stage 8: Monitor, Upgrade, Deactivate, Uninstall

The lifecycle does not stop at activation.

Operators and agents must also be able to:

- inspect extension metadata
- inspect endpoints and health
- upgrade an extension in place
- deactivate it safely
- uninstall it safely
- archive or export data before destructive removal

The default removal path should be:

1. deactivate
2. confirm no dependent workflow is still active
3. export or archive extension-owned data if relevant
4. uninstall
5. optionally drop extension-owned schema or artifacts only after confirmation

## What Must Feel Easy

For Milestone 1, these tasks need to feel straightforward:

- deploy Move Big Rocks on a fresh Linux machine
- log in through the browser-backed CLI flow
- install a first-party extension
- upload files and configure forms
- build a private custom extension
- test it locally
- test it in a safe environment
- activate it
- monitor it
- upgrade it

If any of these require reading five unrelated documents and inventing the missing glue, the user experience is still wrong.

## Where the UX Is Right Today

The direction is right:

- one private instance repo by default
- one CLI
- one agent handoff file
- optional second repo only for real custom logic
- extensions as the main path for off-the-shelf and customer-built capabilities

That is the right product model.

## Where the UX Is Still Not Good Enough

The remaining friction points are now smaller and mostly about polish:

- some first-party extensions are still intentionally narrower than the long-term
  product vision
- preview and security review policy still spans the instance repo plus the
  extension repo, even though the supported command path is now explicit
- sandbox activation and extension review guidance still spans multiple files

Those are the remaining experience gaps to close.

## Milestone 1 UX Decision

Yes, we are targeting the right user experience.

The right Milestone 1 shape is:

- free core Move Big Rocks is valuable on its own
- one private instance repo is enough for most customers
- the CLI is the main operator and agent surface
- extensions are the way to add optional product areas
- custom extensions are private by default
- agents should be able to help with setup, deployment, extension authoring, validation, activation, and operation

The work that remains is not a change of direction. It is finishing the runtime, repo, and documentation layers so the experience becomes true end to end.
