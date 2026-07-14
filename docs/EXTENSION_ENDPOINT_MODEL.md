# Extension Endpoint Model

This document defines how extensions expose HTTP endpoints in a secure, standard
way, and how an extension reaches core capabilities.

The short version is:

- extensions do not open arbitrary internet-facing ports on their own
- core Move Big Rocks owns external routing, auth, rate limits, request tracing,
  and audit boundaries
- extensions declare endpoint types in the manifest
- core mounts those endpoints into approved path families and proxies to the
  extension runtime when needed
- an extension reaches core data through the host API, not through core stores,
  services, or a copied core

The boundary between core and an extension is a wire contract, not a shared code
surface, so an extension runtime can be written in any language. See
[ADR 0029](ADRs/0029-language-neutral-extension-runtime-contract.md) and
[ADR 0026](ADRs/0026-extension-host-lifecycle-and-public-extension-sdk-boundary.md).

## Design Goals

- keep endpoint behavior predictable for humans and agents
- let extensions add real routes without bypassing core auth and tenancy
- make endpoint security policy declarative and reviewable
- support both asset-backed and service-backed extension runtimes
- keep the runtime language independent of the core

## Two Runtime Shapes

An extension is one of two shapes, and an extension may combine them:

- asset-backed: the bundle ships static assets and core serves them directly. No
  extension process runs. This covers public pages and assets and simple admin
  pages.
- service-backed: the bundle ships a runtime that core supervises as a child
  process. The runtime serves HTTP over a unix socket and receives scheduled
  jobs and consumer events over the runtime protocol. This covers ingest,
  webhooks, admin actions, and machine APIs.

The service-backed runtime is language independent. Core supervises it and
speaks to it over the runtime protocol, so the runtime binary may be Go, and may
later be another language, without changing this contract.

## Standard Endpoint Classes

Every extension endpoint declares one of these classes:

- `public-page`
  Public HTML or static-site routes such as careers pages.
- `public-asset`
  Static assets such as scripts, stylesheets, and images.
- `public-ingest`
  Public write endpoints such as analytics capture or form-adjacent intake.
- `webhook`
  Signed inbound callbacks from trusted providers.
- `admin-page`
  Admin UI pages rendered inside the shared Move Big Rocks shell.
- `admin-action`
  Admin-side mutation endpoints used by forms or UI actions.
- `extension-api`
  JSON endpoints for the CLI, agent tooling, or extension-owned frontend code.
- `health`
  Internal runtime health endpoints used by core supervision.

These classes are different on purpose. A public ingest route does not inherit
the same defaults as an admin action endpoint.

## Path Families

Extension paths are predictable:

- public pages and assets: `/ext/<extension-slug>/*`
- admin pages: `/admin/ext/<extension-slug>/*`
- admin actions: `/admin/ext/<extension-slug>/actions/*`
- extension APIs: `/api/ext/<extension-slug>/*`
- public ingest and provider-facing endpoints: `/ingest/ext/<extension-slug>/*`
- internal health: loopback or unix-socket only, not public

Compatible aliases can be added where product continuity requires them, but they
register through the same endpoint catalog. For example, `web-analytics` may
register `/js/analytics.js` as an alias to its standard asset route, and
`error-tracking` may register provider-compatible ingest aliases. Aliases are
explicit, reviewed, and reserved to first-party or privileged extensions when
they affect public product contracts.

Core reserves its own paths, such as `/auth`, `/health`, `/metrics`, `/pricing`,
and `/signup`, and rejects extension routes that collide with them. Active
public paths must not collide across workspaces, and active admin paths must not
collide within a workspace.

## Manifest Contract

An endpoint declaration includes:

- `name`
- `class`
- `mountPath`
- `methods`
- `auth`
- `contentTypes`
- `maxBodyBytes`
- `rateLimit`
- `workspaceBinding`
- `serviceTarget`
- `uiIntegration` when relevant

Auth modes:

- `public`
- `signed-webhook`
- `session`
- `agent-token`
- `extension-token`
- `internal-only`

Workspace binding modes:

- `none`
- `workspace-from-session`
- `workspace-from-agent-token`
- `workspace-from-route`
- `instance-scoped`

Core rejects invalid combinations. For example, an `admin-page` cannot use
`public`, a `public-ingest` cannot use `session` as its only auth mode, and a
`health` endpoint cannot be internet-facing.

For instance-admin reach, a workspace-scoped extension that exposes admin pages
still resolves to a working entrypoint for an instance admin without an active
workspace: service-backed admin pages rely on core to inject the resolved
install workspace, and asset admin pages that call workspace-bound APIs preserve
the `?workspace=...` hint on those API requests.

## Security Defaults by Endpoint Class

### `public-page`

- `GET` and `HEAD` only by default
- static or proxied content through core
- CSP and security headers owned by core
- no direct mutation behavior

### `public-asset`

- `GET` and `HEAD` only
- content-hash or version-aware cache behavior
- MIME type validation
- no dynamic filesystem access

### `public-ingest`

- `POST` only by default
- strict `Content-Type` allowlist
- tight body-size limit
- aggressive rate limiting
- spam and replay protection where relevant
- request audit trail and tracing

### `webhook`

- provider signature validation owned by core or standardized middleware
- timestamp skew checks and replay protection
- explicit provider secret wiring from the instance repo
- per-endpoint body-size limits

### `admin-page`

- session auth required
- workspace membership and RBAC enforced by core
- mounted under `/extensions/*`
- extension nav registration required
- workspace-scoped pages remain reachable from instance-admin navigation
  without a live workspace session

### `admin-action`

- session auth and RBAC required
- CSRF protection
- mutation audit logging
- explicit method and content-type validation

### `extension-api`

- token or session auth required unless explicitly public
- JSON schema validation for request and response payloads
- clear error contract for CLI and agent use
- request IDs and tracing required

### `health`

- not exposed publicly
- used only by core supervision
- reports extension runtime readiness and version

## Routing Model

Core Move Big Rocks owns the external HTTP surface. That means:

- the main routers terminate TLS and accept external traffic
- core resolves the endpoint declaration from the installed extension catalog
- core applies the declared auth, rate limit, body limit, and content rules
- core injects trusted runtime context
- only then does core serve a static asset or proxy to the extension runtime

Extensions do not attach themselves directly to the public router or to a core
database connection.

## Runtime Protocol for Service-Backed Extensions

A service-backed runtime runs behind a unix socket that core supervises. Core
delivers three things to the runtime over that socket:

- proxied HTTP requests for the runtime's declared endpoints
- scheduled jobs the manifest declares
- consumer events the manifest subscribes to

Core forwards trusted context on each proxied request using headers it controls:

- request ID
- authenticated principal
- workspace ID when resolved
- extension install ID
- instance ID
- a short-lived bearer host token for host-API calls

The runtime is treated as untrusted application code relative to core routing
and tenancy enforcement. It reads forwarded context from the request rather than
holding any core state of its own.

## Reaching Core Capabilities

An extension does not read or write `core_*` schemas and does not import core
stores, services, or a vendored copy of the core. It reaches core capabilities
through sanctioned surfaces:

- the host API at `/__mbr/host/v1/...`, authenticated by the per-request bearer
  host token, carrying JSON, for reading and writing core entities the
  extension is permitted to touch (for example cases, contacts, and queues)
- typed command and event flows through the outbox and event bus for
  fire-and-forget signals and cross-extension coordination
- documented admin action contracts where an operator action is required

The host API applies the calling extension's workspace scope and permissions and
runs each write under the extension's workspace tenant context, so row-level
security applies exactly as it does for a user in that workspace. A capability
that an extension needs is added as a host-API operation expressed in JSON, not
by handing a core object across the boundary.

Typical examples:

- ATS careers submission: a public page renders the job page, a public ingest
  route receives the submission, and the runtime requests core contact and case
  creation through the host API
- analytics capture: a public ingest endpoint receives the event, the runtime
  stores and rolls up analytics data in its own `ext_*` schema, and admin pages
  read that state through the runtime's own APIs and the shared shell
- error tracking: a webhook or ingest endpoint accepts compatible error
  payloads, the runtime groups issues in its own schema, and it links cases and
  publishes typed events through the host API and the event bus

## UI Integration Rules

Endpoint registration also needs UI registration. An extension that defines
admin pages declares:

- admin navigation entries
- page titles and icons
- required permissions
- optional dashboard widgets
- saved filter or workspace-view integration where supported

Core owns the shell, navigation rendering, workspace switcher, and shared auth.
The extension owns the content inside its registered surfaces.

## Runtime Capabilities by Shape

- asset-backed extensions define `public-page`, `public-asset`, and simple
  `admin-page` surfaces backed by packaged assets
- service-backed extensions additionally define `public-ingest`, `webhook`,
  `admin-action`, and richer `extension-api` surfaces
- privileged identity and connector extensions use the service-backed runtime
- first-party or privileged aliases are allowed where compatibility matters,
  such as analytics scripts or provider-compatible endpoints
- every endpoint declaration is visible to the CLI, admin diagnostics, and the
  security review flow

## Operational Requirements

Every installed extension endpoint is:

- listed in the extension catalog
- versioned
- health-checkable
- traceable
- auditable
- disableable without uninstalling the whole instance

This gives operators and agents one predictable place to inspect what an
extension is exposing.

## Related

- [ADR 0026](ADRs/0026-extension-host-lifecycle-and-public-extension-sdk-boundary.md)
- [ADR 0029](ADRs/0029-language-neutral-extension-runtime-contract.md)
- [ADR 0028](ADRs/0028-extension-endpoint-dual-auth-gate.md)
- [EXTENSION_SECURITY_MODEL](EXTENSION_SECURITY_MODEL.md)
