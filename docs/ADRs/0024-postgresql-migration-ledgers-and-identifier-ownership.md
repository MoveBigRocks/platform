# ADR 0024: PostgreSQL Migration Ledgers and Identifier Ownership

**Status:** Accepted
**Date:** 2026-03-16

## Context

Move Big Rocks uses:

- PostgreSQL as the only supported runtime database
- bounded-context core schemas inside one PostgreSQL database
- PostgreSQL-native `uuidv7()` defaults for relational row primary keys

This ADR makes the operational contract explicit:

- which migrations are tracked in `public.schema_migrations`
- which migrations are owned by extensions instead
- how extension-applied versions are recorded
- which identifiers PostgreSQL generates and which identifiers the application owns

## Decision

### 1. Move Big Rocks Uses One Core Migration Ledger in `public`

Core PostgreSQL migrations are:

- authored under `migrations/postgres/`
- executed by the core migration runner
- tracked in `public.schema_migrations`

`public.schema_migrations` is only for the core migration stream.

### 2. Service-Backed Extensions Own the SQL for Their `ext_*` Schemas

For service-backed extensions with owned PostgreSQL schemas:

- the extension package owns the canonical SQL files under its own `migrations/` directory
- those migrations are not added to `migrations/postgres/`
- those versions are not recorded in `public.schema_migrations`

Examples:

- the `error-tracking` extension migration directory outside this repo
- the `web-analytics` extension migration directory outside this repo

### 3. Extension-Applied Versions Are Tracked in `core_extension_runtime`

The core extension runtime applies extension-owned schema migrations safely and
records them separately from the core ledger.

Use:

- `core_extension_runtime.extension_package_registrations` for package schema state
- `core_extension_runtime.schema_migration_history` for applied extension migration versions and checksums

This keeps core schema evolution and extension schema evolution distinct while
using one shared PostgreSQL database.

### 4. PostgreSQL Owns Relational Row Identity

For app-owned relational rows in both core schemas and extension-owned schemas:

- use native PostgreSQL `UUID` columns
- default row primary keys to `uuidv7()`
- omit the primary key on normal inserts
- use `RETURNING id` to hydrate the generated row ID back into application state

This rule applies to relational row IDs, not to every identifier in the
system.

### 5. The Application Owns Non-Row Identifiers

Application-generated identifiers remain correct for concerns that exist before
or outside a single database row insert.

That includes:

- event IDs
- request IDs
- correlation IDs
- public IDs
- human-readable IDs
- slugs
- tokens
- hashes
- capability URLs
- external provider IDs
- natural keys

Do not expose raw row UUIDs in public URLs or public APIs when a dedicated
public identifier is the correct model.

## Consequences

### Positive

- the core migration ledger has one clear home: `public.schema_migrations`
- extension-owned schema history has one clear home: `core_extension_runtime.schema_migration_history`
- extension SQL ownership remains with the extension package that owns the schema
- PostgreSQL remains the canonical source of relational row identity
- public and cross-process identifiers stay decoupled from row IDs

### Negative

- core and extension migration history must be documented carefully because there are two ledgers by design
- implementation code must stay disciplined about not generating persisted row IDs in services and handlers

## Related

- [0021](0021-postgresql-only-runtime-and-extension-schemas.md)
- [0022](0022-postgresql-native-uuidv7-row-ids-and-public-identifiers.md)
- [0023](0023-core-postgresql-bounded-context-schemas.md)
