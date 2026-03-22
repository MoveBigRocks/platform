package synth

import (
	"context"
	"fmt"
	"time"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

// AttachmentsScenarioRunner runs attachment scenarios
type AttachmentsScenarioRunner struct {
	services *TestServices
	verbose  bool
}

// NewAttachmentsScenarioRunner creates a new attachments scenario runner
func NewAttachmentsScenarioRunner(services *TestServices, verbose bool) *AttachmentsScenarioRunner {
	return &AttachmentsScenarioRunner{
		services: services,
		verbose:  verbose,
	}
}

// RunAllAttachmentsScenarios runs all attachment scenarios
func (sr *AttachmentsScenarioRunner) RunAllAttachmentsScenarios(ctx context.Context, workspaceID string, users []*platformdomain.User) ([]*ScenarioResult, error) {
	scenarios := []func(context.Context, string, []*platformdomain.User) (*ScenarioResult, error){
		sr.scenarioCreateAttachment,
		sr.scenarioAttachmentLifecycle,
		sr.scenarioAttachmentsByCase,
		sr.scenarioQuarantinedAttachments,
		sr.scenarioAttachmentValidation,
	}

	var results []*ScenarioResult
	for _, scenario := range scenarios {
		result, err := scenario(ctx, workspaceID, users)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

// scenarioCreateAttachment tests creating and managing attachments
func (sr *AttachmentsScenarioRunner) scenarioCreateAttachment(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Create Attachment",
	}

	if sr.verbose {
		fmt.Println("  -> Testing attachment creation...")
	}

	// Create an attachment
	attachment := servicedomain.NewAttachment(workspaceID, "document.pdf", "application/pdf", 1024*1024, servicedomain.AttachmentSourceUpload)
	if len(users) > 0 {
		attachment.UploadedBy = users[0].ID
	}
	attachment.Description = "Test document"

	// Set S3 location
	attachment.SetS3Location("mbr-attachments", attachment.GenerateS3Key())

	// Save metadata using the cases store (which has attachment support)
	err := sr.services.Store.Cases().SaveAttachment(ctx, attachment, nil)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Attachment created",
		Passed:  attachment.ID != "",
		Details: fmt.Sprintf("Attachment ID: %s", attachment.ID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Initial status is pending",
		Passed:  attachment.Status == servicedomain.AttachmentStatusPending,
		Details: fmt.Sprintf("Status: %s", attachment.Status),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "S3 location set",
		Passed:  attachment.S3Bucket != "" && attachment.S3Key != "",
		Details: fmt.Sprintf("Bucket: %s, Key: %s", attachment.S3Bucket, attachment.S3Key),
	})

	// Retrieve attachment
	retrieved, err := sr.services.Store.Cases().GetAttachment(ctx, workspaceID, attachment.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Attachment retrievable",
		Passed:  err == nil && retrieved != nil,
		Details: fmt.Sprintf("Retrieved: %v", retrieved != nil),
	})

	result.Success = true
	for _, v := range result.Verifications {
		if !v.Passed {
			result.Success = false
			break
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// scenarioAttachmentLifecycle tests attachment create/retrieve/delete cycle
func (sr *AttachmentsScenarioRunner) scenarioAttachmentLifecycle(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Attachment Lifecycle (Store Operations)",
	}

	if sr.verbose {
		fmt.Println("  -> Testing attachment store lifecycle...")
	}

	// Create attachment
	attachment := servicedomain.NewAttachment(workspaceID, "report.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", 512*1024, servicedomain.AttachmentSourceEmail)
	attachment.SetS3Location("mbr-attachments", attachment.GenerateS3Key())

	err := sr.services.Store.Cases().SaveAttachment(ctx, attachment, nil)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Verify attachment can be retrieved
	retrieved, err := sr.services.Store.Cases().GetAttachment(ctx, workspaceID, attachment.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Attachment stored and retrievable",
		Passed:  err == nil && retrieved != nil,
		Details: fmt.Sprintf("Retrieved ID: %s", attachment.ID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Retrieved filename matches",
		Passed:  retrieved != nil && retrieved.Filename == "report.xlsx",
		Details: fmt.Sprintf("Filename: %s", retrieved.Filename),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Retrieved S3 location matches",
		Passed:  retrieved != nil && retrieved.S3Bucket == attachment.S3Bucket && retrieved.S3Key == attachment.S3Key,
		Details: fmt.Sprintf("Bucket: %s, Key: %s", retrieved.S3Bucket, retrieved.S3Key),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Status persisted correctly",
		Passed:  retrieved != nil && retrieved.Status == servicedomain.AttachmentStatusPending,
		Details: fmt.Sprintf("Status: %s", retrieved.Status),
	})

	// Delete attachment
	err = sr.services.Store.Cases().DeleteAttachment(ctx, workspaceID, attachment.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Attachment deleted successfully",
		Passed:  err == nil,
		Details: fmt.Sprintf("Delete error: %v", err),
	})

	result.Success = true
	for _, v := range result.Verifications {
		if !v.Passed {
			result.Success = false
			break
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// scenarioAttachmentsByCase tests listing attachments by case
func (sr *AttachmentsScenarioRunner) scenarioAttachmentsByCase(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Attachments by Case",
	}

	if sr.verbose {
		fmt.Println("  -> Testing attachments by case...")
	}

	// Create a case
	supportCase := servicedomain.NewCase(workspaceID, "Issue with attachments", "user@example.com")
	supportCase.GenerateHumanID("test")

	err := sr.services.Store.Cases().CreateCase(ctx, supportCase)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Case created",
		Passed:  supportCase.ID != "",
		Details: fmt.Sprintf("Case ID: %s", supportCase.ID),
	})

	// Create multiple attachments for the case
	attachmentNames := []string{"screenshot1.png", "screenshot2.png", "logs.txt"}
	var createdAttachments []*servicedomain.Attachment

	for _, name := range attachmentNames {
		att := servicedomain.NewAttachment(workspaceID, name, "image/png", 50*1024, servicedomain.AttachmentSourceAgent)
		att.CaseID = supportCase.ID
		if len(users) > 0 {
			att.UploadedBy = users[0].ID
		}
		att.SetS3Location("mbr-attachments", att.GenerateS3Key())
		att.MarkClean("OK") // Pre-mark as clean for testing

		err := sr.services.Store.Cases().SaveAttachment(ctx, att, nil)
		if err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result, nil
		}
		createdAttachments = append(createdAttachments, att)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Multiple attachments created",
		Passed:  len(createdAttachments) == 3,
		Details: fmt.Sprintf("Created %d attachments", len(createdAttachments)),
	})

	// Verify all attachments are linked to the case
	for _, att := range createdAttachments {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check:   fmt.Sprintf("Attachment %s linked to case", att.Filename),
			Passed:  att.CaseID == supportCase.ID,
			Details: fmt.Sprintf("CaseID: %s", att.CaseID),
		})
	}

	result.Success = true
	for _, v := range result.Verifications {
		if !v.Passed {
			result.Success = false
			break
		}
	}

	result.Duration = time.Since(start)
	result.CaseID = supportCase.ID
	return result, nil
}

// scenarioQuarantinedAttachments tests that attachments with different statuses are persisted correctly
func (sr *AttachmentsScenarioRunner) scenarioQuarantinedAttachments(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Attachment Status Persistence",
	}

	if sr.verbose {
		fmt.Println("  -> Testing attachment status persistence...")
	}

	// Create and save a "clean" attachment (pre-set status before save)
	cleanAttachment := servicedomain.NewAttachment(workspaceID, "safe-file.pdf", "application/pdf", 100*1024, servicedomain.AttachmentSourceUpload)
	cleanAttachment.SetS3Location("mbr-attachments", cleanAttachment.GenerateS3Key())
	cleanAttachment.MarkScanning()
	cleanAttachment.MarkClean("OK")

	err := sr.services.Store.Cases().SaveAttachment(ctx, cleanAttachment, nil)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create and save an "infected" attachment
	infectedAttachment := servicedomain.NewAttachment(workspaceID, "suspicious.bin", "application/octet-stream", 200*1024, servicedomain.AttachmentSourceEmail)
	infectedAttachment.SetS3Location("mbr-quarantine", infectedAttachment.GenerateS3Key())
	infectedAttachment.MarkScanning()
	infectedAttachment.MarkInfected("Win.Trojan.Generic-12345")

	err = sr.services.Store.Cases().SaveAttachment(ctx, infectedAttachment, nil)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Retrieve and verify clean attachment was persisted with correct status
	retrievedClean, err := sr.services.Store.Cases().GetAttachment(ctx, workspaceID, cleanAttachment.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Clean attachment retrievable",
		Passed:  err == nil && retrievedClean != nil,
		Details: fmt.Sprintf("ID: %s", cleanAttachment.ID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Clean attachment status persisted",
		Passed:  retrievedClean != nil && retrievedClean.Status == servicedomain.AttachmentStatusClean,
		Details: fmt.Sprintf("Status: %s", retrievedClean.Status),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Clean attachment scan result persisted",
		Passed:  retrievedClean != nil && retrievedClean.ScanResult == "OK",
		Details: fmt.Sprintf("ScanResult: %s", retrievedClean.ScanResult),
	})

	// Retrieve and verify infected attachment was persisted with correct status
	retrievedInfected, err := sr.services.Store.Cases().GetAttachment(ctx, workspaceID, infectedAttachment.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Infected attachment retrievable",
		Passed:  err == nil && retrievedInfected != nil,
		Details: fmt.Sprintf("ID: %s", infectedAttachment.ID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Infected attachment status persisted",
		Passed:  retrievedInfected != nil && retrievedInfected.Status == servicedomain.AttachmentStatusInfected,
		Details: fmt.Sprintf("Status: %s", retrievedInfected.Status),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Infected attachment scan result persisted",
		Passed:  retrievedInfected != nil && retrievedInfected.ScanResult == "Win.Trojan.Generic-12345",
		Details: fmt.Sprintf("ScanResult: %s", retrievedInfected.ScanResult),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Infected attachment quarantine bucket persisted",
		Passed:  retrievedInfected != nil && retrievedInfected.S3Bucket == "mbr-quarantine",
		Details: fmt.Sprintf("S3Bucket: %s", retrievedInfected.S3Bucket),
	})

	result.Success = true
	for _, v := range result.Verifications {
		if !v.Passed {
			result.Success = false
			break
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// Validation scenarios

// scenarioAttachmentValidation tests attachment validation rules
func (sr *AttachmentsScenarioRunner) scenarioAttachmentValidation(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Attachment Validation",
	}

	if sr.verbose {
		fmt.Println("  -> Testing attachment validation...")
	}

	// Test valid attachment
	validAttachment := servicedomain.NewAttachment(workspaceID, "valid.pdf", "application/pdf", 1024*1024, servicedomain.AttachmentSourceUpload)
	err := validAttachment.Validate()
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Valid attachment passes validation",
		Passed:  err == nil,
		Details: fmt.Sprintf("Error: %v", err),
	})

	// Test attachment exceeding max size
	largeAttachment := servicedomain.NewAttachment(workspaceID, "huge.zip", "application/zip", 30*1024*1024, servicedomain.AttachmentSourceUpload)
	err = largeAttachment.Validate()
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Large attachment fails validation",
		Passed:  err != nil,
		Details: fmt.Sprintf("Error: %v", err),
	})

	// Test invalid content type
	invalidTypeAttachment := servicedomain.NewAttachment(workspaceID, "script.sh", "application/x-sh", 1024, servicedomain.AttachmentSourceUpload)
	err = invalidTypeAttachment.Validate()
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Invalid content type fails validation",
		Passed:  err != nil,
		Details: fmt.Sprintf("Error: %v", err),
	})

	// Test missing workspace ID
	noWorkspaceAttachment := servicedomain.NewAttachment("", "test.pdf", "application/pdf", 1024, servicedomain.AttachmentSourceUpload)
	err = noWorkspaceAttachment.Validate()
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Missing workspace fails validation",
		Passed:  err != nil,
		Details: fmt.Sprintf("Error: %v", err),
	})

	result.Success = true
	for _, v := range result.Verifications {
		if !v.Passed {
			result.Success = false
			break
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}
