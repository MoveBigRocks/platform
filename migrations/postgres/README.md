# Move Big Rocks PostgreSQL Migrations

This is the PostgreSQL migration guide for Move Big Rocks, the AI-native service
operations platform.

## Philosophy

Move Big Rocks uses one physical PostgreSQL application database per instance, one
central migration runner, and ownership-aligned migration files under
`migrations/postgres/`.

The goals are:

- one transactional application database
- one migration runner in core
- smaller, reviewable migration files
- file names that communicate ownership and intent
- no special root-level PostgreSQL bootstrap file

This follows the same high-level model adopted in `../tuinplan`: one database,
one embedded runner, and a concrete ownership map that keeps migration files
reviewable even while the runtime remains one deployable core service.

The core baseline is intentionally limited to core-owned tables plus extension
runtime substrate. Service-backed extension schemas are not part of
`migrations/postgres/`; their canonical baselines live with the extension
bundles that own them.

This pre-publication baseline has already been reset into its final core shape.
Fresh databases land directly in the owning table, column, index, and policy
definitions instead of replaying transition-only placeholder or follow-up
migrations.

## Migration Ledgers

Move Big Rocks uses two distinct migration ledgers on purpose:

- `public.schema_migrations` tracks only the core migration stream under
  `migrations/postgres/`
- `core_extension_runtime.schema_migration_history` tracks applied migrations
  for extension-owned `ext_*` schemas

That means:

- core migrations are always owned and versioned by core
- extension migrations are always authored by the extension package that owns
  the schema
- extension migration versions are not copied into `public.schema_migrations`
- the extension runtime is responsible for locking, applying, and recording
  extension-owned schema migrations

## Identifier Policy

For Move Big Rocks's PostgreSQL runtime:

- app-owned relational row primary keys should use native `UUID` columns
- those row primary keys should default to `uuidv7()`
- repositories should omit PKs on normal inserts and use `RETURNING id`
- public locators should stay separate from row IDs
- event and correlation IDs remain application-generated UUIDv7 values
- tokens, hashes, capability URLs, and natural keys remain non-UUID where
  appropriate

Existing public-facing identifier patterns in Move Big Rocks remain valid:

- workspace URLs use `slug`
- service cases use `human_id`
- public forms entrypoints use dedicated slugs or `public_key`
- jobs use `public_id`
- extension/application public surfaces use dedicated slugs or keys

Do not expose raw row UUIDs in public URLs.

## Current Layout

```text
migrations/postgres/
├── OWNERSHIP.md
├── 000001_core_infra.up.sql
├── 000002_core_platform.up.sql
├── 000003_core_auth.up.sql
├── 000004_core_service.up.sql
├── 000005_core_email.up.sql
├── 000006_core_forms.up.sql
├── 000007_core_automation.up.sql
├── 000008_core_knowledge_resources.up.sql
├── 000009_core_access_audit.up.sql
├── 000010_core_agents.up.sql
├── 000011_core_rls.up.sql
├── 000012_core_oauth.up.sql
└── 000013_core_extension_runtime.up.sql
```

This sequence is the authoritative core PostgreSQL baseline. It bootstraps a
fresh PostgreSQL database directly with ownership-aligned migration files.

## Structure Rules

- All PostgreSQL runtime migrations live under `migrations/postgres/`.
- Files are discovered and applied in numeric order by the central runner.
- The initial baseline may be broad, but follow-up migrations must stay small
  and ownership-specific.
- Once a version is applied to a real environment, that file becomes immutable.
- The concrete table-to-file target layout lives in `migrations/postgres/OWNERSHIP.md`.

## Ownership Split

The baseline is split by logical ownership, not by separate databases:

```text
migrations/postgres/
├── 000001_core_infra.up.sql
├── 000002_core_platform.up.sql
├── 000003_core_auth.up.sql
├── 000004_core_service.up.sql
├── 000005_core_email.up.sql
├── 000006_core_forms.up.sql
├── 000007_core_automation.up.sql
├── 000008_core_knowledge_resources.up.sql
├── 000009_core_access_audit.up.sql
├── 000010_core_agents.up.sql
├── 000011_core_rls.up.sql
├── 000012_core_oauth.up.sql
└── 000013_core_extension_runtime.up.sql
```

Why this shape:

- infrastructure tables and shared primitives land first
- domain-owned tables land before RLS and grants
- access control and RLS stay near the end because they depend on the tenant-scoped tables already existing
- extension runtime metadata stays core-owned but physically separate from extension-owned schemas

## Physical Schema Notes

The core baseline now uses bounded-context schemas inside one PostgreSQL
database:

- `core_infra`
- `core_identity`
- `core_platform`
- `core_service`
- `core_automation`
- `core_knowledge`
- `core_governance`
- `core_extension_runtime`
- `ext_*` for service-backed extension-owned tables

Shared cross-context helpers remain in `public` where appropriate:

- `public.schema_migrations`
- `public.current_workspace_id()`

The migration file split is therefore about ownership and dependency order
inside one application database, not about introducing one database per
context.

Extension-owned schemas remain outside the core baseline and migrate through
their own package-provided SQL executed by the core extension runtime.

Today that means:

- core-owned runtime metadata stays in `migrations/postgres/`
- canonical `enterprise-access` schema SQL lives with the
  `enterprise-access` extension source outside this repo
- canonical `web-analytics` schema SQL lives with the `web-analytics` extension
  source outside this repo
- canonical `error-tracking` schema SQL lives with the `error-tracking` extension
  source outside this repo
- extension-applied versions are recorded in
  `core_extension_runtime.schema_migration_history`

## Baseline Reset Rule

This repository is still pre-publication and may reset the PostgreSQL baseline
when the storage contract changes materially.

That means:

- local and other pre-publication databases should be recreated after identifier
  contract changes like the UUIDv7 adoption and the clean baseline reset
- once a migration version is applied to a published or customer environment,
  that file becomes immutable
- future post-publication schema changes must land as new numbered migrations

## PostgreSQL Requirement

PostgreSQL 18 or newer is required for native `uuidv7()`.

## See Also

- [Ownership Map](OWNERSHIP.md)
- [RFC-0006](../../docs/RFCs/RFC-0006-postgres-and-extension-schemas.md)
- [ADR-0022](../../docs/ADRs/0022-postgresql-native-uuidv7-row-ids-and-public-identifiers.md)
- [ADR-0023](../../docs/ADRs/0023-core-postgresql-bounded-context-schemas.md)
- [ADR-0024](../../docs/ADRs/0024-postgresql-migration-ledgers-and-identifier-ownership.md)
- [ADR-0025](../../docs/ADRs/0025-pre-production-postgresql-baseline-reset.md)
