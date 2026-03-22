# ADR 0018: API Interface and Versioning Strategy

## Status

Accepted

## Decision

Move Big Rocks uses:

- `POST /graphql` as the canonical machine API
- `/v1/...` for stable REST-style integration endpoints
- `/admin/graphql` and `/admin/actions/...` for admin-panel internals

The CLI is versioned separately from HTTP path versioning. It is a client over the same GraphQL and REST surfaces.

## Consequences

- business operations stay concentrated in shared services
- integration endpoints can evolve with explicit versioning
- agent workflows do not require a separate transport-specific contract
