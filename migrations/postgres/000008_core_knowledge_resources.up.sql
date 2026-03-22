-- =====================================================
-- Move Big Rocks Schema: Knowledge
-- Bounded Context: Knowledge resources
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_knowledge;

-- =====================================================
-- Knowledge Resources
-- Markdown-first runtime records for agent and operator retrieval
-- =====================================================
CREATE TABLE IF NOT EXISTS core_knowledge.knowledge_resources (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    owner_team_id UUID NOT NULL REFERENCES core_platform.teams(id) ON DELETE CASCADE,
    slug VARCHAR(200) NOT NULL,
    title VARCHAR(500) NOT NULL,
    kind VARCHAR(50) NOT NULL,
    source_kind VARCHAR(50) NOT NULL DEFAULT 'workspace',
    source_ref TEXT,
    path_ref TEXT,
    artifact_path TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    body_markdown TEXT NOT NULL DEFAULT '',
    frontmatter_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    supported_channels TEXT[] NOT NULL DEFAULT '{}'::text[],
    shared_with_team_ids TEXT[] NOT NULL DEFAULT '{}'::text[],
    surface VARCHAR(50) NOT NULL DEFAULT 'private',
    trust_level VARCHAR(50) NOT NULL DEFAULT 'workspace',
    search_keywords TEXT[] NOT NULL DEFAULT '{}'::text[],
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    review_status VARCHAR(50) NOT NULL DEFAULT 'draft',
    content_hash VARCHAR(64),
    revision_ref VARCHAR(64) NOT NULL DEFAULT '',
    published_revision_ref VARCHAR(64),
    reviewed_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    published_by UUID,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(summary, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(body_markdown, '')), 'C')
    ) STORED
);

CREATE INDEX IF NOT EXISTS idx_knowledge_resources_workspace
    ON core_knowledge.knowledge_resources(workspace_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_resources_owner_team
    ON core_knowledge.knowledge_resources(owner_team_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_resources_status
    ON core_knowledge.knowledge_resources(status);
CREATE INDEX IF NOT EXISTS idx_knowledge_resources_surface
    ON core_knowledge.knowledge_resources(surface);
CREATE INDEX IF NOT EXISTS idx_knowledge_resources_review_status
    ON core_knowledge.knowledge_resources(review_status);
CREATE INDEX IF NOT EXISTS idx_knowledge_resources_deleted
    ON core_knowledge.knowledge_resources(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS uq_knowledge_resources_ws_team_surface_slug
    ON core_knowledge.knowledge_resources(workspace_id, owner_team_id, surface, slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_knowledge_resources_channels
    ON core_knowledge.knowledge_resources USING GIN (supported_channels);
CREATE INDEX IF NOT EXISTS idx_knowledge_resources_shared_teams
    ON core_knowledge.knowledge_resources USING GIN (shared_with_team_ids);
CREATE INDEX IF NOT EXISTS idx_knowledge_resources_search
    ON core_knowledge.knowledge_resources USING GIN (search_vector);

-- =====================================================
-- Case Knowledge Resource Links
-- Explicit links between operational work and relevant knowledge resources
-- =====================================================
CREATE TABLE IF NOT EXISTS core_knowledge.case_knowledge_resource_links (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    case_id UUID NOT NULL REFERENCES core_service.cases(id) ON DELETE CASCADE,
    knowledge_resource_id UUID NOT NULL REFERENCES core_knowledge.knowledge_resources(id) ON DELETE CASCADE,
    linked_by_id UUID,
    linked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_auto_suggested BOOLEAN NOT NULL DEFAULT FALSE,
    relevance_score INTEGER NOT NULL DEFAULT 0,
    was_helpful BOOLEAN,
    feedback_by UUID,
    feedback_at TIMESTAMPTZ,
    feedback_comment TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_case_knowledge_resource_links_workspace
    ON core_knowledge.case_knowledge_resource_links(workspace_id);
CREATE INDEX IF NOT EXISTS idx_case_knowledge_resource_links_case
    ON core_knowledge.case_knowledge_resource_links(case_id);
CREATE INDEX IF NOT EXISTS idx_case_knowledge_resource_links_resource
    ON core_knowledge.case_knowledge_resource_links(knowledge_resource_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_case_knowledge_resource_links_case_resource
    ON core_knowledge.case_knowledge_resource_links(case_id, knowledge_resource_id);
