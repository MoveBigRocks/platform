package servicedomain

import (
	"fmt"
	"net/mail"
	"time"
)

// OutboundEmail represents an outgoing email
type OutboundEmail struct {
	ID          string
	WorkspaceID string

	// Email details
	FromEmail    string
	FromName     string
	ToEmails     []string
	CCEmails     []string
	BCCEmails    []string
	ReplyToEmail string

	// Content
	Subject     string
	HTMLContent string
	TextContent string

	// Template info
	TemplateID   string
	TemplateData map[string]interface{}

	// Sending configuration
	Provider         EmailProvider
	ProviderSettings map[string]interface{}

	// Status tracking
	Status       EmailStatus
	ScheduledFor *time.Time
	SentAt       *time.Time
	DeliveredAt  *time.Time

	// Provider response
	ProviderMessageID string
	ProviderResponse  string

	// Error handling
	ErrorMessage string
	RetryCount   int
	MaxRetries   int
	NextRetryAt  *time.Time

	// Tracking
	OpenedAt    *time.Time
	OpenCount   int
	ClickCount  int
	LastClickAt *time.Time

	// Context
	CaseID          string
	ContactID       string
	CommunicationID string
	UserID          string

	// Categories and tags
	Category string
	Tags     []string

	// Attachments
	AttachmentIDs []string

	// Metadata
	CreatedByID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewOutboundEmail creates a new outbound email
func NewOutboundEmail(workspaceID, fromEmail, subject, content string) *OutboundEmail {
	return &OutboundEmail{
		WorkspaceID:      workspaceID,
		FromEmail:        fromEmail,
		Subject:          subject,
		TextContent:      content,
		Provider:         EmailProviderSendGrid,
		Status:           EmailStatusPending,
		MaxRetries:       3,
		ProviderSettings: make(map[string]interface{}),
		TemplateData:     make(map[string]interface{}),
		ToEmails:         []string{},
		CCEmails:         []string{},
		BCCEmails:        []string{},
		Tags:             []string{},
		AttachmentIDs:    []string{},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// MarkOpened marks an outbound email as opened
func (oe *OutboundEmail) MarkOpened() {
	now := time.Now()
	if oe.OpenedAt == nil {
		oe.OpenedAt = &now
	}
	oe.OpenCount++
	oe.UpdatedAt = now
}

// MarkClicked marks an outbound email as clicked
func (oe *OutboundEmail) MarkClicked() {
	now := time.Now()
	oe.LastClickAt = &now
	oe.ClickCount++
	oe.UpdatedAt = now
}

// Validate validates an outbound email's business rules
// DOMAIN BUSINESS RULES: All email validation rules defined here
func (oe *OutboundEmail) Validate() error {
	// Required fields
	if oe.FromEmail == "" {
		return fmt.Errorf("from email is required")
	}

	if len(oe.ToEmails) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	if oe.Subject == "" {
		return fmt.Errorf("subject is required")
	}

	if oe.HTMLContent == "" && oe.TextContent == "" {
		return fmt.Errorf("email content is required")
	}

	// Validate email address formats
	if _, err := mail.ParseAddress(oe.FromEmail); err != nil {
		return fmt.Errorf("invalid from email: %w", err)
	}

	for _, to := range oe.ToEmails {
		if _, err := mail.ParseAddress(to); err != nil {
			return fmt.Errorf("invalid recipient email %s: %w", to, err)
		}
	}

	// Validate CC emails if present
	for _, cc := range oe.CCEmails {
		if _, err := mail.ParseAddress(cc); err != nil {
			return fmt.Errorf("invalid CC email %s: %w", cc, err)
		}
	}

	// Validate BCC emails if present
	for _, bcc := range oe.BCCEmails {
		if _, err := mail.ParseAddress(bcc); err != nil {
			return fmt.Errorf("invalid BCC email %s: %w", bcc, err)
		}
	}

	return nil
}
