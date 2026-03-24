# Working with Claude on Move Big Rocks Platform

**Company:** DemandOps
**Repository:** Move Big Rocks — context engineering platform for operational teams

## Critical Rules

1. **NO AI ATTRIBUTION IN GIT COMMITS.** No `Co-Authored-By`, no `Generated with`, nothing. Commits will be rejected.
2. **ALL TESTS MUST PASS** — `make test-all`. No shortcuts.
3. **PRODUCTION READY CODE ONLY** — no conceptual or incomplete implementations.
4. **RFC REQUIRED FOR NEW FEATURES** — copy `docs/RFCs/RFC-0000-template.md`, get approval before implementation. Not required for bug fixes, refactoring, docs, CI, or dependency updates.
5. **NO NEW .MD FILES** unless explicitly requested.

## Architecture

Layered DDD: Handler → Service → Domain → Store.

- Handlers: HTTP only, call services
- Services: orchestrate, call domain methods and stores
- Domain: business logic, validation, state transitions
- Stores: persistence abstraction

**Forbidden:** handlers importing stores directly, business logic in services, domain importing infrastructure.

ADRs in `docs/adr/`. RFCs in `docs/RFCs/`.

## Development

- `make test-all` — run all tests
- `make build` — build
- `make setup-hooks` — install pre-commit hooks

## Repo Context

See `.ai-context` for the full system description, source-of-truth stack, core concepts, persistence shape, and documentation policy.

## Deployment Model

This repo builds OCI artifacts on push to main (test → build → semver tag → ghcr.io). It does NOT deploy directly. Deployment happens from private instance repos (e.g. `DemandOps/mbr-prod`) that pin artifact versions in `mbr.instance.yaml`. See `docs/CUSTOMER_INSTANCE_SETUP.md` and `docs/RELEASE_ARTIFACT_CONTRACT.md`.
