package sql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestCaseStoreLinkAttachmentsToCaseRelinksExistingAttachment(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	caseObj := testutil.NewIsolatedCase(t, workspace.ID)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	attachment := servicedomain.NewAttachment(workspace.ID, "resume.pdf", "application/pdf", int64(len([]byte("%PDF-1.4 resume"))), servicedomain.AttachmentSourceUpload)
	attachment.MarkClean("No threats detected")
	require.NoError(t, store.Cases().SaveAttachment(ctx, attachment, nil))

	require.NoError(t, store.Cases().LinkAttachmentsToCase(ctx, workspace.ID, caseObj.ID, []string{attachment.ID}))

	storedAttachment, err := store.Cases().GetAttachment(ctx, workspace.ID, attachment.ID)
	require.NoError(t, err)
	assert.Equal(t, caseObj.ID, storedAttachment.CaseID)

	caseAttachments, err := store.Cases().ListCaseAttachments(ctx, workspace.ID, caseObj.ID)
	require.NoError(t, err)
	require.Len(t, caseAttachments, 1)
	assert.Equal(t, attachment.ID, caseAttachments[0].ID)
	assert.Equal(t, caseObj.ID, caseAttachments[0].CaseID)
}
