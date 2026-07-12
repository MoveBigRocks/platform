package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"

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

	// adminRole is the BYPASSRLS database role the store switches to for
	// cross-workspace operations, or "" when no such role is available to the
	// connecting user. It is resolved once at construction. When empty,
	// WithAdminContext runs in the connecting role, which is correct while
	// row-level security is not enforced.
	adminRole string
}

// adminRoleName is the database role that bypasses row-level security for
// legitimate cross-workspace work (workers, admin portal, cross-tenant email
// routing). Migration 000011 provisions it as BYPASSRLS and grants it to the
// application role. It is a fixed identifier, never user input.
const adminRoleName = "mbr_admin"

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
		adminRole:              detectAdminRole(sqlxDB),
	}, nil
}

// detectAdminRole reports the cross-workspace admin role the connecting user
// may assume, or "" when none is available. It checks that adminRoleName exists
// and that the current user is a member, so WithAdminContext can switch to it
// only when the switch will succeed. A missing role (an older database where
// the RLS rollout has not provisioned it) yields "", leaving admin operations
// in the connecting role. The result is stable for a given role, so it is
// resolved once; provisioning the role later takes effect on the next start.
func detectAdminRole(db *SqlxDB) string {
	if db == nil || db.driver != "postgres" {
		return ""
	}
	var role string
	err := db.DB.QueryRowxContext(
		context.Background(),
		`SELECT rolname FROM pg_roles
		 WHERE rolname = $1 AND pg_has_role(current_user, oid, 'USAGE')`,
		adminRoleName,
	).Scan(&role)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(role)
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
// transaction. It does not set app.current_workspace_id; instead it switches to
// the BYPASSRLS admin role for the duration of the transaction, so the function
// sees every workspace even when the connecting role is subject to row-level
// security. SET LOCAL ROLE is transaction-scoped and reset at commit or
// rollback. When no admin role is available (adminRole is ""), it runs in the
// connecting role, which is correct while RLS is not enforced.
func (s *Store) WithAdminContext(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.enterAdminRole(txCtx); err != nil {
			return err
		}
		return fn(txCtx)
	})
}

// enterAdminRole switches the current transaction to the admin role when one is
// available. It must run inside a transaction so the switch is local; Get(ctx)
// returns the transaction connection in that case.
func (s *Store) enterAdminRole(ctx context.Context) error {
	if s.adminRole == "" || s.sqlxDB.driver != "postgres" {
		return nil
	}
	// adminRole is resolved from pg_roles at construction and is a fixed
	// identifier, not user input, so it is safe to interpolate. SET ROLE does
	// not accept bind parameters.
	if _, err := s.sqlxDB.Get(ctx).ExecContext(ctx, "SET LOCAL ROLE "+pq.QuoteIdentifier(s.adminRole)); err != nil {
		return fmt.Errorf("enter admin role %s: %w", s.adminRole, err)
	}
	return nil
}
