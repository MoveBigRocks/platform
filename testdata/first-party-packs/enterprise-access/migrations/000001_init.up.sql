CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.identity_providers (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    public_id TEXT NOT NULL,
    provider_type TEXT NOT NULL,
    display_name TEXT NOT NULL,
    issuer TEXT,
    discovery_url TEXT,
    user_info_url TEXT,
    authorization_url TEXT,
    token_url TEXT,
    jwks_url TEXT,
    client_id TEXT,
    client_secret_ref TEXT,
    redirect_url TEXT,
    scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    claim_mapping JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'draft',
    enforce_sso BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT identity_providers_type_check CHECK (provider_type IN ('oidc', 'saml')),
    CONSTRAINT identity_providers_status_check CHECK (status IN ('draft', 'active', 'disabled'))
);

CREATE INDEX IF NOT EXISTS idx_identity_providers_install
    ON ${SCHEMA_NAME}.identity_providers(extension_install_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_identity_providers_install_name_unique
    ON ${SCHEMA_NAME}.identity_providers(extension_install_id, lower(display_name));

CREATE UNIQUE INDEX IF NOT EXISTS idx_identity_providers_install_public_id_unique
    ON ${SCHEMA_NAME}.identity_providers(extension_install_id, public_id);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.provisioning_rules (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    extension_install_id UUID NOT NULL REFERENCES core_platform.installed_extensions(id) ON DELETE CASCADE,
    identity_provider_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.identity_providers(id) ON DELETE CASCADE,
    workspace_id UUID REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    email_domain TEXT,
    required_claim TEXT,
    required_value TEXT,
    role_slug TEXT,
    auto_join BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_provisioning_rules_install
    ON ${SCHEMA_NAME}.provisioning_rules(extension_install_id);

CREATE INDEX IF NOT EXISTS idx_provisioning_rules_provider
    ON ${SCHEMA_NAME}.provisioning_rules(identity_provider_id);

CREATE INDEX IF NOT EXISTS idx_provisioning_rules_workspace
    ON ${SCHEMA_NAME}.provisioning_rules(workspace_id);
