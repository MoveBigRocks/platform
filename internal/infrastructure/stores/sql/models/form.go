package models

import (
	"time"
)

// FormSchema represents a public form definition.
type FormSchema struct {
	ID                 string     `db:"id"`
	WorkspaceID        string     `db:"workspace_id"`
	Name               string     `db:"name"`
	Slug               string     `db:"slug"`
	CryptoID           string     `db:"crypto_id"`
	Description        string     `db:"description"`
	SchemaData         string     `db:"schema_data"`
	UISchema           string     `db:"ui_schema"`
	ValidationRules    string     `db:"validation_rules"`
	WorkflowStates     string     `db:"workflow_states"`
	Transitions        string     `db:"transitions"`
	HasWorkflow        bool       `db:"has_workflow"`
	IsPublic           bool       `db:"is_public"`
	RequiresCaptcha    bool       `db:"requires_captcha"`
	CollectEmail       bool       `db:"collect_email"`
	AutoCreateCase     bool       `db:"auto_create_case"`
	AutoCasePriority   string     `db:"auto_case_priority"`
	AutoCaseType       string     `db:"auto_case_type"`
	AutoAssignTeamID   *string    `db:"auto_assign_team_id"`
	AutoTags           string     `db:"auto_tags"`
	NotifyOnSubmission bool       `db:"notify_on_submission"`
	NotificationEmails string     `db:"notification_emails"`
	AllowEmbed         bool       `db:"allow_embed"`
	EmbedDomains       string     `db:"embed_domains"`
	Status             string     `db:"status"`
	SubmissionMessage  string     `db:"submission_message"`
	RedirectURL        string     `db:"redirect_url"`
	CreatedBy          *string    `db:"created_by"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
	DeletedAt          *time.Time `db:"deleted_at"`
}

func (FormSchema) TableName() string {
	return "form_schemas"
}

// PublicFormSubmission represents a submitted public form.
type PublicFormSubmission struct {
	ID               string     `db:"id"`
	WorkspaceID      string     `db:"workspace_id"`
	FormID           string     `db:"form_id"`
	Data             string     `db:"data"`
	SubmitterEmail   string     `db:"submitter_email"`
	SubmitterName    string     `db:"submitter_name"`
	SubmitterIP      string     `db:"submitter_ip"`
	UserAgent        string     `db:"user_agent"`
	Referrer         string     `db:"referrer"`
	Status           string     `db:"status"`
	IsValid          bool       `db:"is_valid"`
	ValidationErrors string     `db:"validation_errors"`
	CurrentStateID   string     `db:"current_state_id"`
	StateHistory     string     `db:"state_history"`
	CompletionToken  string     `db:"completion_token"`
	CaseID           *string    `db:"case_id"`
	ContactID        *string    `db:"contact_id"`
	SpamScore        float64    `db:"spam_score"`
	IsSpam           bool       `db:"is_spam"`
	SpamReasons      string     `db:"spam_reasons"`
	ProcessedAt      *time.Time `db:"processed_at"`
	ProcessedByID    *string    `db:"processed_by_id"`
	ProcessingTime   int64      `db:"processing_time"`
	AttachmentIDs    string     `db:"attachment_ids"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

func (PublicFormSubmission) TableName() string {
	return "form_submissions"
}

// FormAPIToken represents a token for programmatic submission
type FormAPIToken struct {
	ID           string     `db:"id"`
	WorkspaceID  string     `db:"workspace_id"`
	FormID       string     `db:"form_id"`
	Token        string     `db:"token"`
	Name         string     `db:"name"`
	IsActive     bool       `db:"is_active"`
	ExpiresAt    *time.Time `db:"expires_at"`
	AllowedHosts string     `db:"allowed_hosts"`
	LastUsedAt   *time.Time `db:"last_used_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

func (FormAPIToken) TableName() string {
	return "form_api_tokens"
}

// FormAnalytics stores aggregated stats
type FormAnalytics struct {
	FormID      string    `db:"form_id"`
	Views       int       `db:"views"`
	Submissions int       `db:"submissions"`
	Conversions float64   `db:"conversions"`
	LastUpdated time.Time `db:"last_updated"`
}
