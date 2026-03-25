package models

import "time"

type ConversationSession struct {
	ID                          string     `db:"id"`
	WorkspaceID                 string     `db:"workspace_id"`
	Channel                     string     `db:"channel"`
	Status                      string     `db:"status"`
	PrimaryContactID            *string    `db:"primary_contact_id"`
	PrimaryCatalogNodeID        *string    `db:"primary_catalog_node_id"`
	ActiveFormSpecID            *string    `db:"active_form_spec_id"`
	ActiveFormSubmissionID      *string    `db:"active_form_submission_id"`
	LinkedCaseID                *string    `db:"linked_case_id"`
	HandlingTeamID              *string    `db:"handling_team_id"`
	AssignedOperatorUserID      *string    `db:"assigned_operator_user_id"`
	DelegatedRuntimeConnectorID *string    `db:"delegated_runtime_connector_id"`
	Title                       *string    `db:"title"`
	LanguageCode                *string    `db:"language_code"`
	SourceRef                   *string    `db:"source_ref"`
	ExternalSessionKey          *string    `db:"external_session_key"`
	OpenedAt                    time.Time  `db:"opened_at"`
	LastActivityAt              time.Time  `db:"last_activity_at"`
	ClosedAt                    *time.Time `db:"closed_at"`
	MetadataJSON                string     `db:"metadata_json"`
	CreatedAt                   time.Time  `db:"created_at"`
	UpdatedAt                   time.Time  `db:"updated_at"`
}

func (ConversationSession) TableName() string { return "conversation_sessions" }

type ConversationParticipant struct {
	ID                    string     `db:"id"`
	WorkspaceID           string     `db:"workspace_id"`
	ConversationSessionID string     `db:"conversation_session_id"`
	ParticipantKind       string     `db:"participant_kind"`
	ParticipantRef        string     `db:"participant_ref"`
	RoleInSession         string     `db:"role_in_session"`
	DisplayName           *string    `db:"display_name"`
	JoinedAt              time.Time  `db:"joined_at"`
	LeftAt                *time.Time `db:"left_at"`
	MetadataJSON          string     `db:"metadata_json"`
	CreatedAt             time.Time  `db:"created_at"`
}

func (ConversationParticipant) TableName() string { return "conversation_participants" }

type ConversationMessage struct {
	ID                    string    `db:"id"`
	WorkspaceID           string    `db:"workspace_id"`
	ConversationSessionID string    `db:"conversation_session_id"`
	ParticipantID         *string   `db:"participant_id"`
	Role                  string    `db:"role"`
	Kind                  string    `db:"kind"`
	Visibility            string    `db:"visibility"`
	ContentText           *string   `db:"content_text"`
	ContentJSON           string    `db:"content_json"`
	SearchVector          string    `db:"search_vector"`
	CreatedAt             time.Time `db:"created_at"`
}

func (ConversationMessage) TableName() string { return "conversation_messages" }

type ConversationWorkingState struct {
	ConversationSessionID     string    `db:"conversation_session_id"`
	WorkspaceID               string    `db:"workspace_id"`
	PrimaryCatalogNodeID      *string   `db:"primary_catalog_node_id"`
	SuggestedCatalogNodesJSON string    `db:"suggested_catalog_nodes_json"`
	ClassificationConfidence  *float64  `db:"classification_confidence"`
	ActivePolicyProfileRef    *string   `db:"active_policy_profile_ref"`
	ActiveFormSpecID          *string   `db:"active_form_spec_id"`
	ActiveFormSubmissionID    *string   `db:"active_form_submission_id"`
	CollectedFieldsJSON       string    `db:"collected_fields_json"`
	MissingFieldsJSON         string    `db:"missing_fields_json"`
	RequiresOperatorReview    bool      `db:"requires_operator_review"`
	UpdatedAt                 time.Time `db:"updated_at"`
}

func (ConversationWorkingState) TableName() string { return "conversation_working_state" }

type ConversationOutcome struct {
	ID                    string    `db:"id"`
	WorkspaceID           string    `db:"workspace_id"`
	ConversationSessionID string    `db:"conversation_session_id"`
	Kind                  string    `db:"kind"`
	ResultRefJSON         string    `db:"result_ref_json"`
	CreatedAt             time.Time `db:"created_at"`
}

func (ConversationOutcome) TableName() string { return "conversation_outcomes" }
