package platformservices

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	shared "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

// coreHostAttachmentUploader stores and virus-scans attachment bytes; the core
// attachment service satisfies it.
type coreHostAttachmentUploader interface {
	Upload(ctx context.Context, attachment *servicedomain.Attachment, reader io.Reader) error
}

// coreHostAttachmentStore persists attachment metadata and links; the core case
// store satisfies it.
type coreHostAttachmentStore interface {
	SaveAttachment(ctx context.Context, att *servicedomain.Attachment, data io.Reader) error
	GetAttachment(ctx context.Context, workspaceID, attID string) (*servicedomain.Attachment, error)
	LinkAttachmentsToCase(ctx context.Context, workspaceID, caseID string, attachmentIDs []string) error
}

// coreHostRuleEngine runs automation rules for a case; the core rules engine
// satisfies it.
type coreHostRuleEngine interface {
	EvaluateRulesForCase(ctx context.Context, caseObj *servicedomain.Case, event string, changes *automationservices.FieldChanges) error
}

// coreHostArtifactPublisher publishes generated content to an extension's
// artifact surfaces; the core extension service satisfies it.
type coreHostArtifactPublisher interface {
	PublishExtensionArtifact(ctx context.Context, extensionID, surface, relativePath string, content []byte, actorID string) (*platformdomain.ExtensionArtifactPublication, error)
}

// UploadAttachment stores and scans a file in the extension's workspace and
// records its metadata.
func (s *ExtensionCoreHostService) UploadAttachment(ctx context.Context, extensionID string, input runtimehost.UploadAttachmentInput) (*runtimehost.HostAttachment, error) {
	if s.attachments == nil || s.attachmentStore == nil {
		return nil, fmt.Errorf("attachment host service is not configured")
	}
	if strings.TrimSpace(input.Filename) == "" {
		return nil, fmt.Errorf("filename is required")
	}
	if len(input.Content) == 0 {
		return nil, fmt.Errorf("attachment content is required")
	}
	var stored *servicedomain.Attachment
	err := s.runScoped(ctx, extensionID, "attachment:write", func(txCtx context.Context, workspaceID string) error {
		att := servicedomain.NewAttachment(workspaceID, strings.TrimSpace(input.Filename), strings.TrimSpace(input.ContentType), int64(len(input.Content)), servicedomain.AttachmentSourceUpload)
		att.Description = strings.TrimSpace(input.Description)
		att.ContactID = strings.TrimSpace(input.ContactID)
		if err := s.attachments.Upload(txCtx, att, bytes.NewReader(input.Content)); err != nil {
			return err
		}
		if err := s.attachmentStore.SaveAttachment(txCtx, att, nil); err != nil {
			return err
		}
		stored = att
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hostAttachmentFromDomain(stored), nil
}

// GetAttachment returns an attachment in the extension's workspace.
func (s *ExtensionCoreHostService) GetAttachment(ctx context.Context, extensionID, attachmentID string) (*runtimehost.HostAttachment, error) {
	if s.attachmentStore == nil {
		return nil, fmt.Errorf("attachment host service is not configured")
	}
	var found *servicedomain.Attachment
	err := s.runScoped(ctx, extensionID, "attachment:read", func(txCtx context.Context, workspaceID string) error {
		a, getErr := s.attachmentStore.GetAttachment(txCtx, workspaceID, strings.TrimSpace(attachmentID))
		found = a
		return getErr
	})
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrCoreHostNotFound
		}
		return nil, err
	}
	return hostAttachmentFromDomain(found), nil
}

// LinkAttachmentsToCase links stored attachments to a case in the extension's
// workspace.
func (s *ExtensionCoreHostService) LinkAttachmentsToCase(ctx context.Context, extensionID, caseID string, attachmentIDs []string) error {
	if s.attachmentStore == nil {
		return fmt.Errorf("attachment host service is not configured")
	}
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return fmt.Errorf("case id is required")
	}
	err := s.runScoped(ctx, extensionID, "attachment:write", func(txCtx context.Context, workspaceID string) error {
		return s.attachmentStore.LinkAttachmentsToCase(txCtx, workspaceID, caseID, attachmentIDs)
	})
	if errors.Is(err, shared.ErrNotFound) {
		return ErrCoreHostNotFound
	}
	return err
}

// EvaluateRulesForCase runs automation rules for a case in the extension's
// workspace, on the given event and field changes.
func (s *ExtensionCoreHostService) EvaluateRulesForCase(ctx context.Context, extensionID string, input runtimehost.EvaluateRulesInput) error {
	if s.rules == nil {
		return fmt.Errorf("automation host service is not configured")
	}
	caseID := strings.TrimSpace(input.CaseID)
	if caseID == "" || strings.TrimSpace(input.Event) == "" {
		return fmt.Errorf("caseId and event are required")
	}
	err := s.runScoped(ctx, extensionID, "automation:write", func(txCtx context.Context, workspaceID string) error {
		caseObj, getErr := s.cases.GetCaseInWorkspace(txCtx, workspaceID, caseID)
		if getErr != nil {
			return getErr
		}
		changes := automationservices.NewFieldChanges()
		for k, v := range input.Changes {
			changes.Set(k, v)
		}
		return s.rules.EvaluateRulesForCase(txCtx, caseObj, strings.TrimSpace(input.Event), changes)
	})
	if errors.Is(err, shared.ErrNotFound) {
		return ErrCoreHostNotFound
	}
	return err
}

// PublishArtifact publishes content to one of the calling extension's declared
// artifact surfaces. The extension service enforces surface ownership, so this
// does not run inside the workspace tenant transaction.
func (s *ExtensionCoreHostService) PublishArtifact(ctx context.Context, extensionID string, input runtimehost.PublishArtifactInput) error {
	if s == nil || s.extensions == nil || s.artifacts == nil {
		return fmt.Errorf("artifact host service is not configured")
	}
	extension, err := s.resolveExtension(ctx, extensionID, "artifact:write")
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.Surface) == "" || strings.TrimSpace(input.RelativePath) == "" {
		return fmt.Errorf("surface and relativePath are required")
	}
	_, err = s.artifacts.PublishExtensionArtifact(ctx, extension.ID, strings.TrimSpace(input.Surface), strings.TrimSpace(input.RelativePath), input.Content, strings.TrimSpace(input.ActorID))
	return err
}

func hostAttachmentFromDomain(a *servicedomain.Attachment) *runtimehost.HostAttachment {
	if a == nil {
		return nil
	}
	return &runtimehost.HostAttachment{
		ID:          a.ID,
		WorkspaceID: a.WorkspaceID,
		Filename:    a.Filename,
		ContentType: a.ContentType,
		Size:        a.Size,
		Status:      string(a.Status),
		SHA256Hash:  a.SHA256Hash,
		CaseID:      a.CaseID,
		ContactID:   a.ContactID,
		Source:      string(a.Source),
	}
}
