-- =====================================================
-- Move Big Rocks Schema: Access & Audit
-- Bounded Context: Access keys, audit logs, security events
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_governance;

-- =====================================================
-- Custom Fields
-- =====================================================
CREATE TABLE IF NOT EXISTS core_governance.custom_fields (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(100),
    label VARCHAR(255),
    description TEXT,
    type VARCHAR(50),
    data_type VARCHAR(50),
    required BOOLEAN DEFAULT false,
    unique_field BOOLEAN DEFAULT false,
    searchable BOOLEAN DEFAULT false,
    display_order INTEGER DEFAULT 0,
    group_name VARCHAR(100),
    placeholder VARCHAR(255),
    help_text TEXT,
    icon VARCHAR(50),
    hidden BOOLEAN DEFAULT false,
    read_only BOOLEAN DEFAULT false,
    validation JSONB DEFAULT '{}',
    options JSONB DEFAULT '[]',
    default_value JSONB,
    is_computed BOOLEAN DEFAULT false,
    formula TEXT,
    dependencies JSONB DEFAULT '[]',
    view_roles JSONB DEFAULT '[]',
    edit_roles JSONB DEFAULT '[]',
    tags JSONB DEFAULT '[]',
    is_system BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID
);

CREATE INDEX IF NOT EXISTS idx_custom_fields_workspace ON core_governance.custom_fields(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_custom_fields_ws_name_unique
    ON core_governance.custom_fields(workspace_id, name);

-- =====================================================
-- Audit Logs
-- =====================================================
CREATE TABLE IF NOT EXISTS core_governance.audit_logs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    user_id UUID,
    user_email VARCHAR(255),
    user_name VARCHAR(255),
    action VARCHAR(100),
    resource VARCHAR(100),
    resource_id VARCHAR(36),
    resource_name VARCHAR(255),
    old_value JSONB,
    new_value JSONB,
    changes JSONB DEFAULT '[]',
    ip_address VARCHAR(45),
    user_agent TEXT,
    session_id UUID,
    request_id TEXT,
    api_key_id TEXT,
    success BOOLEAN DEFAULT true,
    error_message TEXT,
    metadata JSONB DEFAULT '{}',
    tags JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_workspace ON core_governance.audit_logs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON core_governance.audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON core_governance.audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON core_governance.audit_logs(resource);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id ON core_governance.audit_logs(resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON core_governance.audit_logs(created_at);

-- =====================================================
-- Audit Configurations
-- =====================================================
CREATE TABLE IF NOT EXISTS core_governance.audit_configurations (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID UNIQUE NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    enabled_categories JSONB DEFAULT '[]',
    log_level VARCHAR(20) DEFAULT 'info',
    retention_days INTEGER DEFAULT 90,
    log_api_requests BOOLEAN DEFAULT true,
    log_user_actions BOOLEAN DEFAULT true,
    log_system_events BOOLEAN DEFAULT true,
    log_data_changes BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_config_workspace ON core_governance.audit_configurations(workspace_id);

-- =====================================================
-- Alert Rules (Audit)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_governance.alert_rules (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255),
    description TEXT,
    event_type VARCHAR(100),
    severity VARCHAR(20),
    conditions JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT true,
    channels JSONB DEFAULT '[]',
    recipients JSONB DEFAULT '[]',
    throttle_minutes INTEGER DEFAULT 15,
    last_triggered TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_workspace ON core_governance.alert_rules(workspace_id);

-- =====================================================
-- Security Events
-- =====================================================
CREATE TABLE IF NOT EXISTS core_governance.security_events (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    type VARCHAR(100),
    severity VARCHAR(20),
    description TEXT,
    user_id UUID,
    ip_address VARCHAR(45),
    user_agent TEXT,
    location VARCHAR(255),
    resource VARCHAR(100),
    resource_id VARCHAR(36),
    detection_method VARCHAR(50),
    risk_score INTEGER DEFAULT 0,
    indicators JSONB DEFAULT '[]',
    auto_blocked BOOLEAN DEFAULT false,
    requires_review BOOLEAN DEFAULT false,
    reviewed_by UUID,
    reviewed_at TIMESTAMPTZ,
    action_taken TEXT,
    metadata JSONB DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_security_events_workspace ON core_governance.security_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_security_events_type ON core_governance.security_events(type);
CREATE INDEX IF NOT EXISTS idx_security_events_severity ON core_governance.security_events(severity);
CREATE INDEX IF NOT EXISTS idx_security_events_user ON core_governance.security_events(user_id);

-- =====================================================
-- Audit Log Retention Policies
-- =====================================================
CREATE TABLE IF NOT EXISTS core_governance.audit_log_retentions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID UNIQUE NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    default_retention INTEGER DEFAULT 90,
    authentication_logs INTEGER DEFAULT 365,
    data_access_logs INTEGER DEFAULT 180,
    configuration_logs INTEGER DEFAULT 365,
    security_logs INTEGER DEFAULT 730,
    archive_enabled BOOLEAN DEFAULT false,
    archive_location TEXT,
    archive_after_days INTEGER DEFAULT 90,
    compliance_mode BOOLEAN DEFAULT false,
    compliance_standard VARCHAR(50),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID
);

CREATE INDEX IF NOT EXISTS idx_retention_workspace ON core_governance.audit_log_retentions(workspace_id);
