# ADR 0010: Agent API and GraphQL Architecture

## Status

Accepted

## Decision

GraphQL is the canonical machine API for Move Big Rocks, and the supported agent surface is a CLI that wraps that API.

## Consequences

- one resolver and service stack serves admin workflows, extensions, and agents
- machine access is easier to debug and script
- auth stays centered on sessions for humans and `hat_*` tokens for automation
- transport-specific complexity stays out of the product surface
