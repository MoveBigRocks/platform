# ADR 0023: Core PostgreSQL Bounded-Context Schemas

**Status:** Accepted
**Date:** 2026-03-15

## Context

Move Big Rocks uses:

- PostgreSQL as the only runtime database
- extension-owned `ext_*` schemas for service-backed first-party extensions
- PostgreSQL-native `uuidv7()` row IDs with separate application-generated public identifiers

Core tables living in `public` weakens ownership boundaries. It makes the
runtime look flatter than the codebase and leaves storage less aligned with the
bounded contexts already present in the application.

Move Big Rocks therefore uses ownership-aligned PostgreSQL schemas inside one
application database and does not rely on mutable `search_path` for
correctness.

## Decision

Core PostgreSQL tables live in a small set of bounded-context schemas:

- `core_infra`
- `core_identity`
- `core_platform`
- `core_service`
- `core_automation`
- `core_knowledge`
- `core_governance`
- `core_extension_runtime`

Service-backed first-party extensions own one package-scoped `ext_*` schema
each.

This remains one logical application database, not one database per bounded
context.

## Schema Map

### `core_infra`

- outbox and delivery infrastructure
- idempotency and rate-limit state
- shared file metadata

### `core_identity`

- users
- workspace and user membership and authz rows
- sessions and magic links
- roles, portal tokens, subscriptions, notifications
- agents, agent tokens, workspace memberships
- OAuth clients and token state

### `core_platform`

- workspaces
- teams and team membership
- contacts
- workspace settings
- installed extensions and extension assets

### `core_service`

- cases and collections
- communications and attachments
- email operational state
- forms definitions and submissions

### `core_automation`

- rules
- workflows
- jobs and execution state

### `core_knowledge`

- knowledge resources
- knowledge navigation and linking metadata
- retrieval and generation metadata

### `core_governance`

- custom fields
- audit configuration and audit logs
- alert rules
- security events
- retention configuration

## Query and Migration Rules

- repositories and stores use explicit schema-qualified table names
- core and extension migration SQL do not rely on mutable `search_path`
- cross-schema references are allowed where ownership and dependency order make them appropriate
- shared tenancy helpers may remain in `public` when that is the cleanest shared anchor

Shared anchors:

- `public.current_workspace_id()` is the shared RLS helper
- `public.schema_migrations` is the core migration ledger

## Consequences

### Positive

- core storage ownership is visible in PostgreSQL itself
- schema boundaries match the application's bounded-context structure much more closely
- first-party extension schemas and core schemas follow one coherent model
- schema evolution stays explicit without runtime `search_path` coupling

### Negative

- core repository SQL requires explicit schema qualification
- migration and test fixtures must keep schema-qualified references accurate

## Related

- [0006](0006-bounded-context-structure.md)
- [0021](0021-postgresql-only-runtime-and-extension-schemas.md)
- [0022](0022-postgresql-native-uuidv7-row-ids-and-public-identifiers.md)
- [RFC-0006](../RFCs/RFC-0006-postgres-and-extension-schemas.md)
