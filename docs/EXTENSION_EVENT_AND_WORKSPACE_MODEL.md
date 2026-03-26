# Extension Event and Workspace Model

This document defines how extensions should integrate with Move Big Rocks's existing outbox, event bus, workspace model, forms, cases, collections, and tags.

The goal is to preserve the strengths Move Big Rocks already has:

- typed events
- outbox-backed durability
- shared support primitives
- predictable multi-workspace behavior

Extensions should use those mechanisms, not bypass them.

## Core Rule

Extensions should not invent a second integration system.

They should integrate through:

- the same outbox pattern
- the same event bus
- the same typed event discipline
- the same case, form, attachment, and automation primitives

This matters because the free core product is those primitives. Extensions should amplify them, not replace them.

## Event Model

## What Is Good Today

Move Big Rocks already has:

- a durable outbox publisher in [`internal/infrastructure/outbox/outbox.go`](https://github.com/movebigrocks/platform/blob/main/internal/infrastructure/outbox/outbox.go)
- type-safe event contracts in [`pkg/eventbus/types.go`](https://github.com/movebigrocks/platform/blob/main/pkg/eventbus/types.go)
- typed shared events in [`internal/shared/domain/events_case.go`](https://github.com/movebigrocks/platform/blob/main/internal/shared/domain/events_case.go)
- typed command events in [`internal/shared/events/commands.go`](https://github.com/movebigrocks/platform/blob/main/internal/shared/events/commands.go)
- typed form submission events in [`internal/shared/contracts/services.go`](https://github.com/movebigrocks/platform/blob/main/internal/shared/contracts/services.go)

That is the right base.

## What Needs To Change for Extensions

Today, external packages cannot cleanly define their own new typed `EventType` values because `eventbus.EventType` is intentionally closed.

That is good for safety, but it means the extension platform needs a sanctioned registration path.

Milestone 1 should add:

- an extension event type registration API in the extension SDK
- namespaced extension event types
- a versioned contract for extension-owned events
- an extension event catalog in core

Current implementation status:

- extension manifests can now declare published and subscribed event types
- those declarations are visible through the installed-extension metadata
- workspace runtime event catalog and activation-time validation against available core and active-extension events now exist
- live subscription dispatch and extension-owned event consumers are still pending

Recommended naming format:

- `ext.<publisher>.<extension>.<event>`

Examples:

- `ext.demandops.ats.job_posting_published`
- `ext.demandops.ats.application_received`
- `ext.demandops.web_analytics.property_created`

This keeps marketplace events distinct from core events such as `case.created` and `form.submitted`.

## Event Catalog Lifecycle

When an extension is installed, its declared event types should be registered in a global event catalog.

That gives the overall system awareness of:

- which extension events exist
- which version each event schema uses
- which extensions publish them
- which extensions or rules subscribe to them

The catalog should drive:

- automation rule builders
- CLI discovery
- extension validation
- admin observability and debugging

## What Happens on Uninstall

An extension's events should be removed from the **live available event set**, but not erased from history.

That means:

- they no longer appear as active choices for new subscriptions or new rules
- the extension can no longer emit them
- the extension can no longer receive new deliveries for them
- existing event records remain queryable and auditable
- old event schemas remain archived for replay, debugging, and migration purposes

So the right lifecycle is:

- `active`
- `disabled`
- `archived`

Not:

- active
- deleted as if it never existed

Deleting event definitions entirely would break:

- audit history
- past automation logs
- replay and data import workflows
- incident debugging for removed extensions

## Publish/Subscribe Rules

Extensions should be able to:

- publish typed extension-owned events through the outbox
- subscribe to core event streams
- subscribe to their own extension event streams
- emit command events to request work from core

Core automation should also be able to subscribe to both core and extension events and trigger configured actions when those events occur.

Subscriptions should be validated against the event catalog so extensions cannot subscribe to undeclared or unavailable event types accidentally.

The current operator surface for this is:

- `mbr extensions events list --workspace WORKSPACE_ID`

That command lets an agent or human inspect the currently available runtime event catalog before activating or troubleshooting a pack.

Core should expose stable subscription points for:

- `case.created`
- `case.status_changed`
- `form.submitted`
- relevant automation and job lifecycle events

Extensions should not directly mutate core internals when a typed event or service contract exists.

## Command Pattern

When an extension wants core to do something like:

- create a case
- send an email
- add a notification

it should use the same command/event patterns that core already uses.

Examples:

- [`CreateCaseRequestedEvent`](https://github.com/movebigrocks/platform/blob/main/internal/shared/events/commands.go)
- [`SendEmailRequestedEvent`](https://github.com/movebigrocks/platform/blob/main/internal/shared/events/commands.go)

For service-backed extensions, direct in-process service contracts may still exist internally, but the architectural default should be:

- write domain data
- publish typed event
- let subscribed consumers react

The same pattern should support event-driven callouts such as:

- notify an agent
- send a webhook
- send an email
- dispatch to a connector extension such as Slack

## Workspace Model

Extensions should be able to declare workspace needs explicitly.

Recommended install-time modes:

- `install_into_existing_workspace`
- `provision_dedicated_workspace`

This should be an explicit extension installation choice, not a hidden assumption.

Current implementation status:

- the manifest now supports workspace-plan metadata for those install modes
- runtime workspace creation is still pending, so agents or operators still create the workspace before installation today

## ATS Recommendation

For the ATS extension:

- support a **dedicated ATS workspace** as the recommended default
- also allow installation into an existing workspace if a customer explicitly wants shared operational context there

Why:

- a dedicated ATS workspace gives cleaner separation for recruiting operations
- shared installation is still useful for some small teams

The extension installer should be able to:

- create the workspace if it does not exist
- seed the required forms
- seed the required collections
- seed the required automation rules
- register the required endpoints and routes
- apply the required workspace view and filter configuration

## ATS Data Model

The ATS extension should reuse core primitives as much as possible.

Recommended mapping:

- ATS workspace: dedicated or chosen workspace
- careers site: extension public route
- job posting: extension-owned entity
- candidate: existing contact
- application submission: existing form submission
- candidate case: existing case

## Collections Versus Labels

Do **not** replace collections with labels entirely.

The right model is:

- `collection` = primary container or bucket
- `tags` = flexible multi-label classification

Move Big Rocks already has both in the case model:

- `CollectionID`
- `Tags`

Relevant code:

- [`internal/service/domain/case.go`](https://github.com/movebigrocks/platform/blob/main/internal/service/domain/case.go)
- [`internal/service/domain/collection.go`](https://github.com/movebigrocks/platform/blob/main/internal/service/domain/collection.go)

For ATS:

- each job posting should own one collection
- each candidate case belongs to that job collection
- tags act as flexible labels for additional filtering

Examples of ATS labels:

- `referral`
- `remote`
- `graduate`
- `priority`
- `internal_candidate`

This gives:

- one primary job bucket
- many optional filter labels

That is better than labels alone.

## Pipeline Stage

Do not model ATS pipeline stage as just a free-form tag.

Pipeline stage is:

- single-valued
- ordered
- workflow-sensitive

Recommended model:

- keep coarse case status in core
- store ATS pipeline stage as extension-owned state or typed custom field

Examples:

- `applied`
- `screening`
- `interview`
- `offer`
- `hired`
- `rejected`

Tags remain available for non-exclusive labels.

## Forms and Case Creation

The ATS extension should reuse the base form system.

Recommended flow:

1. ATS publishes a careers page through extension public routes.
2. Each job page renders a form.
3. The form belongs to the ATS workspace.
4. Form submission is accepted by core.
5. Core emits `form.submitted`.
6. Core form handling creates the candidate case in the ATS workspace.
7. Core emits `case.created` for that candidate case.
8. ATS subscribes to the resulting case event for follow-up automation such as:
   - the job's collection
   - ATS-specific tags
   - ATS-specific custom fields

This preserves the base form capabilities while letting ATS own the recruiting-specific logic.

## Workspace Views and Filters

Extensions should be able to configure the workspace experience they need.

Milestone 1 should support extension-owned workspace view configuration such as:

- default collection filters
- default tag or label filters
- visible saved filters
- extension-specific list presets
- extension-specific status or stage views

For ATS, that means the workspace should be able to present views like:

- all candidates
- candidates for one job
- screening
- interview
- rejected
- hired

Core case pages should remain usable even without those presets, but the extension should be able to improve the workspace UX.

## Automation and Callouts

Forms, cases, workspaces, labels, automation, identity, and the outbox plus event-bus integration architecture are part of the free core product.

That means Move Big Rocks core should be able to:

- observe core events
- observe extension events
- evaluate automation rules
- trigger callout actions

Connector extensions should provide optional delivery targets such as:

- Slack
- WhatsApp
- external agent endpoints
- other third-party messaging systems

But the event-driven automation engine that decides **when** to call out belongs in core.

## Careers Site Model

The ATS extension should own:

- public careers landing page
- job detail pages
- branding-aware HTML templates
- form placement and rendering

Core should own:

- hosting shell
- route mounting
- form processing
- attachment form
- auth and tenancy

This keeps the careers site lightweight while preserving Move Big Rocks's operational primitives.

## Marketplace Implications

If third parties are later going to sell extensions, they need the same event and workspace model:

- typed extension-owned events
- stable core event subscriptions
- workspace provisioning choices
- collection plus tags support
- workspace view configuration

Without those, first-party packs may work but third-party packs will have to rely on internal assumptions.

## Bottom Line

The existing Move Big Rocks architecture is already close to what the ATS extension needs.

The right path is:

- reuse forms
- reuse cases
- reuse contacts
- keep collections as the primary grouping primitive
- use tags as flexible labels
- let extensions publish and subscribe through the same outbox and event bus
- add a sanctioned extension event registration mechanism
- add extension-controlled workspace view and filter configuration

That gives Move Big Rocks a consistent extension architecture for first-party packs, customer-built extensions, and later marketplace packs.
