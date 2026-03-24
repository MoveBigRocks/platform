CREATE TABLE ${SCHEMA_NAME}.git_repos (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    application_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.projects(id),
    workspace_id UUID NOT NULL,
    extension_install_id UUID NOT NULL,
    repo_url TEXT NOT NULL DEFAULT '',
    default_branch TEXT NOT NULL DEFAULT 'main',
    access_token TEXT NOT NULL DEFAULT '',
    path_prefix TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE ${SCHEMA_NAME}.projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.error_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE ${SCHEMA_NAME}.git_repos ENABLE ROW LEVEL SECURITY;

CREATE POLICY projects_tenant_isolation ON ${SCHEMA_NAME}.projects
    USING (workspace_id::text = current_setting('app.workspace_id', true));

CREATE POLICY error_events_tenant_isolation ON ${SCHEMA_NAME}.error_events
    USING (workspace_id::text = current_setting('app.workspace_id', true));

CREATE POLICY git_repos_tenant_isolation ON ${SCHEMA_NAME}.git_repos
    USING (workspace_id::text = current_setting('app.workspace_id', true));
