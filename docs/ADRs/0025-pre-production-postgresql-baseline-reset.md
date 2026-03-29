# ADR 0025: PostgreSQL Baseline Reset Policy

**Status:** Accepted
**Date:** 2026-03-16

## Context

When Move Big Rocks establishes a durable PostgreSQL baseline before production
adoption, the baseline should represent the owned final schema shape directly
rather than preserving superseded migration files that exist only to step
between intermediate shapes.

Those superseded files make fresh database creation noisier and preserve debt in
the canonical starting point.

## Decision

### 1. Fresh Databases Land Directly in the Owned Schema Shape

For a baseline reset:

- core baseline migrations create the owned table, column, index, and policy shapes directly
- extension-owned baselines create the extension-owned schema directly
- placeholder slots and one-field follow-up migrations are removed

### 2. Ownership-Aligned Files Absorb Their Own Schema Shape

That means:

- `core_platform.installed_extensions` owns its bundle payload and instance-scoped install shape in its owning baseline file
- the OAuth baseline lands directly in the owned `workspace_scopes` table design
- `enterprise-access` owns `user_info_url` in its extension baseline

### 3. Baseline Resets Recreate Databases Instead of Upgrading Across the Reset

Because the baseline itself is being rewritten:

- old local and staging databases on the pre-reset shape are discarded and recreated
- the recreated database becomes the authoritative baseline going forward

### 4. After the Reset, Migration Streams Are Append-Only

Once the new baseline is established:

- core migration files under `migrations/postgres/` are immutable
- extension-owned migration files are immutable once applied for a published extension version
- schema changes land as new numbered migrations

## Consequences

### Positive

- recreated databases start from a cleaner and more readable baseline
- migration ownership is easier to understand because each file reflects the schema it owns
- production adoption begins from an append-only baseline with less baggage

### Negative

- any database on the superseded baseline is recreated rather than upgraded across the reset
- migration version references from the superseded baseline are no longer authoritative

## Related

- [0021](0021-postgresql-only-runtime-and-extension-schemas.md)
- [0022](0022-postgresql-native-uuidv7-row-ids-and-public-identifiers.md)
- [0023](0023-core-postgresql-bounded-context-schemas.md)
- [0024](0024-postgresql-migration-ledgers-and-identifier-ownership.md)
