package knowledgeservices

import (
	"context"
	"testing"
	"time"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/eventbus"

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
