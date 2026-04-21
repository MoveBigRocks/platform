# ADR 0028: Dual-Auth Gate for Extension Endpoints

**Status:** accepted
**Date:** 2026-04-21
**Deciders:** @adrianmcphee

## Context

The platform's `/extensions/*` route group is guarded by
`ContextAuthMiddleware.AuthRequired()`, which is session-only: it reads the
`mbr_session` cookie, validates the session, and redirects to login on
failure. Session-authenticated humans in the admin subdomain rely on this.

The platform already implements endpoint-level auth enforcement per extension
endpoint manifest (see `cmd/api/extension_service_targets.go:193-252`). That
enforcement recognises `auth: agent_token` and delegates to
`PrincipalAuthMiddleware.AuthenticateAgent()` when needed. The
workspace-binding logic already handles `workspace_from_agent_token` by
extracting workspace from the agent token's auth context.

RFC-0015 introduces an agent-callable surface on `/extensions/<slug>/api/agent/*`
that shares `serviceTarget` values (and therefore handlers) with the
session-authed admin endpoints. The goal is a single declarative path per
resource where the endpoint's `auth` mode decides which kind of caller is
allowed, so extensions do not need to duplicate business logic or fragment
their URL space.

With the route-group middleware forcing session auth, agent tokens arriving
at `/extensions/<slug>/api/agent/*` get rejected at the middleware gate
before endpoint-level `enforceExtensionEndpointAuth` ever runs. The endpoint
declaration is declaratively correct but runtime-inert.

Three design options were considered:

1. **Dual-auth gate (this ADR):** keep one route group and one path, teach
   the gate middleware to accept either session or agent token, and let the
   existing endpoint-level auth enforcement pick whichever one the endpoint
   actually requires.
2. **Separate agent route group:** add `/ext-agent/<slug>/*` (or similar) as
   a new route group with `PrincipalAuthMiddleware.AuthenticateAgent()`,
   wired to a new `ResolveAgentServiceRoute` with a third scope
   (`agent`) in `isServiceEndpointInScope`. Extensions declare agent
   endpoints under the new prefix.
3. **Middleware strip-and-authenticate:** remove the gate middleware from
   `/extensions/*` entirely and have every endpoint handler perform its own
   auth as the first action.

## Decision

Adopt the **dual-auth gate**.

Implementation:

- Add `ContextAuthMiddleware.AuthRequiredForSessionOrAgent(principalAuth)`.
  When the incoming request carries an `Authorization: Bearer ...` header,
  the middleware invokes `PrincipalAuthMiddleware.AuthenticateAgent()` and
  populates `auth_context` with `AuthMethodAgentToken`. Otherwise it falls
  through to the existing session path (`AuthRequired` behaviour), which
  populates `auth_context` with `AuthMethodSession`.
- Apply the new middleware to the `/extensions/*` route group in place of
  the session-only `AuthRequired()`.
- `RequireOperationalAccess()` remains applied and already works off
  `auth_context`; for agents it accepts the workspace context seeded by
  `workspace_from_agent_token` binding.
- The admin extension route handler continues to rely on
  `resolvedAdminRouteWorkspaceID(ctx)` for session callers; for agent callers
  it reads the workspace from `auth_context.WorkspaceID` (populated by
  `PrincipalAuthMiddleware`).
- Endpoint-level `enforceExtensionEndpointAuth` is unchanged. It continues
  to enforce the declared `auth` mode per endpoint:
  - `auth: session` endpoint + agent caller → `401 Unauthorized`
  - `auth: agent_token` endpoint + session caller → `401 Unauthorized`
  - Matched pair → `200`

Bearer detection rule: presence of a case-insensitive `bearer ` prefix on
the `Authorization` header. The agent authentication middleware does the
token validation and workspace binding; the gate only decides which auth
path to take.

## Consequences

### Positive

- One URL per resource. Extensions declare `session` and `agent_token`
  endpoints on the same mount path; the client decides which by how it
  authenticates.
- No new endpoint class, no new scope, no new resolver. The existing
  `ResolveAdminServiceRoute` + `extension_api` + `admin` scope already
  matches the request path; auth decides the caller type.
- Existing session-authed admin UI behaviour is preserved. Browsers without
  an `Authorization` header take the identical session path.
- Agent callers can reuse the admin path shape documented in
  `docs/EXTENSION_ENDPOINT_MODEL.md`, which keeps operator mental models
  aligned.

### Negative

- The `/extensions/*` gate middleware becomes auth-aware rather than purely
  "require a valid session". Readers of the code must follow one extra
  indirection to see why an agent token makes it past a gate that used to
  be session-only.
- Agents with a valid token for a workspace will be visible on
  `/extensions/*` which is an admin-subdomain path. This does not expand
  what agents can do (endpoint-level auth still enforces the declared mode)
  but it does broaden the surface where agent tokens are accepted.

### Neutral

- The RFC-0015 `agentPaths` contract bucket and path-convention remain
  unchanged. Documentation in RFC-0015 is amended to describe the
  dual-auth middleware as the runtime mechanism that makes the declared
  `auth: agent_token` endpoints reachable.

## Compliance

Verified by:

- `cmd/api/extension_service_targets_test.go` integration tests that
  exercise an `auth: agent_token` endpoint with a pre-authenticated agent
  `AuthContext`, asserting `201` when the caller matches the endpoint's
  declared auth mode and `401`/`403` otherwise.
- An end-to-end curl against a deployed MBR instance that creates a
  web-analytics property using `mbr agents tokens create` output and a
  `Bearer` header, with the admin UI continuing to function unaffected.
- Linter pass of `extension.contract.json` via
  `mbr extensions lint --write-contract`, confirming `agentPaths` is emitted
  for the new endpoints and the admin UI paths remain in `adminPaths`.

## Related

- **Related RFCs:** RFC-0002 (Agent Access), RFC-0004 (Extension System),
  RFC-0015 (Agent-Callable Extension Endpoints)
- **Related ADRs:** ADR 0010 (Agent API and GraphQL Architecture),
  ADR 0016 (CLI and Agent Authentication Guidelines),
  ADR 0026 (Extension Host Lifecycle and Public Extension SDK Boundary)
