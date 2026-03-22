# ADR 0001: Use Architecture Decision Records

**Status:** Accepted

## Context

Architectural decisions need documentation for:
- Onboarding new team members
- Understanding why things are built a certain way
- Avoiding repeated discussions about settled decisions

Without documentation, decisions exist only in code comments (scattered), commit messages (hard to find), or institutional memory (easily lost).

## Decision

Use Architecture Decision Records (ADRs) to document significant architectural decisions.

**What qualifies as significant:**
- Affects multiple bounded contexts
- Impacts system-wide patterns (authentication, events, storage)
- Technology choices (database, frameworks)
- Establishes domain-driven design patterns
- Affects API contracts or multi-tenancy

**ADR structure:**
- Status (Accepted/Deprecated)
- Context (why we need this decision)
- Decision (what we decided)
- Consequences (what becomes easier/harder)

**Location:** `/docs/ADRs/` with sequential numbering (0001, 0002, etc.)

## Consequences

**Positive:**
- Architectural knowledge preserved and searchable
- Onboarding new developers is easier
- Context for code reviews and refactoring
- Can reference ADRs in code comments

**Negative:**
- Overhead when making architectural decisions
- Requires discipline to keep ADRs current

## References

- ADR template: `docs/ADRs/0000-template.md`
- ADR index: `docs/ADRs/README.md`
