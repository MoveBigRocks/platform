package synth

import (
	"context"
	"fmt"
	"time"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// FormsScenarioRunner runs form system scenarios
type FormsScenarioRunner struct {
	services *TestServices
	verbose  bool
}

// NewFormsScenarioRunner creates a new forms scenario runner
func NewFormsScenarioRunner(services *TestServices, verbose bool) *FormsScenarioRunner {
	return &FormsScenarioRunner{
		services: services,
		verbose:  verbose,
	}
}

// RunAllFormsScenarios runs all public-form scenarios.
func (sr *FormsScenarioRunner) RunAllFormsScenarios(ctx context.Context, workspaceID string, users []*platformdomain.User) ([]*ScenarioResult, error) {
	scenarios := []func(context.Context, string, []*platformdomain.User) (*ScenarioResult, error){
		sr.scenarioCreateFormDefinition,
		sr.scenarioFormSubmission,
		sr.scenarioFormSubmissionProcessing,
		sr.scenarioFormToCase,
		sr.scenarioFormAnalytics,
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

// scenarioCreateFormDefinition tests creating and managing public form definitions.
func (sr *FormsScenarioRunner) scenarioCreateFormDefinition(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Create Form Definition",
	}

	if sr.verbose {
		fmt.Println("  -> Creating form definition...")
	}

	// Create a contact form definition.
	userID := ""
	if len(users) > 0 {
		userID = users[0].ID
	}

	form := servicedomain.NewFormSchema(workspaceID, "Contact Us", "contact-us", userID)
	form.Description = "Customer contact form"
	form.IsPublic = true
	form.RequiresAuth = false
	form.AutoCreateCase = true
	form.AutoCasePriority = "medium"
	form.Status = servicedomain.FormStatusDraft
	form.SchemaData = shareddomain.TypedSchemaFromMap(map[string]interface{}{
		"fields": []map[string]interface{}{
			{"name": "name", "type": "text", "required": true, "label": "Your Name"},
			{"name": "email", "type": "email", "required": true, "label": "Email Address"},
			{"name": "subject", "type": "text", "required": true, "label": "Subject"},
			{"name": "message", "type": "textarea", "required": true, "label": "Message"},
		},
	})

	err := sr.services.Store.Forms().CreateFormSchema(ctx, form)
	if err != nil {
		return failScenario(result, start, err)
	}

	// Verify form was created
	retrieved, err := sr.services.Store.Forms().GetFormSchema(ctx, form.ID)
	if err != nil {
		result.Error = fmt.Errorf("failed to retrieve form: %w", err)
		result.Duration = time.Since(start)
		return result, nil
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Form created successfully",
		Passed:  retrieved != nil,
		Details: fmt.Sprintf("Form ID: %s", form.ID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Form name matches",
		Passed:  retrieved.Name == "Contact Us",
		Details: fmt.Sprintf("Expected 'Contact Us', got '%s'", retrieved.Name),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Form slug matches",
		Passed:  retrieved.Slug == "contact-us",
		Details: fmt.Sprintf("Expected 'contact-us', got '%s'", retrieved.Slug),
	})

	// Activate the form
	form.Status = servicedomain.FormStatusActive
	err = sr.services.Store.Forms().UpdateFormSchema(ctx, form)
	if err != nil {
		result.Error = fmt.Errorf("failed to activate form: %w", err)
		result.Duration = time.Since(start)
		return result, nil
	}

	// Retrieve by slug
	bySlug, err := sr.services.Store.Forms().GetFormSchemaBySlug(ctx, workspaceID, "contact-us")
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Form retrievable by slug",
		Passed:  err == nil && bySlug != nil && bySlug.ID == form.ID,
		Details: fmt.Sprintf("Slug lookup: %v", err == nil),
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

// scenarioFormSubmission tests form submission creation.
func (sr *FormsScenarioRunner) scenarioFormSubmission(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Form Submission",
	}

	if sr.verbose {
		fmt.Println("  -> Testing form submission...")
	}

	// First create a form definition.
	userID := ""
	if len(users) > 0 {
		userID = users[0].ID
	}

	form := servicedomain.NewFormSchema(workspaceID, "Feedback Form", "feedback", userID)
	form.Status = servicedomain.FormStatusActive
	form.IsPublic = true

	err := sr.services.Store.Forms().CreateFormSchema(ctx, form)
	if err != nil {
		result.Error = fmt.Errorf("failed to create form: %w", err)
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create a submission
	submissionData := map[string]interface{}{
		"name":    "John Doe",
		"email":   "john@example.com",
		"rating":  5,
		"comment": "Great product!",
	}

	submission := servicedomain.NewPublicFormSubmission(workspaceID, form.ID, shareddomain.MetadataFromMap(submissionData))
	submission.SubmitterEmail = "john@example.com"
	submission.SubmitterName = "John Doe"
	submission.SubmitterIP = "192.168.1.1"

	err = sr.services.Store.Forms().CreateFormSubmission(ctx, submission)
	if err != nil {
		result.Error = fmt.Errorf("failed to create submission: %w", err)
		result.Duration = time.Since(start)
		return result, nil
	}

	// Verify submission was created
	retrieved, err := sr.services.Store.Forms().GetFormSubmission(ctx, submission.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Submission created successfully",
		Passed:  err == nil && retrieved != nil,
		Details: fmt.Sprintf("Submission ID: %s", submission.ID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Submission data preserved",
		Passed:  retrieved.GetStringField("name") == "John Doe",
		Details: fmt.Sprintf("Name field: %s", retrieved.GetStringField("name")),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Submitter email captured",
		Passed:  retrieved.SubmitterEmail == "john@example.com",
		Details: fmt.Sprintf("Email: %s", retrieved.SubmitterEmail),
	})

	// List submissions for the form
	submissions, err := sr.services.Store.Forms().ListFormSubmissions(ctx, form.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Submission listed for form",
		Passed:  err == nil && len(submissions) >= 1,
		Details: fmt.Sprintf("Found %d submissions", len(submissions)),
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

// scenarioFormSubmissionProcessing tests submission status transitions.
func (sr *FormsScenarioRunner) scenarioFormSubmissionProcessing(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Form Submission Processing",
	}

	if sr.verbose {
		fmt.Println("  -> Testing submission processing lifecycle...")
	}

	// Create form definition.
	userID := ""
	if len(users) > 0 {
		userID = users[0].ID
	}

	form := servicedomain.NewFormSchema(workspaceID, "Support Request", "support-request", userID)
	form.Status = servicedomain.FormStatusActive
	err := sr.services.Store.Forms().CreateFormSchema(ctx, form)
	if err != nil {
		result.Error = fmt.Errorf("failed to create form: %w", err)
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create submission
	submission := servicedomain.NewPublicFormSubmission(workspaceID, form.ID, shareddomain.MetadataFromMap(map[string]interface{}{
		"issue": "Login problem",
		"email": "user@test.com",
	}))

	err = sr.services.Store.Forms().CreateFormSubmission(ctx, submission)
	if err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Initial status is pending",
		Passed:  submission.Status == servicedomain.SubmissionStatusPending,
		Details: fmt.Sprintf("Status: %s", submission.Status),
	})

	// Transition to processing
	submission.Status = servicedomain.SubmissionStatusProcessing
	submission.UpdatedAt = time.Now()
	err = sr.services.Store.Forms().UpdateFormSubmission(ctx, submission)
	if err != nil {
		return failScenario(result, start, err)
	}

	retrieved, _ := sr.services.Store.Forms().GetFormSubmission(ctx, submission.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Status updated to processing",
		Passed:  retrieved.Status == servicedomain.SubmissionStatusProcessing,
		Details: fmt.Sprintf("Status: %s", retrieved.Status),
	})

	// Complete processing
	now := time.Now()
	submission.Status = servicedomain.SubmissionStatusCompleted
	submission.ProcessedAt = &now
	submission.ProcessingTime = 150 // milliseconds
	submission.IsValid = true
	err = sr.services.Store.Forms().UpdateFormSubmission(ctx, submission)
	if err != nil {
		return failScenario(result, start, err)
	}

	retrieved, _ = sr.services.Store.Forms().GetFormSubmission(ctx, submission.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Status updated to completed",
		Passed:  retrieved.Status == servicedomain.SubmissionStatusCompleted,
		Details: fmt.Sprintf("Status: %s", retrieved.Status),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Processing time recorded",
		Passed:  retrieved.ProcessingTime == 150,
		Details: fmt.Sprintf("Processing time: %d ms", retrieved.ProcessingTime),
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

// scenarioFormToCase tests form submission creating a case.
func (sr *FormsScenarioRunner) scenarioFormToCase(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Form to Case Creation",
	}

	if sr.verbose {
		fmt.Println("  -> Testing form-to-case workflow...")
	}

	// Create form with auto-case settings
	userID := ""
	if len(users) > 0 {
		userID = users[0].ID
	}

	form := servicedomain.NewFormSchema(workspaceID, "Bug Report", "bug-report", userID)
	form.Status = servicedomain.FormStatusActive
	form.AutoCreateCase = true
	form.AutoCasePriority = "high"
	form.AutoTags = []string{"bug", "form-submission"}

	err := sr.services.Store.Forms().CreateFormSchema(ctx, form)
	if err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Form configured for auto-case creation",
		Passed:  form.AutoCreateCase,
		Details: fmt.Sprintf("AutoCreateCase: %v, Priority: %s", form.AutoCreateCase, form.AutoCasePriority),
	})

	// Create submission
	submission := servicedomain.NewPublicFormSubmission(workspaceID, form.ID, shareddomain.MetadataFromMap(map[string]interface{}{
		"title":       "App crashes on startup",
		"description": "The application crashes immediately after launch",
		"email":       "reporter@example.com",
	}))
	submission.SubmitterEmail = "reporter@example.com"

	err = sr.services.Store.Forms().CreateFormSubmission(ctx, submission)
	if err != nil {
		return failScenario(result, start, err)
	}

	// Simulate case creation (in real system this would be done by a handler)
	newCase := servicedomain.NewCase(workspaceID, "Bug Report: App crashes on startup", "reporter@example.com")
	newCase.GenerateHumanID("test")
	newCase.Priority = servicedomain.CasePriorityHigh
	newCase.Tags = []string{"bug", "form-submission"}
	newCase.Channel = "form"
	newCase.Description = "The application crashes immediately after launch"

	err = sr.services.Store.Cases().CreateCase(ctx, newCase)
	if err != nil {
		return failScenario(result, start, err)
	}

	// Update submission with case ID
	submission.CaseID = newCase.ID
	submission.Status = servicedomain.SubmissionStatusCompleted
	now := time.Now()
	submission.ProcessedAt = &now
	err = sr.services.Store.Forms().UpdateFormSubmission(ctx, submission)
	if err != nil {
		return failScenario(result, start, err)
	}

	// Verify the linkage
	retrieved, _ := sr.services.Store.Forms().GetFormSubmission(ctx, submission.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Submission linked to case",
		Passed:  retrieved.CaseID == newCase.ID,
		Details: fmt.Sprintf("Case ID: %s", retrieved.CaseID),
	})

	// Verify case was created with correct data
	retrievedCase, err := sr.services.Store.Cases().GetCase(ctx, newCase.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Case created from form submission",
		Passed:  err == nil && retrievedCase != nil,
		Details: fmt.Sprintf("Case: %s", retrievedCase.Subject),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Case has correct priority",
		Passed:  retrievedCase.Priority == servicedomain.CasePriorityHigh,
		Details: fmt.Sprintf("Priority: %s", retrievedCase.Priority),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Case has form tags",
		Passed:  len(retrievedCase.Tags) >= 2,
		Details: fmt.Sprintf("Tags: %v", retrievedCase.Tags),
	})

	result.Success = true
	for _, v := range result.Verifications {
		if !v.Passed {
			result.Success = false
			break
		}
	}

	result.Duration = time.Since(start)
	result.CaseID = newCase.ID
	return result, nil
}

// scenarioFormAnalytics tests form analytics retrieval.
func (sr *FormsScenarioRunner) scenarioFormAnalytics(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Form Analytics",
	}

	if sr.verbose {
		fmt.Println("  -> Testing form analytics...")
	}

	// Create form
	userID := ""
	if len(users) > 0 {
		userID = users[0].ID
	}

	form := servicedomain.NewFormSchema(workspaceID, "Survey", "survey-"+id.New()[:8], userID)
	form.Status = servicedomain.FormStatusActive
	err := sr.services.Store.Forms().CreateFormSchema(ctx, form)
	if err != nil {
		return failScenario(result, start, err)
	}

	// Create multiple submissions
	for i := 0; i < 5; i++ {
		submission := servicedomain.NewPublicFormSubmission(workspaceID, form.ID, shareddomain.MetadataFromMap(map[string]interface{}{
			"response": fmt.Sprintf("Response %d", i+1),
		}))
		submission.SubmitterEmail = fmt.Sprintf("user%d@example.com", i+1)
		submission.Status = servicedomain.SubmissionStatusCompleted
		now := time.Now()
		submission.ProcessedAt = &now
		submission.IsValid = true

		err := sr.services.Store.Forms().CreateFormSubmission(ctx, submission)
		if err != nil {
			return failScenario(result, start, err)
		}
	}

	// Get analytics
	analytics, err := sr.services.Store.Forms().GetFormAnalytics(ctx, form.ID)
	if err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Analytics retrieved",
		Passed:  analytics != nil,
		Details: "Analytics object returned",
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Analytics has correct form ID",
		Passed:  analytics.FormID == form.ID,
		Details: fmt.Sprintf("Form ID: %s", analytics.FormID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Analytics tracks submissions",
		Passed:  analytics.TotalSubmissions >= 5,
		Details: fmt.Sprintf("Total submissions: %d", analytics.TotalSubmissions),
	})

	// List all forms in workspace
	forms, err := sr.services.Store.Forms().ListWorkspaceFormSchemas(ctx, workspaceID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Forms listed for workspace",
		Passed:  err == nil && len(forms) >= 1,
		Details: fmt.Sprintf("Found %d forms", len(forms)),
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
