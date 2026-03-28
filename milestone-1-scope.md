# Milestone 1 Scope

**Document version:** 2026-03-28
**Status:** Active source of truth

## Milestone 1 In One Sentence

Milestone 1 ships the full first marketable shape of Move Big Rocks: a
self-hosted, agent-operable operations core, a shared CLI and GraphQL contract,
an installable extension runtime, four core first-party packs, two in-scope
beta first-party packs, and a self-hosted evaluation path that proves the model
end to end. Hosted sandboxes are deferred to
[`docs/RFCs/RFC-0013-hosted-sandbox-control-plane.md`](docs/RFCs/RFC-0013-hosted-sandbox-control-plane.md).

## Problem Space

Move Big Rocks exists because organisations are now dealing with one combined
failure mode, not several separate ones.

Operational work is fragmented across specialised SaaS tools and internal glue.
The important context is fragmented too: vision, mission, goals, strategy,
plans, prompts, runbooks, and working knowledge are spread across decks, docs,
tickets, chats, and local folders. Those concepts are often not load-bearing
enough for humans to reason from consistently, let alone for agents to use
safely.

That problem existed before agentic work, but agentic work makes it more
urgent. Claude Code, Codex, and similar agents are becoming part of how teams
actually work. Agentic flows are becoming normal. More people can build useful
software and automation quickly, but they still need somewhere coherent to
deploy, operate, govern, and share that work.

If the organisation has no shared operational and strategic context model, the
result is not less overhead but more: more side channels, more local prompt
systems, more Slack traffic, more meetings, more alignment theatre, and even
more busy work and SaaS sprawl layered on top of the old mess.

Companies that want durable value from AI therefore need a new operating system
for work, not just more models or more disconnected AI tooling.

Milestone 1 exists to prove that Move Big Rocks can be the operational core
that replaces that fragmentation with one deliberate system for work,
knowledge, strategic context, extensions, and agent activity.

## Why This Milestone Exists

Milestone 1 is not just a feature bucket. It is the first complete proof that
the product thesis works as one coherent system:

- the free core must already be useful for real multi-team operational work,
  including context engineering through concept spec libraries, forms, and
  knowledge resources that teams can share across workspaces
- humans and agents must be able to operate the same system through the same
  contract
- extensions must be the real delivery mechanism for product depth and the
  commercial layer
- first-party packs must prove that the extension model is credible in real
  product categories
- evaluation must stay fast without weakening the self-hosted production thesis

## Definition Of Done

Milestone 1 is complete only when all of the following are true:

- the free core is valuable on its own for team-aware operational work with
  forms, queues, conversations, cases, knowledge, automation, and audit
- the operator and agent surface is real through `mbr`, GraphQL, admin, and the
  public bootstrap/docs path
- the core service-desk loop is operator-complete across the supported product
  surfaces: manual case creation, inbound email intake, public form intake,
  public conversation intake, queue placement, assign, priority, internal note,
  reply, handoff, status transitions, escalation from conversation to case, and
  attachment handling are all available and workflow-proven
- installable extensions are a real runtime model with activation, validation,
  lifecycle operations, artifact surfaces, permissions, and event contracts
- extensions and agents can request in-scope core case actions through a real
  sanctioned core-action contract; placeholder command streams with no
  production consumer do not satisfy this bar
- the core first-party extension set is real: ATS, enterprise access, error
  tracking, and web analytics all ship on that runtime model
- the in-scope beta first-party extension set is real: sales-pipeline and
  community-feature-requests ship on that runtime model with clear beta
  labeling, install and activation proof, and public bundle publication
  evidence
- the release, migration, and verification story is strong enough that the
  milestone is actually shippable rather than merely described, including
  milestone-proof evidence for CLI release artifacts, first-party bundle
  validation, and public bundle publication inputs

Nothing in that definition is optional. Milestone 1 is the whole chain working
together, not a partial platform slice.

## How To Read This Document

- `Outcome` defines the full Milestone 1 deliverable set
- `Core Scope` and `Queue, Conversation, and Case Model` define the base product
  behavior that everything else depends on
- `Repository and Delivery Model` and `Agent Surface` define how operators and
  agents get to first value and then run the product
- `Extension Model`, `First-Party Bundle Flow`, and the first-party pack sections
  define how Move Big Rocks delivers product depth without bloating core

## Current Proof Synthesis

The living readiness and evidence matrix for this milestone is tracked in
[`docs/MILESTONE_1_READINESS.md`](docs/MILESTONE_1_READINESS.md). The runnable
launch-proof flow is tracked in [`docs/MILESTONE_1_PROOF.md`](docs/MILESTONE_1_PROOF.md).
Keep this scope document as the target and use the readiness and proof docs to
record what is currently proven, how to rerun it, and where external release
confirmation still happens.

## Outcome

Milestone 1 delivers Move Big Rocks as a self-hosted operations center with the
full stack below in one coherent release:

- a free core operations product for multi-team work
- a CLI-first agent model with workspace and team context
- a shared GraphQL API
- installable extensions
- support for team-authored private extensions
- a first-party ATS extension with a careers site
- an enterprise access extension for SSO
- a Sentry-compatible error tracking extension
- a privacy-first web analytics extension
- a beta sales pipeline extension
- a beta community feature requests extension
- a signed-bundle activation flow for first-party and custom extensions

The goal is not to ship every product category inside core. The goal is to ship
a strong base that makes focused extensions easy to install, operate, and
automate.

That breadth is intentional. Milestone 1 is proving the whole product shape,
not only the core runtime in isolation.

The public distribution model is:

- free self-hosted core
- free public first-party standard-risk bundles for ATS, error tracking, and
  web analytics, plus beta public bundles for sales-pipeline and
  community-feature-requests
- separately controlled privileged first-party packs such as enterprise access
- internal company use, modification, and private extensions allowed under the
  public license, but no selling or licensing of the platform, derivative
  works of it, extensions, or access to them, and no hosted-service offering,
  without separate written permission from Move Big Rocks BV

The four core packs remain the launch-grade baseline. The two beta packs are
intentionally in scope for Milestone 1 as public beta products: present,
installable, documented, and published through the same bundle path, but
explicitly labeled beta and allowed a narrower workflow-depth bar than the four
core launch packs.

That base is already the initial product. The free core is valuable on its own
because it provides:

- workspaces
- teams and team membership
- service catalog nodes and topics
- queues
- queue items
- conversations
- cases
- case labels and categorization
- versioned concept specs for structured knowledge
- concept spec libraries that group related specs by team type or domain
- Markdown knowledge resources with explicit in-workspace audience control
- team-to-team sharing of approved knowledge
- cross-workspace sharing of concept spec libraries and form specs
- concept-spec-defined audience defaults and visibility ceilings for structured knowledge
- built-in Strategic Context Stack concepts such as purpose, vision, mission, goal, strategy, bet, OKR, KPI, milestone goal, and workstream
- typed Markdown references plus canonical structured relations for concept instances
- structured forms (form specs and submissions) for capturing what must be
  known before work is accepted, routed, or acted on, usable internally by one
  team or shared across teams and workspaces
- attachments
- replies, form-driven request capture, and escalation flows
- automation
- identity and agent access
- outbox and event-bus integration
- hosting and deployment conventions

## Core Scope

Core Move Big Rocks provides these primitives and platform services:

- workspaces as the logical and security boundary inside an instance
- teams as the operational ownership boundary inside a workspace
- users, sessions, roles, memberships, and team memberships
- magic-link login and break-glass operator access
- agents and hashed `hat_*` tokens
- contacts
- service catalog nodes and operational topics
- queues
- queue items
- conversations and transcripts
- cases
- case tags as flexible labels
- versioned concept specs that define structured team and extension knowledge
- Markdown knowledge resources with explicit owner-team, shared-team, and workspace audience modes
- team-aware knowledge sharing, review state, and publication flows
- knowledge resources stored as Markdown concept instances pinned to an exact concept spec version
- concept specs that define default and allowed in-workspace visibility for their instances
- knowledge instances that store actual audience, share targets, and minimum owner-team role for access
- concept spec libraries that group related specs by team type or domain and can be shared across workspaces
- instance-global concept spec libraries that can be registered once and used across workspaces
- workspace and team concept instances for strategic context such as goals, strategies, bets, OKRs, and KPIs
- workspace and team concept instances for delivery context such as milestone goals and workstreams
- typed Markdown references for concepts, teams, queues, catalog nodes, and related work, with canonical relations stored in frontmatter and structured metadata
- service catalog bindings between topics, teams, queues, knowledge, and forms
- form specs (internally `FormSpec`) that define what must be known before a
  request is accepted, routed, or acted on, usable internally by one team or
  shared across teams and workspaces
- form submissions (internally `PublicFormSubmission`) as collected instances of
  those specs
- attachments
- automation rules and jobs
- public route hosting
- GraphQL
- admin panel
- audit trails and internal health instrumentation
- outbox-driven event delivery
- event-triggered automation and callouts
- modular email delivery and inbound routing interfaces
- Git-compatible version history and sync workflows for Markdown knowledge artifacts
- Git-backed version history for concept spec definitions and their Markdown instances
- core-managed Git-backed artifact surfaces for publishable extension content
- optional ACL-filtered local checkout and sync flows for knowledge and concept artifacts

## Queue, Conversation, and Case Model

Milestone 1 treats queues as the operational inbox for both conversations and
cases.

The minimum model is:

- a queue contains `queue_item` records
- a queue item points to either a `conversation_session` or a `case`
- a conversation can be queued, handled, and resolved without becoming a case
- a case can be created from a conversation when durable follow-through is needed
- a case can also be created from form submissions, alerts, or internal requests without an originating conversation
- a case should retain a link to its source conversation when one exists
- a case may later link additional conversations without becoming identical to them
- a case should still own a canonical work thread so durable work remains conversational
- the case-owned work thread is the durable thread for work, decisions,
  approvals, summaries, and replies; linked conversations remain related source
  or follow-up interaction context rather than replacing the case
- handoff and escalation should act on the case or conversation, with `queue_item` following as a consequence
- agents should be able to hand off cases and conversations between teams
  through the CLI on behalf of their human when permissions and policy allow it
- delegated routing policy should be explicit on workspace membership
  constraints, not merely implied by generic write access

The operator experience should still feel unified:

- one queue view can show both queued conversations and queued cases
- one thread-first timeline can show the source conversation alongside the case record
- the case timeline should be the durable conversational surface for work, decisions, approvals, and follow-up
- escalation from conversation to case preserves provenance and audit history
- delegated cross-team routing remains visible and auditable in that thread
- operators and agents should be able to execute the normal service-desk loop
  without dropping into unsupported surfaces: reply, handoff, escalate, assign,
  reprioritize, annotate, and attach files should all work through the approved
  CLI, API, and admin paths

## Product-Complete Operational Loop

Milestone 1 is not complete merely because the primitives exist. It is complete
when one real operational loop is mature enough to replace a fragmented support
or intake stack for a small team.

The minimum complete loop is:

- intake from inbound email
- intake from public forms
- intake from a supported public conversation surface owned by core, such as a
  website widget, app chat surface, or equivalent public conversation adapter
- manual operator case creation without bypassing the approved product surfaces
- queue visibility for both conversations and cases
- conversation reply, handoff, and escalation with preserved provenance
- case create, assign, unassign, set priority, add internal note, reply,
  handoff, set status, and attachment handling
- durable linkage across the resulting conversation, case, queue item,
  attachment, and notification records

That loop must be:

- available through supported product surfaces rather than only through service
  methods
- usable by humans and agents without hidden setup steps
- represented in the workflow-proof matrix and milestone proof bundle
- strong enough that the public service-desk and shared-operations claims remain
  honest

For Milestone 1, the supported core-owned public conversation surface is the
web-chat intake API under `/v1/conversations` and
`/v1/conversations/:session_id/messages`, which is designed to sit behind the
website widget or equivalent public chat adapter while keeping the
conversation, queue, and escalation records inside core Move Big Rocks.

## Repository and Delivery Model

Milestone 1 standardizes on this delivery shape:

- a public core repo for the shared source and releases
- a public instance template repo for deployment bootstrapping
- a public extension SDK/template repo
- one public first-party extension monorepo at the start
- private customer instance repos created from the template
- private customer extension repos by default
- a public docs and bootstrap surface for humans and agents

The private customer instance repo is the deployment control plane for one
installation. It stores pinned versions, deployment workflows, extension
config, branding overrides, and operational policy. It is not a long-lived fork
of the core repo.

The default production path is:

1. Create a private instance repo from the public template.
2. Hand that repo to an operator or agent.
3. Deploy pinned Move Big Rocks releases to a Linux host.
4. Install signed extension bundles into the running instance.

Customer-facing install docs make this path obvious and agent-friendly.

Hosted sandbox evaluation is preserved in
[`docs/RFCs/RFC-0013-hosted-sandbox-control-plane.md`](docs/RFCs/RFC-0013-hosted-sandbox-control-plane.md)
for possible future resurrection. It is not part of Milestone 1.

Milestone 1 should also expose:

- a runtime bootstrap endpoint on each runtime such as `/.well-known/mbr-instance.json`
- a stable CLI and GraphQL contract that agents can use once a runtime is known

Milestone 1 should make it clear that Move Big Rocks can replace meaningful
slices of Confluence and Jira for operations-heavy teams. The intended scope is
not every PM feature. The intended scope is one system where strategic context,
knowledge, routing, forms, queues, conversations, and cases stay connected
and remain usable by both humans and agents.

Milestone 1 should also make the execution model legible:

- The Strategic Context Stack sets the governing frame
- milestone goals and workstreams act as the delivery layer beneath that stack
- the milestone scope document is the milestone artefact that stabilises the target
- agent-assisted proof synthesis should reconcile what is proven, open, partially evidenced, and blocked

## Agent Surface

Milestone 1 ships a Go-based `mbr` CLI for macOS, Linux, and Windows.

Minimum requirements:

- signed binaries and checksums
- browser-based login for human operators
- OS credential-store-backed credential persistence where available, with secure fallback otherwise
- current workspace and team context selection
- workspace-scoped agent tokens for automation
- noun-led CLI ergonomics inspired by `gh`
- strict exit codes
- `--json` support on every command
- GraphQL-backed reads and writes
- machine-readable bootstrap discovery so an agent can install the CLI and discover the right next steps before any runtime is selected
- file upload support for attachments and resumes
- Markdown knowledge sync and publish workflows
- optional local checkout, pull, push, and status workflows for concept-aware Markdown knowledge
- artifact history, diff, and publish workflows for core and extension-managed content surfaces
- team-aware knowledge authoring, review, publish, and share workflows
- service catalog and form submission workflows that are usable by both humans and agents
- concept-aware authoring workflows that let agents scaffold, validate, and relate strategic-context Markdown instead of treating it as unstructured notes
- delivery-aware concept workflows that let teams coordinate around milestone goals and workstreams without requiring a backlog full of tiny tickets

Minimum command surface:

- `mbr auth login`
- `mbr auth whoami`
- `mbr context view`
- `mbr context set`
- `mbr teams list`
- `mbr teams show`
- `mbr teams create`
- `mbr teams members list`
- `mbr queues list`
- `mbr queues show`
- `mbr queues items`
- `mbr queues create`
- `mbr conversations list`
- `mbr conversations show`
- `mbr conversations reply`
- `mbr conversations handoff`
- `mbr conversations escalate`
- `mbr cases list`
- `mbr cases show`
- `mbr cases create`
- `mbr cases assign`
- `mbr cases unassign`
- `mbr cases set-priority`
- `mbr cases add-note`
- `mbr cases reply`
- `mbr cases handoff`
- `mbr cases set-status`
- `mbr catalog list`
- `mbr catalog show`
- `mbr artifacts list`
- `mbr artifacts show`
- `mbr artifacts history`
- `mbr artifacts diff`
- `mbr artifacts publish`
- `mbr knowledge list`
- `mbr knowledge search`
- `mbr knowledge show`
- `mbr knowledge upsert`
- `mbr knowledge sync`
- `mbr knowledge checkout`
- `mbr knowledge pull`
- `mbr knowledge push`
- `mbr knowledge status`
- `mbr knowledge review`
- `mbr knowledge publish`
- `mbr knowledge share`
- `mbr concepts list`
- `mbr concepts show`
- `mbr concepts register`
- `mbr forms specs list`
- `mbr forms specs show`
- `mbr forms submissions list`
- `mbr forms submissions show`
- `mbr contacts list`
- `mbr extensions list`
- `mbr extensions install`
- `mbr extensions activate`
- `mbr extensions deactivate`
- `mbr extensions upgrade`
- `mbr extensions validate`
- `mbr extensions configure`
- `mbr extensions monitor`
- extension-declared command catalogs and agent-skill listing and retrieval for agent workflows

The supported baseline includes:

- `mbr auth login` with browser login for humans, token bootstrap for agents, and persisted local CLI config
- `mbr auth logout`
- `mbr auth whoami`
- `mbr context view`
- `mbr context set`
- `mbr workspaces list`
- `mbr workspaces create`
- `mbr teams list`
- `mbr teams show`
- `mbr teams create`
- `mbr teams members list`
- `mbr queues list`
- `mbr queues show`
- `mbr queues items`
- `mbr queues create`
- `mbr conversations list`
- `mbr conversations show`
- `mbr conversations reply`
- `mbr conversations handoff`
- `mbr conversations escalate`
- `mbr health check`
- `mbr cases list`
- `mbr cases show`
- `mbr cases create`
- `mbr cases assign`
- `mbr cases unassign`
- `mbr cases set-priority`
- `mbr cases add-note`
- `mbr cases reply`
- `mbr cases handoff`
- `mbr cases set-status`
- `mbr catalog list`
- `mbr catalog show`
- `mbr artifacts list`
- `mbr artifacts show`
- `mbr artifacts history`
- `mbr artifacts diff`
- `mbr artifacts publish`
- `mbr knowledge list`
- `mbr knowledge search`
- `mbr knowledge show`
- `mbr knowledge upsert`
- `mbr knowledge sync`
- `mbr knowledge checkout`
- `mbr knowledge pull`
- `mbr knowledge push`
- `mbr knowledge status`
- `mbr knowledge review`
- `mbr knowledge publish`
- `mbr knowledge share`
- `mbr concepts list`
- `mbr concepts show`
- `mbr concepts register`
- `mbr forms specs list`
- `mbr forms specs show`
- `mbr forms submissions list`
- `mbr forms submissions show`
- `mbr contacts list`
- `mbr attachments upload`
- `mbr extensions list`
- `mbr extensions show`
- `mbr extensions monitor` with runtime health refresh
- `mbr extensions install` from a local bundle file, local extension directory, HTTPS bundle URL, OCI ref, or marketplace alias when a private catalog is later used, with dedicated-workspace provisioning when the manifest declares it and the operator is using browser/session auth
- `mbr extensions upgrade` from a local bundle file, local extension directory, HTTPS bundle URL, OCI ref, or marketplace alias when a private catalog is later used
- optional server-side signed bundle enforcement via `INSTANCE_ID`, `EXTENSION_TRUST_REQUIRE_VERIFICATION`, and trusted publisher keys
- `mbr extensions configure`
- `mbr extensions validate`
- `mbr extensions activate`
- `mbr extensions deactivate`

## Extension Model

An extension is a signed bundle with a manifest, migrations, assets, CLI
registrations, and lifecycle hooks.

Every extension declares:

- `kind`: `product`, `identity`, `connector`, or `operational`
- `scope`: `workspace` or `instance`
- `risk`: `standard` or `privileged`
- extension-owned schema migrations for its `ext_*` PostgreSQL schema when it stores structured state
- any core-managed artifact surfaces it uses for versioned Markdown, templates, or published site content

Milestone 1 supports:

- install
- activate in a workspace or instance as declared
- configure
- upgrade
- deactivate
- uninstall

Each extension can declare:

- requested permissions against core primitives
- extension-owned entities
- extension-owned PostgreSQL schema state and migrations
- workspace provisioning requirements or defaults
- default queues
- knowledge resources
- form specs
- automation rules
- Git-backed versioned artifact surfaces for websites, templates, prompts, or published docs
- public routes
- admin routes
- standard endpoint declarations for pages, ingest routes, webhooks, admin actions, and extension APIs
- admin navigation items
- dashboard or stats widgets
- scheduled jobs
- published event types
- subscribed core event types
- CLI commands

Extensions run out of process. Core Move Big Rocks owns authentication, routing,
isolation, and event delivery.

Core also owns the versioned artifact service. Extensions can consume it, but
they should not invent their own storage, review, or publication trust model
when they need versioned Markdown or publishable content.

Knowledge visibility in Milestone 1 is explicitly about in-workspace access,
not internet publication. The minimum model is:

- a concept spec can define the default and maximum audience for its instances
- a knowledge instance can be visible to its owner team, named peer teams, or the whole workspace
- owner-team access can optionally require a minimum role such as `member` or `lead`
- wider visibility changes must be validated by core and audited
- review queues and notifications must respect the same audience rules so human teammates and their agents only see RFCs and similar items they are allowed to access

Local sync in Milestone 1 is also explicit:

- Move Big Rocks remains the server-side source of truth
- a visible local working copy is optional and initiated through `mbr`
- local checkout must be filtered by the actor's current access rights
- local edits must flow back through validation, ACL checks, and audit before they become accepted revisions

Hosted sandbox evaluation is preserved in
[`docs/RFCs/RFC-0013-hosted-sandbox-control-plane.md`](docs/RFCs/RFC-0013-hosted-sandbox-control-plane.md)
for future consideration. It is not part of Milestone 1.

That means an extension can:

- attach to an existing workspace
- provision a dedicated workspace when the install flow allows it
- seed team-owned knowledge, forms, queues, automation rules, routes, and UI defaults
- persist structured extension state in its own PostgreSQL schema
- contribute versioned concept specs for structured Markdown artifacts
- store versioned content in core-managed Git-backed artifact surfaces
- expose approved endpoints through the standard endpoint model
- publish approved artifact-backed sites and pages through declared endpoint mounts

Teams can author their own extensions, but extension runtime scope remains
`workspace` or `instance`. Team authorship does not create a new trust boundary
inside the runtime model.

Extensions that contribute knowledge, forms, routing, or queue defaults should
participate in the same team-aware CLI and API surfaces rather than creating
parallel management paths.

Extensions and teams can also contribute versioned concept specs. That lets an
extension define its own structured Markdown artifacts, while the instances
still live in the same core knowledge system and remain usable by humans and
agents through the same CLI and GraphQL contract.

Milestone 1 supports two runtime classes:

- `bundle` extensions for bundle-first product packs such as ATS, where the
  extension owns its workflow vocabulary and published assets while still
  building on shared primitives
- `service-backed` extensions for dynamic packs such as web analytics, error
  tracking, enterprise access, sales-pipeline,
  community-feature-requests, and connector integrations

Extensions use the same outbox and event bus pattern as core. That means
Milestone 1 also includes:

- typed extension event registration
- stable core event subscriptions
- extension-owned event consumers
- typed command and event flows for requesting core actions

For Milestone 1, that sanctioned core-action contract must be real enough for
extension or agent initiated case workflows that matter to the product promise.
At minimum, case creation and follow-up actions used by in-scope product loops
must execute through a supported contract with a production consumer and proof.

Core automation reacts to both core and extension events and triggers outbound
actions or connector deliveries when those events occur.

Privileged extensions such as enterprise auth and external channel connectors
are first-party and explicitly reviewed.

Endpoint exposure is standardized:

- core owns the external routers and proxy boundaries
- extensions declare endpoint class, mount path, auth mode, body limits, rate limits, and workspace binding
- bundle extensions get simpler page and asset endpoints
- service-backed extensions add ingest endpoints, webhooks, admin actions, and richer extension APIs
- service-backed extensions declare at least one internal health endpoint so activation and monitoring can verify runtime health
- admin endpoints mount inside the shared shell rather than replacing it

See [docs/EXTENSION_ENDPOINT_MODEL.md](https://github.com/movebigrocks/platform/blob/main/docs/EXTENSION_ENDPOINT_MODEL.md).

## First-Party Bundle Flow

Move Big Rocks does not process payments or run a public marketplace inside
Milestone 1.

Milestone 1 does support a fully agentic extension activation flow:

1. The operator chooses a signed bundle from a local file, HTTPS URL, OCI ref,
   or another supported bundle source.
2. The CLI resolves or receives the bundle.
3. An operator or agent installs the bundle into a Move Big Rocks server.
4. The server validates the signature, digest, manifest, and any required
   bundle-install credential.
5. The extension is activated in the chosen workspace or instance.
6. Move Big Rocks provisions the declared routes, queues, data, and commands.
7. The extension is live without manual code deployment.

That same runtime supports both paths that matter for Milestone 1:

- a free public first-party bundle set for ATS, error tracking, and web
  analytics, plus beta public bundles for sales-pipeline and
  community-feature-requests
- customer-built private extensions on the same primitives, including
  team-authored private extensions

## Reference Extension: ATS + Careers Site

The first extension is an applicant tracking system with a simple careers
website.

Required model:

- `job_posting` belongs to a workspace
- each job posting owns one queue
- each applicant is a contact
- each submission creates a candidate case
- labels act as flexible metadata on candidate cases
- pipeline stage is extension-owned state, not a core case status
- ATS owns recruiting workflow state in its extension-local PostgreSQL schema
- the careers site is a public extension route backed by versioned extension artifacts
- each job page publishes its own application form
- resume and portfolio uploads attach to the case

The ATS installer supports:

- installation into an existing workspace
- or provisioning of a dedicated ATS workspace as the recommended default

Workspace filtering combines:

- queue as the primary job bucket
- labels as flexible metadata
- extension-owned saved filters and stage presets

Required workflows:

- create a job
- publish a job
- close a job
- reopen a job
- review candidates
- move a candidate through stages
- reject or hire a candidate
- add recruiter notes
- trigger automation on submission and stage changes

## Additional Initial Extensions

Milestone 1 defines three additional core first-party packs beyond ATS:

- `enterprise-access`
  OIDC, SAML, and directory sync. This is an identity extension. Core owns sessions, users, memberships, and authorization.
- `error-tracking`
  Sentry-compatible ingestion and issue workflows. This is a product extension built on shared primitives.
- `web-analytics`
  Cookie-free analytics for simple websites and extension-hosted sites. This is a product extension.

These packs use the same extension-aware admin navigation, route mounting,
event consumers, and service-backed runtime model.

## In-Scope Beta Packs

Milestone 1 also includes two public beta first-party packs on the same
runtime and bundle pipeline:

- `sales-pipeline`
  A workspace-scoped revenue-operations pack for opportunity intake, stage
  movement, dedicated sales workspace provisioning, and operator review.
- `community-feature-requests`
  A workspace-scoped community-feedback pack for public idea capture, voting,
  triage, and roadmap review.

Beta packs are in scope for Milestone 1 when they have:

- canonical checked-in source and public catalog entries
- install, validation, activation, and health proof on the shared runtime
- clear beta labeling in docs, bundle metadata, and publication guidance
- public bundle publication evidence in the same release pipeline as the other
  public first-party packs

## Modular Capability Packaging

Milestone 1 treats these as optional first-party extensions rather than
mandatory core product layers:

- ATS and careers site
- enterprise access
- error tracking
- analytics
- sales pipeline
- community feature requests
- operational health and probes

## Email and Channel Policy

Milestone 1 is opinionated but modular:

- recommend Postmark for outbound and inbound email
- support generic SMTP for outbound relay
- keep the provider layer adapter-based so SES and other vendors can be added without redesigning the support domain
- do not make Move Big Rocks itself a mail server

Slack alerting, WhatsApp, and similar integrations use the same extension
lifecycle, but they are connector extensions rather than core product features.

## Customer-Built Software

Milestone 1 also makes Move Big Rocks a safe host for agent-built operational apps.

Use this rule:

- if the software needs Move Big Rocks primitives, Move Big Rocks auth, or Move Big Rocks public routes, build it as an extension
- if it is a separate service, keep it in its own repo and integrate it with Move Big Rocks

The customer instance repo is the control plane for deploying and operating
those capabilities, but not the source-code home for every custom app.
