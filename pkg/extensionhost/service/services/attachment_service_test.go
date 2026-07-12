package serviceapp

import (
	"context"
	"io"
	"testing"

	"github.com/movebigrocks/platform/internal/infrastructure/antivirus"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestNewAttachmentServiceRequiresRealScannerWhenConfigured(t *testing.T) {
	t.Parallel()

	service, err := NewAttachmentService(AttachmentServiceConfig{
		MalwareScanningRequired: true,
	})
	if err == nil {
		t.Fatal("expected construction to fail without ClamAV")
	}
	if service != nil {
		t.Fatal("expected no attachment service when required scanning is unavailable")
	}
}

func TestAttachmentUploadLimitsActualBytesInsteadOfTrustingDeclaredSize(t *testing.T) {
	t.Parallel()

	service := &AttachmentService{scanner: antivirus.NewMockScanner(true)}
	attachment := servicedomain.NewAttachment(
		"019d5907-aab8-7ff8-9bb5-a016937aeac0",
		"evidence.pdf",
		"application/pdf",
		1,
		servicedomain.AttachmentSourceUpload,
	)

	err := service.Upload(
		context.Background(),
		attachment,
		io.LimitReader(zeroReader{}, servicedomain.MaxAttachmentSize+1),
	)
	if err == nil {
		t.Fatal("expected oversized upload to be rejected")
	}
	if attachment.Status != servicedomain.AttachmentStatusError {
		t.Fatalf("expected error status, got %s", attachment.Status)
	}
}
