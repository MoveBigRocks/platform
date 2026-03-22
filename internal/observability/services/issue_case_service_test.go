package observabilityservices

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/id"
)

func TestFormatIssueSubject(t *testing.T) {
	testCases := []struct {
		name        string
		issueTitle  string
		expectMatch string
	}{
		{
			"formats basic issue title",
			"NullPointerException in UserService",
			"Error affecting you: NullPointerException in UserService",
		},
		{
			"handles empty issue title",
			"",
			"Error affecting you: ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatIssueSubject(tc.issueTitle)
			assert.Equal(t, tc.expectMatch, result)
		})
	}
}

func TestFormatIssueDescription(t *testing.T) {
	testCases := []struct {
		name        string
		issueTitle  string
		issueLevel  string
		expectMatch string
	}{
		{
			"formats basic description",
			"Database connection timeout",
			"error",
			"We've detected an error that may be affecting your experience: Database connection timeout",
		},
		{
			"handles different level",
			"API rate limit exceeded",
			"warning",
			"We've detected an error that may be affecting your experience: API rate limit exceeded",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatIssueDescription(tc.issueTitle, tc.issueLevel)
			assert.Equal(t, tc.expectMatch, result)
		})
	}
}

func TestIssueCaseService_CreateCaseForIssue(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	workspaceID := testutil.CreateTestWorkspace(t, store, "issue-test")

	caseService := serviceapp.NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	svc := NewIssueCaseService(store.Cases(), caseService)
	ctx := context.Background()

	t.Run("creates case from issue with internal channel", func(t *testing.T) {
		linkedIssueID := id.New()
		params := CreateCaseForIssueParams{
			WorkspaceID:  workspaceID,
			IssueID:      linkedIssueID,
			ProjectID:    "project-xyz",
			IssueTitle:   "Payment processing error",
			IssueLevel:   "error",
			Priority:     servicedomain.CasePriorityHigh,
			ContactEmail: "affected@example.com",
		}

		caseObj, err := svc.CreateCaseForIssue(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, caseObj)

		assert.Equal(t, servicedomain.CaseChannelInternal, caseObj.Channel)
		assert.Contains(t, caseObj.Subject, "Payment processing error")
		assert.Contains(t, caseObj.Description, "Payment processing error")
		assert.Equal(t, servicedomain.CasePriorityHigh, caseObj.Priority)

		issueID, ok := caseObj.CustomFields.GetString("linked_issue_id")
		assert.True(t, ok)
		assert.Equal(t, linkedIssueID, issueID)

		projectID, ok := caseObj.CustomFields.GetString("linked_project_id")
		assert.True(t, ok)
		assert.Equal(t, "project-xyz", projectID)

		level, ok := caseObj.CustomFields.GetString("issue_level")
		assert.True(t, ok)
		assert.Equal(t, "error", level)

		source, ok := caseObj.CustomFields.GetString("source")
		assert.True(t, ok)
		assert.Equal(t, "auto_monitoring", source)
	})

	t.Run("case has linked issue in LinkedIssueIDs", func(t *testing.T) {
		linkedIssueID := id.New()
		params := CreateCaseForIssueParams{
			WorkspaceID: workspaceID,
			IssueID:     linkedIssueID,
			ProjectID:   "project-link",
			IssueTitle:  "Test error",
			IssueLevel:  "warning",
			Priority:    servicedomain.CasePriorityMedium,
		}

		caseObj, err := svc.CreateCaseForIssue(ctx, params)
		require.NoError(t, err)

		assert.Contains(t, caseObj.LinkedIssueIDs, linkedIssueID)
	})
}
