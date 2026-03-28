# Workflow Proof Matrix

This document defines the minimum workflow evidence required before a
user-visible capability can be marked `Proven` in milestone or readiness docs.

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
| Inbound new email creates a case | `Partially evidenced` | Postmark webhook -> inbound email record -> email event worker -> case creation -> case communication persistence | inbound email stored pending, worker processes it, case created, communication created, case state/message count correct | webhook test stops at "stored and event published"; workflow proof does not yet archive this round trip |
| Agent or human reply from a case sends email | `Open` | admin/case reply -> case communication -> outbound email persistence -> `email-commands` consumer -> provider send -> provider message ID/status persistence | reply accepted, outbound record created, consumer runs, provider message ID persisted, later delivery/bounce can update same record | no production consumer for `email-commands`; reply path does not currently persist an `outbound_emails` record |
| Customer reply updates the correct existing case | `Open` | Postmark webhook with `In-Reply-To`/`References` -> parser -> inbound worker -> case match -> reopen/open state transition | parser populates headers, worker matches prior case, no duplicate case created, case status transition is correct | matching logic exists, but real Postmark parser path does not currently prove header extraction and tests inject headers manually |
| Public form submission creates case and sends notification | `Partially evidenced` | public form submit -> submission persistence -> form event handler -> case creation -> `email-commands` consumer -> durable notification side effect | submission completed, case linked, notification command consumed, outbound send/persistence visible | handler test proves queued email event only; notification send is not proven |
| Rule `send_email` action delivers a real side effect | `Open` | case/form trigger -> rules engine -> notification action handler -> `email-commands` consumer -> durable outbound side effect | rule fires, command emitted, consumer runs, durable outbound state recorded | current tests stop at change-set or queued event level |
| Knowledge review notification reaches a durable notification side effect | `Open` | knowledge service action -> `notification-commands` consumer -> durable delivery/persistence | notification command emitted, consumer runs, durable side effect recorded | producer exists, but no production consumer proof exists |

## Stream Parity Checklist

Every production command stream used by any workflow above must have:

| Stream | Producer Exists | Consumer Exists | Workflow Proof Exists | Status |
| --- | --- | --- | --- | --- |
| `email-commands` | Yes | No proven production consumer | No | `Open` |
| `notification-commands` | Yes | No proven production consumer | No | `Open` |
| `case-commands` | Contract exists | No proven production consumer | No | `Open` |

## Immediate Uplift Order

1. Implement and prove the `email-commands` consumer path.
2. Persist outbound email records for case replies and correlate provider
   message IDs back to those records.
3. Fix provider parsing and workflow-proof the reply-thread path using real
   webhook payloads.
4. Implement and prove the `notification-commands` consumer path.
5. Reclassify or replace simulated scenario-runner flows that currently stand in
   for workflow proof.
6. Add milestone-proof artifacts for milestone-critical operational workflows.
7. Make CI outputs distinguish hard-gated checks from soft-gated integration
   sweeps so readiness is not inferred from an ambiguous "passed" signal.

## Update Rule

When a scoped workflow changes:

- update this matrix
- update the relevant tests
- update milestone/readiness docs if the proof status changed
