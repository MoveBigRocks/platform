# ADR 0015: Workspace-Scoped Agent Access

## Status

Accepted

## Decision

Agent and extension access is scoped to a single workspace per credential set.

## Rationale

- workspace remains the hard tenant boundary
- permissions stay easy to reason about
- extension installations are explicit and local to a workspace
- cross-workspace automation should use explicit profiles or repeated invocations, not broad bearer grants
