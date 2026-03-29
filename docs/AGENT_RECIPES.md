# Agent Recipes

This document gives agents a small set of standard workflows for Move Big Rocks.

These are intended operating recipes.

## Recipe 1: First Move Big Rocks Deployment

Use when the user wants:

- a fresh self-hosted Move Big Rocks instance
- core Move Big Rocks first
- help from Codex, Claude Code, OpenClaw, or another capable agent host

Agent steps:

1. Read [README.md](https://github.com/MoveBigRocks/platform/blob/main/README.md), [START_WITH_AN_AGENT.md](https://github.com/MoveBigRocks/platform/blob/main/START_WITH_AN_AGENT.md), [docs/CUSTOMER_INSTANCE_SETUP.md](https://github.com/MoveBigRocks/platform/blob/main/docs/CUSTOMER_INSTANCE_SETUP.md), and [docs/AGENT_CLI.md](https://github.com/MoveBigRocks/platform/blob/main/docs/AGENT_CLI.md).
2. If using GitHub CLI for repo creation, verify `gh auth status` first.
3. Help the user create a private instance repo from [MoveBigRocks/instance-template](https://github.com/MoveBigRocks/instance-template).
4. Read the instance repo `START_HERE.md`.
5. Validate `mbr.instance.yaml`.
6. Gather the required secrets and host access.
7. Deploy the pinned core release.
8. Verify health and admin access.
9. Set up outbound email.
10. Create the first workspace and first forms flow if needed through the CLI.

Success criteria:

- Move Big Rocks is live
- admin login works
- the CLI can log in
- the instance repo reflects the deployed desired state

## Recipe 2: Install a First-Party Extension

Use when the user wants:

- ATS
- web analytics
- error tracking
- another first-party extension

Agent steps:

1. Open the private instance repo.
2. Read `START_HERE.md` and `extensions/desired-state.yaml`.
3. Confirm the user has the required license or registry access.
4. Install the extension with the CLI.
5. Validate signature and license behavior.
6. Configure the extension.
7. Activate it in a sandbox workspace first when appropriate.
8. Run `mbr extensions monitor`.
9. Only then activate it in production.

Success criteria:

- the extension is installed
- validation passes
- health checks pass
- the expected user-facing workflow is live

## Recipe 3: Build a Private Custom Extension

Use when the user wants:

- custom operational logic
- custom routes or admin workflows
- a new internal tool on top of Move Big Rocks primitives

Agent steps:

1. Decide whether the request belongs in the instance repo or a real extension repo.
2. If using GitHub CLI for repo creation, verify `gh auth status` first.
3. If it needs real custom logic, create a separate private extension repo.
4. Start from [MoveBigRocks/extension-sdk](https://github.com/MoveBigRocks/extension-sdk).
5. Implement the extension.
6. Run it locally against a local Move Big Rocks instance.
7. Install from the source directory during development.
8. Validate the manifest and runtime health.
9. Run the threat model and review checklist.
10. Package the extension.
11. Install and activate it in a safe environment before production.

Success criteria:

- the extension runs locally
- the extension can be installed through the documented lifecycle
- the user can operate it through the instance repo and CLI

## Recipe 4: ATS Setup

Use when the user wants:

- a careers site
- applicant form
- candidate review workflows

Agent steps:

1. Deploy Move Big Rocks core if it is not already live.
2. Check whether the first-party ATS extension is available.
3. If available, install and configure it.
4. If not, build the required custom extension on top of:
   - form specs
   - cases
   - contacts
   - labels or collections
   - automation
   - public routes
5. Configure the careers site branding and forms flow.
6. Verify application submission, attachment handling, and candidate case creation.

Success criteria:

- a careers site is live
- applications create the expected records and workflows
- the reviewing workspace is usable

## Recipe 5: Forms and Capture

Use when the user wants:

- lead capture
- support form
- internal request flows
- a default example to learn from

Agent steps:

1. Inspect existing forms flows in the target workspace.
2. If the instance is fresh, create a simple demo forms flow first.
3. Verify the submission flow creates the expected case or event.
4. Add labels, automation rules, or routing rules as needed.
5. Show the user how to edit or duplicate the forms flow.

Success criteria:

- there is at least one working example forms flow
- submission is observable
- automation and routing behave as expected

## Recipe 6: Configure Analytics or Error Tracking

Use when the user wants:

- web analytics installed
- Sentry-compatible error ingest
- a safe extension-first operational setup

Agent steps:

1. Confirm the extension is licensed or otherwise available.
2. Install it through the CLI.
3. Configure required domain, token, or endpoint settings.
4. Activate it in the appropriate test environment when possible.
5. Verify ingest and admin views.
6. Run monitoring and health checks.

Success criteria:

- the extension is healthy
- the expected ingest path works
- the expected admin views are reachable

## Recipe 7: Connect OpenClaw

Use when the user wants:

- to connect a local OpenClaw setup to Move Big Rocks
- local-agent help for cases, forms, setup, or extension work
- a simple optional local-agent workflow without making OpenClaw mandatory

Agent steps:

1. Read [README.md](https://github.com/MoveBigRocks/platform/blob/main/README.md), [START_WITH_AN_AGENT.md](https://github.com/MoveBigRocks/platform/blob/main/START_WITH_AN_AGENT.md), [docs/AGENT_CLI.md](https://github.com/MoveBigRocks/platform/blob/main/docs/AGENT_CLI.md), and [docs/RFCs/RFC-0008-openclaw-local-agent-connector.md](https://github.com/MoveBigRocks/platform/blob/main/docs/RFCs/RFC-0008-openclaw-local-agent-connector.md).
2. Confirm the user wants OpenClaw to remain local and optional.
3. Link the local OpenClaw setup through the documented CLI flow.
4. Verify the local bridge can connect back to Move Big Rocks safely.
5. Test one case-assist or operator-assist task end to end.
6. Confirm all results and proposed actions are visible and tracked in Move Big Rocks.

Success criteria:

- Move Big Rocks works with or without the connector
- OpenClaw can help from the user's machine
- meaningful actions remain controlled and audited in Move Big Rocks

## Recipe 8: Launch Customer Chat

Use when the user wants:

- a website chat widget
- mobile in-app support chat
- customer conversations that can use knowledge and forms before escalating into cases

Agent steps:

1. Read [README.md](https://github.com/MoveBigRocks/platform/blob/main/README.md), [docs/RFCs/RFC-0007-agent-native-knowledge-and-forms.md](https://github.com/MoveBigRocks/platform/blob/main/docs/RFCs/RFC-0007-agent-native-knowledge-and-forms.md), and [docs/RFCs/RFC-0009-supervised-conversation-sessions-and-chat-surfaces.md](https://github.com/MoveBigRocks/platform/blob/main/docs/RFCs/RFC-0009-supervised-conversation-sessions-and-chat-surfaces.md).
2. Define the knowledge resources and conversation policy that the chat surface should follow.
3. Define the form specs that the conversation is allowed to fill or submit.
4. Configure the website widget or mobile client to connect to Move Big Rocks rather than directly to a model runtime.
5. If OpenClaw is connected, verify it participates only through Move Big Rocks's supervised connector path.
6. Test the full flow: resolve in chat, prepare forms, and escalate to case.

Success criteria:

- customer chat is visible in Move Big Rocks as conversation sessions
- knowledge and forms are used during the conversation
- escalation into cases is controlled and auditable
- Move Big Rocks remains the system of record whether or not OpenClaw is involved

## Recipe Rule

If an agent cannot complete one of these recipes without inventing
undocumented steps, the docs or product surface need to be tightened.
