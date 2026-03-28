//go:build integration

package servicehandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	serviceresolvers "github.com/movebigrocks/platform/internal/service/resolvers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/workflowproof"
)

func TestAttachmentUploadHandler_UploadVisibleInCaseSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	operator := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, operator))

	caseService := serviceapp.NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	caseObj, err := caseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspace.ID,
		Subject:      "Attachment review",
		ContactEmail: "customer@example.com",
		ContactName:  "Casey Customer",
	})
	require.NoError(t, err)

	attachmentService, s3Server := newTestAttachmentService(t)
	handler := NewAttachmentUploadHandler(attachmentService, store.Cases())

	router := gin.New()
	router.POST("/attachments", func(c *gin.Context) {
		c.Set("auth_context", &platformdomain.AuthContext{
			Principal:     &platformdomain.Agent{ID: testutil.UniqueUserID(t), WorkspaceID: workspace.ID},
			PrincipalType: platformdomain.PrincipalTypeAgent,
			WorkspaceID:   workspace.ID,
			WorkspaceIDs:  []string{workspace.ID},
			Permissions:   []string{platformdomain.PermissionAttachmentWrite},
		})
		handler.HandleAgentUpload(c)
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("workspace_id", workspace.ID))
	require.NoError(t, writer.WriteField("case_id", caseObj.ID))
	require.NoError(t, writer.WriteField("description", "Customer invoice screenshot"))
	part, err := writer.CreateFormFile("file", "invoice.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("%PDF-1.4 attachment body"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusCreated, resp.Code)

	var uploadResult map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &uploadResult))
	attachmentID, ok := uploadResult["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, attachmentID)

	attachment, err := store.Cases().GetAttachment(ctx, workspace.ID, attachmentID)
	require.NoError(t, err)
	assert.Equal(t, caseObj.ID, attachment.CaseID)
	assert.Equal(t, "invoice.pdf", attachment.Filename)
	assert.Equal(t, "Customer invoice screenshot", attachment.Description)
	assert.Equal(t, "clean", string(attachment.Status))
	assert.NotEmpty(t, attachment.S3Key)
	assert.Contains(t, attachment.S3Key, attachment.ID)

	puts := s3Server.PutRequests()
	require.Len(t, puts, 1)
	assert.Equal(t, http.MethodPut, puts[0].Method)
	assert.Contains(t, puts[0].Path, attachment.ID)
	assert.Contains(t, puts[0].Path, "invoice.pdf")

	resolver := serviceresolvers.NewResolver(serviceresolvers.Config{CaseService: caseService})
	authCtx := graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Principal:     operator,
		PrincipalType: platformdomain.PrincipalTypeUser,
		WorkspaceID:   workspace.ID,
		WorkspaceIDs:  []string{workspace.ID},
		Permissions:   []string{platformdomain.PermissionCaseRead},
	})

	caseResolver, err := resolver.Case(authCtx, caseObj.ID)
	require.NoError(t, err)
	attachments, err := caseResolver.Attachments(authCtx)
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	assert.Equal(t, attachmentID, string(attachments[0].ID()))
	assert.Equal(t, "invoice.pdf", attachments[0].Filename())
	assert.Equal(t, "clean", attachments[0].Status())

	workflowproof.WriteJSON(t, "case-operator-attachment-upload", map[string]interface{}{
		"workspace_id":   workspace.ID,
		"case_id":        caseObj.ID,
		"attachment_id":  attachment.ID,
		"status":         attachment.Status,
		"filename":       attachment.Filename,
		"storage_key":    attachment.S3Key,
		"visible_count":  len(attachments),
		"upload_request": strings.TrimSpace(puts[0].Path),
	})
}
