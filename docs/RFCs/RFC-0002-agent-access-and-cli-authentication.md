# RFC-0002: Agent Access and CLI Authentication

## Status

draft

## Summary

Move Big Rocks standardises on a CLI-first agent operating model backed by GraphQL and
workspace-scoped agent tokens.

## Decisions

- GraphQL remains the canonical machine API.
- A first-party Go CLI becomes the supported operational shell.
- The core CLI remains generic; extensions declare commands and bundled skills instead of adding hardcoded product-specific verbs to core.
- Any capable agent host such as Claude Code or Codex should be able to operate Move Big Rocks through this contract.
- Non-interactive automation uses hashed `hat_*` bearer tokens.
- Interactive operator login uses a browser flow and the OS credential store when available, with secure config-file fallback when it is not.
- Milestone 1 standardises on GraphQL plus the first-party CLI as the supported agent surface.
- Optional local connectors such as OpenClaw should layer on this contract rather than bypass it.
- If Move Big Rocks exposes MCP, it is a thin adapter over the same auth, audit, and extension discovery contract rather than a separate semantics layer.

## Why

- simpler auth
- less integration drift
- easier cross-platform distribution
- easier debugging for humans and agents
- one consistent operating model across Codex, Claude Code, OpenClaw, and other capable hosts

## Milestone 1 Requirements

- `mbr auth login`
- `mbr auth whoami`
- `mbr spec export --json`
- extension command and skill discovery through generic CLI commands
- extension skill retrieval through the generic CLI
- stable JSON output for all commands
- idempotency keys on writes
- macOS, Linux, and Windows binaries
- signed releases and checksums

## MCP Criteria

Move Big Rocks does not need MCP to make agents effective in Milestone 1. If MCP is exposed, it happens only once the primary contract is stable and remains a thin adapter over that contract.

Preconditions:

- the CLI covers the main operator and agent workflows end to end
- `--json` output, exit codes, and idempotent write behavior are stable
- `mbr spec export --json` or an equivalent machine-readable capability surface exists
- extension command catalog and bundled skill discovery are stable
- auth, permissions, and audit behavior can be reused without MCP-specific exceptions
- there is concrete demand from MCP-native hosts that justifies the maintenance cost

## CLI Extensibility Model

Milestone 1 does not put extension-specific verbs such as ATS job commands into the core CLI binary.

Instead:

- extensions declare a namespaced command catalog in their manifest
- extensions may bundle operator or agent skill assets
- the core CLI exposes those declarations through generic discovery commands
- the generic `mbr extensions monitor` flow refreshes persisted runtime health for installed extensions instead of acting as a static inspect alias
- agents read skill content and then drive generic core commands, GraphQL mutations, or extension endpoints as needed

This keeps the CLI predictable for humans, stable for agents, and ready for separately shipped extensions.

## Security

- tokens stored hashed server-side
- least-privilege workspace memberships
- OS credential-store-backed operator credentials where supported, with secure fallback otherwise
- audit logging for extension and agent actions
- extension skill assets are treated as untrusted content until verified by bundle signature and install validation
