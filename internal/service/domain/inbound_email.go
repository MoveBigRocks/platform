package servicedomain

import (
	"time"

	"github.com/movebigrocks/platform/pkg/id"
)

// InboundEmail represents an incoming email
type InboundEmail struct {
	ID          string
	WorkspaceID string

	// Email headers
	MessageID  string
	InReplyTo  string
	References []string

	// Sender info
	FromEmail string
	FromName  string
	ToEmails  []string
	CCEmails  []string
	BCCEmails []string

	// Content
	Subject     string
	HTMLContent string
	TextContent string

	// Processing status
	ProcessingStatus EmailProcessingStatus // "pending", "processed", "failed", "spam"
	ProcessedAt      *time.Time
	ProcessingError  string

	// Spam detection
	SpamScore   float64
	SpamReasons []string
	IsSpam      bool

	// Auto-processing results
	CaseID          string // Auto-created or matched case
	ContactID       string // Auto-created or matched contact
	CommunicationID string // Created communication

	// Thread detection
	ThreadID         string
	IsThreadStart    bool
	PreviousEmailIDs []string

	// Loop detection
	IsLoop    bool
	LoopScore float64

	// Bounce detection
	IsBounce          bool
	BounceType        string // "hard", "soft", "complaint"
	OriginalMessageID string

	// Auto-response detection
	IsAutoResponse   bool
	AutoResponseType string // "out_of_office", "auto_reply"

	// Status flags
	IsRead bool
	Tags   []string

	// Attachments
	AttachmentIDs       []string
	AttachmentCount     int
	TotalAttachmentSize int64 // bytes

	// Raw email data
	RawContent string
	Headers    map[string]string

	// Metadata
	ReceivedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewInboundEmail creates a new inbound email
func NewInboundEmail(workspaceID, messageID, fromEmail, subject, content string) *InboundEmail {
	return &InboundEmail{
		ID:               id.New(),
		WorkspaceID:      workspaceID,
		MessageID:        messageID,
		FromEmail:        fromEmail,
		Subject:          subject,
		TextContent:      content,
		ProcessingStatus: EmailProcessingStatusPending,
		SpamReasons:      []string{},
		ToEmails:         []string{},
		CCEmails:         []string{},
		BCCEmails:        []string{},
		References:       []string{},
		PreviousEmailIDs: []string{},
		AttachmentIDs:    []string{},
		Headers:          make(map[string]string),
		ReceivedAt:       time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// MarkUpdated updates the UpdatedAt timestamp for InboundEmail
func (e *InboundEmail) MarkUpdated() {
	e.UpdatedAt = time.Now()
}
