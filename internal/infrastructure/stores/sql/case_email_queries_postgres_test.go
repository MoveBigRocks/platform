package sql_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/id"
)

func TestCaseStoreListCasesByMessageIDOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceOne := testutil.NewIsolatedWorkspace(t)
	workspaceTwo := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceOne))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceTwo))

	targetCase := testutil.NewIsolatedCase(t, workspaceOne.ID)
	targetCase.Subject = "Target case"
	otherCase := testutil.NewIsolatedCase(t, workspaceTwo.ID)
	otherCase.Subject = "Other workspace case"
	unmatchedCase := testutil.NewIsolatedCase(t, workspaceOne.ID)
	unmatchedCase.Subject = "Different message id"

	require.NoError(t, store.Cases().CreateCase(ctx, targetCase))
	require.NoError(t, store.Cases().CreateCase(ctx, otherCase))
	require.NoError(t, store.Cases().CreateCase(ctx, unmatchedCase))

	messageID := "<matching-message@example.com>"

	targetComm := servicedomain.NewCommunication(targetCase.ID, workspaceOne.ID, shareddomain.CommTypeEmail, "Outbound anchor")
	targetComm.Direction = shareddomain.DirectionOutbound
	targetComm.IsInternal = false
	targetComm.MessageID = messageID
	require.NoError(t, store.Cases().CreateCommunication(ctx, targetComm))

	otherComm := servicedomain.NewCommunication(otherCase.ID, workspaceTwo.ID, shareddomain.CommTypeEmail, "Other workspace anchor")
	otherComm.Direction = shareddomain.DirectionOutbound
	otherComm.IsInternal = false
	otherComm.MessageID = messageID
	require.NoError(t, store.Cases().CreateCommunication(ctx, otherComm))

	unmatchedComm := servicedomain.NewCommunication(unmatchedCase.ID, workspaceOne.ID, shareddomain.CommTypeEmail, "Different anchor")
	unmatchedComm.Direction = shareddomain.DirectionOutbound
	unmatchedComm.IsInternal = false
	unmatchedComm.MessageID = "<different-message@example.com>"
	require.NoError(t, store.Cases().CreateCommunication(ctx, unmatchedComm))

	matches, err := store.Cases().ListCasesByMessageID(ctx, workspaceOne.ID, messageID)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, targetCase.ID, matches[0].ID)
}

func TestCaseStoreListCasesBySubjectOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceOne := testutil.NewIsolatedWorkspace(t)
	workspaceTwo := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceOne))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceTwo))

	subject := "Password reset"
	first := testutil.NewIsolatedCase(t, workspaceOne.ID)
	first.Subject = subject
	second := testutil.NewIsolatedCase(t, workspaceOne.ID)
	second.Subject = subject
	otherWorkspace := testutil.NewIsolatedCase(t, workspaceTwo.ID)
	otherWorkspace.Subject = subject
	differentSubject := testutil.NewIsolatedCase(t, workspaceOne.ID)
	differentSubject.Subject = "Invoice question"

	require.NoError(t, store.Cases().CreateCase(ctx, first))
	require.NoError(t, store.Cases().CreateCase(ctx, second))
	require.NoError(t, store.Cases().CreateCase(ctx, otherWorkspace))
	require.NoError(t, store.Cases().CreateCase(ctx, differentSubject))

	matches, err := store.Cases().ListCasesBySubject(ctx, workspaceOne.ID, subject)
	require.NoError(t, err)
	require.Len(t, matches, 2)
	assert.ElementsMatch(t, []string{first.ID, second.ID}, []string{matches[0].ID, matches[1].ID})
}

func TestInboundEmailStoreGetEmailsByThreadOnPostgres(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	threadID := id.New()
	firstReceived := time.Now().UTC().Add(-2 * time.Minute)
	secondReceived := firstReceived.Add(time.Minute)

	first := servicedomain.NewInboundEmail(workspace.ID, "<thread-1@example.com>", "customer@example.com", "Need help", "First thread message")
	first.ThreadID = threadID
	first.ReceivedAt = firstReceived
	first.CreatedAt = firstReceived
	first.UpdatedAt = firstReceived
	require.NoError(t, store.InboundEmails().CreateInboundEmail(ctx, first))

	second := servicedomain.NewInboundEmail(workspace.ID, "<thread-2@example.com>", "support@example.com", "Re: Need help", "Second thread message")
	second.ThreadID = threadID
	second.ReceivedAt = secondReceived
	second.CreatedAt = secondReceived
	second.UpdatedAt = secondReceived
	require.NoError(t, store.InboundEmails().CreateInboundEmail(ctx, second))

	unrelated := servicedomain.NewInboundEmail(workspace.ID, "<other-thread@example.com>", "customer@example.com", "Separate thread", "Other thread message")
	unrelated.ThreadID = id.New()
	require.NoError(t, store.InboundEmails().CreateInboundEmail(ctx, unrelated))

	threadEmails, err := store.InboundEmails().GetEmailsByThread(ctx, threadID)
	require.NoError(t, err)
	require.Len(t, threadEmails, 2)
	assert.Equal(t, first.ID, threadEmails[0].ID)
	assert.Equal(t, second.ID, threadEmails[1].ID)
}
