# Customer Instance Setup

This is the canonical production setup guide for Move Big Rocks.

It is written for both humans and agents. If you are using Codex or Claude Code, the repo the agent should open is the **private instance repo**, not the public core repo.

There is no vendor-hosted sandbox path right now. The shortest supported path
is one Ubuntu 22.04+ host or VPS you control. If you are comfortable with a
technical setup, you can also run Move Big Rocks locally while evaluating.

For the full intended lifecycle after initial setup, including when to create a custom extension repo and how extensions should move from development to production, see [Instance and Extension Lifecycle](https://github.com/MoveBigRocks/platform/blob/main/docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md).

## What You Are Creating

A standard Move Big Rocks deployment uses up to three repo types:

1. **Public core repo**
   Shared source, docs, CLI, and signed release artifacts.
2. **Private instance repo**
   The deployment control plane for one live Move Big Rocks installation.
3. **Optional custom extension repo**
   Source code for a customer-built extension.

Do not deploy production from a long-lived fork of the core repo by default.

For most customers, day one should require only:

- the public core repo for understanding the product
- one private instance repo for deployment and operations

Only create a custom extension repo when you are actually building custom extension logic.

The point of this model is that Move Big Rocks becomes the hub for both paths:

- install off-the-shelf extensions when they already exist
- build your own private extensions when you need custom logic
- use the same primitives, deployment model, and review flow for both

## What You Need Before You Start

Have these inputs ready:

- one Ubuntu 22.04+ host or VPS
- SSH access to that host
- one domain you control
- one admin email address
- outbound email choice
- object storage choice for attachments and backups
- GitHub account or organization for the private instance repo
- any purchased extension refs or license grants

Recommended defaults for Milestone 1:

- host: one Ubuntu DigitalOcean Droplet or equivalent VPS
- outbound email: Postmark
- inbound email: Postmark webhook
- storage: S3-compatible object storage
- DNS: `app.<domain>`, `admin.<domain>`, `api.<domain>`

Provider stance:

- fastest documented path: one Ubuntu DigitalOcean Droplet
- equally supported runtime target: any Ubuntu 22.04+ VPS with SSH access and DNS control
- Move Big Rocks does not depend on DigitalOcean-specific APIs for its core deployment model

For exact outbound and inbound Postmark steps, use
[Postmark Setup](https://github.com/MoveBigRocks/platform/blob/main/docs/POSTMARK_SETUP.md).

If the goal is "the agent should do almost everything", the human should only
need to provide:

- the host
- DNS control
- one admin email
- provider credentials
- GitHub auth if the agent will create repos with `gh`
- SSH access to the host

## Step 1: Create the Private Instance Repo

Create a private repo from [MoveBigRocks/instance-template](https://github.com/MoveBigRocks/instance-template).

```bash
gh auth login -h github.com
gh auth status
gh repo create acme/mbr-prod --private --template MoveBigRocks/instance-template --clone
cd mbr-prod
```

If you prefer the GitHub UI, use the template repo directly and create a new
private repo from it.

The new repo should contain:

- `mbr.instance.yaml`
- `scripts/read-instance-config.sh`
- `agents/bootstrap.md`
- `extensions/desired-state.yaml`
- `branding/site.json`
- `security/extension-threat-model.md`
- `security/review-checklist.md`

This repo should stay private because it owns deployment policy and secret wiring for one live installation.

If an agent is going to create the repo for you, it should verify `gh auth status`
before attempting `gh repo create`.

## Step 2: Fill in the Desired State

Edit `mbr.instance.yaml` first.

At minimum, set:

- `metadata.name`
- `metadata.instanceID`
- `spec.domain.app`
- `spec.domain.admin`
- `spec.domain.api`
- `spec.domain.cookie`
- `spec.deployment.linuxTarget.host`
- `spec.deployment.release.core.version`
- `spec.deployment.release.core.servicesArtifact`
- `spec.deployment.release.core.migrationsArtifact`
- `spec.deployment.release.core.manifestArtifact`
- `spec.auth.breakGlassAdminEmail`
- `spec.email.outbound.provider`
- `spec.email.outbound.fromEmail`
- `spec.storage.provider`
- `spec.storage.region`
- `spec.storage.attachmentsBucket`
- `spec.storage.backupsBucket`

Treat this file as the source of truth for:

- which Move Big Rocks release should be running
- which extensions should exist
- which host should receive deployments
- which domains and cookie scope should be used
- which core artifact refs should be deployed
- which storage and email adapters should be used
- which security rules apply to custom extensions

The deploy scripts and service files come from the private instance repo itself
via `deploy/`; only the core runtime artifacts are pinned from the public core
release.

After deployment, `mbr health check --json` should expose the configured `instanceID` back to the operator. That is the identifier marketplace licenses should bind to.

The artifact fields are defined in [Release Artifact Contract](https://github.com/MoveBigRocks/platform/blob/main/docs/RELEASE_ARTIFACT_CONTRACT.md).

Validate it with:

```bash
scripts/read-instance-config.sh mbr.instance.yaml
```

## Step 3: Add Secrets to the Instance Repo

Set repository or environment secrets in the private instance repo.

Put non-secret deployment state in `mbr.instance.yaml`. Keep only sensitive values in GitHub secrets.

Typical Milestone 1 secrets:

- `SSH_KEY`
  Base64-encoded deploy key for the `mbr` user.
- `JWT_SECRET`
  Strong random string for session signing.
- `METRICS_TOKEN`
  Strong random bearer token for protected metrics.
- `POSTMARK_SERVER_TOKEN`
  Postmark outbound server token when using Postmark.
- `POSTMARK_WEBHOOK_SECRET`
  Random webhook secret for inbound validation when using Postmark inbound webhooks.
- `SMTP_USERNAME`
  SMTP username when using generic SMTP outbound.
- `SMTP_PASSWORD`
  SMTP password when using generic SMTP outbound.
- `SES_ACCESS_KEY`
  AWS SES access key when using SES outbound.
- `SES_SECRET_KEY`
  AWS SES secret key when using SES outbound.
- `AWS_ACCESS_KEY_ID`
  Storage access key when using `s3-compatible` storage.
- `AWS_SECRET_ACCESS_KEY`
  Storage secret key when using `s3-compatible` storage.
- `BACKUP_ALERT_EMAIL`
  Optional email address for backup verification failures.
- `REGISTRY_USERNAME`
  Optional OCI registry username for private artifact pulls.
- `REGISTRY_TOKEN`
  Optional OCI registry token for private artifact pulls.

If you are using paid extensions, also add the license or registry credentials required by those bundles.

For production instances, also treat extension trust verification as mandatory,
not optional. Production should set:

- `INSTANCE_ID`
- `EXTENSION_TRUST_REQUIRE_VERIFICATION=true`
- `EXTENSION_TRUSTED_PUBLISHERS_JSON`
- `ENTERPRISE_ACCESS_ALLOWED_HOSTS` if `enterprise-access` will be enabled
- `ENTERPRISE_ACCESS_ALLOW_ENV_SECRET_REFS=false` unless an explicit break-glass exception is approved

Do not install remote or paid extension bundles into production without that
trust configuration in place.

## Step 4: Prepare DNS and the Host

Before the first deployment:

1. Point the domains declared in `mbr.instance.yaml` at the host.
2. Create the `mbr` deploy user on the host.
3. Install the base host dependencies from the setup guide in the instance repo.
4. Confirm the host can accept SSH from the deploy key.

The host should be considered disposable infrastructure. The instance repo is the durable control plane.

## Step 5: Let an Agent Bootstrap the Instance

Open the **private instance repo** in Codex or Claude Code.

Tell the agent to follow `agents/bootstrap.md`.

The bootstrap run should:

1. Validate `mbr.instance.yaml`.
2. Confirm the parsed host, domains, provider choices, and artifact refs from `scripts/read-instance-config.sh`.
3. Confirm DNS and host reachability.
4. Bootstrap the host.
5. Pull the pinned Move Big Rocks artifacts for the declared version.
6. Render the environment file from `mbr.instance.yaml` plus repo secrets.
7. Deploy the core release.
8. Create the first admin user.
9. Verify health for the app, admin, and API domains.
10. Configure outbound email.

The agent should not modify the public core repo to do this.

## Step 6: Install Optional Extensions

Optional capabilities should be installed after core is healthy.

Use `extensions/desired-state.yaml` to record which extensions should exist.

Example categories:

- `ats`
- `enterprise-access`
- `error-tracking`
- `web-analytics`

The recommended activation flow is:

1. Add the licensed extension ref to `extensions/desired-state.yaml`.
2. Install the bundle into the running instance.
3. Validate signature, manifest, and license.
4. Configure extension-specific settings.
5. For `enterprise-access`, restrict provider hosts to the approved IdP domains and use non-literal `clientSecretRef` values.
6. Run the required checks in a preview workspace first.
7. Monitor the extension in that preview workspace.
8. Activate it in the target production workspace only after the preview pass is clean.

Do not activate privileged auth or connector extensions through the generic custom-extension path.

Use a dedicated preview workspace on the live instance for this preview pass.

Current repo baseline:

- local development can install from a bundle file, an extension source directory with `manifest.json` plus `assets/`, or an HTTPS bundle URL
- the ATS reference extension should come from the public `MoveBigRocks/extension-examples` repo, not from the public core repo

## Step 7: Build a Custom Extension Only If You Need One

If you want custom functionality beyond branding, content overrides, and extension configuration:

1. Create a separate extension repo from [MoveBigRocks/extension-sdk](https://github.com/MoveBigRocks/extension-sdk).
2. Build the extension there.
3. Run the threat model and review checklist.
4. Publish a signed bundle to your registry, keep a local bundle file, or during development install directly from the extension source directory.
5. Add the extension ref to the instance repo.
6. Install, validate, and activate it from the instance repo workflow.

Example bootstrap:

```bash
gh auth status
gh repo create acme/custom-extension --private --template MoveBigRocks/extension-sdk --clone
cd custom-extension
```

Do not put substantial custom extension source code into the instance repo.

This is not only for niche custom workflows. If you want your own version of something like analytics, an internal dashboard, or another operational workflow, you should be able to build it as an extension instead of waiting for a marketplace pack.

Simple customizations that should stay in the instance repo:

- branding
- copy
- content overrides
- desired-state changes
- extension configuration values

Heavier customizations that should move to a custom extension repo:

- new business logic
- custom routes
- custom admin workflows
- extension-owned entities
- complex templates with real application behavior

Use this rule:

- if the software needs Move Big Rocks primitives, Move Big Rocks auth, or Move Big Rocks public routes, build it as an extension
- if it is a standalone service, keep it in its own repo and integrate with Move Big Rocks

## Upgrading Core

To deploy a new platform release to a running instance:

1. Check available releases on the [platform tags page](https://github.com/MoveBigRocks/platform/tags) or with `git tag --sort=-v:refname` in the platform repo.
2. In the private instance repo, update all four fields under `spec.deployment.release.core` in `mbr.instance.yaml`:
   - `version`
   - `servicesArtifact`
   - `migrationsArtifact`
   - `manifestArtifact`
3. Commit and push to `main`. The deploy workflow handles blue-green rollout, health checks, and smoke tests.
4. To roll back, revert the version change and push again.

The artifact version tag must match across all four fields. The deploy workflow will pull the OCI artifacts, deploy to the inactive slot, health-check, and switch traffic.

For a detailed runbook with verification commands, rollback procedures, and slot reference, see `deploy/UPGRADE.md` in your instance repo.

## Deployment Notes

- **GHCR packages must be public** for unauthenticated `oras pull` during deployment. If packages are private, the deploy workflow must authenticate with `REGISTRY_USERNAME` and `REGISTRY_TOKEN`.
- **Ports 8080/8081 are defaults.** On shared hosts where another service already binds those ports, update the port numbers in the systemd service files, Caddyfile, and deploy workflow accordingly.
- **PostgreSQL 18+ required.** The schema uses native UUIDv7 generation (`gen_random_uuid_v7()`), which requires PostgreSQL 18 or later.

## Success Checklist

A setup is complete when all of these are true:

- the app, admin, and API domains resolve correctly
- the instance serves health endpoints successfully
- the first admin can log in
- outbound email works
- backups are configured
- the private instance repo pins the deployed core version
- optional extensions are recorded in `extensions/desired-state.yaml`
- any custom extension has passed the threat model and review checklist before activation

## Rules of Thumb

- Open the private instance repo when operating a live installation.
- Open the core repo when changing Move Big Rocks itself.
- Open a custom extension repo only when building real new product logic.
- Keep the base product free and small.
- Keep privileged auth and connector extensions first-party only until the privileged runtime has been fully dogfooded and opened up beyond trusted first-party publishers.
