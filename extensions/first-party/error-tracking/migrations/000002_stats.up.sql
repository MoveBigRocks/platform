CREATE TABLE ${SCHEMA_NAME}.project_stats (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL,
    extension_install_id UUID NOT NULL,
    project_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.projects(id),
    date DATE NOT NULL,
    event_count BIGINT NOT NULL DEFAULT 0,
    issue_count BIGINT NOT NULL DEFAULT 0,
    user_count BIGINT NOT NULL DEFAULT 0,
    error_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    new_issues BIGINT NOT NULL DEFAULT 0,
    resolved_issues BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE ${SCHEMA_NAME}.issue_stats (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL,
    extension_install_id UUID NOT NULL,
    issue_id UUID NOT NULL REFERENCES ${SCHEMA_NAME}.issues(id),
    date DATE NOT NULL,
    event_count BIGINT NOT NULL DEFAULT 0,
    user_count BIGINT NOT NULL DEFAULT 0,
    first_occurrence TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_occurrence TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
