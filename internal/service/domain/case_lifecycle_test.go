package servicedomain

import (
	"testing"
	"time"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCaseWithDefaultsAndCustomFields(t *testing.T) {
	c := NewCaseWithDefaults(NewCaseParams{
		WorkspaceID: "ws_123",
		Subject:     "Case subject",
		ContactID:   "contact_1",
	})

	require.Equal(t, "ws_123", c.WorkspaceID)
	require.Equal(t, CaseStatusNew, c.Status)
	require.Equal(t, CasePriorityMedium, c.Priority)
	require.Equal(t, CaseChannelAPI, c.Channel)
	require.NotNil(t, c.Tags)
	require.NotNil(t, c.CustomFields)

	c.SetCustomString("source", "email")
	c.SetCustomInt("count", 3)
	c.SetCustomBool("auto", true)

	source, ok := c.GetCustomString("source")
	require.True(t, ok)
	require.Equal(t, "email", source)

	count, ok := c.GetCustomInt("count")
	require.True(t, ok)
	require.EqualValues(t, 3, count)

	auto, ok := c.GetCustomBool("auto")
	require.True(t, ok)
	require.True(t, auto)
}

func TestCaseValidateAndIssueLinkLifecycle(t *testing.T) {
	c := NewCase("ws_123", "Broken import", "user@example.com")
	require.NoError(t, c.Validate())

	c.WorkspaceID = ""
	require.EqualError(t, c.Validate(), "workspace_id is required")
	c.WorkspaceID = "ws_123"

	c.Status = CaseStatus("bad")
	require.EqualError(t, c.Validate(), "invalid status: bad")
	c.Status = CaseStatusNew

	require.EqualError(t, c.LinkIssue("", "project_1"), "issue_id cannot be empty")
	require.NoError(t, c.LinkIssue("issue_1", "project_1"))
	require.NoError(t, c.LinkIssue("issue_1", "project_1"))
	require.Equal(t, []string{"issue_1"}, c.LinkedIssueIDs)
	linkedIssueID, ok := c.CustomFields.GetString("linked_issue_id")
	require.True(t, ok)
	require.Equal(t, "issue_1", linkedIssueID)
	linkedProjectID, ok := c.CustomFields.GetString("linked_project_id")
	require.True(t, ok)
	require.Equal(t, "project_1", linkedProjectID)

	c.UnlinkIssue("issue_1")
	require.Empty(t, c.LinkedIssueIDs)
	_, exists := c.CustomFields.Get("linked_issue_id")
	require.False(t, exists)
}

func TestCaseStatusAndAssignmentTransitions(t *testing.T) {
	createdAt := time.Now().Add(-2 * time.Hour)
	resolvedAt := createdAt.Add(90 * time.Minute)
	closedAt := resolvedAt.Add(15 * time.Minute)

	c := NewCase("ws_123", "Escalation", "user@example.com")
	c.CreatedAt = createdAt

	require.NoError(t, c.Assign("user_1", "team_1"))
	require.Equal(t, "user_1", c.AssignedToID)
	require.Equal(t, "team_1", c.TeamID)
	require.Equal(t, CaseStatusOpen, c.Status)

	c.Unassign()
	require.Empty(t, c.AssignedToID)

	require.EqualError(t, c.Assign("", ""), "must provide either user_id or team_id")
	require.NoError(t, c.SetPriority(CasePriorityHigh))
	require.Equal(t, CasePriorityHigh, c.Priority)
	require.EqualError(t, c.SetPriority(CasePriority("invalid")), "invalid priority: invalid")

	require.NoError(t, c.MarkResolved(resolvedAt))
	require.Equal(t, CaseStatusResolved, c.Status)
	require.NotNil(t, c.ResolvedAt)
	require.Equal(t, 90, c.ResolutionTimeMinutes)
	require.EqualError(t, c.MarkResolved(resolvedAt), "case is already resolved")

	require.NoError(t, c.MarkClosed(closedAt))
	require.Equal(t, CaseStatusClosed, c.Status)
	require.EqualError(t, c.MarkClosed(closedAt), "case is already closed")

	require.NoError(t, c.Reopen())
	require.Equal(t, CaseStatusOpen, c.Status)
	require.Equal(t, 1, c.ReopenCount)
	require.Nil(t, c.ResolvedAt)
	require.Nil(t, c.ClosedAt)

	require.EqualError(t, c.SetStatus(CaseStatusClosed), "can only close a resolved case")
	require.NoError(t, c.SetStatus(CaseStatusResolved))
	require.Equal(t, CaseStatusResolved, c.Status)
	require.NoError(t, c.SetStatus(CaseStatusClosed))
	require.Equal(t, CaseStatusClosed, c.Status)
	require.EqualError(t, c.SetStatus(CaseStatusResolved), "cannot resolve a closed case, reopen it first")
	require.EqualError(t, c.SetStatus(CaseStatus("bad")), "invalid status: bad")
}

func TestCaseNotificationsTagsAndAutomationHelpers(t *testing.T) {
	now := time.Now()
	c := NewCase("ws_123", "Automation", "user@example.com")

	c.NotifyContact(now, "resolved_template")
	require.True(t, c.ContactNotified)
	require.NotNil(t, c.ContactNotifiedAt)
	require.Equal(t, "resolved_template", c.NotificationTemplate)

	c.MarkIssueResolved(now)
	require.True(t, c.IssueResolved)
	require.NotNil(t, c.IssueResolvedAt)

	c.MarkAsAutoCreated("automation_rule", "issue_root")
	require.True(t, c.AutoCreated)
	require.Equal(t, shareddomain.SourceType("automation_rule"), c.Source)
	require.Equal(t, "issue_root", c.RootCauseIssueID)

	c.CreatedAt = now.Add(-30 * time.Minute)
	c.RecordFirstResponse(now)
	require.NotNil(t, c.FirstResponseAt)
	require.Equal(t, 30, c.ResponseTimeMinutes)

	firstResponseAt := c.FirstResponseAt
	c.RecordFirstResponse(now.Add(10 * time.Minute))
	require.Same(t, firstResponseAt, c.FirstResponseAt)

	require.EqualError(t, c.AddTag(""), "tag cannot be empty")
	require.NoError(t, c.AddTag("vip"))
	require.NoError(t, c.AddTag("vip"))
	require.True(t, c.HasTag("vip"))
	require.Len(t, c.Tags, 1)
	require.EqualError(t, c.RemoveTag(""), "tag cannot be empty")
	require.NoError(t, c.RemoveTag("vip"))
	require.False(t, c.HasTag("vip"))

	c.SetCategory("billing")
	require.Equal(t, "billing", c.Category)

	c.Status = CaseStatusResolved
	require.True(t, c.AutoClose())
	require.Equal(t, CaseStatusClosed, c.Status)
	require.True(t, c.HasTag("auto-closed"))

	c.Status = CaseStatusOpen
	c.TransitionAfterAgentReply()
	require.Equal(t, CaseStatusPending, c.Status)
	c.IncrementMessageCount()
	require.Equal(t, 1, c.MessageCount)
}

func TestCommunicationDirectionHelpers(t *testing.T) {
	comm := NewCommunication("case_1", "ws_1", shareddomain.CommTypeNote, "body")
	require.False(t, comm.IsInbound())
	require.False(t, comm.IsOutbound())

	comm.SetDirection(shareddomain.DirectionInbound)
	require.True(t, comm.IsInbound())
	require.False(t, comm.IsInternal)

	comm.FromUserID = "user_1"
	require.True(t, comm.IsFromHuman())

	agentComm := NewAgentCommunication("case_1", "ws_1", "agent_1", shareddomain.CommTypeEmail, "body")
	require.True(t, agentComm.IsFromAgent())
	require.False(t, agentComm.IsFromHuman())
	assert.Equal(t, shareddomain.DirectionInternal, agentComm.Direction)
}
