-- =============================================================================
-- Move Big Rocks Schema: Row-Level Security and Security Infrastructure
-- =============================================================================
-- This migration sets up PostgreSQL Row-Level Security (RLS) for multi-tenant
-- isolation and creates security infrastructure (admin role, helper functions).
--
-- Design principles:
-- 1. All tables with workspace_id get RLS enabled
-- 2. Policies use session variable set by application middleware
-- 3. Empty/null workspace_id returns no rows (fail-safe via NULL comparison)
-- 4. Superuser bypasses RLS (PostgreSQL default)
--
-- How it works:
-- 1. Application middleware calls: SET LOCAL app.current_workspace_id = '<workspace-uuid>'
-- 2. All queries automatically filter by workspace_id = public.current_workspace_id()
-- 3. If middleware fails to set variable, queries return empty (not all data)
-- =============================================================================

-- =============================================================================
-- HELPER FUNCTION: Get current workspace from session variable
-- =============================================================================

CREATE OR REPLACE FUNCTION public.current_workspace_id()
RETURNS UUID
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
AS $$
BEGIN
    -- Returns NULL if not set, which will match no rows in RLS predicates.
    RETURN NULLIF(current_setting('app.current_workspace_id', true), '')::uuid;
END;
$$;

COMMENT ON FUNCTION public.current_workspace_id() IS
'Returns the current workspace UUID from session variable. Used by RLS policies.';

-- =============================================================================
-- ENABLE RLS ON ALL TENANT-SCOPED TABLES
-- =============================================================================

-- Workspace Domain
ALTER TABLE core_platform.teams ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_platform.teams FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_platform.teams;
CREATE POLICY tenant_isolation ON core_platform.teams FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_platform.team_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_platform.team_members FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_platform.team_members;
CREATE POLICY tenant_isolation ON core_platform.team_members FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_platform.contacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_platform.contacts FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_platform.contacts;
CREATE POLICY tenant_isolation ON core_platform.contacts FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_platform.workspace_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_platform.workspace_settings FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_platform.workspace_settings;
CREATE POLICY tenant_isolation ON core_platform.workspace_settings FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_identity.user_workspace_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_identity.user_workspace_roles FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_identity.user_workspace_roles;
CREATE POLICY tenant_isolation ON core_identity.user_workspace_roles FOR ALL USING (workspace_id = public.current_workspace_id());

-- Service Domain
ALTER TABLE core_service.case_queues ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.case_queues FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.case_queues;
CREATE POLICY tenant_isolation ON core_service.case_queues FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.queue_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.queue_items FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.queue_items;
CREATE POLICY tenant_isolation ON core_service.queue_items FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.cases FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.cases;
CREATE POLICY tenant_isolation ON core_service.cases FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.service_catalog_nodes ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.service_catalog_nodes FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.service_catalog_nodes;
CREATE POLICY tenant_isolation ON core_service.service_catalog_nodes FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.service_catalog_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.service_catalog_bindings FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.service_catalog_bindings;
CREATE POLICY tenant_isolation ON core_service.service_catalog_bindings FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.conversation_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.conversation_sessions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.conversation_sessions;
CREATE POLICY tenant_isolation ON core_service.conversation_sessions FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.conversation_participants ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.conversation_participants FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.conversation_participants;
CREATE POLICY tenant_isolation ON core_service.conversation_participants FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.conversation_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.conversation_messages FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.conversation_messages;
CREATE POLICY tenant_isolation ON core_service.conversation_messages FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.conversation_working_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.conversation_working_state FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.conversation_working_state;
CREATE POLICY tenant_isolation ON core_service.conversation_working_state FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.conversation_outcomes ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.conversation_outcomes FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.conversation_outcomes;
CREATE POLICY tenant_isolation ON core_service.conversation_outcomes FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.communications ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.communications FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.communications;
CREATE POLICY tenant_isolation ON core_service.communications FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.case_assignment_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.case_assignment_history FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.case_assignment_history;
CREATE POLICY tenant_isolation ON core_service.case_assignment_history FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.attachments ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.attachments FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.attachments;
CREATE POLICY tenant_isolation ON core_service.attachments FOR ALL USING (workspace_id = public.current_workspace_id());

-- Email Domain
ALTER TABLE core_service.email_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.email_templates FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.email_templates;
CREATE POLICY tenant_isolation ON core_service.email_templates FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.outbound_emails ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.outbound_emails FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.outbound_emails;
CREATE POLICY tenant_isolation ON core_service.outbound_emails FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.inbound_emails ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.inbound_emails FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.inbound_emails;
CREATE POLICY tenant_isolation ON core_service.inbound_emails FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.email_threads ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.email_threads FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.email_threads;
CREATE POLICY tenant_isolation ON core_service.email_threads FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.email_thread_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.email_thread_links FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.email_thread_links;
CREATE POLICY tenant_isolation ON core_service.email_thread_links FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.thread_merges ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.thread_merges FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.thread_merges;
CREATE POLICY tenant_isolation ON core_service.thread_merges FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.thread_splits ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.thread_splits FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.thread_splits;
CREATE POLICY tenant_isolation ON core_service.thread_splits FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.thread_analytics ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.thread_analytics FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.thread_analytics;
CREATE POLICY tenant_isolation ON core_service.thread_analytics FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.email_stats ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.email_stats FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.email_stats;
CREATE POLICY tenant_isolation ON core_service.email_stats FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.email_blacklists ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.email_blacklists FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.email_blacklists;
CREATE POLICY tenant_isolation ON core_service.email_blacklists FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.quarantined_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.quarantined_messages FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.quarantined_messages;
CREATE POLICY tenant_isolation ON core_service.quarantined_messages FOR ALL USING (workspace_id = public.current_workspace_id());

-- Forms Domain
ALTER TABLE core_service.form_specs ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.form_specs FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.form_specs;
CREATE POLICY tenant_isolation ON core_service.form_specs FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.form_access_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.form_access_tokens FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.form_access_tokens;
CREATE POLICY tenant_isolation ON core_service.form_access_tokens FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_service.form_submissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_service.form_submissions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_service.form_submissions;
CREATE POLICY tenant_isolation ON core_service.form_submissions FOR ALL USING (workspace_id = public.current_workspace_id());

-- Automation Domain
ALTER TABLE core_automation.rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.rules FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.rules;
CREATE POLICY tenant_isolation ON core_automation.rules FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_automation.rule_executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.rule_executions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.rule_executions;
CREATE POLICY tenant_isolation ON core_automation.rule_executions FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_automation.workflows ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.workflows FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.workflows;
CREATE POLICY tenant_isolation ON core_automation.workflows FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_automation.workflow_instances ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.workflow_instances FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.workflow_instances;
CREATE POLICY tenant_isolation ON core_automation.workflow_instances FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_automation.assignment_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.assignment_rules FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.assignment_rules;
CREATE POLICY tenant_isolation ON core_automation.assignment_rules FOR ALL USING (workspace_id = public.current_workspace_id());

-- Jobs Domain
ALTER TABLE core_automation.jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.jobs FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.jobs;
CREATE POLICY tenant_isolation ON core_automation.jobs FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_automation.job_queues ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.job_queues FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.job_queues;
CREATE POLICY tenant_isolation ON core_automation.job_queues FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_automation.job_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.job_templates FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.job_templates;
CREATE POLICY tenant_isolation ON core_automation.job_templates FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_automation.recurring_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.recurring_jobs FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.recurring_jobs;
CREATE POLICY tenant_isolation ON core_automation.recurring_jobs FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_automation.job_executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_automation.job_executions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_automation.job_executions;
CREATE POLICY tenant_isolation ON core_automation.job_executions FOR ALL USING (workspace_id = public.current_workspace_id());

-- Knowledge Domain
ALTER TABLE core_knowledge.knowledge_resources ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_knowledge.knowledge_resources FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_knowledge.knowledge_resources;
CREATE POLICY tenant_isolation ON core_knowledge.knowledge_resources FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_knowledge.case_knowledge_resource_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_knowledge.case_knowledge_resource_links FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_knowledge.case_knowledge_resource_links;
CREATE POLICY tenant_isolation ON core_knowledge.case_knowledge_resource_links FOR ALL USING (workspace_id = public.current_workspace_id());

-- Access & Audit Domain

ALTER TABLE core_governance.custom_fields ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_governance.custom_fields FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_governance.custom_fields;
CREATE POLICY tenant_isolation ON core_governance.custom_fields FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_governance.audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_governance.audit_logs FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_governance.audit_logs;
CREATE POLICY tenant_isolation ON core_governance.audit_logs FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_governance.audit_configurations ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_governance.audit_configurations FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_governance.audit_configurations;
CREATE POLICY tenant_isolation ON core_governance.audit_configurations FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_governance.alert_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_governance.alert_rules FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_governance.alert_rules;
CREATE POLICY tenant_isolation ON core_governance.alert_rules FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_governance.security_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_governance.security_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_governance.security_events;
CREATE POLICY tenant_isolation ON core_governance.security_events FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_governance.audit_log_retentions ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_governance.audit_log_retentions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_governance.audit_log_retentions;
CREATE POLICY tenant_isolation ON core_governance.audit_log_retentions FOR ALL USING (workspace_id = public.current_workspace_id());

-- Auth Domain (tenant-scoped)
ALTER TABLE core_identity.notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_identity.notifications FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_identity.notifications;
CREATE POLICY tenant_isolation ON core_identity.notifications FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_identity.portal_access_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_identity.portal_access_tokens FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_identity.portal_access_tokens;
CREATE POLICY tenant_isolation ON core_identity.portal_access_tokens FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_identity.roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_identity.roles FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_identity.roles;
CREATE POLICY tenant_isolation ON core_identity.roles FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_identity.user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_identity.user_roles FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_identity.user_roles;
CREATE POLICY tenant_isolation ON core_identity.user_roles FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_identity.subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_identity.subscriptions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_identity.subscriptions;
CREATE POLICY tenant_isolation ON core_identity.subscriptions FOR ALL USING (workspace_id = public.current_workspace_id());

-- Agents Domain
ALTER TABLE core_identity.agents ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS agents_tenant_isolation ON core_identity.agents;
CREATE POLICY agents_tenant_isolation ON core_identity.agents
    FOR ALL USING (workspace_id = public.current_workspace_id());

ALTER TABLE core_identity.agent_tokens ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS agent_tokens_tenant_isolation ON core_identity.agent_tokens;
CREATE POLICY agent_tokens_tenant_isolation ON core_identity.agent_tokens
    FOR ALL USING (
        agent_id IN (SELECT id FROM core_identity.agents WHERE workspace_id = public.current_workspace_id())
    );

ALTER TABLE core_identity.workspace_memberships ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS workspace_memberships_tenant_isolation ON core_identity.workspace_memberships;
CREATE POLICY workspace_memberships_tenant_isolation ON core_identity.workspace_memberships
    FOR ALL USING (workspace_id = public.current_workspace_id());

-- =============================================================================
-- ADMIN ROLE FOR CROSS-TENANT OPERATIONS
-- =============================================================================
-- Workers that need to query across all workspaces (e.g., auto-close, notifications)
-- use this role to bypass RLS. The application switches to this role using:
--   SET LOCAL ROLE mbr_admin;
-- within a transaction.

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'mbr_admin') THEN
        BEGIN
            CREATE ROLE mbr_admin;
        EXCEPTION
            WHEN insufficient_privilege THEN
                RAISE NOTICE 'Skipping CREATE ROLE mbr_admin: %', SQLERRM;
        END;
    END IF;
EXCEPTION
    WHEN duplicate_object THEN
        NULL;
END
$$;

-- Grant mbr_admin the ability to bypass RLS
DO $$
BEGIN
    ALTER ROLE mbr_admin BYPASSRLS;
EXCEPTION
    WHEN insufficient_privilege THEN
        RAISE NOTICE 'Skipping ALTER ROLE mbr_admin BYPASSRLS: %', SQLERRM;
    WHEN OTHERS THEN
        IF SQLERRM LIKE '%tuple concurrently updated%' THEN
            RAISE NOTICE 'Skipping concurrent ALTER ROLE mbr_admin BYPASSRLS';
        ELSE
            RAISE;
        END IF;
END
$$;

-- Grant mbr_admin permission to access all core schemas when the role exists.
DO $$
DECLARE
    schema_name TEXT;
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'mbr_admin') THEN
        FOREACH schema_name IN ARRAY ARRAY[
            'public',
            'core_infra',
            'core_identity',
            'core_platform',
            'core_service',
            'core_automation',
            'core_knowledge',
            'core_governance',
            'core_extension_runtime'
        ]
        LOOP
            IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = schema_name) THEN
                EXECUTE format('GRANT USAGE ON SCHEMA %I TO mbr_admin', schema_name);
                EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %I TO mbr_admin', schema_name);
                EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO mbr_admin', schema_name);
            END IF;
        END LOOP;
    ELSE
        RAISE NOTICE 'Skipping mbr_admin schema grants because the role does not exist';
    END IF;
EXCEPTION
    WHEN insufficient_privilege THEN
        RAISE NOTICE 'Skipping some mbr_admin grants: %', SQLERRM;
END
$$;

-- Allow the application database user to switch to mbr_admin role
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'mbr') THEN
        GRANT mbr_admin TO mbr;
    END IF;
EXCEPTION
    WHEN OTHERS THEN
        IF SQLERRM LIKE '%tuple concurrently updated%' THEN
            RAISE NOTICE 'Skipping concurrent GRANT mbr_admin TO mbr';
        ELSE
            RAISE;
        END IF;
END
$$;

-- Also grant mbr_admin to the current database user when possible. Production
-- environments may not use a username literally named "mbr".
DO $$
BEGIN
    EXECUTE format('GRANT mbr_admin TO %I', current_user);
EXCEPTION
    WHEN OTHERS THEN
        IF SQLERRM LIKE '%tuple concurrently updated%' THEN
            RAISE NOTICE 'Skipping concurrent GRANT mbr_admin TO %', current_user;
        ELSE
            RAISE NOTICE 'Could not grant mbr_admin to %: %', current_user, SQLERRM;
        END IF;
END
$$;

-- Verify the role can be used in environments where the grant succeeds.
DO $$
BEGIN
    SET LOCAL ROLE mbr_admin;
    RESET ROLE;
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING 'Cannot switch to mbr_admin role: %. Admin panel may not work correctly.', SQLERRM;
END
$$;
