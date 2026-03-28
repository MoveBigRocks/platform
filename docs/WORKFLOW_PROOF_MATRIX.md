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
| Manual operator case creation and initial queue placement | `Proven` | supported admin/API/CLI create path -> case persistence -> queue visibility -> audit-visible operator state | case created through a supported product surface, queue placement visible, initial state queryable, actor/audit context preserved | Covered by [`internal/service/resolvers/case_operator_workflow_integration_test.go`](../internal/service/resolvers/case_operator_workflow_integration_test.go) and archived as `workflow-proof/case-operator-manual-create.json` |
| Case ownership and work management: assign, unassign, set priority, add internal note | `Proven` | supported admin/API/CLI mutation path -> durable case/work-thread state -> queue visibility remains coherent | each action succeeds through a supported product surface, resulting state is queryable, and operator context remains visible in the durable thread | Covered by [`internal/service/resolvers/case_operator_workflow_integration_test.go`](../internal/service/resolvers/case_operator_workflow_integration_test.go) and archived as `workflow-proof/case-operator-work-management.json` |
| Case handoff and status transitions remain first-class operator workflows | `Proven` | supported admin/API/CLI mutation path -> durable case routing/state change -> queue visibility and audit trail remain coherent | handoff preserves queue/team/assignee provenance, status changes are queryable, and the durable thread shows the operator-visible routing/result state | Covered by [`internal/service/resolvers/case_operator_workflow_integration_test.go`](../internal/service/resolvers/case_operator_workflow_integration_test.go) and archived as `workflow-proof/case-operator-handoff.json` plus `workflow-proof/case-operator-status-transition.json` |
| Conversation reply, handoff, and escalation remain first-class operator workflows | `Proven` | supported product surface -> conversation mutation -> queue parity -> escalation preserves provenance into case | reply persists in conversation, handoff updates routing, escalation creates/links case and preserves provenance | Covered by [`internal/service/resolvers/conversation_operator_workflow_integration_test.go`](../internal/service/resolvers/conversation_operator_workflow_integration_test.go) and archived as `workflow-proof/conversation-operator-reply.json`, `workflow-proof/conversation-operator-handoff.json`, and `workflow-proof/conversation-operator-escalation.json` |
| Manual case attachment upload is durable and linked to operational work | `Proven` | supported upload entrypoint -> scan/store -> attachment metadata persistence -> case query surface | upload accepted, durable metadata stored, attachment linked to case, resulting attachment visible from the supported case record | Covered by [`internal/service/handlers/attachment_upload_handler_integration_test.go`](../internal/service/handlers/attachment_upload_handler_integration_test.go) and archived as `workflow-proof/case-operator-attachment-upload.json` |
| Inbound email attachments survive the real webhook path | `Proven` | Postmark webhook -> attachment decode -> scan/store -> inbound email/case linkage | attachment IDs persisted, failed attachments remain auditable, resulting case/email record exposes attachment linkage | Covered by [`internal/service/handlers/postmark_webhooks_integration_test.go`](../internal/service/handlers/postmark_webhooks_integration_test.go) and archived as `workflow-proof/inbound-email-attachments.json` |
| ATS application intake preserves resume attachment and portfolio context | `Proven` | ATS application surface -> shared attachment lookup -> candidate case creation -> attachment linkage -> case query surface | uploaded resume attachment exists before submission, intake links it to the created candidate case, resume and portfolio context survive in case custom fields, and the linked attachment is visible on the candidate case | Covered by [`MoveBigRocks/extensions/ats/runtime/service_test.go`](https://github.com/MoveBigRocks/extensions/blob/37f1c80b5eb6f14701c00344d38d9bf2dcf607db/ats/runtime/service_test.go) and [`MoveBigRocks/extensions/tools/ats-scenario-proof/main.go`](https://github.com/MoveBigRocks/extensions/blob/37f1c80b5eb6f14701c00344d38d9bf2dcf607db/tools/ats-scenario-proof/main.go), archived as `ats-scenario/ats-scenario.json` |
| Public conversation intake reaches a supervised conversation record owned by core | `Proven` | public widget or equivalent public conversation surface -> conversation session -> queue item -> operator follow-up path | public intake creates a core-owned conversation record, queue visibility exists, operators can continue from the same record | Covered by [`internal/service/handlers/public_conversation_handler_test.go`](../internal/service/handlers/public_conversation_handler_test.go) and archived as `workflow-proof/public-conversation-intake.json` |
| Extension or agent initiated case action executes through a sanctioned core-action contract | `Proven` | extension/agent producer -> supported case-action contract -> production consumer -> durable case result | command consumed by a production worker, resulting case action recorded durably, follow-up response event visible, and failure retry state remains queryable | Covered by [`internal/service/handlers/case_command_handler_test.go`](../internal/service/handlers/case_command_handler_test.go), [`internal/workers/manager_test.go`](../internal/workers/manager_test.go), and [`internal/infrastructure/container/container_integration_test.go`](../internal/infrastructure/container/container_integration_test.go), archived as `workflow-proof/case-command-create.json` plus `workflow-proof/case-command-failure-visible.json` |

## Stream Parity Checklist

Every production command stream used by any workflow above must have:

| Stream | Producer Exists | Worker Manager Wiring | Container Startup Wiring | Failure Proof Exists | Workflow Proof Exists | Status |
| --- | --- | --- | --- | --- | --- | --- |
| `email-commands` | Yes | Yes | Yes | Yes | Yes | `Proven` |
| `notification-commands` | Yes | Yes | Yes | Yes | Yes | `Proven` |
| `case-commands` | Yes | Yes | Yes | Yes | Yes | `Proven` |

The stream-wiring and failure-proof evidence currently lives in:

- [`internal/workers/manager_test.go`](../internal/workers/manager_test.go)
- [`internal/infrastructure/container/container_integration_test.go`](../internal/infrastructure/container/container_integration_test.go)
- [`internal/service/handlers/email_command_handler_test.go`](../internal/service/handlers/email_command_handler_test.go) archived as `workflow-proof/email-command-failure-visible.json`
- [`internal/service/handlers/notification_command_handler_test.go`](../internal/service/handlers/notification_command_handler_test.go) archived as `workflow-proof/notification-command-failure-visible.json`
- [`internal/service/handlers/case_command_handler_test.go`](../internal/service/handlers/case_command_handler_test.go) archived as `workflow-proof/case-command-create.json` plus `workflow-proof/case-command-failure-visible.json`

## Current State

- The production command-driven operational workflows now have end-to-end automated proof.
- Base operator case creation, work-management actions, and attachment-bearing
  case workflows now also have real workflow proof through supported product
  surfaces.
- ATS candidate intake now also has explicit attachment-bearing workflow proof:
  the archived scenario requires a clean uploaded resume attachment to be
  linked onto the created candidate case while preserving portfolio context in
  durable case fields.
- Conversation reply, handoff, escalation, and the supported public web-chat
  intake path are now proven through supported product surfaces, and the proof
  bundle archives machine-readable artifacts for each step of that loop.
- The sanctioned extension-or-agent case creation contract is now real through
  `case-commands`, with worker-manager wiring, container startup wiring,
  success proof, and failure-visible retry proof.
- The milestone proof bundle archives machine-readable workflow artifacts for
  those flows, including failure-visible command artifacts.
- The full `go test -tags=integration ./...` sweep is green and hard-gated in
  CI.
- Worker-manager registration and full container startup wiring are covered by
  automated tests for the scoped command streams.
- Milestone 1 is now proven against the expanded product-complete bar, so new
  regressions in any row above should be treated as Milestone 1 evidence
  regressions rather than as optional follow-on work.

## Update Rule

When a scoped workflow changes:

- update this matrix
- update the relevant tests
- update milestone/readiness docs if the proof status changed
