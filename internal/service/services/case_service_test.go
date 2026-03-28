package serviceapp

import (
	"context"
	"testing"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/id"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestStore creates a SQLite store for testing.
// Uses testutil.SetupTestSQLStore for configuration.
func setupTestStore(t *testing.T) (stores.Store, func()) {
	t.Helper()
	return testutil.SetupTestSQLStore(t)
}

// setupTestWorkspace creates a workspace in the store and returns its ID.
// This should be called before creating cases.
func setupTestWorkspace(t *testing.T, store stores.Store, slug string) string {
	t.Helper()
	return testutil.CreateTestWorkspace(t, store, slug)
}

func TestCaseService_CreateCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "test-1")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Creates case with NEW status", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Test case subject",
			Description:  "Test case description",
			ContactEmail: "customer@example.com",
			ContactName:  "Test Customer",
			Channel:      servicedomain.CaseChannelEmail,
		}

		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, caseObj)

		assert.NotEmpty(t, caseObj.ID)
		assert.Equal(t, workspaceID, caseObj.WorkspaceID)
		assert.Equal(t, "Test case subject", caseObj.Subject)
		assert.Equal(t, servicedomain.CaseStatusNew, caseObj.Status, "New cases should have NEW status")
		assert.Equal(t, servicedomain.CasePriorityMedium, caseObj.Priority)
		assert.NotEmpty(t, caseObj.HumanID, "Case HumanID should be assigned")

		// Verify persistence
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, caseObj.ID, retrieved.ID)
		assert.Equal(t, servicedomain.CaseStatusNew, retrieved.Status)
	})

	t.Run("Creates case with custom priority", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Urgent issue",
			ContactEmail: "urgent@example.com",
			Priority:     servicedomain.CasePriorityUrgent,
		}

		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CasePriorityUrgent, caseObj.Priority)
	})

	t.Run("Creates case with assigned agent", func(t *testing.T) {
		agentID := id.New()
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Pre-assigned case",
			ContactEmail: "customer@example.com",
			AssignedToID: agentID,
		}

		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, agentID, caseObj.AssignedToID)
	})
}

func TestCaseService_UpdateCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "update-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	// Create a case first
	params := CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "Original subject",
		ContactEmail: "customer@example.com",
	}
	caseObj, err := svc.CreateCase(ctx, params)
	require.NoError(t, err)

	t.Run("Updates case fields", func(t *testing.T) {
		caseObj.Subject = "Updated subject"
		caseObj.Priority = servicedomain.CasePriorityHigh

		err := svc.UpdateCase(ctx, caseObj)
		require.NoError(t, err)

		// Verify update persisted
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated subject", retrieved.Subject)
		assert.Equal(t, servicedomain.CasePriorityHigh, retrieved.Priority)
	})
}

func TestCaseService_HandoffCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "handoff-test")
	ctx := context.Background()
	originalTeamID := id.New()
	targetTeamID := id.New()
	operatorUserID := id.New()
	ownerUserID := id.New()
	sourceTeam := &platformdomain.Team{
		ID:          originalTeamID,
		WorkspaceID: workspaceID,
		Name:        "Support",
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.Workspaces().CreateTeam(ctx, sourceTeam))
	targetTeam := &platformdomain.Team{
		ID:          targetTeamID,
		WorkspaceID: workspaceID,
		Name:        "Billing",
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.Workspaces().CreateTeam(ctx, targetTeam))

	sourceQueue := servicedomain.NewQueue(workspaceID, "Support Inbox", "support-inbox", "")
	require.NoError(t, store.Queues().CreateQueue(ctx, sourceQueue))
	targetQueue := servicedomain.NewQueue(workspaceID, "Billing Escalations", "billing-escalations", "")
	require.NoError(t, store.Queues().CreateQueue(ctx, targetQueue))

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil, WithQueueItemStore(store.QueueItems()))
	caseObj, err := svc.CreateCase(ctx, CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "Refund request",
		ContactEmail: "customer@example.com",
		QueueID:      sourceQueue.ID,
		TeamID:       originalTeamID,
	})
	require.NoError(t, err)

	err = svc.HandoffCase(ctx, caseObj.ID, CaseHandoffParams{
		QueueID:          targetQueue.ID,
		TeamID:           targetTeamID,
		Reason:           "refund specialist required",
		PerformedByID:    operatorUserID,
		PerformedByName:  "Ada Operator",
		PerformedByType:  "user",
		OnBehalfOfUserID: ownerUserID,
	})
	require.NoError(t, err)

	updated, err := svc.GetCase(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, targetQueue.ID, updated.QueueID)
	assert.Equal(t, targetTeamID, updated.TeamID)
	assert.Equal(t, 1, updated.MessageCount)

	queueItem, err := store.QueueItems().GetQueueItemByCaseID(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, targetQueue.ID, queueItem.QueueID)

	communications, err := store.Cases().ListCaseCommunications(ctx, caseObj.ID)
	require.NoError(t, err)
	require.Len(t, communications, 1)
	assert.Equal(t, shareddomain.CommTypeSystem, communications[0].Type)
	assert.Contains(t, communications[0].Body, "Case handed off.")
	assert.Contains(t, communications[0].Body, "Queue:")
	assert.Contains(t, communications[0].Body, "Performed by: user")
	assert.Contains(t, communications[0].Body, "Routing mode: direct user")
	assert.Contains(t, communications[0].Body, "On behalf of user: "+ownerUserID)
	assert.Contains(t, communications[0].Body, "Reason: refund specialist required")
}

func TestCaseService_MarkCaseResolved(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "resolve-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Marks case as resolved", func(t *testing.T) {
		// Create case
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Issue to resolve",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusNew, caseObj.Status)

		// Resolve it
		resolvedAt := time.Now()
		err = svc.MarkCaseResolved(ctx, caseObj.ID, resolvedAt)
		require.NoError(t, err)

		// Verify resolution
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusResolved, retrieved.Status)
		assert.NotNil(t, retrieved.ResolvedAt, "ResolvedAt should be set")
		assert.NotNil(t, retrieved.ClosedAt, "ClosedAt should be set for auto-close tracking")
	})
}

func TestCaseService_ReopenCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "reopen-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Reopens resolved case manually", func(t *testing.T) {
		// Create and resolve case
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Issue to reopen",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		err = svc.MarkCaseResolved(ctx, caseObj.ID, time.Now())
		require.NoError(t, err)

		// Manually reopen by updating status
		caseObj, err = svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.True(t, caseObj.CanBeReopened())

		caseObj.Status = servicedomain.CaseStatusOpen
		caseObj.ResolvedAt = nil
		caseObj.ClosedAt = nil
		caseObj.ReopenCount++
		err = svc.UpdateCase(ctx, caseObj)
		require.NoError(t, err)

		// Verify reopen
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusOpen, retrieved.Status)
		assert.Equal(t, 1, retrieved.ReopenCount)
		assert.Nil(t, retrieved.ResolvedAt, "ResolvedAt should be cleared")
		assert.Nil(t, retrieved.ClosedAt, "ClosedAt should be cleared")
	})

	t.Run("Cannot reopen non-resolved case", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Open case",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		assert.False(t, caseObj.CanBeReopened(), "NEW case cannot be reopened")
	})
}

func TestCaseService_AutoCloseResolvedCases(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "autoclose-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Closes resolved cases older than grace period", func(t *testing.T) {
		// Create 3 resolved cases with old ClosedAt timestamps
		var resolvedCaseIDs []string
		for i := 0; i < 3; i++ {
			params := CreateCaseParams{
				WorkspaceID:  workspaceID,
				Subject:      "Auto-close test case",
				ContactEmail: "customer@example.com",
			}
			caseObj, err := svc.CreateCase(ctx, params)
			require.NoError(t, err)

			// Manually set to resolved with old timestamp
			caseObj.Status = servicedomain.CaseStatusResolved
			oldTime := time.Now().Add(-48 * time.Hour) // 48 hours ago
			caseObj.ResolvedAt = &oldTime
			caseObj.ClosedAt = &oldTime
			err = svc.UpdateCase(ctx, caseObj)
			require.NoError(t, err)

			resolvedCaseIDs = append(resolvedCaseIDs, caseObj.ID)
		}

		// Create 1 recently resolved case (should NOT be closed)
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Recent resolved case",
			ContactEmail: "customer@example.com",
		}
		recentCase, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		recentCase.Status = servicedomain.CaseStatusResolved
		recentTime := time.Now().Add(-1 * time.Hour) // 1 hour ago
		recentCase.ResolvedAt = &recentTime
		recentCase.ClosedAt = &recentTime
		err = svc.UpdateCase(ctx, recentCase)
		require.NoError(t, err)

		// Run auto-close with 24h grace period
		result, err := svc.AutoCloseResolvedCases(ctx, 24*time.Hour, 100)
		require.NoError(t, err)

		// Note: In a shared test database, there may be additional resolved cases from other tests.
		// We verify that at least our 3 cases were processed and that specific cases are closed correctly.
		assert.GreaterOrEqual(t, result.Processed, 3, "Should process at least 3 old resolved cases")
		assert.GreaterOrEqual(t, result.Closed, 3, "Should close at least 3 cases")
		assert.Equal(t, 0, result.Errors)

		// Verify old cases are now closed
		for _, caseID := range resolvedCaseIDs {
			c, err := svc.GetCase(ctx, caseID)
			require.NoError(t, err)
			assert.Equal(t, servicedomain.CaseStatusClosed, c.Status, "Case should be closed")
			assert.Contains(t, c.Tags, "auto-closed", "Should have auto-closed tag")
		}

		// Verify recent case is still resolved
		c, err := svc.GetCase(ctx, recentCase.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusResolved, c.Status, "Recent case should still be resolved")
	})

	t.Run("Does not close non-resolved cases", func(t *testing.T) {
		// Create an open case with old timestamp
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Open case",
			ContactEmail: "customer@example.com",
		}
		openCase, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		openCase.Status = servicedomain.CaseStatusOpen
		oldTime := time.Now().Add(-48 * time.Hour)
		openCase.ClosedAt = &oldTime // Even with old ClosedAt, shouldn't close open case
		err = svc.UpdateCase(ctx, openCase)
		require.NoError(t, err)

		// Run auto-close
		_, err = svc.AutoCloseResolvedCases(ctx, 24*time.Hour, 100)
		require.NoError(t, err)

		// Verify open case is still open
		c, err := svc.GetCase(ctx, openCase.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusOpen, c.Status, "Open case should remain open")
	})

	t.Run("Respects batch size", func(t *testing.T) {
		// Create a separate workspace for this subtest
		batchWorkspaceID := setupTestWorkspace(t, store, "batch-test")

		// Create 5 old resolved cases
		for i := 0; i < 5; i++ {
			params := CreateCaseParams{
				WorkspaceID:  batchWorkspaceID,
				Subject:      "Batch test case",
				ContactEmail: "customer@example.com",
			}
			caseObj, err := svc.CreateCase(ctx, params)
			require.NoError(t, err)

			caseObj.Status = servicedomain.CaseStatusResolved
			oldTime := time.Now().Add(-48 * time.Hour)
			caseObj.ClosedAt = &oldTime
			caseObj.ResolvedAt = &oldTime
			err = svc.UpdateCase(ctx, caseObj)
			require.NoError(t, err)
		}

		// Run with batch size of 2
		result, err := svc.AutoCloseResolvedCases(ctx, 24*time.Hour, 2)
		require.NoError(t, err)

		assert.Equal(t, 2, result.Processed, "Should only process batch size")
		assert.Equal(t, 2, result.Closed)
	})

	t.Run("Does not duplicate auto-closed tag", func(t *testing.T) {
		// Use a separate store to ensure clean state
		localStore, localCleanup := setupTestStore(t)
		defer localCleanup()

		// Create workspace for this subtest
		tagWorkspaceID := setupTestWorkspace(t, localStore, "tag-unique")

		localSvc := NewCaseService(localStore.Queues(), localStore.Cases(), localStore.Workspaces(), nil)

		// Create a resolved case with auto-closed tag already
		params := CreateCaseParams{
			WorkspaceID:  tagWorkspaceID,
			Subject:      "Already tagged case",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := localSvc.CreateCase(ctx, params)
		require.NoError(t, err)

		caseObj.Status = servicedomain.CaseStatusResolved
		caseObj.Tags = []string{"auto-closed"}
		oldTime := time.Now().Add(-48 * time.Hour)
		caseObj.ClosedAt = &oldTime
		caseObj.ResolvedAt = &oldTime
		err = localSvc.UpdateCase(ctx, caseObj)
		require.NoError(t, err)

		// Run auto-close
		result, err := localSvc.AutoCloseResolvedCases(ctx, 24*time.Hour, 100)
		require.NoError(t, err)

		// We only care that our case was processed (may process other leftover cases too)
		assert.GreaterOrEqual(t, result.Closed, 1)

		// Verify the case only has one auto-closed tag
		updatedCase, err := localSvc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		tagCount := 0
		for _, tag := range updatedCase.Tags {
			if tag == "auto-closed" {
				tagCount++
			}
		}
		assert.Equal(t, 1, tagCount, "Should have exactly one auto-closed tag")
	})
}

func TestCaseService_CreateCaseFromEmail(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "email-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("creates case with email channel", func(t *testing.T) {
		caseObj, err := svc.CreateCaseFromEmail(ctx, workspaceID, "Support Request", "I need help with my account", "customer@example.com", "John Doe")
		require.NoError(t, err)
		require.NotNil(t, caseObj)

		assert.Equal(t, servicedomain.CaseChannelEmail, caseObj.Channel)
		assert.Equal(t, "Support Request", caseObj.Subject)
		assert.Equal(t, "I need help with my account", caseObj.Description)
		assert.Equal(t, "customer@example.com", caseObj.ContactEmail)
		assert.Equal(t, "John Doe", caseObj.ContactName)
		assert.Equal(t, servicedomain.CasePriorityMedium, caseObj.Priority)
	})
}

func TestCaseService_CreateCaseFromForm(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "form-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("creates case with web channel", func(t *testing.T) {
		params := CreateCaseParams{
			ContactEmail: "formuser@example.com",
			ContactName:  "Form User",
		}

		caseObj, err := svc.CreateCaseFromForm(ctx, workspaceID, "Bug Report", "Found a bug in the app", params)
		require.NoError(t, err)
		require.NotNil(t, caseObj)

		assert.Equal(t, servicedomain.CaseChannelWeb, caseObj.Channel)
		assert.Equal(t, "Bug Report", caseObj.Subject)
		assert.Equal(t, "Found a bug in the app", caseObj.Description)
	})

	t.Run("uses default priority if not specified", func(t *testing.T) {
		params := CreateCaseParams{
			ContactEmail: "user2@example.com",
		}

		caseObj, err := svc.CreateCaseFromForm(ctx, workspaceID, "Question", "How do I do X?", params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CasePriorityMedium, caseObj.Priority)
	})

	t.Run("uses provided priority if specified", func(t *testing.T) {
		params := CreateCaseParams{
			ContactEmail: "user3@example.com",
			Priority:     servicedomain.CasePriorityHigh,
		}

		caseObj, err := svc.CreateCaseFromForm(ctx, workspaceID, "Urgent Issue", "Need help ASAP", params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CasePriorityHigh, caseObj.Priority)
	})
}

func TestCaseService_CreateCaseFromAPI(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "api-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("creates case with API channel", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "API Created Case",
			Description:  "Case created via API",
			ContactEmail: "api@example.com",
			Priority:     servicedomain.CasePriorityLow,
		}

		caseObj, err := svc.CreateCaseFromAPI(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, caseObj)

		assert.Equal(t, servicedomain.CaseChannelAPI, caseObj.Channel)
		assert.Equal(t, "API Created Case", caseObj.Subject)
		assert.Equal(t, servicedomain.CasePriorityLow, caseObj.Priority)
	})
}

func TestCaseService_DeleteCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "delete-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("deletes existing case", func(t *testing.T) {
		// Create a case
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Case to delete",
			ContactEmail: "delete@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Verify it exists
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, caseObj.ID, retrieved.ID)

		// Delete it
		err = svc.DeleteCase(ctx, workspaceID, caseObj.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = svc.GetCase(ctx, caseObj.ID)
		assert.Error(t, err)
	})

	t.Run("returns error for non-existent case", func(t *testing.T) {
		err := svc.DeleteCase(ctx, workspaceID, "non-existent-case-id")
		assert.Error(t, err)
	})
}

func TestCaseService_AddCommunication(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "comm-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("adds communication to case", func(t *testing.T) {
		// Create a case first
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Case with communication",
			ContactEmail: "comm@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Add communication
		agentID := id.New()
		comm := &servicedomain.Communication{
			WorkspaceID: caseObj.WorkspaceID,
			CaseID:      caseObj.ID,
			Direction:   shareddomain.DirectionOutbound,
			Type:        shareddomain.CommTypeEmail,
			Subject:     "Re: Case with communication",
			Body:        "Thank you for contacting us.",
			FromUserID:  agentID,
		}

		err = svc.AddCommunication(ctx, comm)
		require.NoError(t, err)

		assert.NotEmpty(t, comm.ID)
		assert.False(t, comm.CreatedAt.IsZero())
	})

	t.Run("requires case_id", func(t *testing.T) {
		comm := &servicedomain.Communication{
			WorkspaceID: "ws-test",
			Body:        "Test message",
		}

		err := svc.AddCommunication(ctx, comm)
		assert.Error(t, err)
		// Should be a validation error
		assert.Contains(t, err.Error(), "validation")
	})
}

func TestCaseService_LinkIssueToCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "link-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("links issue to case", func(t *testing.T) {
		// Create a case
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Case to link",
			ContactEmail: "link@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Link an issue
		err = svc.LinkIssueToCase(ctx, caseObj.ID, "issue-123", "project-abc")
		require.NoError(t, err)

		// Verify the link
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)

		// Check that the issue was linked (via LinkedIssueIDs or CustomFields)
		assert.True(t, len(retrieved.LinkedIssueIDs) > 0 || !retrieved.CustomFields.IsEmpty())
	})
}

func TestCaseService_UnlinkIssueFromCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "unlink-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("unlinks issue from case", func(t *testing.T) {
		// Create a case
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Case to unlink",
			ContactEmail: "unlink@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Link then unlink an issue
		err = svc.LinkIssueToCase(ctx, caseObj.ID, "issue-456", "project-def")
		require.NoError(t, err)

		err = svc.UnlinkIssueFromCase(ctx, caseObj.ID, "issue-456")
		require.NoError(t, err)

		// Verify the issue was unlinked
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Empty(t, retrieved.LinkedIssueIDs)
	})
}

func TestCaseService_NotifyCaseContact(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create workspace before creating cases
	workspaceID := setupTestWorkspace(t, store, "notify-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("marks contact as notified", func(t *testing.T) {
		// Create a case
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Case for notification",
			ContactEmail: "notify@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Mark as notified
		notifiedAt := time.Now()
		err = svc.NotifyCaseContact(ctx, caseObj.ID, notifiedAt, "case_created")
		require.NoError(t, err)

		// Verify notification was recorded
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.NotNil(t, retrieved.ContactNotifiedAt)
	})
}

func TestCaseValidation(t *testing.T) {
	// Test domain validation directly - service delegates to domain.Validate()

	t.Run("returns error for empty workspace ID", func(t *testing.T) {
		caseObj := &servicedomain.Case{
			Subject:  "Test",
			Status:   servicedomain.CaseStatusNew,
			Priority: servicedomain.CasePriorityMedium,
			Channel:  servicedomain.CaseChannelEmail,
		}
		err := caseObj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workspace_id")
	})

	t.Run("returns error for empty subject", func(t *testing.T) {
		caseObj := &servicedomain.Case{
			CaseIdentity: servicedomain.CaseIdentity{
				WorkspaceID: "ws_test",
			},
			Status:   servicedomain.CaseStatusNew,
			Priority: servicedomain.CasePriorityMedium,
			Channel:  servicedomain.CaseChannelEmail,
		}
		err := caseObj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "subject")
	})

	t.Run("passes validation with required fields", func(t *testing.T) {
		caseObj := &servicedomain.Case{
			CaseIdentity: servicedomain.CaseIdentity{
				WorkspaceID: "ws_test",
			},
			Subject:  "Valid subject",
			Status:   servicedomain.CaseStatusNew,
			Priority: servicedomain.CasePriorityMedium,
			Channel:  servicedomain.CaseChannelEmail,
		}
		err := caseObj.Validate()
		require.NoError(t, err)
	})
}

func TestCaseService_CreateCase_Validation(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("returns error for empty workspace ID", func(t *testing.T) {
		params := CreateCaseParams{
			Subject:      "Test subject",
			ContactEmail: "test@example.com",
		}
		_, err := svc.CreateCase(ctx, params)
		assert.Error(t, err)
	})

	t.Run("returns error for empty subject", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  "ws_test",
			ContactEmail: "test@example.com",
		}
		_, err := svc.CreateCase(ctx, params)
		assert.Error(t, err)
	})
}

func TestCaseService_UpdateCase_Validation(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "update-validation")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("updates case with new priority", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Priority Update Test",
			ContactEmail: "test@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		caseObj.Priority = servicedomain.CasePriorityUrgent
		err = svc.UpdateCase(ctx, caseObj)
		require.NoError(t, err)

		updated, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CasePriorityUrgent, updated.Priority)
	})

	t.Run("updates case with team assignment", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Team Assignment Test",
			ContactEmail: "test@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		teamID := id.New()
		caseObj.TeamID = teamID
		err = svc.UpdateCase(ctx, caseObj)
		require.NoError(t, err)

		updated, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, teamID, updated.TeamID)
	})
}

func TestCaseService_UpdateCase_Errors(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("returns error for empty case ID", func(t *testing.T) {
		caseObj := &servicedomain.Case{
			CaseIdentity: servicedomain.CaseIdentity{
				WorkspaceID: "ws_test",
			},
			Subject:  "Test subject",
			Status:   servicedomain.CaseStatusNew,
			Priority: servicedomain.CasePriorityMedium,
			Channel:  servicedomain.CaseChannelEmail,
		}
		err := svc.UpdateCase(ctx, caseObj)
		assert.Error(t, err)
		// Typed validation error contains field name 'id'
		assert.Contains(t, err.Error(), "id")
	})
}

func TestCaseService_LinkIssueToCase_Errors(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("returns error for non-existent case", func(t *testing.T) {
		err := svc.LinkIssueToCase(ctx, "non_existent_case", "issue_1", "project_1")
		assert.Error(t, err)
	})
}

func TestCaseService_UnlinkIssueFromCase_Errors(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("returns error for non-existent case", func(t *testing.T) {
		err := svc.UnlinkIssueFromCase(ctx, "non_existent_case", "issue_1")
		assert.Error(t, err)
	})
}

func TestCaseService_MarkCaseResolved_Errors(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("returns error for non-existent case", func(t *testing.T) {
		err := svc.MarkCaseResolved(ctx, "non_existent_case", time.Now())
		assert.Error(t, err)
	})
}

func TestCaseService_NotifyCaseContact_Errors(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("returns error for non-existent case", func(t *testing.T) {
		err := svc.NotifyCaseContact(ctx, "non_existent_case", time.Now(), "notification_template")
		assert.Error(t, err)
	})
}

func TestCaseService_CaseLifecycleIntegration(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "lifecycle-test")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Full lifecycle: new → pending → open → resolved → closed", func(t *testing.T) {
		// Step 1: Create case (NEW)
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Complete lifecycle test",
			ContactEmail: "customer@test.com",
			ContactName:  "Test Customer",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusNew, caseObj.Status)

		// Step 2: Agent assigns and responds (NEW → PENDING)
		caseObj.Status = servicedomain.CaseStatusPending
		caseObj.AssignedToID = id.New()
		now := time.Now()
		caseObj.FirstResponseAt = &now
		err = svc.UpdateCase(ctx, caseObj)
		require.NoError(t, err)

		retrieved, _ := svc.GetCase(ctx, caseObj.ID)
		assert.Equal(t, servicedomain.CaseStatusPending, retrieved.Status)
		assert.NotNil(t, retrieved.FirstResponseAt)

		// Step 3: Customer replies (PENDING → OPEN)
		caseObj.Status = servicedomain.CaseStatusOpen
		caseObj.MessageCount++
		err = svc.UpdateCase(ctx, caseObj)
		require.NoError(t, err)

		retrieved, _ = svc.GetCase(ctx, caseObj.ID)
		assert.Equal(t, servicedomain.CaseStatusOpen, retrieved.Status)

		// Step 4: Agent resolves (OPEN → RESOLVED)
		resolvedAt := time.Now()
		err = svc.MarkCaseResolved(ctx, caseObj.ID, resolvedAt)
		require.NoError(t, err)

		retrieved, _ = svc.GetCase(ctx, caseObj.ID)
		assert.Equal(t, servicedomain.CaseStatusResolved, retrieved.Status)
		assert.NotNil(t, retrieved.ResolvedAt)
		assert.NotNil(t, retrieved.ClosedAt)

		// Step 5: Auto-close after grace period (RESOLVED → CLOSED)
		// Simulate old resolved time
		oldTime := time.Now().Add(-25 * time.Hour)
		retrieved.ClosedAt = &oldTime
		retrieved.ResolvedAt = &oldTime
		err = svc.UpdateCase(ctx, retrieved)
		require.NoError(t, err)

		result, err := svc.AutoCloseResolvedCases(ctx, 24*time.Hour, 100)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Closed)

		final, _ := svc.GetCase(ctx, caseObj.ID)
		assert.Equal(t, servicedomain.CaseStatusClosed, final.Status)
		assert.Contains(t, final.Tags, "auto-closed")
	})

	t.Run("Reopen flow: resolved → open → resolved → closed", func(t *testing.T) {
		// Create and resolve
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Reopen test",
			ContactEmail: "customer@test.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		err = svc.MarkCaseResolved(ctx, caseObj.ID, time.Now())
		require.NoError(t, err)

		// Customer replies - reopen manually
		caseObj, err = svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		caseObj.Status = servicedomain.CaseStatusOpen
		caseObj.ResolvedAt = nil
		caseObj.ClosedAt = nil
		caseObj.ReopenCount++
		err = svc.UpdateCase(ctx, caseObj)
		require.NoError(t, err)

		retrieved, _ := svc.GetCase(ctx, caseObj.ID)
		assert.Equal(t, servicedomain.CaseStatusOpen, retrieved.Status)
		assert.Equal(t, 1, retrieved.ReopenCount)

		// Resolve again
		err = svc.MarkCaseResolved(ctx, caseObj.ID, time.Now())
		require.NoError(t, err)

		retrieved, _ = svc.GetCase(ctx, caseObj.ID)
		assert.Equal(t, servicedomain.CaseStatusResolved, retrieved.Status)

		// Simulate grace period and auto-close
		oldTime := time.Now().Add(-25 * time.Hour)
		retrieved.ClosedAt = &oldTime
		retrieved.ResolvedAt = &oldTime
		err = svc.UpdateCase(ctx, retrieved)
		require.NoError(t, err)

		_, err = svc.AutoCloseResolvedCases(ctx, 24*time.Hour, 100)
		require.NoError(t, err)

		final, _ := svc.GetCase(ctx, caseObj.ID)
		assert.Equal(t, servicedomain.CaseStatusClosed, final.Status)
		assert.Equal(t, 1, final.ReopenCount, "Reopen count should be preserved")
	})
}

// ==================== CASE ACTION SERVICE TESTS ====================

func TestCaseService_AssignCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "assign-case")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Assigns case to user", func(t *testing.T) {
		agentID := id.New()
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Unassigned case",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Empty(t, caseObj.AssignedToID)
		assert.Equal(t, servicedomain.CaseStatusNew, caseObj.Status)

		// Assign to user
		err = svc.AssignCase(ctx, caseObj.ID, agentID, "")
		require.NoError(t, err)

		// Verify assignment persisted
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, agentID, retrieved.AssignedToID)
		assert.Equal(t, servicedomain.CaseStatusOpen, retrieved.Status, "Status should transition to OPEN on assignment")
	})

	t.Run("Assigns case to team", func(t *testing.T) {
		teamID := id.New()
		require.NoError(t, store.Workspaces().CreateTeam(ctx, &platformdomain.Team{
			ID:          teamID,
			WorkspaceID: workspaceID,
			Name:        "Team assignment",
			IsActive:    true,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}))
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Team assignment test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		err = svc.AssignCase(ctx, caseObj.ID, "", teamID)
		require.NoError(t, err)

		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, teamID, retrieved.TeamID)
	})

	t.Run("Fails with empty user and team", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Empty assignment test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		err = svc.AssignCase(ctx, caseObj.ID, "", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must provide either user_id or team_id")
	})

	t.Run("Returns error for non-existent case", func(t *testing.T) {
		err := svc.AssignCase(ctx, "non-existent-case", id.New(), "")
		assert.Error(t, err)
	})

	t.Run("Fails when validated assignee user does not exist", func(t *testing.T) {
		validatingSvc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil, WithUserStore(store.Users()))
		caseObj, err := validatingSvc.CreateCase(ctx, CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Validated assignment test",
			ContactEmail: "customer@example.com",
		})
		require.NoError(t, err)

		err = validatingSvc.AssignCase(ctx, caseObj.ID, id.New(), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user")
	})

	t.Run("Fails when target team does not exist", func(t *testing.T) {
		caseObj, err := svc.CreateCase(ctx, CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Missing team assignment test",
			ContactEmail: "customer@example.com",
		})
		require.NoError(t, err)

		err = svc.AssignCase(ctx, caseObj.ID, "", id.New())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "team")
	})
}

func TestCaseService_UnassignCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "unassign-case")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Unassigns user from case", func(t *testing.T) {
		agentID := id.New()
		teamID := id.New()
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Assigned case",
			ContactEmail: "customer@example.com",
			AssignedToID: agentID,
			TeamID:       teamID,
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, agentID, caseObj.AssignedToID)
		assert.Equal(t, teamID, caseObj.TeamID)

		// Unassign
		err = svc.UnassignCase(ctx, caseObj.ID)
		require.NoError(t, err)

		// Verify unassignment persisted
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Empty(t, retrieved.AssignedToID, "User assignment should be cleared")
		assert.Equal(t, teamID, retrieved.TeamID, "Team should remain")
	})
}

func TestCaseService_SetCasePriority(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "set-priority")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Changes case priority", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Priority test",
			ContactEmail: "customer@example.com",
			Priority:     servicedomain.CasePriorityMedium,
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CasePriorityMedium, caseObj.Priority)

		// Escalate to urgent
		err = svc.SetCasePriority(ctx, caseObj.ID, servicedomain.CasePriorityUrgent)
		require.NoError(t, err)

		// Verify priority change persisted
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CasePriorityUrgent, retrieved.Priority)
	})

	t.Run("Rejects invalid priority", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Invalid priority test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		err = svc.SetCasePriority(ctx, caseObj.ID, servicedomain.CasePriority("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid priority")
	})
}

func TestCaseService_SetCaseStatus(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "set-status")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Changes case status", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Status test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusNew, caseObj.Status)

		// Change to pending
		err = svc.SetCaseStatus(ctx, caseObj.ID, servicedomain.CaseStatusPending)
		require.NoError(t, err)

		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusPending, retrieved.Status)
	})

	t.Run("Resolve via set status records lifecycle timestamps", func(t *testing.T) {
		caseObj, err := svc.CreateCase(ctx, CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Resolve lifecycle test",
			ContactEmail: "customer@example.com",
		})
		require.NoError(t, err)

		err = svc.SetCaseStatus(ctx, caseObj.ID, servicedomain.CaseStatusResolved)
		require.NoError(t, err)

		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusResolved, retrieved.Status)
		assert.NotNil(t, retrieved.ResolvedAt)
		assert.NotNil(t, retrieved.ClosedAt)
	})

	t.Run("Moving resolved case back to pending reopens and clears timestamps", func(t *testing.T) {
		caseObj, err := svc.CreateCase(ctx, CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Reopen lifecycle test",
			ContactEmail: "customer@example.com",
		})
		require.NoError(t, err)
		require.NoError(t, svc.MarkCaseResolved(ctx, caseObj.ID, time.Now().UTC()))

		err = svc.SetCaseStatus(ctx, caseObj.ID, servicedomain.CaseStatusPending)
		require.NoError(t, err)

		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusPending, retrieved.Status)
		assert.Nil(t, retrieved.ResolvedAt)
		assert.Nil(t, retrieved.ClosedAt)
		assert.Equal(t, 1, retrieved.ReopenCount)
	})

	t.Run("Close via set status preserves resolved timestamp and stamps closure", func(t *testing.T) {
		caseObj, err := svc.CreateCase(ctx, CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Close lifecycle test",
			ContactEmail: "customer@example.com",
		})
		require.NoError(t, err)
		require.NoError(t, svc.MarkCaseResolved(ctx, caseObj.ID, time.Now().UTC()))

		err = svc.SetCaseStatus(ctx, caseObj.ID, servicedomain.CaseStatusClosed)
		require.NoError(t, err)

		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusClosed, retrieved.Status)
		assert.NotNil(t, retrieved.ResolvedAt)
		assert.NotNil(t, retrieved.ClosedAt)
		assert.False(t, retrieved.ClosedAt.Before(*retrieved.ResolvedAt))
	})
}

func TestCaseService_CloseCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "close-case")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Closes resolved case", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Close test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// First resolve
		err = svc.MarkCaseResolved(ctx, caseObj.ID, time.Now())
		require.NoError(t, err)

		// Then close
		err = svc.CloseCase(ctx, caseObj.ID)
		require.NoError(t, err)

		// Verify closed
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusClosed, retrieved.Status)
		assert.NotNil(t, retrieved.ClosedAt)
	})

	t.Run("Cannot close non-resolved case", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Non-resolved close test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusNew, caseObj.Status)

		// Try to close without resolving
		err = svc.CloseCase(ctx, caseObj.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be resolved before closing")
	})
}

func TestCaseService_AddCaseTag(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "add-tag")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Adds tag to case", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Tag test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Empty(t, caseObj.Tags)

		// Add tag
		err = svc.AddCaseTag(ctx, caseObj.ID, "urgent")
		require.NoError(t, err)

		// Verify tag added
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Contains(t, retrieved.Tags, "urgent")
	})

	t.Run("Idempotent - adding duplicate tag succeeds", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Duplicate tag test",
			ContactEmail: "customer@example.com",
			Tags:         []string{"existing"},
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Add same tag again
		err = svc.AddCaseTag(ctx, caseObj.ID, "existing")
		require.NoError(t, err)

		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Len(t, retrieved.Tags, 1, "Should not duplicate tag")
	})

	t.Run("Rejects empty tag", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Empty tag test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		err = svc.AddCaseTag(ctx, caseObj.ID, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tag cannot be empty")
	})
}

func TestCaseService_RemoveCaseTag(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "remove-tag")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Removes tag from case", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Tag removal test",
			ContactEmail: "customer@example.com",
			Tags:         []string{"urgent", "billing"},
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Len(t, caseObj.Tags, 2)

		// Remove tag
		err = svc.RemoveCaseTag(ctx, caseObj.ID, "urgent")
		require.NoError(t, err)

		// Verify tag removed
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.NotContains(t, retrieved.Tags, "urgent")
		assert.Contains(t, retrieved.Tags, "billing")
	})

	t.Run("Removing non-existent tag succeeds", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Non-existent tag test",
			ContactEmail: "customer@example.com",
			Tags:         []string{"existing"},
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Remove non-existent tag
		err = svc.RemoveCaseTag(ctx, caseObj.ID, "nonexistent")
		require.NoError(t, err)

		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Len(t, retrieved.Tags, 1, "Existing tags should remain")
	})
}

func TestCaseService_SetCaseCategory(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "set-category")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Sets case category", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Category test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Empty(t, caseObj.Category)

		// Set category
		err = svc.SetCaseCategory(ctx, caseObj.ID, "billing")
		require.NoError(t, err)

		// Verify category set
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, "billing", retrieved.Category)
	})

	t.Run("Changes existing category", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Category change test",
			ContactEmail: "customer@example.com",
			Category:     "billing",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Change category
		err = svc.SetCaseCategory(ctx, caseObj.ID, "technical")
		require.NoError(t, err)

		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, "technical", retrieved.Category)
	})
}

func TestCaseService_AddInternalNote(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "add-note")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Adds internal note to case", func(t *testing.T) {
		agentID := id.New()
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Note test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Add internal note
		note, err := svc.AddInternalNote(ctx, caseObj.ID, workspaceID, agentID, "Test Agent", "This is an internal note")
		require.NoError(t, err)
		require.NotNil(t, note)

		assert.NotEmpty(t, note.ID)
		assert.Equal(t, caseObj.ID, note.CaseID)
		assert.Equal(t, shareddomain.CommTypeNote, note.Type)
		assert.Equal(t, shareddomain.DirectionInternal, note.Direction)
		assert.True(t, note.IsInternal)
		assert.Equal(t, agentID, note.FromUserID)
		assert.Equal(t, "Test Agent", note.FromName)
		assert.Equal(t, "This is an internal note", note.Body)

		// Verify communication persisted
		comms, err := svc.GetCaseCommunications(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Len(t, comms, 1)
		assert.Equal(t, note.ID, comms[0].ID)
	})
}

func TestCaseService_ReplyToCase(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "reply-to-case")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Adds reply and transitions case to pending", func(t *testing.T) {
		agentID := id.New()
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Reply test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusNew, caseObj.Status)
		assert.Nil(t, caseObj.FirstResponseAt)

		// Reply to case
		replyParams := ReplyToCaseParams{
			CaseID:      caseObj.ID,
			WorkspaceID: workspaceID,
			UserID:      agentID,
			UserName:    "Test Agent",
			UserEmail:   "agent@example.com",
			Body:        "Thank you for contacting us.",
			ToEmails:    []string{"customer@example.com"},
			Subject:     "Re: Reply test",
		}
		reply, err := svc.ReplyToCase(ctx, replyParams)
		require.NoError(t, err)
		require.NotNil(t, reply)

		assert.NotEmpty(t, reply.ID)
		assert.Equal(t, shareddomain.CommTypeEmail, reply.Type)
		assert.Equal(t, shareddomain.DirectionOutbound, reply.Direction)
		assert.False(t, reply.IsInternal)
		assert.Equal(t, "agent@example.com", reply.FromEmail)
		assert.Contains(t, reply.ToEmails, "customer@example.com")

		// Verify case updated
		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusPending, retrieved.Status, "Status should transition to PENDING")
		assert.NotNil(t, retrieved.FirstResponseAt, "FirstResponseAt should be set")
		assert.Equal(t, 1, retrieved.MessageCount)
	})

	t.Run("Reply from open case also transitions to pending", func(t *testing.T) {
		agentID := id.New()
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Open case reply test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Manually set to open
		caseObj.Status = servicedomain.CaseStatusOpen
		err = svc.UpdateCase(ctx, caseObj)
		require.NoError(t, err)

		// Reply
		replyParams := ReplyToCaseParams{
			CaseID:      caseObj.ID,
			WorkspaceID: workspaceID,
			UserID:      agentID,
			UserName:    "Test Agent",
			UserEmail:   "agent@example.com",
			Body:        "Following up.",
			ToEmails:    []string{"customer@example.com"},
		}
		_, err = svc.ReplyToCase(ctx, replyParams)
		require.NoError(t, err)

		retrieved, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusPending, retrieved.Status)
	})
}

func TestCaseService_GetCaseCommunications(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "get-comms")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Retrieves all communications for a case", func(t *testing.T) {
		firstAgentID := id.New()
		secondAgentID := id.New()
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Communications test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		// Add multiple communications
		_, err = svc.AddInternalNote(ctx, caseObj.ID, workspaceID, firstAgentID, "Agent One", "First note")
		require.NoError(t, err)

		_, err = svc.AddInternalNote(ctx, caseObj.ID, workspaceID, secondAgentID, "Agent Two", "Second note")
		require.NoError(t, err)

		// Retrieve communications
		comms, err := svc.GetCaseCommunications(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Len(t, comms, 2)
	})

	t.Run("Returns empty list for case with no communications", func(t *testing.T) {
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "No communications test",
			ContactEmail: "customer@example.com",
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)

		comms, err := svc.GetCaseCommunications(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Empty(t, comms)
	})
}

// ==================== INTEGRATION WORKFLOW TESTS ====================

func TestCaseService_CompleteAgentWorkflow(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "agent-workflow")

	svc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	ctx := context.Background()

	t.Run("Complete agent triage and resolution workflow", func(t *testing.T) {
		agentID := id.New()
		teamID := id.New()
		require.NoError(t, store.Workspaces().CreateTeam(ctx, &platformdomain.Team{
			ID:          teamID,
			WorkspaceID: workspaceID,
			Name:        "Support",
			IsActive:    true,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}))
		// Customer creates a case
		params := CreateCaseParams{
			WorkspaceID:  workspaceID,
			Subject:      "Cannot login to my account",
			Description:  "I've tried multiple times but keep getting an error",
			ContactEmail: "customer@example.com",
			ContactName:  "John Customer",
			Channel:      servicedomain.CaseChannelEmail,
		}
		caseObj, err := svc.CreateCase(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusNew, caseObj.Status)

		// Agent triages: assign, set priority, categorize, tag
		err = svc.AssignCase(ctx, caseObj.ID, agentID, teamID)
		require.NoError(t, err)

		err = svc.SetCasePriority(ctx, caseObj.ID, servicedomain.CasePriorityHigh)
		require.NoError(t, err)

		err = svc.SetCaseCategory(ctx, caseObj.ID, "authentication")
		require.NoError(t, err)

		err = svc.AddCaseTag(ctx, caseObj.ID, "login-issue")
		require.NoError(t, err)

		// Verify triage state
		triaged, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusOpen, triaged.Status)
		assert.Equal(t, agentID, triaged.AssignedToID)
		assert.Equal(t, teamID, triaged.TeamID)
		assert.Equal(t, servicedomain.CasePriorityHigh, triaged.Priority)
		assert.Equal(t, "authentication", triaged.Category)
		assert.Contains(t, triaged.Tags, "login-issue")

		// Agent adds internal note
		_, err = svc.AddInternalNote(ctx, caseObj.ID, workspaceID, agentID, "Agent Smith", "Customer account locked due to failed attempts. Unlocking now.")
		require.NoError(t, err)

		// Agent replies to customer
		replyParams := ReplyToCaseParams{
			CaseID:      caseObj.ID,
			WorkspaceID: workspaceID,
			UserID:      agentID,
			UserName:    "Agent Smith",
			UserEmail:   "support@company.com",
			Body:        "Hi John, I've unlocked your account. Please try logging in again.",
			ToEmails:    []string{"customer@example.com"},
			Subject:     "Re: Cannot login to my account",
		}
		_, err = svc.ReplyToCase(ctx, replyParams)
		require.NoError(t, err)

		// Verify reply state
		replied, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusPending, replied.Status)
		assert.NotNil(t, replied.FirstResponseAt)

		// Verify communications
		comms, err := svc.GetCaseCommunications(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Len(t, comms, 2, "Should have internal note and reply")

		// Customer confirms fix, agent resolves
		err = svc.MarkCaseResolved(ctx, caseObj.ID, time.Now())
		require.NoError(t, err)

		// Final close
		err = svc.CloseCase(ctx, caseObj.ID)
		require.NoError(t, err)

		// Verify final state
		final, err := svc.GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusClosed, final.Status)
		assert.NotNil(t, final.ClosedAt)
	})
}
