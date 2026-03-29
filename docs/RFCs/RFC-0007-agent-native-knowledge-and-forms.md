# RFC-0007: Agent-Native Knowledge and Forms

## Status

draft

## Summary

Move Big Rocks treats `knowledge` and `forms` as agent-native operational primitives,
not SaaS-era UI objects.

This RFC introduces two core concepts:

- **Knowledge Resources**: Markdown-first, metadata-rich, provenance-aware
  resources that humans and agents can retrieve, cite, and ship through
  extensions.
- **Form Specs**: agent-readable form contracts that define what data,
  evidence, policy, and approvals are required before Move Big Rocks accepts work or
  mutates state.

Web forms, chat capture, email capture, CLI capture, imports, and
extension-owned surfaces are renderers of the same forms model.

This RFC does not introduce a resident on-box model runtime or local agent
daemon. That is a separate platform decision.

## Product Rule

Move Big Rocks keeps human-friendly knowledge pages and form pages, but rebases them on
two machine-first primitives:

- `KnowledgeResource`
- `FormSpec`

The human UI is one consumer of those primitives. Agents, CLI flows, imports,
email processing, case workflows, and extensions are peer consumers.

In Move Big Rocks:

- knowledge is scoped operational context with provenance
- forms is a contract for what must be known before Move Big Rocks accepts work or executes an action

## Domain Model

### Entity: `KnowledgeResource`

- `id: UUID`
- `workspace_id: UUID?`
- `scope: enum`
  - `instance`
  - `workspace`
  - `extension-install`
- `owner_kind: enum`
  - `core`
  - `workspace`
  - `extension`
- `owner_ref: string`
- `slug: string`
- `title: string`
- `kind: enum`
  - `policy`
  - `procedure`
  - `playbook`
  - `troubleshooting`
  - `faq`
  - `agent-skill`
  - `domain-context`
  - `template-guidance`
- `format: enum`
  - `markdown`
- `body_markdown: text`
- `frontmatter_json: jsonb`
- `applies_when: jsonb`
  Context selectors such as case labels, collection, extension, entity type,
  workflow stage, customer segment, or event type.
- `related_entity_refs: jsonb`
- `related_form_spec_ids: jsonb`
- `source_kind: enum`
  - `human-authored`
  - `extension-bundled`
  - `imported`
  - `generated`
- `source_ref: string`
- `trust_level: enum`
  - `draft`
  - `reviewed`
  - `system`
  - `extension-signed`
- `reviewed_by: UUID?`
- `reviewed_at: timestamptz?`
- `supersedes_resource_id: UUID?`
- `effective_from: timestamptz?`
- `effective_to: timestamptz?`
- `visibility: enum`
  - `private`
  - `workspace`
  - `public`
- `created_at: timestamptz`
- `updated_at: timestamptz`
- `deleted_at: timestamptz?`

Business rules:

- every knowledge resource has provenance
- extension-bundled resources are immutable source artifacts; workspace changes are explicit overlays instead of edits to signed bundle content
- retrieval respects workspace, visibility, extension activation, and trust
- generated resources exist, but carry lower trust until reviewed

### Entity: `FormSpec`

- `id: UUID`
- `workspace_id: UUID`
- `owner_kind: enum`
  - `core`
  - `workspace`
  - `extension`
- `owner_ref: string`
- `slug: string`
- `title: string`
- `description: text`
- `status: enum`
  - `draft`
  - `active`
  - `archived`
- `target_kind: enum`
  - `case`
  - `contact-update`
  - `case-action`
  - `extension-entity`
  - `extension-action`
- `target_ref: string`
- `input_schema: jsonb`
  Canonical required and optional fields.
- `inference_policy: jsonb`
  Which fields can be inferred, by whom, and from which sources.
- `evidence_requirements: jsonb`
  Which claims require attachments, quoted text, IDs, prior records, or
  explicit user confirmation.
- `validation_policy: jsonb`
- `approval_policy: jsonb`
- `routing_policy: jsonb`
  What collection, assignee, extension endpoint, or workflow receives the
  submission.
- `knowledge_hints: jsonb`
  Retrieval hints for relevant knowledge resources.
- `renderers: jsonb`
  Declared supported renderers such as web form, admin form, chat, email,
  import, CLI.
- `submission_mode: enum`
  - `draftable`
  - `direct-submit`
  - `requires-review`
- `created_at: timestamptz`
- `updated_at: timestamptz`
- `deleted_at: timestamptz?`

Business rules:

- form specs are renderer-agnostic
- an agent may prepare a draft even when required fields are missing
- final submission satisfies the form spec's validation, evidence, and approval policy
- extensions can contribute form specs, but core enforces workspace, permission, and audit boundaries

### Entity: `FormSubmission`

- `id: UUID`
- `workspace_id: UUID`
- `form_spec_id: UUID`
- `status: enum`
  - `draft`
  - `ready`
  - `submitted`
  - `rejected`
  - `completed`
- `submitted_via: enum`
  - `web`
  - `admin`
  - `cli`
  - `email`
  - `agent`
  - `import`
- `supplied_data: jsonb`
- `inferred_data: jsonb`
- `evidence_data: jsonb`
- `missing_requirements: jsonb`
- `validation_results: jsonb`
- `approval_state: jsonb`
- `result_ref: jsonb`
  Created case ID, updated contact ID, extension-owned entity ID, or action
  receipt.
- `linked_case_id: UUID?`
- `linked_contact_id: UUID?`
- `submitter_principal_id: string?`
- `created_at: timestamptz`
- `updated_at: timestamptz`

Business rules:

- drafts can be partial and collaborative
- the final submitted payload preserves provenance between supplied and inferred values
- Move Big Rocks can explain why a submission was accepted, rejected, or routed to review

### Derived UI Concepts

These stay useful but become secondary:

- `FormSurface`
  A declared renderer of an `FormSpec`
- `KnowledgeLibrary`
  A navigational grouping of `KnowledgeResource` records

## Extension Model Changes

Extensions can publish:

- bundled knowledge resources as Markdown plus frontmatter
- form specs for extension-owned entities and actions
- explicit references between their knowledge and forms artifacts

Extension manifests support:

- `knowledgeResources`
- `formSpecs`
- declarative asset references for Markdown resources

Rules:

- standard-risk extensions can publish workspace-scoped knowledge and form specs
- core owns permissions, audit, workspace isolation, and write execution
- privileged connectors and identity extensions remain separate concerns

## API Contract

GraphQL schema additions:

```graphql
type KnowledgeResource {
  id: ID!
  workspaceId: ID
  scope: String!
  ownerKind: String!
  ownerRef: String!
  slug: String!
  title: String!
  kind: String!
  format: String!
  bodyMarkdown: String!
  frontmatterJson: JSON!
  appliesWhen: JSON!
  relatedEntityRefs: JSON!
  relatedFormSpecIds: [ID!]!
  sourceKind: String!
  sourceRef: String
  trustLevel: String!
  reviewedAt: Time
  visibility: String!
}

type FormSpec {
  id: ID!
  workspaceId: ID!
  ownerKind: String!
  ownerRef: String!
  slug: String!
  title: String!
  description: String
  status: String!
  targetKind: String!
  targetRef: String!
  inputSchema: JSON!
  inferencePolicy: JSON!
  evidenceRequirements: JSON!
  validationPolicy: JSON!
  approvalPolicy: JSON!
  routingPolicy: JSON!
  knowledgeHints: JSON!
  renderers: JSON!
  submissionMode: String!
}

type FormSubmission {
  id: ID!
  workspaceId: ID!
  formSpecId: ID!
  status: String!
  submittedVia: String!
  suppliedData: JSON!
  inferredData: JSON!
  evidenceData: JSON!
  missingRequirements: JSON!
  validationResults: JSON!
  approvalState: JSON!
  resultRef: JSON!
}

extend type Query {
  knowledgeResources(workspaceId: ID!, query: String, context: JSON): [KnowledgeResource!]!
  formSpecs(workspaceId: ID!, targetKind: String, ownerKind: String): [FormSpec!]!
  formSubmission(id: ID!): FormSubmission
}

extend type Mutation {
  createKnowledgeResource(input: CreateKnowledgeResourceInput!): KnowledgeResource!
  createFormSpec(input: CreateFormSpecInput!): FormSpec!
  prepareFormSubmission(input: PrepareFormSubmissionInput!): FormSubmission!
  submitFormSubmission(id: ID!): FormSubmission!
}
```

CLI additions:

- `mbr knowledge list`
- `mbr knowledge show`
- `mbr knowledge search`
- `mbr form specs list`
- `mbr form specs show`
- `mbr forms drafts prepare`
- `mbr forms drafts validate`
- `mbr forms drafts submit`

These commands remain thin wrappers over the same GraphQL and HTTP contract.

Renderer-specific public forms routes exist when needed, but they map to
`FormSpec` execution under the hood.

## Data Flow

### Example: Customer Complaint Captured by an Agent

```text
Customer message arrives
    ↓
Case or interaction is opened in Move Big Rocks
    ↓
Agent asks Move Big Rocks which form spec applies
    ↓
Move Big Rocks retrieves matching FormSpec + relevant KnowledgeResources
    ↓
Agent prepares FormSubmission draft
    ↓
Move Big Rocks validates required data, evidence, and approval policy
    ↓
Submission creates or updates case or extension-owned action
    ↓
Follow-up automation and extension events run
```

### Example: Extension-Owned Procedure

```text
Extension activates in workspace
    ↓
Bundle installs knowledge resources + form specs
    ↓
Agent works a case tied to that extension
    ↓
Move Big Rocks retrieves extension-scoped and workspace-scoped knowledge
    ↓
Agent follows the matching form spec
    ↓
Core records final result and audit trail
```

## Storage

Tables under `core_knowledge`:

- `knowledge_resources`
- `knowledge_resource_links`
- `knowledge_resource_overlays`

Tables under `core_service`:

- `form_specs`
- `form_spec_renderers`
- `form_submissions`
- `form_submission_events`

Knowledge pages and public forms surfaces remain legitimate UI surfaces, but they are views
over these underlying primitives.

## ADR Compliance

| ADR | Title | Compliance |
|-----|-------|------------|
| 0005 | Event-Driven Architecture | Form submission and extension follow-up remain event-driven instead of introducing side channels. |
| 0009 | Code Architecture Patterns | Keeps knowledge and forms as explicit bounded context concepts rather than scattered renderer logic. |
| 0010 | Agent API and GraphQL Architecture | Uses GraphQL and CLI as the canonical machine contract. |
| 0015 | Workspace-Scoped Agent Access | Retrieval and submission remain workspace-scoped and permissioned. |
| 0021 | PostgreSQL-Only Runtime and Extension-Owned Schemas | Shared primitives stay in core schemas; extension-owned entities remain in extension-owned schemas. |
| 0023 | Core PostgreSQL Bounded-Context Schemas | Knowledge and service continue to own their respective tables. |
| 0024 | PostgreSQL Migration Ledgers and Identifier Ownership | New records use core migration ledgers and preserve explicit ownership. |

## Alternatives Considered

### Alternative 1: Keep document-centric knowledge and renderer-centric forms

**Pros:** lower schema churn, simpler short-term implementation  
**Cons:** preserves the wrong primitive shape  
**Why rejected:** it improves the middle layer rather than moving Move Big Rocks to an
agent-native design by construction

### Alternative 2: Push knowledge and forms entirely into extensions

**Pros:** smaller core surface  
**Cons:** every extension reinvents retrieval, validation, provenance, and
agent-facing contracts  
**Why rejected:** knowledge and forms are cross-cutting operational primitives

### Alternative 3: Make Markdown files on disk the only source of truth

**Pros:** agent-friendly authoring, easy extension bundling, simple mental model  
**Cons:** weak runtime queryability, awkward audit and workspace overlay model  
**Why rejected:** Markdown is the preferred authoring format for many
resources, but Move Big Rocks needs database-native indexing, linkage, and audit state

## Acceptance Criteria

- Move Big Rocks can answer "what knowledge applies here?" with provenance-aware
  resources rather than flat document search results
- Move Big Rocks can answer "what information is required?" for a pending forms action
- extensions can ship both knowledge and forms artifacts without inventing
  their own parallel systems
- knowledge pages and public forms pages remain natural UI surfaces without
  defining the underlying product model
