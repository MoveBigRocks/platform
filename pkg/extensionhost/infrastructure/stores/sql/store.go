package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
)

// Store implements the shared.Store interface using SQLite/sqlx
type Store struct {
	db                     *DB
	sqlxDB                 *SqlxDB
	userStore              *UserStore
	sandboxStore           *SandboxStore
	queueStore             *QueueStore
	queueItemStore         *QueueItemStore
	extensionStore         *ExtensionStore
	extensionRuntimeStore  *ExtensionRuntimeStore
	outboxStore            *OutboxStore
	caseStore              *CaseStore
	workspaceStore         *WorkspaceStore
	formStore              *FormStore
	serviceCatalogStore    *ServiceCatalogStore
	conversationStore      *ConversationStore
	formSpecStore          *FormSpecStore
	emailStore             *EmailStore
	notificationStore      *NotificationStore
	contactStore           *ContactStore
	ruleStore              *RuleStore
	jobStore               *JobStore
	conceptSpecStore       *ConceptSpecStore
	knowledgeResourceStore *KnowledgeResourceStore
	agentStore             *AgentStore
	idempotencyStore       *IdempotencyStore
	auditStore             *AuditStore
}

// NewStore creates a new SQL-based Store
func NewStore(db *DB) (*Store, error) {
	stdDB, err := db.GetSQLDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}
	sqlxDB := NewSqlxDB(stdDB, db.Driver())

	return &Store{
		db:                     db,
		sqlxDB:                 sqlxDB,
		userStore:              NewUserStore(sqlxDB),
		sandboxStore:           NewSandboxStore(sqlxDB),
		queueStore:             NewQueueStore(sqlxDB),
		queueItemStore:         NewQueueItemStore(sqlxDB),
		extensionStore:         NewExtensionStore(sqlxDB),
		extensionRuntimeStore:  NewExtensionRuntimeStore(sqlxDB),
		outboxStore:            NewOutboxStore(sqlxDB),
		caseStore:              NewCaseStore(sqlxDB),
		workspaceStore:         NewWorkspaceStore(sqlxDB),
		formStore:              NewFormStore(sqlxDB),
		serviceCatalogStore:    NewServiceCatalogStore(sqlxDB),
		conversationStore:      NewConversationStore(sqlxDB),
		formSpecStore:          NewFormSpecStore(sqlxDB),
		emailStore:             NewEmailStore(sqlxDB),
		notificationStore:      NewNotificationStore(sqlxDB),
		contactStore:           NewContactStore(sqlxDB),
		ruleStore:              NewRuleStore(sqlxDB),
		jobStore:               NewJobStore(sqlxDB),
		conceptSpecStore:       NewConceptSpecStore(sqlxDB),
		knowledgeResourceStore: NewKnowledgeResourceStore(sqlxDB),
		agentStore:             NewAgentStore(sqlxDB),
		idempotencyStore:       NewIdempotencyStore(sqlxDB),
		auditStore:             NewAuditStore(sqlxDB),
	}, nil
}

func (s *Store) Users() shared.UserStore {
	return s.userStore
}

func (s *Store) Sandboxes() shared.SandboxStore {
	return s.sandboxStore
}

func (s *Store) Queues() shared.QueueStore {
	return s.queueStore
}

func (s *Store) QueueItems() shared.QueueItemStore {
	return s.queueItemStore
}

func (s *Store) Extensions() shared.ExtensionStore {
	return s.extensionStore
}

func (s *Store) ExtensionRuntime() shared.ExtensionRuntimeStore {
	return s.extensionRuntimeStore
}

func (s *Store) ExtensionSchemaMigrator() *ExtensionSchemaMigrator {
	return NewExtensionSchemaMigrator(s.sqlxDB, s.extensionStore, s.extensionRuntimeStore)
}

func (s *Store) Outbox() shared.OutboxStore {
	return s.outboxStore
}

// Cases returns the case store
func (s *Store) Cases() shared.CaseStore {
	return s.caseStore
}

func (s *Store) Workspaces() shared.WorkspaceStore {
	return s.workspaceStore
}

func (s *Store) Forms() shared.FormStore {
	return s.formStore
}

func (s *Store) ServiceCatalog() shared.ServiceCatalogStore {
	return s.serviceCatalogStore
}

func (s *Store) Conversations() shared.ConversationStore {
	return s.conversationStore
}

func (s *Store) FormSpecs() shared.FormSpecStore {
	return s.formSpecStore
}

// Email sub-stores (the underlying emailStore implements all these interfaces)
func (s *Store) EmailTemplates() shared.EmailTemplateStore {
	return s.emailStore
}

func (s *Store) OutboundEmails() shared.OutboundEmailStore {
	return s.emailStore
}

func (s *Store) InboundEmails() shared.InboundEmailStore {
	return s.emailStore
}

func (s *Store) EmailThreads() shared.EmailThreadStore {
	return s.emailStore
}

func (s *Store) EmailSecurity() shared.EmailSecurityStore {
	return s.emailStore
}

func (s *Store) Contacts() shared.ContactStore {
	return s.contactStore
}

func (s *Store) Notifications() shared.NotificationStore {
	return s.notificationStore
}

func (s *Store) Rules() shared.RuleStore {
	return s.ruleStore
}

func (s *Store) Idempotency() shared.IdempotencyStore {
	return s.idempotencyStore
}

func (s *Store) Jobs() shared.JobStore {
	return s.jobStore
}

func (s *Store) ConceptSpecs() shared.ConceptSpecStore {
	return s.conceptSpecStore
}

func (s *Store) KnowledgeResources() shared.KnowledgeResourceStore {
	return s.knowledgeResourceStore
}

func (s *Store) Agents() shared.AgentStore {
	return s.agentStore
}

func (s *Store) Audits() shared.AuditStore {
	return s.auditStore
}

// WithTransaction executes a function within a transaction context using sqlx
func (s *Store) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.sqlxDB.Transaction(ctx, fn)
}

func (s *Store) HealthCheck(ctx context.Context) error {
	return s.db.Ping(ctx)
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying database wrapper
func (s *Store) DB() *DB {
	return s.db
}

// SqlxDB returns the sqlx database wrapper for stores that need direct access.
func (s *Store) SqlxDB() *SqlxDB {
	return s.sqlxDB
}

// GetSQLDB returns the underlying *sql.DB for low-level operations.
func (s *Store) GetSQLDB() (*sql.DB, error) {
	return s.db.GetSQLDB()
}

// SetTenantContext sets the workspace used by the core row-level-security
// policies for the current transaction. Migration 000011 filters every
// tenant-scoped table by workspace_id = public.current_workspace_id(), which
// reads the app.current_workspace_id session variable and returns no rows when
// it is unset. This sets that variable with transaction scope via set_config,
// so it must be called inside a transaction (Get(ctx) returns the transaction
// connection); called outside one it has no effect.
//
// RLS only changes query results when the application connects as a database
// role that does not bypass row-level security. Until that role is in place and
// every read path sets this context, this is correct plumbing that is inert in
// production. See docs/ADRs/0003 for the enforcement rollout.
func (s *Store) SetTenantContext(ctx context.Context, workspaceID string) error {
	if s.sqlxDB.driver != "postgres" {
		return nil
	}
	if _, err := s.sqlxDB.Get(ctx).ExecContext(
		ctx,
		`SELECT set_config('app.current_workspace_id', $1, true)`,
		strings.TrimSpace(workspaceID),
	); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}
	return nil
}

// WithAdminContext executes a function for cross-workspace operations inside a
// transaction. It intentionally does not set app.current_workspace_id, so under
// a role that bypasses row-level security the function sees all workspaces.
// Under a non-bypassing role this would need to switch to a BYPASSRLS admin
// role; that is part of the enforcement rollout referenced in SetTenantContext.
func (s *Store) WithAdminContext(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.WithTransaction(ctx, fn)
}
