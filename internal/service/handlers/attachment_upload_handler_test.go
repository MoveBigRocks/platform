package servicehandlers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

type stubAttachmentUploader struct {
	upload func(ctx context.Context, attachment *servicedomain.Attachment, reader io.Reader) error
}

func (s stubAttachmentUploader) Upload(ctx context.Context, attachment *servicedomain.Attachment, reader io.Reader) error {
	if s.upload != nil {
		return s.upload(ctx, attachment, reader)
	}
	attachment.MarkClean("clean")
	attachment.SetS3Location("attachments", attachment.GenerateS3Key())
	return nil
}

type stubAttachmentCaseStore struct {
	saved []*servicedomain.Attachment
	cases map[string]*servicedomain.Case
}

func (s *stubAttachmentCaseStore) SaveAttachment(ctx context.Context, att *servicedomain.Attachment, data io.Reader) error {
	s.saved = append(s.saved, att)
	return nil
}

func (s *stubAttachmentCaseStore) GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*servicedomain.Case, error) {
	if s.cases == nil {
		return nil, assert.AnError
	}
	caseObj, ok := s.cases[caseID]
	if !ok || caseObj.WorkspaceID != workspaceID {
		return nil, assert.AnError
	}
	return caseObj, nil
}

func TestAttachmentUploadHandler_HandleAgentUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &stubAttachmentCaseStore{}
	handler := NewAttachmentUploadHandler(stubAttachmentUploader{}, store)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("workspace_id", "ws_123"))
	require.NoError(t, writer.WriteField("description", "Resume"))
	part, err := writer.CreateFormFile("file", "resume.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("%PDF-1.4"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req
	c.Set("auth_context", &platformdomain.AuthContext{
		WorkspaceID: "ws_123",
		Permissions: []string{platformdomain.PermissionAttachmentWrite},
		Principal:   &platformdomain.Agent{ID: "agent_123", WorkspaceID: "ws_123"},
	})

	handler.HandleAgentUpload(c)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Len(t, store.saved, 1)
	assert.Equal(t, "ws_123", store.saved[0].WorkspaceID)
	assert.Equal(t, "agent_123", store.saved[0].UploadedBy)
	assert.Equal(t, servicedomain.AttachmentSourceAgent, store.saved[0].Source)
	assert.Equal(t, "Resume", store.saved[0].Description)
	assert.Equal(t, servicedomain.AttachmentStatusClean, store.saved[0].Status)
}

func TestAttachmentUploadHandler_HandleAdminUploadRequiresWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewAttachmentUploadHandler(stubAttachmentUploader{}, &stubAttachmentCaseStore{})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "resume.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("resume"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/actions/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req
	c.Set("user_id", "user_123")

	handler.HandleAdminUpload(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
