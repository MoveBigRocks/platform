# RFC-0006: PostgreSQL-First Storage and Extension-Owned Schemas

## Status

verified

## Summary

Move Big Rocks runs on PostgreSQL and allows curated service-backed extensions to own
one PostgreSQL schema per extension package, while bundle-first extensions stay
on the shared primitive model.

This RFC defines:

- PostgreSQL-first production storage
- clear ownership boundaries between core data and extension-owned data
- a core-owned migration and runtime contract for service-backed extensions
- explicit repository and persistence boundaries across the codebase

## What This RFC Decides

- PostgreSQL is the production datastore for Move Big Rocks.
- Store and repository boundaries are mandatory.
- Move Big Rocks uses one shared PostgreSQL database per instance.
- Core tables live in bounded-context `core_*` schemas alongside `core_extension_runtime`.
- Bundle extensions remain shared-primitive extensions.
- Curated service-backed extensions may own one PostgreSQL schema per extension package.
- Service-backed extension migrations are shipped by the extension bundle and executed, locked, audited, and tracked by core.
- Service-backed extension package versioning is instance-scoped.

## Architectural Preconditions

### Repository Pattern Is Mandatory

All database interaction is contained in repository and store implementations
and migration runtime infrastructure.

Required shape:

- domain packages contain business rules, invariants, and state transitions
- application services orchestrate domain logic, repository calls, outbox publishing, and policy checks
- resolvers and HTTP handlers map inputs and outputs only
- repositories and stores own SQL, driver details, transactions, row mapping, and query optimization

Forbidden outside infrastructure persistence and migration packages:

- SQL query text
- driver names and DSNs
- schema names
- `database/sql` and `sqlx` usage
- direct table access from domain, resolver, handler, or orchestration code

### Fat Domain Model, Thin Orchestration

Business rules are testable without a database.

That means:

- domain objects validate and normalize their own state
- domain methods own state transitions
- services compose domain decisions with repositories and outbox and event calls
- resolvers and handlers do not implement workflow logic

## Production Target

PostgreSQL is the runtime database for Move Big Rocks.

PostgreSQL is the CI and integration environment, the production deployment
target, and the required environment for service-backed runtime and migration
tests.

## Physical Schema Layout

Move Big Rocks uses this physical layout:

```text
core_infra
core_identity
core_platform
core_service
core_automation
core_knowledge
core_governance
core_extension_runtime    # instance-scoped extension runtime and migration metadata
ext_<publisher>_<slug>    # owned schema for each service-backed extension package
public                    # shared migration/runtime helpers only
```

Important clarification:

- core domain ownership remains logical even though migration files are not one-to-one with schemas
- schemas are ownership namespaces inside one database, not service boundaries
- repositories and migrations use explicit schema qualification

## Storage Ownership Model

Core owns shared primitives and core runtime metadata, including:

- workspaces
- memberships
- sessions
- agent tokens
- contacts
- cases
- collections
- knowledge resources
- form specs
- form submissions
- attachments
- automation rules
- outbox and event infrastructure
- audit data
- extension install records
- extension schema registration and migration history

Curated service-backed extensions may own:

- extension-specific entities
- ingest buffers
- rollups and materialized views
- extension-local workflow state
- extension-local search and index state
- extension-local configuration beyond shared install config

Bundle extensions:

- use shared primitives only
- may seed knowledge, forms, collections, assets, and automation rules
- do not own arbitrary tables or schemas

## Core Baseline Migrations

Core PostgreSQL baseline migrations contain only core-owned relational state
and extension runtime substrate:

- shared primitives and universal workflows such as workspaces, users, memberships, contacts, cases, collections, knowledge resources, forms, files, outbox, and audit
- extension platform metadata such as install rows, assets, bundle payloads, package registration, and schema migration history
- tenancy helpers, RLS helpers, and shared operational roles and grants

Core baseline migrations do not become a dumping ground for optional product
extensions.

## Extension-Owned Migrations

Service-backed extension migrations contain only schema-local relational state
for that package:

- extension-specific entities
- extension-local ingest buffers
- extension-local rollups, projections, caches, and materialized views
- extension-local workflow state and provider/config state
- RLS policies for extension-owned workspace tables

Extension migrations may reference only the allowed core anchors:

- `core_platform.workspaces`
- `core_platform.installed_extensions`
- shared tenancy helpers such as `public.current_workspace_id()`

They do not create or alter core business tables in `core_*` schemas.

## What Stays Out of Schema Migrations

These belong in install or activate flows, runtime registration, or dedicated
operational tooling rather than in canonical schema migrations:

- seeded collections, knowledge resources, form specs, automation rules, and default config
- route, admin navigation, dashboard, job, and event-consumer registration
- data import and export tasks
- archival and export tasks
- one-off repair tasks

## Runtime Classes and Storage Rights

### Bundle Extensions

Bundle extensions:

- are workspace-scoped unless their manifest declares otherwise
- use shared primitives only
- may declare asset-backed endpoints and route metadata
- may store lightweight install config in core-owned install metadata
- do not ship owned-schema migrations

### Service-Backed Extensions

Service-backed extensions:

- may own exactly one PostgreSQL schema per extension package
- may ship PostgreSQL migrations for that schema
- may expose service-backed endpoints through the supervised runtime
- may register extension jobs and event consumers
- may maintain extension-local state

## Versioning and Scope Rules

### One Schema Per Package, Not Per Install

The schema unit is the extension package, not the workspace install.

Examples:

- `ext_demandops_web_analytics`
- `ext_demandops_error_tracking`

Schema naming rule:

- format: `ext_<normalized_publisher>_<normalized_slug>`
- characters allowed after normalization: lowercase ASCII letters, digits, and underscores
- schema names are globally unique within the instance

Each workspace install stores workspace-scoped rows inside that shared package
schema.

### Service-Backed Package Version Is Instance-Scoped

The rule is:

- one service-backed package slug may have many workspace installs in an instance
- all installs of that service-backed package converge on the same package version
- installing the same package into a new workspace uses the already-registered package version
- upgrading a service-backed package is an instance-scoped operation that migrates the shared schema once and then updates install records for every install of that package

Bundle extensions remain versioned per install because they do not own shared
schemas.

## Data Model Changes

### Workspace-Scoped Install Record

Entity: `InstalledExtension`

- id
- workspace_id
- publisher
- slug
- package_key (`<publisher>/<slug>`)
- kind
- scope
- risk
- runtime_class (`bundle`, `service_backed`)
- storage_class (`shared_primitives_only`, `owned_schema`)
- installed_bundle_version
- status
- validation_status
- health_status
- config
- manifest
- timestamps

Important rule:

- `InstalledExtension` is not the canonical source of schema version state for service-backed packages

### Instance-Scoped Package Registration

Entity: `ExtensionPackageRegistration`

- package_key
- runtime_class
- storage_class
- schema_name
- active_bundle_version
- current_schema_version
- target_schema_version
- state (`pending`, `migrating`, `ready`, `failed`, `disabled`)
- last_error
- updated_at

This record is unique per service-backed package in the instance and lives in
`core_extension_runtime`.

### Append-Only Migration History

Entity: `SchemaMigrationHistory`

- id
- package_key
- schema_name
- migration_version
- checksum_sha256
- bundle_version
- status (`applied`, `failed`)
- started_at
- finished_at
- error_message
- applied_by_release

This table is append-only and is the canonical migration audit trail.

## Manifest and Bundle Contract

### Required Manifest Fields for Service-Backed Packages

Service-backed manifests declare:

- `runtimeClass`
- `storageClass`
- `schema.name`
- `schema.packageKey`
- `schema.targetVersion`
- `schema.migrationEngine` set to `postgres_sql`
- runtime protocol and service metadata

Bundle manifests are rejected if:

- `runtimeClass = bundle` and `storageClass = owned_schema`
- `runtimeClass = service_backed` and schema declaration is missing
- schema name does not match the approved naming convention
- package key does not match manifest publisher plus slug

### Bundle Transport

Service-backed bundles contain:

- `manifest.json`
- `assets/` when relevant
- `migrations/`
- service runtime metadata

The bundle digest and signature cover:

- manifest
- assets
- migrations
- runtime metadata

### Source-Directory Install

Local source-directory installs read:

- `manifest.json`
- `assets/`
- `migrations/`

This keeps local development and production artifact installs on the same
migration contract.

## Extension Migration Contract

### Ownership

The extension authors the migration set for its owned schema.

Core owns:

- validating the migration set
- schema creation
- migration execution
- migration locking
- migration history
- conformance checks
- upgrade failure handling

### Migration File Rules

Rules:

- forward-only migrations only
- filename format: `000001_init.up.sql`, `000002_add_region_city.up.sql`
- version numbers are strictly increasing and unique within the package
- each migration file is deterministic and idempotent only through the migration ledger
- non-transactional DDL is disallowed

### Schema Qualification

Do not rely on mutable `search_path`.

Extension migration SQL must:

- use explicit schema qualification
- or use the single allowed template token `${SCHEMA_NAME}`, substituted by the core migration executor

No other template substitution is allowed.

### Migration Ledger

Migration state is tracked in `core_extension_runtime`, not in a mutable table
inside each extension schema.

Reason:

- schema ownership is package-scoped, not install-scoped
- migration history survives uninstall and purge flows
- bootstrap works before the extension schema contains runtime tables
- one shared ledger makes package-wide audit and lock coordination straightforward

### Locking

Core acquires a PostgreSQL advisory lock keyed by package key or schema name
before running a service-backed migration flow.

That lock covers:

- activation
- upgrade
- schema bootstrap
- replay or retry after failure

Concurrent migration attempts for the same package are rejected or serialized.

### Checksum and Drift Rules

For each applied migration version, core records the file checksum.

Rules:

- if a migration version has already been applied with the same checksum, skip it
- if a migration version has already been applied with a different checksum, fail hard
- if a migration set is missing a previously applied version, fail hard
- if a new migration is out of order, fail hard

### Failure Semantics

If migration execution fails:

- mark package registration state as `failed`
- append a failed attempt row to migration history
- do not activate or upgrade the service-backed runtime
- preserve the last known good schema state
- require operator retry after the bundle or environment problem is fixed

Rollback means keeping the previously applied schema as authoritative and
refusing the new activation or upgrade.

## Runtime Responsibilities

The supervised service-target runtime is responsible for:

1. verifying bundle signature and license
2. verifying runtime class, storage class, and schema declaration
3. ensuring package registration exists
4. acquiring migration lock
5. ensuring the owned schema exists
6. running pending migrations
7. ensuring manifest-declared service targets are registered
8. probing internal health endpoints before activation completes
9. routing public and admin service-backed endpoints through the shared registry
10. marking runtime health and migration state

Activation succeeds only after migration succeeds.

## Data Access Rules

### Do

- use repository and store interfaces for all persistence access
- use core APIs, commands, GraphQL, and events for shared primitives
- store extension-owned data inside the owned schema
- include `workspace_id` for workspace-scoped data
- include `extension_install_id` for install-owned rows
- publish extension-originated events through the core-owned outbox path

### Do Not

- access core tables directly from extension runtime code without sanctioned contracts
- let extensions alter core schemas
- rely on cross-schema mutation as the default integration model
- put extension-specific relational state into shared core tables without explicit core ownership

## Service-Backed Security Model

Service-backed schemas remain curated and privilege-bounded.

The model is:

- service-backed runtime is supervised by core
- package signing and validation are mandatory
- schema ownership is explicit
- installation and upgrade remain auditable
- extension APIs, endpoints, jobs, and migrations run within the declared capability envelope

## Related ADRs

- [0021](../ADRs/0021-postgresql-only-runtime-and-extension-schemas.md)
- [0022](../ADRs/0022-postgresql-native-uuidv7-row-ids-and-public-identifiers.md)
- [0023](../ADRs/0023-core-postgresql-bounded-context-schemas.md)
- [0024](../ADRs/0024-postgresql-migration-ledgers-and-identifier-ownership.md)
