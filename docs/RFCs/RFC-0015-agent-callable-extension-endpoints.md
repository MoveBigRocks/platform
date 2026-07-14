# RFC-0015: Agent-Callable Extension Endpoints

**Status:** draft
**Author:** @adrianmcphee
**Created:** 2026-04-21

## Summary

Establish the convention, contract, and first reference implementation for
extension endpoints that accept `auth: agent_token`, so extensions can be
managed by agents and the `mbr` CLI without relying on browser-session cookies.
Adds an `agentPaths` bucket to the extension contract so agent-callable paths
are first-class alongside `adminPaths` and `publicPaths`. Web-analytics
property management becomes the first dogfood adopter.

This is the inbound direction, an agent or the `mbr` CLI calling into an
extension over `/extensions/*`. It is distinct from the outbound host API an
extension uses to call back into core; both surfaces coexist on the same
runtime.

## Problem Statement

The platform already implements `auth: agent_token` and
`workspace_from_agent_token` workspace binding for extension endpoints in the
extension service-target layer. The enum values
`ExtensionEndpointAuthAgentToken` and `ExtensionWorkspaceBindingFromAgentToken`
exist in the extension host domain. The endpoint model doc lists `agent_token`
as a recommended auth mode for `extension_api` endpoints.

However, **no extension in the codebase currently uses it**. Every
`extension_api` endpoint across `ats`, `error-tracking`, and `web-analytics`
declares `auth: session`, which requires a browser session cookie. This means:

- Agents and the `mbr` CLI cannot call any extension-owned API without a
  browser login. Agent tokens minted via `mbr agents tokens create` are
  rejected at `enforceExtensionEndpointAuth`.
- Extensions cannot participate in the agent-native operating model promised
  by RFC-0002 (Agent Access) and RFC-0004 (Extension System).
- Operators must either reach for the admin UI or bypass the SDK entirely
  (direct SQL). This contradicts the "dogfood the platform" intent.

A concrete trigger: registering a new tracked site in the web-analytics
extension (e.g., `tuinplan.nl`) currently has no agent-callable path.

## Proposed Solution

### Overview

Define a convention for exposing the same extension business logic through two
parallel endpoint surfaces:

- **Admin surface**: `auth: session`, `workspaceBinding: workspace_from_session`,
  mounted under the extension's admin path tree. Continues to serve browser
  UIs.
- **Agent surface**: `auth: agent_token`,
  `workspaceBinding: workspace_from_agent_token`, mounted under
  `/extensions/<slug>/api/agent/...`. Callable from `mbr` CLI and agents.

Both surfaces point to the **same `serviceTarget`**, so the handler, service,
domain, and store layers are shared. Channel-specific concerns (auth binding,
path naming) live only in the manifest.

### Contract changes

Add `agentPaths` as a first-class field in `extension.contract.json`, derived
from the manifest:

```json
{
  "schemaVersion": 1,
  "extensionSlug": "web-analytics",
  "publicPaths": [...],
  "adminPaths": [...],
  "agentPaths": [
    "/extensions/web-analytics/api/agent/properties",
    "/extensions/web-analytics/api/agent/properties/:id"
  ],
  "healthPaths": [...]
}
```

Derivation rule: an endpoint with `class: extension_api` goes to:
- `agentPaths` if its `auth: agent_token`
- `adminPaths` otherwise (keeping existing behaviour for session endpoints)

This keeps `adminPaths` pure (browser-session surface) and introduces
`agentPaths` as the canonical inventory of agent-callable paths for that
extension.

### Runtime: dual-auth gate on `/extensions/*`

The `/extensions/*` route group was previously gated by
`ContextAuthMiddleware.AuthRequired()`, which is session-only and redirects
to login. Declared `auth: agent_token` endpoints under that prefix were
runtime-inert: the gate rejected agent tokens before
`enforceExtensionEndpointAuth` ever ran.

ADR 0028 formalises the fix: a `ContextAuthMiddleware.AuthRequiredForSessionOrAgent`
dual-auth gate that accepts either a session cookie or an `Authorization:
Bearer ...` token. Bearer requests go through
`PrincipalAuthMiddleware.AuthenticateAgent()`; cookie requests take the
existing session path. Both populate `auth_context` so downstream middleware
(`RequireOperationalAccess`) and the endpoint-level
`enforceExtensionEndpointAuth` behave identically.

`enforceExtensionEndpointAuth` remains the per-endpoint enforcement point.
A session caller hitting `auth: agent_token` is rejected with `401`, and
vice versa. The declared auth mode in the manifest is still the source of
truth for who can call a given endpoint.

### Changes Required

1. **`platform/cmd/mbr/extension_contract.go`**
   - Add `AgentPaths []string` field to `extensionContractFile` struct
   - Add `contractAgentPaths(manifest)` + `detailAgentPaths(detail)` helpers
   - Split `contractAdminPaths` so `extension_api` endpoints with
     `auth: agent_token` route to `agentPaths`, not `adminPaths`
   - Extend `deriveExtensionContractFromManifest`,
     `deriveExtensionContractFromDetail`, `compareExtensionContract`,
     `normalizeExtensionContract` to handle the new field
   - Tests in `extension_contract_test.go` (or equivalent) for derivation
     and lint comparison

2. **`platform/pkg/extensionhost/infrastructure/middleware/context_auth.go`**
   - Add `AuthRequiredForSessionOrAgent(principalAuth)` that detects a
     `Bearer` Authorization header and delegates to
     `PrincipalAuthMiddleware.AuthenticateAgent()`, otherwise falls through
     to the session-cookie path used by `AuthRequired`.

3. **`platform/cmd/api/routers.go`**
   - Replace `AuthRequired()` with `AuthRequiredForSessionOrAgent(principalAuth)`
     on the `/extensions/*` route group.
   - The admin extension handler continues to use
     `resolvedAdminRouteWorkspaceID(ctx)`; for agent callers that falls back
     to `auth_context.WorkspaceID` which is seeded by the agent-token
     workspace binding.

4. **`extensions/web-analytics/manifest.json`**
   - Add 5 new endpoints pointing to existing `serviceTarget` values:
     - `GET /extensions/web-analytics/api/agent/properties`
     - `POST /extensions/web-analytics/api/agent/properties`
     - `GET /extensions/web-analytics/api/agent/properties/:id`
     - `PATCH /extensions/web-analytics/api/agent/properties/:id`
     - `DELETE /extensions/web-analytics/api/agent/properties/:id`
   - All with `class: extension_api`, `auth: agent_token`,
     `workspaceBinding: workspace_from_agent_token`
   - Extend `commands` with `web-analytics.properties.create`,
     `.get`, `.update`, `.delete`
   - Extend `agentSkills` with `manage-properties`

5. **`extensions/web-analytics/assets/agent-skills/manage-properties.md`**
   New bundled skill documenting the agent workflow: mint agent token, discover
   endpoints via contract, call the agent surface, verify.

6. **`extensions/web-analytics/extension.contract.json`**
   Regenerate to include the new `agentPaths` entries.

### What Does NOT Change

- Handlers, services, domain, stores: zero changes. The handler layer already
  reads `workspace_id` from the gin context; the platform populates it
  identically for session and agent-token bindings.
- Existing session endpoints: untouched. The admin UI continues to work
  against the current `/api/properties` paths.
- ATS and error-tracking extensions: unchanged. Each extension opts in by
  declaring agent endpoints when there is a concrete agent use case.
- Platform auth enforcement: already implemented. No changes to
  `enforceExtensionEndpointAuth` or workspace binding logic.

## ADR Compliance

| ADR / RFC | Title | Compliance |
|-----------|-------|------------|
| RFC-0002 | Agent Access and CLI Authentication | This RFC extends the agent-token surface to extension endpoints, honouring the `workspace_from_agent_token` binding. |
| RFC-0004 | Extension System | First concrete use of the declarative `agent_token` auth mode defined for extensions. |
| ADR 0016 | CLI and Agent Authentication Guidelines | Agent token (`hat_*`) flow is reused unchanged; no new credential shape introduced. |
| ADR 0026 | Extension Host Lifecycle and Public Extension SDK Boundary | No handler, service, or store code is touched below the manifest layer; only declarative endpoints and a platform-level middleware change are required. |
| ADR 0028 | Dual-Auth Gate for Extension Endpoints | This RFC specifies the declarative and contract-level changes; ADR 0028 specifies the runtime gate that accepts either session or agent-token auth on `/extensions/*`. |

## Alternatives Considered

### Alternative 1: Multi-auth endpoints

Let a single endpoint declare `auth: ["session", "agent_token"]` so one path
serves both surfaces.

**Pros:** no path duplication; one endpoint per resource.
**Cons:** requires platform changes to the manifest schema and
`enforceExtensionEndpointAuth` switch. Conflates surfaces for rate limiting,
audit, and observability. Future-me wants to attach different rate limits to
agent traffic than to human traffic; separate endpoints make that trivial.
**Why rejected:** larger blast radius for the same outcome. Can still be
adopted later if duplication becomes painful.

### Alternative 2: Flip session to agent_token on existing endpoints

Change the existing `/api/properties` endpoints from `auth: session` to
`auth: agent_token`. Update the admin UI to mint short-lived agent tokens on
the client side.

**Pros:** one endpoint per resource.
**Cons:** breaks the admin UI's auth model (browsers use session cookies, not
bearer tokens). Invasive refactor for no gain; browser UIs have session
cookies for good reasons.
**Why rejected:** wrong shape for browser UI.

### Alternative 3: Mix agent_token endpoints into adminPaths

Keep the contract schema unchanged; let `extension_api` endpoints with
`auth: agent_token` show up in `adminPaths` alongside session endpoints.

**Pros:** zero platform-side changes.
**Cons:** semantically wrong; `adminPaths` becomes a grab-bag of
"any authenticated path". Makes it harder for tooling and operators to reason
about which paths are browser-only vs agent-callable. Leaves the dogfood
story half-built.
**Why rejected:** the user explicitly asked for "no shortcuts". The contract
is the canonical inventory; it should reflect the distinction.

## Related

- **RFCs:** RFC-0002 (Agent Access), RFC-0004 (Extension System)
- **Docs:** `docs/EXTENSION_ENDPOINT_MODEL.md`, `docs/AGENT_CLI.md`
