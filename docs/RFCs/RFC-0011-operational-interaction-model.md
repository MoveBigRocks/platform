# RFC-0011: Operational Interaction Model

## Status

draft

## Summary

This RFC defines the concrete object graph for customer conversations, service
catalog classification, forms, case escalation, and agent-runtime delegation.

The goal is one clean operational model where:

- customers can start with a conversation
- Move Big Rocks can classify that conversation against a service catalog
- Move Big Rocks can retrieve the right knowledge and forms
- Move Big Rocks can delegate bounded help to an agent runtime such as OpenClaw
- Move Big Rocks can escalate into a case when operational work is required

For the concrete PostgreSQL storage, indexing, and retention strategy behind
this model, see [RFC-0012](RFC-0012-conversation-storage-and-query-strategy.md).

## Design Principle

The system should answer five different questions with five different
primitives:

- `ConversationSession`: what is happening in this live interaction?
- `ServiceCatalogNode`: what service or request domain does this belong to?
- `KnowledgeResource`: what guidance applies here?
- `FormSpec` and `FormSubmission`: what information is required and what has been collected?
- `Case`: what operational work item now exists?

Those are related, but they are not the same object.

## Core Object Graph

```text
Workspace
  ├─ Contacts
  ├─ ServiceCatalogNodes
  ├─ KnowledgeResources
  ├─ FormSpecs
  ├─ ConversationSessions
  │    ├─ ConversationParticipants
  │    ├─ ConversationMessages
  │    ├─ ConversationWorkingState
  │    └─ ConversationOutcomes
  ├─ Cases
  ├─ Collections
  └─ AgentRuntimeConnectors
```

The important shared references are:

- `ConversationSession.primary_catalog_node_id`
- `ConversationSession.primary_contact_id`
- `ConversationSession.active_form_submission_id`
- `ConversationSession.linked_case_id`
- `Case.primary_catalog_node_id`
- `Case.originating_conversation_session_id`
- `ServiceCatalogNode -> KnowledgeResource`
- `ServiceCatalogNode -> FormSpec`
- `ServiceCatalogNode -> routing policy -> Collection`

## Concrete Domain Model

### 1. `ConversationSession`

This is the container for a live interaction.

Suggested fields:

- `id`
- `workspace_id`
- `channel`
  - `web_widget`
  - `mobile_sdk`
  - `operator_console`
  - `agent_console`
- `status`
  - `active`
  - `waiting_for_customer`
  - `waiting_for_operator`
  - `ready_to_submit_form`
  - `escalated_to_case`
  - `resolved`
  - `closed`
  - `blocked`
- `primary_contact_id`
- `primary_catalog_node_id`
- `active_form_submission_id`
- `linked_case_id`
- `assigned_operator_user_id`
- `delegated_runtime_connector_id`
- `title`
- `language`
- `source_ref`
  Browser visitor ID, mobile device ID, authenticated app user ID, or other
  channel-scoped source handle.
- `opened_at`
- `last_activity_at`
- `closed_at`
- `metadata_json`

A conversation session is not just a message log. It is a stateful operational
record.

### 2. `ConversationParticipant`

This tells us who is in the conversation.

Suggested fields:

- `id`
- `conversation_session_id`
- `participant_kind`
  - `visitor`
  - `contact`
  - `operator_user`
  - `workspace_agent`
  - `runtime_connector`
  - `system`
- `participant_ref`
  Contact ID, user ID, agent ID, connector ID, or anonymous visitor handle.
- `role_in_session`
  - `requester`
  - `helper`
  - `owner`
  - `observer`
- `display_name`
- `joined_at`
- `left_at`
- `metadata_json`

Rules:

- every session has at least one requester participant
- a session can begin with an anonymous visitor and later resolve to a contact
- internal humans and runtimes are explicit participants, not hidden system effects

### 3. `ConversationMessage`

This is one event inside the session.

Suggested fields:

- `id`
- `conversation_session_id`
- `participant_id`
- `role`
  - `customer`
  - `operator`
  - `agent`
  - `runtime`
  - `system`
- `kind`
  - `text`
  - `status_update`
  - `knowledge_citation`
  - `form_prompt`
  - `form_update`
  - `tool_call`
  - `tool_result`
  - `escalation_event`
- `content_text`
- `content_json`
- `visibility`
  - `customer_visible`
  - `internal_only`
- `created_at`

Rules:

- every customer-visible reply is a message
- every internal escalation or automated action that matters is also a message or outcome event
- citations and tool actions remain inspectable

### 4. `ConversationWorkingState`

This is the live state of the interaction.

Suggested fields:

- `conversation_session_id`
- `suggested_catalog_node_ids`
- `classification_confidence`
- `active_policy_profile_ref`
- `cited_knowledge_resource_ids`
- `collected_fields_json`
- `missing_fields_json`
- `active_form_spec_id`
- `active_form_submission_id`
- `requires_operator_review`
- `last_decision_reason`
- `updated_at`

This is the answer to:

- what do we think this is about?
- what do we already know?
- what is still missing?
- what is allowed next?

### 5. `ConversationOutcome`

This records durable outcomes from the session.

Suggested fields:

- `id`
- `conversation_session_id`
- `kind`
  - `resolved_in_conversation`
  - `form_draft_created`
  - `form_submitted`
  - `case_created`
  - `case_updated`
  - `human_handoff`
- `result_ref_json`
- `created_at`

### 6. `ServiceCatalogNode`

This is the semantic classifier and self-service tree node.

Suggested fields:

- `id`
- `workspace_id`
- `parent_node_id`
- `slug`
- `path_slug`
- `title`
- `description_markdown`
- `node_kind`
  - `domain`
  - `service`
  - `request_type`
  - `issue_type`
  - `offering`
- `visibility`
  - `public`
  - `workspace`
  - `restricted`
- `supported_channels_json`
- `default_case_category`
- `default_collection_id`
- `default_priority`
- `routing_policy_json`
- `entitlement_policy_json`
- `search_keywords_json`
- `display_order`
- `created_at`
- `updated_at`

Rules:

- the tree is human-browsable
- the node is machine-classifiable
- a leaf node should be specific enough to drive forms and routing

### 7. `ServiceCatalogBinding`

This binds catalog nodes to other operational records.

Suggested fields:

- `id`
- `workspace_id`
- `catalog_node_id`
- `target_kind`
  - `conversation_session`
  - `case`
  - `form_spec`
  - `knowledge_resource`
- `target_id`
- `binding_kind`
  - `primary`
  - `secondary`
  - `suggested`
  - `historical`
- `confidence`
- `created_at`

### 8. `AgentRuntimeConnector`

This is the provider-neutral connector to a long-running runtime.

Suggested fields:

- `id`
- `workspace_id`
- `user_id`
- `provider`
  - `openclaw`
  - `perplexity_personal_computer`
  - `generic_runtime_bridge`
- `status`
  - `linked`
  - `degraded`
  - `revoked`
- `capabilities_json`
- `last_seen_at`
- `created_at`
- `updated_at`

Rules:

- OpenClaw is the easiest provider, not the only provider
- the provider contract is generic even when provider aliases exist in the CLI

### 9. `Case`

The case model should gain two stronger references:

- `primary_catalog_node_id`
- `originating_conversation_session_id`

This lets us say:

- what service domain this case belongs to
- whether it came from a conversation session

## What Is In A Chat Session?

A chat session is:

- a collection of messages
- a collection of participants
- a classification state
- a knowledge citation history
- an forms collection state
- a policy state
- an escalation history
- an outcome record

So the shortest correct answer is:

**A conversation session contains messages, but it is not reducible to a
message list.**

## Relationship To Existing Case Concepts

### `Category`

`Category` should stop being the strongest semantic classifier.

Instead:

- `primary_catalog_node_id` is the canonical semantic classification
- `Category` becomes a display/reporting projection if still useful

Example:

- catalog node path: `billing/refunds/request_refund`
- display category: `Billing / Refunds`

### `CollectionID`

`CollectionID` remains important, but it means queue ownership, not semantic
classification.

Example:

- catalog node: `billing/refunds/request_refund`
- collection: `finance-operations`

### `Tags`

Tags remain flexible metadata:

- urgency
- VIP
- outage-related
- renewal-risk

They are not the primary service taxonomy.

## Relationship To Knowledge

Knowledge should attach to service catalog nodes through bindings.

That gives us:

- one service node
- many knowledge resources
- different kinds of knowledge for the same node

Examples:

- policy
- FAQ
- troubleshooting
- playbook
- agent skill

The session stores citations to the knowledge used during the interaction.

## Relationship To Forms

Form specs should also attach to service catalog nodes.

That means:

- the catalog tells us which request types exist
- the form spec tells us what must be collected for that request type

The conversation working state keeps track of:

- which form spec is active
- what has been collected
- what is still missing

## Relationship To Contacts And Users

### Contact

A conversation usually belongs to one primary external requester.

The session can:

- start with an anonymous visitor
- resolve to a contact later
- attach to an existing contact immediately if the channel is authenticated

### User

Users are internal operators.

They relate to the session through:

- assignment
- takeover
- supervision
- approval

## Suggested UX Model

### Public Widget Or Mobile Chat

The customer sees:

- a simple chat surface
- helpful service suggestions
- guided forms when needed
- clear escalation to human support when required

The customer does not see:

- raw case internals
- raw runtime connector internals
- direct access to a local runtime

### Operator View

The operator sees one timeline with:

- session transcript
- participants
- current service classification
- knowledge used
- forms progress
- escalations and outcomes
- linked case if created

### Agent View

An agent host sees one structured object graph:

- current session
- suggested and primary service nodes
- active knowledge resources
- active form spec and missing fields
- whether delegation to a runtime is allowed
- whether escalation to a case is required

## API Shape

GraphQL should expose:

- `conversationSessions`
- `conversationSession(id)`
- `conversationParticipants(sessionId)`
- `conversationMessages(sessionId)`
- `serviceCatalogNodes(workspaceId, parentId, channel)`
- `classifyConversationSession(id)`
- `escalateConversationToCase(id)`

CLI should expose:

- `mbr conversations list`
- `mbr conversations show`
- `mbr conversations messages`
- `mbr conversations classify`
- `mbr conversations escalate`
- `mbr service-catalog list`
- `mbr service-catalog tree`
- `mbr service-catalog show`

## Authoring Model

For knowledge and catalog authoring, the best shape is:

- Markdown + frontmatter for authored source
- PostgreSQL for runtime indexing, bindings, visibility, and audit

That keeps the system:

- easy to author
- easy for agents to read
- easy for Move Big Rocks to reason over

## Example

```text
Customer opens chat widget
    ↓
Move Big Rocks creates ConversationSession
    ↓
Move Big Rocks suggests ServiceCatalogNode = billing/refunds/request_refund
    ↓
Move Big Rocks retrieves refund policy + refund form spec
    ↓
Conversation gathers missing fields
    ↓
OpenClaw helps draft response behind Move Big Rocks
    ↓
Move Big Rocks submits forms
    ↓
Move Big Rocks creates case in finance-operations collection
    ↓
Case links back to originating ConversationSession
```

## Acceptance Criteria

- the object graph clearly separates conversation, classification, knowledge, forms, and case work
- cases and conversations both bind to service catalog nodes
- collections remain routing objects rather than semantic classification objects
- agent runtimes fit through a provider-neutral connector contract
- OpenClaw remains first-class without becoming the only supported runtime
