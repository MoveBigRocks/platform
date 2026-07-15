# ADR 0029: Language-Neutral Extension Runtime Contract

**Status:** Accepted
**Date:** 2026-07-14

## Context

[ADR 0026](0026-extension-host-lifecycle-and-public-extension-sdk-boundary.md)
establishes that an extension depends only on a public SDK and sanctioned host
APIs, never on core stores, services, handlers, or `platform/internal/...`. Any
capability an extension needs from the host is reached through a sanctioned host
API or an event contract.

ADR 0026 assumes a single Go SDK. It does not state what the boundary is made of,
and a Go-only boundary can silently leak Go-specific coupling while still passing
review, because the reviewer and the extension speak the same language.

The boundary is in fact realized as wire protocols, not as a Go import surface:

- Core to extension: HTTP over a unix socket. The host delivers proxied
  requests, scheduled jobs, and consumer events to the extension runtime.
- Extension to host: the host API under `/__mbr/host/v1/...`, authenticated by a
  per-request bearer host token, carrying JSON request and response bodies.
- Persistence: an extension owns an `ext_*` PostgreSQL schema created and
  migrated by plain SQL files it ships in its bundle.
- Packaging and trust: a bundle is `manifest.json`, assets, and migrations,
  signed with an Ed25519 signature over the bundle bytes.

None of these are Go-specific. The only Go-specific artifacts are the SDK
scaffolding an author imports and the compiled runtime binary the bundle ships.
Both are per-language and neither is part of the boundary itself.

Three forces push toward stating this explicitly. First, ADR 0026's boundary is
only provably real when it holds for a runtime written in another language; a
second language is the acceptance test for the boundary. Second, a marketplace
and third-party surface, and UI-first extensions authored by frontend
developers, are better served by a TypeScript runtime than by requiring Go.
Third, a second hand-authored SDK would recreate the very duplication the
boundary exists to remove, so the contract, not the SDK, must be the source of
truth.

## Decision

### 1. The extension boundary is a language-neutral wire contract

The boundary is the wire contract, not a set of importable Go packages. The
contract is the union of the runtime protocol (delivery of HTTP requests,
scheduled jobs, and consumer events over a unix socket), the host API
(`/__mbr/host/v1/...`, bearer host token, JSON bodies), the bundle format
(`manifest.json`, assets, SQL migrations), and Ed25519 bundle signing. Every
capability an extension needs from the host is a host-API operation or an event,
expressed in JSON.

### 2. The contract is the source of truth and is published as a schema

The wire contract is published as a machine-readable schema in this repository:
OpenAPI for the host API, and a documented message contract for the runtime
channel. SDKs conform to that schema and are generated from it where practical.
An SDK is a client of the contract, not an independent definition of it.

### 3. Go is the first runtime and proves the boundary

The Go SDK is the reference runtime. First-party extensions run on it as host-API
clients that hold no build-time access to core packages and no copied core code.
A first-party extension that embeds core stores, services, or a vendored copy of
the core does not satisfy this contract and is a defect to fix, consistent with
ADR 0026 section 5.

### 4. A TypeScript runtime is a supported future runtime over the same contract

A TypeScript runtime is added over the identical wire contract when a concrete
marketplace, third-party, or UI-first driver exists. It is a generated client
plus a sandboxed runtime process. It is not a reimplementation or a second
authoritative definition of the boundary.

### 5. Litmus test for every host capability

A capability is admissible only if a runtime in another language could use it
over the wire. "Could a non-Go runtime do this?" is the review question for any
new host-API operation or protocol change. A capability that can only be reached
by importing a Go type or a core package is a boundary defect, the same defect
ADR 0026 names as reaching into core internals.

### 6. Language neutrality is preserved by construction

The contract stays neutral by holding to these properties: request and response
bodies are JSON with no language-specific encoding; identity and authorization
are the bearer host token plus the manifest permission set; persistence is
extension-owned SQL migrations, never core schema access; and trust is a
signature over bytes. A host capability is not admitted by passing a Go object
across the boundary.

### 7. Non-goals

This ADR does not build a TypeScript runtime, does not add a second
hand-authored SDK, does not change the host-owned lifecycle established in ADR
0026 section 1, and does not decide on WebAssembly or other runtime hosts, which
would be a separate decision.

## Consequences

### Positive

- The boundary is provably language-neutral, because a non-Go runtime consumes
  the same contract with no core coupling.
- Polyglot support becomes a bounded project (a generated client plus a sandbox),
  not a rewrite of the boundary.
- The litmus test disciplines every host-API addition, so the seam ADR 0026
  forbids cannot reopen through a Go-shaped convenience.
- A marketplace and third-party authoring surface becomes reachable without a
  language mandate.

### Negative

- The host API is maintained as a published, versioned schema, which is more
  rigor than internal Go structs carry on their own.
- Each host capability is expressed in JSON over the wire, so a design that wants
  to hand a Go object to an extension is not available.
- A TypeScript runtime, when added, brings npm supply-chain exposure and a second
  process runtime to supervise, which must be sandboxed and resource-bounded.

### Neutral

- The Go SDK remains the reference runtime and the richest client.
- The wire contract, once published as a schema, doubles as the authoring
  documentation for any language.

## Compliance

- New host capabilities are added as host-API operations with JSON DTOs, not by
  exposing Go types across the boundary.
- The host API and the runtime protocol are published as a schema checked into
  the repository.
- No extension imports `platform/internal/...`, core store or service
  implementations, or a vendored copy of the core, inheriting ADR 0026 section 4.
- First-party extensions run as host-API clients, which proves the Go runtime; a
  TypeScript runtime, when built, consumes the same published schema.

## Implementation close-out (2026-07-15)

The temporary `extension-sdk/extensionhost` copy of core has been removed. ATS
and error-tracking now own their extension data and reach core only through
`/__mbr/host/v1/...`. Error-tracking's cross-workspace access is represented by
an instance scope grant plus `workspace:read`, while each case and event
operation remains workspace-targeted and permission-checked by the host.

The Go SDK exposes only focused runtime packages, including `runtimehost`,
`runtimehttp`, `extdb`, `apierrors`, `bundletrust`, and `testdb`. Bundle signing
and verification share the public `bundletrust` implementation, so deleting the
core copy does not create a second trust algorithm. First-party tests use fake
host clients or the public PostgreSQL helper instead of copied platform stores.

The security-parity gate that compared the copied tree with core has also been
retired. There is no copied tree left to synchronize; host API tests and
isolated module builds now enforce the boundary directly.

## Related

- **Builds on:** [0026](0026-extension-host-lifecycle-and-public-extension-sdk-boundary.md)
- **Related ADRs:** [0018](0018-api-interface-versioning.md), [0028](0028-extension-endpoint-dual-auth-gate.md)
- **Related RFCs:** RFC-0004 (extension system), RFC-0015 (agent-callable extension endpoints), RFC-0016 (in-process extension runtime supervisor)
