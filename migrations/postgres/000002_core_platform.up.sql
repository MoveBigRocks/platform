-- =====================================================
-- Move Big Rocks Schema: Platform
-- Bounded Context: Users, workspaces, teams, contacts
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_identity;
CREATE SCHEMA IF NOT EXISTS core_platform;

-- =====================================================
-- Users (global, not tenant-scoped)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255),
    avatar TEXT,
    instance_role VARCHAR(50),  -- super_admin, admin, operator
    is_active BOOLEAN DEFAULT true,
    email_verified BOOLEAN DEFAULT false,
    locked_until TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    last_login_ip VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_email ON core_identity.users(email);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON core_identity.users(deleted_at);

-- =====================================================
-- Workspaces (tenants)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_platform.workspaces (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    short_code VARCHAR(4) UNIQUE NOT NULL, -- 2-4 char code for case IDs (e.g., 'tp')
    description TEXT,
    logo_url TEXT,
    primary_color VARCHAR(7),
    accent_color VARCHAR(7),
    settings JSONB DEFAULT '{}',
    features JSONB DEFAULT '{}',
    storage_bucket VARCHAR(255),
    max_users INTEGER DEFAULT 10,
    max_cases INTEGER DEFAULT 1000,
    max_storage BIGINT DEFAULT 5368709120,  -- 5GB
    is_active BOOLEAN DEFAULT true,
    is_suspended BOOLEAN DEFAULT false,
    suspend_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_workspaces_slug ON core_platform.workspaces(slug);
CREATE INDEX IF NOT EXISTS idx_workspaces_deleted_at ON core_platform.workspaces(deleted_at);

-- =====================================================
-- User-Workspace Roles
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.user_workspace_roles (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES core_identity.users(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,  -- owner, admin, member, viewer
    permissions JSONB DEFAULT '[]',
    invited_by UUID,
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, workspace_id)
);

CREATE INDEX IF NOT EXISTS idx_user_workspace_roles_user ON core_identity.user_workspace_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_workspace_roles_workspace ON core_identity.user_workspace_roles(workspace_id);

-- =====================================================
-- Teams
-- =====================================================
CREATE TABLE IF NOT EXISTS core_platform.teams (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    email_address VARCHAR(255),
    settings JSONB DEFAULT '{}',
    response_time_hours INTEGER DEFAULT 24,
    resolution_time_hours INTEGER DEFAULT 72,
    auto_assign BOOLEAN DEFAULT false,
    auto_assign_keywords JSONB DEFAULT '[]',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_teams_workspace ON core_platform.teams(workspace_id);

-- =====================================================
-- Team Members
-- =====================================================
CREATE TABLE IF NOT EXISTS core_platform.team_members (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    team_id UUID NOT NULL REFERENCES core_platform.teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES core_identity.users(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    role VARCHAR(50),
    is_active BOOLEAN DEFAULT true,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(team_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_team_members_team ON core_platform.team_members(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user ON core_platform.team_members(user_id);
CREATE INDEX IF NOT EXISTS idx_team_members_workspace ON core_platform.team_members(workspace_id);

-- =====================================================
-- Contacts (customers)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_platform.contacts (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    email VARCHAR(255),
    name VARCHAR(255),
    phone VARCHAR(50),
    company VARCHAR(255),
    tags JSONB DEFAULT '[]',
    notes TEXT,
    custom_fields JSONB DEFAULT '{}',
    preferred_language VARCHAR(10),
    timezone VARCHAR(50),
    is_blocked BOOLEAN DEFAULT false,
    blocked_reason TEXT,
    total_cases INTEGER DEFAULT 0,
    last_contact_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_contacts_workspace ON core_platform.contacts(workspace_id);
CREATE INDEX IF NOT EXISTS idx_contacts_email ON core_platform.contacts(email);

-- =====================================================
-- Workspace Settings
-- =====================================================
CREATE TABLE IF NOT EXISTS core_platform.workspace_settings (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID UNIQUE NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    email_from_name VARCHAR(255),
    email_from_address VARCHAR(255),
    settings_json JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_settings_workspace ON core_platform.workspace_settings(workspace_id);

-- =====================================================
-- Installed Extensions
-- =====================================================
CREATE TABLE IF NOT EXISTS core_platform.installed_extensions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    slug VARCHAR(120) NOT NULL,
    name VARCHAR(255) NOT NULL,
    publisher VARCHAR(255) NOT NULL,
    version VARCHAR(100) NOT NULL,
    description TEXT,
    license_token TEXT NOT NULL,
    bundle_sha256 VARCHAR(64) NOT NULL,
    bundle_size BIGINT NOT NULL DEFAULT 0,
    bundle_payload BYTEA NOT NULL DEFAULT '\x'::bytea,
    manifest_json JSONB NOT NULL,
    config_json JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(50) NOT NULL DEFAULT 'installed',
    validation_status VARCHAR(50) NOT NULL DEFAULT 'unknown',
    validation_message TEXT,
    health_status VARCHAR(50) NOT NULL DEFAULT 'unknown',
    health_message TEXT,
    installed_by_id UUID,
    installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    activated_at TIMESTAMPTZ,
    deactivated_at TIMESTAMPTZ,
    validated_at TIMESTAMPTZ,
    last_health_check_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_installed_extensions_workspace ON core_platform.installed_extensions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_installed_extensions_status ON core_platform.installed_extensions(status);
CREATE INDEX IF NOT EXISTS idx_installed_extensions_deleted_at ON core_platform.installed_extensions(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_installed_extensions_workspace_slug_active
    ON core_platform.installed_extensions(workspace_id, slug)
    WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_installed_extensions_instance_slug_active
    ON core_platform.installed_extensions(slug)
    WHERE workspace_id IS NULL AND deleted_at IS NULL;

-- =====================================================
-- Extension Assets
-- =====================================================
CREATE TABLE IF NOT EXISTS core_platform.extension_assets (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    extension_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    kind VARCHAR(50) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    content BYTEA NOT NULL,
    is_customizable BOOLEAN NOT NULL DEFAULT false,
    checksum VARCHAR(64) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(extension_id, path)
);

CREATE INDEX IF NOT EXISTS idx_extension_assets_extension ON core_platform.extension_assets(extension_id);
CREATE INDEX IF NOT EXISTS idx_extension_assets_deleted_at ON core_platform.extension_assets(deleted_at);
