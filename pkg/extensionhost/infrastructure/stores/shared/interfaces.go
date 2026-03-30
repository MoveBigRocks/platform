package shared

import (
	"context"
	"database/sql"
	"io"
	"time"

	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// Store is the main storage interface that composes all sub-stores
type Store interface {
	// Core sub-stores
	Users() UserStore
	Workspaces() WorkspaceStore
	Sandboxes() SandboxStore
	Queues() QueueStore
	QueueItems() QueueItemStore
	Cases() CaseStore
	Contacts() ContactStore
	Extensions() ExtensionStore
	ExtensionRuntime() ExtensionRuntimeStore

	// Email sub-stores
	EmailTemplates() EmailTemplateStore
	OutboundEmails() OutboundEmailStore
	InboundEmails() InboundEmailStore
	EmailThreads() EmailThreadStore
	EmailSecurity() EmailSecurityStore

	// Agent sub-stores
	Agents() AgentStore

	// Other sub-stores
	Jobs() JobStore
	ServiceCatalog() ServiceCatalogStore
	Conversations() ConversationStore
	FormSpecs() FormSpecStore
	ConceptSpecs() ConceptSpecStore
	KnowledgeResources() KnowledgeResourceStore
	Forms() FormStore
	Rules() RuleStore
	Outbox() OutboxStore
	Idempotency() IdempotencyStore
	Notifications() NotificationStore

	// WithTransaction executes a function within a transaction context.
	// If the function returns an error, all staged operations are abandoned.
	// If the function succeeds, all operations are committed atomically.
	// The transaction object is embedded in the context.
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error

	// Utility
	HealthCheck(ctx context.Context) error

	// Close closes the underlying database connection.
	// Should be called when the store is no longer needed.
	Close() error

	// GetSQLDB returns the underlying *sql.DB for low-level operations.
	GetSQLDB() (*sql.DB, error)

	// SetTenantContext sets the tenant context for the current connection.
	// For SQLite, this is a no-op as tenant isolation is enforced at the query level.
	SetTenantContext(ctx context.Context, workspaceID string) error

	// WithAdminContext executes a function for cross-workspace operations.
	// This is used by workers that need to query across all workspaces (e.g., auto-close, notifications).
	// For SQLite, this simply runs the function within a transaction.
	// SECURITY: Only use this for legitimate cross-tenant administrative operations.
	WithAdminContext(ctx context.Context, fn func(ctx context.Context) error) error
}

// UserCRUD handles basic user CRUD operations
type UserCRUD interface {
	CreateUser(ctx context.Context, user *platformdomain.User) error
	GetUser(ctx context.Context, userID string) (*platformdomain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*platformdomain.User, error)
	GetUsersByIDs(ctx context.Context, userIDs []string) ([]*platformdomain.User, error)
	UpdateUser(ctx context.Context, user *platformdomain.User) error
	ListUsers(ctx context.Context) ([]*platformdomain.User, error)
}

// SessionStore handles session management
type SessionStore interface {
	SaveSession(ctx context.Context, session *platformdomain.Session) error
	GetSessionByHash(ctx context.Context, tokenHash string) (*platformdomain.Session, error)
	UpdateSession(ctx context.Context, session *platformdomain.Session) error
	DeleteSessionByHash(ctx context.Context, tokenHash string) error
	CleanupExpiredSessions(ctx context.Context) error
}

// MagicLinkStore handles magic link operations
type MagicLinkStore interface {
	SaveMagicLink(ctx context.Context, link *platformdomain.MagicLinkToken) error
	GetMagicLink(ctx context.Context, token string) (*platformdomain.MagicLinkToken, error)
	MarkMagicLinkUsed(ctx context.Context, token string) error
	CleanupExpiredMagicLinks(ctx context.Context) error

	// Rate limiting (database-backed for distributed deployments)
	CheckRateLimit(ctx context.Context, key string, maxAttempts int, window, blockDuration time.Duration) (bool, time.Duration, error)
	CleanupExpiredRateLimits(ctx context.Context) error
}

// UserStore composes all user-related interfaces.
// Services can depend on narrower interfaces when full UserStore is not needed.
type UserStore interface {
	UserCRUD
	SessionStore
	MagicLinkStore
	DeleteUser(ctx context.Context, userID string) error
}

// WorkspaceCRUD handles basic workspace CRUD operations
type WorkspaceCRUD interface {
	CreateWorkspace(ctx context.Context, workspace *platformdomain.Workspace) error
	GetWorkspace(ctx context.Context, workspaceID string) (*platformdomain.Workspace, error)
	GetWorkspaceBySlug(ctx context.Context, slug string) (*platformdomain.Workspace, error)
	GetWorkspacesByIDs(ctx context.Context, workspaceIDs []string) ([]*platformdomain.Workspace, error)
	UpdateWorkspace(ctx context.Context, workspace *platformdomain.Workspace) error
	ListWorkspaces(ctx context.Context) ([]*platformdomain.Workspace, error)
	ListUserWorkspaces(ctx context.Context, userID string) ([]*platformdomain.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
}

// WorkspaceRoleStore handles user-workspace role operations
type WorkspaceRoleStore interface {
	CreateUserWorkspaceRole(ctx context.Context, role *platformdomain.UserWorkspaceRole) error
	GetUserWorkspaceRoles(ctx context.Context, userID string) ([]*platformdomain.UserWorkspaceRole, error)
	GetWorkspaceUsers(ctx context.Context, workspaceID string) ([]*platformdomain.UserWorkspaceRole, error)
	DeleteUserWorkspaceRole(ctx context.Context, userID, workspaceID string) error
}

// TeamStore handles team operations
type TeamStore interface {
	CreateTeam(ctx context.Context, team *platformdomain.Team) error
	GetTeam(ctx context.Context, teamID string) (*platformdomain.Team, error)
	ListWorkspaceTeams(ctx context.Context, workspaceID string) ([]*platformdomain.Team, error)
	AddTeamMember(ctx context.Context, member *platformdomain.TeamMember) error
	GetTeamMembers(ctx context.Context, workspaceID, teamID string) ([]*platformdomain.TeamMember, error)
}

// WorkspaceSettingsStore handles workspace settings
type WorkspaceSettingsStore interface {
	GetWorkspaceSettings(ctx context.Context, workspaceID string) (*platformdomain.WorkspaceSettings, error)
	CreateWorkspaceSettings(ctx context.Context, settings *platformdomain.WorkspaceSettings) error
}

// WorkspaceStore composes all workspace-related interfaces.
// Services can depend on narrower interfaces when full WorkspaceStore is not needed.
type WorkspaceStore interface {
	WorkspaceCRUD
	WorkspaceRoleStore
	TeamStore
	WorkspaceSettingsStore
}

// SandboxStore handles vendor-operated trial sandbox lifecycle state.
type SandboxStore interface {
	CreateSandbox(ctx context.Context, sandbox *platformdomain.Sandbox) error
	GetSandbox(ctx context.Context, sandboxID string) (*platformdomain.Sandbox, error)
	GetSandboxBySlug(ctx context.Context, slug string) (*platformdomain.Sandbox, error)
	GetSandboxByVerificationTokenHash(ctx context.Context, tokenHash string) (*platformdomain.Sandbox, error)
	ListReapableSandboxes(ctx context.Context, now time.Time) ([]*platformdomain.Sandbox, error)
	UpdateSandbox(ctx context.Context, sandbox *platformdomain.Sandbox) error
}

// QueueStore handles workspace-scoped case queues.
type QueueStore interface {
	CreateQueue(ctx context.Context, queue *servicedomain.Queue) error
	GetQueue(ctx context.Context, queueID string) (*servicedomain.Queue, error)
	GetQueueBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.Queue, error)
	ListWorkspaceQueues(ctx context.Context, workspaceID string) ([]*servicedomain.Queue, error)
	UpdateQueue(ctx context.Context, queue *servicedomain.Queue) error
	DeleteQueue(ctx context.Context, workspaceID, queueID string) error
}

// QueueItemStore handles the concrete work items assigned to queues.
type QueueItemStore interface {
	CreateQueueItem(ctx context.Context, item *servicedomain.QueueItem) error
	GetQueueItem(ctx context.Context, itemID string) (*servicedomain.QueueItem, error)
	GetQueueItemByCaseID(ctx context.Context, caseID string) (*servicedomain.QueueItem, error)
	GetQueueItemByConversationSessionID(ctx context.Context, sessionID string) (*servicedomain.QueueItem, error)
	ListQueueItems(ctx context.Context, queueID string) ([]*servicedomain.QueueItem, error)
	UpdateQueueItem(ctx context.Context, item *servicedomain.QueueItem) error
	DeleteQueueItem(ctx context.Context, itemID string) error
	DeleteQueueItemByCaseID(ctx context.Context, caseID string) error
	DeleteQueueItemByConversationSessionID(ctx context.Context, sessionID string) error
}

// ConceptSpecStore handles versioned concept spec definitions for structured knowledge.
type ConceptSpecStore interface {
	CreateConceptSpec(ctx context.Context, spec *knowledgedomain.ConceptSpec) error
	GetConceptSpec(ctx context.Context, workspaceID, key, version string) (*knowledgedomain.ConceptSpec, error)
	ListWorkspaceConceptSpecs(ctx context.Context, workspaceID string) ([]*knowledgedomain.ConceptSpec, error)
}

// ExtensionStore handles installed extension lifecycle and asset storage.
type ExtensionStore interface {
	CreateInstalledExtension(ctx context.Context, extension *platformdomain.InstalledExtension) error
	GetInstalledExtension(ctx context.Context, extensionID string) (*platformdomain.InstalledExtension, error)
	GetInstalledExtensionBySlug(ctx context.Context, workspaceID, slug string) (*platformdomain.InstalledExtension, error)
	GetInstanceExtensionBySlug(ctx context.Context, slug string) (*platformdomain.InstalledExtension, error)
	ListWorkspaceExtensions(ctx context.Context, workspaceID string) ([]*platformdomain.InstalledExtension, error)
	ListInstanceExtensions(ctx context.Context) ([]*platformdomain.InstalledExtension, error)
	ListAllExtensions(ctx context.Context) ([]*platformdomain.InstalledExtension, error)
	UpdateInstalledExtension(ctx context.Context, extension *platformdomain.InstalledExtension) error
	DeleteInstalledExtension(ctx context.Context, extensionID string) error
	GetExtensionBundle(ctx context.Context, extensionID string) ([]byte, error)
	ReplaceExtensionAssets(ctx context.Context, extensionID string, assets []*platformdomain.ExtensionAsset) error
	ListExtensionAssets(ctx context.Context, extensionID string) ([]*platformdomain.ExtensionAsset, error)
	GetExtensionAsset(ctx context.Context, extensionID, assetPath string) (*platformdomain.ExtensionAsset, error)
	UpdateExtensionAsset(ctx context.Context, asset *platformdomain.ExtensionAsset) error
}

// ExtensionRuntimeStore handles instance-scoped schema registration and migration history.
type ExtensionRuntimeStore interface {
	UpsertExtensionPackageRegistration(ctx context.Context, registration *platformdomain.ExtensionPackageRegistration) error
	GetExtensionPackageRegistration(ctx context.Context, packageKey string) (*platformdomain.ExtensionPackageRegistration, error)
	ListExtensionPackageRegistrations(ctx context.Context) ([]*platformdomain.ExtensionPackageRegistration, error)
	CreateExtensionSchemaMigration(ctx context.Context, migration *platformdomain.ExtensionSchemaMigration) error
	ListExtensionSchemaMigrations(ctx context.Context, packageKey string) ([]*platformdomain.ExtensionSchemaMigration, error)
}

// CaseCRUD handles basic case CRUD operations
type CaseCRUD interface {
	CreateCase(ctx context.Context, caseObj *servicedomain.Case) error
	GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error)
	// GetCaseInWorkspace retrieves a case only if it belongs to the specified workspace.
	// Returns error if case not found OR belongs to different workspace (defense-in-depth).
	GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*servicedomain.Case, error)
	GetCaseByHumanID(ctx context.Context, humanID string) (*servicedomain.Case, error)
	GetCasesByIDs(ctx context.Context, caseIDs []string) ([]*servicedomain.Case, error)
	// GetCaseByIssueAndContact finds an existing case for idempotency checks.
	// Returns the case or ErrNotFound if no case exists for this issue/contact combination.
	GetCaseByIssueAndContact(ctx context.Context, workspaceID, issueID, contactID string) (*servicedomain.Case, error)
	UpdateCase(ctx context.Context, caseObj *servicedomain.Case) error
	ListWorkspaceCases(ctx context.Context, workspaceID string, filter CaseFilter) ([]*servicedomain.Case, error)
	ListWorkspaceCasesFast(ctx context.Context, workspaceID string, filter CaseFilter) ([]*servicedomain.Case, error)
	ListCases(ctx context.Context, filters contracts.CaseFilters) ([]*servicedomain.Case, int, error)
	DeleteCase(ctx context.Context, workspaceID, caseID string) error
}

// CaseCommunications handles case communication operations
type CaseCommunications interface {
	CreateCommunication(ctx context.Context, comm *servicedomain.Communication) error
	UpdateCommunication(ctx context.Context, comm *servicedomain.Communication) error
	GetCommunication(ctx context.Context, workspaceID, commID string) (*servicedomain.Communication, error)
	ListCaseCommunications(ctx context.Context, caseID string) ([]*servicedomain.Communication, error)
	ListCommunications(ctx context.Context, workspaceID, caseID string) ([]*servicedomain.Communication, error)
}

// CaseAttachments handles attachment operations
type CaseAttachments interface {
	SaveAttachment(ctx context.Context, att *servicedomain.Attachment, data io.Reader) error
	GetAttachment(ctx context.Context, workspaceID, attID string) (*servicedomain.Attachment, error)
	ListCaseAttachments(ctx context.Context, workspaceID, caseID string) ([]*servicedomain.Attachment, error)
	LinkAttachmentsToCase(ctx context.Context, workspaceID, caseID string, attachmentIDs []string) error
	LinkInboundEmailAttachments(ctx context.Context, workspaceID, emailID, caseID, communicationID string) error
	DeleteAttachment(ctx context.Context, workspaceID, attID string) error
}

// CaseBulkOps handles bulk operations
type CaseBulkOps interface {
	MarkCaseNotified(ctx context.Context, workspaceID, caseID string) error
	ListResolvedCasesForAutoClose(ctx context.Context, resolvedBefore time.Time, limit int) ([]*servicedomain.Case, error)
}

// CaseKnowledge handles knowledge-resource linking for cases.
type CaseKnowledge interface {
	CreateCaseKnowledgeResourceLink(ctx context.Context, link *knowledgedomain.CaseKnowledgeResourceLink) error
	GetCaseKnowledgeResourceLinks(ctx context.Context, caseID string) ([]*knowledgedomain.CaseKnowledgeResourceLink, error)
	DeleteCaseKnowledgeResourceLink(ctx context.Context, linkID string) error
	LinkCaseToKnowledgeResource(ctx context.Context, caseID, knowledgeResourceID string) error
}

// CaseQueries handles case query operations
type CaseQueries interface {
	ListCasesByMessageID(ctx context.Context, workspaceID, messageID string) ([]*servicedomain.Case, error)
	ListCasesBySubject(ctx context.Context, workspaceID, subject string) ([]*servicedomain.Case, error)
	GetCaseCount(ctx context.Context, workspaceID string, filter CaseFilter) (int, error)
}

// CaseAssignmentHistory handles assignment history operations
type CaseAssignmentHistory interface {
	CreateCaseAssignmentHistory(ctx context.Context, history *shareddomain.CaseAssignmentHistory) error
	GetCaseAssignmentHistory(ctx context.Context, historyID string) (*shareddomain.CaseAssignmentHistory, error)
	ListCaseAssignmentHistoryByCase(ctx context.Context, caseID string) ([]*shareddomain.CaseAssignmentHistory, error)
	UpdateCaseAssignmentHistory(ctx context.Context, history *shareddomain.CaseAssignmentHistory) error
}

// CaseStore composes all case-related interfaces.
// Services can depend on narrower interfaces when full CaseStore is not needed.
type CaseStore interface {
	CaseCRUD
	CaseCommunications
	CaseAttachments
	CaseBulkOps
	CaseKnowledge
	CaseQueries
	CaseAssignmentHistory
}

// ContactStore handles contact/customer operations
type ContactStore interface {
	CreateContact(ctx context.Context, contact *platformdomain.Contact) error
	GetContact(ctx context.Context, workspaceID, contactID string) (*platformdomain.Contact, error)
	GetContactByEmail(ctx context.Context, workspaceID, email string) (*platformdomain.Contact, error)
	UpdateContact(ctx context.Context, contact *platformdomain.Contact) error
	ListWorkspaceContacts(ctx context.Context, workspaceID string) ([]*platformdomain.Contact, error)
	DeleteContact(ctx context.Context, workspaceID, contactID string) error
}

// NotificationStore handles persisted user notifications.
type NotificationStore interface {
	CreateNotification(ctx context.Context, notification *shareddomain.Notification) error
	GetNotification(ctx context.Context, workspaceID, notificationID string) (*shareddomain.Notification, error)
	ListUserNotifications(ctx context.Context, workspaceID, userID string) ([]*shareddomain.Notification, error)
}

// ServiceCatalogStore handles the operational service catalog and its bindings.
type ServiceCatalogStore interface {
	CreateServiceCatalogNode(ctx context.Context, node *servicedomain.ServiceCatalogNode) error
	GetServiceCatalogNode(ctx context.Context, nodeID string) (*servicedomain.ServiceCatalogNode, error)
	GetServiceCatalogNodeByPath(ctx context.Context, workspaceID, pathSlug string) (*servicedomain.ServiceCatalogNode, error)
	ListWorkspaceServiceCatalogNodes(ctx context.Context, workspaceID string) ([]*servicedomain.ServiceCatalogNode, error)
	ListChildServiceCatalogNodes(ctx context.Context, workspaceID, parentNodeID string) ([]*servicedomain.ServiceCatalogNode, error)
	UpdateServiceCatalogNode(ctx context.Context, node *servicedomain.ServiceCatalogNode) error
	CreateServiceCatalogBinding(ctx context.Context, binding *servicedomain.ServiceCatalogBinding) error
	ListServiceCatalogBindings(ctx context.Context, catalogNodeID string) ([]*servicedomain.ServiceCatalogBinding, error)
	ListServiceCatalogBindingsForTarget(ctx context.Context, workspaceID, targetKind, targetID string) ([]*servicedomain.ServiceCatalogBinding, error)
}

// ConversationStore handles live supervised conversations.
type ConversationStore interface {
	CreateConversationSession(ctx context.Context, session *servicedomain.ConversationSession) error
	GetConversationSession(ctx context.Context, sessionID string) (*servicedomain.ConversationSession, error)
	GetConversationSessionByExternalKey(ctx context.Context, workspaceID string, channel servicedomain.ConversationChannel, externalSessionKey string) (*servicedomain.ConversationSession, error)
	UpdateConversationSession(ctx context.Context, session *servicedomain.ConversationSession) error
	ListWorkspaceConversationSessions(ctx context.Context, workspaceID string, filter servicedomain.ConversationSessionFilter) ([]*servicedomain.ConversationSession, error)
	CreateConversationParticipant(ctx context.Context, participant *servicedomain.ConversationParticipant) error
	ListConversationParticipants(ctx context.Context, sessionID string) ([]*servicedomain.ConversationParticipant, error)
	CreateConversationMessage(ctx context.Context, message *servicedomain.ConversationMessage) error
	ListConversationMessages(ctx context.Context, sessionID string, visibility servicedomain.ConversationMessageVisibility) ([]*servicedomain.ConversationMessage, error)
	UpsertConversationWorkingState(ctx context.Context, state *servicedomain.ConversationWorkingState) error
	GetConversationWorkingState(ctx context.Context, sessionID string) (*servicedomain.ConversationWorkingState, error)
	CreateConversationOutcome(ctx context.Context, outcome *servicedomain.ConversationOutcome) error
	ListConversationOutcomes(ctx context.Context, sessionID string) ([]*servicedomain.ConversationOutcome, error)
}

// FormSpecStore handles form specs and their collected submissions.
type FormSpecStore interface {
	CreateFormSpec(ctx context.Context, spec *servicedomain.FormSpec) error
	GetFormSpec(ctx context.Context, specID string) (*servicedomain.FormSpec, error)
	GetFormSpecBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.FormSpec, error)
	GetFormSpecByPublicKey(ctx context.Context, publicKey string) (*servicedomain.FormSpec, error)
	UpdateFormSpec(ctx context.Context, spec *servicedomain.FormSpec) error
	ListWorkspaceFormSpecs(ctx context.Context, workspaceID string) ([]*servicedomain.FormSpec, error)
	DeleteFormSpec(ctx context.Context, workspaceID, specID string) error
	CreateFormSubmission(ctx context.Context, submission *servicedomain.FormSubmission) error
	GetFormSubmission(ctx context.Context, submissionID string) (*servicedomain.FormSubmission, error)
	UpdateFormSubmission(ctx context.Context, submission *servicedomain.FormSubmission) error
	ListFormSubmissions(ctx context.Context, workspaceID string, filter servicedomain.FormSubmissionFilter) ([]*servicedomain.FormSubmission, error)
}

// KnowledgeResourceStore handles the new markdown-first knowledge runtime records.
type KnowledgeResourceStore interface {
	CreateKnowledgeResource(ctx context.Context, resource *knowledgedomain.KnowledgeResource) error
	GetKnowledgeResource(ctx context.Context, resourceID string) (*knowledgedomain.KnowledgeResource, error)
	GetKnowledgeResourceBySlug(ctx context.Context, workspaceID, teamID string, surface knowledgedomain.KnowledgeSurface, slug string) (*knowledgedomain.KnowledgeResource, error)
	UpdateKnowledgeResource(ctx context.Context, resource *knowledgedomain.KnowledgeResource) error
	ListWorkspaceKnowledgeResources(ctx context.Context, workspaceID string, filter *knowledgedomain.KnowledgeResourceFilter) ([]*knowledgedomain.KnowledgeResource, int, error)
	DeleteKnowledgeResource(ctx context.Context, workspaceID, resourceID string) error
}

// EmailTemplateStore handles email template operations
type EmailTemplateStore interface {
	CreateEmailTemplate(ctx context.Context, template *servicedomain.EmailTemplate) error
	GetEmailTemplate(ctx context.Context, templateID string) (*servicedomain.EmailTemplate, error)
	UpdateEmailTemplate(ctx context.Context, template *servicedomain.EmailTemplate) error
	ListWorkspaceEmailTemplates(ctx context.Context, workspaceID string) ([]*servicedomain.EmailTemplate, error)
}

// OutboundEmailStore handles outgoing email operations
type OutboundEmailStore interface {
	CreateOutboundEmail(ctx context.Context, email *servicedomain.OutboundEmail) error
	GetOutboundEmail(ctx context.Context, emailID string) (*servicedomain.OutboundEmail, error)
	GetOutboundEmailByProviderMessageID(ctx context.Context, providerMessageID string) (*servicedomain.OutboundEmail, error)
	UpdateOutboundEmail(ctx context.Context, email *servicedomain.OutboundEmail) error
}

// InboundEmailStore handles incoming email operations
type InboundEmailStore interface {
	CreateInboundEmail(ctx context.Context, email *servicedomain.InboundEmail) error
	GetInboundEmail(ctx context.Context, emailID string) (*servicedomain.InboundEmail, error)
	UpdateInboundEmail(ctx context.Context, email *servicedomain.InboundEmail) error
	GetEmailsByThread(ctx context.Context, threadID string) ([]*servicedomain.InboundEmail, error)
}

// EmailThreadStore handles email thread operations
type EmailThreadStore interface {
	CreateEmailThread(ctx context.Context, thread *servicedomain.EmailThread) error
	GetEmailThread(ctx context.Context, threadID string) (*servicedomain.EmailThread, error)
	UpdateEmailThread(ctx context.Context, thread *servicedomain.EmailThread) error
}

// EmailSecurityStore handles blacklist and quarantine
type EmailSecurityStore interface {
	CreateEmailBlacklist(ctx context.Context, blacklist *servicedomain.EmailBlacklist) error
	ListWorkspaceEmailBlacklists(ctx context.Context, workspaceID string) ([]*servicedomain.EmailBlacklist, error)
	CheckEmailBlacklist(ctx context.Context, workspaceID, email, domain string) (*servicedomain.EmailBlacklist, error)
}

// JobStore handles background job operations
type JobStore interface {
	CreateJob(ctx context.Context, job *automationdomain.Job) error
	GetJob(ctx context.Context, jobID string) (*automationdomain.Job, error)
	UpdateJob(ctx context.Context, job *automationdomain.Job) error
	ListWorkspaceJobs(ctx context.Context, workspaceID string, status automationdomain.JobStatus, queue string, limit, offset int) ([]*automationdomain.Job, int, error)
}

// FormStore handles public form surfaces and submissions backed by form specs.
type FormStore interface {
	// Form definitions
	CreateFormSchema(ctx context.Context, form *servicedomain.FormSchema) error
	GetFormSchema(ctx context.Context, formID string) (*servicedomain.FormSchema, error)
	GetFormSchemaBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.FormSchema, error)
	GetFormByCryptoID(ctx context.Context, cryptoID string) (*servicedomain.FormSchema, error)
	GetFormBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.FormSchema, error)
	UpdateFormSchema(ctx context.Context, form *servicedomain.FormSchema) error
	ListWorkspaceFormSchemas(ctx context.Context, workspaceID string) ([]*servicedomain.FormSchema, error)
	ListAllFormSchemas(ctx context.Context) ([]*servicedomain.FormSchema, error)
	ListPublicForms(ctx context.Context) ([]*servicedomain.FormSchema, error)
	DeleteFormSchema(ctx context.Context, workspaceID, formID string) error

	// Public form submissions
	CreateFormSubmission(ctx context.Context, submission *servicedomain.PublicFormSubmission) error
	GetFormSubmission(ctx context.Context, submissionID string) (*servicedomain.PublicFormSubmission, error)
	UpdateFormSubmission(ctx context.Context, submission *servicedomain.PublicFormSubmission) error
	ListFormSubmissions(ctx context.Context, formID string) ([]*servicedomain.PublicFormSubmission, error)
	GetFormAnalytics(ctx context.Context, formID string) (*servicedomain.FormAnalytics, error)

	// Public form API tokens
	CreateFormAPIToken(ctx context.Context, token *servicedomain.FormAPIToken) error
	GetFormAPIToken(ctx context.Context, token string) (*servicedomain.FormAPIToken, error)
}

// RuleStore handles automation rules and workflows
type RuleStore interface {
	// Rules
	CreateRule(ctx context.Context, rule *automationdomain.Rule) error
	GetRule(ctx context.Context, ruleID string) (*automationdomain.Rule, error)
	UpdateRule(ctx context.Context, rule *automationdomain.Rule) error
	ListWorkspaceRules(ctx context.Context, workspaceID string) ([]*automationdomain.Rule, error)
	ListAllRules(ctx context.Context) ([]*automationdomain.Rule, error)
	ListActiveRules(ctx context.Context, workspaceID string) ([]*automationdomain.Rule, error)
	DeleteRule(ctx context.Context, workspaceID, ruleID string) error

	// Rule execution
	CreateRuleExecution(ctx context.Context, execution *automationdomain.RuleExecution) error
	UpdateRuleExecution(ctx context.Context, execution *automationdomain.RuleExecution) error
	ListRuleExecutions(ctx context.Context, ruleID string) ([]*automationdomain.RuleExecution, error)

	// Rule stats - atomic update to avoid race conditions
	IncrementRuleStats(ctx context.Context, workspaceID, ruleID string, success bool, executedAt time.Time) error
}

// OutboxStore handles outbox pattern for reliable event publishing
type OutboxStore interface {
	SaveOutboxEvent(ctx context.Context, event *OutboxEvent) error
	GetOutboxEvent(ctx context.Context, eventID string) (*OutboxEvent, error)
	GetPendingOutboxEvents(ctx context.Context, limit int) ([]*OutboxEvent, error)
	UpdateOutboxEvent(ctx context.Context, event *OutboxEvent) error
	DeletePublishedOutboxEvents(ctx context.Context, before time.Time) error
	// RecoverStalePublishingEvents reverts events stuck in "publishing" status back to "pending"
	RecoverStalePublishingEvents(ctx context.Context, staleThreshold time.Duration) (int, error)
}

// IdempotencyStore tracks processed events to prevent duplicate handling
type IdempotencyStore interface {
	MarkProcessed(ctx context.Context, eventID, handlerGroup string) error
	IsProcessed(ctx context.Context, eventID, handlerGroup string) (bool, error)
}

// OutboxEvent represents an event waiting to be published
type OutboxEvent struct {
	ID            string     `json:"id"`
	Stream        string     `json:"stream"`
	AggregateType *string    `json:"aggregate_type,omitempty"`
	AggregateID   *string    `json:"aggregate_id,omitempty"`
	EventType     string     `json:"event_type"`
	EventData     []byte     `json:"event_data"`
	CorrelationID *string    `json:"correlation_id,omitempty"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	CreatedAt     time.Time  `json:"created_at"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	NextRetry     *time.Time `json:"next_retry,omitempty"`
}

// BulkUpdateResult contains the results of a bulk update operation
type BulkUpdateResult struct {
	Succeeded []string
	Failed    []string
	Errors    []error
}

// Filter types
type CaseFilter struct {
	Status         string     `json:"status,omitempty"`
	StatusNot      string     `json:"status_not,omitempty"`
	Priority       string     `json:"priority,omitempty"`
	QueueID        string     `json:"queue_id,omitempty"`
	TeamID         string     `json:"team_id,omitempty"`
	AssignedToID   string     `json:"assigned_to_id,omitempty"`
	ContactID      string     `json:"contact_id,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
	Search         string     `json:"search,omitempty"`
	ResolvedBefore *time.Time `json:"resolved_before,omitempty"`
	Limit          int        `json:"limit,omitempty"`
	Offset         int        `json:"offset,omitempty"`
}

// CaseFilters re-exports contracts.CaseFilters for store packages.
type CaseFilters = contracts.CaseFilters

// =============================================================================
// Agent Store Interfaces
// =============================================================================

// AgentStore handles agent and agent token operations
type AgentStore interface {
	// Agent CRUD
	CreateAgent(ctx context.Context, agent *platformdomain.Agent) error
	GetAgentByID(ctx context.Context, agentID string) (*platformdomain.Agent, error)
	GetAgentByName(ctx context.Context, workspaceID, name string) (*platformdomain.Agent, error)
	ListAgents(ctx context.Context, workspaceID string) ([]*platformdomain.Agent, error)
	UpdateAgent(ctx context.Context, agent *platformdomain.Agent) error
	DeleteAgent(ctx context.Context, agentID string) error

	// Agent Token operations
	CreateAgentToken(ctx context.Context, token *platformdomain.AgentToken) error
	GetAgentTokenByHash(ctx context.Context, tokenHash string) (*platformdomain.AgentToken, error)
	GetAgentTokenByID(ctx context.Context, tokenID string) (*platformdomain.AgentToken, error)
	ListAgentTokens(ctx context.Context, agentID string) ([]*platformdomain.AgentToken, error)
	UpdateAgentTokenUsage(ctx context.Context, tokenID, ip string) error
	RevokeAgentToken(ctx context.Context, tokenID, revokedByID string) error

	// Workspace Membership operations
	CreateWorkspaceMembership(ctx context.Context, membership *platformdomain.WorkspaceMembership) error
	GetWorkspaceMembership(ctx context.Context, workspaceID, principalID string, principalType platformdomain.PrincipalType) (*platformdomain.WorkspaceMembership, error)
	GetWorkspaceMembershipByID(ctx context.Context, membershipID string) (*platformdomain.WorkspaceMembership, error)
	RevokeWorkspaceMembership(ctx context.Context, membershipID, revokedByID string) error
}
