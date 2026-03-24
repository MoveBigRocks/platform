CREATE TABLE ${SCHEMA_NAME}.properties (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL,
    extension_install_id UUID NOT NULL,
    domain TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    status TEXT NOT NULL DEFAULT 'pending',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ${SCHEMA_NAME}.hostname_rules (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL,
    extension_install_id UUID NOT NULL,
    property_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.properties(id),
    pattern TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ${SCHEMA_NAME}.goals (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL,
    extension_install_id UUID NOT NULL,
    property_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.properties(id),
    goal_type TEXT NOT NULL DEFAULT 'event',
    event_name TEXT NOT NULL DEFAULT '',
    page_path TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
