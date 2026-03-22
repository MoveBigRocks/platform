# ADR 0016: CLI and Agent Authentication Guidelines

## Status

Accepted

## Decision

Move Big Rocks uses:

- browser sessions for humans
- `hat_*` bearer tokens for automation and extensions
- OS credential-store-backed storage for interactive CLI credentials where available, with secure fallback otherwise

## Operational Rules

- every machine-facing command supports `--json`
- write operations should be idempotent where practical
- credentials must never be stored in plaintext on the server
- agent actions and extension lifecycle actions must be audited
