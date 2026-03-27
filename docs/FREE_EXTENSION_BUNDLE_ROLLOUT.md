# Free Extension Bundle Rollout

This document records the current rollout plan for the first public
off-the-shelf Move Big Rocks extension set.

## Goal

Make the standard-risk first-party extension set easy to discover, trust, and
install:

- `ats`
- `error-tracking`
- `web-analytics`

Keep `enterprise-access` separate as a privileged first-party identity pack.

## Public Positioning

- Move Big Rocks core is source-available and self-hostable.
- ATS, error tracking, and web analytics are free public first-party bundles.
- Teams can build their own extensions.
- The public license allows internal company use, modification, and private
  extensions, but does not permit selling or licensing the platform,
  derivative works of it, extensions, or access to them, and does not permit
  offering them as a hosted service, without separate written permission from
  Move Big Rocks BV.

## Distribution Model

- `platform`
  Public core source of truth for runtime contracts, CLI behavior, docs, and
  milestone scope.
- `packs`
  Controlled first-party authoring home for privileged packs and other
  first-party work that should not publish from the public repo.
- `extensions`
  Public first-party extensions repo and publication surface for ATS, error
  tracking, and web analytics, including the release workflow and
  `catalog/public-bundles.json`.
- `site-prod`
  Public website and install/discovery surfaces.
- `ghcr.io/movebigrocks/mbr-ext-*`
  Public signed OCI bundle delivery surface for the free first-party bundle
  set.

## Repo Structure Decision

- keep first-party production extensions out of `platform`
- keep them together in one official public first-party extensions repo for now
- keep templates, scaffolds, and sample material in `extension-sdk`
- keep privileged or not-yet-public first-party work in `packs`
- split a first-party extension into its own repo later only when ownership,
  release cadence, support burden, or compliance requirements truly diverge

## Topology Options

### Option A: Keep first-party extensions in `platform`

Pros:

- one repo to browse
- simple cross-repo coordination during early development

Cons:

- teaches the wrong pattern to builders, because customer-built extensions do
  not live in `platform`
- couples core releases and extension releases too tightly
- makes the core repo noisier and less trustworthy as a bounded runtime source
- weakens the product signal that extensions are real installable products with
  their own lifecycle

Verdict:

- not recommended as the long-term public model

### Option B: One public first-party extensions repo with multiple extensions

Pros:

- matches how most customers should think: core repo, instance repo, optional
  custom extension repo, plus an official first-party extensions catalog
- gives ATS, error tracking, and web analytics one coherent discovery and trust
  surface
- allows one shared release workflow, one signing setup, one public catalog,
  and one place to explain install and support policy
- still lets each extension have its own directory, manifest, migrations, and
  release tags
- keeps the public source structure close enough to what outside builders will
  do in their own extension repos

Cons:

- some shared repo coordination is still needed when one extension changes
- repo naming must make clear that this is not a toy example collection

Verdict:

- recommended now

### Option C: One repo per first-party extension

Pros:

- strongest product separation
- independent issue trackers, release cadence, branch protection, and
  maintainers
- clean public message that ATS is a real product, not just one folder beside
  others

Cons:

- more operational overhead immediately
- repeated release workflows, signing setup, catalog maintenance, and policy
  docs unless a separate catalog repo is also introduced
- weaker initial discovery surface because the catalog becomes fragmented
- harder to present a coherent first-party extension lineup while adoption is
  still early

Verdict:

- probably right later for mature extensions with separate teams or materially
  different support/compliance needs, but premature right now

## Recommended Model

Use a hybrid model:

- `platform` stays the core runtime and contract repo
- `extension-sdk` stays the public scaffold and sample repo for builders
- one public first-party extensions repo, ideally renamed to
  `MoveBigRocks/extensions`, owns ATS, error tracking, and web analytics plus
  the public release catalog
- `packs` remains the controlled repo for privileged or not-yet-public
  first-party work
- GHCR remains the actual install surface, with one package per extension:
  `ghcr.io/movebigrocks/mbr-ext-ats`,
  `ghcr.io/movebigrocks/mbr-ext-error-tracking`, and
  `ghcr.io/movebigrocks/mbr-ext-web-analytics`

That gives us product separation at the package level and catalog coherence at
the repo level.

## Workstreams

1. License and wording
   - make the public license posture explicit
   - remove stale "paid extension" language from public-facing docs
2. Pack source reconciliation
   - choose one canonical authoring source per first-party pack
   - keep proof snapshots aligned with authoring manifests
3. Public docs and website
   - explain which packs are free public bundles
   - explain where bundle refs live
   - explain how to install them from the standard signed-bundle flow
4. Publication pipeline
   - build signed bundle artifacts
   - publish public OCI refs
   - flip first-published GHCR package visibility to `Public`
   - record digests and version mapping
5. Install ergonomics
   - allow public signed bundles to install without an instance-bound token
   - keep `--license-token` available for controlled bundle flows

## Immediate Follow-Up

- keep the public first-party extensions repo and the controlled first-party repo
  aligned, with no mirror manifests left behind in `platform`
- run the first public bundle publication workflow
- change each first-published GHCR package to `Public` in package settings and
  verify anonymous pull by OCI ref
- keep the public site, README, and operator docs aligned with the actual
  publication state
