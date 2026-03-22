package models

import "time"

type KnowledgeResource struct {
	ID                 string     `db:"id"`
	WorkspaceID        string     `db:"workspace_id"`
	OwnerTeamID        string     `db:"owner_team_id"`
	Slug               string     `db:"slug"`
	Title              string     `db:"title"`
	Kind               string     `db:"kind"`
	ConceptSpecKey     string     `db:"concept_spec_key"`
	ConceptSpecVersion string     `db:"concept_spec_version"`
	SourceKind         string     `db:"source_kind"`
	SourceRef          *string    `db:"source_ref"`
	PathRef            *string    `db:"path_ref"`
	Summary            string     `db:"summary"`
	BodyMarkdown       string     `db:"body_markdown"`
	FrontmatterJSON    string     `db:"frontmatter_json"`
	SupportedChannels  string     `db:"supported_channels"`
	SharedWithTeamIDs  string     `db:"shared_with_team_ids"`
	Surface            string     `db:"surface"`
	TrustLevel         string     `db:"trust_level"`
	SearchKeywords     string     `db:"search_keywords"`
	Status             string     `db:"status"`
	ReviewStatus       string     `db:"review_status"`
	ContentHash        *string    `db:"content_hash"`
	ReviewedAt         *time.Time `db:"reviewed_at"`
	ArtifactPath       string     `db:"artifact_path"`
	RevisionRef        *string    `db:"revision_ref"`
	PublishedRevision  *string    `db:"published_revision_ref"`
	PublishedAt        *time.Time `db:"published_at"`
	PublishedBy        *string    `db:"published_by"`
	SearchVector       string     `db:"search_vector"`
	CreatedBy          *string    `db:"created_by"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
	DeletedAt          *time.Time `db:"deleted_at"`
}

func (KnowledgeResource) TableName() string { return "knowledge_resources" }

type CaseKnowledgeResourceLink struct {
	ID                  string     `db:"id"`
	WorkspaceID         string     `db:"workspace_id"`
	CaseID              string     `db:"case_id"`
	KnowledgeResourceID string     `db:"knowledge_resource_id"`
	LinkedByID          *string    `db:"linked_by_id"`
	LinkedAt            time.Time  `db:"linked_at"`
	IsAutoSuggested     bool       `db:"is_auto_suggested"`
	RelevanceScore      int        `db:"relevance_score"`
	WasHelpful          *bool      `db:"was_helpful"`
	FeedbackBy          *string    `db:"feedback_by"`
	FeedbackAt          *time.Time `db:"feedback_at"`
	FeedbackComment     string     `db:"feedback_comment"`
}

func (CaseKnowledgeResourceLink) TableName() string { return "case_knowledge_resource_links" }
