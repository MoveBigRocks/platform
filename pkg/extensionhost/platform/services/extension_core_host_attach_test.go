package platformservices

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	shared "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

type fakeAttachmentUploader struct{ uploaded bool }

func (f *fakeAttachmentUploader) Upload(_ context.Context, _ *servicedomain.Attachment, _ io.Reader) error {
	f.uploaded = true
	return nil
}

type fakeAttachmentStore struct {
	saved     *servicedomain.Attachment
	getResult *servicedomain.Attachment
	getErr    error
	linked    []string
}

func (f *fakeAttachmentStore) SaveAttachment(_ context.Context, att *servicedomain.Attachment, _ io.Reader) error {
	f.saved = att
	return nil
}

func (f *fakeAttachmentStore) GetAttachment(_ context.Context, _, _ string) (*servicedomain.Attachment, error) {
	return f.getResult, f.getErr
}

func (f *fakeAttachmentStore) LinkAttachmentsToCase(_ context.Context, _, _ string, ids []string) error {
	f.linked = ids
	return nil
}

type fakeArtifactPublisher struct{ surface string }

func (f *fakeArtifactPublisher) PublishExtensionArtifact(_ context.Context, _, surface, _ string, _ []byte, _ string) (*platformdomain.ExtensionArtifactPublication, error) {
	f.surface = surface
	return &platformdomain.ExtensionArtifactPublication{}, nil
}

func TestCoreHostUploadAttachmentScopesToWorkspaceAndScans(t *testing.T) {
	uploader := &fakeAttachmentUploader{}
	store := &fakeAttachmentStore{}
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions:      &fakeExtensionResolver{ext: activeWorkspaceExtension("attachment:write")},
		Attachments:     uploader,
		AttachmentStore: store,
		Tenant:          &fakeTenantRunner{},
	})
	out, err := svc.UploadAttachment(context.Background(), "ext-1", runtimehost.UploadAttachmentInput{
		Filename: "resume.pdf",
		Content:  []byte("PDF-bytes"),
	})
	if err != nil {
		t.Fatalf("UploadAttachment: %v", err)
	}
	if !uploader.uploaded {
		t.Fatal("attachment must be uploaded and scanned")
	}
	if store.saved == nil || store.saved.WorkspaceID != "ws-1" {
		t.Fatalf("attachment must be saved in ws-1, got %+v", store.saved)
	}
	if out.WorkspaceID != "ws-1" || out.Filename != "resume.pdf" {
		t.Fatalf("unexpected returned attachment: %+v", out)
	}
}

func TestCoreHostUploadAttachmentRequiresPermission(t *testing.T) {
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions:      &fakeExtensionResolver{ext: activeWorkspaceExtension("attachment:read")}, // no write
		Attachments:     &fakeAttachmentUploader{},
		AttachmentStore: &fakeAttachmentStore{},
		Tenant:          &fakeTenantRunner{},
	})
	_, err := svc.UploadAttachment(context.Background(), "ext-1", runtimehost.UploadAttachmentInput{Filename: "x", Content: []byte("y")})
	if !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("expected forbidden without attachment:write, got %v", err)
	}
}

func TestCoreHostGetAttachmentMapsNotFound(t *testing.T) {
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions:      &fakeExtensionResolver{ext: activeWorkspaceExtension("attachment:read")},
		AttachmentStore: &fakeAttachmentStore{getErr: shared.ErrNotFound},
		Tenant:          &fakeTenantRunner{},
	})
	_, err := svc.GetAttachment(context.Background(), "ext-1", "missing")
	if !errors.Is(err, ErrCoreHostNotFound) {
		t.Fatalf("expected ErrCoreHostNotFound, got %v", err)
	}
}

func TestCoreHostPublishArtifactRequiresPermission(t *testing.T) {
	pub := &fakeArtifactPublisher{}
	svc := NewExtensionCoreHostService(CoreHostDeps{
		Extensions: &fakeExtensionResolver{ext: activeWorkspaceExtension("case:write")}, // no artifact:write
		Artifacts:  pub,
		Tenant:     &fakeTenantRunner{},
	})
	err := svc.PublishArtifact(context.Background(), "ext-1", runtimehost.PublishArtifactInput{Surface: "website", RelativePath: "index.html", Content: []byte("x")})
	if !errors.Is(err, ErrExtensionHostForbidden) {
		t.Fatalf("expected forbidden without artifact:write, got %v", err)
	}
	if pub.surface != "" {
		t.Fatal("publisher must not be called when permission is denied")
	}
}

// referenced to keep the automationservices import meaningful for readers of the
// rule-evaluation dependency; the engine itself is exercised in integration.
var _ = automationservices.NewFieldChanges
