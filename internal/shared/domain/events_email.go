package shareddomain

import (
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

// Email Processing Events

// EmailReceived is published when an inbound email is received
type EmailReceived struct {
	eventbus.BaseEvent

	// Event payload
	EmailID     string
	WorkspaceID string
	From        string
	To          string
	Subject     string
	ThreadID    string
	InReplyTo   string
	ReceivedAt  time.Time
}

// EmailThreadMatched is published when an email is matched to a case thread
type EmailThreadMatched struct {
	eventbus.BaseEvent

	// Event payload
	EmailID     string
	ThreadID    string
	CaseID      string
	WorkspaceID string
	Confidence  float64
	MatchedAt   time.Time
}

// EmailParsed is published when email content is extracted and parsed
type EmailParsed struct {
	eventbus.BaseEvent

	// Event payload
	EmailID     string
	WorkspaceID string
	From        string
	To          string
	Subject     string
	TextContent string
	HTMLContent string
	MessageID   string
	InReplyTo   string
	References  []string
	Headers     map[string]string
	ParsedAt    time.Time
}

// EmailSpamDetected is published when spam detection completes
type EmailSpamDetected struct {
	eventbus.BaseEvent

	// Event payload
	EmailID     string
	WorkspaceID string
	SpamScore   float64
	IsSpam      bool
	Reasons     []string
	DetectedAt  time.Time
}

// EmailThreadCreated is published when a new email thread is created
type EmailThreadCreated struct {
	eventbus.BaseEvent

	// Event payload
	ThreadID       string
	ThreadKey      string
	WorkspaceID    string
	Subject        string
	FirstEmailID   string
	FirstMessageID string
	CreatedAt      time.Time
}

// ContactCreatedFromEmail is published when a new contact is created from an email
type ContactCreatedFromEmail struct {
	eventbus.BaseEvent

	// Event payload
	ContactID    string
	WorkspaceID  string
	EmailID      string
	Email        string
	Name         string
	Organization string
	CreatedAt    time.Time
}

// EmailProcessingCompleted is published when email processing completes successfully
type EmailProcessingCompleted struct {
	eventbus.BaseEvent

	// Event payload
	EmailID            string
	WorkspaceID        string
	ThreadID           string
	ContactID          string
	CaseID             string
	WasSpam            bool
	CaseCreated        bool
	ContactCreated     bool
	ProcessingSteps    []string
	ProcessingDuration int64
	CompletedAt        time.Time
}

// EmailProcessingFailed is published when email processing fails
type EmailProcessingFailed struct {
	eventbus.BaseEvent

	// Event payload
	EmailID     string
	WorkspaceID string
	Error       string
	Stage       string
	RawFrom     string
	FailedAt    time.Time
}

// Validation Methods

// Validate validates the ContactCreatedFromEmail event
func (e ContactCreatedFromEmail) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("contact_id", e.ContactID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("email_id", e.EmailID); err != nil {
		return err
	}
	if err := validateEmailRequired("email", e.Email); err != nil {
		return err
	}
	if e.CreatedAt.IsZero() {
		return ErrRequiredField("created_at")
	}
	return nil
}
