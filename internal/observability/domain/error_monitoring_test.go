package observabilitydomain

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAlertCooldownEvaluation(t *testing.T) {
	alert := &Alert{CooldownMinutes: 30}
	require.False(t, alert.IsInCooldownPeriod())
	require.True(t, alert.CanEvaluate())

	lastTriggered := time.Now().Add(-10 * time.Minute)
	alert.LastTriggered = &lastTriggered
	require.True(t, alert.IsInCooldownPeriod())
	require.False(t, alert.CanEvaluate())

	expired := time.Now().Add(-time.Hour)
	alert.LastTriggered = &expired
	require.False(t, alert.IsInCooldownPeriod())
	require.True(t, alert.CanEvaluate())
}

func TestIssueLifecycleAndCaseLinking(t *testing.T) {
	event := NewErrorEvent("project_1", "event_1")
	event.Level = ErrorLevelError
	event.Timestamp = time.Now().Add(-2 * time.Hour)
	issue := NewIssue("project_1", "Boom", "handler", event)

	now := time.Now()
	require.NoError(t, issue.MarkResolved(now, "user_1", "wont_fix"))
	require.True(t, issue.IsResolved())
	require.Equal(t, "wont_fix", issue.Resolution)
	require.EqualError(t, issue.MarkResolved(now, "user_1", "fixed"), "issue is already resolved")

	require.NoError(t, issue.Reopen())
	require.Equal(t, IssueStatusUnresolved, issue.Status)

	issue.MarkIgnored()
	require.True(t, issue.IsMuted())
	require.NoError(t, issue.Unmute())
	require.Equal(t, IssueStatusUnresolved, issue.Status)

	issue.MarkMuted()
	require.True(t, issue.IsMuted())
	require.NoError(t, issue.SetStatus(IssueStatusUnresolved, now, "user_1"))
	require.Equal(t, IssueStatusUnresolved, issue.Status)
	require.NoError(t, issue.SetStatus(IssueStatusResolved, now, "user_1"))
	require.True(t, issue.IsResolved())
	require.EqualError(t, issue.SetStatus("bad", now, "user_1"), "invalid issue status: bad")

	issue.Assign("owner_1")
	require.Equal(t, "owner_1", issue.AssignedTo)
	issue.Unassign()
	require.Empty(t, issue.AssignedTo)

	issue.RecordEvent(now, "event_2", "user_2")
	require.EqualValues(t, 2, issue.EventCount)
	require.EqualValues(t, 2, issue.UserCount)
	require.Equal(t, "event_2", issue.LastEventID)

	issue.UpdateLastSeen(now.Add(time.Minute), "event_3")
	require.Equal(t, "event_3", issue.LastEventID)

	issue.LinkCase("case_1")
	issue.LinkCase("case_1")
	require.True(t, issue.HasRelatedCase)
	require.Equal(t, []string{"case_1"}, issue.RelatedCaseIDs)

	issue.UnlinkCase("case_1")
	require.False(t, issue.HasRelatedCase)
	require.Empty(t, issue.RelatedCaseIDs)

	issue.LinkCase("case_2")
	issue.ClearCaseLinks()
	require.False(t, issue.HasRelatedCase)
	require.Nil(t, issue.RelatedCaseIDs)

	issue.SetResolutionNotes("rolled out fix")
	require.Equal(t, "rolled out fix", issue.ResolutionNotes)
}

func TestProjectDefaultsAndQuotaHelpers(t *testing.T) {
	project := NewProject("workspace_1", "team_1", "API", "api", "go")
	require.Equal(t, "workspace_1", project.WorkspaceID)
	require.Equal(t, "team_1", project.TeamID)
	require.Equal(t, "API", project.Name)
	require.Equal(t, "api", project.Slug)
	require.Equal(t, "go", project.Platform)
	require.Equal(t, ProjectStatusActive, project.Status)
	require.Len(t, project.PublicKey, 32)
	require.Len(t, project.SecretKey, 64)
	require.Len(t, project.AppKey, 32)
	require.Contains(t, project.DSN, project.PublicKey)
	require.Contains(t, project.DSN, fmt.Sprintf("/%d", project.ProjectNumber))
	require.True(t, project.IsActive())
	require.True(t, project.WithinQuota(project.EventsPerHour-1))
	require.False(t, project.WithinQuota(project.EventsPerHour))

	project.Status = ProjectStatusPaused
	require.False(t, project.IsActive())
}

func TestErrorMonitoringGeneratorsAndStorageHelpers(t *testing.T) {
	require.Len(t, generateRandomKey(32), 32)
	require.NotZero(t, generateProjectNumber())

	event := NewErrorEvent("project_1", "event_1")
	require.NotNil(t, event.Tags)
	require.False(t, event.Timestamp.IsZero())
	require.False(t, event.Received.IsZero())

	event.Timestamp = time.Date(2026, time.March, 16, 12, 0, 0, 0, time.UTC)
	require.Equal(t, "events/2026/03/16/project_1/event_1.json", event.GetStoragePath())

	event.Size = 11 * 1024
	require.True(t, event.ShouldStore())

	event.Size = 128
	event.Breadcrumbs = make([]Breadcrumb, 51)
	require.True(t, event.ShouldStore())

	event.Breadcrumbs = make([]Breadcrumb, 50)
	require.False(t, event.ShouldStore())
}

func TestIssueResolvedInVersion(t *testing.T) {
	event := NewErrorEvent("project_1", "event_1")
	event.Level = ErrorLevelError
	issue := NewIssue("project_1", "Boom", "handler", event)

	now := time.Now()
	require.NoError(t, issue.MarkResolvedInVersion(now, "user_1", "v1.2.3", "abc123"))
	require.True(t, issue.IsResolved())
	require.Equal(t, "v1.2.3", issue.ResolvedInVersion)
	require.Equal(t, "abc123", issue.ResolvedInCommit)
	require.EqualError(t, issue.MarkResolvedInVersion(now, "user_1", "v1.2.4", "def456"), "issue is already resolved")
}

func TestIssueSetStatusBranches(t *testing.T) {
	event := NewErrorEvent("project_1", "event_1")
	event.Level = ErrorLevelError
	now := time.Now()

	ignored := NewIssue("project_1", "Boom", "handler", event)
	require.NoError(t, ignored.SetStatus(IssueStatusIgnored, now, "user_1"))
	require.Equal(t, IssueStatusIgnored, ignored.Status)

	muted := NewIssue("project_1", "Boom", "handler", event)
	require.NoError(t, muted.SetStatus(IssueStatusMuted, now, "user_1"))
	require.Equal(t, IssueStatusMuted, muted.Status)

	defaulted := NewIssue("project_1", "Boom", "handler", event)
	defaulted.Status = "custom"
	require.NoError(t, defaulted.SetStatus("", now, "user_1"))
	require.Equal(t, IssueStatusUnresolved, defaulted.Status)
}
