# Architecture

Move Big Rocks is a single Go service with a shared GraphQL API, a workspace-scoped
agent model, and a PostgreSQL-first storage architecture. The product surface
is intentionally split into a small free core and optional extensions.

## Core Principles

- Workspace is the logical and security boundary inside an instance.
- Team is the operational ownership boundary inside a workspace.
- GraphQL is the canonical machine API.
- The CLI is a thin client over GraphQL and a few purpose-built HTTP endpoints.
- Extensions run out of process and integrate through events, GraphQL, and routed HTTP surfaces.
- Customer-facing product extensions are optional unless they provide a cross-cutting primitive.
- The free core is the support and operations foundation; off-the-shelf and customer-built functionality sits in installable extensions and services.
- Conversations are live interaction surfaces; cases are durable work items.
- Knowledge is Markdown-first and designed for deliberate sharing between humans and agents.
- Concept specs define structured knowledge concepts and are versioned independently from their instances.
- Form specs define the structured information contract for work; forms and
  chat flows are renderers of that model.
- Queue items let one queue hold conversations and cases without collapsing those models together.
- Cases, conversation sessions, labels, queues, queue items, concept specs, knowledge resources, form specs, automation, identity, and event-driven integration are core primitives.

## Core Domains

- **Platform**: users, sessions, agents, workspaces, teams, memberships, team memberships, permissions
- **Service**: contacts, service catalog nodes, topics, queues, queue items, cases, conversation sessions, conversation messages, case labels, interactions, attachments, form specs, form submissions
- **Automation**: rules, triggers, jobs
- **Integration**: outbox, event bus, event-triggered callouts, routed public forms
- **Knowledge**: concept specs, knowledge resources, knowledge links, retrieval metadata
- **Governance**: audit, retention, safety, and security events

Queues are the user-facing operational bucket. Topics stay semantic: they
explain what something is about, while queues explain where it is worked.

The free core already gives a customer a useful operational base:

- workspaces for tenancy and isolation
- teams for ownership and collaboration
- service catalog nodes and topics for classification, routing, and self-service structure
- conversation sessions for customer chat and guided service flows
- queues for team-owned or shared work across conversations and cases
- forms that creates work
- cases that can be replied to and automated
- Markdown knowledge with explicit team, shared-team, and workspace visibility
- identity and agent access for humans and agents
- event-driven automation that can notify external systems when something happens

## Queue and Work Item Model

Queues are not just case lists. A queue contains `queue_item` records, and a
queue item points at either a conversation or a case.

The intended rules are:

- a conversation can enter a queue for triage, routing, and handling
- a conversation can resolve without becoming a case
- a case can be created from a conversation when durable ownership or follow-through is required
- a case can also originate from forms, alerts, or internal requests without an initial conversation
- a case links to its source conversation when one exists, and may later link additional conversations
- a case owns a canonical work thread, so durable work remains conversational even when it spans multiple linked sessions

This keeps the data model clean while still allowing one unified inbox and one
thread-first operator experience.

## Knowledge Surfaces

Markdown is the canonical knowledge format. Each team should be able to keep
private operating notes, publish approved guidance, and share selected
knowledge with other teams without duplicating content into separate tools.

The intended shape is:

- team-only knowledge for drafts, internal runbooks, prompts, and notes
- selectively shared knowledge for approved guidance other teams and agents can consume
- workspace-visible knowledge for common standards and cross-team policy

The access model should be separate from the raw Git path:

- `ConceptSpec` defines default and allowed audiences for instances, such as `team`, `shared_teams`, and `workspace`
- `KnowledgeResource` stores the actual audience, share targets, and any minimum owner-team role required to read the instance
- widening visibility must be an explicit write validated by core; it is not inferred from a filesystem move
- internet publication for extension websites and docs is a different concern from in-workspace knowledge visibility

Knowledge should be split into two layers:

- `ConceptSpec` stores the versioned definition of a structured concept such as `core/rfc`, `core/adr`, `marketing/campaign-brief`, or `ext/ats/interview-kit`
- `KnowledgeResource` stores the actual Markdown instance, its access policy, and pins to one exact concept spec version

That keeps structure explicit for humans and agents while preserving plain
Markdown authoring.

This model should also support **The Strategic Context Stack** without turning
Move Big Rocks into a generic planning suite:

- core concept libraries should include built-in definitions such as `core/purpose`, `core/vision`, `core/mission`, `core/goal`, `core/strategy`, `core/bet`, `core/okr`, `core/kpi`, `core/milestone-goal`, and `core/workstream`
- instance operators can register instance-global concept libraries and templates that workspaces may use
- actual concept instances remain scoped to one workspace and its teams
- teams can use those concepts to connect strategy and operating context directly to queues, service catalog nodes, form specs, knowledge, extensions, and work
- this is how Move Big Rocks can replace important slices of Confluence and Jira for operations-heavy teams without becoming a separate planning product

The Strategic Context Stack should stay explicit in the model:

- `purpose`, `vision`, `mission`, `goal`, `strategy`, `bet`, `okr`, and `kpi` are separate concept types with different horizons and responsibilities
- milestone goals and workstreams are the delivery layer below that stack
- the milestone artefact is the concrete release target that operationalises the stack for one delivery
- proof synthesis should reconcile milestone intent with current evidence rather than collapsing everything into ticket churn

For product and delivery teams, the same concept layer should also support
milestone goals and workstreams:

- milestone goals can define intended outcome, scope boundary, linked strategy, and success signals
- workstreams can group the related changes, concepts, extensions, queues, and work items needed to reach that outcome
- agents can reason over milestone goals plus linked concepts and work instead of depending on a large set of low-context tickets
- queue items and cases remain available when a piece of work needs explicit ownership, interruption handling, approval, or durable tracking
- delivery quality should be judged by drive versus drift and by the state of proof, not by visible busyness

The reference model should also be explicit:

- Markdown bodies can include typed references such as `@goal/...`, `@strategy/...`, `@bet/...`, `@okr/...`, `@kpi/...`, `@milestone/...`, `@workstream/...`, `@team/...`, `@queue/...`, and `@catalog/...`
- those inline references are convenience for humans and agents
- the canonical graph remains frontmatter and structured metadata, for example `goal_refs`, `strategy_refs`, `queue_refs`, and `catalog_refs`

Git-compatible version history is a good fit for these Markdown artifacts, but
the user-facing product model should remain draft, review, publish, share,
search, and cite.

The local working-copy model should be:

- server-side storage remains authoritative
- the CLI can materialize an explicit local checkout for permitted knowledge and concept artifacts
- that checkout is filtered by current ACL, not a full blind clone of everything in the workspace
- local edits go back through concept validation, permission checks, and audit before they become accepted revisions

The same core-managed Git-backed artifact capability should also be available
to extensions for websites, templates, prompts, and other content-bearing
surfaces that need reviewable history and controlled publication.

## Agent Interface

The machine interface has two layers:

1. **GraphQL**
   Shared schema for agents, admin workflows, and extensions.
2. **CLI**
   A stable operator and agent shell built in Go and distributed as signed
   binaries for macOS, Linux, and Windows.

Authentication stays simple:

- browser sessions for humans
- `hat_*` bearer tokens for automation and installed extensions
- explicit workspace and team context in CLI and agent flows
- OS credential-store-backed CLI credentials for interactive operator use, with secure fallback when unavailable

Core auth remains responsible for:

- magic-link login
- session issuance
- workspace membership and RBAC
- break-glass local operator access

Enterprise SSO belongs in a privileged identity extension, not in the core auth
baseline.

## Extension Runtime

Extensions declare a versioned manifest with:

- metadata and version
- kind, scope, and risk level
- requested capabilities
- workspace provisioning or attachment rules
- queues and seed data
- knowledge resources and form specs
- public routes
- admin routes
- scheduled jobs
- event consumers
- CLI command registrations
- admin navigation items
- dashboard widgets
- install, upgrade, activate, deactivate, and uninstall hooks
- published event types
- subscribed event types
- endpoint declarations for pages, ingest surfaces, APIs, and webhooks
- artifact surface declarations for versioned Markdown, templates, and published site content

Move Big Rocks treats extensions as one runtime with multiple classes:

- `product`: ATS, analytics, error tracking
- `identity`: enterprise access
- `connector`: Slack, WhatsApp, email transport adapters, and user-local agent bridges such as OpenClaw
- `operational`: probes, diagnostics, operational health

Identity and connector extensions need stricter review and stronger permission
boundaries than ordinary product extensions.

Move Big Rocks supports two runtime classes:

- `bundle` extensions for bundle-first product extensions such as ATS, where the
  extension owns product vocabulary, assets, and workflows while still running
  on shared primitives
- `service-backed` extensions for dynamic extensions such as analytics, error tracking, enterprise access, and connector extensions

The core service is responsible for:

- authenticating requests
- scoping all extension access to a workspace or instance as declared
- proxying routed traffic
- delivering events through the outbox and event bus
- storing extension installation state
- storing and governing the shared Git-backed artifact service used by core and extensions
- exposing sanctioned command surfaces so extensions can request core actions such as creating cases, sending replies, or updating contacts without bypassing core boundaries

Extensions are able to:

- publish typed extension-owned events through the same outbox and event-bus pattern as core
- subscribe to stable core events and extension events they are allowed to consume
- request core actions against shared primitives such as cases, forms, contacts, queues, and messages
- persist extension-owned operational state in their own `ext_*` PostgreSQL schemas
- use core-managed Git-backed artifact surfaces for versioned content and publishable pages
- attach to an existing workspace or provision a dedicated workspace when explicitly allowed by the install flow
- seed knowledge, forms, automations, queues, filters, and endpoint mounts as part of activation

Teams can author their own extensions. The authoring and operational ownership
can sit with one team, but extension installation scope remains `workspace` or
`instance` so the trust model stays consistent.

Core automation remains the place that decides when callouts happen. Connector
extensions provide delivery targets such as Slack, email transport, WhatsApp,
and local-agent bridges.

User-local agent connectors are a special connector subtype:

- they link one Move Big Rocks user to one local agent runtime
- they use outbound bridge connections instead of inbound public exposure
- they remain optional and revocable
- they never replace Move Big Rocks's core audit, approval, and write-enforcement model

Extension endpoints follow a standard catalog and routing model. Core owns the
public routers, auth, rate limits, tracing, and proxy boundaries; extensions
declare approved endpoint classes and core mounts them. See
[docs/EXTENSION_ENDPOINT_MODEL.md](https://github.com/movebigrocks/platform/blob/main/docs/EXTENSION_ENDPOINT_MODEL.md).

The storage split should stay strict:

- PostgreSQL is for structured operational state and workflow records
- Git-backed artifacts are for concept definitions, human-authored content, and publishable assets
- endpoint mounts expose published artifacts and extension responses through
  core-controlled routing

## Conversational Surfaces

Move Big Rocks treats customer chat as a core operational primitive rather than as a
thin case-creation wrapper.

Core owns:

- conversation sessions
- queue placement and queue-item state for queued conversations
- conversation transcripts and message metadata
- policy and knowledge retrieval during a conversation
- forms drafting during a conversation
- escalation from conversation to case when needed
- website widget and mobile SDK protocol boundaries

Runtime storage principle:

- PostgreSQL stores the live conversation graph, classification state, and audit trail
- object storage holds large attachments, uploaded files, screenshots, audio, and oversized runtime artifacts

The rule is:

- a conversation may stay a conversation
- a conversation may produce an forms draft
- a conversation may escalate into a case
- a queued conversation and a queued case should be visible in the same queue surface through queue items
- a case links to conversations; it does not replace the conversation model
- Move Big Rocks decides which of those outcomes is allowed and records the audit trail

Optional local-agent connectors such as OpenClaw can participate as supervised
conversation delegates, but the public chat surface connects to Move Big Rocks, not
directly to the user's local runtime.

## Repository Topology

The runtime architecture assumes a matching repo architecture:

- the public core repo holds shared source, release pipelines, and deploy assets
- a private instance repo created from a public template holds deployment desired state for one installation
- extension source lives in separate repos from both core and the instance repo

The instance repo contains pinned versions, deploy workflows, extension refs,
branding, and operational policy. It is not a long-lived fork of core.

## Packaging Optional Capabilities

Customer-facing capabilities move to extensions when they are not required for
every deployment.

That means:

- ATS is an extension
- enterprise access is an extension
- analytics is an extension
- error tracking is an extension
- operational health is an extension

Core Move Big Rocks keeps the minimum internal instrumentation needed to run safely.

## Email and Channel Architecture

Email is modular and adapter-driven.

Recommended production path:

- Postmark for outbound and inbound
- generic SMTP for outbound relay
- provider adapters for additional vendors such as SES

Design rules:

- core owns conversations, cases, threading, audit trails, and delivery state
- provider adapters own transport-specific API calls and webhook parsing
- inbound email is push-based instead of mailbox polling by default
- Move Big Rocks itself is not a full mail server

The outbound mail layer stays behind provider interfaces and a provider
registry so new vendors can be added without rewriting the orchestration
service.

## Artifact Delivery

Core and extension delivery follow one consistent shape:

- core publishes release artifacts and OCI artifacts
- extension bundles are signed and distributed over OCI-compatible registry transport
- local file installation remains available for development and emergency recovery

This lets the CLI install either a local bundle or a registry ref while keeping
verification rules the same.

## Delivery Shape

- Go backend
- Gin HTTP routers
- GraphQL via shared resolvers and services
- PostgreSQL + sqlx
- outbox-driven background processing
- Go-template admin UI
- public route hosting for extension websites
