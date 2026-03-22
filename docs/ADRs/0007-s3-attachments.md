# ADR 0007: S3 Attachments with Virus Scanning

**Status:** Accepted

## Context

Move Big Rocks stores file attachments from:
- Email attachments (inbound emails)
- Case attachments (uploaded by users)
- Form submissions (file upload fields)

**Requirements:**
- Secure storage with presigned URLs
- Virus scanning before storage
- Workspace isolation
- Scalable storage

## Decision

**S3/DigitalOcean Spaces for attachment storage with ClamAV virus scanning.**

### Storage Architecture

```
mbr-attachments bucket:
└── {workspace_id}/
    └── attachments/
        └── {year}/{month}/{attachment_id}/{filename}
```

Database stores metadata only:
```go
type Attachment struct {
    ID            string
    CaseID        string
    Filename      string
    Size          int64
    ContentType   string
    StorageBucket string
    StorageKey    string
    Status        string // pending, scanning, clean, infected
}
```

### Upload Flow

```
1. File received (email webhook or API)
2. Upload to S3 quarantine location
3. Scan with ClamAV (INSTREAM protocol)
4. If clean: Move to final location, save metadata
5. If infected: Delete from quarantine, reject upload
```

### AttachmentService

```go
func (s *AttachmentService) Upload(ctx context.Context, att *Attachment, data io.Reader) error {
    // 1. Upload to quarantine
    quarantineKey := fmt.Sprintf("quarantine/%s/%s", att.ID, att.Filename)
    s.s3Client.PutObject(ctx, s.bucket, quarantineKey, data)

    // 2. Scan for viruses
    scanResult := s.scanner.ScanFile(ctx, s.bucket, quarantineKey)
    if scanResult.Infected {
        s.s3Client.DeleteObject(ctx, s.bucket, quarantineKey)
        return ErrVirusDetected
    }

    // 3. Move to final location
    finalKey := att.GenerateS3Key()
    s.s3Client.CopyObject(ctx, s.bucket, quarantineKey, finalKey)
    s.s3Client.DeleteObject(ctx, s.bucket, quarantineKey)

    return nil
}
```

### ClamAV Integration

- TCP connection on port 3310
- INSTREAM protocol for streaming
- Auto-updating virus definitions

### Presigned URLs

Files served only via presigned URLs:

```go
func (s *AttachmentService) GetPresignedURL(ctx context.Context, att *Attachment) (string, error) {
    return s.s3Client.PresignGetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(att.StorageBucket),
        Key:    aws.String(att.StorageKey),
    }, time.Hour)
}
```

## Configuration

```bash
S3_ATTACHMENTS_BUCKET=mbr-attachments
S3_ENDPOINT=https://ams3.digitaloceanspaces.com
S3_REGION=ams3
AWS_ACCESS_KEY_ID=xxx
AWS_SECRET_ACCESS_KEY=xxx
CLAMAV_ADDR=localhost:3310
```

## Consequences

**Positive:**
- Scalable storage (S3 is effectively unlimited)
- Virus protection via ClamAV
- Workspace isolation via path prefixes
- Secure downloads via presigned URLs

**Negative:**
- Requires S3 credentials in production
- ClamAV requires ~1GB RAM
- Slight latency for virus scanning

## References

- AttachmentService: `internal/service/services/attachment_service.go`
- ClamAV scanner: `internal/infrastructure/antivirus/clamav.go`
- Attachment domain: `internal/service/domain/attachment.go`
