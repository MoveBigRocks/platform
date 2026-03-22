# Extension Endpoint Model

This document defines how extensions should expose HTTP endpoints in a secure, standard way.

The short version is:

- extensions do not open arbitrary internet-facing ports on their own
- core Move Big Rocks owns external routing, auth, rate limits, request tracing, and audit boundaries
- extensions declare endpoint types in the manifest
- core mounts those endpoints into approved path families and proxies to the extension runtime when needed

Current implemented slice:

- asset-backed `public-page` and `public-asset` endpoints are now mounted through core
- asset-backed `admin-page` endpoints are now mounted through core when they live under `/admin/extensions/*`
- `publicRoutes` and `adminRoutes` are resolved through the same runtime
- service-backed public routes can now resolve installed `public-asset`, `public-page`, and `public-ingest` endpoints through the shared target registry
- admin-side service-backed routes under `/admin/extensions/*` can now resolve installed `admin-page` and `admin-action` endpoints through the same target registry
- parameterized service endpoints such as `/api/:projectNumber/envelope` are now supported for compatibility-sensitive first-party packs
- service-backed runtime health can now be checked through internal manifest-declared `health` endpoints during activation and `mbr extensions monitor`
- service-backed scheduled jobs and event consumers now run through the same service-target runtime registry
- supervised process management and full external `extension-api` extraction are still pending

This is required for packs such as `web-analytics`, `error-tracking`, and future connector or product extensions that need public ingest routes, admin pages, or machine APIs.

## Design Goals

- keep endpoint behavior predictable for humans and agents
- preserve the current analytics and error-tracking UI and API behavior
- let extensions add real routes without bypassing core auth and tenancy
- make endpoint security policy declarative and reviewable
- support both bundle and service-backed extension runtimes

## Standard Endpoint Classes

Every extension endpoint should declare one of these classes:

- `public-page`
  Public HTML or static-site routes such as careers pages.
- `public-asset`
  Static assets such as scripts, stylesheets, and images.
- `public-ingest`
  Public write endpoints such as analytics capture or form-adjacent forms.
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

These classes are different on purpose. A public ingest route should not inherit the same defaults as an admin action endpoint.

## Path Families

The long-term target is:

Milestone 1 should keep extension paths predictable:

- public pages and assets:
  - `/ext/<extension-slug>/*`
- admin pages:
  - `/admin/ext/<extension-slug>/*`
- admin actions:
  - `/admin/ext/<extension-slug>/actions/*`
- extension APIs:
  - `/api/ext/<extension-slug>/*`
- public ingest and provider-facing endpoints:
  - `/ingest/ext/<extension-slug>/*`
- internal health:
  - loopback or unix-socket only, not public

Compatible aliases can be added where needed for product continuity, but they should still be registered through the same endpoint catalog. For example:

- `web-analytics` may register `/js/analytics.js` as an alias to its standard asset route
- `error-tracking` may register Sentry-compatible ingest aliases

Aliases should be explicit, reviewed, and reserved to first-party or privileged packs when they affect public product contracts.

The currently implemented asset-backed runtime supports:

- public routes on their declared mount paths, except reserved core paths such as `/auth`, `/health`, `/metrics`, `/pricing`, and `/signup`
- admin routes on their declared mount paths only when they live under `/admin/extensions/*`

That narrower contract is deliberate. It keeps asset-backed packs predictable while the richer service-backed runtime is still being completed.

## Manifest Contract

An endpoint declaration should include:

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

Recommended auth modes:

- `public`
- `signed-webhook`
- `session`
- `agent-token`
- `extension-token`
- `internal-only`

Recommended workspace binding modes:

- `none`
- `workspace-from-session`
- `workspace-from-agent-token`
- `workspace-from-route`
- `instance-scoped`

Core should reject invalid combinations. Example:

- `admin-page` cannot use `public`
- `public-ingest` cannot use `session` as its only auth mode
- `health` cannot be internet-facing

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
- mounted under `/admin/extensions/*` in the current runtime
- extension nav registration required

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
- must report extension runtime readiness and version

## Routing Model

Core Move Big Rocks owns the external HTTP surface.

That means:

- the main routers still terminate TLS and accept external traffic
- core resolves the endpoint declaration from the installed extension catalog
- core applies the declared auth, rate limit, body limit, and content rules
- core injects trusted runtime context
- only then does core serve a static asset or proxy to the extension runtime

Extensions should not attach themselves directly to the public router or database connection.

## Runtime Contract for Service-Backed Extensions

Service-backed extensions should run behind a supervised internal interface, such as:

- loopback HTTP
- unix socket
- a core-managed sidecar process

Core should pass trusted context using signed or internal-only headers:

- request ID
- authenticated principal
- workspace ID when resolved
- extension install ID
- instance ID

The extension runtime should be treated as untrusted application code relative to core routing and tenancy enforcement.

## How Endpoints Hook Into Core Services

Extensions should not reach directly into core storage internals. They should integrate through sanctioned surfaces:

- GraphQL reads and writes for approved primitives
- typed command/event flows through the outbox and event bus
- documented admin action contracts where needed

Typical examples:

- ATS careers form submission:
  - public page endpoint renders the job page
  - form submission reaches a public ingest route
  - extension requests core case/contact creation through sanctioned core actions
- analytics capture:
  - public ingest endpoint receives event payload
  - extension stores or rolls up analytics data in its own bounded runtime
  - admin pages and widgets read analytics state through extension APIs and shared shell integration
- error tracking:
  - webhook or ingest endpoint accepts compatible error payloads
  - extension groups and processes issues
  - extension publishes typed events for automation and support linking

## UI Integration Rules

Endpoint registration also needs UI registration.

If an extension defines admin pages, it must also be able to declare:

- admin navigation entries
- page titles and icons
- required permissions
- optional dashboard widgets
- saved filter or workspace-view integration where supported

Core owns the shell, navigation rendering, workspace switcher, and shared auth. The extension owns the content inside its registered surfaces.

## Milestone 1 Rules

Milestone 1 should standardize on these rules:

- bundle extensions can define `public-page`, `public-asset`, and simple `admin-page` surfaces backed by packaged assets
- service-backed extensions can additionally define `public-ingest`, `webhook`, `admin-action`, and richer `extension-api` surfaces
- privileged identity and connector packs can only use the service-backed runtime
- first-party-only aliases are allowed where compatibility matters, such as analytics scripts or Sentry-compatible endpoints
- every endpoint declaration must be visible to the CLI, admin diagnostics, and security review flow

Implemented today:

- bundle extensions can mount asset-backed public pages and assets from stored extension assets
- bundle extensions can mount asset-backed admin pages under `/admin/extensions/*`
- route resolution is workspace-aware for admin pages and instance-wide for public pages
- active public path collisions are rejected across workspaces
- active admin path collisions are rejected within a workspace

## Operational Requirements

Every installed extension endpoint should be:

- listed in the extension catalog
- versioned
- health-checkable
- traceable
- auditable
- disableable without uninstalling the whole instance

This gives operators and agents one predictable place to inspect what an extension is exposing.
