package models

import (
	"time"
)

// OutboundEmail represents an outgoing email
type OutboundEmail struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Email details
	FromEmail    string `db:"from_email"`
	FromName     string `db:"from_name"`
	ToEmails     string `db:"to_emails"`
	CCEmails     string `db:"cc_emails"`
	BCCEmails    string `db:"bcc_emails"`
	ReplyToEmail string `db:"reply_to_email"`

	// Content
	Subject     string `db:"subject"`
	HTMLContent string `db:"html_content"`
	TextContent string `db:"text_content"`

	// Template info
	TemplateID   *string `db:"template_id"`
	TemplateData string  `db:"template_data"`

	// Sending configuration
	Provider         string `db:"provider"`
	ProviderSettings string `db:"provider_settings"`

	// Status tracking
	Status       string     `db:"status"`
	ScheduledFor *time.Time `db:"scheduled_for"`
	SentAt       *time.Time `db:"sent_at"`
	DeliveredAt  *time.Time `db:"delivered_at"`

	// Provider response
	ProviderMessageID string `db:"provider_message_id"`
	ProviderResponse  string `db:"provider_response"`

	// Error handling
	ErrorMessage string     `db:"error_message"`
	RetryCount   int        `db:"retry_count"`
	MaxRetries   int        `db:"max_retries"`
	NextRetryAt  *time.Time `db:"next_retry_at"`

	// Tracking
	OpenedAt    *time.Time `db:"opened_at"`
	OpenCount   int        `db:"open_count"`
	ClickCount  int        `db:"click_count"`
	LastClickAt *time.Time `db:"last_click_at"`

	// Context
	CaseID          *string `db:"case_id"`
	ContactID       *string `db:"contact_id"`
	CommunicationID *string `db:"communication_id"`
	UserID          *string `db:"user_id"`

	// Categories and tags
	Category string `db:"category"`
	Tags     string `db:"tags"`

	// Attachments
	AttachmentIDs string `db:"attachment_ids"`

	// Metadata
	CreatedByID *string    `db:"created_by_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

func (OutboundEmail) TableName() string {
	return "outbound_emails"
}

// InboundEmail represents an incoming email
type InboundEmail struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Email headers
	MessageID       string `db:"message_id"`
	InReplyTo       string `db:"in_reply_to"`
	EmailReferences string `db:"email_references"`

	// Sender info
	FromEmail string `db:"from_email"`
	FromName  string `db:"from_name"`
	ToEmails  string `db:"to_emails"`
	CCEmails  string `db:"cc_emails"`
	BCCEmails string `db:"bcc_emails"`

	// Content
	Subject     string `db:"subject"`
	HTMLContent string `db:"html_content"`
	TextContent string `db:"text_content"`

	// Processing status
	ProcessingStatus string     `db:"processing_status"`
	ProcessedAt      *time.Time `db:"processed_at"`
	ProcessingError  string     `db:"processing_error"`

	// Spam detection
	SpamScore   float64 `db:"spam_score"`
	SpamReasons string  `db:"spam_reasons"`
	IsSpam      bool    `db:"is_spam"`

	// Auto-processing results
	CaseID          *string `db:"case_id"`
	ContactID       *string `db:"contact_id"`
	CommunicationID *string `db:"communication_id"`

	// Thread detection
	ThreadID         *string `db:"thread_id"`
	IsThreadStart    bool    `db:"is_thread_start"`
	PreviousEmailIDs string  `db:"previous_email_ids"`

	// Loop detection
	IsLoop    bool    `db:"is_loop"`
	LoopScore float64 `db:"loop_score"`

	// Bounce detection
	IsBounce          bool   `db:"is_bounce"`
	BounceType        string `db:"bounce_type"`
	OriginalMessageID string `db:"original_message_id"`

	// Auto-response detection
	IsAutoResponse   bool   `db:"is_auto_response"`
	AutoResponseType string `db:"auto_response_type"`

	// Status flags
	IsRead bool   `db:"is_read"`
	Tags   string `db:"tags"`

	// Attachments
	AttachmentIDs       string `db:"attachment_ids"`
	AttachmentCount     int    `db:"attachment_count"`
	TotalAttachmentSize int64  `db:"total_attachment_size"`

	// Raw email data
	RawContent string `db:"raw_content"`
	Headers    string `db:"headers"`

	// Metadata
	ReceivedAt time.Time `db:"received_at"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

func (InboundEmail) TableName() string {
	return "inbound_emails"
}

// EmailTemplate represents an email template
type EmailTemplate struct {
	ID          string    `db:"id"`
	WorkspaceID string    `db:"workspace_id"`
	Key         string    `db:"key"`
	Name        string    `db:"name"`
	Subject     string    `db:"subject"`
	BodyHTML    string    `db:"body_html"`
	BodyText    string    `db:"body_text"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (EmailTemplate) TableName() string {
	return "email_templates"
}

// EmailThread represents an email conversation thread
type EmailThread struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Thread identification
	ThreadKey string `db:"thread_key"`
	Subject   string `db:"subject"`

	// Thread metadata
	Type     string `db:"type"`
	Status   string `db:"status"`
	Priority string `db:"priority"`

	// Participants (JSON)
	Participants string `db:"participants"`

	// Associated entities
	CaseID     *string `db:"case_id"`
	ContactIDs string  `db:"contact_ids"`

	// Email tracking
	EmailCount   int     `db:"email_count"`
	UnreadCount  int     `db:"unread_count"`
	LastEmailID  *string `db:"last_email_id"`
	FirstEmailID *string `db:"first_email_id"`
	MessageIDs   string  `db:"message_ids"`

	// Timing
	FirstEmailAt *time.Time `db:"first_email_at"`
	LastEmailAt  *time.Time `db:"last_email_at"`
	LastActivity *time.Time `db:"last_activity"`

	// Thread analysis
	SentimentScore  float64 `db:"sentiment_score"`
	IsImportant     bool    `db:"is_important"`
	HasAttachments  bool    `db:"has_attachments"`
	AttachmentCount int     `db:"attachment_count"`

	// Thread relationships
	ParentThreadID *string `db:"parent_thread_id"`
	ChildThreadIDs string  `db:"child_thread_ids"`
	MergedFromIDs  string  `db:"merged_from_ids"`
	MergedIntoID   *string `db:"merged_into_id"`

	// Auto-detection
	DetectedBy      string  `db:"detected_by"`
	DetectionMethod string  `db:"detection_method"`
	DetectionScore  float64 `db:"detection_score"`

	// Tags and labels
	Tags   string `db:"tags"`
	Labels string `db:"labels"`

	// Security and spam
	IsSpam        bool    `db:"is_spam"`
	SpamScore     float64 `db:"spam_score"`
	IsQuarantined bool    `db:"is_quarantined"`

	// Thread settings
	IsWatched  bool `db:"is_watched"`
	IsMuted    bool `db:"is_muted"`
	IsArchived bool `db:"is_archived"`

	// Metadata
	Notes        string `db:"notes"`
	CustomFields string `db:"custom_fields"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (EmailThread) TableName() string {
	return "email_threads"
}

// EmailThreadLink links emails to threads
type EmailThreadLink struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Link details
	ThreadID  string `db:"thread_id"`
	EmailID   string `db:"email_id"`
	EmailType string `db:"email_type"`

	// Position
	Position int  `db:"position"`
	IsFirst  bool `db:"is_first"`
	IsLast   bool `db:"is_last"`

	// Email metadata snapshot
	Subject   string     `db:"subject"`
	FromEmail string     `db:"from_email"`
	FromName  string     `db:"from_name"`
	ToEmails  string     `db:"to_emails"`
	SentAt    *time.Time `db:"sent_at"`

	CreatedAt time.Time `db:"created_at"`
}

// ThreadMerge records thread merge operations
type ThreadMerge struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Merge details
	SourceThreadID string `db:"source_thread_id"`
	TargetThreadID string `db:"target_thread_id"`
	Status         string `db:"status"`

	// Merge context
	MergedByID   string `db:"merged_by_id"`
	MergedByName string `db:"merged_by_name"`
	MergeReason  string `db:"merge_reason"`

	// Merge data
	EmailsMerged    int    `db:"emails_merged"`
	ConflictsFound  int    `db:"conflicts_found"`
	ConflictDetails string `db:"conflict_details"`

	// Timing
	MergedAt   *time.Time `db:"merged_at"`
	RevertedAt *time.Time `db:"reverted_at"`
	RevertedBy string     `db:"reverted_by"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ThreadSplit records thread split operations
type ThreadSplit struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Split details
	SourceThreadID string `db:"source_thread_id"`
	NewThreadID    string `db:"new_thread_id"`
	Status         string `db:"status"`

	// Split context
	SplitByID   string `db:"split_by_id"`
	SplitByName string `db:"split_by_name"`
	SplitReason string `db:"split_reason"`

	// Split data
	SplitAtEmailID string `db:"split_at_email_id"`
	EmailsMoved    int    `db:"emails_moved"`

	// Timing
	SplitAt    *time.Time `db:"split_at"`
	RevertedAt *time.Time `db:"reverted_at"`
	RevertedBy string     `db:"reverted_by"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ThreadAnalytics tracks thread metrics
type ThreadAnalytics struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	ThreadID    string `db:"thread_id"`
	Date        string `db:"date"`

	// Volume metrics
	MessageCount  int `db:"message_count"`
	InboundCount  int `db:"inbound_count"`
	OutboundCount int `db:"outbound_count"`
	InternalCount int `db:"internal_count"`

	// Timing metrics
	ResponseTimeAvg int `db:"response_time_avg"`
	ResponseTimeMin int `db:"response_time_min"`
	ResponseTimeMax int `db:"response_time_max"`

	// Engagement
	OpenCount    int `db:"open_count"`
	ClickCount   int `db:"click_count"`
	ReplyCount   int `db:"reply_count"`
	ForwardCount int `db:"forward_count"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// EmailStats tracks workspace email statistics
type EmailStats struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	Date        string `db:"date"`

	// Volume
	SentCount     int `db:"sent_count"`
	ReceivedCount int `db:"received_count"`
	BouncedCount  int `db:"bounced_count"`
	FailedCount   int `db:"failed_count"`
	PendingCount  int `db:"pending_count"`

	// Engagement
	OpenedCount  int `db:"opened_count"`
	ClickedCount int `db:"clicked_count"`
	RepliedCount int `db:"replied_count"`

	// Threading
	ThreadsCreated int `db:"threads_created"`
	ThreadsMerged  int `db:"threads_merged"`
	ThreadsSplit   int `db:"threads_split"`

	// Quality
	SpamCount       int `db:"spam_count"`
	QuarantineCount int `db:"quarantine_count"`
	BlacklistHits   int `db:"blacklist_hits"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// EmailBlacklist blocks email addresses or domains
type EmailBlacklist struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Block target
	Email   string `db:"email"`
	Domain  string `db:"domain"`
	Pattern string `db:"pattern"`

	// Block details
	Type     string `db:"type"`
	Reason   string `db:"reason"`
	IsActive bool   `db:"is_active"`

	// Block scope
	BlockInbound  bool `db:"block_inbound"`
	BlockOutbound bool `db:"block_outbound"`

	// Temporary
	ExpiresAt *time.Time `db:"expires_at"`

	// Stats
	BlockCount    int        `db:"block_count"`
	LastBlockedAt *time.Time `db:"last_blocked_at"`

	// Metadata
	CreatedByID string     `db:"created_by_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

func (EmailBlacklist) TableName() string {
	return "email_blacklists"
}

// QuarantinedMessage holds emails pending review
type QuarantinedMessage struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Email reference
	EmailID   string `db:"email_id"`
	EmailType string `db:"email_type"`
	MessageID string `db:"message_id"`

	// Quarantine reason
	Reason       string  `db:"reason"`
	ReasonDetail string  `db:"reason_detail"`
	RiskScore    float64 `db:"risk_score"`

	// Status
	Status     string     `db:"status"`
	ReviewedAt *time.Time `db:"reviewed_at"`
	ReviewedBy string     `db:"reviewed_by"`

	// Decision
	ApprovedAt *time.Time `db:"approved_at"`
	ApprovedBy string     `db:"approved_by"`
	RejectedAt *time.Time `db:"rejected_at"`
	RejectedBy string     `db:"rejected_by"`

	// Raw content
	RawHeaders  string `db:"raw_headers"`
	RawContent  string `db:"raw_content"`
	ContentSize int64  `db:"content_size"`

	// Expiration
	ExpiresAt *time.Time `db:"expires_at"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
