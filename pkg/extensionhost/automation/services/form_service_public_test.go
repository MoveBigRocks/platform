package automationservices

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/eventbus"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

type formServiceTestOutbox struct {
	events []interface{}
	err    error
}

func (o *formServiceTestOutbox) Publish(ctx context.Context, stream eventbus.Stream, event interface{}) error {
	if o.err != nil {
		return o.err
	}
	o.events = append(o.events, event)
	return nil
}

func (o *formServiceTestOutbox) PublishEvent(ctx context.Context, stream eventbus.Stream, event eventbus.Event) error {
	if o.err != nil {
		return o.err
	}
	o.events = append(o.events, event)
	return nil
}

func TestFormServiceCreatePublicSubmissionCommitsAndPublishes(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	form := servicedomain.NewFormSchema(workspace.ID, "Job Application", "job-application", "test")
	form.IsPublic = true
	form.Status = servicedomain.FormStatusActive
	require.NoError(t, store.Forms().CreateFormSchema(ctx, form))

	outbox := &formServiceTestOutbox{}
	service := NewFormServiceWithDeps(store.Forms(), nil, store, store, outbox)

	submission := servicedomain.NewPublicFormSubmission(workspace.ID, form.ID, shareddomain.MetadataFromMap(map[string]interface{}{
		"full_name": "Candidate Example",
		"email":     "candidate@example.com",
	}))
	submission.SubmitterEmail = "candidate@example.com"
	submission.SubmitterName = "Candidate Example"

	event := contracts.NewFormSubmittedEvent(
		form.ID,
		form.Slug,
		submission.ID,
		workspace.ID,
		submission.SubmitterEmail,
		submission.SubmitterName,
		submission.Data.ToInterfaceMap(),
	)

	require.NoError(t, service.CreatePublicSubmission(ctx, workspace.ID, submission, &event))

	stored, err := store.Forms().GetFormSubmission(ctx, submission.ID)
	require.NoError(t, err)
	assert.Equal(t, submission.ID, stored.ID)
	require.Len(t, outbox.events, 1)
	published, ok := outbox.events[0].(contracts.FormSubmittedEvent)
	require.True(t, ok)
	assert.Equal(t, event.SubmissionID, published.SubmissionID)
}

func TestFormServiceCreatePublicSubmissionRollsBackWhenPublishFails(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	form := servicedomain.NewFormSchema(workspace.ID, "Job Application", "job-application", "test")
	form.IsPublic = true
	form.Status = servicedomain.FormStatusActive
	require.NoError(t, store.Forms().CreateFormSchema(ctx, form))

	service := NewFormServiceWithDeps(store.Forms(), nil, store, store, &formServiceTestOutbox{
		err: errors.New("outbox unavailable"),
	})

	submission := servicedomain.NewPublicFormSubmission(workspace.ID, form.ID, shareddomain.MetadataFromMap(map[string]interface{}{
		"full_name": "Candidate Example",
		"email":     "candidate@example.com",
	}))
	submission.SubmitterEmail = "candidate@example.com"
	submission.SubmitterName = "Candidate Example"

	event := contracts.NewFormSubmittedEvent(
		form.ID,
		form.Slug,
		submission.ID,
		workspace.ID,
		submission.SubmitterEmail,
		submission.SubmitterName,
		submission.Data.ToInterfaceMap(),
	)

	err := service.CreatePublicSubmission(ctx, workspace.ID, submission, &event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish form submission event")

	submissions, getErr := store.Forms().ListFormSubmissions(ctx, form.ID)
	require.NoError(t, getErr)
	assert.Len(t, submissions, 0)
}
