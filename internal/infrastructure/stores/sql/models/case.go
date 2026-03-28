package models

import (
	"time"
)

// Case represents a support case
type Case struct {
	ID                        string  `db:"id"`
	WorkspaceID               string  `db:"workspace_id"`
	HumanID                   string  `db:"human_id"`
	Subject                   string  `db:"subject"`
	Description               string  `db:"description"`
	Status                    string  `db:"status"`
	Priority                  string  `db:"priority"`
	Channel                   string  `db:"channel"`
	Category                  string  `db:"category"`
	QueueID                   *string `db:"queue_id"`
	ContactID                 *string `db:"contact_id"`
	PrimaryCatalogNodeID      *string `db:"primary_catalog_node_id"`
	OriginatingConversationID *string `db:"originating_conversation_session_id"`
	ContactEmail              string  `db:"contact_email"`
	ContactName               string  `db:"contact_name"`
	AssignedToID              *string `db:"assigned_to_id"`
	TeamID                    *string `db:"team_id"`
	Source                    string  `db:"source"`
	SourceID                  string  `db:"source_id"`
	SourceLink                string  `db:"source_link"`
	Tags                      string  `db:"tags"`
	Resolution                string  `db:"resolution"`
	ResolutionNote            string  `db:"resolution_note"`
	// SLA Fields
	ResponseDueAt         *time.Time `db:"response_due_at"`
	ResolutionDueAt       *time.Time `db:"resolution_due_at"`
	FirstResponseAt       *time.Time `db:"first_response_at"`
	ResolvedAt            *time.Time `db:"resolved_at"`
	ClosedAt              *time.Time `db:"closed_at"`
	ResponseTimeMinutes   int        `db:"response_time_minutes"`
	ResolutionTimeMinutes int        `db:"resolution_time_minutes"`
	// Metrics
	ReopenCount  int `db:"reopen_count"`
	MessageCount int `db:"message_count"`
	// Issue Tracking
	LinkedIssueIDs       string     `db:"linked_issue_ids"`
	RootCauseIssueID     *string    `db:"root_cause_issue_id"`
	IssueResolved        bool       `db:"issue_resolved"`
	IssueResolvedAt      *time.Time `db:"issue_resolved_at"`
	ContactNotified      bool       `db:"contact_notified"`
	ContactNotifiedAt    *time.Time `db:"contact_notified_at"`
	NotificationTemplate string     `db:"notification_template"`
	// Source Info
	AutoCreated  bool `db:"auto_created"`
	IsSystemCase bool `db:"is_system_case"`

	// Custom Fields (JSON)
	CustomFields string `db:"custom_fields"`

	CreatedBy *string    `db:"created_by"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (Case) TableName() string {
	return "cases"
}

// Communication represents a message within a case
type Communication struct {
	ID              string    `db:"id"`
	CaseID          string    `db:"case_id"`
	WorkspaceID     string    `db:"workspace_id"`
	Type            string    `db:"type"`
	Direction       string    `db:"direction"`
	Subject         string    `db:"subject"`
	Body            string    `db:"body"`
	BodyHTML        string    `db:"body_html"`
	FromEmail       string    `db:"from_email"`
	FromName        string    `db:"from_name"`
	FromUserID      *string   `db:"from_user_id"`
	FromAgentID     *string   `db:"from_agent_id"`
	ToEmails        string    `db:"to_emails"`
	CCEmails        string    `db:"cc_emails"`
	BCCEmails       string    `db:"bcc_emails"`
	MessageID       string    `db:"message_id"`
	InReplyTo       string    `db:"in_reply_to"`
	EmailReferences string    `db:"email_references"`
	AttachmentIDs   string    `db:"attachment_ids"`
	IsInternal      bool      `db:"is_internal"`
	IsRead          bool      `db:"is_read"`
	IsSpam          bool      `db:"is_spam"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

func (Communication) TableName() string {
	return "communications"
}

// CaseAssignmentHistory tracks case assignment changes
type CaseAssignmentHistory struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	CaseID      string `db:"case_id"`

	// Assignment target
	AssignmentType   string  `db:"assignment_type"`
	AssignedToUserID *string `db:"assigned_to_user_id"`
	AssignedToTeamID *string `db:"assigned_to_team_id"`
	AssignedUserName string  `db:"assigned_user_name"`
	AssignedTeamName string  `db:"assigned_team_name"`

	// Previous assignment
	PreviousUserID   *string `db:"previous_user_id"`
	PreviousTeamID   *string `db:"previous_team_id"`
	PreviousUserName string  `db:"previous_user_name"`
	PreviousTeamName string  `db:"previous_team_name"`

	// Assignment metadata
	Reason string `db:"reason"`
	Status string `db:"status"`

	// Timing
	AssignedAt  time.Time  `db:"assigned_at"`
	AcceptedAt  *time.Time `db:"accepted_at"`
	CompletedAt *time.Time `db:"completed_at"`
	Duration    *int       `db:"duration"`

	// Assignment context
	AssignedByID   *string `db:"assigned_by_id"`
	AssignedByName string  `db:"assigned_by_name"`
	AssignedByType string  `db:"assigned_by_type"`

	// Rule/workflow context
	RuleID     *string `db:"rule_id"`
	WorkflowID *string `db:"workflow_id"`

	// Priority
	Priority    int        `db:"priority"`
	IsUrgent    bool       `db:"is_urgent"`
	SLADeadline *time.Time `db:"sla_deadline"`

	// Workload
	WorkloadBefore int `db:"workload_before"`
	WorkloadAfter  int `db:"workload_after"`

	// Skills (JSON arrays)
	RequiredSkills  string  `db:"required_skills"`
	MatchedSkills   string  `db:"matched_skills"`
	SkillMatchScore float64 `db:"skill_match_score"`

	// Availability
	AssigneeAvailable     bool   `db:"assignee_available"`
	AssigneeTimezone      string `db:"assignee_timezone"`
	AssignmentDuringHours bool   `db:"assignment_during_hours"`

	// Performance
	ResponseTime         *int     `db:"response_time"`
	ResolutionTime       *int     `db:"resolution_time"`
	CustomerSatisfaction *float64 `db:"customer_satisfaction"`

	// Escalation
	WasEscalated      bool       `db:"was_escalated"`
	EscalatedAt       *time.Time `db:"escalated_at"`
	EscalatedToUserID *string    `db:"escalated_to_user_id"`
	EscalatedToTeamID *string    `db:"escalated_to_team_id"`
	EscalationReason  string     `db:"escalation_reason"`

	// Transfer
	WasTransferred      bool       `db:"was_transferred"`
	TransferredAt       *time.Time `db:"transferred_at"`
	TransferredToUserID *string    `db:"transferred_to_user_id"`
	TransferredToTeamID *string    `db:"transferred_to_team_id"`
	TransferReason      string     `db:"transfer_reason"`

	// Acceptance
	WasAccepted     bool       `db:"was_accepted"`
	AcceptedByID    *string    `db:"accepted_by_id"`
	RejectedAt      *time.Time `db:"rejected_at"`
	RejectionReason string     `db:"rejection_reason"`

	// Auto-assignment
	AutoAssignmentConfig  string  `db:"auto_assignment_config"`
	AssignmentScore       float64 `db:"assignment_score"`
	AlternativeCandidates string  `db:"alternative_candidates"`

	// Notifications
	NotificationSent     bool       `db:"notification_sent"`
	NotificationSentAt   *time.Time `db:"notification_sent_at"`
	NotificationMethod   string     `db:"notification_method"`
	NotificationViewed   bool       `db:"notification_viewed"`
	NotificationViewedAt *time.Time `db:"notification_viewed_at"`

	// Case context snapshot
	CaseStatus       string    `db:"case_status"`
	CasePriority     string    `db:"case_priority"`
	CaseSubject      string    `db:"case_subject"`
	CaseCreatedAt    time.Time `db:"case_created_at"`
	CaseCustomFields string    `db:"case_custom_fields"`

	// Metadata
	Comments     string    `db:"comments"`
	Tags         string    `db:"tags"`
	CustomFields string    `db:"custom_fields"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

func (CaseAssignmentHistory) TableName() string {
	return "case_assignment_history"
}

// Attachment stores file attachment metadata
type Attachment struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// File info
	Filename     string `db:"filename"`
	OriginalName string `db:"original_name"`
	ContentType  string `db:"content_type"`
	Size         int64  `db:"size"`
	Checksum     string `db:"checksum"`

	// Storage
	StorageKey    string `db:"storage_key"`
	StorageType   string `db:"storage_type"`
	StorageBucket string `db:"storage_bucket"`

	// Context
	CaseID                *string `db:"case_id"`
	CommunicationID       *string `db:"communication_id"`
	EmailID               *string `db:"email_id"`
	ConversationSessionID *string `db:"conversation_session_id"`
	ConversationMessageID *string `db:"conversation_message_id"`
	FormSubmissionID      *string `db:"form_submission_id"`

	// Security
	IsScanned   bool       `db:"is_scanned"`
	ScanResult  string     `db:"scan_result"`
	ScannedAt   *time.Time `db:"scanned_at"`
	ScanDetails string     `db:"scan_details"`

	// Access control
	IsPublic      bool    `db:"is_public"`
	AllowDownload bool    `db:"allow_download"`
	AccessToken   *string `db:"access_token"`

	// Metadata
	Description string `db:"description"`
	Tags        string `db:"tags"`
	Metadata    string `db:"metadata"`

	// Upload context
	UploadedByID   *string `db:"uploaded_by_id"`
	UploadedByType string  `db:"uploaded_by_type"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (Attachment) TableName() string {
	return "attachments"
}
