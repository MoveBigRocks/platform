package serviceapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/movebigrocks/platform/internal/infrastructure/antivirus"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// AttachmentService handles attachment uploads with virus scanning and S3 storage
type AttachmentService struct {
	s3Client *s3.Client
	scanner  antivirus.Scanner
	bucket   string
	logger   *logger.Logger
}

// AttachmentServiceConfig holds configuration for AttachmentService
type AttachmentServiceConfig struct {
	// S3 configuration
	S3Endpoint  string // Empty for AWS, set for MinIO/DigitalOcean Spaces
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string

	// ClamAV configuration
	ClamAVAddr    string // "host:port", empty to disable scanning
	ClamAVTimeout time.Duration

	// Logger
	Logger *logger.Logger
}

// NewAttachmentService creates a new attachment service
func NewAttachmentService(cfg AttachmentServiceConfig) (*AttachmentService, error) {
	// Build S3 client options
	var awsOpts []func(*config.LoadOptions) error

	awsOpts = append(awsOpts, config.WithRegion(cfg.S3Region))

	// Use static credentials if provided
	if cfg.S3AccessKey != "" && cfg.S3SecretKey != "" {
		awsOpts = append(awsOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), awsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Build S3 client with optional custom endpoint
	var s3Opts []func(*s3.Options)
	if cfg.S3Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = true // Required for MinIO
		})
	}

	s3Client := s3.NewFromConfig(awsCfg, s3Opts...)

	// Create scanner (ClamAV or mock)
	var scanner antivirus.Scanner
	if cfg.ClamAVAddr != "" {
		scanner = antivirus.NewClamAVScanner(antivirus.ClamAVConfig{
			Addr:    cfg.ClamAVAddr,
			Timeout: cfg.ClamAVTimeout,
		})
	} else {
		// Use mock scanner that always returns clean (for development)
		scanner = antivirus.NewMockScanner(true)
		if cfg.Logger != nil {
			cfg.Logger.Warn("ClamAV not configured, using mock scanner (all files pass)")
		}
	}

	return &AttachmentService{
		s3Client: s3Client,
		scanner:  scanner,
		bucket:   cfg.S3Bucket,
		logger:   cfg.Logger,
	}, nil
}

// Upload uploads a file with virus scanning
func (s *AttachmentService) Upload(ctx context.Context, attachment *servicedomain.Attachment, reader io.Reader) error {
	if attachment == nil {
		return fmt.Errorf("attachment is required")
	}

	// Validate attachment
	if err := attachment.Validate(); err != nil {
		attachment.MarkError(err.Error())
		return fmt.Errorf("invalid attachment: %w", err)
	}

	// Check for blocked extensions
	if servicedomain.IsBlockedExtension(attachment.Filename) {
		attachment.MarkError("file extension is blocked for security reasons")
		return fmt.Errorf("file extension is blocked for security reasons")
	}

	// Buffer the file for scanning and upload
	var buf bytes.Buffer
	hasher := sha256.New()
	tee := io.TeeReader(reader, hasher)

	if _, err := io.Copy(&buf, tee); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Calculate hash
	attachment.SHA256Hash = hex.EncodeToString(hasher.Sum(nil))

	// Scan for viruses
	attachment.MarkScanning()
	scanResult, err := s.scanner.Scan(ctx, bytes.NewReader(buf.Bytes()))
	if err != nil {
		attachment.MarkError(err.Error())
		s.logError("Virus scan failed", err, attachment)
		return fmt.Errorf("virus scan failed: %w", err)
	}

	if !scanResult.Clean {
		attachment.MarkInfected(scanResult.Message)
		s.logWarn("Malware detected in attachment", attachment, scanResult)
		return fmt.Errorf("%w: %s", antivirus.ErrMalwareDetected, scanResult.VirusName)
	}

	attachment.MarkClean(scanResult.Message)

	// Generate S3 key and upload
	s3Key := attachment.GenerateS3Key()
	attachment.SetS3Location(s.bucket, s3Key)

	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String(attachment.ContentType),
		Metadata: map[string]string{
			"attachment-id": attachment.ID,
			"workspace-id":  attachment.WorkspaceID,
			"sha256":        attachment.SHA256Hash,
			"source":        string(attachment.Source),
		},
	})
	if err != nil {
		attachment.S3Bucket = ""
		attachment.S3Key = ""
		attachment.MarkError(err.Error())
		s.logError("Failed to upload to S3", err, attachment)
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	s.logInfo("Attachment uploaded successfully", attachment)
	return nil
}

// UploadFromBase64 uploads an attachment from base64-encoded data (for email attachments)
func (s *AttachmentService) UploadFromBase64(ctx context.Context, attachment *servicedomain.Attachment, data []byte) error {
	return s.Upload(ctx, attachment, bytes.NewReader(data))
}

// Logging helpers

func (s *AttachmentService) logInfo(msg string, attachment *servicedomain.Attachment) {
	if s.logger == nil {
		return
	}
	s.logger.WithFields(map[string]interface{}{
		"attachment_id": attachment.ID,
		"workspace_id":  attachment.WorkspaceID,
		"filename":      attachment.Filename,
		"size":          attachment.Size,
	}).Info(msg)
}

func (s *AttachmentService) logWarn(msg string, attachment *servicedomain.Attachment, scanResult *antivirus.ScanResult) {
	if s.logger == nil {
		return
	}
	s.logger.WithFields(map[string]interface{}{
		"attachment_id": attachment.ID,
		"workspace_id":  attachment.WorkspaceID,
		"filename":      attachment.Filename,
		"virus_name":    scanResult.VirusName,
		"scan_result":   scanResult.Message,
	}).Warn(msg)
}

func (s *AttachmentService) logError(msg string, err error, attachment *servicedomain.Attachment) {
	if s.logger == nil {
		return
	}
	s.logger.WithError(err).WithFields(map[string]interface{}{
		"attachment_id": attachment.ID,
		"workspace_id":  attachment.WorkspaceID,
		"filename":      attachment.Filename,
	}).Error(msg)
}
