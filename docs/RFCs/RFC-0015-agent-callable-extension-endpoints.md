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

## Problem Statement

The platform already implements `auth: agent_token` and
`workspace_from_agent_token` workspace binding for extension endpoints
(`cmd/api/extension_service_targets.go:212-228, :290-308`). The enum values
`ExtensionEndpointAuthAgentToken` and `ExtensionWorkspaceBindingFromAgentToken`
exist in `pkg/extensionhost/platform/domain/extension.go:110, :120`. The
endpoint model doc lists `agent_token` as a recommended auth mode for
`extension_api` endpoints.

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

2. **`extensions/web-analytics/manifest.json`**
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

3. **`extensions/web-analytics/assets/agent-skills/manage-properties.md`**
   New bundled skill documenting the agent workflow: mint agent token, discover
   endpoints via contract, call the agent surface, verify.

4. **`extensions/web-analytics/extension.contract.json`**
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
| ADR on Handler → Service → Domain → Store | — | No architectural changes. Handlers remain thin and auth-agnostic. |

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
**Cons:** semantically wrong — `adminPaths` becomes a grab-bag of
"any authenticated path". Makes it harder for tooling and operators to reason
about which paths are browser-only vs agent-callable. Leaves the dogfood
story half-built.
**Why rejected:** the user explicitly asked for "no shortcuts". The contract
is the canonical inventory; it should reflect the distinction.

## Verification Criteria

### Unit Tests
- [ ] `deriveExtensionContractFromManifest` emits `agentPaths` only for
      `extension_api` + `auth: agent_token`
- [ ] `extension_api` + `auth: session` still lands in `adminPaths`
- [ ] `compareExtensionContract` flags drift between declared contract
      `agentPaths` and manifest-derived `agentPaths`
- [ ] Normalization sorts `agentPaths` deduplicated

### Integration Tests
- [ ] `extension_service_targets_test.go`: agent-token request to a
      web-analytics agent endpoint succeeds and populates `workspace_id` from
      the token; session request to the same path is rejected
- [ ] Session request to existing admin property endpoint still succeeds

### Acceptance Criteria
- [ ] After the web-analytics release, `mbr agents tokens create` + curl to
      `POST /extensions/web-analytics/api/agent/properties` with
      `{"domain":"tuinplan.nl","timezone":"Europe/Amsterdam"}` returns `201`
      and creates a row in `ext_demandops_web_analytics.properties`
- [ ] The admin UI at `admin.mbr.demandops.com/extensions/web-analytics`
      continues to list, create, update, delete properties with no regressions
- [ ] `extension.contract.json` lint passes; contract file includes the new
      `agentPaths` block

## Implementation Checklist

- [ ] RFC approved
- [ ] Platform: `agentPaths` contract bucket + derivation + tests
- [ ] Platform: integration test for agent-token path on extension API
- [ ] Extensions: web-analytics manifest + contract + agent skill
- [ ] `make test-all` passes (platform)
- [ ] Extension `make check` passes (extensions)
- [ ] Platform + extension builds cut new OCI versions
- [ ] `mbr-prod` pin bumped, reconciled on mbr.demandops.com
- [ ] tuinplan.nl property created via the new agent surface as end-to-end proof
- [ ] RFC status → `verified`

## Open Questions

- [ ] Permissions/scopes on agent tokens: do we gate `properties.create` behind
      a named permission on the agent membership, or is workspace membership
      sufficient for this first iteration? Proposal: workspace membership for
      the initial release; tighten with named permissions in a follow-up
      once we have more than one extension surface to compare against.
- [ ] Do we want to document a naming convention for the agent path prefix
      (`/api/agent/...` in this RFC)? Proposal: yes, make it a convention
      documented in `docs/EXTENSION_ENDPOINT_MODEL.md` in a follow-up PR; keep
      this RFC scoped to web-analytics as the first adopter.

## Related

- **RFCs:** RFC-0002 (Agent Access), RFC-0004 (Extension System)
- **Docs:** `docs/EXTENSION_ENDPOINT_MODEL.md`, `docs/AGENT_CLI.md`

---

## Changelog

| Date | Author | Change |
|------|--------|--------|
| 2026-04-21 | @adrianmcphee | Initial draft |
