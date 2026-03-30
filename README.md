# Move Big Rocks

**Replace SaaS tool sprawl with one self-hosted operations platform. Move Big Rocks is free to use with source code available, gives you a growing set of free off-the-shelf extensions, and lets you build your own with Claude Code, Codex, or other tools. Manage operational knowledge in structured Markdown with real versioning, team ownership, and agent-native workflows.**

This repository is the source of truth for Move Big Rocks core.

Public site: [movebigrocks.com](https://movebigrocks.com)

[![License: MBR Source Available](https://img.shields.io/badge/License-MBR%20Source%20Available-blue.svg)](LICENSE)

## The Problem

Paying $500 a month for an ATS might make sense in isolation. So might $300
for error tracking, $200 for analytics, $150 for a knowledge base, $400 for
a CRM, and $100 for a forms tool. Each one has its own business case, its
own champion, its own budget line. But a mid-market company runs 20 or 30
of these tools, each charging per seat, each modelling work differently,
each siloing the context that matters. Together they do not make sense at
all. Some companies have literally hundreds of SaaS tools deployed. The
aggregate is six or even seven figures per year spent on a disconnected set
of systems that were never designed to work with each other.

These tools fragment your operations, fragment your knowledge, and create a
sprawling attack surface of dozens or hundreds of separate vendors with
access to your data, your credentials, and your employees' identities.
Every additional SaaS tool is another vendor to vet, another SSO
integration to maintain, another set of API keys to rotate, another third
party processing your data under their terms.

And when the person who championed a tool leaves the company, the tool
enters a kind of zombie mode: still live, still billing, still holding
data, but no longer sponsored or maintained by anyone internally. Nobody
knows what it connects to, nobody knows what would break if it were turned
off, and nobody wants to find out. Most organisations have dozens of these
zombie tools running at any given time, each one an unmanaged cost and an
unmanaged risk.

This was already expensive, inefficient, and risky for humans. For agents,
it is unworkable. Claude Code, Codex, and other agent tools are becoming
part of how teams operate. But an agent cannot reason coherently across
twenty disconnected systems with twenty different data models, twenty
authentication schemes, and twenty ways of representing the same work. SaaS
sprawl was never designed for this, and layering AI on top of it makes the
mess worse, not better.

The time has come for an alternative that is an order of magnitude cheaper,
an order of magnitude more coherent, and built from the ground up for both
humans and agents. One operational core where work, knowledge, strategic
context, and agent activity live in the same system. Extensions you can
install for a fraction of what the standalone tool costs, or build your own
with an agent in an afternoon.

That is what Move Big Rocks is.

## Provenance

Move Big Rocks is not a speculative blank-sheet project.

- It is based on a multi-tenant service management platform first built in
  2010.
- That earlier system ran in production for more than 10 years across campuses
  around Australia, supporting students with all kinds of issues across
  multiple departments including student services.
- AI-assisted migration made it practical to carry that codebase forward into
  Go with a cleaner architecture, explicit extensions, and a stronger
  agent-operable command surface.

## What Move Big Rocks Does About It

Move Big Rocks replaces your SaaS sprawl with one self-hosted operational
core and a set of extensions, each an order of magnitude cheaper than the
tool it replaces:

- **Service catalog and forms** — define what work exists and what information
  must be collected before work is accepted, routed, or acted on
- **Queues, conversations, and cases** — handle live interactions, keep
  conversations conversational when possible, and create durable cases only
  when real follow-through is needed
- **Structured Markdown knowledge** — RFCs, templates, runbooks, constraints,
  prompts, and team-specific models in one versioned system with explicit
  audience control instead of scattered wikis and slide decks
- **Strategic context and delivery** — goals, strategy, bets, OKRs, milestone
  goals, and workstreams as real concept types so agents and humans reason
  from the same context instead of shallow tickets
- **Automation and events** — trigger actions when operational events happen
  without hidden glue
- **Agent access built in** — one CLI and one GraphQL API for humans and
  agents, not parallel shadow workflows
- **Extensions you can vibe-code** — use the SDK and an agent to build your
  own extensions against the same runtime model, or install off-the-shelf
  extensions that replace entire standalone SaaS products

The core platform is free. All first-party extensions are currently free
too. When we introduce paid extensions, pricing will be per instance, not
per user, and an order of magnitude below what the standalone SaaS tool
charges. Every installation made before that date will be grandfathered at
no cost.

## Extensions You Can Install

Each extension replaces a standalone SaaS product, running on the same
operational core:

| Extension | What it replaces | What they charge |
|-----------|-----------------|-----------------|
| **ATS** | Homerun, Greenhouse, Lever | €79/mo to $12K+/yr depending on scale |
| **Error tracking** | Sentry, Bugsnag, Rollbar | $26/mo to thousands/mo at volume |
| **Web analytics** | Plausible, Mixpanel | $9/mo to $2,500+/mo at scale |
| **Sales pipeline** | Pipedrive, HubSpot | $14-99/user/mo (Pipedrive) to $500-1,500/mo (HubSpot) |
| **Community feature requests** | Canny, UserVoice | $79/mo (Canny) to $899-1,499/mo (UserVoice) |
| **Enterprise access** | SSO tax on every SaaS tool | $1,000-5,000/yr per vendor |

All of the above are currently free to install. When paid tiers are
introduced, pricing will be per instance with no per-seat tax — a fraction
of what these tools charge today.

Every extension runs on the same core primitives — same teams, same queues,
same knowledge, same agent contract. Your ATS candidates, error tracking
issues, sales pipeline deals, feature requests, and support cases live in one
system instead of six disconnected tools with six logins and six bills.

Install guidance:
[`docs/CUSTOMER_INSTANCE_SETUP.md`](docs/CUSTOMER_INSTANCE_SETUP.md),
[`docs/AGENT_CLI.md`](docs/AGENT_CLI.md), and
[movebigrocks.com/extensions](https://movebigrocks.com/extensions).

## License Model

Move Big Rocks is source-available under the MBR Source Code Available License
1.0. The public model is meant to be clear:

- you can self-host and use Move Big Rocks inside your own organisation,
  including for your own internal
  commercial operations
- you can modify it for your own needs
- you can use the free first-party public bundles
- you can build and use your own extensions
- you may not sell, license, or otherwise commercialize the platform itself,
  copies of it, or derivative works of it
- you may not sell access to the platform or offer it as a hosted or managed
  service
- you may not sell, license, or otherwise commercialize extensions, add-ons,
  or derivative works built for Move Big Rocks without separate written
  permission from Move Big Rocks BV
- if you fork or redistribute it, you must keep the license and notices intact

See [`LICENSE`](LICENSE) for the governing text.

## Build Your Own Extensions

Move Big Rocks is a platform you can build on. Give an agent the extension
SDK and describe what you want. It scaffolds, implements, tests, and deploys
a working extension through the same CLI and lifecycle as the first-party set.

- Use the same extension model as the first-party extensions
- Own structured state in your own `ext_*` PostgreSQL schema
- Use Git-backed artifact surfaces for websites, templates, and published docs
- Register concept specs and agent skills so your extension is discoverable
- Test in a preview workspace, validate, then promote to production
- Keep your extensions private or publish them

The extension model is closer to Shopify app extensions than to WordPress
plugins: explicit manifests, explicit permissions, lifecycle hooks, and
out-of-process execution. The difference is that anyone with an agent can
build one.

## Knowledge For Agentic Operations

Move Big Rocks treats knowledge as a first-class operational primitive, not an
afterthought wiki:

- **Markdown-first** — author in the format humans and agents already prefer
- **Structured by concept specs** — versioned definitions for RFCs, templates,
  constraints, skills, goals, strategies, and team-specific models
- **Explicit audience control** — team-only, shared with named peers, or
  workspace-visible, never wider than the concept spec allows
- **Workspaces and teams** — workspaces isolate tenants, teams own operational
  responsibility, knowledge respects both boundaries
- **Local filesystem workflows** — check out an ACL-filtered working copy,
  edit real files, diff changes, and sync them back
- **Strategic Context Stack** — purpose, vision, mission, goals, strategy,
  bets, OKRs, KPIs, milestone goals, and workstreams as structured concept
  instances linked to queues, catalog, and work
- **Git-backed versioning** — revision history without forcing raw Git
  workflows on every user

The structured knowledge model works like this:

- `ConceptSpec` defines the versioned structure, workflow, and agent guidance
  for a concept such as an RFC, checklist, template, or team-specific brief
- `KnowledgeResource` stores the actual Markdown instance, parsed frontmatter,
  review state, in-workspace access policy, Git revision metadata, and a pinned
  `concept_spec_key` plus `concept_spec_version`
- agents and humans work against the same concept definitions and instance records
- Markdown bodies can use typed references such as `@goal/...`,
  `@strategy/...`, `@milestone/...`, `@workstream/...`, `@queue/...`, and
  `@catalog/...`; the canonical relation graph stays in structured metadata
- the server-side database and artifact service remain the source of truth
- `mbr` can materialize an ACL-filtered local checkout of the knowledge a user
  or agent is allowed to see, including create, update, and delete sync back to
  the server
- local knowledge edits are validated against concept specs and permissions
  before they are accepted back, and concept specs themselves expose register,
  list, show, history, and diff workflows through `mbr concepts ...`

The Strategic Context Stack should stay explicit: `purpose`, `vision`,
`mission`, `goal`, `strategy`, `bet`, `okr`, and `kpi` have different jobs and
different time horizons. Milestone goals and workstreams sit below that stack
as the delivery layer. Teams can define milestone goals, workstreams, and
linked strategic context in one place, and agents can work from those richer
concept instances instead of requiring a backlog full of tiny tickets.

This is what lets Move Big Rocks replace meaningful slices of Confluence and
Jira: teams keep strategy, operating context, templates, and work in one
auditable, agent-usable system instead of splitting them across isolated SaaS
tools and local folders.

## Agent-Native By Design

Move Big Rocks is designed so a capable agent can operate it efficiently.

That includes Claude Code, Codex, OpenClaw, and other hosts that can use CLI,
GraphQL, or a thin adapter over the same contract.

The model is consistent:

- give the agent the Move Big Rocks repo or the instance repo
- give it the current `mbr` CLI and machine-readable contract
- give it the relevant workspace and team context
- let it operate through Move Big Rocks approved surfaces

That means an agent should be able to:

- deploy and configure Move Big Rocks
- work conversations and cases
- retrieve and publish knowledge
- fill out forms and submit requests
- install and configure extensions
- help teams author private extensions

Move Big Rocks is the place where records live, permissions are enforced,
approvals happen, and audit trails are recorded. Agents operate through the
same contract as humans — one `mbr` CLI with `--json` on every command, one
GraphQL API, machine-readable bootstrap endpoints for discovery, and explicit
workspace and team context.

A user should be able to tell their agent "create me a Move Big Rocks instance
repo, validate it, and deploy it to one Ubuntu VPS I control" and the agent
handles most of the work through GitHub, SSH, and `mbr`.

## Move Big Rocks With OpenClaw

OpenClaw is optional. If a user connects a local OpenClaw setup to Move Big
Rocks, it becomes a stronger operational hub for setup, operations, support
work, and extension authoring.

OpenClaw can help with instance setup, case assistance, knowledge retrieval,
form draft preparation, and extension scaffolding. Move Big Rocks owns the
operational data, the permissions, the approvals, the extension lifecycle, and
the audit trail. OpenClaw can help control Move Big Rocks, but Move Big Rocks
stays in charge.

## Build It Yourself, With Agent Help

Move Big Rocks is self-hosted in production and, for now, evaluation is owned
too.

The current try-it path is:

- start on one Ubuntu VPS you control, or run it locally if you are
  comfortable with a technical setup
- give the agent this repo for product understanding
- have it create a private instance repo from the template
- let it validate `mbr.instance.yaml`, prepare secrets, deploy the pinned core
  artifacts, and hand back the URLs and next steps
- use a dedicated preview workspace on that instance for extension trials
  before broader rollout

There is no vendor-hosted sandbox path right now. The product bar instead is
that an agent can help you stand up an owned instance with minimal follow-up.

Hosted sandboxes are still a possible future evaluation path, but they are
deferred rather than available today. See
[RFC-0013](docs/RFCs/RFC-0013-hosted-sandbox-control-plane.md) for the
preserved design and comment on
[issue #1](https://github.com/MoveBigRocks/platform/issues/1) if you would
like that path to become real.

## Self-Hosted, Not SaaS

- **Free core** — self-host the platform without per-seat rent
- **Off-the-shelf extensions** — use focused first-party extensions when the
  depth matters, on the same shared base
- **Owned evaluation path** — start on one Ubuntu VPS you control, or use a
  local technical setup if you prefer
- **Your instance, your control** — one private instance repo, one Linux host,
  pinned releases, no vendor lock-in
- **Buy or build on the same base** — use first-party extensions or ship your own
  private or team-authored extensions against the same runtime model

## Why Move Big Rocks

- **Shared operational primitives**: workspace, team, queue, service catalog node, case, conversation, label, contact, concept spec, knowledge resource, form spec, attachment, automation, agent
- **Structured Markdown knowledge**: versioned concept specs define RFCs, templates, constraints, skills, ideas, strategic context, and other team concepts with default and allowed visibility and review rules
- **Versioned artifact surfaces**: Git-backed Markdown and publishable content artifacts managed by core for teams and extensions
- **CLI-first agent access**: stable commands, JSON output, strict exit codes, browser login for humans, token auth for agents
- **Explicit delegated routing**: agent handoff and escalation governed through workspace membership constraints, not generic write access
- **Owned first deployment**: no vendor sandbox required; start on
  infrastructure you control and keep the path to production honest
- **Single source of truth**: GraphQL backed by shared services and audit trails
- **Optional product layers**: ATS, community feature requests, sales pipeline, enterprise access, error tracking, web analytics, operational health, and agent-runtime connectors install as extensions
- **Self-hosted by default**: one Go service, PostgreSQL, predictable deployment

## Pick The Right Repo

Use this repo if you are:

- understanding Move Big Rocks as a product
- working on Move Big Rocks core source
- defining the core architecture, CLI, and extension model

Use a private instance repo if you are:

- deploying or operating a live Move Big Rocks instance
- configuring instance-specific domains, secrets, or extensions
- doing customer-specific branding or rollout work

For those paths:

- deploy or operate an instance: [START_WITH_AN_AGENT.md](https://github.com/MoveBigRocks/platform/blob/main/START_WITH_AN_AGENT.md) and [MoveBigRocks/instance-template](https://github.com/MoveBigRocks/instance-template)
- build a custom extension: [MoveBigRocks/extension-sdk](https://github.com/MoveBigRocks/extension-sdk)

## Core Model

Move Big Rocks core owns the operational primitives that extensions build on:

- **Workspace** — the tenant and security boundary inside an instance
- **Team** — the operational ownership boundary inside a workspace
- **Service catalog** — what work exists in the organisation
- **Forms** — what information must be collected before work is accepted or
  routed (structured specs with validation, tied to catalog and routing)
- **Queue** — where work is processed, holding queue items that point to
  conversations or cases
- **Conversation** — a live interaction that can resolve on its own or
  escalate into a case
- **Case** — durable work with ownership, SLA, approvals, and follow-through
- **Knowledge resource** — Markdown content pinned to a versioned concept spec
  with explicit audience control
- **Concept spec** — the versioned definition for a structured knowledge
  concept (RFC, template, goal, strategy, milestone, workstream, etc.)
- **Automation** — event-driven rules that trigger actions
- **Agent** — first-class principal with workspace-scoped tokens and audit
- **Contact** — the external person a conversation or case is about
- **Attachment** — files uploaded to cases, forms, or knowledge

## Product Shape

Move Big Rocks core stays small and composable:

- **Core**: auth, workspaces, teams, memberships, agents, contacts, service
  catalog nodes, queues, conversations, cases, case labels, Markdown knowledge,
  concept specs, forms, attachments, automation, GraphQL, admin panel, public
  routes, event-driven integrations
- **Public first-party extensions**: ATS, community feature requests, error
  tracking, sales pipeline, and web analytics in the public first-party
  extensions repo at `MoveBigRocks/extensions`
- **Optional first-party extensions**: enterprise access, operational health,
  agent-runtime connectors

## Delivery Model

Production delivery follows a split repo model:

- **Public core repo** for source, releases, docs, and shared runtime contracts
- **Public instance template repo** at `MoveBigRocks/instance-template`
- **Public extension SDK repo** at `MoveBigRocks/extension-sdk`
- **Public first-party extensions repo** at `MoveBigRocks/extensions`
- **Private instance repo** created from `MoveBigRocks/instance-template` and
  used as the deployment control plane for one customer installation
- **Optional custom extension repo** created from `MoveBigRocks/extension-sdk`
  when a team needs custom extension code

Customers deploy pinned core releases from their private instance repo, then
install signed extension bundles into that running instance. This is what makes
the product agent-friendly: an agent can open the instance repo, configure
secrets, deploy the server, install extensions, and keep the installation
upgraded without managing a permanent core fork.

## Production Setup

Production is operated from a private instance repo created from
`MoveBigRocks/instance-template` and driven through pinned Move Big Rocks
releases from there.

Most customers need only one private instance repo. A separate custom extension
repo is only needed when building custom extension logic.

If you want to explore Move Big Rocks through Codex or Claude Code, start with:

- [START_WITH_AN_AGENT.md](https://github.com/MoveBigRocks/platform/blob/main/START_WITH_AN_AGENT.md)

Start here:

- [Customer Instance Setup](https://github.com/MoveBigRocks/platform/blob/main/docs/CUSTOMER_INSTANCE_SETUP.md)
- [Customer FAQ](https://github.com/MoveBigRocks/platform/blob/main/docs/CUSTOMER_FAQ.md)
- [Instance and Extension Lifecycle](https://github.com/MoveBigRocks/platform/blob/main/docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md)
- [Agent Recipes](https://github.com/MoveBigRocks/platform/blob/main/docs/AGENT_RECIPES.md)
- [MoveBigRocks/instance-template](https://github.com/MoveBigRocks/instance-template)
- [MoveBigRocks/extension-sdk](https://github.com/MoveBigRocks/extension-sdk)
- [Infrastructure Guide](https://github.com/MoveBigRocks/platform/blob/main/docs/INFRA_GUIDE.md)

## Local Development Quick Start

Move Big Rocks uses PostgreSQL in local development and production. Start or
point at a local PostgreSQL instance first, then:

```bash
git clone https://github.com/movebigrocks/platform
cd platform
cp .env.example .env
make run
```

`make run` reads `.env` when present. If `DATABASE_DSN` is left unset there, it
defaults to `postgres://$USER@127.0.0.1:5432/postgres?sslmode=disable`.

Access the service at `http://lvh.me:8080`.

If you want explicit binaries:

```bash
make build
./bin/mbr-server
```

The operator CLI lives at `./bin/mbr`.

## Agent Access

Create a workspace-scoped agent token:

```bash
make create-agent WORKSPACE=demo NAME="Operations Agent" OWNER=owner@example.com
```

The `mbr` CLI is the main operator and agent surface for authentication, teams,
queues, cases, knowledge, forms, attachments, and extension lifecycle work.

## PostgreSQL Runtime Contract

Move Big Rocks uses one PostgreSQL application database per instance.

- Core migrations are authored under `migrations/postgres/` and tracked in
  `public.schema_migrations`.
- Core tables live in bounded-context schemas such as `core_platform`,
  `core_service`, `core_automation`, and `core_knowledge`.
- Service-backed extensions own their schema-local migration files.
- Applied extension schema migrations are tracked under
  `core_extension_runtime.schema_migration_history`.
- Row primary keys use PostgreSQL-native `UUID` columns with `uuidv7()` defaults.

## Key Documents

- [Vision](https://github.com/MoveBigRocks/platform/blob/main/docs/vision.md)
- [Milestone 1 Scope](https://github.com/MoveBigRocks/platform/blob/main/milestone-1-scope.md)
- [START_WITH_AN_AGENT.md](https://github.com/MoveBigRocks/platform/blob/main/START_WITH_AN_AGENT.md)
- [Agent CLI](https://github.com/MoveBigRocks/platform/blob/main/docs/AGENT_CLI.md)
- [Customer Instance Setup](https://github.com/MoveBigRocks/platform/blob/main/docs/CUSTOMER_INSTANCE_SETUP.md)
- [Customer FAQ](https://github.com/MoveBigRocks/platform/blob/main/docs/CUSTOMER_FAQ.md)
- [Instance and Extension Lifecycle](https://github.com/MoveBigRocks/platform/blob/main/docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md)
- [Agent Recipes](https://github.com/MoveBigRocks/platform/blob/main/docs/AGENT_RECIPES.md)
- [MoveBigRocks/instance-template](https://github.com/MoveBigRocks/instance-template)
- [MoveBigRocks/extension-sdk](https://github.com/MoveBigRocks/extension-sdk)
