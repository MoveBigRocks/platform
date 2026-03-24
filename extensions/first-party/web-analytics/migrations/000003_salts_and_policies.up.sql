CREATE TABLE ${SCHEMA_NAME}.salts (
    id SERIAL PRIMARY KEY,
    salt BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE ${SCHEMA_NAME}.properties ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.events ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.sessions ENABLE ROW LEVEL SECURITY;

CREATE POLICY properties_tenant_isolation ON ${SCHEMA_NAME}.properties
    USING (workspace_id::text = current_setting('app.workspace_id', true));

CREATE POLICY events_tenant_isolation ON ${SCHEMA_NAME}.events
    USING (workspace_id::text = current_setting('app.workspace_id', true));

CREATE POLICY sessions_tenant_isolation ON ${SCHEMA_NAME}.sessions
    USING (workspace_id::text = current_setting('app.workspace_id', true));
