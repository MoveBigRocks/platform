# Workflow Proof Matrix

This document defines the minimum workflow evidence required before a
user-visible capability can be marked `Proven` in milestone or readiness docs.

The implementation and proof sequence for closing the current gaps lives in
[`docs/WORKFLOW_PROOF_CLOSURE_PLAN.md`](./WORKFLOW_PROOF_CLOSURE_PLAN.md).

## Rules

- A workflow is not `Proven` if tests only show that a record was stored or a
  command event was published.
- A workflow is not `Proven` if a scenario runner simulates the final state
  directly.
- A workflow becomes `Proven` only when the real production path completes
  through entrypoint, parsing, persistence, worker/consumer, and durable side
  effect.

## Current Matrix

| Workflow | Status Today | Production Path That Must Be Proven | Minimum Automated Assertions | Current Gap |
| --- | --- | --- | --- | --- |
| Inbound new email creates a case | `Proven` | Postmark webhook -> inbound email record -> email event worker -> case creation -> case communication persistence | inbound email stored pending, worker processes it, case created, communication created, case state/message count correct | Covered by [`internal/service/handlers/postmark_webhooks_integration_test.go`](../internal/service/handlers/postmark_webhooks_integration_test.go) and archived as `workflow-proof/inbound-new-email-case-create.json` |
| Agent or human reply from a case sends email | `Proven` | admin/case reply -> case communication -> outbound email persistence -> `email-commands` consumer -> provider send -> provider message ID/status persistence | reply accepted, outbound record created, consumer runs, provider message ID persisted, later delivery/bounce can update same record | Covered by [`internal/service/handlers/email_command_handler_test.go`](../internal/service/handlers/email_command_handler_test.go) and archived as `workflow-proof/case-reply-send.json` |
| Customer reply updates the correct existing case | `Proven` | Postmark webhook with `In-Reply-To`/`References` -> parser -> inbound worker -> case match -> reopen/open state transition | parser populates headers, worker matches prior case, no duplicate case created, case status transition is correct | Covered by [`internal/service/handlers/postmark_webhooks_integration_test.go`](../internal/service/handlers/postmark_webhooks_integration_test.go) and archived as `workflow-proof/inbound-reply-threading.json` |
| Public form submission creates case and sends notification | `Proven` | public form submit -> submission persistence -> form event handler -> case creation -> `email-commands` consumer -> durable notification side effect | submission completed, case linked, notification command consumed, outbound send/persistence visible | Covered by [`internal/service/handlers/form_public_handler_test.go`](../internal/service/handlers/form_public_handler_test.go) and archived as `workflow-proof/public-form-case-notification.json` |
| Rule `send_email` action delivers a real side effect | `Proven` | case/form trigger -> rules engine -> notification action handler -> `email-commands` consumer -> durable outbound side effect | rule fires, command emitted, consumer runs, durable outbound state recorded | Covered by [`internal/automation/services/action_handlers_notification_integration_test.go`](../internal/automation/services/action_handlers_notification_integration_test.go) and archived as `workflow-proof/rule-send-email.json` |
| Knowledge review notification reaches a durable notification side effect | `Proven` | knowledge service action -> `notification-commands` consumer -> durable delivery/persistence | notification command emitted, consumer runs, durable side effect recorded | Covered by [`internal/knowledge/services/knowledge_service_test.go`](../internal/knowledge/services/knowledge_service_test.go) and archived as `workflow-proof/knowledge-review-notification.json` |
| Manual operator case creation and initial queue placement | `Partially evidenced` | supported admin/API/CLI create path -> case persistence -> queue visibility -> audit-visible operator state | case created through a supported product surface, queue placement visible, initial state queryable, actor/audit context preserved | Service and API creation paths exist, but the operator-complete loop is not yet represented as a milestone-proof row and the supported CLI surface is incomplete |
| Case ownership and work management: assign, unassign, set priority, add internal note | `Open` | supported admin/API/CLI mutation path -> durable case/work-thread state -> queue visibility remains coherent | each action succeeds through a supported product surface, resulting state is queryable, and operator context remains visible in the durable thread | Service methods exist, but the supported CLI surface and workflow-proof evidence are incomplete |
| Conversation reply, handoff, and escalation remain first-class operator workflows | `Open` | supported product surface -> conversation mutation -> queue parity -> escalation preserves provenance into case | reply persists in conversation, handoff updates routing, escalation creates/links case and preserves provenance | Conversation services and CLI commands exist, but there are no milestone-proof rows or archived workflow artifacts for these loops |
| Manual case attachment upload is durable and linked to operational work | `Partially evidenced` | supported upload entrypoint -> scan/store -> attachment metadata persistence -> case thread/query surface | upload accepted, durable metadata stored, attachment linked to case, resulting attachment visible from the case record | Handler and service tests exist, but no milestone-proof row or archived workflow artifact exists |
| Inbound email attachments survive the real webhook path | `Partially evidenced` | Postmark webhook -> attachment decode -> scan/store -> inbound email/case linkage | attachment IDs persisted, failed attachments remain auditable, resulting case/email record exposes attachment linkage | Attachment ingest code exists, but the current webhook proof rows do not assert attachment outcomes |
| Public conversation intake reaches a supervised conversation record owned by core | `Open` | public widget or equivalent public conversation surface -> conversation session -> queue item -> operator follow-up path | public intake creates a core-owned conversation record, queue visibility exists, operators can continue from the same record | Public promise exists in site/docs, but the repo does not yet prove a supported public conversation intake surface |
| Extension or agent initiated case action executes through a sanctioned core-action contract | `Open` | extension/agent producer -> supported case-action contract -> production consumer -> durable case result | command consumed by a production worker, resulting case action recorded durably, follow-up event or response visible | `case-commands` still has no production consumer or workflow proof |

## Stream Parity Checklist

Every production command stream used by any workflow above must have:

| Stream | Producer Exists | Worker Manager Wiring | Container Startup Wiring | Failure Proof Exists | Workflow Proof Exists | Status |
| --- | --- | --- | --- | --- | --- | --- |
| `email-commands` | Yes | Yes | Yes | Yes | Yes | `Proven` |
| `notification-commands` | Yes | Yes | Yes | Yes | Yes | `Proven` |
| `case-commands` | Partial | No | No | No | No | `Open` |

The stream-wiring and failure-proof evidence currently lives in:

- [`internal/workers/manager_test.go`](../internal/workers/manager_test.go)
- [`internal/infrastructure/container/container_integration_test.go`](../internal/infrastructure/container/container_integration_test.go)
- [`internal/service/handlers/email_command_handler_test.go`](../internal/service/handlers/email_command_handler_test.go) archived as `workflow-proof/email-command-failure-visible.json`
- [`internal/service/handlers/notification_command_handler_test.go`](../internal/service/handlers/notification_command_handler_test.go) archived as `workflow-proof/notification-command-failure-visible.json`

## Current State

- The originally scoped command-driven operational workflows now have end-to-end automated proof.
- The milestone proof bundle archives machine-readable workflow artifacts for
  those flows, including failure-visible command artifacts.
- The full `go test -tags=integration ./...` sweep is green and hard-gated in
  CI.
- Worker-manager registration and full container startup wiring are covered by
  automated tests for the scoped command streams.
- Milestone 1 has now been expanded to a product-complete bar, so the open rows
  above are real closure blockers rather than optional follow-on work.

## Update Rule

When a scoped workflow changes:

- update this matrix
- update the relevant tests
- update milestone/readiness docs if the proof status changed
