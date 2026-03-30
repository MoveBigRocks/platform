package servicedomain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAttachment(t *testing.T) {
	att := NewAttachment("ws-123", "document.pdf", "application/pdf", 1024, AttachmentSourceUpload)

	assert.NotEmpty(t, att.ID)
	assert.Equal(t, "ws-123", att.WorkspaceID)
	assert.Equal(t, "document.pdf", att.Filename)
	assert.Equal(t, "application/pdf", att.ContentType)
	assert.Equal(t, int64(1024), att.Size)
	assert.Equal(t, AttachmentStatusPending, att.Status)
	assert.Equal(t, AttachmentSourceUpload, att.Source)
	assert.NotNil(t, att.Metadata)
}

func TestAttachment_SetS3Location(t *testing.T) {
	att := NewAttachment("ws-123", "test.pdf", "application/pdf", 100, AttachmentSourceUpload)
	beforeUpdate := att.UpdatedAt
	att.SetS3Location("my-bucket", "path/to/file.pdf")

	assert.Equal(t, "my-bucket", att.S3Bucket)
	assert.Equal(t, "path/to/file.pdf", att.S3Key)
	assert.True(t, att.UpdatedAt.After(beforeUpdate))
}

func TestAttachment_ScanningLifecycle(t *testing.T) {
	t.Run("pending -> scanning -> clean", func(t *testing.T) {
		att := NewAttachment("ws-123", "safe.pdf", "application/pdf", 100, AttachmentSourceEmail)
		assert.Equal(t, AttachmentStatusPending, att.Status)
		assert.Nil(t, att.ScannedAt)

		att.MarkScanning()
		assert.Equal(t, AttachmentStatusScanning, att.Status)
		assert.Nil(t, att.ScannedAt) // Not scanned yet

		att.MarkClean("OK")
		assert.Equal(t, AttachmentStatusClean, att.Status)
		assert.Equal(t, "OK", att.ScanResult)
		require.NotNil(t, att.ScannedAt)
		assert.True(t, att.IsClean())
		assert.False(t, att.IsQuarantined())
	})

	t.Run("pending -> scanning -> infected", func(t *testing.T) {
		att := NewAttachment("ws-123", "malware.exe", "application/octet-stream", 100, AttachmentSourceEmail)

		att.MarkScanning()
		att.MarkInfected("Win.Trojan.Generic-12345")

		assert.Equal(t, AttachmentStatusInfected, att.Status)
		assert.Equal(t, "Win.Trojan.Generic-12345", att.ScanResult)
		require.NotNil(t, att.ScannedAt)
		assert.False(t, att.IsClean())
		assert.True(t, att.IsQuarantined())
	})

	t.Run("pending -> scanning -> error", func(t *testing.T) {
		att := NewAttachment("ws-123", "corrupt.bin", "application/octet-stream", 100, AttachmentSourceUpload)

		att.MarkScanning()
		att.MarkError("Scan timeout")

		assert.Equal(t, AttachmentStatusError, att.Status)
		assert.Equal(t, "Scan timeout", att.ScanResult)
		require.NotNil(t, att.ScannedAt)
		assert.False(t, att.IsClean())
		assert.False(t, att.IsQuarantined())
	})
}

func TestAttachment_GenerateS3Key(t *testing.T) {
	att := NewAttachment("ws-123", "report.pdf", "application/pdf", 100, AttachmentSourceUpload)
	key := att.GenerateS3Key()

	assert.Contains(t, key, "ws-123")
	assert.Contains(t, key, "attachments")
	assert.Contains(t, key, "/report.pdf")
	assert.Contains(t, key, "report.pdf")
}

func TestAttachment_GetS3Path(t *testing.T) {
	att := NewAttachment("ws-123", "test.pdf", "application/pdf", 100, AttachmentSourceUpload)
	att.SetS3Location("my-bucket", "path/to/file.pdf")

	path := att.GetS3Path()
	assert.Equal(t, "s3://my-bucket/path/to/file.pdf", path)
}

func TestIsBlockedExtension(t *testing.T) {
	// These are in BlockedExtensions map
	blocked := []string{"virus.exe", "script.bat", "macro.vbs", "program.com", "powershell.ps1"}
	allowed := []string{"document.pdf", "image.png", "report.xlsx", "readme.txt", "shell.sh"}

	for _, filename := range blocked {
		assert.True(t, IsBlockedExtension(filename), "Expected %s to be blocked", filename)
	}

	for _, filename := range allowed {
		assert.False(t, IsBlockedExtension(filename), "Expected %s to be allowed", filename)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.pdf", "normal.pdf"},
		{"../../../etc/passwd", "passwd"}, // filepath.Base extracts just filename
		{"file with spaces.txt", "file with spaces.txt"},
		{"file\x00name.txt", "filename.txt"},
		{"/absolute/path/file.txt", "file.txt"}, // filepath.Base extracts just filename
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		assert.Equal(t, tt.expected, result, "sanitizeFilename(%q)", tt.input)
	}
}

func TestAttachmentValidateAndContentTypeNormalization(t *testing.T) {
	valid := NewAttachment("ws-123", "report.pdf", "application/pdf; charset=utf-8", 1024, AttachmentSourceUpload)
	require.NoError(t, valid.Validate())
	assert.True(t, isAllowedContentType("application/pdf; charset=utf-8"))
	assert.False(t, isAllowedContentType("application/x-msdownload"))

	tests := []struct {
		name        string
		attachment  *Attachment
		expectedErr string
	}{
		{
			name:        "missing workspace",
			attachment:  &Attachment{Filename: "a.pdf", ContentType: "application/pdf", Size: 1},
			expectedErr: "workspace_id is required",
		},
		{
			name:        "missing filename",
			attachment:  &Attachment{WorkspaceID: "ws", ContentType: "application/pdf", Size: 1},
			expectedErr: "filename is required",
		},
		{
			name:        "invalid size",
			attachment:  &Attachment{WorkspaceID: "ws", Filename: "a.pdf", ContentType: "application/pdf", Size: 0},
			expectedErr: "size must be positive",
		},
		{
			name:        "too large",
			attachment:  &Attachment{WorkspaceID: "ws", Filename: "a.pdf", ContentType: "application/pdf", Size: MaxAttachmentSize + 1},
			expectedErr: "attachment size 26214401 exceeds maximum 26214400 bytes",
		},
		{
			name:        "unsupported type",
			attachment:  &Attachment{WorkspaceID: "ws", Filename: "a.bin", ContentType: "application/x-msdownload", Size: 10},
			expectedErr: "content type application/x-msdownload is not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.EqualError(t, tt.attachment.Validate(), tt.expectedErr)
		})
	}
}
