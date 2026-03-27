# Agent CLI

This document defines the supported machine interface for Move Big Rocks.

Move Big Rocks is operable by any capable agent host through one consistent machine
contract.

Examples:

- Claude Code
- Codex
- OpenClaw
- Perplexity Personal Computer or similar agent-host runtimes
- other hosts that can call CLI commands, GraphQL, or a thin adapter over the same contract

## Principles

- The command-line interface is the primary operator and agent entry point.
- GraphQL is the canonical backend API.
- Every command supports `--json`.
- Write operations use idempotency keys where required by the backend.
- Exit codes are stable and machine-friendly.
- Optional agent-runtime connectors use the same underlying contract rather than inventing a second semantics layer.
- If Move Big Rocks exposes MCP, it is an adapter over this same contract, not a separate product surface.

## Authentication

Move Big Rocks supports two practical auth modes:

- **Interactive operator login**
  `mbr auth login` opens the browser when no token is supplied, completes
  login against the admin surface, and stores a session-backed CLI login
  locally.
- **Automation and installed extensions**
  Workspace-scoped `hat_*` agent tokens authenticate non-interactive workloads.

The CLI supports:

- `mbr auth whoami`
- `mbr auth login`
- `mbr auth logout`
- direct token use through `--token` or `MBR_TOKEN`
- local persistence of the Move Big Rocks URL and admin base URL
- OS credential-store-backed interactive credentials on supported systems, with secure file fallback when a credential store is unavailable

That means an external agent host usually operates Move Big Rocks in one of two ways:

- shelling out to `mbr ... --json`
- calling Move Big Rocks GraphQL directly when needed

Optional agent-runtime connectors such as OpenClaw route meaningful work through
one of those two shapes.

## Runtime Discovery

Once `mbr` is installed or a runtime URL is known, an external agent should not
need to scrape HTML to discover how to proceed.

The platform discovery model is:

- create or select a runtime you control
- query the runtime bootstrap endpoint such as `https://<runtime>/.well-known/mbr-instance.json`
- continue through `mbr` and GraphQL

Public marketing pages, install guides, and other site-owned discovery flows
may point agents at the runtime contract, but those surfaces live outside the
platform repo. The authoritative machine-readable contract defined here starts
at the runtime bootstrap endpoint.

## Command Surface

<!-- BEGIN GENERATED CLI COMMAND SURFACE -->
> Generated from `internal/clispec`. Run `go run ./cmd/tools/sync-agent-cli-doc` to regenerate.

```text
mbr auth whoami [--url URL] [--token TOKEN] [--json]
mbr auth login --url URL [--token TOKEN | --token-stdin] [--json]
mbr auth logout
mbr context view [--json]
mbr context set [--workspace WORKSPACE_ID] [--team TEAM_ID] [--clear-team | --clear] [--json]
mbr spec export [--json]
mbr workspaces list [--url URL] [--token TOKEN] [--json]
mbr workspaces create --name NAME --slug SLUG [--description TEXT] [--url URL] [--json]
mbr teams list [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr teams create [--workspace WORKSPACE_ID] --name NAME [--description TEXT] [--email ADDRESS] [--response-hours N] [--resolution-hours N] [--auto-assign] [--auto-assign-keywords CSV] [--inactive] [--url URL] [--token TOKEN] [--json]
mbr teams show TEAM_ID [--url URL] [--token TOKEN] [--json]
mbr teams members list [--team TEAM_ID] [--url URL] [--token TOKEN] [--json]
mbr agents list [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr agents show AGENT_ID [--url URL] [--token TOKEN] [--json]
mbr agents create [--workspace WORKSPACE_ID] --name NAME [--description TEXT] [--url URL] [--token TOKEN] [--json]
mbr agents update AGENT_ID [--name NAME] [--description TEXT] [--url URL] [--token TOKEN] [--json]
mbr agents suspend AGENT_ID --reason TEXT [--url URL] [--token TOKEN] [--json]
mbr agents activate AGENT_ID [--url URL] [--token TOKEN] [--json]
mbr agents revoke AGENT_ID --reason TEXT [--url URL] [--token TOKEN] [--json]
mbr agents tokens list AGENT_ID [--url URL] [--token TOKEN] [--json]
mbr agents tokens create AGENT_ID --name NAME [--expires-in-days N] [--url URL] [--token TOKEN] [--json]
mbr agents tokens revoke TOKEN_ID [--url URL] [--token TOKEN] [--json]
mbr agents memberships show AGENT_ID [--url URL] [--token TOKEN] [--json]
mbr agents memberships grant AGENT_ID [--workspace WORKSPACE_ID] [--role ROLE] [--permissions CSV] [--expires-in-days N] [--input-file PATH | --input-json JSON] [--allowed-ips CSV] [--allowed-projects CSV] [--allowed-teams CSV] [--allow-delegated-routing[=BOOL]] [--delegated-routing-teams CSV] [--active-hours-start HH:MM] [--active-hours-end HH:MM] [--active-timezone TZ] [--active-days CSV] [--rate-limit-per-minute N] [--rate-limit-per-hour N] [--url URL] [--token TOKEN] [--json]
mbr agents memberships revoke MEMBERSHIP_ID [--url URL] [--token TOKEN] [--json]
mbr catalog list [--workspace WORKSPACE_ID] [--parent NODE_ID] [--url URL] [--token TOKEN] [--json]
mbr catalog show NODE_ID_OR_PATH [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr forms specs list [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr forms specs show SPEC_ID_OR_SLUG [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr forms specs create [--workspace WORKSPACE_ID] (--input-file PATH | --input-json JSON) [--url URL] [--token TOKEN] [--json]
mbr forms specs update SPEC_ID_OR_SLUG [--workspace WORKSPACE_ID] (--input-file PATH | --input-json JSON) [--url URL] [--token TOKEN] [--json]
mbr forms submissions list [--workspace WORKSPACE_ID] [--spec SPEC_ID] [--status STATUS] [--limit N] [--offset N] [--url URL] [--token TOKEN] [--json]
mbr forms submissions create SPEC_ID_OR_SLUG [--workspace WORKSPACE_ID] (--input-file PATH | --input-json JSON) [--url URL] [--token TOKEN] [--json]
mbr forms submissions show SUBMISSION_ID [--url URL] [--token TOKEN] [--json]
mbr teams members add [--team TEAM_ID] --user USER_ID [--role ROLE] [--url URL] [--token TOKEN] [--json]
mbr knowledge list [--workspace WORKSPACE_ID] [--team TEAM_ID] [--surface SURFACE] [--review-status STATUS] [--kind KIND] [--status STATUS] [--search QUERY] [--limit N] [--url URL] [--token TOKEN] [--json]
mbr knowledge show RESOURCE [--workspace WORKSPACE_ID --team TEAM_ID --surface SURFACE] [--url URL] [--token TOKEN] [--json]
mbr knowledge search QUERY [--workspace WORKSPACE_ID] [--team TEAM_ID] [--surface SURFACE] [--review-status STATUS] [--kind KIND] [--status STATUS] [--limit N] [--url URL] [--token TOKEN] [--json]
mbr knowledge review-queue [--workspace WORKSPACE_ID] [--team TEAM_ID] [--kind KIND] [--review-status STATUS] [--limit N] [--url URL] [--token TOKEN] [--json]
mbr knowledge upsert [RESOURCE] [--workspace WORKSPACE_ID] [--team TEAM_ID] --slug SLUG [--surface SURFACE] [--title TITLE] [--kind KIND] [--concept-spec KEY] [--concept-version VERSION] [--status STATUS] [--summary TEXT] [--body TEXT | --file PATH] [--source-kind KIND] [--source-ref REF] [--path-ref PATH] [--channels CSV] [--share-with CSV] [--keywords CSV] [--frontmatter-file PATH | --frontmatter-json JSON] [--url URL] [--token TOKEN] [--json]
mbr knowledge checkout PATH [--workspace WORKSPACE_ID] [--team TEAM_ID] [--surface SURFACE] [--kind KIND] [--review-status STATUS] [--status STATUS] [--search QUERY] [--url URL] [--token TOKEN] [--json]
mbr knowledge status [PATH] [--url URL] [--token TOKEN] [--json]
mbr knowledge pull [PATH] [--url URL] [--token TOKEN] [--json]
mbr knowledge push [PATH] [--url URL] [--token TOKEN] [--json]
mbr knowledge delete RESOURCE [--workspace WORKSPACE_ID --team TEAM_ID --surface SURFACE] [--url URL] [--token TOKEN] [--json]
mbr knowledge sync PATH [--workspace WORKSPACE_ID] [--team TEAM_ID] [--surface SURFACE] [--kind KIND] [--concept-spec KEY] [--concept-version VERSION] [--status STATUS] [--review-status STATUS] [--share-with TEAM_IDS] [--source-kind KIND] [--source-ref REF] [--url URL] [--token TOKEN] [--json]
mbr knowledge import PATH [--workspace WORKSPACE_ID] [--team TEAM_ID] [--surface SURFACE] [--kind KIND] [--concept-spec KEY] [--concept-version VERSION] [--status STATUS] [--review-status STATUS] [--share-with TEAM_IDS] [--source-kind KIND] [--source-ref REF] [--mode preview|apply] [--url URL] [--token TOKEN] [--json]
mbr knowledge review RESOURCE [--workspace WORKSPACE_ID --team TEAM_ID --surface SURFACE] [--status STATUS] [--url URL] [--token TOKEN] [--json]
mbr knowledge publish RESOURCE [--workspace WORKSPACE_ID --team TEAM_ID --surface SURFACE] [--to-surface SURFACE] [--url URL] [--token TOKEN] [--json]
mbr knowledge share RESOURCE [--workspace WORKSPACE_ID --team TEAM_ID --surface SURFACE] --share-with TEAM_IDS [--url URL] [--token TOKEN] [--json]
mbr knowledge history RESOURCE [--workspace WORKSPACE_ID --team TEAM_ID --surface SURFACE] [--limit N] [--url URL] [--token TOKEN] [--json]
mbr knowledge diff RESOURCE [--workspace WORKSPACE_ID --team TEAM_ID --surface SURFACE] [--from REVISION] [--to REVISION] [--url URL] [--token TOKEN] [--json]
mbr concepts list [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr concepts show KEY [--workspace WORKSPACE_ID] [--version VERSION] [--url URL] [--token TOKEN] [--json]
mbr concepts register FILE [--workspace WORKSPACE_ID] [--team TEAM_ID] [--source-kind KIND] [--source-ref REF] [--status STATUS] [--url URL] [--token TOKEN] [--json]
mbr concepts history KEY [--workspace WORKSPACE_ID] [--version VERSION] [--limit N] [--url URL] [--token TOKEN] [--json]
mbr concepts diff KEY [--workspace WORKSPACE_ID] [--version VERSION] [--from REVISION] [--to REVISION] [--url URL] [--token TOKEN] [--json]
mbr artifacts list --extension EXTENSION_ID --surface SURFACE [--url URL] [--token TOKEN] [--json]
mbr artifacts show --extension EXTENSION_ID --surface SURFACE --path PATH [--ref REVISION] [--url URL] [--token TOKEN] [--json]
mbr artifacts history --extension EXTENSION_ID --surface SURFACE --path PATH [--limit N] [--url URL] [--token TOKEN] [--json]
mbr artifacts diff --extension EXTENSION_ID --surface SURFACE --path PATH [--from REVISION] [--to REVISION] [--url URL] [--token TOKEN] [--json]
mbr artifacts publish --extension EXTENSION_ID --surface SURFACE --path PATH (--file FILE | --content TEXT) [--url URL] [--token TOKEN] [--json]
mbr queues list [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr queues show QUEUE_ID [--url URL] [--token TOKEN] [--json]
mbr queues create [--workspace WORKSPACE_ID] --name NAME [--slug SLUG] [--description TEXT] [--url URL] [--token TOKEN] [--json]
mbr queues items QUEUE_ID [--url URL] [--token TOKEN] [--json]
mbr conversations list [--workspace WORKSPACE_ID] [--status STATUS] [--channel CHANNEL] [--catalog-node NODE_ID] [--contact CONTACT_ID] [--case CASE_ID] [--limit N] [--offset N] [--url URL] [--token TOKEN] [--json]
mbr conversations show SESSION_ID [--url URL] [--token TOKEN] [--json]
mbr conversations reply SESSION_ID [--participant PARTICIPANT_ID] [--role ROLE] [--kind KIND] [--visibility VISIBILITY] (--content TEXT | --file PATH) [--url URL] [--token TOKEN] [--json]
mbr conversations handoff SESSION_ID --queue QUEUE_ID [--team TEAM_ID] [--operator USER_ID] [--reason TEXT] [--url URL] [--token TOKEN] [--json]
mbr conversations escalate SESSION_ID --queue QUEUE_ID [--team TEAM_ID] [--operator USER_ID] [--subject TEXT] [--description TEXT] [--priority PRIORITY] [--category TEXT] [--reason TEXT] [--url URL] [--token TOKEN] [--json]
mbr forms list [--workspace WORKSPACE_ID] [--status STATUS] [--limit N] [--url URL] [--token TOKEN] [--json]
mbr forms create [--workspace WORKSPACE_ID] --name NAME --slug SLUG [--description TEXT] [--definition-file PATH | --definition-json JSON] [--status STATUS] [--public] [--requires-captcha] [--collect-email] [--auto-create-case] [--submission-message TEXT] [--redirect-url URL] [--url URL] [--token TOKEN] [--json]
mbr automation rules list [--workspace WORKSPACE_ID] [--limit N] [--url URL] [--token TOKEN] [--json]
mbr automation rules create [--workspace WORKSPACE_ID] --title TITLE (--conditions-file PATH | --conditions-json JSON) (--actions-file PATH | --actions-json JSON) [--description TEXT] [--active] [--priority N] [--max-per-hour N] [--max-per-day N] [--url URL] [--token TOKEN] [--json]
mbr cases list [--workspace WORKSPACE_ID] [--status STATUS] [--priority PRIORITY] [--queue QUEUE_ID] [--assignee USER_ID] [--limit N] [--url URL] [--token TOKEN] [--json]
mbr cases show CASE_ID [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr cases set-status CASE_ID --status STATUS [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr cases handoff CASE_ID --queue QUEUE_ID [--team TEAM_ID] [--assignee USER_ID] [--reason TEXT] [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr contacts list [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr attachments upload PATH [--workspace WORKSPACE_ID] [--case CASE_ID] [--description TEXT] [--content-type MIME] [--url URL] [--token TOKEN] [--json]
mbr health check [--url URL] [--token TOKEN] [--json]
mbr extensions list ([--workspace WORKSPACE_ID] | --instance) [--url URL] [--token TOKEN] [--json]
mbr extensions lint SOURCE_DIR [--contract PATH] [--write-contract] [--json]
mbr extensions verify SOURCE_DIR [--workspace WORKSPACE_ID] [--license-token TOKEN] [--contract PATH] [--url URL] [--token TOKEN] [--json]
mbr extensions nav ([--workspace WORKSPACE_ID] | --instance) [--url URL] [--token TOKEN] [--json]
mbr extensions widgets ([--workspace WORKSPACE_ID] | --instance) [--url URL] [--token TOKEN] [--json]
mbr extensions show --id EXTENSION_ID [--url URL] [--token TOKEN] [--json]
mbr extensions deploy BUNDLE_SOURCE [--workspace WORKSPACE_ID] [--license-token TOKEN] [--config-file PATH | --config-json JSON] [--no-activate] [--no-monitor] [--url URL] [--token TOKEN] [--json]
mbr extensions monitor --id EXTENSION_ID [--url URL] [--token TOKEN] [--json]
mbr extensions events list [--workspace WORKSPACE_ID] [--url URL] [--token TOKEN] [--json]
mbr extensions skills list --id EXTENSION_ID [--url URL] [--token TOKEN] [--json]
mbr extensions skills show --id EXTENSION_ID --name SKILL_NAME [--url URL] [--token TOKEN] [--json]
mbr extensions install BUNDLE_SOURCE [--workspace WORKSPACE_ID] [--license-token TOKEN] [--url URL] [--token TOKEN] [--json]
mbr extensions upgrade BUNDLE_SOURCE --id EXTENSION_ID [--license-token TOKEN] [--url URL] [--token TOKEN] [--json]
mbr extensions configure --id EXTENSION_ID (--config-file PATH | --config-json JSON) [--url URL] [--token TOKEN] [--json]
mbr extensions validate --id EXTENSION_ID [--url URL] [--token TOKEN] [--json]
mbr extensions activate --id EXTENSION_ID [--url URL] [--token TOKEN] [--json]
mbr extensions deactivate --id EXTENSION_ID [--reason TEXT] [--url URL] [--token TOKEN] [--json]
mbr extensions uninstall --id EXTENSION_ID [--deactivate] [--reason TEXT] [--export-out PATH | --confirm-no-export] [--dry-run] [--url URL] [--token TOKEN] [--json]
```
<!-- END GENERATED CLI COMMAND SURFACE -->

## Auth Matrix

<!-- BEGIN GENERATED CLI AUTH MATRIX -->
> Generated from `internal/clispec`. Run `go run ./cmd/tools/sync-agent-cli-doc` to regenerate.

| Command | Auth | JSON | Operation | Idempotency |
| --- | --- | --- | --- | --- |
| `auth whoami` | Bearer token or browser-backed session | yes | read | read_only |
| `auth login` | No remote auth required | yes | local | local_state |
| `auth logout` | Local CLI state | no | local | local_state |
| `context view` | Local CLI state | yes | local | read_only |
| `context set` | Local CLI state | yes | local | local_state |
| `spec export` | No remote auth required | yes | local | not_applicable |
| `workspaces list` | Bearer token or browser-backed session | yes | read | read_only |
| `workspaces create` | Browser-backed session only | yes | write | server_managed |
| `teams list` | Bearer token or browser-backed session | yes | read | read_only |
| `teams create` | Browser-backed session only | yes | write | server_managed |
| `teams show` | Bearer token or browser-backed session | yes | read | read_only |
| `teams members list` | Bearer token or browser-backed session | yes | read | read_only |
| `agents list` | Bearer token or browser-backed session | yes | read | read_only |
| `agents show` | Bearer token or browser-backed session | yes | read | read_only |
| `agents create` | Browser-backed session only | yes | write | server_managed |
| `agents update` | Browser-backed session only | yes | write | server_managed |
| `agents suspend` | Browser-backed session only | yes | write | server_managed |
| `agents activate` | Browser-backed session only | yes | write | server_managed |
| `agents revoke` | Browser-backed session only | yes | write | server_managed |
| `agents tokens list` | Bearer token or browser-backed session | yes | read | read_only |
| `agents tokens create` | Browser-backed session only | yes | write | server_managed |
| `agents tokens revoke` | Browser-backed session only | yes | write | server_managed |
| `agents memberships show` | Bearer token or browser-backed session | yes | read | read_only |
| `agents memberships grant` | Browser-backed session only | yes | write | server_managed |
| `agents memberships revoke` | Browser-backed session only | yes | write | server_managed |
| `catalog list` | Bearer token or browser-backed session | yes | read | read_only |
| `catalog show` | Bearer token or browser-backed session | yes | read | read_only |
| `forms specs list` | Bearer token or browser-backed session | yes | read | read_only |
| `forms specs show` | Bearer token or browser-backed session | yes | read | read_only |
| `forms specs create` | Bearer token or browser-backed session | yes | write | server_managed |
| `forms specs update` | Bearer token or browser-backed session | yes | write | server_managed |
| `forms submissions list` | Bearer token or browser-backed session | yes | read | read_only |
| `forms submissions create` | Bearer token or browser-backed session | yes | write | server_managed |
| `forms submissions show` | Bearer token or browser-backed session | yes | read | read_only |
| `teams members add` | Browser-backed session only | yes | write | server_managed |
| `knowledge list` | Bearer token or browser-backed session | yes | read | read_only |
| `knowledge show` | Bearer token or browser-backed session | yes | read | read_only |
| `knowledge search` | Bearer token or browser-backed session | yes | read | read_only |
| `knowledge review-queue` | Bearer token or browser-backed session | yes | read | read_only |
| `knowledge upsert` | Bearer token or browser-backed session | yes | write | server_managed |
| `knowledge checkout` | Bearer token or browser-backed session | yes | read | read_only |
| `knowledge status` | Bearer token or browser-backed session | yes | read | read_only |
| `knowledge pull` | Bearer token or browser-backed session | yes | read | read_only |
| `knowledge push` | Bearer token or browser-backed session | yes | write | server_managed |
| `knowledge delete` | Bearer token or browser-backed session | yes | write | server_managed |
| `knowledge sync` | Bearer token or browser-backed session | yes | write | server_managed |
| `knowledge import` | Bearer token or browser-backed session | yes | write | server_managed |
| `knowledge review` | Bearer token or browser-backed session | yes | write | server_managed |
| `knowledge publish` | Bearer token or browser-backed session | yes | write | server_managed |
| `knowledge share` | Bearer token or browser-backed session | yes | write | server_managed |
| `knowledge history` | Bearer token or browser-backed session | yes | read | read_only |
| `knowledge diff` | Bearer token or browser-backed session | yes | read | read_only |
| `concepts list` | Bearer token or browser-backed session | yes | read | read_only |
| `concepts show` | Bearer token or browser-backed session | yes | read | read_only |
| `concepts register` | Browser-backed session only | yes | write | server_managed |
| `concepts history` | Bearer token or browser-backed session | yes | read | read_only |
| `concepts diff` | Bearer token or browser-backed session | yes | read | read_only |
| `artifacts list` | Bearer token or browser-backed session | yes | read | read_only |
| `artifacts show` | Bearer token or browser-backed session | yes | read | read_only |
| `artifacts history` | Bearer token or browser-backed session | yes | read | read_only |
| `artifacts diff` | Bearer token or browser-backed session | yes | read | read_only |
| `artifacts publish` | Bearer token or browser-backed session | yes | write | server_managed |
| `queues list` | Bearer token or browser-backed session | yes | read | read_only |
| `queues show` | Bearer token or browser-backed session | yes | read | read_only |
| `queues create` | Bearer token or browser-backed session | yes | write | server_managed |
| `queues items` | Bearer token or browser-backed session | yes | read | read_only |
| `conversations list` | Bearer token or browser-backed session | yes | read | read_only |
| `conversations show` | Bearer token or browser-backed session | yes | read | read_only |
| `conversations reply` | Bearer token or browser-backed session | yes | write | server_managed |
| `conversations handoff` | Bearer token or browser-backed session | yes | write | server_managed |
| `conversations escalate` | Bearer token or browser-backed session | yes | write | server_managed |
| `forms list` | Browser-backed session only | yes | read | read_only |
| `forms create` | Browser-backed session only | yes | write | server_managed |
| `automation rules list` | Browser-backed session only | yes | read | read_only |
| `automation rules create` | Browser-backed session only | yes | write | server_managed |
| `cases list` | Bearer token or browser-backed session | yes | read | read_only |
| `cases show` | Bearer token or browser-backed session | yes | read | read_only |
| `cases set-status` | Bearer token or browser-backed session | yes | write | server_managed |
| `cases handoff` | Bearer token or browser-backed session | yes | write | server_managed |
| `contacts list` | Bearer token or browser-backed session | yes | read | read_only |
| `attachments upload` | Bearer token or browser-backed session | yes | write | server_managed |
| `health check` | Bearer token or browser-backed session | yes | read | read_only |
| `extensions list` | Bearer token or browser-backed session | yes | read | read_only |
| `extensions lint` | No remote auth required | yes | local | local_state |
| `extensions verify` | Bearer token or browser-backed session | yes | write | server_managed |
| `extensions nav` | Bearer token or browser-backed session | yes | read | read_only |
| `extensions widgets` | Bearer token or browser-backed session | yes | read | read_only |
| `extensions show` | Bearer token or browser-backed session | yes | read | read_only |
| `extensions deploy` | Bearer token or browser-backed session | yes | write | server_managed |
| `extensions monitor` | Bearer token or browser-backed session | yes | read | read_only |
| `extensions events list` | Bearer token or browser-backed session | yes | read | read_only |
| `extensions skills list` | Bearer token or browser-backed session | yes | read | read_only |
| `extensions skills show` | Bearer token or browser-backed session | yes | read | read_only |
| `extensions install` | Bearer token or browser-backed session | yes | write | server_managed |
| `extensions upgrade` | Bearer token or browser-backed session | yes | write | server_managed |
| `extensions configure` | Bearer token or browser-backed session | yes | write | server_managed |
| `extensions validate` | Bearer token or browser-backed session | yes | write | server_managed |
| `extensions activate` | Bearer token or browser-backed session | yes | write | server_managed |
| `extensions deactivate` | Bearer token or browser-backed session | yes | write | server_managed |
| `extensions uninstall` | Bearer token or browser-backed session | yes | write | server_managed |
<!-- END GENERATED CLI AUTH MATRIX -->

## Extension Discovery

Extension-specific operator verbs do not belong in the core CLI. Instead:

- extensions declare their own command catalog in the manifest
- extensions can ship bundled agent-skill Markdown assets
- the core CLI exposes that metadata through `mbr extensions show` and `mbr extensions monitor`
- the core CLI exposes the workspace runtime event catalog through `mbr extensions events list --workspace WORKSPACE_ID`
- agents can list available skills with `mbr extensions skills list --id EXTENSION_ID`
- agents can read bundled skill content with `mbr extensions skills show --id EXTENSION_ID --name SKILL_NAME`
- agents use those extension-declared skills or commands instead of expecting product-specific binaries in core
- core operator flows for forms surfaces remain available through `mbr forms ...` and automation rules through `mbr automation rules ...`

This is what makes Move Big Rocks legible to agents:

- the core CLI stays stable
- the machine contract is inspectable
- extensions publish additional context instead of forcing hardcoded binaries

`mbr extensions monitor --id EXTENSION_ID` performs a runtime health refresh
before returning extension detail. For service-backed extensions, health is
driven by manifest-declared `health` endpoints.

`mbr extensions show` and `mbr extensions monitor` return runtime
diagnostics for service-backed endpoints, scheduled jobs, and event consumers,
including status, failure counts, timestamps, bootstrap status, and the latest
error when one exists.

`mbr extensions events list --workspace WORKSPACE_ID` returns the runtime
event catalog for a workspace, including core event types,
extension-published event types, and which active extensions publish or
subscribe to each event.

## Transport

The CLI talks to:

- `POST /graphql` for most reads and writes
- `POST /admin/graphql` for browser-session-backed operator workflows
- a small set of dedicated HTTP endpoints for auth, file upload, and extension bundle transfer

## MCP Position

Move Big Rocks does not require MCP to make agents effective. The supported machine
contract is the CLI plus GraphQL.

If Move Big Rocks exposes MCP, it follows these rules:

- reuse the same auth model, permission checks, and audit trail
- derive capabilities from the same command and extension discovery model
- avoid MCP-only business logic or a separate configuration model
- treat MCP as a host-integration convenience, not a replacement for the primary CLI and GraphQL contract

The same rule applies to agent-runtime connectors such as OpenClaw: they compose
with the CLI and GraphQL contract rather than bypass it.

The same ergonomics should apply to self-host bootstrap. The goal is that a
user can give an external agent one short instruction such as "create me a
Move Big Rocks instance repo and deploy it to one Ubuntu VPS I control", and
the agent can complete most of the workflow through GitHub, SSH, and `mbr`
with minimal follow-up. The public bootstrap, README, and instance-template
handoff should therefore make the build-it-yourself path explicit instead of
relying on hidden evaluator infrastructure.

The same ergonomics should apply to concept-aware Markdown work:

- concept specs can define RFCs, templates, constraints, skills, strategic context, delivery context, and extension-specific models
- strategic-context concepts such as goals, strategies, bets, OKRs, KPIs, milestone goals, and workstreams should be first-class instances, not ad hoc notes
- Markdown bodies may use typed references such as `@goal/...`, `@strategy/...`, `@milestone/...`, `@workstream/...`, `@queue/...`, and `@catalog/...`
- the CLI and API should preserve those references while treating structured metadata as the canonical relation graph

That includes The Strategic Context Stack as an explicit concept layer, with the
milestone artefact and proof loop available to agents as machine-readable
context rather than hidden project-management folklore.

## Installation Sources

The CLI supports the following installation sources:

- local bundle file for development and private distribution
- local extension directory with `manifest.json` plus `assets/` for source-first authoring
- HTTPS bundle URL for simple remote delivery
- OCI registry reference for signed published bundles
- marketplace alias resolved into a signed artifact when a private catalog is in use

Example forms:

```bash
mbr extensions install ./my-extension --workspace ws_123
mbr extensions install ./dist/my-extension.hext --workspace ws_123
mbr extensions install https://downloads.example.com/my-extension-0.1.0.hext --workspace ws_123
mbr extensions install ghcr.io/example/my-extension:v0.1.0 --workspace ws_123
mbr extensions install vendor/my-extension@0.1.0 --workspace ws_123 --license-token LICENSE_TOKEN
```

The current public first-party distribution model is signed OCI refs rather
than a marketplace catalog. The intended free public bundle set is:

- `ghcr.io/movebigrocks/mbr-ext-ats:<version>`
- `ghcr.io/movebigrocks/mbr-ext-error-tracking:<version>`
- `ghcr.io/movebigrocks/mbr-ext-web-analytics:<version>`

Those bundles are intended to be freely available. The public source and
publication surface for them is the public first-party extensions repo at
`MoveBigRocks/extensions`. Public signed bundles can install without an
instance-bound token. `--license-token` remains available for private catalog
and instance-bound bundle flows.

When a bundle manifest declares
`workspacePlan.mode = provision_dedicated_workspace`, the CLI can omit
`--workspace` and provision the declared workspace first when the operator is
using browser/session-backed auth.

When a bundle manifest is `scope = instance`, the CLI can omit `--workspace`
and install the pack as an instance-scoped extension. Instance-scoped packs can
be listed with `mbr extensions list --instance`.

Resolver inputs:

- `MBR_MARKETPLACE_URL` points the CLI at the marketplace alias resolver when a private catalog is in use
- `MBR_REGISTRY_TOKEN` provides bearer auth for private OCI registries
- `MBR_REGISTRY_USERNAME` and `MBR_REGISTRY_PASSWORD` provide basic auth when bearer auth is not used
- the server can enforce signed bundle installs with:
  - `INSTANCE_ID`
  - `EXTENSION_TRUST_REQUIRE_VERIFICATION=true`
  - `EXTENSION_TRUSTED_PUBLISHERS_JSON`

Service-backed extensions declare at least one internal `health` endpoint in
their manifest so the runtime can monitor them consistently after activation
and during operator checks.

## Cross-Platform Delivery

Build the CLI in Go and distribute:

- Homebrew and tarballs for macOS
- tarballs and package-manager delivery for Linux
- winget or Scoop plus zip artifacts for Windows

The local packaging entrypoint is `make cli-release-local`. Tagged releases are
published by [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml).
Each release produces cross-platform archives, `checksums.txt`,
`release-manifest.json`, and Sigstore signatures for the published archives.
See [`docs/CLI_RELEASES.md`](./CLI_RELEASES.md) for the artifact matrix and
verification flow.

Safe removal flow for installed extensions:

1. preview the removal plan with `mbr extensions uninstall --id EXTENSION_ID --dry-run --json`
2. export the removal bundle with `--export-out PATH`, or explicitly acknowledge
   no export with `--confirm-no-export`
3. add `--deactivate` when the installation is still active
4. rerun the same command without `--dry-run` to complete uninstall

When `INSTANCE_ID` is configured on the server, `mbr health check --json`
exposes it through the health payload so agents and operators can confirm which
installation a signed bundle or instance-bound grant should target.

## Direct API Access

For commands that are not in the CLI, agents can call GraphQL directly:

```bash
curl -sS https://api.your-app-domain/graphql \
  -H "Authorization: Bearer hat_YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{"query":"query { __typename }"}'
```

For human operator flows that need browser-session auth, run:

```bash
mbr auth login --url https://app.yourdomain.com
```
