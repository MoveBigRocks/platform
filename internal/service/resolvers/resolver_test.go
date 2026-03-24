package resolvers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/graph/model"
	storeshared "github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

type testServiceIssueProvider struct {
	issuesByID    []*observabilitydomain.Issue
	issueEvents   []*observabilitydomain.ErrorEvent
	projectIssues []*observabilitydomain.Issue
	projectTotal  int
}

func (p testServiceIssueProvider) GetIssuesByIDs(_ context.Context, _ []string) ([]*observabilitydomain.Issue, error) {
	return p.issuesByID, nil
}

func (p testServiceIssueProvider) GetIssueEvents(_ context.Context, _ string, _ int) ([]*observabilitydomain.ErrorEvent, error) {
	return p.issueEvents, nil
}

func (p testServiceIssueProvider) ListIssues(_ context.Context, _ storeshared.IssueFilters) ([]*observabilitydomain.Issue, int, error) {
	return p.projectIssues, p.projectTotal, nil
}

type testServiceProjectProvider struct {
	project *observabilitydomain.Project
}

func (p testServiceProjectProvider) GetProject(_ context.Context, _ string) (*observabilitydomain.Project, error) {
	return p.project, nil
}

type testExtensionChecker struct {
	enabled bool
}

func (c testExtensionChecker) HasActiveExtensionInWorkspace(_ context.Context, _ string, _ string) (bool, error) {
	return c.enabled, nil
}

func TestCaseResolverLinkedIssuesRespectsExtensionState(t *testing.T) {
	issue := &observabilitydomain.Issue{
		ID:          "issue_1",
		WorkspaceID: "ws_1",
		ProjectID:   "proj_1",
		Title:       "Broken checkout",
		Status:      "unresolved",
		Level:       "error",
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}

	caseObj := servicedomain.NewCase("ws_1", "Need help", "person@example.com")
	caseObj.LinkedIssueIDs = []string{"issue_1"}

	disabledResolver := &Resolver{
		issueService:     testServiceIssueProvider{issuesByID: []*observabilitydomain.Issue{issue}},
		extensionChecker: testExtensionChecker{enabled: false},
	}
	disabled, err := (&CaseResolver{case_: caseObj, r: disabledResolver}).LinkedIssues(context.Background())
	require.NoError(t, err)
	require.Empty(t, disabled)

	enabledResolver := &Resolver{
		issueService:     testServiceIssueProvider{issuesByID: []*observabilitydomain.Issue{issue}},
		extensionChecker: testExtensionChecker{enabled: true},
	}
	enabled, err := (&CaseResolver{case_: caseObj, r: enabledResolver}).LinkedIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, enabled, 1)
	assert.Equal(t, model.ID("issue_1"), enabled[0].ID())
}

func TestServiceProjectResolverIssuesRespectsExtensionState(t *testing.T) {
	project := &observabilitydomain.Project{
		ID:          "proj_1",
		WorkspaceID: "ws_1",
		Name:        "Storefront",
		Slug:        "storefront",
		DSN:         "dsn",
		CreatedAt:   time.Now(),
	}
	issue := &observabilitydomain.Issue{
		ID:          "issue_1",
		WorkspaceID: "ws_1",
		ProjectID:   "proj_1",
		Title:       "Broken checkout",
		Status:      "unresolved",
		Level:       "error",
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}

	disabledResolver := &Resolver{
		issueService:     testServiceIssueProvider{projectIssues: []*observabilitydomain.Issue{issue}, projectTotal: 1},
		extensionChecker: testExtensionChecker{enabled: false},
	}
	disabled, err := (&ServiceProjectResolver{project: project, r: disabledResolver}).Issues(context.Background(), struct {
		Filter *model.IssueFilterInput
	}{})
	require.NoError(t, err)
	require.Zero(t, disabled.TotalCount())

	enabledResolver := &Resolver{
		issueService:     testServiceIssueProvider{projectIssues: []*observabilitydomain.Issue{issue}, projectTotal: 1},
		extensionChecker: testExtensionChecker{enabled: true},
	}
	enabled, err := (&ServiceProjectResolver{project: project, r: enabledResolver}).Issues(context.Background(), struct {
		Filter *model.IssueFilterInput
	}{})
	require.NoError(t, err)
	require.Equal(t, int32(1), enabled.TotalCount())
}

func TestServiceIssueResolverEventsRespectsExtensionState(t *testing.T) {
	issue := &observabilitydomain.Issue{
		ID:          "issue_1",
		WorkspaceID: "ws_1",
		ProjectID:   "proj_1",
		Title:       "Broken checkout",
		Status:      "unresolved",
		Level:       "error",
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}
	event := &observabilitydomain.ErrorEvent{
		ID:        "evt_1",
		IssueID:   "issue_1",
		EventID:   "sentry-evt",
		Timestamp: time.Now(),
		Message:   "boom",
	}

	disabledResolver := &Resolver{
		issueService:     testServiceIssueProvider{issueEvents: []*observabilitydomain.ErrorEvent{event}},
		extensionChecker: testExtensionChecker{enabled: false},
	}
	disabled, err := (&ServiceIssueResolver{issue: issue, r: disabledResolver}).Events(context.Background(), struct {
		First *int32
		After *string
	}{})
	require.NoError(t, err)
	require.Zero(t, disabled.TotalCount())

	enabledResolver := &Resolver{
		issueService:     testServiceIssueProvider{issueEvents: []*observabilitydomain.ErrorEvent{event}},
		extensionChecker: testExtensionChecker{enabled: true},
	}
	enabled, err := (&ServiceIssueResolver{issue: issue, r: enabledResolver}).Events(context.Background(), struct {
		First *int32
		After *string
	}{})
	require.NoError(t, err)
	require.Equal(t, int32(1), enabled.TotalCount())
}

func TestCanAccessKnowledgeResourceSupportsWorkspaceWideAndSharedTeamVisibility(t *testing.T) {
	t.Parallel()

	workspaceMember := &platformdomain.AuthContext{
		WorkspaceID:  "ws_1",
		WorkspaceIDs: []string{"ws_1"},
		Membership: &platformdomain.WorkspaceMembership{
			Constraints: platformdomain.MembershipConstraints{AllowedTeamIDs: []string{"team_peer"}},
		},
	}
	workspaceWide := &knowledgedomain.KnowledgeResource{
		WorkspaceID: "ws_1",
		OwnerTeamID: "team_owner",
		Surface:     knowledgedomain.KnowledgeSurfaceWorkspaceWide,
	}
	require.True(t, canAccessKnowledgeResource(workspaceMember, workspaceWide))

	sharedTeamMember := &platformdomain.AuthContext{
		WorkspaceID:  "ws_1",
		WorkspaceIDs: []string{"ws_1"},
		Membership: &platformdomain.WorkspaceMembership{
			Constraints: platformdomain.MembershipConstraints{AllowedTeamIDs: []string{"team_peer"}},
		},
	}
	sharedResource := &knowledgedomain.KnowledgeResource{
		WorkspaceID:       "ws_1",
		OwnerTeamID:       "team_owner",
		SharedWithTeamIDs: []string{"team_peer"},
		Surface:           knowledgedomain.KnowledgeSurfacePrivate,
	}
	require.True(t, canAccessKnowledgeResource(sharedTeamMember, sharedResource))

	privateResource := &knowledgedomain.KnowledgeResource{
		WorkspaceID: "ws_1",
		OwnerTeamID: "team_owner",
		Surface:     knowledgedomain.KnowledgeSurfacePrivate,
	}
	require.False(t, canAccessKnowledgeResource(workspaceMember, privateResource))
}
