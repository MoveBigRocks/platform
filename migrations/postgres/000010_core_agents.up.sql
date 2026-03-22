-- =====================================================
-- Move Big Rocks Schema: Agents & Unified Authorization
-- Bounded Context: Platform - AI/automation agents
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_identity;

-- =====================================================
-- Agents
-- Non-human principals that can act in a workspace
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.agents (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,

    -- Identity
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Accountability
    owner_id UUID NOT NULL REFERENCES core_identity.users(id),

    -- Lifecycle
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    status_reason TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by_id UUID NOT NULL REFERENCES core_identity.users(id),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT agents_workspace_name_unique UNIQUE (workspace_id, name)
);

CREATE INDEX IF NOT EXISTS idx_agents_workspace ON core_identity.agents(workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_agents_owner ON core_identity.agents(owner_id);
CREATE INDEX IF NOT EXISTS idx_agents_status ON core_identity.agents(status) WHERE deleted_at IS NULL;

-- =====================================================
-- Agent Tokens
-- Authentication credentials for agents
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.agent_tokens (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    agent_id UUID NOT NULL REFERENCES core_identity.agents(id) ON DELETE CASCADE,

    -- Token (only hash stored, never plaintext)
    token_hash VARCHAR(64) NOT NULL,
    token_prefix VARCHAR(16) NOT NULL,

    -- Metadata
    name VARCHAR(255) NOT NULL,

    -- Lifecycle
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoked_by_id UUID REFERENCES core_identity.users(id),

    -- Usage tracking
    last_used_at TIMESTAMPTZ,
    last_used_ip VARCHAR(45),
    use_count BIGINT NOT NULL DEFAULT 0,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by_id UUID NOT NULL REFERENCES core_identity.users(id),

    CONSTRAINT agent_tokens_hash_unique UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_agent_tokens_agent ON core_identity.agent_tokens(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_tokens_hash ON core_identity.agent_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_agent_tokens_prefix ON core_identity.agent_tokens(token_prefix);

-- =====================================================
-- Workspace Memberships
-- Unified authorization for users and agents
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.workspace_memberships (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,

    -- Polymorphic principal (user or agent)
    principal_id UUID NOT NULL,
    principal_type VARCHAR(20) NOT NULL CHECK (principal_type IN ('user', 'agent')),

    -- Role determines base permissions
    role VARCHAR(50) NOT NULL,

    -- Permissions - array of resource:action strings
    -- e.g., ["case:read", "case:write", "issue:read"]
    permissions JSONB NOT NULL DEFAULT '[]',

    -- Constraints - restrictions on access
    constraints JSONB NOT NULL DEFAULT '{}',

    -- Lifecycle
    granted_by_id UUID NOT NULL REFERENCES core_identity.users(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoked_by_id UUID REFERENCES core_identity.users(id),

    -- One membership per principal per workspace
    CONSTRAINT workspace_memberships_unique
        UNIQUE (workspace_id, principal_id, principal_type)
);

CREATE INDEX IF NOT EXISTS idx_workspace_memberships_workspace ON core_identity.workspace_memberships(workspace_id);
CREATE INDEX IF NOT EXISTS idx_workspace_memberships_principal ON core_identity.workspace_memberships(principal_id, principal_type);
-- Partial index for active memberships (not revoked, no expiry check - handled at query time)
CREATE INDEX IF NOT EXISTS idx_workspace_memberships_active ON core_identity.workspace_memberships(workspace_id)
    WHERE revoked_at IS NULL;

-- =====================================================
-- Add agent authorship to communications
-- =====================================================
ALTER TABLE core_service.communications
    ADD COLUMN IF NOT EXISTS from_agent_id UUID REFERENCES core_identity.agents(id);

CREATE INDEX IF NOT EXISTS idx_communications_agent ON core_service.communications(from_agent_id);
