package sql_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	sqlstore "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/refext"
	"github.com/movebigrocks/platform/pkg/id"
)

func TestOutboxStoreGetPendingOutboxEventsOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	dueRetry := now.Add(-time.Minute)
	futureRetry := now.Add(time.Hour)

	due := &shared.OutboxEvent{
		ID:        id.New(),
		Stream:    "cases",
		EventType: "case.created",
		EventData: []byte(`{"case_id":"one"}`),
		Status:    "pending",
		Attempts:  0,
		CreatedAt: now.Add(-2 * time.Minute),
		NextRetry: &dueRetry,
	}
	future := &shared.OutboxEvent{
		ID:        id.New(),
		Stream:    "cases",
		EventType: "case.created",
		EventData: []byte(`{"case_id":"two"}`),
		Status:    "pending",
		Attempts:  0,
		CreatedAt: now.Add(-time.Minute),
		NextRetry: &futureRetry,
	}

	require.NoError(t, store.Outbox().SaveOutboxEvent(ctx, due))
	require.NoError(t, store.Outbox().SaveOutboxEvent(ctx, future))

	pending, err := store.Outbox().GetPendingOutboxEvents(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, due.ID, pending[0].ID)
}

func TestIdempotencyStoreMarkProcessedOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	eventID := id.New()

	require.NoError(t, store.Idempotency().MarkProcessed(ctx, eventID, "case-sync"))

	processed, err := store.Idempotency().IsProcessed(ctx, eventID, "case-sync")
	require.NoError(t, err)
	assert.True(t, processed)
}

func TestRuleStoreIncrementRuleStatsOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	rule := automationdomain.NewRule(workspace.ID, "Auto Assign", "user-rule-owner")
	require.NoError(t, store.Rules().CreateRule(ctx, rule))

	beforeUpdate := rule.UpdatedAt
	executedAt := time.Now().UTC()
	require.NoError(t, store.Rules().IncrementRuleStats(ctx, workspace.ID, rule.ID, true, executedAt))

	updated, err := store.Rules().GetRule(ctx, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, updated.TotalExecutions)
	require.NotNil(t, updated.LastExecutedAt)
	assert.WithinDuration(t, executedAt, *updated.LastExecutedAt, time.Second)
	assert.True(t, updated.UpdatedAt.After(beforeUpdate) || updated.UpdatedAt.Equal(beforeUpdate))
}

func TestErrorMonitoringStoreDeleteProjectOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "error-tracking")

	project := observabilitydomain.NewProject(workspace.ID, "", "API", "api", "go")
	require.NoError(t, store.Projects().CreateProject(ctx, project))

	require.NoError(t, store.Projects().DeleteProject(ctx, workspace.ID, project.ID))

	_, err := store.Projects().GetProject(ctx, project.ID)
	require.ErrorIs(t, err, shared.ErrNotFound)
}

func TestErrorMonitoringStoreUsesExtensionOwnedSchemaOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	installed := refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "error-tracking")

	project := observabilitydomain.NewProject(workspace.ID, "", "API", "api", "go")
	require.NoError(t, store.Projects().CreateProject(ctx, project))

	concrete, ok := store.(*sqlstore.Store)
	require.True(t, ok)

	rawDB, err := concrete.GetSQLDB()
	require.NoError(t, err)

	var extensionInstallID string
	err = rawDB.QueryRowContext(ctx, `
		SELECT extension_install_id
		FROM ext_demandops_error_tracking.projects
		WHERE id = $1
	`, project.ID).Scan(&extensionInstallID)
	require.NoError(t, err)
	assert.Equal(t, installed.ID, extensionInstallID)
}

func TestCaseStoreLinkCaseToKnowledgeResourceOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	team := &platformdomain.Team{
		WorkspaceID: workspace.ID,
		Name:        "Support",
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.Workspaces().CreateTeam(ctx, team))

	caseObj := testutil.NewIsolatedCase(t, workspace.ID)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	resource := knowledgedomain.NewKnowledgeResource(workspace.ID, team.ID, "reset-postgresql", "Reset PostgreSQL")
	resource.Kind = knowledgedomain.KnowledgeResourceKindGuide
	resource.Status = knowledgedomain.KnowledgeResourceStatusActive
	resource.BodyMarkdown = "Use the new baseline."
	require.NoError(t, store.KnowledgeResources().CreateKnowledgeResource(ctx, resource))

	require.NoError(t, store.Cases().LinkCaseToKnowledgeResource(ctx, caseObj.ID, resource.ID))

	links, err := store.Cases().GetCaseKnowledgeResourceLinks(ctx, caseObj.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, caseObj.ID, links[0].CaseID)
	assert.Equal(t, resource.ID, links[0].KnowledgeResourceID)
}

func TestEmailStoreAllowsBlankOptionalUUIDsOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	outbound := servicedomain.NewOutboundEmail(workspace.ID, "support@example.com", "Subject", "Body")
	outbound.ToEmails = []string{"customer@example.com"}
	outbound.FromName = "Support"
	require.NoError(t, store.OutboundEmails().CreateOutboundEmail(ctx, outbound))

	storedOutbound, err := store.OutboundEmails().GetOutboundEmail(ctx, outbound.ID)
	require.NoError(t, err)
	assert.Empty(t, storedOutbound.TemplateID)
	assert.Empty(t, storedOutbound.CaseID)
	assert.Empty(t, storedOutbound.ContactID)
	assert.Empty(t, storedOutbound.CommunicationID)
	assert.Empty(t, storedOutbound.UserID)
	assert.Empty(t, storedOutbound.CreatedByID)

	storedOutbound.Status = servicedomain.EmailStatusSent
	storedOutbound.UpdatedAt = time.Now().UTC()
	require.NoError(t, store.OutboundEmails().UpdateOutboundEmail(ctx, storedOutbound))

	inbound := servicedomain.NewInboundEmail(workspace.ID, "<msg-1@example.com>", "customer@example.com", "Help", "Please help")
	inbound.ToEmails = []string{"support@example.com"}
	require.NoError(t, store.InboundEmails().CreateInboundEmail(ctx, inbound))

	storedInbound, err := store.InboundEmails().GetInboundEmail(ctx, inbound.ID)
	require.NoError(t, err)
	assert.Equal(t, workspace.ID, storedInbound.WorkspaceID)
	assert.Empty(t, storedInbound.CaseID)
	assert.Empty(t, storedInbound.ContactID)
	assert.Empty(t, storedInbound.CommunicationID)
	assert.Empty(t, storedInbound.ThreadID)

	storedInbound.IsRead = true
	storedInbound.UpdatedAt = time.Now().UTC()
	require.NoError(t, store.InboundEmails().UpdateInboundEmail(ctx, storedInbound))

	now := time.Now().UTC()
	thread := &servicedomain.EmailThread{
		WorkspaceID:    workspace.ID,
		ThreadKey:      "thread-" + id.New(),
		Subject:        "Support thread",
		Type:           servicedomain.ThreadTypeConversation,
		Status:         servicedomain.ThreadStatusActive,
		Priority:       servicedomain.ThreadPriorityNormal,
		Participants:   []servicedomain.ThreadParticipant{},
		ContactIDs:     []string{},
		EmailCount:     1,
		UnreadCount:    1,
		MessageIDs:     []string{},
		FirstEmailAt:   now,
		LastEmailAt:    now,
		LastActivity:   now,
		ChildThreadIDs: []string{},
		MergedFromIDs:  []string{},
		Tags:           []string{},
		Labels:         []string{},
		CustomFields:   map[string]interface{}{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, store.EmailThreads().CreateEmailThread(ctx, thread))

	storedThread, err := store.EmailThreads().GetEmailThread(ctx, thread.ID)
	require.NoError(t, err)
	assert.Empty(t, storedThread.CaseID)
	assert.Empty(t, storedThread.LastEmailID)
	assert.Empty(t, storedThread.FirstEmailID)
	assert.Empty(t, storedThread.ParentThreadID)
	assert.Empty(t, storedThread.MergedIntoID)

	storedThread.Subject = "Updated support thread"
	storedThread.UpdatedAt = time.Now().UTC()
	require.NoError(t, store.EmailThreads().UpdateEmailThread(ctx, storedThread))
}

func TestUserStoreSaveMagicLinkWithoutUserOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	link := &platformdomain.MagicLinkToken{
		Token:     id.New(),
		Email:     testutil.UniqueEmail(t),
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}

	require.NoError(t, store.Users().SaveMagicLink(ctx, link))

	stored, err := store.Users().GetMagicLink(ctx, link.Token)
	require.NoError(t, err)
	assert.Equal(t, link.Token, stored.Token)
	assert.Equal(t, link.Email, stored.Email)
	assert.Empty(t, stored.UserID)
}
