# Customer Onboarding Standard

This document defines the bar for a clear Move Big Rocks onboarding experience.

## What Must Be Immediately Understandable

A new customer or agent should understand, without extra explanation:

- Move Big Rocks is an operations center
- Move Big Rocks works on its own
- optional capabilities arrive as extensions
- a private instance repo is the deployment control plane
- custom extension code belongs in a separate repo only when needed
- any capable agent host can operate Move Big Rocks through the CLI and GraphQL contract
- OpenClaw is optional and local, not a requirement

## What The Setup Path Must Feel Like

The setup path should feel simple:

1. open the public Move Big Rocks repo
2. understand the product in a few minutes
3. create a private instance repo from the template
4. point an agent or human operator at that repo
5. deploy to one Ubuntu host, ideally as easily as creating a DigitalOcean Droplet
6. verify health, login, email, and the first workspace
7. install extensions only after core is healthy

## Required Product Qualities

The onboarding experience meets the bar when:

- the repo model is obvious
- the CLI covers the promised deployment and operations flows
- extension installation follows one documented lifecycle
- the same product story works for humans and for agents
- the default hosting path is simple without making the product cloud-locked

## Operational Standard

The public core repo is for understanding Move Big Rocks and evolving core.

The private instance repo is for:

- deployment
- pinned versions
- secrets wiring
- domain configuration
- extension desired state
- operational policy

The public core repo is not the live deployment control plane.

## Extension Standard

Optional packs such as ATS, analytics, error tracking, enterprise access, and
connector integrations must all read as extensions, not disguised core
features.

That means:

- extension install and activation are visible in the docs and CLI
- extension-specific workflows do not require hidden core knowledge
- extension authoring follows the same agent-first operating model

## Launch Bar

Move Big Rocks is ready for first-time users when:

- a human can understand the product quickly
- an agent can deploy it from the documented repo model
- a new user can imagine spinning up a DigitalOcean Droplet and trying it
- the docs make that feel inviting rather than intimidating
- the product story stays consistent across README, setup docs, RFCs, ADRs, and lifecycle docs
