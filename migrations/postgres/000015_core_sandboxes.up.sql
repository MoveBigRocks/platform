-- =====================================================
-- Move Big Rocks Schema: Sandbox control plane
-- Bounded Context: vendor-operated sandbox lifecycle
-- =====================================================

CREATE TABLE IF NOT EXISTS core_platform.sandboxes (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    slug VARCHAR(120) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    requested_email VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    runtime_url TEXT,
    login_url TEXT,
    bootstrap_url TEXT,
    verification_token_hash VARCHAR(64) NOT NULL UNIQUE,
    manage_token_hash VARCHAR(64) NOT NULL,
    verification_requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified_at TIMESTAMPTZ,
    activation_deadline_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    extended_at TIMESTAMPTZ,
    destroyed_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sandboxes_status ON core_platform.sandboxes(status);
CREATE INDEX IF NOT EXISTS idx_sandboxes_requested_email ON core_platform.sandboxes(requested_email);
CREATE INDEX IF NOT EXISTS idx_sandboxes_activation_deadline ON core_platform.sandboxes(activation_deadline_at);
