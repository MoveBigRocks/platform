# ADR 0022: PostgreSQL Native UUIDv7 Row IDs and Public Identifiers

**Status:** Accepted
**Date:** 2026-03-15

## Context

Move Big Rocks runs on PostgreSQL, and PostgreSQL 18 provides native `uuidv7()`
support for time-ordered UUID generation.

The system has several different identifier concerns:

- relational row primary keys
- event and correlation identifiers
- public URLs and stable locators
- tokens, hashes, secrets, and capability links
- external provider identifiers

Those concerns do not share one blanket UUID rule.

## Decision

### 1. App-Owned Relational Row Primary Keys Are PostgreSQL-Owned by Default

For app-owned relational rows in Move Big Rocks's PostgreSQL runtime:

- use native PostgreSQL `UUID` columns
- default synthetic primary keys to `uuidv7()`
- omit the primary key on normal inserts
- use `RETURNING id` to read the generated value back into application state

This applies to:

- core tables under `migrations/postgres/`
- first-party extension-owned PostgreSQL schemas when the row is app-owned

Examples:

- `users.id`
- `workspaces.id`
- `cases.id`
- `form_specs.id`
- `jobs.id`
- `ext_demandops_error_tracking.projects.id`
- `ext_demandops_web_analytics.properties.id`

### 2. Internal References Use Native `UUID`

If a column references an internal UUID-backed row, it also uses native `UUID`
in PostgreSQL.

Examples:

- `workspaces.workspace_settings.workspace_id`
- `cases.workspace_id`
- `form_submissions.form_spec_id`
- `agent_tokens.agent_id`
- `ext_demandops_error_tracking.issues.project_id`

### 3. Parent-Derived or Natural Keys Stay Parent-Derived or Natural

Do not add a synthetic UUID just for uniformity when another identifier is the
real identity.

Examples:

- `processed_events (event_id, handler_group)`
- `rate_limit_entries.key`
- `magic_links.token`
- `portal_access_tokens.token`
- `oauth_access_tokens.token_hash`
- `core_extension_runtime.extension_package_registrations.package_key`

### 4. Public Identifiers Stay Application-Generated and Separate From Row IDs

Public-facing identifiers are a separate concern from relational identity.

Use dedicated public identifiers such as:

- `workspaces.slug`
- `cases.human_id`
- `form_specs.crypto_id`
- `jobs.public_id`
- `projects.slug`
- `projects.public_key`

Rules:

- do not expose raw relational row UUIDs in public URLs
- do not encode row UUIDs just to pretend they are public IDs
- choose a dedicated slug, token, human-readable reference, or public ID based on the actual user-facing need

### 5. Event and Cross-Process Identifiers Remain Application-Generated

Event IDs often need to exist before persistence or outside a single database
write. They remain application-generated UUIDv7 values where UUIDs are desired.

Use application-generated UUIDv7 for:

- event IDs
- request IDs
- correlation IDs
- outbox payload correlation identifiers

### 6. Tokens, Hashes, Codes, and External Identifiers Do Not Get Forced Into UUIDv7

Keep non-UUID identifiers for:

- token hashes
- access tokens
- magic link codes
- capability URLs
- secret references
- external provider IDs
- natural keys

### 7. Repositories Own Persistence Semantics

Repositories and stores control row insertion semantics:

- PostgreSQL path: omit the PK and use `RETURNING id`
- explicit ID override is allowed only for narrow import, repair, and test cases
- services and handlers do not manufacture persisted row IDs as a default
- invalid placeholder IDs are not treated as canonical row IDs

### 8. Go Boundaries May Continue to Use Canonical String Forms

The database stores native `UUID` values.

Go code may continue to use canonical string forms at service, JSON, GraphQL,
and handler boundaries where that keeps integration simple, as long as:

- the repository contract treats row IDs as PostgreSQL `UUID`
- comments do not claim the application is the canonical source of row identity
- public identifiers stay separate from row IDs

## Consequences

### Positive

- row identity moves to PostgreSQL where it belongs
- time-ordered UUID benefits are preserved with native storage
- public URLs and public references stay decoupled from internal row identity
- first-party extensions follow the same identifier discipline as core

### Negative

- store create paths need `RETURNING`-based refactors
- tests that used arbitrary non-UUID primary keys need updates or deliberate overrides

## Practical Rule Of Thumb

When adding a new identifier:

1. If it is the primary key of a new app-owned relational row, let PostgreSQL generate `uuidv7()`.
2. If it is a public URL or public-facing locator, generate a dedicated public slug, token, human ID, or public ID in the application.
3. If it is an event or cross-process identifier, generate it in the application.
4. If it is a token, hash, external ID, or natural key, do not force UUIDv7.

## Related

- [0005](0005-event-driven-architecture.md)
- [0015](0015-workspace-scoped-agent-access.md)
- [0021](0021-postgresql-only-runtime-and-extension-schemas.md)
