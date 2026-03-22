# Customer FAQ

## Do I fork the main Move Big Rocks repo to deploy production?

No. The normal production path is a private instance repo created from the
public instance template. The core repo publishes releases. The instance repo
deploys them.

## What is a private instance repo in plain English?

It is the private deployment-and-operations repo for one live Move Big Rocks
installation. It holds pinned versions, deployment workflows, branding,
extension config, and secret wiring. It is not a private copy of the Move Big Rocks
source tree.

## Why is the instance repo private if the core repo is public?

Because the instance repo contains environment wiring, deployment targets,
operational policy, and references to secrets. The product code can be public
without making the live deployment control plane public.

## What goes in `mbr.instance.yaml` versus GitHub secrets?

Put non-secret desired state in `mbr.instance.yaml`:

- host
- domains
- cookie scope
- pinned core artifact refs
- storage provider and bucket names
- email provider and sender identity

Put only sensitive values in GitHub secrets:

- deploy SSH key
- JWT secret
- metrics token
- Postmark, SMTP, SES, or storage credentials
- private registry credentials

## Do I need a separate repo for my custom extension?

Usually yes. Keep custom extension source in its own repo. Only tiny local
overrides belong in the instance repo.

## How many repos do I actually need on day one?

Usually one private instance repo.

You only need a second private repo if you are building a custom extension with
real logic. If you are just deploying core and installing first-party
extensions, do not create an extra repo.

## Can my custom extension stay private forever?

Yes. Public source is optional. Marketplace publication is opt-in.

## What if I never buy an extension?

That is supported. Core Move Big Rocks is useful on its own as a support and operations
foundation for multiple teams.

## Can I try Move Big Rocks without self-hosting it first?

Yes. The intended evaluation path is to spin up a hosted sandbox.

The sandbox model should be:

- disposable and time-limited
- 5 days free by default
- extendable for 30 more days for $50
- reachable on an auto-generated subdomain such as `magic-dumpling-26.movebigrocks.io`
- seeded with demo data
- created and managed through `mbr`, then usable through the browser and the same contract as a real instance
- safe for evaluation, extension trials, and agent-led exploration
- inclusive of first-party extension access in sandbox mode during the trial window
- easy to export before deletion
- easy for an agent to create from one short user instruction through `mbr`

Production should still move to a real self-hosted instance repo, but the
sandbox should be the fastest way to experience the product.

## Do I have to buy extensions separately in a sandbox?

No. The intended sandbox model includes first-party extension access for
evaluation during the sandbox window.

That means a sandbox user should be able to try things like:

- ATS
- web analytics
- error tracking
- operational health / observability

Production licensing remains separate. Sandbox access is for evaluation, not
for indefinite hosted use.

## Do I need OpenClaw to use Move Big Rocks?

No. OpenClaw is optional.

Move Big Rocks is useful on its own as an operations center for cases, knowledge,
forms, automation, and extensions.

## Can teams keep some knowledge private and publish other knowledge?

Yes. That should be a core Move Big Rocks capability.

The intended model is:

- each team can keep knowledge visible only to itself
- teams can share selected knowledge with named peer teams
- the workspace can also hold shared cross-team knowledge

Knowledge should be Markdown-first, searchable, citable, and versioned. Git is
a good implementation detail for history and sync, but the product should feel
like draft, review, publish, and share rather than raw Git plumbing.

## What does OpenClaw add if I connect it?

The intended model is:

- OpenClaw helps you use Move Big Rocks
- OpenClaw stays local to your machine
- Move Big Rocks remains the system of record

That means OpenClaw can help with:

- setup and deployment guidance
- case assistance
- knowledge retrieval
- forms draft preparation
- extension authoring and operational help

Move Big Rocks owns:

- cases and records
- extension install and activation
- permissions
- approvals
- audit trails

## Can Move Big Rocks power a website chat widget or in-app support chat?

Yes. That should be a core Move Big Rocks capability.

The intended model is:

- customers talk to Move Big Rocks through a website widget or app chat surface
- Move Big Rocks uses knowledge and policy during the conversation
- Move Big Rocks can fill forms on the customer's behalf when appropriate
- Move Big Rocks can escalate the conversation into a case when operational follow-through is needed

That conversation is not just a case and not just a chatbot transcript. It is a
supervised conversation inside Move Big Rocks.

## Does that mean customers would talk directly to OpenClaw?

No.

If OpenClaw is connected, it should participate behind Move Big Rocks as a supervised
local delegate. The public chat surface should connect to Move Big Rocks, not directly
to a local OpenClaw runtime.

## Can another agent host such as Claude Code or Codex operate Move Big Rocks too?

Yes. That is a core product rule.

Any capable agent host that can use the current `mbr` CLI, GraphQL API, or a thin
adapter over the same contract can operate Move Big Rocks. OpenClaw is one optional
connector, not the only agent story.

## Can teams create their own extensions?

Yes.

The intended model is:

- a team can author and own a private extension repo
- the extension can seed team-owned knowledge, queues, routes, and workflows
- the runtime install scope still stays `workspace` or `instance`

That gives teams room to build their own operational packs without inventing a
separate trust model for every team.

## What about always-on agent runtimes beyond OpenClaw?

Move Big Rocks should support those through one shared connector model.

OpenClaw should be the easiest out-of-the-box provider, but the product should
also fit other long-running agent hosts and personal agent runtimes through the
same `agent runtime` connector contract rather than one-off integrations.

## Can an agent set this up for me?

Yes. That is the intended model. The agent should open the private instance
repo, read `agents/bootstrap.md`, wire secrets, deploy the pinned release, and
then install optional extensions.

If you are not ready for a production deployment yet, the agent should also be
able to spin up a sandbox first and use that as the trial environment.

The intended experience is that you can tell Codex or Claude Code something
like "create me a Move Big Rocks sandbox" and the agent can handle the rest
through the CLI with only minimal follow-up.

## What repo should I give to Codex or Claude Code?

Give the agent the private instance repo for deployment and operations tasks.
Give it the core repo only when you want to change Move Big Rocks itself.

## Can I point the agent at one markdown file and let it start?

Yes.

Use:

- [START_WITH_AN_AGENT.md](https://github.com/movebigrocks/platform/blob/main/START_WITH_AN_AGENT.md) in the public core repo for discovery
- `START_HERE.md` in the private instance repo for deployment and operations

## How does the instance repo know what to deploy?

The repo reads `mbr.instance.yaml` through `scripts/read-instance-config.sh`.
That file is the canonical non-secret contract for hostnames, release refs,
providers, and buckets.

## Do I need to run my own mail server?

No. Move Big Rocks recommends Postmark and supports generic SMTP for outbound relay.
Move Big Rocks is not positioned as a full mail server.

For the concrete outbound and inbound Postmark flow, see [Postmark Setup](https://github.com/movebigrocks/platform/blob/main/docs/POSTMARK_SETUP.md).

## Can I use AWS SES?

Yes. The email layer stays provider-adapter based so SES and other vendors fit
without redesigning the domain model.

## How do paid extensions work?

The customer buys them outside Move Big Rocks, receives a license grant or registry
entitlement, and then installs a signed bundle into the running instance.

## Will I receive source code for paid extensions?

Not by default. First-party commercial extensions normally ship as signed
bundles. Customer-built extensions are separate and can stay private.

## What if I want enterprise SSO?

That is the first-party `enterprise-access` identity extension. Core owns
users, sessions, memberships, and authorization.

## Are analytics and error tracking part of core?

No. They are optional first-party extensions. Internal health and audit logging
stay in core, but customer-facing analytics and error-tracking experiences are
not mandatory core product layers.

## What if I want to build a vibe-coded app on top of Move Big Rocks?

If it needs Move Big Rocks primitives, Move Big Rocks auth, or Move Big Rocks-hosted public routes,
build it as an extension. If it is really its own service, keep it in a
separate repo and integrate with Move Big Rocks.

This is meant to be a real choice, not a fallback. You should be able to use
an off-the-shelf pack when one exists, or build your own private extension on
the same primitives when you want something custom. That includes team-owned
private extensions.

## How long should initial setup take?

The target is roughly thirty minutes for a fresh Linux host when the customer
already has DNS, SSH access, and provider credentials ready.

## Do I have to recreate GitHub secrets when upgrading Move Big Rocks?

No. Version upgrades normally only change the pinned release refs in the
private instance repo. Secrets remain in the repo or environment settings
unless you are rotating them.

## Can I keep using the same instance repo for years?

Yes. That is the point. The instance repo is the durable control plane for that
installation.

## Can I have more than one instance repo?

Yes. Each instance repo describes one live installation.

Typical examples:

- one private repo for production
- one private repo for an internal demo
- one private repo for a second customer or business unit
