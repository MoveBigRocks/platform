package servicedomain

import (
	"time"

	shared "github.com/movebigrocks/platform/internal/shared/domain"
)

// ThreadStatus represents the status of an email thread
type ThreadStatus string

const (
	ThreadStatusActive   ThreadStatus = "active"   // Thread is active
	ThreadStatusClosed   ThreadStatus = "closed"   // Thread is closed
	ThreadStatusMerged   ThreadStatus = "merged"   // Thread was merged into another
	ThreadStatusArchived ThreadStatus = "archived" // Thread is archived
	ThreadStatusSpam     ThreadStatus = "spam"     // Thread marked as spam
)

// ThreadType represents the type of email thread
type ThreadType string

const (
	ThreadTypeConversation ThreadType = "conversation" // Normal conversation
	ThreadTypeAutoReply    ThreadType = "auto_reply"   // Auto-reply thread
	ThreadTypeBounce       ThreadType = "bounce"       // Bounce message thread
	ThreadTypeNotification ThreadType = "notification" // Notification thread
	ThreadTypeMarketing    ThreadType = "marketing"    // Marketing email thread
)

// ThreadPriority represents the priority of an email thread
type ThreadPriority string

const (
	ThreadPriorityLow    ThreadPriority = "low"
	ThreadPriorityNormal ThreadPriority = "normal"
	ThreadPriorityHigh   ThreadPriority = "high"
	ThreadPriorityUrgent ThreadPriority = "urgent"
)

// EmailThread represents an email conversation thread
type EmailThread struct {
	ID          string
	WorkspaceID string

	// Thread identification
	ThreadKey  string   // Unique key for grouping emails
	Subject    string   // Thread subject (normalized)
	MessageIDs []string // Email Message-IDs in this thread

	// Thread metadata
	Type     ThreadType
	Status   ThreadStatus
	Priority ThreadPriority

	// Participants
	Participants []ThreadParticipant

	// Associated entities
	CaseID     string   // Associated case
	ContactIDs []string // Associated contacts

	// Email tracking
	EmailCount   int    // Total emails in thread
	UnreadCount  int    // Unread emails
	LastEmailID  string // ID of most recent email
	FirstEmailID string // ID of first email

	// Timing
	FirstEmailAt time.Time // When first email was received/sent
	LastEmailAt  time.Time // When last email was received/sent
	LastActivity time.Time // Last activity in thread

	// Thread analysis
	SentimentScore  float64 // Overall sentiment (-1 to 1)
	IsImportant     bool    // Marked as important
	HasAttachments  bool    // Thread contains attachments
	AttachmentCount int     // Total attachments

	// Thread relationships
	ParentThreadID string   // Parent thread if this is a sub-thread
	ChildThreadIDs []string // Child/sub-threads
	MergedFromIDs  []string // Threads merged into this one
	MergedIntoID   string   // Thread this was merged into

	// Auto-detection
	DetectedBy      string  // "system", "user", "rule"
	DetectionMethod string  // How thread was detected
	DetectionScore  float64 // Confidence score (0-1)

	// Thread tags and labels
	Tags   []string
	Labels []string

	// Security and spam
	IsSpam        bool
	SpamScore     float64
	IsQuarantined bool

	// Thread settings
	IsWatched  bool // User is watching this thread
	IsMuted    bool // Thread is muted
	IsArchived bool // Thread is archived

	// Metadata
	Notes        string
	CustomFields map[string]interface{}
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

// ThreadParticipant represents a participant in an email thread
type ThreadParticipant struct {
	Email       string
	Name        string
	ContactID   string    // Associated contact ID
	UserID      string    // Associated user ID if internal
	Role        string    // "sender", "recipient", "cc", "bcc"
	IsInternal  bool      // Internal to organization
	FirstSeenAt time.Time // When first seen in thread
	LastSeenAt  time.Time // When last seen in thread
	EmailCount  int       // Emails sent by this participant
}

// EmailThreadLink represents the relationship between an email and a thread
type EmailThreadLink struct {
	ID          string
	WorkspaceID string

	// Link details
	ThreadID  string
	EmailID   string // Could be InboundEmail or OutboundEmail
	EmailType string // "inbound" or "outbound"

	// Position in thread
	Position int  // Position in thread (0-based)
	IsFirst  bool // First email in thread
	IsLast   bool // Last email in thread

	// Email metadata snapshot
	Subject   string
	FromEmail string
	FromName  string
	ToEmails  []string
	SentAt    time.Time // When email was sent/received

	// Threading headers
	MessageID  string
	InReplyTo  string
	References []string

	// Link confidence
	Confidence float64 // How confident we are in this link (0-1)
	LinkReason string  // Why this email was linked

	// Status
	IsActive   bool // Link is active
	IsVerified bool // Link has been verified
	VerifiedAt *time.Time
	VerifiedBy string

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ThreadMerge represents a merge operation between email threads
type ThreadMerge struct {
	ID          string
	WorkspaceID string

	// Merge details
	TargetThreadID  string   // Thread to merge into
	SourceThreadIDs []string // Threads being merged
	MergeReason     string   // Reason for merge

	// Status
	Status      shared.ExecutionStatus
	RequestedAt time.Time
	ExecutedAt  *time.Time
	CompletedAt *time.Time

	// User context
	RequestedByID string
	ExecutedByID  string

	// Merge results
	EmailsMerged       int
	ParticipantsMerged int
	AttachmentsMerged  int

	// Rollback data
	CanRollback  bool
	RollbackData string // JSON serialized rollback info

	// Error handling
	ErrorMessage string

	// Metadata
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ThreadSplit represents a split operation for email threads
type ThreadSplit struct {
	ID          string
	WorkspaceID string

	// Split details
	SourceThreadID string   // Original thread
	NewThreadID    string   // New thread created
	SplitEmailIDs  []string // Emails moved to new thread
	SplitReason    string   // Reason for split

	// Status
	Status      shared.ExecutionStatus
	RequestedAt time.Time
	ExecutedAt  *time.Time
	CompletedAt *time.Time

	// User context
	RequestedByID string
	ExecutedByID  string

	// Split results
	EmailsMoved int

	// Rollback capability
	CanRollback  bool
	RollbackData string

	// Error handling
	ErrorMessage string

	// Metadata
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ThreadAnalytics represents analytics data for email threads
type ThreadAnalytics struct {
	ID          string
	WorkspaceID string
	ThreadID    string
	Date        time.Time // Daily analytics

	// Email metrics
	EmailsReceived int
	EmailsSent     int
	EmailsTotal    int

	// Response metrics
	ResponseTime      int // Average response time in minutes
	FirstResponseTime int // Time to first response in minutes

	// Participant metrics
	ParticipantCount     int
	InternalParticipants int
	ExternalParticipants int

	// Content metrics
	AverageEmailLength int // Average email length in characters
	AttachmentCount    int

	// Engagement metrics
	OpenCount  int // Email opens
	ClickCount int // Link clicks

	// Sentiment analysis
	SentimentScore float64 // Average sentiment
	PositiveEmails int
	NegativeEmails int
	NeutralEmails  int

	// Resolution metrics
	WasResolved    bool
	ResolutionTime int // Time to resolution in hours

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// EmailThreadFilters represents filters for email thread queries
type EmailThreadFilters struct {
	Status        string
	Type          string
	Priority      string
	MessageID     string
	Subject       string
	CaseID        string
	ContactID     string
	IsSpam        *bool
	IsArchived    *bool
	IsImportant   *bool
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Limit         int
	Offset        int
}

// AddEmail adds an email to the thread
func (et *EmailThread) AddEmail(emailID, messageID string, sentAt time.Time, hasAttachments bool, attachmentCount int) {
	et.MessageIDs = append(et.MessageIDs, messageID)
	et.EmailCount++
	et.UnreadCount++
	et.LastEmailID = emailID
	et.LastEmailAt = sentAt
	et.LastActivity = time.Now()

	if hasAttachments {
		et.HasAttachments = true
		et.AttachmentCount += attachmentCount
	}

	et.UpdatedAt = time.Now()
}

// AddParticipant adds a participant to the thread
func (et *EmailThread) AddParticipant(participant ThreadParticipant) {
	// Check if participant already exists
	for i, existing := range et.Participants {
		if existing.Email == participant.Email {
			// Update existing participant
			et.Participants[i].LastSeenAt = participant.LastSeenAt
			et.Participants[i].EmailCount++
			et.UpdatedAt = time.Now()
			return
		}
	}

	// Add new participant
	et.Participants = append(et.Participants, participant)
	et.UpdatedAt = time.Now()
}

// MarkAsRead marks emails in the thread as read
func (et *EmailThread) MarkAsRead(count int) {
	if count > et.UnreadCount {
		count = et.UnreadCount
	}
	et.UnreadCount -= count
	et.UpdatedAt = time.Now()
}

// SetImportant marks the thread as important or not important
func (et *EmailThread) SetImportant(important bool) {
	et.IsImportant = important
	et.UpdatedAt = time.Now()
}

// Archive archives the thread
func (et *EmailThread) Archive() {
	et.IsArchived = true
	et.Status = ThreadStatusArchived
	et.UpdatedAt = time.Now()
}

// Close closes the thread
func (et *EmailThread) Close() {
	et.Status = ThreadStatusClosed
	et.UpdatedAt = time.Now()
}

// Merge marks the thread as merged into another thread
func (et *EmailThread) Merge(targetThreadID string) {
	et.Status = ThreadStatusMerged
	et.MergedIntoID = targetThreadID
	et.UpdatedAt = time.Now()
}

// AddChildThread adds a child thread
func (et *EmailThread) AddChildThread(childThreadID string) {
	for _, existingID := range et.ChildThreadIDs {
		if existingID == childThreadID {
			return // Already exists
		}
	}
	et.ChildThreadIDs = append(et.ChildThreadIDs, childThreadID)
	et.UpdatedAt = time.Now()
}

// UpdateSentiment updates the sentiment score
func (et *EmailThread) UpdateSentiment(sentimentScore float64) {
	et.SentimentScore = sentimentScore
	et.UpdatedAt = time.Now()
}

// Watch enables watching for this thread
func (et *EmailThread) Watch() {
	et.IsWatched = true
	et.UpdatedAt = time.Now()
}

// Mute mutes the thread
func (et *EmailThread) Mute() {
	et.IsMuted = true
	et.UpdatedAt = time.Now()
}

// MarkAsSpam marks the thread as spam
func (et *EmailThread) MarkAsSpam(spamScore float64) {
	et.IsSpam = true
	et.SpamScore = spamScore
	et.Status = ThreadStatusSpam
	et.UpdatedAt = time.Now()
}

// UpdateLastActivity updates the last activity timestamp
func (et *EmailThread) UpdateLastActivity() {
	et.LastActivity = time.Now()
	et.UpdatedAt = time.Now()
}
