# Move Big Rocks PostgreSQL Migration Ownership Map

This document is the concrete table-to-file plan for the PostgreSQL baseline
reset described in
[RFC-0006](../../docs/RFCs/RFC-0006-postgres-and-extension-schemas.md).

The goal is the same as the `../tuinplan` approach:

- one physical application database per environment
- one central migration runner
- ownership-aligned files instead of one giant init file
- extension-owned schemas managed separately from the core baseline

## Principles

- file ownership is logical, not a promise of separate databases
- dependency order matters more than perfect one-to-one domain purity
- tenant-scoped tables should exist before RLS policies are applied
- applied migration versions are immutable
- service-backed extension schemas are not part of the core baseline

## Physical Schema Context

Current physical schema layout:

- `core_infra`
- `core_identity`
- `core_platform`
- `core_service`
- `core_automation`
- `core_knowledge`
- `core_governance`
- `core_extension_runtime`
- `ext_<publisher>_<slug>` for service-backed extension-owned tables
- `public` only for shared migration/runtime helpers such as
  `public.schema_migrations` and `current_workspace_id()`

This ownership map is therefore about migration organization within the shared
PostgreSQL database, not about splitting the runtime into separate datastores.

Core migration versions are tracked in `public.schema_migrations`.
Extension-owned schema versions are tracked separately in
`core_extension_runtime.schema_migration_history`.

## Target PostgreSQL File Layout

```text
migrations/postgres/
├── 000001_core_infra.up.sql
├── 000002_core_platform.up.sql
├── 000003_core_auth.up.sql
├── 000004_core_service.up.sql
├── 000005_core_email.up.sql
├── 000006_core_forms.up.sql
├── 000007_core_automation.up.sql
├── 000008_core_knowledge_resources.up.sql
├── 000009_core_access_audit.up.sql
├── 000010_core_agents.up.sql
├── 000011_core_rls.up.sql
├── 000012_core_oauth.up.sql
└── 000013_core_extension_runtime.up.sql
```

## Table-to-File Assignment

### 000001_core_infra

Logical ownership: platform infrastructure

Physical schema: `core_infra`

- `outbox_events`
- `event_dlq`
- `processed_events`
- `rate_limit_entries`
- `files`

Notes:

- this file also folds in the current idempotency and store-hardening baseline fixes
- infrastructure primitives must exist before domain-specific tables that depend on them

### 000002_core_platform

Logical ownership: workspace and extension platform primitives

Physical schemas: `core_identity` and `core_platform`

- `users`
- `workspaces`
- `user_workspace_roles`
- `teams`
- `team_members`
- `contacts`
- `workspace_settings`
- `installed_extensions`
- `extension_assets`

Notes:

- `teams` and `team_members` stay here because they are shared workspace-level primitives used by service, observability, and automation
- `installed_extensions` and `extension_assets` are platform-owned even though service-backed extension schemas live elsewhere
- `installed_extensions` lands directly in its final shape here, including
  `bundle_payload`, nullable `workspace_id` for instance-scoped installs, and
  partial uniqueness for workspace-scoped versus instance-scoped slugs

### 000003_core_auth

Logical ownership: core authentication and workspace-scoped authz primitives

Physical schema: `core_identity`

- `sessions`
- `magic_links`
- `roles`
- `user_roles`
- `portal_access_tokens`
- `subscriptions`
- `notifications`

### 000004_core_service

Logical ownership: service operations and case management

Physical schema: `core_service`

- `service_catalog_nodes`
- `service_catalog_bindings`
- `conversation_sessions`
- `conversation_participants`
- `conversation_messages`
- `conversation_working_state`
- `conversation_outcomes`
- `cases`
- `communications`
- `case_assignment_history`
- `attachments`

Notes:

- service classification and conversation data land before forms and email because both depend on the shared operational graph
- service case data lands before email because email threads and messages can point at cases

### 000005_core_email

Logical ownership: email operations

Physical schema: `core_service`

- `email_templates`
- `outbound_emails`
- `inbound_emails`
- `email_threads`
- `email_thread_links`
- `thread_merges`
- `thread_splits`
- `thread_analytics`
- `email_stats`
- `email_blacklists`
- `quarantined_messages`

### 000006_core_forms

Logical ownership: form specs and submission state

Physical schema: `core_service`

- `form_specs`
- `form_submissions`

### 000007_core_automation

Logical ownership: rules, workflows, and jobs

Physical schema: `core_automation`

- `rules`
- `rule_executions`
- `workflows`
- `workflow_instances`
- `assignment_rules`
- `jobs`
- `job_queues`
- `job_templates`
- `recurring_jobs`
- `job_executions`

### 000008_core_knowledge_resources

Logical ownership: knowledge resources

Physical schema: `core_knowledge`

- `knowledge_resources`

### 000009_core_access_audit

Logical ownership: audit and security configuration

Physical schema: `core_governance`

- `custom_fields`
- `audit_logs`
- `audit_configurations`
- `alert_rules`
- `security_events`
- `audit_log_retentions`

### 000010_core_agents

Logical ownership: non-human principals and unified workspace membership

Physical schema: `core_identity`

- `agents`
- `agent_tokens`
- `workspace_memberships`
- service-side `communications.from_agent_id` alteration

Notes:

- this file must land after service because it alters `communications`

### 000011_core_rls

Logical ownership: tenancy enforcement and admin-role infrastructure

- `public.current_workspace_id()` helper
- RLS enablement across tenant-scoped tables
- tenant isolation policies for core tables
- `mbr_admin` role creation and grants

Notes:

- keep RLS and grant setup in its own late-stage file so all referenced tables already exist
- do not mix business table creation and RLS policy creation in the same file unless there is a strong dependency reason

### 000012_core_oauth

Logical ownership: OAuth 2.1 authorization for agents and external applications

Physical schema: `core_identity`

- `oauth_clients`
- `oauth_authorization_codes`
- `oauth_access_tokens`
- `oauth_refresh_tokens`
- token cleanup function
- direct per-workspace OAuth scope storage

Notes:

- OAuth remains separate because it is both agent-related and auth-related
- this file lands directly in the final `workspace_scopes` shape instead of
  replaying table-copy and rename steps during fresh database creation

### 000013_core_extension_runtime

Logical ownership: instance-scoped runtime metadata for service-backed extensions

- `core_extension_runtime.extension_package_registrations`
- `core_extension_runtime.schema_migration_history`

Notes:

- this is core-owned metadata, not extension-owned data
- keep it separate from workspace install rows and separate from extension-owned schemas

## Dependency Order

The file order is deliberate:

1. infrastructure tables first
2. platform/workspace primitives next
3. auth before higher-level auth-dependent flows
4. service before email because email threads/messages may connect to cases
5. forms, automation, and knowledge after workspace primitives exist
6. access and audit after the domain tables they reference exist
7. agents after service because of the communications alteration
8. RLS and grants after the tenant-scoped tables exist, including agent tables
9. OAuth after its auth and agent dependencies exist
10. extension runtime metadata near the end because it is operational metadata, not domain data

## What Stays Out of This Baseline

These are intentionally not part of the core PostgreSQL baseline:

- service-backed extension-owned `ext_*` schemas
- extension-shipped `migrations/` executed by the core extension runtime
- operational data import/export tooling
- one-off data migration scripts

## First-Party Extension Canonical Baselines

These are the target schema baselines for first-party service-backed extensions:

### `web-analytics`

Canonical location:

- the `web-analytics` extension source outside this repo

Owns:

- analytics properties
- hostname allowlists/rules
- goals
- salts
- events
- sessions

### `error-tracking`

Canonical location:

- the `error-tracking` extension source outside this repo

Owns:

- projects/applications
- issues
- error events
- alerts
- project stats
- issue stats
- git repository mappings used by error resolution flows

### `enterprise-access`

Canonical location:

- the `enterprise-access` extension source outside this repo

Owns:

- enterprise identity providers
- provider discovery and userinfo endpoints
- provisioning rules that map provider claims into Move Big Rocks memberships
