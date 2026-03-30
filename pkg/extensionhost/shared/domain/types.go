package shareddomain

import "time"

// Shared Types - used across multiple domain event types

// ============================================================================
// Case Domain Types (used in events and case domain)
// ============================================================================

// CaseStatus represents the lifecycle status of a case
type CaseStatus string

const (
	CaseStatusNew      CaseStatus = "new"
	CaseStatusOpen     CaseStatus = "open"
	CaseStatusPending  CaseStatus = "pending"
	CaseStatusResolved CaseStatus = "resolved"
	CaseStatusClosed   CaseStatus = "closed"
	CaseStatusSpam     CaseStatus = "spam"
)

// CasePriority represents the urgency level of a case
type CasePriority string

const (
	CasePriorityLow    CasePriority = "low"
	CasePriorityMedium CasePriority = "medium"
	CasePriorityHigh   CasePriority = "high"
	CasePriorityUrgent CasePriority = "urgent"
)

// CaseChannel represents the channel through which a case was created
type CaseChannel string

const (
	CaseChannelEmail    CaseChannel = "email"
	CaseChannelWeb      CaseChannel = "web"
	CaseChannelAPI      CaseChannel = "api"
	CaseChannelPhone    CaseChannel = "phone"
	CaseChannelChat     CaseChannel = "chat"
	CaseChannelInternal CaseChannel = "internal"
)

// CommunicationDirection represents the direction of a communication
type CommunicationDirection string

const (
	CommunicationDirectionInbound  CommunicationDirection = "inbound"
	CommunicationDirectionOutbound CommunicationDirection = "outbound"
)

// DateRange represents a time range for queries and reports
type DateRange struct {
	Type      DateRangeType
	StartDate *time.Time
	EndDate   *time.Time
}

// DateRangeType represents the type of date range
type DateRangeType string

const (
	DateRangeTypeAbsolute DateRangeType = "absolute"
	DateRangeTypeRelative DateRangeType = "relative"
)

// ExecutionStatus represents the status of an execution
type ExecutionStatus string

const (
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCanceled  ExecutionStatus = "canceled"
)

// ActionResultStatus represents the status of an action result
type ActionResultStatus string

const (
	ActionResultStatusSuccess ActionResultStatus = "success"
	ActionResultStatusFailed  ActionResultStatus = "failed"
	ActionResultStatusSkipped ActionResultStatus = "skipped"
)

// ActionResult represents the result of executing a single action
type ActionResult struct {
	ActionID     string
	Status       ActionResultStatus
	StartedAt    time.Time
	CompletedAt  *time.Time
	ResultData   Metadata
	ErrorMessage string
}
