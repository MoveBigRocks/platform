package servicedomain

import (
	"fmt"
	"time"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/shared/util"
)

// Type aliases for shared domain types
type (
	CaseStatus             = shareddomain.CaseStatus
	CasePriority           = shareddomain.CasePriority
	CaseChannel            = shareddomain.CaseChannel
	CommunicationDirection = shareddomain.CommunicationDirection
)

// Re-export common shared-domain constants for service-domain consumers.
const (
	CaseStatusNew      = shareddomain.CaseStatusNew
	CaseStatusOpen     = shareddomain.CaseStatusOpen
	CaseStatusPending  = shareddomain.CaseStatusPending
	CaseStatusResolved = shareddomain.CaseStatusResolved
	CaseStatusClosed   = shareddomain.CaseStatusClosed
	CaseStatusSpam     = shareddomain.CaseStatusSpam
)

const (
	CasePriorityLow    = shareddomain.CasePriorityLow
	CasePriorityMedium = shareddomain.CasePriorityMedium
	CasePriorityHigh   = shareddomain.CasePriorityHigh
	CasePriorityUrgent = shareddomain.CasePriorityUrgent
)

const (
	CaseChannelEmail    = shareddomain.CaseChannelEmail
	CaseChannelWeb      = shareddomain.CaseChannelWeb
	CaseChannelAPI      = shareddomain.CaseChannelAPI
	CaseChannelPhone    = shareddomain.CaseChannelPhone
	CaseChannelChat     = shareddomain.CaseChannelChat
	CaseChannelInternal = shareddomain.CaseChannelInternal
)

// =============================================================================
// Composable Case Structs
// Breaking up the god object into focused, reusable components
// =============================================================================

// CaseIdentity contains the core identification fields
type CaseIdentity struct {
	ID          string
	WorkspaceID string
	HumanID     string // Format: prefix-yymm-random (e.g., ac-2512-a3e9ef)
}

// CaseContact contains contact/customer information
type CaseContact struct {
	ContactID    string
	ContactEmail string
	ContactName  string
}

// CaseAssignment contains assignment information
type CaseAssignment struct {
	TeamID       string
	AssignedToID string
}

// CaseSLA contains SLA tracking fields
type CaseSLA struct {
	ResponseDueAt         *time.Time
	ResolutionDueAt       *time.Time
	FirstResponseAt       *time.Time
	ResolvedAt            *time.Time
	ClosedAt              *time.Time
	ResponseTimeMinutes   int
	ResolutionTimeMinutes int
}

// CaseMetrics contains case metrics and counters
type CaseMetrics struct {
	ReopenCount  int
	MessageCount int
}

// CaseRelationships contains case hierarchy and relationships
type CaseRelationships struct {
	ParentCaseID   string
	ChildCaseIDs   []string
	RelatedCaseIDs []string
}

// CaseIssueTracking contains error monitoring integration fields
type CaseIssueTracking struct {
	LinkedIssueIDs       []string
	RootCauseIssueID     string
	IssueResolved        bool
	IssueResolvedAt      *time.Time
	ContactNotified      bool
	ContactNotifiedAt    *time.Time
	NotificationTemplate string
}

// CaseSourceInfo contains source/creation metadata
type CaseSourceInfo struct {
	Source       shareddomain.SourceType
	AutoCreated  bool
	IsSystemCase bool
}

// CaseTimestamps contains audit timestamps
type CaseTimestamps struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// =============================================================================
// Main Case Struct - Composed from smaller structs
// =============================================================================

// Case represents a support ticket/request with type-safe fields
type Case struct {
	// Embedded identity
	CaseIdentity

	// Basic info
	Subject     string
	Description string

	// Classification
	Status               CaseStatus
	Priority             CasePriority
	Channel              CaseChannel
	Category             string
	QueueID              string
	PrimaryCatalogNodeID string
	Tags                 []string

	// Embedded structs for organization
	CaseContact
	CaseAssignment
	CaseSLA
	CaseMetrics
	CaseRelationships
	CaseIssueTracking
	CaseSourceInfo
	CaseTimestamps

	OriginatingConversationID string

	// Type-safe custom fields
	CustomFields shareddomain.TypedCustomFields
}

// =============================================================================
// Constructors
// =============================================================================

// NewCase creates a new case with generated ID
func NewCase(workspaceID, subject, contactEmail string) *Case {
	now := time.Now()
	return &Case{
		CaseIdentity: CaseIdentity{
			WorkspaceID: workspaceID,
		},
		Subject: subject,
		CaseContact: CaseContact{
			ContactEmail: contactEmail,
		},
		Status:       CaseStatusNew,
		Priority:     CasePriorityMedium,
		Channel:      CaseChannelWeb,
		Tags:         []string{},
		CustomFields: shareddomain.NewTypedCustomFields(),
		CaseTimestamps: CaseTimestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

// NewCaseParams contains all optional fields for case creation
type NewCaseParams struct {
	WorkspaceID               string
	Subject                   string
	Description               string
	Status                    CaseStatus
	Priority                  CasePriority
	Channel                   CaseChannel
	Category                  string
	QueueID                   string
	PrimaryCatalogNodeID      string
	OriginatingConversationID string
	ContactID                 string
	ContactName               string
	ContactEmail              string
	TeamID                    string
	AssignedToID              string
	Tags                      []string
	CustomFields              shareddomain.TypedCustomFields
}

// NewCaseWithDefaults creates a case with business rule defaults
func NewCaseWithDefaults(params NewCaseParams) *Case {
	now := time.Now()

	// Apply defaults
	status := params.Status
	if status == "" {
		status = CaseStatusNew
	}

	priority := params.Priority
	if priority == "" {
		priority = CasePriorityMedium
	}

	channel := params.Channel
	if channel == "" {
		channel = CaseChannelAPI
	}

	tags := params.Tags
	if tags == nil {
		tags = []string{}
	}

	// Use provided custom fields or create empty
	customFields := params.CustomFields
	if customFields.Len() == 0 {
		customFields = shareddomain.NewTypedCustomFields()
	}

	return &Case{
		CaseIdentity: CaseIdentity{
			WorkspaceID: params.WorkspaceID,
		},
		Subject:              params.Subject,
		Description:          params.Description,
		Status:               status,
		Priority:             priority,
		Channel:              channel,
		Category:             params.Category,
		QueueID:              params.QueueID,
		PrimaryCatalogNodeID: params.PrimaryCatalogNodeID,
		Tags:                 tags,
		CaseContact: CaseContact{
			ContactID:    params.ContactID,
			ContactName:  params.ContactName,
			ContactEmail: params.ContactEmail,
		},
		CaseAssignment: CaseAssignment{
			TeamID:       params.TeamID,
			AssignedToID: params.AssignedToID,
		},
		OriginatingConversationID: params.OriginatingConversationID,
		CustomFields:              customFields,
		CaseTimestamps: CaseTimestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

// =============================================================================
// Type-Safe Custom Field Accessors
// =============================================================================

// GetCustomString retrieves a string custom field
func (c *Case) GetCustomString(field string) (string, bool) {
	return c.CustomFields.GetString(field)
}

// GetCustomInt retrieves an int custom field
func (c *Case) GetCustomInt(field string) (int64, bool) {
	return c.CustomFields.GetInt(field)
}

// GetCustomBool retrieves a bool custom field
func (c *Case) GetCustomBool(field string) (bool, bool) {
	return c.CustomFields.GetBool(field)
}

// SetCustomString sets a string custom field
func (c *Case) SetCustomString(field, value string) {
	c.CustomFields.SetString(field, value)
}

// SetCustomInt sets an int custom field
func (c *Case) SetCustomInt(field string, value int64) {
	c.CustomFields.SetInt(field, value)
}

// SetCustomBool sets a bool custom field
func (c *Case) SetCustomBool(field string, value bool) {
	c.CustomFields.SetBool(field, value)
}

// =============================================================================
// Business Logic Methods
// =============================================================================

// GenerateHumanID generates a human-readable case ID in format: prefix-yymm-random
// Example: ac-2512-a3e9ef (Acme, December 2025, random Base58)
func (c *Case) GenerateHumanID(prefix string) {
	c.HumanID = util.GenerateHumanID(prefix, 6)
}

// IsOverdue checks if the case is overdue based on SLA
func (c *Case) IsOverdue() bool {
	now := time.Now()
	if c.ResponseDueAt != nil && c.FirstResponseAt == nil && now.After(*c.ResponseDueAt) {
		return true
	}
	if c.ResolutionDueAt != nil && c.ResolvedAt == nil && now.After(*c.ResolutionDueAt) {
		return true
	}
	return false
}

// CanBeReopened checks if a case can be reopened
func (c *Case) CanBeReopened() bool {
	return c.Status == CaseStatusResolved || c.Status == CaseStatusClosed
}

// Validate validates the case fields
func (c *Case) Validate() error {
	if c.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if c.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if !isValidCaseStatus(c.Status) {
		return fmt.Errorf("invalid status: %s", c.Status)
	}
	if !isValidCasePriority(c.Priority) {
		return fmt.Errorf("invalid priority: %s", c.Priority)
	}
	if !isValidCaseChannel(c.Channel) {
		return fmt.Errorf("invalid channel: %s", c.Channel)
	}
	return nil
}

// LinkIssue links an issue to this case
func (c *Case) LinkIssue(issueID, projectID string) error {
	if issueID == "" {
		return fmt.Errorf("issue_id cannot be empty")
	}

	c.CustomFields.SetString("linked_issue_id", issueID)
	if projectID != "" {
		c.CustomFields.SetString("linked_project_id", projectID)
	}

	// Check for duplicates in LinkedIssueIDs
	for _, linkedID := range c.LinkedIssueIDs {
		if linkedID == issueID {
			return nil
		}
	}

	c.LinkedIssueIDs = append(c.LinkedIssueIDs, issueID)
	return nil
}

// UnlinkIssue removes an issue link from this case
func (c *Case) UnlinkIssue(issueID string) {
	c.CustomFields.Delete("linked_issue_id")
	c.CustomFields.Delete("linked_project_id")

	newLinkedIDs := make([]string, 0, len(c.LinkedIssueIDs))
	for _, linkedID := range c.LinkedIssueIDs {
		if linkedID != issueID {
			newLinkedIDs = append(newLinkedIDs, linkedID)
		}
	}
	c.LinkedIssueIDs = newLinkedIDs
}

// MarkResolved transitions the case to resolved status
func (c *Case) MarkResolved(resolvedAt time.Time) error {
	if c.Status == CaseStatusResolved || c.Status == CaseStatusClosed {
		return fmt.Errorf("case is already %s", c.Status)
	}

	c.Status = CaseStatusResolved
	c.ResolvedAt = &resolvedAt
	c.ClosedAt = &resolvedAt

	if !c.CreatedAt.IsZero() {
		c.ResolutionTimeMinutes = int(resolvedAt.Sub(c.CreatedAt).Minutes())
	}
	return nil
}

// MarkClosed transitions the case to closed status
func (c *Case) MarkClosed(closedAt time.Time) error {
	if c.Status == CaseStatusClosed {
		return fmt.Errorf("case is already closed")
	}
	if c.Status != CaseStatusResolved {
		return fmt.Errorf("case must be resolved before closing (current status: %s)", c.Status)
	}

	c.Status = CaseStatusClosed
	c.ClosedAt = &closedAt
	return nil
}

// Reopen reopens a resolved or closed case
func (c *Case) Reopen() error {
	if !c.CanBeReopened() {
		return fmt.Errorf("case cannot be reopened from status: %s", c.Status)
	}

	c.Status = CaseStatusOpen
	c.ReopenCount++
	c.ResolvedAt = nil
	c.ClosedAt = nil
	return nil
}

// SetStatus sets the case status with validation
func (c *Case) SetStatus(newStatus CaseStatus) error {
	if !isValidCaseStatus(newStatus) {
		return fmt.Errorf("invalid status: %s", newStatus)
	}

	switch newStatus {
	case CaseStatusClosed:
		if c.Status != CaseStatusResolved {
			return fmt.Errorf("can only close a resolved case")
		}
	case CaseStatusResolved:
		if c.Status == CaseStatusClosed {
			return fmt.Errorf("cannot resolve a closed case, reopen it first")
		}
	}

	c.Status = newStatus
	return nil
}

// NotifyContact marks that the contact has been notified
func (c *Case) NotifyContact(notifiedAt time.Time, template string) {
	c.ContactNotified = true
	c.ContactNotifiedAt = &notifiedAt
	c.NotificationTemplate = template
}

// MarkIssueResolved marks the linked issue as resolved
func (c *Case) MarkIssueResolved(resolvedAt time.Time) {
	c.IssueResolved = true
	c.IssueResolvedAt = &resolvedAt
}

// MarkAsAutoCreated marks a case as auto-created from error monitoring
func (c *Case) MarkAsAutoCreated(source string, rootCauseIssueID string) {
	c.Source = shareddomain.SourceType(source)
	c.AutoCreated = true
	c.RootCauseIssueID = rootCauseIssueID
}

// RecordFirstResponse records the first response time
func (c *Case) RecordFirstResponse(responseAt time.Time) {
	if c.FirstResponseAt != nil {
		return
	}
	c.FirstResponseAt = &responseAt

	if !c.CreatedAt.IsZero() {
		c.ResponseTimeMinutes = int(responseAt.Sub(c.CreatedAt).Minutes())
	}
}

// Assign assigns the case to a user and/or team
func (c *Case) Assign(userID, teamID string) error {
	if userID == "" && teamID == "" {
		return fmt.Errorf("must provide either user_id or team_id")
	}

	c.AssignedToID = userID
	c.TeamID = teamID

	if c.Status == CaseStatusNew {
		c.Status = CaseStatusOpen
	}
	return nil
}

// Unassign removes the case assignment
func (c *Case) Unassign() {
	c.AssignedToID = ""
}

// SetPriority changes the case priority with validation
func (c *Case) SetPriority(priority CasePriority) error {
	if !isValidCasePriority(priority) {
		return fmt.Errorf("invalid priority: %s", priority)
	}
	c.Priority = priority
	return nil
}

// AddTag adds a tag to the case if not already present
func (c *Case) AddTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	for _, t := range c.Tags {
		if t == tag {
			return nil
		}
	}
	c.Tags = append(c.Tags, tag)
	return nil
}

// RemoveTag removes a tag from the case.
// Returns an error if tag is empty.
func (c *Case) RemoveTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	newTags := make([]string, 0, len(c.Tags))
	for _, t := range c.Tags {
		if t != tag {
			newTags = append(newTags, t)
		}
	}
	c.Tags = newTags
	return nil
}

// HasTag checks if the case has a specific tag
func (c *Case) HasTag(tag string) bool {
	for _, t := range c.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// SetCategory sets the case category
func (c *Case) SetCategory(category string) {
	c.Category = category
}

// AutoClose closes a resolved case that has passed its grace period.
// Returns false if the case cannot be auto-closed (wrong status).
// This encapsulates the business rule: resolved cases auto-close after grace period.
func (c *Case) AutoClose() bool {
	// Business rule: only resolved cases can be auto-closed
	if c.Status != CaseStatusResolved {
		return false
	}

	c.Status = CaseStatusClosed
	_ = c.AddTag("auto-closed") // Ignore error - tag may already exist
	return true
}

// TransitionAfterAgentReply transitions the case status after an agent sends a reply.
// Business rule: when agent replies, case moves to pending (waiting for customer).
func (c *Case) TransitionAfterAgentReply() {
	if c.Status == CaseStatusNew || c.Status == CaseStatusOpen {
		c.Status = CaseStatusPending
	}
}

// IncrementMessageCount increments the message counter
func (c *Case) IncrementMessageCount() {
	c.MessageCount++
}

// =============================================================================
// Validation Helpers
// =============================================================================

func isValidCaseStatus(status CaseStatus) bool {
	switch status {
	case CaseStatusNew, CaseStatusOpen, CaseStatusPending,
		CaseStatusResolved, CaseStatusClosed, CaseStatusSpam:
		return true
	}
	return false
}

func isValidCasePriority(priority CasePriority) bool {
	switch priority {
	case CasePriorityLow, CasePriorityMedium, CasePriorityHigh, CasePriorityUrgent:
		return true
	}
	return false
}

func isValidCaseChannel(channel CaseChannel) bool {
	switch channel {
	case CaseChannelEmail, CaseChannelWeb, CaseChannelAPI,
		CaseChannelPhone, CaseChannelChat, CaseChannelInternal:
		return true
	}
	return false
}

// =============================================================================
// Communication - with type-safe enums
// =============================================================================

// Communication represents a message/interaction on a case
type Communication struct {
	ID          string
	CaseID      string
	WorkspaceID string

	// Type-safe communication type and direction
	Type      shareddomain.CommunicationType
	Direction shareddomain.Direction

	// Content
	Subject  string
	Body     string
	BodyHTML string

	// Sender info (one of FromUserID or FromAgentID is set)
	FromEmail   string
	FromName    string
	FromUserID  string
	FromAgentID string

	// Recipients
	ToEmails  []string
	CCEmails  []string
	BCCEmails []string

	// Status
	IsInternal bool
	IsRead     bool
	IsSpam     bool

	// Email specific
	MessageID  string
	InReplyTo  string
	References []string

	// Attachments
	AttachmentIDs []string

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewCommunication creates a new communication with type-safe enums
func NewCommunication(caseID, workspaceID string, commType shareddomain.CommunicationType, body string) *Communication {
	return &Communication{
		CaseID:      caseID,
		WorkspaceID: workspaceID,
		Type:        commType,
		Body:        body,
		Direction:   shareddomain.DirectionInternal,
		IsInternal:  true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// SetDirection sets the communication direction
func (c *Communication) SetDirection(dir shareddomain.Direction) {
	c.Direction = dir
	c.IsInternal = (dir == shareddomain.DirectionInternal)
	c.UpdatedAt = time.Now()
}

// IsInbound returns true if the communication is inbound
func (c *Communication) IsInbound() bool {
	return c.Direction == shareddomain.DirectionInbound
}

// IsOutbound returns true if the communication is outbound
func (c *Communication) IsOutbound() bool {
	return c.Direction == shareddomain.DirectionOutbound
}

// IsFromAgent returns true if the communication was created by an agent
func (c *Communication) IsFromAgent() bool {
	return c.FromAgentID != ""
}

// IsFromHuman returns true if the communication was created by a human
func (c *Communication) IsFromHuman() bool {
	return c.FromUserID != "" && c.FromAgentID == ""
}

// NewAgentCommunication creates a new communication from an agent
func NewAgentCommunication(caseID, workspaceID, agentID string, commType shareddomain.CommunicationType, body string) *Communication {
	return &Communication{
		CaseID:      caseID,
		WorkspaceID: workspaceID,
		Type:        commType,
		Body:        body,
		FromAgentID: agentID,
		Direction:   shareddomain.DirectionInternal,
		IsInternal:  true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
