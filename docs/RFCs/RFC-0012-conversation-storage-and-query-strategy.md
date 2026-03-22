# RFC-0012: Conversation Storage and Query Strategy

## Status

draft

## Summary

Move Big Rocks should store conversation sessions and their operational graph in
PostgreSQL.

That includes:

- conversation sessions
- participants
- messages
- working state
- outcomes
- service catalog nodes
- service catalog bindings

Attachments, uploaded files, screenshots, audio, and oversized raw provider
artifacts should remain outside PostgreSQL in object storage, with structured
references kept in PostgreSQL.

## Decision

### 1. Store the Hot Operational Graph in PostgreSQL

Use PostgreSQL for:

- conversation sessions
- service catalog nodes
- conversation participants
- conversation messages
- conversation working state
- conversation outcomes
- catalog bindings
- runtime connector records

Why:

- Move Big Rocks needs transactional linkage between conversations, contacts, forms, cases, and service catalog classification
- operators need filtered inboxes, timelines, and audit queries
- agents need structured retrieval over live state
- policy decisions and escalations should commit atomically with the records they affect

### 2. Do Not Store Hot Conversation State as Files

Do not make the conversation transcript a Markdown file tree or append-only log
file as the primary runtime store.

That would make it harder to:

- paginate and filter live sessions
- join to contacts, cases, and forms
- support policy and entitlement checks
- power operator inboxes
- apply retention and audit rules cleanly

Markdown remains a good authoring format for knowledge and parts of the service
catalog. It is not the primary runtime store for live conversations.

### 3. Store Large Blobs Outside PostgreSQL

Use object storage for:

- file attachments
- screenshots
- audio clips
- uploaded documents
- oversized raw runtime traces or provider payloads

The PostgreSQL row stores:

- metadata
- structured summary
- object storage reference
- checksum
- size
- content type

## Schema Placement

Recommended schema ownership:

- `core_service.conversation_sessions`
- `core_service.conversation_participants`
- `core_service.conversation_messages`
- `core_service.conversation_working_state`
- `core_service.conversation_outcomes`
- `core_service.service_catalog_nodes`
- `core_service.service_catalog_bindings`
- `core_platform.agent_runtime_connectors`

Knowledge and forms remain in their own existing bounded contexts:

- `core_knowledge.knowledge_resources`
- `core_service.form_specs`
- `core_service.form_submissions`

## Table Design

### `core_service.conversation_sessions`

```text
id UUID PK DEFAULT uuidv7()
workspace_id UUID NOT NULL
channel TEXT NOT NULL
status TEXT NOT NULL
primary_contact_id UUID NULL
primary_catalog_node_id UUID NULL
active_form_spec_id UUID NULL
active_form_submission_id UUID NULL
linked_case_id UUID NULL
assigned_operator_user_id UUID NULL
delegated_runtime_connector_id UUID NULL
title TEXT NULL
language_code TEXT NULL
source_ref TEXT NULL
external_session_key TEXT NULL
opened_at TIMESTAMPTZ NOT NULL
last_activity_at TIMESTAMPTZ NOT NULL
closed_at TIMESTAMPTZ NULL
metadata_json JSONB NOT NULL DEFAULT '{}'
created_at TIMESTAMPTZ NOT NULL
updated_at TIMESTAMPTZ NOT NULL
```

### `core_service.conversation_participants`

```text
id UUID PK DEFAULT uuidv7()
workspace_id UUID NOT NULL
conversation_session_id UUID NOT NULL
participant_kind TEXT NOT NULL
participant_ref TEXT NOT NULL
role_in_session TEXT NOT NULL
display_name TEXT NULL
joined_at TIMESTAMPTZ NOT NULL
left_at TIMESTAMPTZ NULL
metadata_json JSONB NOT NULL DEFAULT '{}'
created_at TIMESTAMPTZ NOT NULL
```

### `core_service.conversation_messages`

```text
id UUID PK DEFAULT uuidv7()
workspace_id UUID NOT NULL
conversation_session_id UUID NOT NULL
participant_id UUID NULL
role TEXT NOT NULL
kind TEXT NOT NULL
visibility TEXT NOT NULL
content_text TEXT NULL
content_json JSONB NOT NULL DEFAULT '{}'
created_at TIMESTAMPTZ NOT NULL
```

Important recommendation:

- keep `content_text` and `content_json` bounded
- do not treat this row as a blob dump for arbitrary provider traces

### `core_service.conversation_working_state`

```text
conversation_session_id UUID PK
workspace_id UUID NOT NULL
primary_catalog_node_id UUID NULL
suggested_catalog_nodes_json JSONB NOT NULL DEFAULT '[]'
classification_confidence NUMERIC NULL
active_policy_profile_ref TEXT NULL
active_form_spec_id UUID NULL
active_form_submission_id UUID NULL
collected_fields_json JSONB NOT NULL DEFAULT '{}'
missing_fields_json JSONB NOT NULL DEFAULT '{}'
requires_operator_review BOOLEAN NOT NULL DEFAULT FALSE
updated_at TIMESTAMPTZ NOT NULL
```

This is a mutable state table, not an event log.

### `core_service.conversation_outcomes`

```text
id UUID PK DEFAULT uuidv7()
workspace_id UUID NOT NULL
conversation_session_id UUID NOT NULL
kind TEXT NOT NULL
result_ref_json JSONB NOT NULL DEFAULT '{}'
created_at TIMESTAMPTZ NOT NULL
```

### `core_service.service_catalog_nodes`

```text
id UUID PK DEFAULT uuidv7()
workspace_id UUID NOT NULL
parent_node_id UUID NULL
slug TEXT NOT NULL
path_slug TEXT NOT NULL
title TEXT NOT NULL
description_markdown TEXT NOT NULL DEFAULT ''
node_kind TEXT NOT NULL
status TEXT NOT NULL
visibility TEXT NOT NULL
supported_channels TEXT[] NOT NULL DEFAULT '{}'
default_case_category TEXT NULL
default_collection_id UUID NULL
default_priority TEXT NULL
routing_policy_json JSONB NOT NULL DEFAULT '{}'
entitlement_policy_json JSONB NOT NULL DEFAULT '{}'
search_keywords TEXT[] NOT NULL DEFAULT '{}'
display_order INTEGER NOT NULL DEFAULT 0
created_at TIMESTAMPTZ NOT NULL
updated_at TIMESTAMPTZ NOT NULL
```

### `core_service.service_catalog_bindings`

```text
id UUID PK DEFAULT uuidv7()
workspace_id UUID NOT NULL
catalog_node_id UUID NOT NULL
target_kind TEXT NOT NULL
target_id UUID NOT NULL
binding_kind TEXT NOT NULL
confidence NUMERIC NULL
created_at TIMESTAMPTZ NOT NULL
```

## Use Typed Columns First, JSONB Second

Use typed columns for:

- workspace ID
- contact ID
- case ID
- catalog node ID
- status
- channel
- visibility
- timestamps

Use `JSONB` only for:

- low-selectivity metadata
- flexible provider payload envelopes
- working-state details that are not part of the high-cardinality filter path

Do not hide core query fields inside `JSONB`.

## Recommended Indexes

### `conversation_sessions`

Primary patterns:

- operator inbox by workspace + status
- recent sessions by workspace
- sessions by contact
- sessions by catalog node
- linked-case lookup
- active widget resume by external session key

Recommended indexes:

```sql
CREATE INDEX idx_conversation_sessions_workspace_status_activity
  ON core_service.conversation_sessions (workspace_id, status, last_activity_at DESC, id DESC);

CREATE INDEX idx_conversation_sessions_workspace_catalog_status_activity
  ON core_service.conversation_sessions (workspace_id, primary_catalog_node_id, status, last_activity_at DESC, id DESC);

CREATE INDEX idx_conversation_sessions_workspace_contact_activity
  ON core_service.conversation_sessions (workspace_id, primary_contact_id, last_activity_at DESC, id DESC);

CREATE INDEX idx_conversation_sessions_linked_case
  ON core_service.conversation_sessions (linked_case_id)
  WHERE linked_case_id IS NOT NULL;

CREATE UNIQUE INDEX uq_conversation_sessions_external_session
  ON core_service.conversation_sessions (workspace_id, channel, external_session_key)
  WHERE external_session_key IS NOT NULL AND closed_at IS NULL;
```

### `conversation_participants`

Primary patterns:

- list participants for a session
- find sessions involving a given participant

Recommended indexes:

```sql
CREATE INDEX idx_conversation_participants_session
  ON core_service.conversation_participants (conversation_session_id, joined_at, id);

CREATE INDEX idx_conversation_participants_lookup
  ON core_service.conversation_participants (workspace_id, participant_kind, participant_ref);
```

### `conversation_messages`

Primary patterns:

- fetch transcript for one session
- fetch customer-visible transcript for widget resume
- search messages across a workspace
- search messages inside one session

Recommended indexes:

```sql
CREATE INDEX idx_conversation_messages_session_created
  ON core_service.conversation_messages (conversation_session_id, created_at, id);

CREATE INDEX idx_conversation_messages_session_visibility_created
  ON core_service.conversation_messages (conversation_session_id, visibility, created_at, id);

CREATE INDEX idx_conversation_messages_workspace_created
  ON core_service.conversation_messages (workspace_id, created_at DESC, id DESC);
```

For full-text search:

```sql
ALTER TABLE core_service.conversation_messages
  ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('simple', coalesce(content_text, ''))
  ) STORED;

CREATE INDEX idx_conversation_messages_search
  ON core_service.conversation_messages
  USING GIN (search_vector);
```

Optional fuzzy and substring search:

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_conversation_messages_content_trgm
  ON core_service.conversation_messages
  USING GIN (content_text gin_trgm_ops);
```

Use the trigram index only if operator search actually needs substring or
fuzzy-match behavior. Do not create it by default if full-text search is
enough.

### `conversation_working_state`

Primary patterns:

- load the current working state for one session
- filter sessions requiring operator review

Recommended indexes:

```sql
CREATE INDEX idx_conversation_working_state_review
  ON core_service.conversation_working_state (workspace_id, requires_operator_review, updated_at DESC);
```

### `service_catalog_nodes`

Primary patterns:

- render tree by parent
- resolve node by slug/path
- search by title and description
- filter by channel and visibility

Recommended indexes:

```sql
CREATE UNIQUE INDEX uq_service_catalog_nodes_workspace_path
  ON core_service.service_catalog_nodes (workspace_id, path_slug);

CREATE UNIQUE INDEX uq_service_catalog_nodes_workspace_parent_slug
  ON core_service.service_catalog_nodes (workspace_id, parent_node_id, slug);

CREATE INDEX idx_service_catalog_nodes_tree
  ON core_service.service_catalog_nodes (workspace_id, parent_node_id, display_order, id);

CREATE INDEX idx_service_catalog_nodes_visibility
  ON core_service.service_catalog_nodes (workspace_id, visibility, status, display_order, id);

CREATE INDEX idx_service_catalog_nodes_channels
  ON core_service.service_catalog_nodes
  USING GIN (supported_channels);

ALTER TABLE core_service.service_catalog_nodes
  ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(description_markdown, '')), 'B')
  ) STORED;

CREATE INDEX idx_service_catalog_nodes_search
  ON core_service.service_catalog_nodes
  USING GIN (search_vector);
```

### `service_catalog_bindings`

Primary patterns:

- resolve all bindings for a target
- resolve all targets under a catalog node

Recommended indexes:

```sql
CREATE INDEX idx_service_catalog_bindings_target
  ON core_service.service_catalog_bindings (workspace_id, target_kind, target_id, binding_kind);

CREATE INDEX idx_service_catalog_bindings_catalog
  ON core_service.service_catalog_bindings (workspace_id, catalog_node_id, binding_kind, target_kind);
```

## JSONB Index Guidance

If `JSONB` fields become hot query paths:

- use targeted expression indexes first
- use a `GIN` index only when the query pattern justifies it
- prefer `jsonb_path_ops` for containment-heavy query paths
- prefer default `jsonb_ops` when key-exists operators are needed

Do not add broad `GIN` indexes to every JSONB column “just in case.”

## Query Model

### Operator Inbox

Common query:

- active sessions in workspace
- ordered by most recent activity
- filtered by assigned operator, status, catalog node, or contact

This is powered mainly by `conversation_sessions`, not by scanning messages.

### Transcript View

Common query:

- fetch transcript for one session
- paginate forward or backward by `created_at, id`

Use keyset pagination, not `OFFSET`.

### Customer Widget Resume

Common query:

- fetch customer-visible messages for one open session

Use:

- `conversation_session_id`
- `visibility = 'customer_visible'`
- cursor on `created_at, id`

### Search

There are two search modes:

- **structured search** over sessions, contacts, catalog nodes, and cases
- **text search** over conversation message content

Do not make text search the first or only inbox mechanism.

## Size And Retention Guidance

PostgreSQL is the right default home for conversations unless and until usage
shows otherwise.

Guidance:

- store normal message text inline
- keep message bodies bounded
- keep blobs out of row storage
- use append-only message inserts
- use retention policies for abandoned or closed sessions

If message volume becomes very large:

- add declarative partitioning on `conversation_messages(created_at)`
- partition monthly or quarterly
- keep the parent table contract unchanged for the application

Do not start with partitioning unless the scale justifies it.

## Why PostgreSQL Is The Right Default

PostgreSQL is already Move Big Rocks's system of record.

Conversations belong there because they are:

- relational
- audit-sensitive
- operationally joined to cases, contacts, forms, and catalog nodes
- not just opaque logs

Using PostgreSQL now keeps the product simple, truthful, and easy for agents
and humans to reason about.

## Implementation Notes

- use `UUID` primary keys with `uuidv7()`
- store `workspace_id` on all high-volume rows for direct filtering and RLS
- prefer typed enums or constrained text values over free-form strings
- use generated `tsvector` columns for full-text search
- use object storage for any payload that is large, binary, or not queried in-line
