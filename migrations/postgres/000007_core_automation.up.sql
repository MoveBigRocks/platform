-- =====================================================
-- Move Big Rocks Schema: Automation
-- Bounded Context: Rules, workflows, jobs
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_automation;

-- =====================================================
-- Rules
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.rules (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    title VARCHAR(255),
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    is_system BOOLEAN DEFAULT false,
    system_rule_key VARCHAR(100),
    priority INTEGER DEFAULT 0,
    conditions JSONB DEFAULT '[]',
    actions JSONB DEFAULT '[]',
    mute_for JSONB DEFAULT '[]',
    max_executions_per_day INTEGER,
    max_executions_per_hour INTEGER,
    team_id UUID,
    case_types JSONB DEFAULT '[]',
    priorities JSONB DEFAULT '[]',
    total_executions INTEGER DEFAULT 0,
    last_executed_at TIMESTAMPTZ,
    average_execution_time BIGINT DEFAULT 0,
    success_rate DOUBLE PRECISION DEFAULT 0,
    created_by_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_rules_workspace ON core_automation.rules(workspace_id);
CREATE INDEX IF NOT EXISTS idx_rules_deleted ON core_automation.rules(deleted_at);

-- =====================================================
-- Rule Executions
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.rule_executions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    rule_id UUID NOT NULL REFERENCES core_automation.rules(id) ON DELETE CASCADE,
    case_id UUID,
    trigger_type VARCHAR(50),
    context JSONB DEFAULT '{}',
    status VARCHAR(50),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    execution_time BIGINT,
    actions_executed JSONB DEFAULT '[]',
    changes JSONB DEFAULT '{}',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rule_exec_workspace ON core_automation.rule_executions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_rule_exec_rule ON core_automation.rule_executions(rule_id);
CREATE INDEX IF NOT EXISTS idx_rule_exec_case ON core_automation.rule_executions(case_id);

-- =====================================================
-- Workflows
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.workflows (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255),
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    version INTEGER DEFAULT 1,
    steps JSONB DEFAULT '[]',
    triggers JSONB DEFAULT '[]',
    variables JSONB DEFAULT '{}',
    timeout_minutes INTEGER DEFAULT 60,
    max_retries INTEGER DEFAULT 3,
    parallel_execution BOOLEAN DEFAULT false,
    total_executions INTEGER DEFAULT 0,
    successful_runs INTEGER DEFAULT 0,
    failed_runs INTEGER DEFAULT 0,
    average_runtime BIGINT DEFAULT 0,
    created_by_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_workflows_workspace ON core_automation.workflows(workspace_id);
CREATE INDEX IF NOT EXISTS idx_workflows_deleted ON core_automation.workflows(deleted_at);

-- =====================================================
-- Workflow Instances
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.workflow_instances (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    workflow_id UUID NOT NULL REFERENCES core_automation.workflows(id) ON DELETE CASCADE,
    name VARCHAR(255),
    status VARCHAR(50),
    current_step VARCHAR(100),
    case_id UUID,
    contact_id UUID,
    user_id UUID,
    context JSONB DEFAULT '{}',
    variables JSONB DEFAULT '{}',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    execution_time BIGINT,
    completed_steps JSONB DEFAULT '[]',
    failed_steps JSONB DEFAULT '[]',
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    created_by_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wf_instances_workspace ON core_automation.workflow_instances(workspace_id);
CREATE INDEX IF NOT EXISTS idx_wf_instances_workflow ON core_automation.workflow_instances(workflow_id);
CREATE INDEX IF NOT EXISTS idx_wf_instances_case ON core_automation.workflow_instances(case_id);

-- =====================================================
-- Assignment Rules
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.assignment_rules (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255),
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    priority INTEGER DEFAULT 0,
    conditions JSONB DEFAULT '[]',
    strategy VARCHAR(50),  -- round_robin, load_balancing, skills, availability
    target_users JSONB DEFAULT '[]',
    target_teams JSONB DEFAULT '[]',
    required_skills JSONB DEFAULT '[]',
    preferred_skills JSONB DEFAULT '[]',
    max_workload INTEGER,
    require_availability BOOLEAN DEFAULT false,
    business_hours_only BOOLEAN DEFAULT false,
    timezone VARCHAR(50),
    business_hours JSONB DEFAULT '{}',
    auto_escalate BOOLEAN DEFAULT false,
    escalation_delay INTEGER,
    escalation_targets JSONB DEFAULT '[]',
    fallback_strategy VARCHAR(50),
    fallback_targets JSONB DEFAULT '[]',
    times_used INTEGER DEFAULT 0,
    last_used_at TIMESTAMPTZ,
    success_rate DOUBLE PRECISION DEFAULT 0,
    created_by_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_assign_rules_workspace ON core_automation.assignment_rules(workspace_id);
CREATE INDEX IF NOT EXISTS idx_assign_rules_deleted ON core_automation.assignment_rules(deleted_at);

-- =====================================================
-- Jobs
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.jobs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    public_id VARCHAR(36) UNIQUE,
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255),
    queue VARCHAR(100),
    priority INTEGER DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending',
    payload JSONB DEFAULT '{}',
    result JSONB DEFAULT '{}',
    error TEXT,
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    scheduled_for TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    worker_id VARCHAR(100),
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_jobs_workspace ON core_automation.jobs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_jobs_queue ON core_automation.jobs(queue);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON core_automation.jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_priority ON core_automation.jobs(priority);
CREATE INDEX IF NOT EXISTS idx_jobs_scheduled ON core_automation.jobs(scheduled_for);
CREATE INDEX IF NOT EXISTS idx_jobs_locked ON core_automation.jobs(locked_until);
CREATE INDEX IF NOT EXISTS idx_jobs_deleted ON core_automation.jobs(deleted_at);

-- =====================================================
-- Job Queues
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.job_queues (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(100),
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    max_workers INTEGER DEFAULT 5,
    max_retries INTEGER DEFAULT 3,
    retry_delay INTEGER DEFAULT 60,
    process_timeout INTEGER DEFAULT 300,
    pending_jobs INTEGER DEFAULT 0,
    running_jobs INTEGER DEFAULT 0,
    completed_jobs INTEGER DEFAULT 0,
    failed_jobs INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_job_queues_workspace ON core_automation.job_queues(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_job_queues_ws_name_unique
    ON core_automation.job_queues(workspace_id, name);

-- =====================================================
-- Job Templates
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.job_templates (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255),
    description TEXT,
    job_name VARCHAR(255),
    queue VARCHAR(100),
    priority INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    payload_template JSONB DEFAULT '{}',
    variable_schema JSONB DEFAULT '{}',
    times_used INTEGER DEFAULT 0,
    last_used_at TIMESTAMPTZ,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_job_templates_workspace ON core_automation.job_templates(workspace_id);

-- =====================================================
-- Recurring Jobs
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.recurring_jobs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255),
    description TEXT,
    job_name VARCHAR(255),
    queue VARCHAR(100),
    priority INTEGER DEFAULT 0,
    payload JSONB DEFAULT '{}',
    cron_expression VARCHAR(100),
    timezone VARCHAR(50),
    is_active BOOLEAN DEFAULT true,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_job_id UUID,
    run_count INTEGER DEFAULT 0,
    failed_runs INTEGER DEFAULT 0,
    max_runs INTEGER,
    stop_after TIMESTAMPTZ,
    missed_run_policy VARCHAR(50),
    overlap_policy VARCHAR(50),
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recurring_jobs_workspace ON core_automation.recurring_jobs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_recurring_jobs_next ON core_automation.recurring_jobs(next_run_at);

-- =====================================================
-- Job Executions
-- =====================================================
CREATE TABLE IF NOT EXISTS core_automation.job_executions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES core_automation.jobs(id) ON DELETE CASCADE,
    worker_id VARCHAR(100),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration INTEGER,
    status VARCHAR(50),
    result JSONB DEFAULT '{}',
    error TEXT,
    cpu_usage DOUBLE PRECISION,
    memory_usage BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_job_exec_workspace ON core_automation.job_executions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_job_exec_job ON core_automation.job_executions(job_id);

-- =====================================================
