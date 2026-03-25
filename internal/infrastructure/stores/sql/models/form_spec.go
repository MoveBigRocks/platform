package models

import "time"

type FormSpec struct {
	ID                       string     `db:"id"`
	WorkspaceID              string     `db:"workspace_id"`
	Name                     string     `db:"name"`
	Slug                     string     `db:"slug"`
	PublicKey                *string    `db:"public_key"`
	DescriptionMarkdown      string     `db:"description_markdown"`
	FieldSpecJSON            string     `db:"field_spec_json"`
	EvidenceRequirementsJSON string     `db:"evidence_requirements_json"`
	InferenceRulesJSON       string     `db:"inference_rules_json"`
	ApprovalPolicyJSON       string     `db:"approval_policy_json"`
	SubmissionPolicyJSON     string     `db:"submission_policy_json"`
	DestinationPolicyJSON    string     `db:"destination_policy_json"`
	SupportedChannels        string     `db:"supported_channels"`
	IsPublic                 bool       `db:"is_public"`
	Status                   string     `db:"status"`
	MetadataJSON             string     `db:"metadata_json"`
	CreatedBy                *string    `db:"created_by"`
	CreatedAt                time.Time  `db:"created_at"`
	UpdatedAt                time.Time  `db:"updated_at"`
	DeletedAt                *time.Time `db:"deleted_at"`
}

func (FormSpec) TableName() string { return "form_specs" }

type FormSubmission struct {
	ID                    string     `db:"id"`
	WorkspaceID           string     `db:"workspace_id"`
	FormSpecID            string     `db:"form_spec_id"`
	ConversationSessionID *string    `db:"conversation_session_id"`
	CaseID                *string    `db:"case_id"`
	ContactID             *string    `db:"contact_id"`
	Status                string     `db:"status"`
	Channel               string     `db:"channel"`
	SubmitterEmail        *string    `db:"submitter_email"`
	SubmitterName         *string    `db:"submitter_name"`
	CompletionToken       *string    `db:"completion_token"`
	CollectedFieldsJSON   string     `db:"collected_fields_json"`
	MissingFieldsJSON     string     `db:"missing_fields_json"`
	EvidenceJSON          string     `db:"evidence_json"`
	ValidationErrorsJSON  string     `db:"validation_errors_json"`
	MetadataJSON          string     `db:"metadata_json"`
	SubmittedAt           *time.Time `db:"submitted_at"`
	CreatedAt             time.Time  `db:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at"`
}

func (FormSubmission) TableName() string { return "form_submissions" }
