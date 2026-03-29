# Extension Security Model

**Document version:** 2026-03-13  
**Status:** Active security and trust policy

## Purpose

This document defines which extensions are allowed, how they are reviewed, and how Move Big Rocks should help users ship secure custom software.

The product promise is not just "you can deploy extensions". The promise is:

- you can understand what is trusted
- you can see which repos own which parts of the system
- you can build custom extensions without silently weakening your production host

## Trust Classes

Move Big Rocks should treat extensions as different trust classes, not one flat bucket.

### First-party extensions

Examples:

- `ats`
- `enterprise-access`
- `error-tracking`
- `web-analytics`

Rules:

- source can stay private
- bundles must be signed
- release pipeline is controlled by the Move Big Rocks first-party team
- standard-risk first-party bundles can be published publicly for free
- privileged capabilities are allowed only through first-party reviewed paths in Milestone 1

### Customer-built private extensions

Examples:

- custom forms portal
- internal operations dashboard
- branded careers-site customisation
- workflow extension for a single client

Rules:

- private by default
- can stay private forever
- should live in separate extension repos
- should install into Move Big Rocks as signed bundles
- should be limited to standard-risk extension classes in the generic runtime

### Marketplace extensions

Later, not the default Milestone 1 path.

Rules:

- publication is optional
- source can be public or private
- bundle review is mandatory
- privileged categories need stronger review than ordinary product extensions

## Repo Ownership Matrix

Use this split consistently:

- **Public core repo**
  Shared runtime, releases, docs, and public extension interfaces.
- **Private instance repo**
  Desired state for one live installation.
- **Private or public extension repo**
  Source code for a custom extension.
- **First-party extensions repo**
  Source code for official first-party extensions.

Do not put customer extension source code into the private instance repo unless it is a tiny local-only override.

## Current Runtime Policy

The current generic extension runtime is intentionally narrower than the long-term marketplace model.

Today, the core implementation supports installation only for:

- `scope: workspace`
- `risk: standard`
- `kind: product` or `kind: operational`

The generic runtime does **not** currently install:

- `scope: instance`
- `risk: privileged`
- `kind: identity`
- `kind: connector`

Reason:

- identity and connector extensions touch authentication, external credentials, or transport boundaries
- those categories need stronger review, richer isolation, and more explicit lifecycle controls than the current generic slice provides

This means:

- self-built ATS-like or workflow-like extensions are in scope
- enterprise SSO, Slack connectors, WhatsApp connectors, and similar privileged integrations remain first-party-only paths until the privileged runtime is fully dogfooded and opened up beyond trusted first-party publishers

## Security Rules for Self-Built Extensions

Every self-built extension should follow these rules:

1. **Least privilege**
   Request only the core permissions the extension needs.
2. **Workspace-scoped by default**
   Do not make custom extensions instance-scoped unless there is a clear security and product reason.
3. **No hidden network trust**
   External calls, webhooks, and third-party APIs must be explicit in the extension spec and threat model.
4. **No secret sprawl**
   Secrets belong in instance-repo secret management, not in source files, manifests, or tracked config.
5. **Public routes are treated as hostile input**
   Any public extension website or form must assume spam, malformed input, upload abuse, and replay attempts.
6. **Customizable templates are content, not code execution**
   Treat template overrides as data and reviewed content changes, not as a backdoor to arbitrary execution.
7. **Auditability**
   Installation, activation, configuration, and asset changes must be attributable to a user or agent.
8. **Use sanctioned core action paths**
   Extensions should request shared actions such as case creation, replies, or contact updates through documented core interfaces and event flows rather than bypassing core boundaries.

## Threat Model Checklist

Before activating a self-built extension, the agent or operator should answer:

1. What data does the extension read and write?
2. Which permissions does it request, and can any be removed?
3. Does it expose public routes or form endpoints?
4. Can it receive file uploads?
5. Does it call external APIs or send data outside the host?
6. Which secrets does it need?
7. Can a template or branding override change logic instead of content?
8. What happens if the extension fails validation or becomes unhealthy?
9. Can a malicious or broken configuration leak workspace data?
10. Does the extension introduce a customer-facing security boundary such as auth, email, or messaging?

If the answer to the last question is yes, the extension should not go through the generic self-built path in Milestone 1.

## Verification Gates

Before activation, a self-built extension should pass these gates:

- manifest validation
- runtime policy validation
- route-to-asset consistency checks
- collection seed validation
- customizable-asset bounds checks
- unit and integration tests in the extension repo
- threat-model review recorded in the instance repo
- sandbox-workspace activation before production activation

The current core implementation already enforces some structural checks:

- manifest shape
- route asset presence
- customization asset presence
- generic runtime policy for allowed scope/risk/kind combinations

The remaining checks should live in the extension authoring workflow and instance repo policy until the CLI automates them.

Extensions should also use the same outbox and event-bus pattern as core whenever they publish or consume domain events. That keeps retries, auditability, and operational visibility consistent across first-party and customer-built extensions.

## Recommended Agent Workflow for Custom Extensions

Use this sequence:

1. Create the extension in a separate repo from core and from the instance repo.
2. Keep the extension `workspace` scoped unless there is a compelling reason otherwise.
3. Limit it to `product` or `operational` kind for Milestone 1.
4. Write the manifest with the smallest possible permission set.
5. Run a threat-model prompt and record the result in the instance repo.
6. Run tests.
7. Build a signed bundle.
8. Install into a sandbox workspace on the live instance.
9. Validate and review the public routes and configuration.
10. Activate in production only after the review gates pass.

## How Move Big Rocks Helps Semi-Technical Users

The value proposition for a semi-technical or non-technical user is:

- Move Big Rocks gives them a secure host and a clear runtime model
- the instance repo gives the agent one predictable place to manage deployment and policy
- the extension runtime gives the agent one predictable way to install and configure custom product extensions
- the trust rules tell them what can be self-built safely and what still needs first-party review

That is the missing layer between "I can generate code" and "I can safely run this in production."

## Milestone 1 Product Promise

Milestone 1 should make the following promise clearly:

- anyone can deploy free core Move Big Rocks and use it as a support and operations foundation
- customers can safely install first-party signed bundles, including a free public bundle set for ATS, error tracking, and web analytics
- public signed bundles can verify publisher trust without an instance-bound token
- customers can also build their own standard-risk workspace extensions
- Move Big Rocks plus the instance repo plus the agent workflow gives them a safer path to production than ad hoc vibe-coded deployment
- privileged auth and connector extensions are intentionally more restricted, not forgotten
