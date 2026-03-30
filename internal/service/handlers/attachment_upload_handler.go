package servicehandlers

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/antivirus"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/middleware"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

type attachmentUploader interface {
	Upload(ctx context.Context, attachment *servicedomain.Attachment, reader io.Reader) error
}

type attachmentCaseStore interface {
	SaveAttachment(ctx context.Context, att *servicedomain.Attachment, data io.Reader) error
	GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*servicedomain.Case, error)
}

type AttachmentUploadHandler struct {
	uploader  attachmentUploader
	caseStore attachmentCaseStore
	logger    *logger.Logger
}

func NewAttachmentUploadHandler(uploader attachmentUploader, caseStore attachmentCaseStore) *AttachmentUploadHandler {
	return &AttachmentUploadHandler{
		uploader:  uploader,
		caseStore: caseStore,
		logger:    logger.New().WithField("handler", "attachment_upload"),
	}
}

func (h *AttachmentUploadHandler) HandleAgentUpload(c *gin.Context) {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !authCtx.HasPermission(platformdomain.PermissionAttachmentWrite) {
		middleware.RespondWithError(c, http.StatusForbidden, "Attachment write permission required")
		return
	}

	workspaceID := strings.TrimSpace(c.PostForm("workspace_id"))
	if workspaceID == "" {
		workspaceID = authCtx.WorkspaceID
	}
	if workspaceID == "" || !authCtx.HasWorkspaceAccess(workspaceID) {
		middleware.RespondWithError(c, http.StatusForbidden, "Workspace access denied")
		return
	}

	uploadedBy := ""
	if authCtx.Principal != nil {
		uploadedBy = authCtx.Principal.GetID()
	}

	h.handleUpload(c, workspaceID, uploadedBy, servicedomain.AttachmentSourceAgent)
}

func (h *AttachmentUploadHandler) HandleAdminUpload(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.PostForm("workspace_id"))
	if workspaceID == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "workspace_id is required")
		return
	}

	h.handleUpload(c, workspaceID, strings.TrimSpace(c.GetString("user_id")), servicedomain.AttachmentSourceUpload)
}

func (h *AttachmentUploadHandler) handleUpload(c *gin.Context, workspaceID, uploadedBy string, source servicedomain.AttachmentSource) {
	if h.uploader == nil || h.caseStore == nil {
		middleware.RespondWithError(c, http.StatusServiceUnavailable, "Attachment uploads are not configured")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "file is required")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to open uploaded file")
		return
	}
	defer file.Close()

	caseID := strings.TrimSpace(c.PostForm("case_id"))
	if caseID != "" {
		if _, err := h.caseStore.GetCaseInWorkspace(c.Request.Context(), workspaceID, caseID); err != nil {
			middleware.RespondWithError(c, http.StatusBadRequest, "case_id must belong to the selected workspace")
			return
		}
	}

	contentType := strings.TrimSpace(c.PostForm("content_type"))
	if contentType == "" {
		contentType = strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(pathExt(fileHeader.Filename)))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	attachment := servicedomain.NewAttachment(workspaceID, fileHeader.Filename, contentType, fileHeader.Size, source)
	attachment.CaseID = caseID
	attachment.UploadedBy = uploadedBy
	attachment.Description = strings.TrimSpace(c.PostForm("description"))

	if err := h.uploader.Upload(c.Request.Context(), attachment, file); err != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(err, antivirus.ErrMalwareDetected) {
			statusCode = http.StatusUnprocessableEntity
		} else if !isUploadValidationError(err) {
			statusCode = http.StatusInternalServerError
		}
		h.logger.WithError(err).Warn("Attachment upload failed", "workspace_id", workspaceID, "filename", fileHeader.Filename)
		middleware.RespondWithError(c, statusCode, err.Error())
		return
	}

	if err := h.caseStore.SaveAttachment(c.Request.Context(), attachment, nil); err != nil {
		h.logger.WithError(err).Error("Failed to persist attachment metadata", "attachment_id", attachment.ID)
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to save attachment metadata")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          attachment.ID,
		"workspaceID": attachment.WorkspaceID,
		"caseID":      attachment.CaseID,
		"filename":    attachment.Filename,
		"contentType": attachment.ContentType,
		"size":        attachment.Size,
		"status":      attachment.Status,
		"description": attachment.Description,
		"source":      attachment.Source,
	})
}

func isUploadValidationError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "invalid attachment") ||
		strings.Contains(message, "file extension is blocked") ||
		strings.Contains(message, "content type") ||
		strings.Contains(message, "attachment size")
}

func pathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx == -1 {
		return ""
	}
	return name[idx:]
}
