# Release Artifact Contract

This document defines the artifact contract that a private Move Big Rocks instance repo consumes.

The goal is simple:

- the public core repo builds and publishes artifacts
- the private instance repo pins exact artifact refs
- the instance repo deploy workflow consumes those refs without rebuilding core

## Required Core Artifacts

Every pinned core release in `mbr.instance.yaml` should declare these refs under `spec.deployment.release.core`:

- `version`
  Human-readable release version such as `v1.1.0`
- `servicesArtifact`
  OCI ref for the Linux server binary package, for example `ghcr.io/movebigrocks/mbr-services:v1.1.0`
- `migrationsArtifact`
  OCI ref for database migrations, for example `ghcr.io/movebigrocks/mbr-migrations:v1.1.0`
- `manifestArtifact`
  OCI ref for the release manifest tying the other artifacts together, for example `ghcr.io/movebigrocks/mbr-manifest:v1.1.0`

Deploy scripts, service files, and host bootstrap assets are owned by the
private instance repo created from `MoveBigRocks/instance-template`. They are
not published as a separate core artifact.

## Artifact Contents

`servicesArtifact` should expand to:

- `mbr-server`
- `tools/create-admin`
- `tools/create-agent`

`migrationsArtifact` should expand to:

- the SQL migration tree needed by the release

`manifestArtifact` should contain:

- release version
- git SHA
- build date
- artifact digests and refs for `services` and `migrations`

## Instance-Repo Rules

- the instance repo must pin exact refs in `mbr.instance.yaml`
- non-secret refs live in git
- secrets required to consume private registries live in GitHub secrets
- deployment workflows must not rebuild or re-tag core
- deployment workflows should only pull, verify, unpack, and deploy the pinned artifacts

## Why This Matters

This keeps the product model honest:

- customers and first-party operators deploy the same kind of pinned core release
- agents have one visible desired-state file to inspect
- upgrades become a change to pinned refs, not a rebuild of core
- the public core repo can stay focused on build and publish responsibilities
- the instance repo stays responsible for deploy assets and host policy
