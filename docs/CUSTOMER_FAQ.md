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

Not through a vendor-hosted sandbox right now.

The current evaluation path is to either:

- deploy it on one Ubuntu 22.04+ VPS you control
- run it locally if you are comfortable with a technical setup

The intended agent experience is still simple:

- create the private instance repo from the template
- validate `mbr.instance.yaml`
- wire secrets and deploy the pinned core artifacts
- hand back the app, admin, and API URLs plus the next steps

This keeps evaluation on infrastructure you control from day one.

## How should I test extensions before production?

Use a dedicated preview workspace on your own instance.

That lets you:

- install and validate an extension in a lower-risk workspace first
- monitor it under the real runtime and permission model
- promote it only after the preview pass is clean

Production licensing and trust review still apply. The preview workspace is for
safe evaluation inside an owned instance, not for bypassing review.

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

That gives teams room to build their own operational extensions without inventing a
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

The intended experience is that you can tell Codex or Claude Code something
like "create me a Move Big Rocks instance repo and deploy it to one Ubuntu VPS
I control" and the agent can handle most of the rest with only minimal
follow-up.

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

## Which first-party extensions are free public bundles?

The current free public first-party bundle set is:

- `ats`
- `error-tracking`
- `web-analytics`

They are intended to be distributed as signed public bundles from the public
first-party extensions repo at `MoveBigRocks/extensions`, and install into the
same extension runtime as private custom extensions. Public signed bundles can
install without an instance-bound token; controlled bundle flows can still use
one.

## What about enterprise SSO?

That remains the private first-party `enterprise-access` identity extension. It
is a privileged extension and is not part of the free public bundle set.

## Can I sell Move Big Rocks or sell extensions for it?

Not under the public license.

The public licensing posture is meant to be clear:

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

If you want different commercial rights, that requires a separate agreement.

## Are analytics and error tracking part of core?

No. They are optional first-party extensions. Internal health and audit logging
stay in core, but customer-facing analytics and error-tracking experiences are
not mandatory core product layers.

## What if I want to build a vibe-coded app on top of Move Big Rocks?

If it needs Move Big Rocks primitives, Move Big Rocks auth, or Move Big Rocks-hosted public routes,
build it as an extension. If it is really its own service, keep it in a
separate repo and integrate with Move Big Rocks.

This is meant to be a real choice, not a fallback. You should be able to use
an off-the-shelf extension when one exists, or build your own private extension on
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
