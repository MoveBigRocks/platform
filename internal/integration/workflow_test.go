//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/testutil"
)

// =============================================================================
// TEST SUITE SETUP
// =============================================================================

// trySetupTestStore attempts to create a test store, returning an error if database is unavailable.
// This allows tests to be gracefully skipped when the database is not available.
func trySetupTestStore(t *testing.T) (stores.Store, func(), error) {
	t.Helper()

	testutil.SetupTestEnv(t)
	store, cleanup := testutil.SetupTestSQLStore(t)

	// Verify we can actually use the store by pinging it
	ctx := context.Background()
	_, err := store.Workspaces().ListWorkspaces(ctx)
	if err != nil {
		cleanup()
		return nil, nil, err
	}

	return store, cleanup, nil
}

// workflowTestSuite holds shared dependencies for workflow tests
type workflowTestSuite struct {
	t       *testing.T
	ctx     context.Context
	store   stores.Store
	cleanup func()

	// Services
	caseService *serviceapp.CaseService

	// Test fixtures
	workspace *platformdomain.Workspace
	user      *platformdomain.User
	contact   *platformdomain.Contact
}

// setupWorkflowTest creates a complete test environment with services.
// Uses SQLite for testing. Tests are skipped if the database is not available.
func setupWorkflowTest(t *testing.T) *workflowTestSuite {
	t.Helper()

	// Try to create the store - skip test if database not available
	store, cleanup, err := trySetupTestStore(t)
	if err != nil {
		t.Skipf("Skipping integration test: database not available: %v", err)
		return nil
	}

	ctx := context.Background()
	// Create workspace
	workspace := testutil.NewIsolatedWorkspace(t)
	err = store.Workspaces().CreateWorkspace(ctx, workspace)
	require.NoError(t, err, "Failed to create workspace")

	// Create user
	user := testutil.NewIsolatedUser(t, workspace.ID)
	err = store.Users().CreateUser(ctx, user)
	require.NoError(t, err, "Failed to create user")

	// Create contact
	contact := testutil.NewIsolatedContact(t, workspace.ID)
	err = store.Contacts().CreateContact(ctx, contact)
	require.NoError(t, err, "Failed to create contact")

	// Initialize services - using nil outbox for tests (events won't be published)
	caseService := serviceapp.NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)

	return &workflowTestSuite{
		t:           t,
		ctx:         ctx,
		store:       store,
		cleanup:     cleanup,
		caseService: caseService,
		workspace:   workspace,
		user:        user,
		contact:     contact,
	}
}

func (s *workflowTestSuite) teardown() {
	s.cleanup()
}

// =============================================================================
// CASE LIFECYCLE WORKFLOW TESTS
// =============================================================================

func TestWorkflow_CaseCreationAndRetrieval(t *testing.T) {
	suite := setupWorkflowTest(t)
	if suite == nil {
		return // Test was skipped
	}
	defer suite.teardown()

	// Create a case
	params := serviceapp.CreateCaseParams{
		WorkspaceID:  suite.workspace.ID,
		Subject:      "Test support request",
		Description:  "I need help with something",
		ContactEmail: suite.contact.Email,
		ContactName:  suite.contact.Name,
		Channel:      servicedomain.CaseChannelWeb,
		Priority:     servicedomain.CasePriorityMedium,
	}

	createdCase, err := suite.caseService.CreateCase(suite.ctx, params)
	require.NoError(t, err, "Case creation should succeed")
	require.NotNil(t, createdCase, "Created case should not be nil")

	// Verify case was created with correct values
	assert.Equal(t, suite.workspace.ID, createdCase.WorkspaceID)
	assert.Equal(t, params.Subject, createdCase.Subject)
	assert.Equal(t, servicedomain.CaseStatusNew, createdCase.Status, "New case should have 'new' status")
	assert.NotEmpty(t, createdCase.ID, "Case should have an ID")
	assert.NotEmpty(t, createdCase.HumanID, "Case should have a human-readable ID")

	// Retrieve case by ID
	retrievedCase, err := suite.caseService.GetCase(suite.ctx, createdCase.ID)
	require.NoError(t, err, "Case retrieval should succeed")
	assert.Equal(t, createdCase.ID, retrievedCase.ID)
	assert.Equal(t, createdCase.Subject, retrievedCase.Subject)
}

func TestWorkflow_CaseStatusTransitions(t *testing.T) {
	suite := setupWorkflowTest(t)
	if suite == nil {
		return
	}
	defer suite.teardown()

	// Create a case
	params := serviceapp.CreateCaseParams{
		WorkspaceID:  suite.workspace.ID,
		Subject:      "Status transition test",
		ContactEmail: suite.contact.Email,
		Channel:      servicedomain.CaseChannelEmail,
	}

	caseObj, err := suite.caseService.CreateCase(suite.ctx, params)
	require.NoError(t, err)

	// Verify initial status
	assert.Equal(t, servicedomain.CaseStatusNew, caseObj.Status)

	// Transition: NEW -> OPEN (via assignment)
	err = suite.caseService.AssignCase(suite.ctx, caseObj.ID, suite.user.ID, "")
	require.NoError(t, err, "Assignment should succeed")

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusOpen, caseObj.Status, "Assigned case should be 'open'")
	assert.Equal(t, suite.user.ID, caseObj.AssignedToID)

	// Transition: OPEN -> PENDING
	err = suite.caseService.SetCaseStatus(suite.ctx, caseObj.ID, servicedomain.CaseStatusPending)
	require.NoError(t, err, "Status change to pending should succeed")

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusPending, caseObj.Status)

	// Transition: PENDING -> RESOLVED
	err = suite.caseService.MarkCaseResolved(suite.ctx, caseObj.ID, time.Now())
	require.NoError(t, err, "Marking case resolved should succeed")

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusResolved, caseObj.Status)
	assert.NotNil(t, caseObj.ResolvedAt, "Resolved case should have ResolvedAt timestamp")

	// Transition: RESOLVED -> CLOSED
	err = suite.caseService.CloseCase(suite.ctx, caseObj.ID)
	require.NoError(t, err, "Closing case should succeed")

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusClosed, caseObj.Status)
	assert.NotNil(t, caseObj.ClosedAt, "Closed case should have ClosedAt timestamp")
}

func TestWorkflow_CaseReopening(t *testing.T) {
	suite := setupWorkflowTest(t)
	if suite == nil {
		return
	}
	defer suite.teardown()

	// Create and resolve a case
	params := serviceapp.CreateCaseParams{
		WorkspaceID:  suite.workspace.ID,
		Subject:      "Reopen test",
		ContactEmail: suite.contact.Email,
		Channel:      servicedomain.CaseChannelWeb,
	}

	caseObj, err := suite.caseService.CreateCase(suite.ctx, params)
	require.NoError(t, err)

	// Assign and resolve
	err = suite.caseService.AssignCase(suite.ctx, caseObj.ID, suite.user.ID, "")
	require.NoError(t, err)

	err = suite.caseService.MarkCaseResolved(suite.ctx, caseObj.ID, time.Now())
	require.NoError(t, err)

	// Reopen the case
	err = suite.caseService.ReopenCase(suite.ctx, caseObj.ID)
	require.NoError(t, err, "Reopening should succeed")

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusOpen, caseObj.Status, "Reopened case should be 'open'")
}

func TestWorkflow_CaseTagging(t *testing.T) {
	suite := setupWorkflowTest(t)
	if suite == nil {
		return
	}
	defer suite.teardown()

	// Create a case
	params := serviceapp.CreateCaseParams{
		WorkspaceID:  suite.workspace.ID,
		Subject:      "Tagging test",
		ContactEmail: suite.contact.Email,
		Channel:      servicedomain.CaseChannelWeb,
	}

	caseObj, err := suite.caseService.CreateCase(suite.ctx, params)
	require.NoError(t, err)

	// Add tags
	err = suite.caseService.AddCaseTag(suite.ctx, caseObj.ID, "urgent")
	require.NoError(t, err)

	err = suite.caseService.AddCaseTag(suite.ctx, caseObj.ID, "billing")
	require.NoError(t, err)

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Contains(t, caseObj.Tags, "urgent")
	assert.Contains(t, caseObj.Tags, "billing")

	// Remove a tag
	err = suite.caseService.RemoveCaseTag(suite.ctx, caseObj.ID, "urgent")
	require.NoError(t, err)

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.NotContains(t, caseObj.Tags, "urgent")
	assert.Contains(t, caseObj.Tags, "billing")
}

func TestWorkflow_CasePriorityChange(t *testing.T) {
	suite := setupWorkflowTest(t)
	if suite == nil {
		return
	}
	defer suite.teardown()

	// Create a case with default priority
	params := serviceapp.CreateCaseParams{
		WorkspaceID:  suite.workspace.ID,
		Subject:      "Priority test",
		ContactEmail: suite.contact.Email,
		Channel:      servicedomain.CaseChannelWeb,
	}

	caseObj, err := suite.caseService.CreateCase(suite.ctx, params)
	require.NoError(t, err)

	// Escalate priority
	err = suite.caseService.SetCasePriority(suite.ctx, caseObj.ID, servicedomain.CasePriorityHigh)
	require.NoError(t, err)

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CasePriorityHigh, caseObj.Priority)

	// De-escalate priority
	err = suite.caseService.SetCasePriority(suite.ctx, caseObj.ID, servicedomain.CasePriorityLow)
	require.NoError(t, err)

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CasePriorityLow, caseObj.Priority)
}

// =============================================================================
// CASE ASSIGNMENT WORKFLOW TESTS
// =============================================================================

func TestWorkflow_AssignmentAndUnassignment(t *testing.T) {
	suite := setupWorkflowTest(t)
	if suite == nil {
		return
	}
	defer suite.teardown()

	// Create a second user for assignment tests
	secondUser := testutil.NewIsolatedUser(t, suite.workspace.ID)
	err := suite.store.Users().CreateUser(suite.ctx, secondUser)
	require.NoError(t, err)

	// Create a case
	params := serviceapp.CreateCaseParams{
		WorkspaceID:  suite.workspace.ID,
		Subject:      "Assignment test",
		ContactEmail: suite.contact.Email,
		Channel:      servicedomain.CaseChannelWeb,
	}

	caseObj, err := suite.caseService.CreateCase(suite.ctx, params)
	require.NoError(t, err)

	// Initially unassigned
	assert.Empty(t, caseObj.AssignedToID)

	// Assign to first user
	err = suite.caseService.AssignCase(suite.ctx, caseObj.ID, suite.user.ID, "")
	require.NoError(t, err)

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, suite.user.ID, caseObj.AssignedToID)

	// Reassign to second user
	err = suite.caseService.AssignCase(suite.ctx, caseObj.ID, secondUser.ID, "")
	require.NoError(t, err)

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, secondUser.ID, caseObj.AssignedToID)

	// Unassign
	err = suite.caseService.UnassignCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)

	caseObj, err = suite.caseService.GetCase(suite.ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Empty(t, caseObj.AssignedToID)
}

// =============================================================================
// ERROR CASES
// =============================================================================

func TestWorkflow_GetNonExistentCase(t *testing.T) {
	suite := setupWorkflowTest(t)
	if suite == nil {
		return
	}
	defer suite.teardown()

	_, err := suite.caseService.GetCase(suite.ctx, "nonexistent-case-id")
	assert.Error(t, err, "Getting nonexistent case should error")
}

func TestWorkflow_InvalidStatusTransition(t *testing.T) {
	suite := setupWorkflowTest(t)
	if suite == nil {
		return
	}
	defer suite.teardown()

	// Create a case
	params := serviceapp.CreateCaseParams{
		WorkspaceID:  suite.workspace.ID,
		Subject:      "Invalid transition test",
		ContactEmail: suite.contact.Email,
		Channel:      servicedomain.CaseChannelWeb,
	}

	caseObj, err := suite.caseService.CreateCase(suite.ctx, params)
	require.NoError(t, err)

	// Try to close a NEW case (should fail - can only close RESOLVED cases)
	err = suite.caseService.CloseCase(suite.ctx, caseObj.ID)
	assert.Error(t, err, "Should not be able to close a NEW case directly")
}
