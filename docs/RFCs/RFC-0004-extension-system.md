# RFC-0004: Extension System

## Status

draft

## Summary

Move Big Rocks extensions are signed, versioned bundles that install into a workspace
or instance and reuse shared operational primitives instead of recreating
product silos.

## Goals

- keep core small
- make optional capability packs installable
- enable agent-driven installation and operation
- support a commercial bundle-and-license model
- make ATS the reference extension

## Shared Primitives

Extensions build on:

- workspace
- collection
- case
- contact
- knowledge resource
- form spec
- form submission
- attachment
- automation
- public route
- agent

## Bundle Manifest

Each bundle declares:

- name, version, publisher
- kind, scope, risk level
- requested permissions
- workspace provisioning or attachment rules
- extension-owned entities
- seed collections
- knowledge resources
- form specs
- seed automation rules
- public routes
- admin routes
- scheduled jobs
- CLI command catalog declarations
- bundled agent skills or equivalent operator guidance
- admin navigation items
- dashboard and widget registrations
- install, upgrade, activate, deactivate, and uninstall hooks
- endpoint declarations with auth and routing metadata

Extension kinds:

- `product`
- `identity`
- `connector`
- `operational`

Scope:

- `workspace`
- `instance`

Risk:

- `standard`
- `privileged`

## Installation Model

1. Acquire a signed bundle and license grant outside Move Big Rocks.
2. Upload or reference the bundle.
3. Validate signature, digest, manifest, and license.
4. Activate the extension in a target workspace or instance.
5. Provision routes, defaults, command catalogs, skill assets, and runtime state.

An activation flow may either:

- attach the extension to an existing workspace
- provision a dedicated workspace if the extension declares that as an allowed install mode

Identity and connector packs use stricter review and signing policy than
ordinary product packs.

## Runtime Classes

Move Big Rocks supports two extension runtime classes:

### Bundle Extensions

Used for:

- static or mostly declarative product packs
- seeded collections
- knowledge and forms assets
- configurable assets
- simple public and admin routes

### Service-Backed Extensions

Used for:

- dynamic product packs with custom handlers
- ingest endpoints
- scheduled jobs and event consumers
- rich admin UI
- privileged identity and connector integrations

Core remains responsible for auth, tenancy, routing, lifecycle, and health
supervision. The extension owns its specialized behavior.

## Endpoint Model

Extensions use a standard endpoint contract rather than ad hoc route
registration.

Each endpoint declares:

- endpoint class such as `public-page`, `public-asset`, `public-ingest`, `webhook`, `admin-page`, `admin-action`, `extension-api`, or `health`
- mount path
- auth mode
- allowed methods
- content type policy
- body-size and rate-limit policy
- workspace binding mode
- service target for service-backed runtimes

Service-backed extensions declare at least one internal `health` endpoint so
activation and monitoring use the same contract.

Core owns the external routers, auth, rate limiting, tracing, and proxy
boundaries. Extensions declare endpoints and core mounts them into approved
path families.

This is especially important for:

- analytics capture endpoints
- tracking script routes
- Sentry-compatible ingest endpoints
- admin pages and admin mutation actions
- local-agent bridge endpoints

See [docs/EXTENSION_ENDPOINT_MODEL.md](https://github.com/movebigrocks/platform/blob/main/docs/EXTENSION_ENDPOINT_MODEL.md) for the detailed contract.

## Event and Command Integration

Extensions use the same outbox and event-bus discipline as core.

They are able to:

- publish typed extension-owned events
- subscribe to stable core events
- publish typed command events when requesting core actions such as case creation or email sending
- register event consumers and scheduled jobs in the service-backed runtime

Extension event types are registered into the live event catalog while the
extension is installed. When an extension is removed, those event types leave
the live selectable set but remain archived for history, audit, replay, and
debugging.

## CLI Extensibility

The core `mbr` CLI stays generic. Extension-specific verbs do not get
compiled into core on a per-pack basis.

Instead, an installed extension may declare:

- a namespaced command catalog such as `ats.jobs.publish`
- bundled agent-skill assets, typically Markdown
- machine-readable task schemas or adapters when appropriate

The CLI supports:

- listing installed extensions
- showing an extension's declared commands and skills
- listing skills for a specific installed extension
- reading the content of a specific declared skill asset

The intended operator and agent flow is:

1. discover installed extensions through the generic CLI
2. inspect an extension's declared commands and skill names
3. read the relevant skill asset
4. execute the needed workflow using generic core commands, extension endpoints, extension APIs, or GraphQL

This keeps the CLI stable while letting extensions shape higher-level
workflows.

## Repository and Distribution Model

The extension system assumes a three-layer repo model:

- public core repo for the shared runtime
- instance template repo for customer deployment repos
- extension SDK/template repo for authoring

First-party commercial extensions and customer-built private extensions both
ship against that same runtime contract.

The deployment repo for a customer instance is not a fork of core. It stores
desired state, deploy workflows, and installed extension refs for one
installation.

## Bundle Transport

An extension bundle is the authored package. Move Big Rocks supports local
directories, signed bundle files, HTTPS bundle URLs, OCI bundle refs, and
marketplace aliases for install and upgrade.

Supported acquisition paths:

- local bundle file
- local source directory
- signed OCI artifact
- marketplace alias resolved to a licensed OCI artifact or signed download

Service-backed execution uses the supervised service-target runtime and the same
bundle contract.

## ATS Reference Extension

The ATS package provides:

- `job_posting`
- `candidate`
- `candidate_case`
- careers site public routes
- application form
- recruiter workflow command declarations and agent skills

Each job owns a collection. Each submission creates a case in that collection.

ATS is the reference bundle-first extension because it heavily reuses core
forms, cases, contacts, attachments, and route hosting.

The ATS flow is:

- careers site served through extension public routes
- forms owned by the ATS workspace
- each job mapped to one collection
- each submission creating a case in that collection
- labels used as flexible metadata
- pipeline stage stored as extension-owned state rather than as a core case status

## First-Party Packs

This RFC establishes these first-party packs:

- `ats` as a product extension
- `enterprise-access` as an identity extension
- `error-tracking` as a product extension
- `web-analytics` as a product extension
- `operational-health` as an operational extension

Slack alerting, WhatsApp, email transport providers, and OpenClaw-style local
agent bridges fit the connector class and use the same lifecycle model.

## UI and Integration Requirements

The extension system includes:

- admin navigation registration
- admin page mounting inside the shared shell
- dashboard and widget registration
- extension event consumers and scheduled jobs
- capability interfaces for support, automation, connector, and operational integrations
