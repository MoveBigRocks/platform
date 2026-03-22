package shareddomain

import (
	"time"
)

// AssignmentType represents the type of assignment
type AssignmentType string

const (
	AssignmentTypeUser       AssignmentType = "user"       // Assigned to a specific user
	AssignmentTypeTeam       AssignmentType = "team"       // Assigned to a team
	AssignmentTypeUnassigned AssignmentType = "unassigned" // Unassigned/removed assignment
	AssignmentTypeAutomatic  AssignmentType = "automatic"  // Auto-assigned by system
)

// AssignmentReason represents the reason for assignment
type AssignmentReason string

const (
	AssignmentReasonManual        AssignmentReason = "manual"         // Manual assignment by user
	AssignmentReasonRule          AssignmentReason = "rule"           // Assigned by automation rule
	AssignmentReasonWorkflow      AssignmentReason = "workflow"       // Assigned by workflow
	AssignmentReasonEscalation    AssignmentReason = "escalation"     // Escalation assignment
	AssignmentReasonLoadBalancing AssignmentReason = "load_balancing" // Load balancing assignment
	AssignmentReasonSkills        AssignmentReason = "skills"         // Skills-based assignment
	AssignmentReasonAvailability  AssignmentReason = "availability"   // Availability-based assignment
	AssignmentReasonRoundRobin    AssignmentReason = "round_robin"    // Round-robin assignment
	AssignmentReasonReassignment  AssignmentReason = "reassignment"   // Reassignment
	AssignmentReasonTransfer      AssignmentReason = "transfer"       // Transfer between users/teams
)

// AssignmentStatus represents the status of an assignment
type AssignmentStatus string

const (
	AssignmentStatusActive      AssignmentStatus = "active"      // Currently active assignment
	AssignmentStatusCompleted   AssignmentStatus = "completed"   // Assignment completed
	AssignmentStatusCanceled    AssignmentStatus = "canceled"    // Assignment canceled
	AssignmentStatusTransferred AssignmentStatus = "transferred" // Assignment transferred
	AssignmentStatusEscalated   AssignmentStatus = "escalated"   // Assignment escalated
)

// CaseAssignmentHistory represents the assignment audit trail for cases
type CaseAssignmentHistory struct {
	ID          string
	WorkspaceID string

	// Assignment details
	CaseID         string
	AssignmentType AssignmentType

	// Assignment target
	AssignedToUserID string // User ID if assigned to user
	AssignedToTeamID string // Team ID if assigned to team
	AssignedUserName string // Snapshot of user name
	AssignedTeamName string // Snapshot of team name

	// Previous assignment (for tracking changes)
	PreviousUserID   string
	PreviousTeamID   string
	PreviousUserName string
	PreviousTeamName string

	// Assignment metadata
	Reason AssignmentReason
	Status AssignmentStatus

	// Timing
	AssignedAt  time.Time
	AcceptedAt  *time.Time
	CompletedAt *time.Time
	Duration    *int // Duration in seconds

	// Assignment context
	AssignedByID   string // User who made the assignment
	AssignedByName string // Name of assignor
	AssignedByType string // "user", "system", "rule", "workflow"

	// Rule/workflow context
	RuleID     string // If assigned by rule
	WorkflowID string // If assigned by workflow
	MacroID    string // If assigned by macro

	// Assignment priority and urgency
	Priority    int        // Assignment priority (1-10)
	IsUrgent    bool       // Urgent assignment
	SLADeadline *time.Time // SLA deadline for this assignment

	// Workload context
	WorkloadBefore int // Assignee workload before this assignment
	WorkloadAfter  int // Assignee workload after this assignment

	// Skills and qualifications
	RequiredSkills  []string // Skills required for this assignment
	MatchedSkills   []string // Skills the assignee has
	SkillMatchScore float64  // 0-1 score for skill matching

	// Availability context
	AssigneeAvailable     bool   // Whether assignee was available
	AssigneeTimezone      string // Assignee timezone
	AssignmentDuringHours bool   // Assigned during working hours

	// Performance tracking
	ResponseTime         *int     // Time to first response (seconds)
	ResolutionTime       *int     // Time to resolution (seconds)
	CustomerSatisfaction *float64 // Customer satisfaction score

	// Escalation tracking
	WasEscalated      bool       // Whether this assignment was escalated
	EscalatedAt       *time.Time // When escalated
	EscalatedToUserID string     // Escalated to user
	EscalatedToTeamID string     // Escalated to team
	EscalationReason  string     // Reason for escalation

	// Transfer tracking
	WasTransferred      bool       // Whether this assignment was transferred
	TransferredAt       *time.Time // When transferred
	TransferredToUserID string     // Transferred to user
	TransferredToTeamID string     // Transferred to team
	TransferReason      string     // Reason for transfer

	// Acceptance tracking
	WasAccepted     bool       // Whether assignment was accepted
	AcceptedByID    string     // Who accepted (if different from assignee)
	RejectedAt      *time.Time // When rejected
	RejectionReason string     // Reason for rejection

	// Auto-assignment details
	AutoAssignmentConfig  Metadata              // Config used for auto-assignment
	AssignmentScore       float64               // Score from assignment algorithm
	AlternativeCandidates []AssignmentCandidate // Other candidates considered

	// Notifications
	NotificationSent     bool       // Whether assignee was notified
	NotificationSentAt   *time.Time // When notification was sent
	NotificationMethod   string     // "email", "sms", "push", "webhook"
	NotificationViewed   bool       // Whether notification was viewed
	NotificationViewedAt *time.Time // When notification was viewed

	// Case context (snapshot at time of assignment)
	CaseStatus       string    // Case status when assigned
	CasePriority     string    // Case priority when assigned
	CaseSubject      string    // Case subject (for context)
	CaseCreatedAt    time.Time // When case was created
	CaseCustomFields Metadata  // Case custom fields snapshot

	// Metadata
	Comments     string   // Comments about the assignment
	Tags         []string // Assignment tags
	CustomFields Metadata // Custom assignment fields
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AssignmentCandidate represents a candidate considered for assignment
type AssignmentCandidate struct {
	UserID            string
	TeamID            string
	Name              string
	Score             float64    // Assignment score (0-1)
	Reasons           []string   // Reasons for/against selection
	Workload          int        // Current workload
	Availability      bool       // Currently available
	SkillMatch        float64    // Skill match score (0-1)
	LastAssigned      *time.Time // When last assigned a case
	PerformanceScore  float64    // Historical performance score
	NotSelected       bool       // Whether this candidate was not selected
	NotSelectedReason string     // Why not selected
}

// AssignmentMetrics represents metrics about assignments
type AssignmentMetrics struct {
	ID          string
	WorkspaceID string
	Date        time.Time // Daily metrics

	// Assignment volume
	TotalAssignments  int
	UserAssignments   int
	TeamAssignments   int
	AutoAssignments   int
	ManualAssignments int

	// Assignment outcomes
	CompletedAssignments   int
	TransferredAssignments int
	EscalatedAssignments   int
	RejectedAssignments    int

	// Timing metrics
	AverageResponseTime   int // seconds
	AverageResolutionTime int // seconds
	AverageAssignmentTime int // seconds from creation to assignment

	// Performance metrics
	AssignmentAccuracy   float64 // % of assignments that didn't require transfer
	FirstTimeResolution  float64 // % resolved without reassignment
	CustomerSatisfaction float64 // Average satisfaction score

	// Workload metrics
	AverageWorkloadBefore float64
	AverageWorkloadAfter  float64
	MaxWorkloadReached    int

	// SLA metrics
	SLABreaches       int
	SLAComplianceRate float64

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AssignmentRule represents rules for automatic assignment
type AssignmentRule struct {
	ID          string
	WorkspaceID string

	// Rule details
	Name        string
	Description string
	IsActive    bool
	Priority    int // Higher priority rules run first

	// Conditions
	Conditions []AssignmentRuleCondition // When to apply this rule

	// Assignment strategy
	Strategy string // "round_robin", "load_balancing", "skills", "availability"

	// Target configuration
	TargetUsers []string // Users to consider for assignment
	TargetTeams []string // Teams to consider for assignment

	// Assignment criteria
	RequiredSkills      []string // Skills required
	PreferredSkills     []string // Preferred but not required skills
	MaxWorkload         int      // Max workload for assignee
	RequireAvailability bool     // Only assign to available users

	// Business hours
	BusinessHoursOnly bool                   // Only assign during business hours
	Timezone          string                 // Timezone for business hours
	BusinessHours     map[string][]TimeRange // Business hours by day

	// Escalation
	AutoEscalate      bool     // Auto-escalate if not accepted
	EscalationDelay   int      // Minutes before escalation
	EscalationTargets []string // Users/teams to escalate to

	// Fallback
	FallbackStrategy string   // Strategy if no one available
	FallbackTargets  []string // Fallback assignees

	// Usage tracking
	TimesUsed   int
	LastUsedAt  *time.Time
	SuccessRate float64 // % of successful assignments

	// Metadata
	CreatedByID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// TimeRange represents a time range during the day
type TimeRange struct {
	Start string // "09:00"
	End   string // "17:00"
}

// AssignmentRuleCondition represents a condition for assignment rules
type AssignmentRuleCondition struct {
	Field    string // Field to check
	Operator string // Comparison operator
	Value    Value  // Value to compare
	Type     string // "case", "contact", "user"
}

// AssignToUser assigns the case to a specific user
func (cah *CaseAssignmentHistory) AssignToUser(userID, userName, assignedByID, assignedByName string) {
	cah.AssignmentType = AssignmentTypeUser
	cah.AssignedToUserID = userID
	cah.AssignedUserName = userName
	cah.AssignedByID = assignedByID
	cah.AssignedByName = assignedByName
	cah.AssignedByType = "user"
	cah.Status = AssignmentStatusActive
	cah.UpdatedAt = time.Now()
}

// AssignToTeam assigns the case to a team
func (cah *CaseAssignmentHistory) AssignToTeam(teamID, teamName, assignedByID, assignedByName string) {
	cah.AssignmentType = AssignmentTypeTeam
	cah.AssignedToTeamID = teamID
	cah.AssignedTeamName = teamName
	cah.AssignedByID = assignedByID
	cah.AssignedByName = assignedByName
	cah.AssignedByType = "user"
	cah.Status = AssignmentStatusActive
	cah.UpdatedAt = time.Now()
}

// Accept marks the assignment as accepted
func (cah *CaseAssignmentHistory) Accept(acceptedByID string) {
	cah.WasAccepted = true
	cah.AcceptedByID = acceptedByID
	now := time.Now()
	cah.AcceptedAt = &now
	cah.UpdatedAt = now
}

// Complete marks the assignment as completed
func (cah *CaseAssignmentHistory) Complete() {
	cah.Status = AssignmentStatusCompleted
	now := time.Now()
	cah.CompletedAt = &now

	// Calculate duration
	if cah.AcceptedAt != nil {
		duration := int(now.Sub(*cah.AcceptedAt).Seconds())
		cah.Duration = &duration
	} else {
		duration := int(now.Sub(cah.AssignedAt).Seconds())
		cah.Duration = &duration
	}

	cah.UpdatedAt = now
}

// Transfer marks the assignment as transferred
func (cah *CaseAssignmentHistory) Transfer(toUserID, toTeamID, reason string) {
	cah.Status = AssignmentStatusTransferred
	cah.WasTransferred = true
	cah.TransferredToUserID = toUserID
	cah.TransferredToTeamID = toTeamID
	cah.TransferReason = reason
	now := time.Now()
	cah.TransferredAt = &now
	cah.UpdatedAt = now
}

// Escalate marks the assignment as escalated
func (cah *CaseAssignmentHistory) Escalate(toUserID, toTeamID, reason string) {
	cah.Status = AssignmentStatusEscalated
	cah.WasEscalated = true
	cah.EscalatedToUserID = toUserID
	cah.EscalatedToTeamID = toTeamID
	cah.EscalationReason = reason
	now := time.Now()
	cah.EscalatedAt = &now
	cah.UpdatedAt = now
}

// Reject marks the assignment as rejected
func (cah *CaseAssignmentHistory) Reject(reason string) {
	cah.Status = AssignmentStatusCanceled
	cah.RejectionReason = reason
	now := time.Now()
	cah.RejectedAt = &now
	cah.UpdatedAt = now
}

// SetWorkloadContext sets workload before and after assignment
func (cah *CaseAssignmentHistory) SetWorkloadContext(before, after int) {
	cah.WorkloadBefore = before
	cah.WorkloadAfter = after
	cah.UpdatedAt = time.Now()
}

// SetSkillContext sets skill matching context
func (cah *CaseAssignmentHistory) SetSkillContext(required, matched []string, score float64) {
	cah.RequiredSkills = required
	cah.MatchedSkills = matched
	cah.SkillMatchScore = score
	cah.UpdatedAt = time.Now()
}

// SetPerformanceMetrics sets performance metrics for the assignment
func (cah *CaseAssignmentHistory) SetPerformanceMetrics(responseTime, resolutionTime *int, satisfaction *float64) {
	cah.ResponseTime = responseTime
	cah.ResolutionTime = resolutionTime
	cah.CustomerSatisfaction = satisfaction
	cah.UpdatedAt = time.Now()
}

// SetCaseContext sets case context snapshot
func (cah *CaseAssignmentHistory) SetCaseContext(status, priority, subject string, createdAt time.Time, customFields Metadata) {
	cah.CaseStatus = status
	cah.CasePriority = priority
	cah.CaseSubject = subject
	cah.CaseCreatedAt = createdAt
	cah.CaseCustomFields = customFields
	cah.UpdatedAt = time.Now()
}

// SetSLADeadline sets the SLA deadline for this assignment
func (cah *CaseAssignmentHistory) SetSLADeadline(deadline time.Time) {
	cah.SLADeadline = &deadline
	cah.UpdatedAt = time.Now()
}

// MarkUrgent marks the assignment as urgent
func (cah *CaseAssignmentHistory) MarkUrgent() {
	cah.IsUrgent = true
	cah.UpdatedAt = time.Now()
}

// SetNotificationSent marks notification as sent
func (cah *CaseAssignmentHistory) SetNotificationSent(method string) {
	cah.NotificationSent = true
	cah.NotificationMethod = method
	now := time.Now()
	cah.NotificationSentAt = &now
	cah.UpdatedAt = now
}

// MarkNotificationViewed marks notification as viewed
func (cah *CaseAssignmentHistory) MarkNotificationViewed() {
	cah.NotificationViewed = true
	now := time.Now()
	cah.NotificationViewedAt = &now
	cah.UpdatedAt = now
}

// AddAlternativeCandidate adds an alternative candidate that was considered
func (cah *CaseAssignmentHistory) AddAlternativeCandidate(candidate AssignmentCandidate) {
	cah.AlternativeCandidates = append(cah.AlternativeCandidates, candidate)
	cah.UpdatedAt = time.Now()
}
