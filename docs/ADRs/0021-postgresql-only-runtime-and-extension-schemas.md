# ADR 0021: PostgreSQL-Only Runtime and Extension-Owned Schemas

**Status:** Accepted
**Date:** 2026-03-13

## Context

Move Big Rocks runs on PostgreSQL. Core runtime storage uses the Postgres migration
tree, and curated first-party service-backed extensions own package-scoped
`ext_*` schemas in the same PostgreSQL environment.

## Decision

Move Big Rocks uses PostgreSQL as its only supported runtime database.

The runtime contract is:

- core tables live in PostgreSQL and are migrated through the Postgres migration tree
- service-backed extensions own one package-scoped PostgreSQL schema each
- extension migrations are authored by the extension and executed, locked, and
  tracked by core under `core_extension_runtime`
- deployment, verification, and instance-template tooling use `DATABASE_DSN`
  and PostgreSQL client checks only

## Consequences

### Positive

- one production storage path instead of dual SQLite/PostgreSQL behavior
- cleaner extension ownership boundaries for analytics and error-tracking
- simpler deploy and verification assets on the app host
- no CGo SQLite dependency in the runtime build

### Negative

- PostgreSQL is now a hard infrastructure requirement
- backup and PITR responsibility moves to the PostgreSQL provider or operator runbook

## Related

- [0014](0014-blue-green-deployment.md)
- [0019](0019-embed-static-assets.md)
- [RFC-0006](../RFCs/RFC-0006-postgres-and-extension-schemas.md)
