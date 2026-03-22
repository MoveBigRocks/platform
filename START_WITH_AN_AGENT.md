# Start With an Agent

If you want to explore Move Big Rocks through Codex, Claude Code, OpenClaw, or another
capable agent host, start here.

This file is the public handoff for agent-first discovery.

## What Move Big Rocks Is

Move Big Rocks is self-hosted operational software and context engineering
infrastructure built around shared primitives:

- workspaces
- teams and team membership
- service catalog nodes
- queues
- cases
- conversation sessions
- case labels
- concept specs
- contacts
- knowledge resources
- form specs
- form submissions
- attachments
- automation
- identity
- agent access

Optional product areas are installed as extensions instead of being permanently
built into core.

Workspaces isolate a tenant. Teams own operational work inside the workspace.
Queues are where work is processed. Knowledge is Markdown-first and should be
structured by versioned concept specs with explicit in-workspace visibility.

That means:

- concept specs define concepts such as RFCs, templates, skills, constraints, and team-specific models
- concept specs can also define strategic-context and delivery-context concepts such as purpose, vision, mission, goal, strategy, bet, OKR, KPI, milestone goal, and workstream
- concept spec libraries group related specs by team type or domain and can be shared across workspaces
- concept specs also define default and allowed audience rules for instances
- knowledge instances carry the actual audience, such as owner team only, selected peer teams, or the whole workspace
- forms and concepts can be used internally by one team, shared across teams, or published across workspaces
- internet publication is separate from this and applies mainly to extension-backed sites and routes
- typed Markdown references such as `@goal/...`, `@strategy/...`, `@milestone/...`, `@workstream/...`, `@queue/...`, and `@catalog/...` are useful for authors and agents, but canonical relations should still live in structured metadata

That is one reason Move Big Rocks can replace meaningful slices of Confluence
and Jira for operations-heavy teams: strategic context, knowledge, routing,
forms, queues, conversations, and cases can live in one agent-ready system.

For delivery teams, this also means agents can work from milestone goals and
workstreams with full context instead of needing a huge ticket backlog just to
understand what the team is trying to achieve.

The intended language for that model is **The Strategic Context Stack** above
the milestone artefact and delivery layer, not a loose mix of interchangeable
planning words.

Move Big Rocks also supports an optional local-agent connector model. If you connect a
local OpenClaw setup, Move Big Rocks becomes a stronger operational hub for setup,
operations, support work, and extension authoring. If you do not, Move Big Rocks stands
on its own.

For knowledge work, the operating rule is:

- Move Big Rocks remains the source of truth for permissions, review, and accepted revisions
- an agent can optionally materialize an ACL-filtered local checkout of concepts and knowledge Markdown when real filesystem work is helpful
- not every machine should get an automatic full visible clone by default

Before the CLI is installed, the intended discovery path is:

- the agent queries the public bootstrap endpoint
- the endpoint points to docs, CLI install metadata, and sandbox bootstrap guidance
- the agent installs `mbr` and continues through the normal machine contract

If the agent starts with only a homepage URL rather than the bootstrap URL, the
homepage source should explicitly point at the bootstrap endpoint instead of
forcing the agent to scrape prose and guess.

## The Main Paths

Tell your agent which of these you want.

### 1. Deploy core Move Big Rocks

Use this if you want:

- a self-hosted operations center
- workspaces, teams, queues, conversations, cases, knowledge, forms, attachments, and automation
- a clean starting point before adding any extensions

### 1a. Spin up a sandbox

Use this if you want:

- a disposable hosted trial in minutes
- 5 free days with a simple path to extend for 30 more days
- sandbox creation through `mbr` with a ready URL returned directly, then browser access plus the same `mbr` contract an agent would use later
- a safe place to explore teams, queues, knowledge, concept specs, and extensions
- access to first-party extensions in sandbox mode for evaluation
- a short evaluation path before committing to a real deployment

### 2. Install a first-party extension

Use this if you want to add:

- ATS
- web analytics
- error tracking
- enterprise access
- operational health

### 3. Connect Move Big Rocks to an always-on agent runtime

Use this if you want:

- your local OpenClaw setup or another always-on agent runtime to help operate Move Big Rocks
- case assistance, knowledge retrieval, forms drafting, and extension help
- Move Big Rocks to remain the place where work is controlled and audited

### 4. Build a private custom extension

Use this if you want Move Big Rocks to power:

- a team-owned internal workflow
- a custom admin tool
- a custom forms flow
- a custom public site
- a custom analytics or operational workflow

## What Repo the Agent Should Use

Use the public core repo to understand Move Big Rocks and choose the right path.

Then move into the right working repo quickly:

- deploy or operate one live installation: create or open a private instance
  repo based on [MoveBigRocks/instance-template](https://github.com/MoveBigRocks/instance-template)
- build a private custom extension: create or open a private extension repo
  based on [MoveBigRocks/extension-sdk](https://github.com/MoveBigRocks/extension-sdk)

The default should be:

- one private instance repo
- zero additional repos unless custom extension code is needed

## Minimal User Checklist

If you want the agent to do almost everything, the user should only need to:

1. create or pick one Linux host
2. make sure DNS can point at that host
3. have one admin email address ready
4. choose an email provider and object storage provider
5. authenticate GitHub CLI if the agent will create repos
6. give the agent access to:
   - this public repo for discovery
   - the private instance repo for deployment
   - SSH access to the host
   - the required provider credentials
7. let the agent verify the private instance repo protections before production deploys:
   - default-branch merge protection
   - production environment reviewers
   - least-privilege deployment secrets

That is the intended handoff boundary. The agent should do the repo setup,
deployment, validation, extension install, and follow-up operations from there.

## What to Tell the Agent

Use one of these prompts.

### Prompt: Deploy my first Move Big Rocks instance

```text
Open this Move Big Rocks repo and read README.md, START_WITH_AN_AGENT.md, docs/CUSTOMER_INSTANCE_SETUP.md, docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md, docs/AGENT_CLI.md, and docs/AGENT_RECIPES.md.

Help me create and configure a private Move Big Rocks instance repo based on MoveBigRocks/instance-template, then deploy my first Move Big Rocks instance to a fresh Linux host. Keep secrets out of tracked files, verify the repo and deployment-environment protections before production rollout, tell me exactly which accounts, credentials, DNS records, and SSH access you need from me, and do the rest step by step.
```

### Prompt: Spin up a Move Big Rocks sandbox

```text
Open this Move Big Rocks repo and help me spin up a sandbox so I can evaluate the product quickly. Use the sandbox path rather than a full production deployment. Make sure I get a browser URL, login path, and any `mbr` bootstrap steps I need. Seed or verify demo data so I can explore teams, queues, conversations, knowledge, concept specs, and extensions with an agent.
```

### Prompt: Install an extension into my instance

```text
Open my Move Big Rocks instance repo and help me install, configure, validate, and activate an extension safely. Use the current `mbr` CLI and the documented lifecycle. Test it in a sandbox workspace on the live instance before activating it in production.
```

### Prompt: Build me a private custom extension

```text
Open this Move Big Rocks repo and help me decide whether my requirement belongs in the instance repo or in a separate custom extension repo. If it needs a real extension, help me create a private extension repo based on MoveBigRocks/extension-sdk, implement the extension, run it locally, test it, threat-model it, package it, and install it into my Move Big Rocks instance safely.
```

### Prompt: Connect Move Big Rocks to my agent runtime

```text
Open this Move Big Rocks repo and help me connect my local OpenClaw setup, Perplexity Personal Computer, or another supported always-on agent runtime to Move Big Rocks in a way that is simple, secure, and fully tracked in Move Big Rocks. Keep the runtime under my control, and make sure Move Big Rocks remains the system of record for cases, conversations, forms, extensions, approvals, and audit.
```

### Prompt: Build me an ATS setup

```text
Open this Move Big Rocks repo and help me deploy a Move Big Rocks instance, then set up an ATS workflow with a careers site, application forms, case routing, and a review workflow. Use the existing Move Big Rocks primitives and extension model. If a first-party ATS pack is available, use it. If not, help me build the required custom extension safely.
```

## What the Agent Should Read First

If the agent is in the public core repo, it should start with:

1. [README.md](https://github.com/MoveBigRocks/platform/blob/main/README.md)
2. [docs/CUSTOMER_INSTANCE_SETUP.md](https://github.com/MoveBigRocks/platform/blob/main/docs/CUSTOMER_INSTANCE_SETUP.md)
3. [docs/AGENT_CLI.md](https://github.com/MoveBigRocks/platform/blob/main/docs/AGENT_CLI.md)
4. [docs/AGENT_RECIPES.md](https://github.com/MoveBigRocks/platform/blob/main/docs/AGENT_RECIPES.md)
5. [docs/RFCs/RFC-0007-agent-native-knowledge-and-forms.md](https://github.com/MoveBigRocks/platform/blob/main/docs/RFCs/RFC-0007-agent-native-knowledge-and-forms.md)
6. [docs/RFCs/RFC-0008-openclaw-local-agent-connector.md](https://github.com/MoveBigRocks/platform/blob/main/docs/RFCs/RFC-0008-openclaw-local-agent-connector.md)
7. [docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md](https://github.com/MoveBigRocks/platform/blob/main/docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md) when deciding whether a task belongs in the instance repo or a custom extension repo
8. [MoveBigRocks/extension-sdk](https://github.com/MoveBigRocks/extension-sdk) when the user is building a custom extension

Then it should move into the private instance repo as soon as deployment or
operations work begins.

If the agent is already in the private instance repo, it should start with:

1. `START_HERE.md`
2. `mbr.instance.yaml`
3. `extensions/desired-state.yaml`

## What Inputs the Agent Will Usually Need

To spin up a sandbox, the agent should usually need only:

- an email address for the initial admin login
- a preferred trial name or workspace name

The sandbox path should also let the agent:

- evaluate first-party extensions without separate purchase during the sandbox window
- export the sandbox data cleanly before expiry
- create and control the sandbox primarily through `mbr`, not a separate web provisioning flow
- take one short instruction from the user and handle the rest with minimal follow-up

The intended sandbox agent experience is:

- user says something as simple as `create me a Move Big Rocks sandbox`
- agent authenticates if needed
- agent runs the `mbr sandboxes create ...` flow
- agent gets the ready sandbox URL back directly from that command, without manual verification or polling
- agent returns the sandbox URL, expiry time, login path, and suggested next steps
- the user is not asked for production-only inputs such as DNS, SSH, or provider credentials

To deploy a first instance, the agent will usually ask for:

- a Linux host or VPS
- SSH access
- a domain
- an admin email
- email provider credentials
- object storage credentials
- registry or license information if installing paid extensions

## The Product Bar

The product is working the way we want only if this feels natural:

- a user opens Claude Code, Codex, OpenClaw, or another capable agent host
- points it at the Move Big Rocks repo
- understands immediately that Move Big Rocks is a team-aware operations platform
- can spin up a sandbox quickly if they are not ready for production setup
- gets guided to a working Move Big Rocks instance
- installs an extension, connects OpenClaw, or builds a custom one with the same agent

If the experience depends on hidden setup knowledge or too many manual steps,
the docs or product surface need more work.
