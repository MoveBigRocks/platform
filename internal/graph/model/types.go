// Package model defines all GraphQL types used by the API.
// These types are used by graph-gophers/graphql-go resolvers.
package model

import (
	"time"

	"github.com/graph-gophers/graphql-go"
)

// =============================================================================
// Scalar Types
// =============================================================================

// ID is an alias for graphql.ID for use in GraphQL model types
type ID = graphql.ID

// =============================================================================
// Core Types
// =============================================================================

// User represents a human user
type User struct {
	ID        ID
	Email     string
	Name      string
	AvatarURL *string
}

// Agent represents an AI agent
type Agent struct {
	ID           ID
	WorkspaceID  ID
	Name         string
	Description  *string
	OwnerID      ID
	Status       string
	StatusReason *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CreatedByID  ID
}

// AgentToken represents an API token for an agent
type AgentToken struct {
	ID          ID
	AgentID     ID
	TokenPrefix string
	Name        string
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	LastUsedAt  *time.Time
	LastUsedIP  *string
	UseCount    int32
	CreatedAt   time.Time
	CreatedByID ID
}

// AgentTokenResult includes the plaintext token (shown only once)
type AgentTokenResult struct {
	Token          *AgentToken
	PlaintextToken string
}

// WorkspaceMembership represents a principal's membership in a workspace
type WorkspaceMembership struct {
	ID            ID
	WorkspaceID   ID
	PrincipalID   ID
	PrincipalType string
	Role          string
	Permissions   []string
	Constraints   MembershipConstraints
	GrantedAt     time.Time
	ExpiresAt     *time.Time
	RevokedAt     *time.Time
}

type MembershipConstraints struct {
	RateLimitPerMinute      *int32
	RateLimitPerHour        *int32
	AllowedIPs              []string
	AllowedProjectIDs       []ID
	AllowedTeamIDs          []ID
	AllowDelegatedRouting   bool
	DelegatedRoutingTeamIDs []ID
	ActiveHoursStart        *string
	ActiveHoursEnd          *string
	ActiveTimezone          *string
	ActiveDays              []int32
}

// Workspace represents a tenant workspace
type Workspace struct {
	ID        ID
	Name      string
	ShortCode string
}

// =============================================================================
// Observability Types
// =============================================================================

// Project represents an error monitoring project
type Project struct {
	ID          ID
	WorkspaceID ID
	Name        string
	Slug        string
	Platform    *string
	DSN         string
	CreatedAt   time.Time
}

// Application represents an application within a project
type Application struct {
	ID          ID
	ProjectID   ID
	Name        string
	Environment *string
}

// GitRepo represents a git repository linked to an application
type GitRepo struct {
	ID            ID
	ApplicationID ID
	RepoURL       string
	DefaultBranch string
	PathPrefix    *string
	CreatedAt     time.Time
}

// Issue represents an error issue
type Issue struct {
	ID         ID
	ProjectID  ID
	Title      string
	Culprit    *string
	Status     string
	Level      string
	FirstSeen  time.Time
	LastSeen   time.Time
	EventCount int32
	UserCount  int32
	CreatedAt  time.Time
}

// ErrorEvent represents a single error event
type ErrorEvent struct {
	ID             ID
	IssueID        ID
	EventID        string
	Message        *string
	Level          string
	Platform       *string
	Timestamp      time.Time
	Environment    *string
	Release        *string
	ExceptionType  *string
	ExceptionValue *string
	Stacktrace     *Stacktrace
	Tags           []Tag
	Contexts       map[string]interface{}
	Extra          map[string]interface{}
}

// Stacktrace represents a stack trace
type Stacktrace struct {
	Frames []StackFrame
}

// StackFrame represents a single frame in a stack trace
type StackFrame struct {
	Filename *string
	Function *string
	Module   *string
	Lineno   *int32
	Colno    *int32
	AbsPath  *string
	Context  []ContextLine
	InApp    *bool
}

// ContextLine represents a line of source code context
type ContextLine struct {
	LineNo int32
	Line   string
}

// Tag represents a key-value tag
type Tag struct {
	Key   string
	Value string
}

// =============================================================================
// Service Types
// =============================================================================

// Case represents a service case
type Case struct {
	ID          ID
	CaseID      string // Human-readable ID
	WorkspaceID ID
	Subject     string
	Status      string
	Priority    string
	ContactID   *ID
	AssigneeID  *ID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ResolvedAt  *time.Time
}

// Attachment represents a durable file linked to operational work.
type Attachment struct {
	ID          ID
	WorkspaceID ID
	CaseID      *ID
	EmailID     *ID
	Filename    string
	ContentType string
	Size        int32
	Status      string
	Description *string
	Source      string
	CreatedAt   time.Time
}

// Contact represents a customer contact
type Contact struct {
	ID    ID
	Email string
	Name  *string
}

// Communication represents a message in a case
type Communication struct {
	ID            ID
	CaseID        ID
	Direction     string
	Channel       string
	Subject       *string
	Body          string
	BodyHTML      *string
	FromEmail     *string
	FromName      *string
	FromUserID    *ID
	FromAgentID   *ID
	AttachmentIDs []ID
	IsInternal    bool
	CreatedAt     time.Time
}

// =============================================================================
// Pagination Types (Relay-style)
// =============================================================================

// PageInfo contains pagination information
type PageInfo struct {
	HasNextPage     bool
	HasPreviousPage bool
	StartCursor     *string
	EndCursor       *string
}

// CaseConnection represents a paginated list of cases
type CaseConnection struct {
	Edges      []CaseEdge
	PageInfo   PageInfo
	TotalCount int32
}

// CaseEdge represents an edge in a case connection
type CaseEdge struct {
	Node   *Case
	Cursor string
}

// IssueConnection represents a paginated list of issues
type IssueConnection struct {
	Edges      []IssueEdge
	PageInfo   PageInfo
	TotalCount int32
}

// IssueEdge represents an edge in an issue connection
type IssueEdge struct {
	Node   *Issue
	Cursor string
}

// ErrorEventConnection represents a paginated list of error events
type ErrorEventConnection struct {
	Edges      []ErrorEventEdge
	PageInfo   PageInfo
	TotalCount int32
}

// ErrorEventEdge represents an edge in an error event connection
type ErrorEventEdge struct {
	Node   *ErrorEvent
	Cursor string
}

// =============================================================================
// Admin Types
// =============================================================================

// AdminUser represents a user in admin context
type AdminUser struct {
	ID             ID
	Email          string
	Name           string
	AvatarURL      *string
	InstanceRole   *string
	IsActive       bool
	EmailVerified  bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	WorkspaceCount int32
}

// AdminUserConnection represents a paginated list of admin users
type AdminUserConnection struct {
	Edges      []AdminUserEdge
	PageInfo   PageInfo
	TotalCount int32
}

// AdminUserEdge represents an edge in an admin user connection
type AdminUserEdge struct {
	Node   *AdminUser
	Cursor string
}

// AdminUserWithWorkspaces includes user with their workspace memberships
type AdminUserWithWorkspaces struct {
	User       *AdminUser
	Workspaces []WorkspaceRoleInfo
}

// WorkspaceRoleInfo represents a user's role in a workspace
type WorkspaceRoleInfo struct {
	Workspace *Workspace
	Role      string
	JoinedAt  string
}

// AdminWorkspace represents a workspace in admin context
type AdminWorkspace struct {
	ID           ID
	Name         string
	ShortCode    string
	Description  *string
	MemberCount  int32
	CaseCount    int32
	ProjectCount int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AdminWorkspaceConnection represents a paginated list of admin workspaces
type AdminWorkspaceConnection struct {
	Edges      []AdminWorkspaceEdge
	PageInfo   PageInfo
	TotalCount int32
}

// AdminWorkspaceEdge represents an edge in an admin workspace connection
type AdminWorkspaceEdge struct {
	Node   *AdminWorkspace
	Cursor string
}

// AdminCase represents a case in admin context
type AdminCase struct {
	ID            ID
	CaseID        string
	WorkspaceID   ID
	WorkspaceName *string
	Subject       string
	Status        string
	Priority      string
	ContactEmail  *string
	ContactName   *string
	AssigneeID    *ID
	AssigneeName  *string
	Tags          []string
	Category      *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ResolvedAt    *time.Time
}

// AdminCaseConnection represents a paginated list of admin cases
type AdminCaseConnection struct {
	Edges      []AdminCaseEdge
	PageInfo   PageInfo
	TotalCount int32
}

// AdminCaseEdge represents an edge in an admin case connection
type AdminCaseEdge struct {
	Node   *AdminCase
	Cursor string
}

// AdminCaseDetail includes case with related data
type AdminCaseDetail struct {
	Case           *AdminCase
	Communications []Communication
	LinkedIssues   []Issue
	AvailableUsers []User
}

// AdminIssue represents an issue in admin context
type AdminIssue struct {
	ID            ID
	ProjectID     ID
	ProjectName   *string
	WorkspaceID   *ID
	WorkspaceName *string
	Title         string
	ShortID       string
	Culprit       *string
	Status        string
	Level         string
	FirstSeen     time.Time
	LastSeen      time.Time
	EventCount    int32
	UserCount     int32
}

// AdminIssueConnection represents a paginated list of admin issues
type AdminIssueConnection struct {
	Edges      []AdminIssueEdge
	PageInfo   PageInfo
	TotalCount int32
}

// AdminIssueEdge represents an edge in an admin issue connection
type AdminIssueEdge struct {
	Node   *AdminIssue
	Cursor string
}

// AdminIssueDetail includes issue with related data
type AdminIssueDetail struct {
	Issue        *AdminIssue
	Events       []ErrorEvent
	RelatedCases []AdminCase
}

// AdminProject represents a project in admin context
type AdminProject struct {
	ID            ID
	WorkspaceID   ID
	WorkspaceName *string
	Name          string
	Slug          string
	Platform      *string
	Environment   *string
	Status        *string
	EventCount    int32
	DSN           string
	CreatedAt     time.Time
}

// AdminProjectConnection represents a paginated list of admin projects
type AdminProjectConnection struct {
	Edges      []AdminProjectEdge
	PageInfo   PageInfo
	TotalCount int32
}

// AdminProjectEdge represents an edge in an admin project connection
type AdminProjectEdge struct {
	Node   *AdminProject
	Cursor string
}

// AdminRule represents an automation rule in admin context
type AdminRule struct {
	ID                   ID
	WorkspaceID          ID
	WorkspaceName        *string
	Title                string
	Description          *string
	IsActive             bool
	Priority             int32
	MaxExecutionsPerHour int32
	MaxExecutionsPerDay  int32
	Conditions           map[string]interface{}
	Actions              map[string]interface{}
	ExecutionCount       int32
	LastExecutedAt       *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CreatedByID          ID
}

// AdminRuleConnection represents a paginated list of admin rules
type AdminRuleConnection struct {
	Edges      []AdminRuleEdge
	PageInfo   PageInfo
	TotalCount int32
}

// AdminRuleEdge represents an edge in an admin rule connection
type AdminRuleEdge struct {
	Node   *AdminRule
	Cursor string
}

// AdminForm represents a form in admin context
type AdminForm struct {
	ID                 ID
	WorkspaceID        ID
	WorkspaceName      *string
	Name               string
	Slug               string
	Description        *string
	Status             string
	CryptoID           string
	IsPublic           bool
	RequiresCaptcha    bool
	CollectEmail       bool
	AutoCreateCase     bool
	AutoCasePriority   *string
	AutoCaseType       *string
	AutoAssignTeamID   *string
	AutoTags           []string
	NotifyOnSubmission bool
	NotificationEmails []string
	SubmissionMessage  *string
	RedirectURL        *string
	SchemaData         map[string]interface{}
	SubmissionCount    int32
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CreatedByID        ID
}

// AdminFormConnection represents a paginated list of admin forms
type AdminFormConnection struct {
	Edges      []AdminFormEdge
	PageInfo   PageInfo
	TotalCount int32
}

// AdminFormEdge represents an edge in an admin form connection
type AdminFormEdge struct {
	Node   *AdminForm
	Cursor string
}
