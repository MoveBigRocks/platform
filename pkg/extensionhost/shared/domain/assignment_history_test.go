package shareddomain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaseAssignmentHistoryUserLifecycle(t *testing.T) {
	now := time.Now()
	history := &CaseAssignmentHistory{
		AssignedAt: now.Add(-10 * time.Minute),
	}

	history.AssignToUser("user-1", "Ada", "assignor-1", "Grace")
	assert.Equal(t, AssignmentTypeUser, history.AssignmentType)
	assert.Equal(t, "user-1", history.AssignedToUserID)
	assert.Equal(t, "Ada", history.AssignedUserName)
	assert.Equal(t, "assignor-1", history.AssignedByID)
	assert.Equal(t, "Grace", history.AssignedByName)
	assert.Equal(t, "user", history.AssignedByType)
	assert.Equal(t, AssignmentStatusActive, history.Status)

	history.Accept("user-1")
	require.True(t, history.WasAccepted)
	require.Equal(t, "user-1", history.AcceptedByID)
	require.NotNil(t, history.AcceptedAt)

	acceptedAt := now.Add(-3 * time.Minute)
	history.AcceptedAt = &acceptedAt
	history.Complete()

	assert.Equal(t, AssignmentStatusCompleted, history.Status)
	require.NotNil(t, history.CompletedAt)
	require.NotNil(t, history.Duration)
	assert.GreaterOrEqual(t, *history.Duration, 179)
}

func TestCaseAssignmentHistoryTeamTransferEscalationAndRejection(t *testing.T) {
	history := &CaseAssignmentHistory{}

	history.AssignToTeam("team-1", "Support", "assignor-1", "Grace")
	assert.Equal(t, AssignmentTypeTeam, history.AssignmentType)
	assert.Equal(t, "team-1", history.AssignedToTeamID)
	assert.Equal(t, "Support", history.AssignedTeamName)
	assert.Equal(t, AssignmentStatusActive, history.Status)

	history.Transfer("user-2", "team-2", "handoff")
	assert.Equal(t, AssignmentStatusTransferred, history.Status)
	assert.True(t, history.WasTransferred)
	assert.Equal(t, "user-2", history.TransferredToUserID)
	assert.Equal(t, "team-2", history.TransferredToTeamID)
	assert.Equal(t, "handoff", history.TransferReason)
	require.NotNil(t, history.TransferredAt)

	history.Escalate("user-3", "team-3", "sla breach")
	assert.Equal(t, AssignmentStatusEscalated, history.Status)
	assert.True(t, history.WasEscalated)
	assert.Equal(t, "user-3", history.EscalatedToUserID)
	assert.Equal(t, "team-3", history.EscalatedToTeamID)
	assert.Equal(t, "sla breach", history.EscalationReason)
	require.NotNil(t, history.EscalatedAt)

	history.Reject("out of office")
	assert.Equal(t, AssignmentStatusCanceled, history.Status)
	assert.Equal(t, "out of office", history.RejectionReason)
	require.NotNil(t, history.RejectedAt)
}

func TestCaseAssignmentHistoryContextAndNotificationHelpers(t *testing.T) {
	history := &CaseAssignmentHistory{}
	customFields := NewMetadata()
	customFields.SetString("source", "automation")
	responseTime := 45
	resolutionTime := 300
	satisfaction := 4.5
	deadline := time.Now().Add(2 * time.Hour).UTC()
	caseCreatedAt := time.Now().Add(-4 * time.Hour).UTC()

	history.SetWorkloadContext(3, 4)
	history.SetSkillContext([]string{"go", "triage"}, []string{"go"}, 0.8)
	history.SetPerformanceMetrics(&responseTime, &resolutionTime, &satisfaction)
	history.SetCaseContext("open", "high", "Customer outage", caseCreatedAt, customFields)
	history.SetSLADeadline(deadline)
	history.MarkUrgent()
	history.SetNotificationSent("email")
	history.MarkNotificationViewed()
	history.AddAlternativeCandidate(AssignmentCandidate{
		UserID:            "user-9",
		TeamID:            "team-9",
		Name:              "Jordan",
		Score:             0.62,
		Reasons:           []string{"busy", "timezone mismatch"},
		Workload:          7,
		Availability:      false,
		SkillMatch:        0.5,
		PerformanceScore:  0.7,
		NotSelected:       true,
		NotSelectedReason: "overloaded",
	})

	assert.Equal(t, 3, history.WorkloadBefore)
	assert.Equal(t, 4, history.WorkloadAfter)
	assert.Equal(t, []string{"go", "triage"}, history.RequiredSkills)
	assert.Equal(t, []string{"go"}, history.MatchedSkills)
	assert.Equal(t, 0.8, history.SkillMatchScore)
	require.NotNil(t, history.ResponseTime)
	require.NotNil(t, history.ResolutionTime)
	require.NotNil(t, history.CustomerSatisfaction)
	assert.Equal(t, 45, *history.ResponseTime)
	assert.Equal(t, 300, *history.ResolutionTime)
	assert.Equal(t, 4.5, *history.CustomerSatisfaction)
	assert.Equal(t, "open", history.CaseStatus)
	assert.Equal(t, "high", history.CasePriority)
	assert.Equal(t, "Customer outage", history.CaseSubject)
	assert.Equal(t, caseCreatedAt, history.CaseCreatedAt)
	assert.Equal(t, "automation", history.CaseCustomFields.GetString("source"))
	require.NotNil(t, history.SLADeadline)
	assert.Equal(t, deadline, *history.SLADeadline)
	assert.True(t, history.IsUrgent)
	assert.True(t, history.NotificationSent)
	assert.Equal(t, "email", history.NotificationMethod)
	require.NotNil(t, history.NotificationSentAt)
	assert.True(t, history.NotificationViewed)
	require.NotNil(t, history.NotificationViewedAt)
	require.Len(t, history.AlternativeCandidates, 1)
	assert.Equal(t, "overloaded", history.AlternativeCandidates[0].NotSelectedReason)
}
