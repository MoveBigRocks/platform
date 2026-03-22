# RFC-0001: Rate Limiting Model

## Status

draft

## Summary

Move Big Rocks enforces rate limits as an infrastructure concern with one clear rule:
the enforcement backend must match the deployment topology.

For a single application instance, in-memory counters are acceptable. For any
topology where requests are served by more than one Move Big Rocks process, rate-limit
state must move to a shared backend.

## Decision

Move Big Rocks supports two valid rate-limiting modes:

### 1. Single-Instance Mode

Use instance-local in-memory counters when:

- one Move Big Rocks application process serves requests
- restart-level reset semantics are acceptable
- operational simplicity is preferred over shared-state durability

Properties:

- zero additional infrastructure
- fast local checks
- no cross-process coordination

### 2. Shared-State Mode

Use a shared backend when:

- more than one Move Big Rocks application process serves traffic
- rate-limit guarantees must survive process restarts
- enforcement must remain consistent across nodes

Valid shared backends:

- PostgreSQL-backed counters
- Redis-backed counters

Properties:

- cross-instance enforcement
- restart durability
- explicit operational dependency on the shared backend

## Selection Rules

- Single-server, single-process Move Big Rocks uses in-memory rate limiting by default.
- Multi-process or multi-node Move Big Rocks uses a shared-state backend.
- Move Big Rocks does not present instance-local rate limiting as a distributed guarantee.

## Backend Characteristics

### PostgreSQL-Backed Counters

Use when:

- Move Big Rocks already depends on PostgreSQL and request volume is moderate
- operators want one fewer infrastructure dependency

Tradeoffs:

- simple deployment story
- durable shared state
- higher write load on the database

### Redis-Backed Counters

Use when:

- request volume is high
- lower-latency shared coordination matters
- operators accept a dedicated cache dependency

Tradeoffs:

- strong performance
- flexible algorithms such as sliding windows and token buckets
- additional infrastructure to run and secure

## Implementation Rules

- rate-limit state is owned by infrastructure packages, not domain logic
- middleware and auth flows consume a rate-limiter interface, not a concrete backend
- expiry and cleanup behavior are explicit for every backend
- deployment docs state which mode a given installation is using

## Non-Goals

- pretending a local in-memory limiter is sufficient for distributed deployments
- baking deployment-topology assumptions into domain services
- forcing one shared backend choice for every installation

## Acceptance Criteria

- single-instance Move Big Rocks enforces local limits without extra infrastructure
- multi-instance Move Big Rocks uses a shared backend for rate-limit state
- docs and health reporting make the selected mode explicit
