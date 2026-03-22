package serviceapp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestFormSpecService_GetSpecBySlugAndList(t *testing.T) {
	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	workspaceID := testutil.CreateTestWorkspace(t, store, "form-specs")
	service := NewFormSpecService(store.FormSpecs(), store.Workspaces())
	ctx := context.Background()

	spec := servicedomain.NewFormSpec(workspaceID, "refund-request", "Refund Request")
	spec.Status = servicedomain.FormSpecStatusActive
	spec.IsPublic = true
	spec.SupportedChannels = []string{"web_chat", "operator_console"}
	spec.DescriptionMarkdown = "Collect the details needed to evaluate a refund request."
	require.NoError(t, store.FormSpecs().CreateFormSpec(ctx, spec))

	loadedSpec, err := service.GetFormSpecBySlug(ctx, workspaceID, "refund-request")
	require.NoError(t, err)
	assert.Equal(t, spec.ID, loadedSpec.ID)

	specs, err := service.ListWorkspaceFormSpecs(ctx, workspaceID)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, spec.ID, specs[0].ID)
}

func TestFormSpecService_CreateAndUpdateSpec(t *testing.T) {
	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	workspaceID := testutil.CreateTestWorkspace(t, store, "form-write")
	service := NewFormSpecService(store.FormSpecs(), store.Workspaces())
	ctx := context.Background()

	created, err := service.CreateFormSpec(ctx, CreateFormSpecParams{
		WorkspaceID:         workspaceID,
		Name:                "Refund Request",
		Slug:                "refund_request",
		DescriptionMarkdown: "Collect refund details.",
		SupportedChannels:   []string{"web_chat", "operator_console"},
		IsPublic:            true,
		Status:              servicedomain.FormSpecStatusActive,
		CreatedBy:           "user_123",
	})
	require.NoError(t, err)
	assert.Equal(t, "refund-request", created.Slug)
	assert.Equal(t, servicedomain.FormSpecStatusActive, created.Status)
	assert.Equal(t, "user_123", created.CreatedBy)

	publicKey := "refund-request-public"
	updated, err := service.UpdateFormSpec(ctx, created.ID, UpdateFormSpecParams{
		PublicKey:         &publicKey,
		SupportedChannels: &[]string{"web_chat"},
	})
	require.NoError(t, err)
	assert.Equal(t, publicKey, updated.PublicKey)
	assert.Equal(t, []string{"web_chat"}, updated.SupportedChannels)
}

func TestFormSpecService_GetSubmissionAndList(t *testing.T) {
	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	workspaceID := testutil.CreateTestWorkspace(t, store, "form-submissions")
	service := NewFormSpecService(store.FormSpecs(), store.Workspaces())
	ctx := context.Background()

	spec := servicedomain.NewFormSpec(workspaceID, "incident-report", "Incident Report")
	spec.Status = servicedomain.FormSpecStatusActive
	require.NoError(t, store.FormSpecs().CreateFormSpec(ctx, spec))

	submission := servicedomain.NewFormSubmission(workspaceID, spec.ID)
	submission.Status = servicedomain.FormSubmissionStatusSubmitted
	submission.Channel = "web_chat"
	submission.SubmitterEmail = "casey@example.com"
	submission.SubmitterName = "Casey"
	submission.CollectedFields.Set("summary", "Site is down")
	submission.MissingFields.Set("impact", "required")
	submission.ValidationErrors = []string{"impact is required"}
	submission.Metadata.Set("source", "widget")
	now := time.Now().UTC()
	submission.SubmittedAt = &now
	require.NoError(t, store.FormSpecs().CreateFormSubmission(ctx, submission))

	loadedSubmission, err := service.GetFormSubmission(ctx, submission.ID)
	require.NoError(t, err)
	assert.Equal(t, submission.ID, loadedSubmission.ID)

	submissions, err := service.ListFormSubmissions(ctx, workspaceID, servicedomain.FormSubmissionFilter{
		FormSpecID: spec.ID,
		Status:       servicedomain.FormSubmissionStatusSubmitted,
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, submissions, 1)
	assert.Equal(t, submission.ID, submissions[0].ID)
}

func TestFormSpecService_CreateSubmissionDefaultsToSubmitted(t *testing.T) {
	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	workspaceID := testutil.CreateTestWorkspace(t, store, "form-submit")
	service := NewFormSpecService(store.FormSpecs(), store.Workspaces())
	ctx := context.Background()

	spec := servicedomain.NewFormSpec(workspaceID, "incident-report", "Incident Report")
	spec.Status = servicedomain.FormSpecStatusActive
	spec.SupportedChannels = []string{"operator_console"}
	require.NoError(t, store.FormSpecs().CreateFormSpec(ctx, spec))

	submission, err := service.CreateFormSubmission(ctx, CreateFormSubmissionParams{
		FormSpecID:   spec.ID,
		SubmitterEmail: "ops@example.com",
		SubmitterName:  "Ops",
	})
	require.NoError(t, err)
	assert.Equal(t, spec.ID, submission.FormSpecID)
	assert.Equal(t, servicedomain.FormSubmissionStatusSubmitted, submission.Status)
	assert.Equal(t, "operator_console", submission.Channel)
	require.NotNil(t, submission.SubmittedAt)
}
