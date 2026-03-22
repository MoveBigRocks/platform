# RFC-0010: Agent-Native Service Catalog and Operational Taxonomy

## Status

draft

## Summary

Move Big Rocks should introduce a core **Service Catalog** primitive that gives
conversations, forms, and cases a shared operational taxonomy.

The catalog is not just a storefront of request forms. It is a structured tree
of services, capabilities, request domains, and operational topics that Move Big Rocks
can use for:

- customer navigation
- agent classification
- knowledge retrieval
- forms selection
- routing
- entitlement checks
- analytics

Both a `ConversationSession` and a `Case` can point to the same catalog node.

## Why This Matters

Today, Move Big Rocks cases carry lightweight classification fields such as:

- `Category string`
- `CollectionID string`
- `Tags []string`

Those are useful, but they do not form a strong operational model.

An agent-native system needs one canonical answer to:

- what service is this about?
- what request or issue type does this belong to?
- what knowledge applies here?
- which form spec applies here?
- where should this route?
- which users or customers are allowed to see or request this?

That is what the service catalog should do.

## Product Rule

The service catalog is a core taxonomy primitive.

It is not only for human portal navigation. It is also for:

- agent classification
- policy selection
- forms binding
- case routing
- entitlement and visibility logic

Cases, conversation sessions, and forms should all be able to attach to a
catalog node.

## Domain Model

### Entity: `ServiceCatalogNode`

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
- `parent_node_id: UUID?`
- `slug: string`
- `title: string`
- `description_markdown: text`
- `node_kind: enum`
  - `domain`
  - `service`
  - `request-type`
  - `issue-type`
  - `service-offering`
  - `playbook-entry`
- `status: enum`
  - `active`
  - `hidden`
  - `archived`
- `visibility: enum`
  - `public`
  - `workspace`
  - `restricted`
- `supported_channels: jsonb`
  Examples: web portal, chat, mobile app, operator console, API.
- `default_case_category: string?`
- `default_collection_id: UUID?`
- `default_priority: string?`
- `knowledge_resource_ids: jsonb`
- `form_spec_ids: jsonb`
- `policy_profile_ref: string?`
- `entitlement_rules: jsonb`
- `routing_rules: jsonb`
- `search_keywords: jsonb`
- `display_order: integer`
- `created_at: timestamptz`
- `updated_at: timestamptz`

Business rules:

- the catalog is hierarchical
- internal nodes organize discovery and classification
- leaf nodes define concrete request or issue types
- a node can be visible to humans, agents, or both depending on channel and visibility rules

### Entity: `ServiceCatalogBinding`

This is the relationship layer between the catalog and other operational
records.

- `id: UUID`
- `workspace_id: UUID`
- `catalog_node_id: UUID`
- `target_kind: enum`
  - `conversation-session`
  - `case`
  - `form-spec`
  - `knowledge-resource`
  - `contact-entitlement`
- `target_id: UUID`
- `binding_kind: enum`
  - `primary`
  - `secondary`
  - `suggested`
  - `historical`
- `confidence: numeric?`
- `created_at: timestamptz`

Business rules:

- a conversation or case should have one primary catalog node when classification is known
- additional suggested or secondary bindings can exist during live triage
- agents can propose a catalog binding, but Move Big Rocks records confidence and audit

### Entity: `ServiceEntitlement`

- `id: UUID`
- `workspace_id: UUID`
- `contact_id: UUID`
- `catalog_node_id: UUID`
- `status: enum`
  - `active`
  - `expired`
  - `suspended`
- `details_json: jsonb`
- `effective_from: timestamptz?`
- `effective_to: timestamptz?`
- `created_at: timestamptz`
- `updated_at: timestamptz`

Business rules:

- entitlements are optional but important for service-aware support
- a catalog node may require an entitlement before a request can be submitted or a conversation can take a particular action

## Relationship to Existing Concepts

### Cases

A case should be able to point to a primary `ServiceCatalogNode`.

That means:

- `Category` becomes a lightweight display field
- `CollectionID` remains a routing or queue outcome
- tags remain flexible metadata
- the service catalog node becomes the stronger semantic classification

So the model becomes:

- catalog node answers "what service/request domain is this?"
- collection answers "which queue or bucket owns the work?"
- tags answer "what flexible labels or states apply?"

### Conversation Sessions

A conversation session should also point to a primary `ServiceCatalogNode`, or
at least a suggested one while classification is still emerging.

That gives the conversation:

- the relevant policy profile
- the likely knowledge resources
- the likely form specs
- the likely escalation target if a case is needed

### Knowledge Resources

Knowledge resources do not belong inside the catalog tree by default, but they
can be bound to catalog nodes.

This is the key distinction:

- catalog node = "what service or request area is this?"
- knowledge resource = "what guidance applies here?"

That lets one service node point to many knowledge resources:

- policy
- troubleshooting
- playbook
- FAQ
- agent skill

### Form Specs

Form specs should bind to catalog nodes too.

That means a node can say:

- these are the request types available here
- these are the form specs the agent or customer can complete
- these are the evidence and approval rules that apply

### Contacts and Users

Contacts relate to the service catalog through entitlements and history.

Examples:

- this customer can request support for `Product A / Billing`
- this customer has no entitlement for `Premium Escalation`
- this customer already has an active service relationship under this node

Users and operators relate through routing and ownership:

- which team owns this node
- which collection receives escalations from this node
- which operators or agents are preferred handlers

## Authoring Model

The best Move Big Rocks model is **Markdown-first authoring with database-native
runtime state**.

That means a service catalog node can be authored from Markdown plus
frontmatter, for example:

```md
---
slug: billing-refunds
title: Billing / Refunds
node_kind: request-type
parent: billing
supported_channels: [portal, chat, mobile]
knowledge_resources:
  - refund-policy
  - billing-faq
form_specs:
  - refund-request
routing:
  collection: billing-support
  priority: medium
---

Refund requests, refund eligibility, and refund dispute handling.
```

But Markdown should not be the only runtime representation.

Move Big Rocks still needs:

- indexed tree traversal
- visibility and entitlement checks
- bindings to cases, conversations, knowledge, and forms
- audit and activation behavior
- extension publication

So the clean model is:

- Markdown and frontmatter for authoring and bundling
- PostgreSQL for runtime graph, indexing, and bindings

## Best-Practice Direction From Leading Platforms

The best patterns across the major platforms are broadly consistent:

- a structured catalog or request taxonomy
- restricted visibility by user, group, organization, or entitlement
- reusable request fields
- routing and fulfillment metadata attached to catalog items
- help-center or portal discoverability
- chat and self-service sitting on top of the same request model

What Move Big Rocks should copy:

- structured categories and service items
- visibility and entitlement rules
- request-type level routing
- asset or product context where relevant

What Move Big Rocks should not copy:

- treating the catalog as a static portal menu only
- making request forms the primary model
- separating chat, knowledge, and request management into different silos

## Move Big Rocks Design Rule

The service catalog should be:

- simple enough for humans to browse
- structured enough for agents to classify against
- expressive enough to bind knowledge, forms, policy, routing, and entitlements
- composable enough for extensions to publish into it

## Acceptance Criteria

- Move Big Rocks can represent a tree of services and request domains
- a case can bind to a catalog node
- a conversation session can bind to a catalog node
- a catalog node can bind to knowledge resources and form specs
- catalog nodes can carry routing and entitlement rules
- the catalog can be authored from Markdown and frontmatter without making Markdown the only runtime source of truth
