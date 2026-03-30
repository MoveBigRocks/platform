# ADR 0026: Extension Host Lifecycle and Public Extension SDK Boundary

**Status:** Accepted
**Date:** 2026-03-30

## Context

Move Big Rocks promises that extension packaging, validation, activation,
monitoring, and rollback remain explicit, human-visible steps on the same Move
Big Rocks platform surface.

That promise is already reflected in the current operator model:

- `mbr extensions ...` is the authoritative CLI surface
- install, activate, monitor, deactivate, and uninstall are host-managed
  operations
- the platform owns route resolution, runtime supervision, health checks, and
  extension schema migration orchestration

At the same time, the intended repo model says customers and first parties
should be able to build real extension repos outside the core platform repo.

Today the architecture is only partially there.

The platform has a real extension host implementation, but the public runtime
authoring path is still entangled with core internals:

- host lifecycle logic lives under `internal/platform/...` even though it is a
  distinct domain
- first-party service-backed extension runtimes import `platform/internal/...`
  packages for config, database access, and runtime helpers
- the current `extension-sdk` is primarily a source template and testing helper,
  not a complete public runtime SDK
- the public examples therefore do not prove that an external extension repo can
  stay cleanly outside the platform internals

This creates the wrong seam:

- extensions look externally installable at the product level
- but the implementation pattern still assumes build-time access to core
  internals

That is not acceptable for the long-term architecture. It weakens the DDD
boundary, makes private extension repos awkward, and encourages accidental
coupling between the platform host and extension runtimes.

## Decision

### 1. Keep Extension Lifecycle Host-Owned Inside Move Big Rocks

The authoritative lifecycle remains inside the platform host:

- package acceptance and bundle verification
- manifest validation and policy enforcement
- install, upgrade, configure, validate, activate, and deactivate
- runtime supervision and health checks
- route registration, collision checks, and request proxying
- uninstall, export, and rollback flows
- the explicit CLI, GraphQL, and admin-surface operator workflows

This remains the responsibility of the Move Big Rocks platform. It is not moved
into a separate extension repo or delegated to an external SDK.

### 2. Model This As a Distinct `extensionhost` Bounded Context in Code

Move Big Rocks should treat extension lifecycle and runtime supervision as a
real bounded context in the codebase rather than as miscellaneous logic under
`internal/platform/...`.

The target code shape is:

```text
internal/
  extensionhost/
    domain/
    services/
    handlers/
    runtime/
```

Thin adapters may temporarily remain under `internal/platform/...` while the
code moves, but the target ownership is `internal/extensionhost/...`.

Responsibilities of this context include:

- installed-extension state
- manifest and contract admission rules
- runtime topology and endpoint resolution
- schema migration orchestration for `ext_*` schemas
- runtime diagnostics and health state
- extension event catalog and runtime supervision

The external operator surface remains `mbr extensions ...`. This decision is
about code ownership and boundaries, not about changing the human-facing
product language.

### 3. Extract a Public Extension SDK Boundary, Not Just a Tiny Contract Package

The reusable boundary for external extension repos is larger than
`extension-contract`.

The primary public extraction unit should be a real `extension-sdk` module or
repo boundary. `extension-contract` may exist inside it as a package, but it is
not sufficient as the architectural unit on its own.

That public SDK should include at least:

- `contract`
  - manifest, endpoint, event, skill, and contract types
  - normalization and validation helpers that are safe to expose publicly
- `runtime`
  - runtime protocol constants
  - forwarded header and context decoding
  - route-param helpers
  - health response helpers
  - loopback or unix-socket HTTP bootstrap helpers
  - consumer and scheduled-job registration helpers
- `extdb`
  - generic helpers for extension-owned `ext_*` schema access and migrations
- `hostclient`
  - sanctioned client helpers for calling back into approved Move Big Rocks host
    APIs using public contracts
- `testing/sdktest`
  - contract-level smoke-test helpers around `mbr extensions ...`

The contract may be a package inside the SDK, but the architecture must not
stop at extracting manifest types alone.

### 4. External Extensions Must Not Import `platform/internal/...`

Custom, private, and first-party extension runtimes must build against public
SDK surfaces and standard dependencies only.

They must not import:

- `platform/internal/...`
- core store implementations
- core service implementations
- core handler helpers

If an extension needs capability from the host, that capability must be exposed
through a sanctioned public host API or event contract, not by reaching into
internal packages.

### 5. First-Party Extensions Must Also Use the Public SDK

The first-party `extensions` repo must prove the same boundary expected from
external extension authors.

That means the first-party service-backed runtimes should migrate to the public
SDK for bootstrap, runtime context handling, and extension-owned database
helpers.

First-party status does not justify leaking platform internals into the normal
extension authoring path.

### 6. Extensions Own Their `ext_*` Schemas, Not Core Schemas

Service-backed extensions may own:

- their bundle assets
- their runtime logic
- their own `ext_*` PostgreSQL schema

They may not directly read or write `core_*` schemas as a normal integration
path.

Cross-boundary interaction must go through sanctioned contracts such as:

- public host APIs
- extension API contracts
- event publication and subscription
- explicit admin actions or future typed host capabilities

### 7. Preserve Current Storage and Operator Continuity During Migration

This ADR does not rename the human-visible lifecycle or require a database
rename as part of the boundary fix.

For now:

- the operator surface remains `mbr extensions ...`
- existing route-mounting and supervision behavior remains host-owned
- the persisted extension runtime ledger in `core_extension_runtime` remains in
  place

If later we decide that the persisted schema should be renamed to align more
closely with `extensionhost`, that should be a separate ADR because it carries
storage and migration consequences beyond the code boundary change described
here.

### 8. Non-Goals

This ADR does not:

- move install, activate, monitor, or rollback into extension repos
- grant extensions direct access to core internals
- redesign the marketplace or commercial distribution model
- solve every privileged or instance-scoped extension policy question in the
  same change
- require immediate repo renaming before the boundary is made real

## Consequences

### Positive

- the codebase matches the product promise more closely: the platform is the
  extension host, and extension repos are real external packages
- the DDD boundary becomes explicit: lifecycle and runtime supervision are a
  host concern, not a sidecar hidden under `internal/platform`
- private extensions such as `mbr-fleet` can be built cleanly without depending
  on platform internals
- first-party extensions become better examples for customers and partners
- sanctioned integration paths become clearer because extensions can no longer
  reach into core by convenience

### Negative

- the platform must invest in a real public SDK instead of relying on internal
  helpers
- some existing first-party extension runtimes will need refactoring
- there will be an interim period where old and new runtime helper paths
  coexist
- some capabilities currently available only through internal packages will need
  new public host-facing contracts before affected extensions can migrate

### Neutral

- the current `extension-sdk` may evolve from "mostly template" into a real
  importable SDK plus template assets
- some package names in the platform repo may move even if external CLI and API
  behavior stays unchanged

## Compliance

We will treat this ADR as satisfied only when the following are true:

- service-backed extension repos can build and run without importing
  `platform/internal/...`
- the authoritative lifecycle still lives on the Move Big Rocks host surface via
  `mbr extensions ...` and the corresponding platform APIs
- at least one first-party service-backed extension has migrated to the public
  SDK path and proves the boundary
- extension-owned schema access is performed through public SDK helpers or
  generic database libraries, not core store implementations
- documentation for extension authors clearly distinguishes host-owned lifecycle
  behavior from SDK-owned runtime behavior

## Related

- [0006](0006-bounded-context-structure.md)
- [0009](0009-code-architecture.md)
- [0010](0010-agent-api-and-graphql-architecture.md)
- [0021](0021-postgresql-only-runtime-and-extension-schemas.md)
- [0023](0023-core-postgresql-bounded-context-schemas.md)
- [0024](0024-postgresql-migration-ledgers-and-identifier-ownership.md)
- [INSTANCE_AND_EXTENSION_LIFECYCLE](../INSTANCE_AND_EXTENSION_LIFECYCLE.md)
- [EXTENSION_ENDPOINT_MODEL](../EXTENSION_ENDPOINT_MODEL.md)
