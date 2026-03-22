-- =====================================================
-- Move Big Rocks Schema: OAuth 2.1 for agent authorization
-- Bounded Context: Agent access
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_identity;

-- OAuth Clients (registered applications like ChatGPT)
CREATE TABLE IF NOT EXISTS core_identity.oauth_clients (
    id VARCHAR(64) PRIMARY KEY,
    secret_hash VARCHAR(128) NOT NULL,
    name VARCHAR(255) NOT NULL,
    redirect_uris TEXT[] NOT NULL DEFAULT '{}',
    grant_types TEXT[] NOT NULL DEFAULT '{authorization_code}',
    scopes TEXT[] NOT NULL DEFAULT '{}',
    token_endpoint_auth_method VARCHAR(50) DEFAULT 'client_secret_post',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_clients_name ON core_identity.oauth_clients(name);

-- OAuth Authorization Codes
CREATE TABLE IF NOT EXISTS core_identity.oauth_authorization_codes (
    code TEXT PRIMARY KEY,
    client_id VARCHAR(64) NOT NULL REFERENCES core_identity.oauth_clients(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES core_identity.users(id) ON DELETE CASCADE,
    workspace_scopes JSONB NOT NULL DEFAULT '{}'::jsonb,
    redirect_uri TEXT NOT NULL,
    code_challenge TEXT DEFAULT '',
    code_challenge_method TEXT DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_codes_client ON core_identity.oauth_authorization_codes(client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_codes_user ON core_identity.oauth_authorization_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_codes_expires ON core_identity.oauth_authorization_codes(expires_at);

-- OAuth Access Tokens
CREATE TABLE IF NOT EXISTS core_identity.oauth_access_tokens (
    token_hash TEXT PRIMARY KEY,
    client_id VARCHAR(64) NOT NULL REFERENCES core_identity.oauth_clients(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES core_identity.users(id) ON DELETE CASCADE,
    workspace_scopes JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_client ON core_identity.oauth_access_tokens(client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_tokens_user ON core_identity.oauth_access_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_tokens_expires ON core_identity.oauth_access_tokens(expires_at);

-- OAuth Refresh Tokens
CREATE TABLE IF NOT EXISTS core_identity.oauth_refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    access_token_hash TEXT NOT NULL,
    client_id VARCHAR(64) NOT NULL REFERENCES core_identity.oauth_clients(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES core_identity.users(id) ON DELETE CASCADE,
    workspace_scopes JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_access_token ON core_identity.oauth_refresh_tokens(access_token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_client ON core_identity.oauth_refresh_tokens(client_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON core_identity.oauth_refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires ON core_identity.oauth_refresh_tokens(expires_at);

-- Cleanup function for expired tokens (run periodically)
CREATE OR REPLACE FUNCTION core_identity.cleanup_expired_oauth_tokens()
RETURNS INTEGER AS $$
DECLARE
    deleted_codes INTEGER;
    deleted_tokens INTEGER;
    deleted_refresh_tokens INTEGER;
BEGIN
    DELETE FROM core_identity.oauth_authorization_codes WHERE expires_at < NOW();
    GET DIAGNOSTICS deleted_codes = ROW_COUNT;

    DELETE FROM core_identity.oauth_access_tokens WHERE expires_at < NOW();
    GET DIAGNOSTICS deleted_tokens = ROW_COUNT;

    DELETE FROM core_identity.oauth_refresh_tokens WHERE expires_at < NOW();
    GET DIAGNOSTICS deleted_refresh_tokens = ROW_COUNT;

    RETURN deleted_codes + deleted_tokens + deleted_refresh_tokens;
END;
$$ LANGUAGE plpgsql;
