-- =====================================================
-- Move Big Rocks Schema: Forms
-- Bounded Context: Form specs and submissions
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_service;

-- =====================================================
-- Form Specs
-- Agent-readable form definitions for humans, chat, imports, and agents
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.form_specs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    public_key VARCHAR(80) UNIQUE,
    description_markdown TEXT NOT NULL DEFAULT '',
    field_spec_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    evidence_requirements_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    inference_rules_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    approval_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    submission_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    destination_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    supported_channels TEXT[] NOT NULL DEFAULT '{}'::text[],
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_form_specs_workspace ON core_service.form_specs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_form_specs_status ON core_service.form_specs(status);
CREATE INDEX IF NOT EXISTS idx_form_specs_deleted ON core_service.form_specs(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_form_specs_ws_slug_unique
    ON core_service.form_specs(workspace_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_form_specs_channels
    ON core_service.form_specs USING GIN (supported_channels);

-- =====================================================
-- Form Access Tokens
-- Scoped access tokens for public/API form submission surfaces
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.form_access_tokens (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    form_spec_id UUID NOT NULL REFERENCES core_service.form_specs(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    allowed_hosts TEXT[] NOT NULL DEFAULT '{}'::text[],
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_form_access_tokens_workspace
    ON core_service.form_access_tokens(workspace_id);
CREATE INDEX IF NOT EXISTS idx_form_access_tokens_spec
    ON core_service.form_access_tokens(form_spec_id);
CREATE INDEX IF NOT EXISTS idx_form_access_tokens_active
    ON core_service.form_access_tokens(is_active);
CREATE INDEX IF NOT EXISTS idx_form_access_tokens_allowed_hosts
    ON core_service.form_access_tokens USING GIN (allowed_hosts);

-- =====================================================
-- Form Submissions
-- Drafts and submitted payloads collected through any allowed surface
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.form_submissions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    form_spec_id UUID NOT NULL REFERENCES core_service.form_specs(id) ON DELETE CASCADE,
    conversation_session_id UUID REFERENCES core_service.conversation_sessions(id) ON DELETE SET NULL,
    case_id UUID REFERENCES core_service.cases(id) ON DELETE SET NULL,
    contact_id UUID REFERENCES core_platform.contacts(id) ON DELETE SET NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    channel VARCHAR(50) NOT NULL DEFAULT 'operator_console',
    submitter_email VARCHAR(255),
    submitter_name VARCHAR(255),
    completion_token VARCHAR(100),
    collected_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    missing_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    evidence_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    validation_errors_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    submitted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_form_submissions_workspace ON core_service.form_submissions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_form_submissions_spec ON core_service.form_submissions(form_spec_id);
CREATE INDEX IF NOT EXISTS idx_form_submissions_status ON core_service.form_submissions(status);
CREATE INDEX IF NOT EXISTS idx_form_submissions_case ON core_service.form_submissions(case_id);
CREATE INDEX IF NOT EXISTS idx_form_submissions_contact ON core_service.form_submissions(contact_id);
CREATE INDEX IF NOT EXISTS idx_form_submissions_conversation
    ON core_service.form_submissions(conversation_session_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_form_submissions_completion_token
    ON core_service.form_submissions(completion_token)
    WHERE completion_token IS NOT NULL;
