# RFC-NNNN: Title

**Status:** draft
**Author:** @username
**Created:** YYYY-MM-DD

## Summary

One paragraph description of the feature or change.

## Problem Statement

What problem are we solving? Why now?

- Current pain points or limitations
- User or business need driving this change
- Why existing solutions are insufficient

## Proposed Solution

### Overview

High-level description of the solution approach.

### Domain Model Changes

Describe any new or modified entities:

```
Entity: EntityName
- field1: Type (description)
- field2: Type (description)

State transitions:
  INITIAL → ACTIVE → COMPLETED
```

Business rules and invariants:
- Rule 1: description
- Rule 2: description

### API Contract

GraphQL schema additions/changes:

```graphql
type NewType {
  id: ID!
  field: String!
}

extend type Mutation {
  newMutation(input: NewInput!): NewType!
}
```

REST endpoints (if applicable):
- `POST /v1/endpoint` - description

### Data Flow

```
User Action
    ↓
API Layer (Handler/Resolver)
    ↓
Service Layer
    ↓
Domain (Business Logic)
    ↓
Store (Persistence)
    ↓
Response
```

Storage changes:
- Database: new tables/columns
- File storage: new prefixes/buckets
- Caching: strategy

## ADR Compliance

| ADR | Title | Compliance |
|-----|-------|------------|
| NNNN | Title | How this RFC complies |

## Alternatives Considered

### Alternative 1: Name

Description of alternative approach.

**Pros:** advantages
**Cons:** disadvantages
**Why rejected:** reason

## Verification Criteria

### Unit Tests
- [ ] Test: description
- [ ] Test: description

### Integration Tests
- [ ] Test: description

### Acceptance Criteria
- [ ] Criterion 1: specific, measurable outcome
- [ ] Criterion 2: specific, measurable outcome

## Implementation Checklist

- [ ] RFC approved
- [ ] Domain types created
- [ ] Repository/store layer implemented
- [ ] Service layer implemented
- [ ] API handlers/resolvers implemented
- [ ] Database migrations created
- [ ] Tests written and passing
- [ ] Documentation updated
- [ ] Code reviewed
- [ ] Deployed to staging
- [ ] Verified against acceptance criteria
- [ ] Deployed to production
- [ ] RFC status updated to `verified`

## Open Questions

- [ ] Question 1?
- [ ] Question 2?

## Related

- **ADRs:** links to relevant ADRs
- **Supersedes:** RFC-NNNN (if replacing another RFC)

---

## Changelog

| Date | Author | Change |
|------|--------|--------|
| YYYY-MM-DD | @username | Initial draft |
