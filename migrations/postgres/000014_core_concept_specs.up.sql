-- =====================================================
-- Move Big Rocks Schema: Concept Specs
-- Bounded Context: Structured knowledge definitions
-- =====================================================

CREATE TABLE IF NOT EXISTS core_knowledge.concept_specs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    owner_team_id UUID REFERENCES core_platform.teams(id) ON DELETE SET NULL,
    spec_key VARCHAR(200) NOT NULL,
    spec_version VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    extends_key VARCHAR(200),
    extends_version VARCHAR(50),
    instance_kind VARCHAR(50) NOT NULL,
    metadata_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    sections_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    workflow_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    agent_guidance_markdown TEXT NOT NULL DEFAULT '',
    artifact_path TEXT NOT NULL,
    revision_ref VARCHAR(64),
    source_kind VARCHAR(50) NOT NULL DEFAULT 'workspace',
    source_ref TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_concept_specs_workspace_key_version
    ON core_knowledge.concept_specs(workspace_id, spec_key, spec_version)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_concept_specs_workspace
    ON core_knowledge.concept_specs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_concept_specs_owner_team
    ON core_knowledge.concept_specs(owner_team_id);
CREATE INDEX IF NOT EXISTS idx_concept_specs_status
    ON core_knowledge.concept_specs(status);
CREATE INDEX IF NOT EXISTS idx_concept_specs_source_kind
    ON core_knowledge.concept_specs(source_kind);

ALTER TABLE core_knowledge.knowledge_resources
    ADD COLUMN IF NOT EXISTS concept_spec_key VARCHAR(200);

ALTER TABLE core_knowledge.knowledge_resources
    ADD COLUMN IF NOT EXISTS concept_spec_version VARCHAR(50);

UPDATE core_knowledge.knowledge_resources
SET concept_spec_key = CASE kind
        WHEN 'policy' THEN 'core/policy'
        WHEN 'guide' THEN 'core/guide'
        WHEN 'skill' THEN 'core/skill'
        WHEN 'context' THEN 'core/context'
        WHEN 'constraint' THEN 'core/constraint'
        WHEN 'best_practice' THEN 'core/best-practice'
        WHEN 'template' THEN 'core/template'
        WHEN 'checklist' THEN 'core/checklist'
        WHEN 'decision' THEN 'core/rfc'
        WHEN 'idea' THEN 'core/idea'
        ELSE 'core/guide'
    END
WHERE concept_spec_key IS NULL OR concept_spec_key = '';

UPDATE core_knowledge.knowledge_resources
SET concept_spec_version = '1'
WHERE concept_spec_version IS NULL OR concept_spec_version = '';

ALTER TABLE core_knowledge.knowledge_resources
    ALTER COLUMN concept_spec_key SET NOT NULL;

ALTER TABLE core_knowledge.knowledge_resources
    ALTER COLUMN concept_spec_version SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_knowledge_resources_concept_spec
    ON core_knowledge.knowledge_resources(concept_spec_key, concept_spec_version);
