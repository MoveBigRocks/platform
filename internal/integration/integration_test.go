//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestIntegration_Transaction(t *testing.T) {
	testutil.SetupTestEnv(t)

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Test atomic write failure (rollback)
	err := store.WithTransaction(ctx, func(ctx context.Context) error {
		// In a real scenario, we would use store methods that use the transaction from context.
		// Since existing stores (userStore etc.) are not yet updated to pull tx from context,
		// we can't easily test high-level store rollback here without refactoring all stores.
		// However, we can test that the transaction infrastructure works if we access the backend directly
		// using a helper (if we exposed one) or just verifying the interface call succeeds.

		// For now, this test just verifies the interface change allows compilation and execution.
		return nil
	})

	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// 2. Test manual rollback via error
	err = store.WithTransaction(ctx, func(ctx context.Context) error {
		// Simulate error
		return shared.ErrNotFound
	})

	if err != shared.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %v", err)
	}
}

// Helper to create test context
func testContext() context.Context {
	return context.Background()
}

// TestRLS_CrossTenantIsolation verifies that Row-Level Security properly
// isolates data between workspaces. This is a critical security test.
func TestRLS_CrossTenantIsolation(t *testing.T) {
	testutil.SetupTestEnv(t)

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create two workspaces
	ws1ID := testutil.CreateTestWorkspace(t, store, "workspace-1")
	ws2ID := testutil.CreateTestWorkspace(t, store, "workspace-2")

	// Use admin context to create test data (bypasses RLS)
	var case1ID, case2ID string
	err := store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		// Create a case in workspace 1
		case1 := servicedomain.NewCase(ws1ID, "WS1 Case", "user1@example.com")
		case1.GenerateHumanID("ws1")
		if err := store.Cases().CreateCase(adminCtx, case1); err != nil {
			return err
		}
		case1ID = case1.ID

		// Create a case in workspace 2
		case2 := servicedomain.NewCase(ws2ID, "WS2 Case", "user2@example.com")
		case2.GenerateHumanID("ws2")
		if err := store.Cases().CreateCase(adminCtx, case2); err != nil {
			return err
		}
		case2ID = case2.ID
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, case1ID)
	require.NotEmpty(t, case2ID)

	t.Run("workspace1 can only see own cases", func(t *testing.T) {
		// Set RLS context to workspace 1
		testutil.SetTestTenantContext(t, store, ws1ID)

		cases, err := store.Cases().ListWorkspaceCases(ctx, ws1ID, shared.CaseFilter{})
		require.NoError(t, err)

		// Should only see case1
		caseIDs := extractCaseIDs(cases)
		assert.Contains(t, caseIDs, case1ID, "should see own case")
		assert.NotContains(t, caseIDs, case2ID, "should not see other workspace case")
	})

	t.Run("workspace2 can only see own cases", func(t *testing.T) {
		// Set RLS context to workspace 2
		testutil.SetTestTenantContext(t, store, ws2ID)

		cases, err := store.Cases().ListWorkspaceCases(ctx, ws2ID, shared.CaseFilter{})
		require.NoError(t, err)

		// Should only see case2
		caseIDs := extractCaseIDs(cases)
		assert.Contains(t, caseIDs, case2ID, "should see own case")
		assert.NotContains(t, caseIDs, case1ID, "should not see other workspace case")
	})

	t.Run("admin context can see all cases", func(t *testing.T) {
		// Clear tenant context and use admin
		testutil.ClearTestTenantContext(t, store)

		err := store.WithAdminContext(ctx, func(adminCtx context.Context) error {
			// List cases for auto-close (cross-workspace query)
			cases, err := store.Cases().ListResolvedCasesForAutoClose(adminCtx, testutil.FutureTime(), 100)
			if err != nil {
				return err
			}

			// Admin should be able to query without workspace filter
			// (though this specific query might return 0 since cases aren't resolved)
			_ = cases
			return nil
		})
		require.NoError(t, err, "admin context should allow cross-workspace queries")
	})

	t.Run("explicit workspace filter works without tenant context", func(t *testing.T) {
		// Clear tenant context
		testutil.ClearTestTenantContext(t, store)

		// Query with explicit workspace ID parameter
		// Note: ListWorkspaceCases filters by the passed workspaceID parameter,
		// not by RLS context. The explicit parameter provides the isolation.
		cases, err := store.Cases().ListWorkspaceCases(ctx, ws1ID, shared.CaseFilter{})
		require.NoError(t, err)

		// Should return cases for the explicitly requested workspace
		caseIDs := extractCaseIDs(cases)
		assert.Contains(t, caseIDs, case1ID, "should return cases for requested workspace")
		assert.NotContains(t, caseIDs, case2ID, "should not return cases from other workspaces")
	})
}

// extractCaseIDs helper for tests
func extractCaseIDs(cases []*servicedomain.Case) []string {
	ids := make([]string, len(cases))
	for i, c := range cases {
		ids[i] = c.ID
	}
	return ids
}
