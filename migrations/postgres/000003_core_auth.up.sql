-- =====================================================
-- Move Big Rocks Schema: Auth
-- Bounded Context: Sessions, roles, tokens, subscriptions
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_identity;

-- =====================================================
-- Sessions (token stored as SHA-256 hash for security)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    token_hash VARCHAR(64) UNIQUE NOT NULL,  -- SHA-256 hash of session token
    user_id UUID NOT NULL REFERENCES core_identity.users(id) ON DELETE CASCADE,
    email VARCHAR(255),
    name VARCHAR(255),
    current_context_type VARCHAR(20),
    current_context_role VARCHAR(50),
    current_context_workspace_id UUID,
    available_contexts JSONB DEFAULT '[]',
    user_agent TEXT,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON core_identity.sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON core_identity.sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON core_identity.sessions(expires_at);

-- =====================================================
-- Magic Links
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.magic_links (
    token VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    user_id UUID,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN DEFAULT false,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_magic_links_email ON core_identity.magic_links(email);
CREATE INDEX IF NOT EXISTS idx_magic_links_user ON core_identity.magic_links(user_id);

-- =====================================================
-- Roles (workspace-scoped permission sets)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.roles (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    type VARCHAR(50),  -- admin, member, viewer, custom
    is_system BOOLEAN DEFAULT false,
    is_default BOOLEAN DEFAULT false,
    permissions JSONB DEFAULT '[]',
    created_by_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_roles_workspace ON core_identity.roles(workspace_id);
CREATE INDEX IF NOT EXISTS idx_roles_name ON core_identity.roles(name);

-- =====================================================
-- User-Role Assignments
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.user_roles (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES core_identity.users(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES core_identity.roles(id) ON DELETE CASCADE,
    assigned_by_id UUID,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user ON core_identity.user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_workspace ON core_identity.user_roles(workspace_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role ON core_identity.user_roles(role_id);

-- =====================================================
-- Portal Access Tokens (for customer portal)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.portal_access_tokens (
    token VARCHAR(64) PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    contact_id UUID NOT NULL,
    email VARCHAR(255),
    type VARCHAR(50),
    used BOOLEAN DEFAULT false,
    used_at TIMESTAMPTZ,
    scopes JSONB DEFAULT '[]',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_portal_tokens_workspace ON core_identity.portal_access_tokens(workspace_id);
CREATE INDEX IF NOT EXISTS idx_portal_tokens_contact ON core_identity.portal_access_tokens(contact_id);
CREATE INDEX IF NOT EXISTS idx_portal_tokens_email ON core_identity.portal_access_tokens(email);
CREATE INDEX IF NOT EXISTS idx_portal_tokens_expires ON core_identity.portal_access_tokens(expires_at);

-- =====================================================
-- Subscriptions (billing)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.subscriptions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID UNIQUE NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    plan VARCHAR(50),
    status VARCHAR(50),
    billing_cycle VARCHAR(20),
    price_per_month INTEGER,
    currency VARCHAR(3) DEFAULT 'USD',
    billing_email VARCHAR(255),
    billing_name VARCHAR(255),
    billing_address_id UUID,
    trial_starts_at TIMESTAMPTZ,
    trial_ends_at TIMESTAMPTZ,
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ,
    cancel_reason TEXT,
    stripe_customer_id VARCHAR(100),
    stripe_subscription_id VARCHAR(100),
    max_users INTEGER,
    max_cases INTEGER,
    max_storage BIGINT,
    max_emails_sent INTEGER,
    current_users INTEGER DEFAULT 0,
    current_cases INTEGER DEFAULT 0,
    current_storage BIGINT DEFAULT 0,
    current_emails_sent INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_plan ON core_identity.subscriptions(plan);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON core_identity.subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_customer ON core_identity.subscriptions(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_sub ON core_identity.subscriptions(stripe_subscription_id);

-- =====================================================
-- Notifications
-- =====================================================
CREATE TABLE IF NOT EXISTS core_identity.notifications (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES core_identity.users(id) ON DELETE CASCADE,
    type VARCHAR(50),
    title VARCHAR(255),
    body TEXT,
    icon_url TEXT,
    target_type VARCHAR(50),
    target_id VARCHAR(36),
    action_url TEXT,
    action_label VARCHAR(100),
    is_read BOOLEAN DEFAULT false,
    read_at TIMESTAMPTZ,
    is_archived BOOLEAN DEFAULT false,
    archived_at TIMESTAMPTZ,
    priority VARCHAR(20) DEFAULT 'normal',
    delivery_methods JSONB DEFAULT '[]',
    email_sent_at TIMESTAMPTZ,
    push_sent_at TIMESTAMPTZ,
    sms_sent_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notifications_workspace ON core_identity.notifications(workspace_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON core_identity.notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_type ON core_identity.notifications(type);
CREATE INDEX IF NOT EXISTS idx_notifications_read ON core_identity.notifications(is_read);
