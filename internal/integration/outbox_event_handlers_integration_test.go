//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/graph/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/outbox"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/refext"
	"github.com/movebigrocks/platform/internal/workers"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/id"
	"github.com/movebigrocks/platform/pkg/logger"
)

func TestOutbox_EventHandlers_ProcessEventsEndToEnd(t *testing.T) {
	testutil.SetupTestEnv(t)

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()

	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "error-tracking")

	project := testutil.NewIsolatedProject(t, workspace.ID)
	require.NoError(t, store.Projects().CreateProject(ctx, project))

	contact := testutil.NewIsolatedContact(t, workspace.ID)
	require.NoError(t, store.Contacts().CreateContact(ctx, contact))

	caseService := serviceapp.NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	caseObj, err := caseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspace.ID,
		Subject:      "Linked case",
		Description:  "Created via outbox integration test",
		ContactEmail: contact.Email,
		Channel:      shareddomain.CaseChannelWeb,
	})
	require.NoError(t, err)

	issueID := id.New()
	issue := &observabilitydomain.Issue{
		ID:          issueID,
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Title:       "Outbox integration issue",
		Level:       observabilitydomain.ErrorLevelError,
		Status:      observabilitydomain.IssueStatusUnresolved,
		Fingerprint: "fp-" + issueID[:8],
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		EventCount:  1,
		UserCount:   1,
		Platform:    "go",
	}
	require.NoError(t, store.Issues().CreateIssue(ctx, issue))

	eventBus := eventbus.NewInMemoryBus()
	outboxService := outbox.NewServiceWithConfig(
		store,
		eventBus,
		logger.New(),
		config.OutboxConfig{
			PollInterval:     25 * time.Millisecond,
			MaxRetries:       5,
			RetentionDays:    1,
			BatchSize:        50,
			MaxBackoff:       250 * time.Millisecond,
			HealthMaxPending: 100,
			HealthMaxAge:     5 * time.Minute,
		},
	)
	manager := workers.NewManager(workers.ManagerDeps{
		EventBus:         eventBus,
		Logger:           logger.New(),
		IdempotencyStore: store.Idempotency(),
		CaseService:      caseService,
		TxRunner:         store,
	})

	outboxService.Start()
	defer outboxService.Stop(2 * time.Second)

	require.NoError(t, manager.Start(ctx))
	defer manager.Stop(2 * time.Second)

	event := shareddomain.IssueCaseLinked{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueCaseLinked),
		IssueID:     issue.ID,
		CaseID:      caseObj.ID,
		ProjectID:   project.ID,
		WorkspaceID: workspace.ID,
		ContactID:   contact.ID,
		LinkedBy:    "integration-test",
		LinkReason:  "e2e",
		LinkedAt:    time.Now(),
	}

	require.NoError(t, outboxService.PublishEvent(ctx, eventbus.StreamCaseEvents, event))

	require.Eventually(t, func() bool {
		updated, err := caseService.GetCase(ctx, caseObj.ID)
		if err != nil {
			return false
		}
		if len(updated.LinkedIssueIDs) == 0 {
			return false
		}
		return updated.LinkedIssueIDs[0] == issue.ID
	}, 15*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		outboxEvent, err := store.Outbox().GetOutboxEvent(ctx, event.EventID)
		return err == nil && outboxEvent.Status == "published"
	}, 15*time.Second, 100*time.Millisecond)

	outboxEvent, err := store.Outbox().GetOutboxEvent(ctx, event.EventID)
	require.NoError(t, err)
	require.Equal(t, "published", outboxEvent.Status)
}

func TestOutbox_ResolveIssue_PublishesBulkCaseResolution(t *testing.T) {
	testutil.SetupTestEnv(t)

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()

	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "error-tracking")

	project := testutil.NewIsolatedProject(t, workspace.ID)
	require.NoError(t, store.Projects().CreateProject(ctx, project))

	contact := testutil.NewIsolatedContact(t, workspace.ID)
	require.NoError(t, store.Contacts().CreateContact(ctx, contact))

	eventBus := eventbus.NewInMemoryBus()
	outboxService := outbox.NewServiceWithConfig(
		store,
		eventBus,
		logger.New(),
		config.OutboxConfig{
			PollInterval:     25 * time.Millisecond,
			MaxRetries:       5,
			RetentionDays:    1,
			BatchSize:        50,
			MaxBackoff:       250 * time.Millisecond,
			HealthMaxPending: 100,
			HealthMaxAge:     5 * time.Minute,
		},
	)

	caseService := serviceapp.NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), outboxService)
	issueService := observabilityservices.NewIssueService(
		store.Issues(),
		store.Projects(),
		store.ErrorEvents(),
		store.Workspaces(),
		outboxService,
	)
	manager := workers.NewManager(workers.ManagerDeps{
		EventBus:         eventBus,
		Logger:           logger.New(),
		IdempotencyStore: store.Idempotency(),
		CaseService:      caseService,
		TxRunner:         store,
	})

	outboxService.Start()
	defer outboxService.Stop(2 * time.Second)

	require.NoError(t, manager.Start(ctx))
	defer manager.Stop(2 * time.Second)

	event := observabilitydomain.NewErrorEvent(project.ID, id.New())
	event.Message = "resolve issue event"
	event.Level = "error"
	event.Environment = "production"
	issue := observabilitydomain.NewIssue(project.ID, "Resolve event integration issue", "resolveIssue", event)
	issue.WorkspaceID = workspace.ID
	issue.FirstSeen = time.Now()
	issue.LastSeen = time.Now()
	require.NoError(t, store.Issues().CreateIssue(ctx, issue))

	systemCase := servicedomain.NewCase(workspace.ID, "System case", contact.Email)
	systemCase.Source = shareddomain.SourceTypeSystem
	systemCase.IsSystemCase = true
	systemCase.GenerateHumanID("test")
	require.NoError(t, store.Cases().CreateCase(ctx, systemCase))

	customerCase := servicedomain.NewCase(workspace.ID, "Customer case", contact.Email)
	customerCase.Source = shareddomain.SourceTypeManual
	customerCase.GenerateHumanID("test")
	require.NoError(t, store.Cases().CreateCase(ctx, customerCase))

	authCtx := shared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Permissions: []string{platformdomain.PermissionIssueWrite},
	})
	_, err := issueService.LinkIssueToCase(authCtx, issue.ID, systemCase.ID)
	require.NoError(t, err)
	_, err = issueService.LinkIssueToCase(authCtx, issue.ID, customerCase.ID)
	require.NoError(t, err)
	require.NoError(t, issueService.ResolveIssue(authCtx, issue.ID, "fixed", ""))

	require.Eventually(t, func() bool {
		c, err := store.Cases().GetCase(ctx, systemCase.ID)
		if err != nil {
			return false
		}
		return c.Status == servicedomain.CaseStatusResolved && c.IssueResolved
	}, 15*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		c, err := store.Cases().GetCase(ctx, customerCase.ID)
		if err != nil {
			return false
		}
		return c.Status == servicedomain.CaseStatusResolved && c.IssueResolved
	}, 15*time.Second, 100*time.Millisecond)
}
