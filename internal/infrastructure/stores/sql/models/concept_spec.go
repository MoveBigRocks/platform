package models

import "time"

type ConceptSpec struct {
	ID              string     `db:"id"`
	WorkspaceID     string     `db:"workspace_id"`
	OwnerTeamID     *string    `db:"owner_team_id"`
	Key             string     `db:"spec_key"`
	Version         string     `db:"spec_version"`
	Name            string     `db:"name"`
	Description     string     `db:"description"`
	ExtendsKey      *string    `db:"extends_key"`
	ExtendsVersion  *string    `db:"extends_version"`
	InstanceKind    string     `db:"instance_kind"`
	MetadataSchema  string     `db:"metadata_schema_json"`
	SectionsSchema  string     `db:"sections_schema_json"`
	WorkflowSchema  string     `db:"workflow_schema_json"`
	AgentGuidanceMD string     `db:"agent_guidance_markdown"`
	ArtifactPath    string     `db:"artifact_path"`
	RevisionRef     *string    `db:"revision_ref"`
	SourceKind      string     `db:"source_kind"`
	SourceRef       *string    `db:"source_ref"`
	Status          string     `db:"status"`
	CreatedBy       *string    `db:"created_by"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at"`
}

func (ConceptSpec) TableName() string { return "concept_specs" }
