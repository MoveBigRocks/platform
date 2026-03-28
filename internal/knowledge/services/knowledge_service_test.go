package knowledgeservices

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicehandlers "github.com/movebigrocks/platform/internal/service/handlers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/workflowproof"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type knowledgeMockOutbox struct {
	streams []eventbus.Stream
	events  []interface{}
}

func (m *knowledgeMockOutbox) Publish(_ context.Context, stream eventbus.Stream, event interface{}) error {
	m.streams = append(m.streams, stream)
	m.events = append(m.events, event)
	return nil
}

func (m *knowledgeMockOutbox) PublishEvent(_ context.Context, stream eventbus.Stream, event eventbus.Event) error {
	m.streams = append(m.streams, stream)
	m.events = append(m.events, event)
	return nil
}

func setupTestStore(t *testing.T) (stores.Store, func()) {
	t.Helper()
	return testutil.SetupTestSQLStore(t)
}

func setupTestWorkspace(t *testing.T, store stores.Store, slug string) string {
	t.Helper()
	return testutil.CreateTestWorkspace(t, store, slug)
}

func setupTestTeam(t *testing.T, store stores.Store, workspaceID, name string) string {
	t.Helper()
	team := &platformdomain.Team{
		WorkspaceID: workspaceID,
		Name:        name,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.Workspaces().CreateTeam(context.Background(), team))
	return team.ID
}

func requireValidationErrors(t *testing.T, err error) []apierrors.ValidationError {
	t.Helper()

	var apiErr *apierrors.APIError
	require.ErrorAs(t, err, &apiErr)
	raw, ok := apiErr.Details["validation_errors"]
	require.True(t, ok)
	validationErrors, ok := raw.([]apierrors.ValidationError)
	require.True(t, ok)
	return validationErrors
}

func TestKnowledgeService_CreateAndGetKnowledgeResource(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "knowledge-create")
	teamID := setupTestTeam(t, store, workspaceID, "Support")
	service := NewKnowledgeService(store.KnowledgeResources(), store.Workspaces(), store.ConceptSpecs(), artifactservices.NewGitService(t.TempDir()), nil, store)
	ctx := context.Background()

	resource, err := service.CreateKnowledgeResource(ctx, CreateKnowledgeResourceParams{
		WorkspaceID:  workspaceID,
		TeamID:       teamID,
		Slug:         "refund-policy",
		Title:        "Refund Policy",
		Kind:         knowledgedomain.KnowledgeResourceKindPolicy,
		Status:       knowledgedomain.KnowledgeResourceStatusActive,
		BodyMarkdown: "# Refund Policy\n\nRefunds are reviewed within 3 business days.",
	})
	require.NoError(t, err)
	require.NotNil(t, resource)

	assert.NotEmpty(t, resource.ID)
	assert.Equal(t, "refund-policy", resource.Slug)
	assert.Equal(t, knowledgedomain.KnowledgeResourceKindPolicy, resource.Kind)
	assert.NotEmpty(t, resource.ContentHash)

	byID, err := service.GetKnowledgeResource(ctx, resource.ID)
	require.NoError(t, err)
	assert.Equal(t, resource.ID, byID.ID)

	bySlug, err := service.GetKnowledgeResourceBySlug(ctx, workspaceID, teamID, knowledgedomain.KnowledgeSurfacePrivate, "refund-policy")
	require.NoError(t, err)
	assert.Equal(t, resource.ID, bySlug.ID)
}

func TestKnowledgeService_ListAndUpdateKnowledgeResource(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "knowledge-list")
	teamID := setupTestTeam(t, store, workspaceID, "Support")
	service := NewKnowledgeService(store.KnowledgeResources(), store.Workspaces(), store.ConceptSpecs(), artifactservices.NewGitService(t.TempDir()), nil, store)
	ctx := context.Background()

	resource, err := service.CreateKnowledgeResource(ctx, CreateKnowledgeResourceParams{
		WorkspaceID:  workspaceID,
		TeamID:       teamID,
		Slug:         "incident-playbook",
		Title:        "Incident Playbook",
		Kind:         knowledgedomain.KnowledgeResourceKindGuide,
		Status:       knowledgedomain.KnowledgeResourceStatusDraft,
		BodyMarkdown: "# Incident Playbook\n\nStart in the incident queue.",
		SearchKeywords: []string{
			"incident",
			"playbook",
		},
	})
	require.NoError(t, err)

	updatedTitle := "Incident Response Playbook"
	updatedStatus := knowledgedomain.KnowledgeResourceStatusActive
	updatedBody := "# Incident Response Playbook\n\nPage the on-call engineer and start triage."
	updatedKeywords := []string{"incident", "on-call"}
	updated, err := service.UpdateKnowledgeResource(ctx, resource.ID, UpdateKnowledgeResourceParams{
		Title:          &updatedTitle,
		Status:         &updatedStatus,
		BodyMarkdown:   &updatedBody,
		SearchKeywords: &updatedKeywords,
	})
	require.NoError(t, err)
	assert.Equal(t, updatedTitle, updated.Title)
	assert.Equal(t, updatedStatus, updated.Status)
	assert.Equal(t, updatedKeywords, updated.SearchKeywords)

	resources, total, err := service.ListWorkspaceKnowledgeResources(ctx, workspaceID, knowledgedomain.KnowledgeResourceFilter{
		Status: knowledgedomain.KnowledgeResourceStatusActive,
		Search: "on-call",
		Limit:  10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, resources, 1)
	assert.Equal(t, updated.ID, resources[0].ID)
}

func TestKnowledgeService_NormalizesKnowledgeKinds(t *testing.T) {
	t.Parallel()

	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "knowledge-kind-normalize")
	teamID := setupTestTeam(t, store, workspaceID, "Marketing")
	service := NewKnowledgeService(store.KnowledgeResources(), store.Workspaces(), store.ConceptSpecs(), artifactservices.NewGitService(t.TempDir()), nil, store)
	ctx := context.Background()

	resource, err := service.CreateKnowledgeResource(ctx, CreateKnowledgeResourceParams{
		WorkspaceID:  workspaceID,
		TeamID:       teamID,
		Slug:         "homepage-template",
		Title:        "Homepage Template",
		Kind:         knowledgedomain.KnowledgeResourceKind("templates"),
		BodyMarkdown: "# Homepage Template\n\nUse this when launching a campaign page.",
	})
	require.NoError(t, err)
	assert.Equal(t, knowledgedomain.KnowledgeResourceKindTemplate, resource.Kind)

	updatedKind := knowledgedomain.KnowledgeResourceKind("best practices")
	resource, err = service.UpdateKnowledgeResource(ctx, resource.ID, UpdateKnowledgeResourceParams{
		Kind: &updatedKind,
	})
	require.NoError(t, err)
	assert.Equal(t, knowledgedomain.KnowledgeResourceKindBestPractice, resource.Kind)
}

func TestKnowledgeService_PublishesRFCReviewSignals(t *testing.T) {
	t.Parallel()

	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "knowledge-rfc-review")
	teamID := setupTestTeam(t, store, workspaceID, "Marketing")
	now := time.Now().UTC()

	author, err := platformdomain.NewManagedUser("author@example.com", "Author", nil, now)
	require.NoError(t, err)
	require.NoError(t, store.Users().CreateUser(context.Background(), author))
	reviewer, err := platformdomain.NewManagedUser("reviewer@example.com", "Reviewer", nil, now)
	require.NoError(t, err)
	require.NoError(t, store.Users().CreateUser(context.Background(), reviewer))
	require.NoError(t, store.Workspaces().AddTeamMember(context.Background(), &platformdomain.TeamMember{
		TeamID:      teamID,
		UserID:      author.ID,
		WorkspaceID: workspaceID,
		Role:        platformdomain.TeamMemberRoleLead,
		IsActive:    true,
		JoinedAt:    now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}))
	require.NoError(t, store.Workspaces().AddTeamMember(context.Background(), &platformdomain.TeamMember{
		TeamID:      teamID,
		UserID:      reviewer.ID,
		WorkspaceID: workspaceID,
		Role:        platformdomain.TeamMemberRoleMember,
		IsActive:    true,
		JoinedAt:    now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}))

	outbox := &knowledgeMockOutbox{}
	service := NewKnowledgeService(store.KnowledgeResources(), store.Workspaces(), store.ConceptSpecs(), artifactservices.NewGitService(t.TempDir()), outbox, store)
	ctx := context.Background()

	_, err = service.CreateKnowledgeResource(ctx, CreateKnowledgeResourceParams{
		WorkspaceID:  workspaceID,
		TeamID:       teamID,
		Slug:         "queue-routing-rfc",
		Title:        "Queue Routing RFC",
		Kind:         knowledgedomain.KnowledgeResourceKindDecision,
		BodyMarkdown: "# Queue Routing RFC\n\nWe should split launch forms from general support.",
		CreatedBy:    author.ID,
	})
	require.NoError(t, err)

	require.Len(t, outbox.events, 3)
	assert.Equal(t, eventbus.StreamKnowledgeEvents, outbox.streams[0])
	assert.Equal(t, eventbus.StreamKnowledgeEvents, outbox.streams[1])
	assert.Equal(t, eventbus.StreamNotificationCommands, outbox.streams[2])

	_, ok := outbox.events[0].(shareddomain.KnowledgeCreated)
	assert.True(t, ok)
	_, ok = outbox.events[1].(shareddomain.KnowledgeReviewRequested)
	assert.True(t, ok)
	notification, ok := outbox.events[2].(sharedevents.SendNotificationRequestedEvent)
	require.True(t, ok)
	assert.Equal(t, []string{reviewer.ID}, notification.Recipients)
	assert.Equal(t, "knowledge_review", notification.SourceType)
}

func TestKnowledgeReviewWorkflow_PersistsInAppNotification(t *testing.T) {
	t.Parallel()

	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "knowledge-rfc-notification")
	teamID := setupTestTeam(t, store, workspaceID, "Marketing")
	now := time.Now().UTC()

	author, err := platformdomain.NewManagedUser("author@example.com", "Author", nil, now)
	require.NoError(t, err)
	require.NoError(t, store.Users().CreateUser(context.Background(), author))
	reviewer, err := platformdomain.NewManagedUser("reviewer@example.com", "Reviewer", nil, now)
	require.NoError(t, err)
	require.NoError(t, store.Users().CreateUser(context.Background(), reviewer))
	require.NoError(t, store.Workspaces().CreateUserWorkspaceRole(context.Background(), &platformdomain.UserWorkspaceRole{
		ID:          testutil.UniqueID("uwr"),
		UserID:      author.ID,
		WorkspaceID: workspaceID,
		Role:        platformdomain.WorkspaceRoleMember,
		CreatedAt:   now,
		UpdatedAt:   now,
	}))
	require.NoError(t, store.Workspaces().CreateUserWorkspaceRole(context.Background(), &platformdomain.UserWorkspaceRole{
		ID:          testutil.UniqueID("uwr"),
		UserID:      reviewer.ID,
		WorkspaceID: workspaceID,
		Role:        platformdomain.WorkspaceRoleMember,
		CreatedAt:   now,
		UpdatedAt:   now,
	}))
	require.NoError(t, store.Workspaces().AddTeamMember(context.Background(), &platformdomain.TeamMember{
		TeamID:      teamID,
		UserID:      author.ID,
		WorkspaceID: workspaceID,
		Role:        platformdomain.TeamMemberRoleLead,
		IsActive:    true,
		JoinedAt:    now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}))
	require.NoError(t, store.Workspaces().AddTeamMember(context.Background(), &platformdomain.TeamMember{
		TeamID:      teamID,
		UserID:      reviewer.ID,
		WorkspaceID: workspaceID,
		Role:        platformdomain.TeamMemberRoleMember,
		IsActive:    true,
		JoinedAt:    now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}))

	outbox := &knowledgeMockOutbox{}
	service := NewKnowledgeService(store.KnowledgeResources(), store.Workspaces(), store.ConceptSpecs(), artifactservices.NewGitService(t.TempDir()), outbox, store)
	notificationService := serviceapp.NewNotificationService(store, nil, logger.NewNop())
	notificationHandler := servicehandlers.NewNotificationCommandHandler(notificationService, logger.NewNop())
	ctx := context.Background()

	resource, err := service.CreateKnowledgeResource(ctx, CreateKnowledgeResourceParams{
		WorkspaceID:        workspaceID,
		TeamID:             teamID,
		Slug:               "queue-routing-rfc",
		Title:              "Queue Routing RFC",
		Kind:               knowledgedomain.KnowledgeResourceKindDecision,
		BodyMarkdown:       "# Queue Routing RFC\n\nWe should split launch forms from general support.",
		CreatedBy:          author.ID,
		ConceptSpecKey:     "core/rfc",
		ConceptSpecVersion: "1",
	})
	require.NoError(t, err)

	var notificationEvent sharedevents.SendNotificationRequestedEvent
	for _, event := range outbox.events {
		candidate, ok := event.(sharedevents.SendNotificationRequestedEvent)
		if !ok {
			continue
		}
		notificationEvent = candidate
	}
	require.NotEmpty(t, notificationEvent.EventID)

	payload, err := json.Marshal(notificationEvent)
	require.NoError(t, err)
	require.NoError(t, notificationHandler.HandleSendNotificationRequested(ctx, payload))

	notifications, err := store.Notifications().ListUserNotifications(ctx, workspaceID, reviewer.ID)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	assert.Equal(t, resource.ID, notifications[0].EntityID)
	assert.Equal(t, "New RFC: Queue Routing RFC", notifications[0].Subject)

	workflowproof.WriteJSON(t, "knowledge-review-notification", map[string]interface{}{
		"workspace_id":      workspaceID,
		"knowledge_id":      resource.ID,
		"recipient_user_id": reviewer.ID,
		"notification_id":   notifications[0].ID,
		"subject":           notifications[0].Subject,
	})
}

func TestKnowledgeService_EnforcesCustomConceptSpecStructure(t *testing.T) {
	t.Parallel()

	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "knowledge-custom-concept")
	teamID := setupTestTeam(t, store, workspaceID, "Operations")
	ctx := context.Background()

	conceptService := NewConceptSpecService(store.ConceptSpecs(), store.Workspaces(), artifactservices.NewGitService(t.TempDir()))
	_, err := conceptService.RegisterConceptSpec(ctx, RegisterConceptSpecParams{
		WorkspaceID:  workspaceID,
		OwnerTeamID:  teamID,
		Key:          "ops/runbook",
		Version:      "1",
		Name:         "Ops Runbook",
		InstanceKind: knowledgedomain.KnowledgeResourceKindGuide,
		MetadataSchema: shareddomain.TypedSchemaFromMap(map[string]interface{}{
			"required": []string{"owner"},
		}),
		SectionsSchema: shareddomain.TypedSchemaFromMap(map[string]interface{}{
			"required": []string{"summary", "steps", "references"},
		}),
		WorkflowSchema: shareddomain.TypedSchemaFromMap(map[string]interface{}{
			"states": []string{"draft", "reviewed", "approved", "archived"},
		}),
		Status: knowledgedomain.ConceptSpecStatusActive,
	})
	require.NoError(t, err)

	service := NewKnowledgeService(store.KnowledgeResources(), store.Workspaces(), store.ConceptSpecs(), artifactservices.NewGitService(t.TempDir()), nil, store)

	_, err = service.CreateKnowledgeResource(ctx, CreateKnowledgeResourceParams{
		WorkspaceID:        workspaceID,
		TeamID:             teamID,
		Slug:               "incident-runbook",
		Title:              "Incident Runbook",
		Kind:               knowledgedomain.KnowledgeResourceKindGuide,
		ConceptSpecKey:     "ops/runbook",
		ConceptSpecVersion: "1",
		Summary:            "How incidents are handled",
		BodyMarkdown:       "## Steps\n\nPage the on-call engineer.",
	})
	require.Error(t, err)

	validationErrors := requireValidationErrors(t, err)
	require.Len(t, validationErrors, 2)
	assert.Contains(t, validationErrors[0].Message+validationErrors[1].Message, "owner")
	assert.Contains(t, validationErrors[0].Message+validationErrors[1].Message, "references")

	resource, err := service.CreateKnowledgeResource(ctx, CreateKnowledgeResourceParams{
		WorkspaceID:        workspaceID,
		TeamID:             teamID,
		Slug:               "incident-runbook",
		Title:              "Incident Runbook",
		Kind:               knowledgedomain.KnowledgeResourceKindGuide,
		ConceptSpecKey:     "ops/runbook",
		ConceptSpecVersion: "1",
		Summary:            "How incidents are handled",
		BodyMarkdown:       "## Steps\n\nPage the on-call engineer.\n\n## References\n\n- @team/on-call",
		Frontmatter: shareddomain.TypedSchemaFromMap(map[string]interface{}{
			"owner": "ops",
		}),
	})
	require.NoError(t, err)
	assert.Equal(t, "incident-runbook", resource.Slug)
}

func TestKnowledgeService_EnforcesCustomConceptSpecWorkflow(t *testing.T) {
	t.Parallel()

	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "knowledge-custom-workflow")
	teamID := setupTestTeam(t, store, workspaceID, "Operations")
	ctx := context.Background()

	conceptService := NewConceptSpecService(store.ConceptSpecs(), store.Workspaces(), artifactservices.NewGitService(t.TempDir()))
	_, err := conceptService.RegisterConceptSpec(ctx, RegisterConceptSpecParams{
		WorkspaceID:  workspaceID,
		OwnerTeamID:  teamID,
		Key:          "ops/approval-only",
		Version:      "1",
		Name:         "Approval Only",
		InstanceKind: knowledgedomain.KnowledgeResourceKindGuide,
		WorkflowSchema: shareddomain.TypedSchemaFromMap(map[string]interface{}{
			"states": []string{"draft", "approved", "archived"},
		}),
		Status: knowledgedomain.ConceptSpecStatusActive,
	})
	require.NoError(t, err)

	service := NewKnowledgeService(store.KnowledgeResources(), store.Workspaces(), store.ConceptSpecs(), artifactservices.NewGitService(t.TempDir()), nil, store)

	resource, err := service.CreateKnowledgeResource(ctx, CreateKnowledgeResourceParams{
		WorkspaceID:        workspaceID,
		TeamID:             teamID,
		Slug:               "approval-only-guide",
		Title:              "Approval Only Guide",
		Kind:               knowledgedomain.KnowledgeResourceKindGuide,
		ConceptSpecKey:     "ops/approval-only",
		ConceptSpecVersion: "1",
		BodyMarkdown:       "# Approval Only Guide",
	})
	require.NoError(t, err)

	_, err = service.ReviewKnowledgeResource(ctx, resource.ID, "reviewer_123", knowledgedomain.KnowledgeReviewStatusReviewed)
	require.Error(t, err)
	validationErrors := requireValidationErrors(t, err)
	require.Len(t, validationErrors, 1)
	assert.Contains(t, validationErrors[0].Message, "draft")
	assert.Contains(t, validationErrors[0].Message, "approved")
	assert.Contains(t, validationErrors[0].Message, "archived")

	published, err := service.PublishKnowledgeResource(ctx, resource.ID, "reviewer_123", knowledgedomain.KnowledgeSurfacePublished)
	require.NoError(t, err)
	assert.Equal(t, knowledgedomain.KnowledgeReviewStatusApproved, published.ReviewStatus)
}
