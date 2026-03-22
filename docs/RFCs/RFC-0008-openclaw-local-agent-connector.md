# RFC-0008: OpenClaw Local Agent Connector

## Status

draft

## Summary

Move Big Rocks supports optional integration with user-controlled agent runtimes so an
always-on agent can help configure, operate, extend, and work inside Move Big Rocks
without becoming a mandatory runtime dependency.

This RFC sits inside a broader Move Big Rocks rule:

- any capable agent host can operate Move Big Rocks through CLI, GraphQL, and Move Big Rocks-managed context
- OpenClaw is the first connector, not the only agent story

The connector is a user-scoped local agent bridge:

- OpenClaw runs on the user's machine
- a local `mbr` helper links that machine to Move Big Rocks
- Move Big Rocks sends structured tasks to the linked local OpenClaw runtime
- results, drafts, and actions are returned to Move Big Rocks with audit and policy enforcement

This makes OpenClaw a practical controller for Move Big Rocks while preserving these
product rules:

- Move Big Rocks remains fully usable without OpenClaw
- Move Big Rocks remains the source of truth for cases, knowledge, forms, extensions, approvals, and audit
- local or external agent runtimes are optional connectors, not core assumptions
- Claude Code, Codex, and other agent hosts continue to operate through the standard Move Big Rocks machine contract

## Connector Model

### Overview

The Move Big Rocks and OpenClaw shape is:

1. User runs OpenClaw locally.
2. User authenticates to Move Big Rocks with the CLI.
3. User runs `mbr openclaw link`.
4. The CLI stores local OpenClaw connection settings securely on that machine.
5. A local bridge process opens an outbound authenticated channel to Move Big Rocks.
6. Move Big Rocks dispatches structured tasks to that user's linked OpenClaw.
7. OpenClaw reads and acts through Move Big Rocks's sanctioned commands and APIs.

This keeps OpenClaw local and user-controlled while making Move Big Rocks the system of
record for:

- work definition
- permissions
- approvals
- audit
- results and follow-up actions

### Provider Model

Move Big Rocks should treat this as a provider family, not a one-provider feature.

Examples of provider types:

- `openclaw`
- `perplexity-personal-computer`
- additional local or long-running agent hosts that adopt a stable connector contract

The provider contract should normalize:

- identity and linking
- task dispatch
- transcript capture
- proposed action envelopes
- capability advertisement
- connector health

OpenClaw should be the first and easiest out-of-the-box provider, but the CLI
and domain model should make the multi-provider shape obvious.

### Why User-Scoped

OpenClaw local setups are tied to a specific operator workstation and trust
boundary.

The connector is therefore:

- linked to a Move Big Rocks user
- visible in that user's workspace context
- not treated as a shared workspace daemon

## Core Product Rules

Core Move Big Rocks owns:

- linked local-agent identity
- connector registration and revocation
- task creation and assignment
- permission and workspace scoping
- approval checkpoints
- audit logs and transcripts
- case, knowledge, forms, and extension APIs

The OpenClaw connector owns:

- local gateway discovery and configuration
- request translation into OpenClaw-compatible API calls
- streaming or polling result handling
- bridge reconnect behavior
- local runtime health reporting

## Domain Model

### Entity: `LocalAgentConnector`

- `id: UUID`
- `user_id: UUID`
- `workspace_id: UUID?`
- `provider: enum`
  - `openclaw`
  - `perplexity-personal-computer`
- `status: enum`
  - `pending`
  - `linked`
  - `degraded`
  - `revoked`
- `display_name: string`
- `capabilities: jsonb`
- `last_seen_at: timestamptz?`
- `bridge_version: string?`
- `local_metadata: jsonb`
- `created_at: timestamptz`
- `updated_at: timestamptz`

Business rules:

- a connector belongs to one user
- a connector acts only within that user's authorized Move Big Rocks access
- a connector is optional and revocable

### Entity: `LocalAgentTask`

- `id: UUID`
- `connector_id: UUID`
- `workspace_id: UUID`
- `case_id: UUID?`
- `kind: enum`
  - `case-assist`
  - `knowledge-query`
  - `form-draft`
  - `extension-authoring`
  - `deployment-help`
  - `operator-task`
- `input_payload: jsonb`
- `status: enum`
  - `queued`
  - `running`
  - `completed`
  - `failed`
  - `cancelled`
- `result_payload: jsonb`
- `proposed_actions: jsonb`
- `requires_approval: boolean`
- `started_at: timestamptz?`
- `completed_at: timestamptz?`
- `created_at: timestamptz`

Business rules:

- tasks are explicit Move Big Rocks records, not hidden side-channel agent activity
- results may propose actions, but execution goes through Move Big Rocks policy
- local agents draft, explain, and propose; Move Big Rocks decides what is allowed

### Entity: `LocalAgentTranscriptEntry`

- `id: UUID`
- `task_id: UUID`
- `role: enum`
  - `system`
  - `mbr`
  - `openclaw`
  - `user`
- `content: text`
- `metadata: jsonb`
- `created_at: timestamptz`

Business rules:

- meaningful exchanges are auditable
- sensitive local secrets are not mirrored back into Move Big Rocks transcripts
- structured result metadata is stored separately from raw text

## OpenClaw Transport Model

The first implementation uses OpenClaw's local authenticated API rather than
requiring a public plugin endpoint.

Interaction model:

- local bridge calls OpenClaw Gateway on `127.0.0.1`
- local bridge uses the user's local OpenClaw token
- Move Big Rocks never calls a public OpenClaw endpoint directly
- the bridge maintains an outbound authenticated session to Move Big Rocks

This keeps OpenClaw local and matches the security posture of local agent
software.

## Move Big Rocks CLI UX

Commands:

```text
mbr agents runtimes list [--json]
mbr agents runtimes providers [--json]
mbr agents runtime link --provider PROVIDER --workspace WORKSPACE_ID [provider-specific flags]
mbr agents runtime status [--json]
mbr agents runtime unlink CONNECTOR_ID
mbr agents runtime bridge run --provider PROVIDER
mbr agents runtime ask --provider PROVIDER --workspace WORKSPACE_ID --task-file PATH
```

Desired operator experience:

1. `mbr auth login`
2. `mbr agents runtime link --provider openclaw`
3. `mbr agents runtime bridge run --provider openclaw`
4. work in Move Big Rocks with optional "Send to local runtime" actions

OpenClaw can still have a friendly shortcut layer such as:

```text
mbr openclaw link ...
mbr openclaw bridge run
```

but those should be aliases over the provider-generic command model rather than
the only product shape.

## Move Big Rocks UI and Workflow Integration

OpenClaw-linked workflows feel natural in these places:

- case page:
  - ask OpenClaw to summarize
  - ask OpenClaw to retrieve relevant knowledge
  - ask OpenClaw to prepare an forms draft
  - ask OpenClaw to draft a reply
- knowledge and forms:
  - ask OpenClaw what applies
  - ask OpenClaw what is missing
- setup and deployment:
  - ask OpenClaw to guide instance setup
  - ask OpenClaw to propose extension config
- extension authoring:
  - ask OpenClaw to scaffold and test an extension against Move Big Rocks's SDK and CLI

In every case, Move Big Rocks remains the place where:

- records are stored
- actions are approved or rejected
- side effects are executed
- history is audited

## API Contract

GraphQL additions:

```graphql
type LocalAgentConnector {
  id: ID!
  provider: String!
  status: String!
  displayName: String!
  capabilities: JSON!
  lastSeenAt: Time
  bridgeVersion: String
}

type LocalAgentTask {
  id: ID!
  connectorId: ID!
  workspaceId: ID!
  caseId: ID
  kind: String!
  status: String!
  inputPayload: JSON!
  resultPayload: JSON
  proposedActions: JSON!
  requiresApproval: Boolean!
}

extend type Query {
  localAgentConnectors(workspaceId: ID!): [LocalAgentConnector!]!
  localAgentTask(id: ID!): LocalAgentTask
}

extend type Mutation {
  linkOpenClawConnector(input: LinkOpenClawConnectorInput!): LocalAgentConnector!
  unlinkLocalAgentConnector(id: ID!): Boolean!
  createLocalAgentTask(input: CreateLocalAgentTaskInput!): LocalAgentTask!
  approveLocalAgentTaskAction(id: ID!, actionKey: String!): LocalAgentTask!
}
```

Bridge endpoints:

- `POST /api/local-agents/connectors/:id/heartbeat`
- `POST /api/local-agents/connectors/:id/tasks/claim`
- `POST /api/local-agents/tasks/:id/result`
- `POST /api/local-agents/tasks/:id/transcript`

These are internal Move Big Rocks endpoints for the local bridge, authenticated with a
short-lived connector credential issued at link time.

## Security Model

OpenClaw integration is a privileged connector path.

Rules:

- OpenClaw link is explicit and per-user
- the local bridge uses outbound connections only
- Move Big Rocks never stores the user's OpenClaw admin token in plaintext
- OpenClaw cannot bypass Move Big Rocks's write and approval paths
- local connector capabilities are least-privilege and revocable
- extension creation and deployment remain Move Big Rocks-side reviewed, installed, and audited

## Data Flow

### Example: Case Assistance

```text
Operator opens a case in Move Big Rocks
    ↓
Operator chooses "Ask OpenClaw"
    ↓
Move Big Rocks creates LocalAgentTask
    ↓
Linked local bridge claims task
    ↓
Bridge calls local OpenClaw
    ↓
OpenClaw uses Move Big Rocks knowledge, forms, and case context
    ↓
Bridge returns result and proposed actions
    ↓
Move Big Rocks records transcript, result, and approval requirements
```

### Example: Extension Authoring

```text
Operator asks OpenClaw to create an extension
    ↓
Move Big Rocks creates LocalAgentTask(kind = extension-authoring)
    ↓
OpenClaw uses Move Big Rocks CLI contract + SDK docs
    ↓
OpenClaw drafts extension assets locally
    ↓
Operator reviews and packages
    ↓
Move Big Rocks installs, validates, activates, and audits through normal extension lifecycle
```

## Relationship to Other Move Big Rocks Primitives

OpenClaw feels native because it works through existing Move Big Rocks concepts:

- **Cases** are the work item
- **Knowledge Resources** provide context
- **Form Specs** define what data is required
- **Extensions** remain the unit of product customization
- **CLI + GraphQL** remain the machine contract

OpenClaw is therefore not a second product model. It is a controller over the
existing Move Big Rocks model.

## ADR Compliance

| ADR | Title | Compliance |
|-----|-------|------------|
| 0005 | Event-Driven Architecture | Local-agent task lifecycle can use outbox and event patterns without hidden side effects. |
| 0010 | Agent API and GraphQL Architecture | GraphQL and CLI remain the canonical contract. |
| 0015 | Workspace-Scoped Agent Access | Connector actions stay within user and workspace access boundaries. |
| 0016 | CLI and Agent Authentication Guidelines | Link, bridge, and task flows use the same CLI-led auth posture. |
| 0021 | PostgreSQL-Only Runtime and Extension-Owned Schemas | Connector records live in core schemas; extension state remains extension-owned. |

## Alternatives Considered

### Alternative 1: Require OpenClaw users to script against Move Big Rocks manually

**Pros:** no Move Big Rocks product work needed  
**Cons:** weak UX, inconsistent security, no auditable task model  
**Why rejected:** the connector should feel designed, not improvised

### Alternative 2: Expose the user's OpenClaw gateway publicly and let Move Big Rocks call it

**Pros:** simpler server-side model  
**Cons:** worse security, worse NAT and firewall UX, breaks the local-trust model  
**Why rejected:** local agent gateways stay local

### Alternative 3: Make OpenClaw a mandatory part of Move Big Rocks

**Pros:** tighter story for one audience  
**Cons:** violates Move Big Rocks's optional-integrations model and makes the base product less universal  
**Why rejected:** Move Big Rocks remains useful with or without OpenClaw

## Acceptance Criteria

- a user can connect Move Big Rocks to a local OpenClaw setup in a small number of steps
- Move Big Rocks remains fully functional when no OpenClaw connector is linked
- OpenClaw can assist with cases, knowledge, forms, setup, and extension authoring through Move Big Rocks's sanctioned model
- all meaningful actions remain controlled and tracked inside Move Big Rocks
