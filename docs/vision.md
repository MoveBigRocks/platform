# Vision

Move Big Rocks is a team-aware operations platform and context engineering
infrastructure for self-hosted businesses that want one system of record and a
thin layer of installable product packs.

## Thesis

Most small and midsize companies do not want separate SaaS products for
hiring, support, analytics, error tracking, and internal operations. They want
one place where multiple teams and multiple agent hosts can see the same
customer, the same conversation, the same case, the same forms, the same
knowledge, and the same audit trail.

The winning shape is:

- a stable operational core
- workspaces for logical and security isolation, and teams for ownership
- conversations for live interaction and cases for durable work
- Markdown-first shared knowledge with explicit team, shared-team, and workspace visibility
- Git-backed versioned artifact surfaces for knowledge and publishable content
- a CLI-first machine interface
- a product model that any capable agent can operate
- a hosted sandbox you can spin up quickly for evaluation and agent-led setup
- extensions that reuse shared primitives
- self-hosted economics instead of permanent per-seat SaaS rent
- optional commercial packs sold on top of a strong free core

That does not mean another disconnected planning suite. It means one system
that can replace meaningful slices of Confluence and Jira for operations-heavy
teams by connecting strategic context, knowledge, service catalog, forms,
queues, conversations, and cases.

The base product is free to self-host for support operations and adjacent
operational workflows. The commercial layer sits above core in first-party
extensions, curated marketplace packs, and professional services.

The evaluation model should be simple too:

- self-hosted is both the production default and the current evaluation default
- the shortest supported path is one Ubuntu VPS you control or a local technical setup
- the first owned runtime should use the same `mbr` and GraphQL contract as any longer-lived instance
- learning in that first owned runtime should convert cleanly into a real deployment
- the public site and docs should make the build-it-yourself path legible to agents rather than hiding the real work behind evaluator theater

## Core Operating Model

- `workspace` is the logical and security boundary inside an instance
- `team` is the operational ownership boundary inside a workspace
- `channel` is where an interaction arrives or is published
- `topic` is what the work or knowledge is about
- `queue` is where work is processed
- `queue_item` is the queue-local work record that points to a conversation or case
- `conversation_session` is the live interaction surface
- `case` is the durable work item created when ownership, SLA, approval, or follow-through exists, with its own canonical work thread
- `concept_spec` is the versioned definition for a structured knowledge concept
- `knowledge_resource` is Markdown-first shared memory with provenance, review state, and citations
- `form_spec` is the structured information contract for a request or action
- `form_submission` is a collected instance of that contract
- `concept_spec_library` groups related concept specs by team type or domain

A form spec is not just a form definition. It defines what must be known,
evidenced, or approved before Move Big Rocks accepts, routes, escalates, or
acts on work. A form or chat flow can render forms, but forms remains the
actual model.

Conversations and cases should feel similar in the operator experience, but
they are not the same object. A conversation can stay conversational, collect
missing information, cite knowledge, and resolve without becoming a case. A
case exists when durable operational ownership is required.

Queues hold both conversations and cases through `queue_item`. A conversation
may enter a queue, get handled, and resolve without ever becoming a case. When
durable work is needed, Move Big Rocks creates a case linked to the source
conversation. A case may also exist without an originating conversation, and a
case may later accumulate additional linked conversations across chat, email,
or follow-up threads.

A case should still feel conversational. The durable record is not just
metadata around detached messages. A case owns a canonical work thread for
notes, decisions, replies, approvals, and agent summaries, while linked
`conversation_session` records provide source interactions and follow-up
context. Agents should also be able to route cases and conversations between
teams through the CLI on behalf of their human principal when policy allows
that action. That policy should live on workspace membership constraints rather
than being an accidental side effect of generic write permission.

## Core Product

Move Big Rocks core owns the operational primitives that multiple extensions need and
that are valuable on their own:

- workspace
- team
- team membership
- user
- agent
- service catalog node
- queue
- queue item
- contact
- conversation
- case
- case labels and categorization
- concept spec
- knowledge resource
- form spec
- form submission
- attachment
- automation rule
- public route

Core also keeps:

- magic-link login
- sessions and cookies
- roles, permissions, and memberships
- audit trails
- internal health and break-glass operator access
- outbox and event-bus integration
- event-driven automation and callouts

The point of free core is that it already gives a customer a secure operational
foundation:

- team-owned knowledge resources that guide work, with explicit team,
  shared-team, and workspace visibility
- service catalog nodes and topics that classify work and self-service paths
- form specs that define what information is required before work is accepted
  or acted on
- queues that can hold conversation and case work through queue items
- conversations that can stay conversational or escalate into work
- cases that can be replied to and automated
- labels and queues for organizing operational work
- identity and agent access
- event infrastructure
- automation that can call out when events happen
- deployable hosting and infrastructure conventions

## Knowledge Strategy

Knowledge is Markdown-first. Every team should be able to maintain deliberate,
versioned operational memory instead of leaving critical context scattered
across local Markdown files, prompts, and chat transcripts.

That knowledge system should be structured, not just tagged blobs. Move Big
Rocks should treat RFCs, ADRs, templates, constraints, checklists, skills,
ideas, strategic context, and team-specific concepts as versioned
`ConceptSpec` definitions. Knowledge resources are the Markdown instances of
those definitions.

Each workspace should support:

- team-only knowledge for drafts, notes, prompts, and internal playbooks
- selectively shared knowledge for approved guidance other named teams and agents can rely on
- workspace-visible knowledge for cross-team standards and common operating policies

The access model should be explicit:

- internet publication is not the same thing as in-workspace visibility
- `ConceptSpec` defines the default and maximum audience for a concept instance
- `KnowledgeResource` stores the actual audience for one instance, such as `team`, `shared_teams`, or `workspace`
- the resource also stores the concrete share targets and any minimum owner-team role needed for access
- widening visibility beyond the default should require owner-team approval and should never exceed what the concept spec allows

Git-compatible version history is a good implementation choice for Markdown
artifacts, but the product abstraction should stay focused on draft, review,
publish, share, search, and cite rather than forcing every user into raw Git
workflows.

The storage model should stay explicit:

- `ConceptSpec` definitions are versioned artifacts, usually YAML-backed, and can be built in by core or registered by teams and extensions
- `ConceptSpec` also carries the default workflow, review rules, and audience constraints for its instances
- `KnowledgeResource` instances store Markdown, parsed frontmatter, review/publication metadata, actual access policy, and a pinned concept spec key/version
- changing a concept definition means creating a new concept spec version, not mutating old instances in place
- concept spec libraries group related specs by team type or domain and can be shared across workspaces; the specs travel but knowledge resource instances stay local to each workspace
- forms follow the same model: a form spec can be used internally by one team, shared across teams, or published across workspaces

The Strategic Context Stack and delivery context belong in that same model:

- core concept spec libraries should include built-in specs such as `core/purpose`, `core/vision`, `core/mission`, `core/goal`, `core/strategy`, `core/bet`, `core/okr`, `core/kpi`, `core/milestone-goal`, and `core/workstream`
- administrators can register instance-global concept spec libraries that every workspace may use
- teams and extensions can publish concept spec libraries across workspaces; adopting workspaces get the structure and fill in their own knowledge resources
- actual strategic-context instances stay scoped to one workspace and can then be private to one team, shared with named peer teams, or visible across that workspace
- teams should be able to connect goals, strategies, bets, OKRs, and KPIs directly to queues, service catalog nodes, form specs, knowledge, extensions, and work

The Strategic Context Stack should be understood as a real layered model:

- `purpose`, `vision`, `mission`, `goal`, `strategy`, `bet`, `okr`, and `kpi` are not interchangeable words
- each layer has a different job, time horizon, and owner
- the stack sits above the delivery layer and keeps the milestone from drifting into local optimisation
- milestone goals and workstreams should operationalise that stack for a specific delivery rather than replacing it

For delivery teams, the same concept layer should reduce ticket explosion:

- a team can define milestone goals and workstreams with full context, assumptions, constraints, and success measures
- agents can reason from those richer concept instances and linked work instead of needing a project plan decomposed into dozens of shallow tickets
- queues, queue items, and cases still matter for interrupt-driven work, approvals, and durable follow-through, but they should not be the only unit of coordination
- execution should stay anchored in a stable milestone artefact plus a proof loop around what is now proven, still open, partially evidenced, and blocked

The reference model should be agent-ready too:

- Markdown bodies can use typed references such as `@goal/...`, `@strategy/...`, `@bet/...`, `@okr/...`, `@kpi/...`, `@milestone/...`, `@workstream/...`, `@team/...`, `@queue/...`, and `@catalog/...`
- those inline references are convenience and should render as first-class links or chips
- the canonical relation graph should still be stored in frontmatter and structured metadata, not inferred only from prose

The sync model should stay explicit too:

- Move Big Rocks remains the source of truth for access control, review state, and accepted revisions
- `mbr` should support an opt-in local checkout for knowledge and concept artifacts so humans and agents can work with real files
- that checkout should be filtered to the workspaces, teams, and resources the current actor is allowed to access
- a hidden local cache is an implementation detail; the visible filesystem checkout is a deliberate user action

This is what lets Move Big Rocks become a serious replacement for scattered
docs and issue-routing systems: teams can keep strategy, operating context,
templates, and work in one auditable, agent-usable substrate instead of
splitting them across isolated SaaS tools and local Markdown folders.

The same internal Git-backed artifact model should also be available to
extensions for content-bearing surfaces such as websites, templates, prompts,
and published docs. Core should own the version ledger, review and publication
flow, and endpoint publishing rules so extensions do not invent their own
storage and trust model.

## Repo Strategy

Move Big Rocks uses a three-layer delivery model:

- a public core repo for the shared runtime and releases
- a private deployment repo per customer installation
- separate extension repos for first-party and customer-built packs

The private deployment repo is not a fork of core. It is a control-plane repo
that stores the desired state for one live installation: pinned versions,
deployment workflows, extension configuration, branding, and operational
runbooks.

## Agent Operating Model

Move Big Rocks is understandable and operable by any capable agent host.

That includes:

- Claude Code
- Codex
- OpenClaw
- other hosts using the same CLI and GraphQL contract

The rule is:

- agents operate Move Big Rocks through the same machine contract
- Move Big Rocks holds the context, permissions, approvals, and audit trail
- agents operate with explicit workspace and team context
- optional local connectors strengthen the operating model, not replace it

The CLI should feel closer to `gh` than to an internal admin script: noun-led,
predictable, easy to inspect with `--json`, and usable by both humans and
agents.

That same ergonomic bar should apply before production too. A user should be
able to ask an agent to create a private instance repo, validate it, and
deploy it to one Ubuntu VPS they control without the process feeling bespoke.

The agent ergonomics bar should be very high:

- the user should be able to ask an agent to build an owned instance in one sentence
- the agent should need only minimal follow-up such as VPS choice, domain, admin email, provider credentials, and SSH access
- the agent should be able to create the repo, validate desired state, deploy the pinned artifacts, and then hand back the URL and next suggested actions
- before `mbr` is installed, the agent should be able to query one public bootstrap endpoint for docs discovery, CLI install metadata, and self-host bootstrap guidance

The public site should therefore have two layers:

- human docs organized around CLI install, agents, self-host, extensions, pricing, and security
- machine-readable bootstrap endpoints for agents and CLIs

That first owned path should also feel product-complete:

- one Ubuntu VPS should be enough for the first real deployment
- local technical setup should remain possible for hands-on evaluators
- extension trials should happen in a dedicated preview workspace on the owned instance
- the agent should do the bulk of the repo, deploy, and validation work through explicit documented surfaces

## Extensions

Extensions are not one flat category. Move Big Rocks classifies them so permissions,
review, and marketplace policy stay clear:

- **product extensions** add workflows, entities, knowledge, forms, routes, and automations
- **identity extensions** add enterprise login and provisioning methods
- **connector extensions** connect Move Big Rocks to external transports, notification systems, and optional local agent runtimes
- **operational extensions** add diagnostics, monitoring, probes, and operational workflows

An extension can:

- define extension-owned entities
- own an extension-local PostgreSQL schema for structured operational state
- register extension-owned concept specs for structured knowledge artifacts
- create or attach to workspace queues
- register public routes and admin routes
- publish knowledge resources and form specs
- use core-managed Git-backed artifact surfaces for versioned Markdown,
  templates, and publishable site content
- create automation defaults
- register CLI commands and JSON schemas
- read and write approved primitives through GraphQL

Teams can author their own extensions. The authoring unit can be a team, but
the runtime installation scope remains `workspace` or `instance`. That keeps
ownership flexible without fragmenting the trust model.

An extension cannot:

- mutate core internals without declared permissions
- run arbitrary in-process code inside the main server
- bypass workspace isolation

Identity extensions are a special case. They can add enterprise login methods
such as OIDC, SAML, and directory sync, but they do not own sessions, users,
memberships, or authorization. Core remains the source of truth for those
concerns.

## First-Party Extension: ATS

ATS is the first-party applicant tracking extension with a simple careers
site:

- each published job owns a queue
- each application form submission creates a case
- each candidate is a contact
- ATS owns extension-local recruiting state in its own PostgreSQL schema
- the careers site is a lightweight public route served by Move Big Rocks from
  extension-managed versioned artifacts
- recruiters use the same workspace, agent, and audit model as the rest of the product

`Application` remains reserved for software and operational application
monitoring. Hiring uses `job_posting`, `candidate`, and `candidate_case`.

## First-Party Extension Set

Move Big Rocks includes a clear set of first-party extension classes:

- `ats`: applicant tracking and careers site
- `enterprise-access`: enterprise SSO and directory sync
- `error-tracking`: Sentry-compatible ingestion and operational issue workflows
- `web-analytics`: privacy-first website analytics
- `operational-health`: probes, health reporting, and operational diagnostics

Slack alerting, WhatsApp, email transport integrations, and agent-runtime
connectors such as OpenClaw belong to the connector class. They use the same
lifecycle system, but they are not the same kind of extension as ATS or
analytics.

## Identity Strategy

Enterprise access is an extension because not every installation needs it, but
it is also a privileged class of extension.

Core keeps:

- magic-link login
- local admin recovery access
- session issuance
- workspace membership and RBAC

The `enterprise-access` extension adds:

- OIDC
- SAML
- claim mapping
- just-in-time provisioning
- directory sync

This keeps enterprise auth optional without fragmenting the core security
model.

## Email and Channel Strategy

Move Big Rocks is not a mail server. It integrates with delivery and inbound providers
through adapters.

The product stance is:

- recommend Postmark for outbound and inbound email
- support generic SMTP for outbound relay
- keep the outbound mail layer adapter-based so SES and other providers fit cleanly
- treat email, Slack, and WhatsApp as connector surfaces rather than core assumptions

Inbound email is push-based when possible:

- provider webhook ingestion for Postmark
- provider event ingestion for other supported providers
- forwarding-based forms instead of mailbox polling as the default

Customer chat is not modeled as a direct channel-to-case shortcut. Move Big Rocks owns
conversations as a first-class primitive so a chat can:

- remain a conversation
- use knowledge and policy
- prepare forms drafts
- escalate into a case only when that is the correct operational outcome

## Agent-Built Software

Move Big Rocks is a secure landing zone for agent-built operational software.

The rule is:

- if a customer-built app needs Move Big Rocks primitives, Move Big Rocks auth, or Move Big Rocks public routes, it is an extension
- if it is a separate service, it lives in its own repo and integrates with Move Big Rocks through GraphQL, events, webhooks, or the CLI

That is what makes Move Big Rocks a real operational hub rather than just another
self-hosted app:

- a customer can buy an off-the-shelf extension and activate it quickly
- or a team can build its own private extension on the same primitives and lifecycle model
- or they can do both in the same installation

The deployment repo carries security prompts, threat-model prompts, and
operational runbooks so an agent has a predictable place to check the
customer's operational policy before shipping custom work.

## Packaging and Distribution

Move Big Rocks does not need to process payments inside the product.

The public bundle flow is:

1. A customer or operator chooses a signed first-party bundle or a custom bundle they built themselves.
2. They install it into a workspace or instance with the `mbr` CLI.
3. Move Big Rocks verifies the publisher signature and any instance-bound install credential in use for that bundle flow.
4. Move Big Rocks provisions the routes, data model, commands, and permissions.
5. The extension goes live without separate code deployment.

This supports the current public model:

- free self-hosted core
- free public standard-risk first-party bundles for ATS, error tracking, and web analytics
- private custom extensions by default
- free redistribution when an extension publisher chooses to give an extension away
- professional services for customizations and custom extension work
- separate written permission required to sell or license the platform,
  derivative works of it, extensions, or access to them, or to offer them as
  a hosted service

That means a customer who does not want the first-party analytics pack can
build their own analytics extension or use another free extension without
changing the core architecture.

## Distribution Model

Packaging follows a clear structure:

- the public core repo is the discovery and deployment entry point
- the core is deployed from pinned releases, not from customer-maintained forks
- free public first-party standard-risk bundles ship as signed OCI bundles
- customer-built extensions are private by default and can stay private or be given away for free
- marketplace publication follows real adoption later rather than being a Milestone 1 dependency
- privileged identity and connector packs remain separately controlled first-party packs unless their trust model is explicitly expanded

## Optional First-Party Packs

Not every Move Big Rocks install needs everything. These remain optional:

- ATS and careers site
- enterprise access
- analytics
- error tracking
- operational health and probes

Move Big Rocks keeps its own internal health, logs, and metrics for operating the
server safely.
