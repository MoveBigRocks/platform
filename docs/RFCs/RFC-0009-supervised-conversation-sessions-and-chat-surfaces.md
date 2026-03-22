# RFC-0009: Supervised Conversation Sessions and Chat Surfaces

## Status

draft

## Summary

Move Big Rocks should treat customer chat as a first-class operational primitive.

This RFC introduces **Conversation Sessions** as a core concept that sits
between ephemeral chat and formal case work:

- a conversation can remain a conversation
- a conversation can use knowledge and policy
- a conversation can prepare or submit forms
- a conversation can escalate into a case when needed

Public website widgets, in-app chat surfaces, and mobile SDK chat surfaces are
renderers of that same core conversation model.

Optional local-agent connectors such as OpenClaw can participate as supervised
conversation delegates, but the public customer chat surface always connects to
Move Big Rocks first.

## Why This Matters

If Move Big Rocks only treats chat as:

- a lightweight chatbot bolted onto a website, or
- a different front door to case creation,

then it misses a key product shape.

A supervised conversation session is different from a case:

- it is live and conversational
- it may not require escalation
- it may gather information gradually
- it may cite knowledge and follow policy in real time
- it may fill forms on behalf of the customer
- it may hand off to a human or create a case only when that is operationally correct

This makes Move Big Rocks a controlled operational broker between:

- the end customer
- Move Big Rocks's knowledge and forms model
- a workspace's policies
- human operators
- optional agent runtimes such as OpenClaw

## Product Rule

Conversation Sessions are core.

The public chat widget is not the core primitive. It is one renderer.

Cases remain core too, but a case is an escalation or operational work item,
not the same thing as the conversation that may lead to it.

The catalog node that describes the service or request domain is a separate
primitive again. A conversation or case can point to that node without becoming
identical to it.

## Core Responsibilities

Core Move Big Rocks owns:

- conversation session records
- conversation transcripts and message metadata
- participant identity and contact linkage
- knowledge retrieval and citation during the session
- policy enforcement during the session
- forms draft and form submission flows during the session
- escalation from conversation to case
- operator visibility, audit, and replay
- public widget and mobile chat protocol boundaries
- routing to a human or a supervised local agent delegate

Extensions can contribute:

- knowledge resources
- form specs
- conversation policies
- domain-specific actions
- domain-specific routing and escalation rules

OpenClaw or another local agent runtime can:

- help answer
- help classify
- help retrieve knowledge
- help fill forms
- propose actions

But Move Big Rocks remains the system of record and the policy gate.

## Domain Model

### Entity: `ConversationSession`

- `id: UUID`
- `workspace_id: UUID`
- `channel: enum`
  - `web-widget`
  - `mobile-sdk`
  - `operator-console`
  - `agent-console`
- `status: enum`
  - `active`
  - `waiting`
  - `escalated`
  - `closed`
  - `blocked`
- `contact_id: UUID?`
- `linked_case_id: UUID?`
- `assigned_operator_id: UUID?`
- `assigned_local_connector_id: UUID?`
- `policy_profile_ref: string?`
- `knowledge_context: jsonb`
- `form_context: jsonb`
- `escalation_state: jsonb`
- `metadata: jsonb`
- `started_at: timestamptz`
- `last_activity_at: timestamptz`
- `closed_at: timestamptz?`

Business rules:

- a conversation session is not automatically a case
- a conversation may link to a case later
- a conversation may be handled by a human, a local agent delegate, or a hybrid flow
- all meaningful actions taken during the conversation are auditable

### Entity: `ConversationMessage`

- `id: UUID`
- `conversation_session_id: UUID`
- `role: enum`
  - `customer`
  - `mbr`
  - `operator`
  - `agent`
  - `system`
- `kind: enum`
  - `text`
  - `tool-call`
  - `tool-result`
  - `knowledge-citation`
  - `form-update`
  - `escalation-event`
- `content_text: text`
- `content_json: jsonb`
- `citations_json: jsonb`
- `created_at: timestamptz`

Business rules:

- customer-visible replies and internal system actions are both part of the session history
- citations and tool outcomes must be inspectable
- local-agent replies must remain attributable to the local connector path that produced them

### Entity: `ConversationPolicyProfile`

- `id: UUID`
- `workspace_id: UUID`
- `name: string`
- `knowledge_resource_ids: jsonb`
- `allowed_actions: jsonb`
- `disallowed_actions: jsonb`
- `escalation_rules: jsonb`
- `form_spec_ids: jsonb`
- `approval_rules: jsonb`
- `created_at: timestamptz`
- `updated_at: timestamptz`

Business rules:

- a conversation session can run under an explicit policy profile
- policy profiles define which knowledge, form specs, and actions are available
- higher-risk actions require escalation or approval

### Entity: `ConversationOutcome`

- `id: UUID`
- `conversation_session_id: UUID`
- `kind: enum`
  - `resolved-in-conversation`
  - `form-draft-created`
  - `form-submitted`
  - `case-created`
  - `case-updated`
  - `human-handoff`
- `result_ref: jsonb`
- `created_at: timestamptz`

## Widget and SDK Model

Move Big Rocks should offer a simple public conversation surface:

- website chat widget
- mobile SDK conversation client
- embeddable web conversation component

Those surfaces connect to Move Big Rocks, not directly to OpenClaw.

The public client receives:

- a short-lived visitor session token
- conversation session ID
- streaming or polled replies from Move Big Rocks
- safe state transitions such as "collect more info", "submit forms", "handoff", or "create case"

This keeps the customer-facing surface:

- stable
- auditable
- provider-neutral
- secure even when a local agent connector is involved

## Relationship to Knowledge and Forms

This RFC depends directly on RFC-0007.

Conversation sessions use:

- **Knowledge Resources** as live policy, procedure, and guidance inputs
- **Form Specs** as machine-readable contracts for what can be collected or submitted

That means a conversation can:

- answer a question using knowledge
- ask follow-up questions required by an form spec
- fill an forms draft on behalf of the customer
- submit the forms when policy allows
- escalate to a case if the outcome requires operational follow-through

## Relationship to the Service Catalog

This RFC also composes with RFC-0010.

The clean fit is:

- `ConversationSession` answers "what is happening in this live exchange?"
- `Case` answers "what operational work item exists now?"
- `ServiceCatalogNode` answers "which service or request domain does this belong to?"

That means a conversation session should be able to carry:

- `primary_catalog_node_id`
- `suggested_catalog_node_ids`

And a case created from a conversation should inherit the resolved primary
catalog node where appropriate.

## Relationship to OpenClaw

This RFC composes with RFC-0008.

The recommended shape is:

1. customer opens a chat widget backed by Move Big Rocks
2. Move Big Rocks creates or resumes a `ConversationSession`
3. Move Big Rocks retrieves the relevant policy profile, knowledge resources, and form specs
4. Move Big Rocks optionally delegates bounded assistance to a linked local OpenClaw connector
5. Move Big Rocks returns a controlled response to the customer
6. Move Big Rocks records transcript, citations, and any proposed or executed actions
7. Move Big Rocks escalates into forms or a case when needed

OpenClaw does not become the public chat endpoint. It becomes a supervised
participant behind Move Big Rocks.

## Data Flow

### Example: Support chat resolved without case creation

```text
Customer opens website chat
    ↓
Move Big Rocks creates ConversationSession
    ↓
Move Big Rocks retrieves policy knowledge and relevant support guidance
    ↓
Move Big Rocks or a supervised local delegate replies with citations
    ↓
Conversation resolves
    ↓
ConversationOutcome(kind = resolved-in-conversation)
```

### Example: Chat fills forms and creates case

```text
Customer opens mobile chat
    ↓
Move Big Rocks determines that a complaint or request forms applies
    ↓
Conversation gathers required fields and evidence
    ↓
Move Big Rocks prepares FormSubmission
    ↓
Policy allows submission
    ↓
Move Big Rocks submits forms and creates case
    ↓
Conversation links to created case
    ↓
Operator sees transcript, forms, and case together
```

### Example: Chat delegated to local OpenClaw

```text
Customer opens website chat
    ↓
Move Big Rocks applies workspace conversation policy
    ↓
Move Big Rocks dispatches bounded assistance task to linked OpenClaw connector
    ↓
OpenClaw retrieves knowledge and prepares reply or forms updates
    ↓
Move Big Rocks validates policy and sends customer-visible response
    ↓
Any escalation or write action remains recorded and controlled in Move Big Rocks
```

## API Contract

GraphQL additions:

- `conversationSessions(workspaceId: ID!): [ConversationSession!]!`
- `conversationSession(id: ID!): ConversationSession`
- `conversationMessages(sessionId: ID!): [ConversationMessage!]!`
- `startConversationSession(input: StartConversationSessionInput!): ConversationSession!`
- `sendConversationMessage(input: SendConversationMessageInput!): ConversationMessage!`
- `escalateConversationToCase(input: EscalateConversationToCaseInput!): ConversationOutcome!`

Public HTTP and realtime additions:

- `POST /api/public/conversations`
- `POST /api/public/conversations/:id/messages`
- streaming endpoint for assistant responses
- website widget bootstrap endpoint

CLI additions:

- `mbr conversations list`
- `mbr conversations show`
- `mbr conversations messages`
- `mbr conversations escalate`

## Security Model

Rules:

- customer-facing chat connects to Move Big Rocks, not to a local agent runtime
- every conversation session is scoped to a workspace and policy profile
- every customer-visible reply is attributable to Move Big Rocks, a human operator, or a supervised agent path
- high-risk actions require approval or case escalation
- transcripts, citations, forms changes, and escalations are auditable
- local connectors remain optional and revocable

## Why This Belongs In Core

This should not be just another extension because:

- it is a cross-cutting operational primitive
- it touches cases, knowledge, forms, contacts, and audit
- it is likely to become the main customer-facing interaction model
- many extensions will want to participate in it rather than reinvent it

Extensions should shape conversation behavior, not own the base conversation
primitive.

## Acceptance Criteria

- Move Big Rocks can host a customer-facing conversation session that is distinct from a case
- a conversation session can use knowledge resources and form specs during the live exchange
- a conversation can remain conversational or escalate into a case based on policy
- a website widget or mobile chat surface can connect to Move Big Rocks without exposing a local agent runtime
- OpenClaw can participate as a supervised delegate behind Move Big Rocks
- operators can inspect transcripts, citations, forms effects, and case escalation from one place
