package model

import (
	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
)

// =============================================================================
// Filter Input Types
// =============================================================================

// CaseFilterInput filters for case queries
type CaseFilterInput struct {
	Status     *[]string
	Priority   *[]string
	QueueID    *string
	AssigneeID *string
	ContactID  *string
	Search     *string
	First      *int32
	After      *string
}

// IssueFilterInput filters for issue queries
type IssueFilterInput struct {
	Status    *[]string
	Level     *[]string
	ProjectID *string
	Search    *string
	First     *int32
	After     *string
}

// KnowledgeResourceFilterInput filters for knowledge resource queries.
type KnowledgeResourceFilterInput struct {
	TeamID       *string
	Kind         *string
	Status       *string
	Surface      *string
	ReviewStatus *string
	Search       *string
	Limit        *int32
	Offset       *int32
}

// FormSubmissionFilterInput filters for form submission queries.
type FormSubmissionFilterInput struct {
	FormSpecID            *string
	ConversationSessionID *string
	CaseID                *string
	ContactID             *string
	Status                *string
	Limit                 *int32
	Offset                *int32
}

// ConversationSessionFilterInput filters for conversation queries.
type ConversationSessionFilterInput struct {
	Status               *string
	Channel              *string
	PrimaryCatalogNodeID *string
	PrimaryContactID     *string
	LinkedCaseID         *string
	Limit                *int32
	Offset               *int32
}

// AdminUserFilterInput filters for admin user queries
type AdminUserFilterInput struct {
	Search       *string
	IsActive     *bool
	InstanceRole *string
	First        *int32
	After        *string
}

// AdminWorkspaceFilterInput filters for admin workspace queries
type AdminWorkspaceFilterInput struct {
	Search *string
	First  *int32
	After  *string
}

// AdminCaseFilterInput filters for admin case queries
type AdminCaseFilterInput struct {
	WorkspaceID *string
	Status      *[]string
	Priority    *[]string
	AssigneeID  *string
	Search      *string
	First       *int32
	After       *string
}

// AdminIssueFilterInput filters for admin issue queries
type AdminIssueFilterInput struct {
	WorkspaceID *string
	ProjectID   *string
	Status      *[]string
	Level       *[]string
	Search      *string
	First       *int32
	After       *string
}

// AdminProjectFilterInput filters for admin project queries
type AdminProjectFilterInput struct {
	WorkspaceID *string
	Search      *string
	First       *int32
	After       *string
}

// AdminRuleFilterInput filters for admin rule queries
type AdminRuleFilterInput struct {
	WorkspaceID *string
	IsActive    *bool
	Search      *string
	First       *int32
	After       *string
}

// AdminFormFilterInput filters for admin form queries
type AdminFormFilterInput struct {
	WorkspaceID *string
	Status      *string
	IsPublic    *bool
	Search      *string
	First       *int32
	After       *string
}

// =============================================================================
// Mutation Input Types
// =============================================================================

// AddCommunicationInput input for adding a communication
type AddCommunicationInput struct {
	CaseID     string
	Body       string
	BodyHTML   *string
	IsInternal *bool
}

// CreateQueueInput input for creating a queue.
type CreateQueueInput struct {
	WorkspaceID string
	Name        string
	Slug        *string
	Description *string
}

// UpdateQueueInput input for updating a queue.
type UpdateQueueInput struct {
	Name        *string
	Slug        *string
	Description *string
}

// CreateFormSpecInput input for creating a form spec.
type CreateFormSpecInput struct {
	WorkspaceID          string
	Name                 string
	Slug                 *string
	PublicKey            *string
	DescriptionMarkdown  *string
	FieldSpec            *graphshared.JSON
	EvidenceRequirements *[]graphshared.JSON
	InferenceRules       *[]graphshared.JSON
	ApprovalPolicy       *graphshared.JSON
	SubmissionPolicy     *graphshared.JSON
	DestinationPolicy    *graphshared.JSON
	SupportedChannels    *[]string
	IsPublic             *bool
	Status               *string
	Metadata             *graphshared.JSON
}

// UpdateFormSpecInput input for updating a form spec.
type UpdateFormSpecInput struct {
	Name                 *string
	Slug                 *string
	PublicKey            *string
	DescriptionMarkdown  *string
	FieldSpec            *graphshared.JSON
	EvidenceRequirements *[]graphshared.JSON
	InferenceRules       *[]graphshared.JSON
	ApprovalPolicy       *graphshared.JSON
	SubmissionPolicy     *graphshared.JSON
	DestinationPolicy    *graphshared.JSON
	SupportedChannels    *[]string
	IsPublic             *bool
	Status               *string
	Metadata             *graphshared.JSON
}

// CreateFormSubmissionInput input for creating a form submission.
type CreateFormSubmissionInput struct {
	FormSpecID            string
	ConversationSessionID *string
	CaseID                *string
	ContactID             *string
	Status                *string
	Channel               *string
	SubmitterEmail        *string
	SubmitterName         *string
	CompletionToken       *string
	CollectedFields       *graphshared.JSON
	MissingFields         *graphshared.JSON
	Evidence              *[]graphshared.JSON
	ValidationErrors      *[]string
	Metadata              *graphshared.JSON
	SubmittedAt           *graphshared.DateTime
}

// CaseHandoffInput moves durable work to a target team/queue.
type CaseHandoffInput struct {
	TeamID     *string
	QueueID    string
	AssigneeID *string
	Reason     *string
}

// CreateKnowledgeResourceInput input for creating a knowledge resource.
type CreateKnowledgeResourceInput struct {
	WorkspaceID        string
	TeamID             string
	Slug               string
	Title              string
	Kind               *string
	ConceptSpecKey     *string
	ConceptSpecVersion *string
	SourceKind         *string
	SourceRef          *string
	PathRef            *string
	Summary            *string
	BodyMarkdown       *string
	Frontmatter        *graphshared.JSON
	SupportedChannels  *[]string
	SharedWithTeamIDs  *[]string
	SearchKeywords     *[]string
	Surface            *string
	Status             *string
}

// UpdateKnowledgeResourceInput input for updating a knowledge resource.
type UpdateKnowledgeResourceInput struct {
	Slug               *string
	Title              *string
	Kind               *string
	ConceptSpecKey     *string
	ConceptSpecVersion *string
	SourceKind         *string
	SourceRef          *string
	PathRef            *string
	Summary            *string
	BodyMarkdown       *string
	Frontmatter        *graphshared.JSON
	SupportedChannels  *[]string
	SearchKeywords     *[]string
	Status             *string
}

// ShareKnowledgeResourceInput updates the teams that can access a knowledge resource.
type ShareKnowledgeResourceInput struct {
	TeamIDs []string
}

// RegisterConceptSpecInput registers a versioned concept spec definition.
type RegisterConceptSpecInput struct {
	WorkspaceID           string
	OwnerTeamID           *string
	Key                   string
	Version               *string
	Name                  string
	Description           *string
	ExtendsKey            *string
	ExtendsVersion        *string
	InstanceKind          string
	MetadataSchema        *graphshared.JSON
	SectionsSchema        *graphshared.JSON
	WorkflowSchema        *graphshared.JSON
	AgentGuidanceMarkdown *string
	SourceKind            *string
	SourceRef             *string
	Status                *string
}

// AddConversationMessageInput appends a message to an existing conversation.
type AddConversationMessageInput struct {
	ParticipantID *string
	Role          *string
	Kind          *string
	Visibility    *string
	ContentText   *string
	Content       *graphshared.JSON
}

type ConversationHandoffInput struct {
	TeamID         *string
	QueueID        string
	OperatorUserID *string
	Reason         *string
}

type EscalateConversationInput struct {
	TeamID         *string
	QueueID        string
	OperatorUserID *string
	Subject        *string
	Description    *string
	Priority       *string
	Category       *string
	Reason         *string
}

// InstallExtensionAssetInput input for extension asset installation.
type InstallExtensionAssetInput struct {
	Path           string
	Content        string
	ContentType    *string
	IsCustomizable *bool
}

// InstallExtensionMigrationInput input for extension migration installation.
type InstallExtensionMigrationInput struct {
	Path    string
	Content string
}

// InstallExtensionInput input for installing an extension bundle.
type InstallExtensionInput struct {
	WorkspaceID  *string
	LicenseToken string
	BundleBase64 *string
	Manifest     graphshared.JSON
	Assets       []InstallExtensionAssetInput
	Migrations   []InstallExtensionMigrationInput
}

// UpgradeExtensionInput input for upgrading an installed extension bundle.
type UpgradeExtensionInput struct {
	LicenseToken *string
	BundleBase64 *string
	Manifest     graphshared.JSON
	Assets       []InstallExtensionAssetInput
	Migrations   []InstallExtensionMigrationInput
}

// UpdateExtensionConfigInput input for updating extension config.
type UpdateExtensionConfigInput struct {
	Config graphshared.JSON
}

// UpdateExtensionAssetInput input for updating a customizable extension asset.
type UpdateExtensionAssetInput struct {
	Path        string
	Content     string
	ContentType *string
}

// PublishExtensionArtifactInput input for publishing managed extension artifact content.
type PublishExtensionArtifactInput struct {
	Surface string
	Path    string
	Content string
}

// CreateAgentInput input for creating an agent
type CreateAgentInput struct {
	WorkspaceID string
	Name        string
	Description *string
}

// UpdateAgentInput input for updating an agent
type UpdateAgentInput struct {
	Name        *string
	Description *string
}

// CreateAgentTokenInput input for creating an agent token
type CreateAgentTokenInput struct {
	AgentID       string
	Name          string
	ExpiresInDays *int32
}

// MembershipConstraintsInput configures scoped workspace access.
type MembershipConstraintsInput struct {
	RateLimitPerMinute      *int32
	RateLimitPerHour        *int32
	AllowedIPs              *[]string
	AllowedProjectIDs       *[]string
	AllowedTeamIDs          *[]string
	AllowDelegatedRouting   *bool
	DelegatedRoutingTeamIDs *[]string
	ActiveHoursStart        *string
	ActiveHoursEnd          *string
	ActiveTimezone          *string
	ActiveDays              *[]int32
}

// GrantMembershipInput input for granting workspace membership
type GrantMembershipInput struct {
	WorkspaceID   string
	AgentID       string
	Role          string
	Permissions   []string
	ExpiresInDays *int32
	Constraints   *MembershipConstraintsInput
}

// CreateUserInput input for creating a user
type CreateUserInput struct {
	Email        string
	Name         string
	InstanceRole *string
}

// UpdateUserInput input for updating a user
type UpdateUserInput struct {
	Email         *string
	Name          *string
	InstanceRole  *string
	IsActive      *bool
	EmailVerified *bool
}

// CreateWorkspaceInput input for creating a workspace
type CreateWorkspaceInput struct {
	Name        string
	ShortCode   string
	Description *string
}

// CreateTeamInput input for creating a team.
type CreateTeamInput struct {
	WorkspaceID         string
	Name                string
	Description         *string
	EmailAddress        *string
	ResponseTimeHours   *int32
	ResolutionTimeHours *int32
	AutoAssign          *bool
	AutoAssignKeywords  *[]string
	IsActive            *bool
}

// AddTeamMemberInput input for adding a user to a team.
type AddTeamMemberInput struct {
	TeamID string
	UserID string
	Role   string
}

// UpdateWorkspaceInput input for updating a workspace
type UpdateWorkspaceInput struct {
	Name        *string
	ShortCode   *string
	Description *string
}

// CreateRuleInput input for creating a rule
type CreateRuleInput struct {
	WorkspaceID          string
	Title                string
	Description          *string
	IsActive             *bool
	Priority             *int32
	MaxExecutionsPerHour *int32
	MaxExecutionsPerDay  *int32
	Conditions           graphshared.JSON
	Actions              graphshared.JSON
}

// UpdateRuleInput input for updating a rule
type UpdateRuleInput struct {
	Title                *string
	Description          *string
	IsActive             *bool
	Priority             *int32
	MaxExecutionsPerHour *int32
	MaxExecutionsPerDay  *int32
	Conditions           *graphshared.JSON
	Actions              *graphshared.JSON
}

// CreateFormInput input for creating a form
type CreateFormInput struct {
	WorkspaceID        string
	Name               string
	Slug               string
	Description        *string
	Status             *string
	IsPublic           *bool
	RequiresCaptcha    *bool
	CollectEmail       *bool
	AutoCreateCase     *bool
	AutoCasePriority   *string
	AutoCaseType       *string
	AutoAssignTeamID   *string
	AutoTags           *[]string
	NotifyOnSubmission *bool
	NotificationEmails *[]string
	SubmissionMessage  *string
	RedirectURL        *string
	SchemaData         *graphshared.JSON
}

// UpdateFormInput input for updating a form
type UpdateFormInput struct {
	Name               *string
	Slug               *string
	Description        *string
	Status             *string
	IsPublic           *bool
	RequiresCaptcha    *bool
	CollectEmail       *bool
	AutoCreateCase     *bool
	AutoCasePriority   *string
	AutoCaseType       *string
	AutoAssignTeamID   *string
	AutoTags           *[]string
	NotifyOnSubmission *bool
	NotificationEmails *[]string
	SubmissionMessage  *string
	RedirectURL        *string
	SchemaData         *graphshared.JSON
}
