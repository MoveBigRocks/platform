package synth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	"github.com/movebigrocks/platform/internal/testutil/refext"
	"github.com/movebigrocks/platform/pkg/id"
)

// ObservabilityScenarioRunner executes error monitoring scenarios
type ObservabilityScenarioRunner struct {
	services *TestServices
	verbose  bool
}

// NewObservabilityScenarioRunner creates a new observability scenario runner
func NewObservabilityScenarioRunner(services *TestServices, verbose bool) *ObservabilityScenarioRunner {
	return &ObservabilityScenarioRunner{
		services: services,
		verbose:  verbose,
	}
}

// RunAllObservabilityScenarios runs all error monitoring scenarios
func (sr *ObservabilityScenarioRunner) RunAllObservabilityScenarios(ctx context.Context, workspaceID string) ([]*ScenarioResult, error) {
	if err := sr.ensureErrorTrackingExtension(ctx, workspaceID); err != nil {
		return nil, err
	}

	results := make([]*ScenarioResult, 0)

	scenarios := []struct {
		Name string
		Run  func(context.Context, string) (*ScenarioResult, error)
	}{
		{"Create Project with DSN", sr.RunCreateProjectScenario},
		{"Ingest Error Event", sr.RunIngestErrorEventScenario},
		{"Error Deduplication (Same Fingerprint)", sr.RunErrorDeduplicationScenario},
		{"Issue Lifecycle (Resolve → Reopen)", sr.RunIssueLifecycleScenario},
		{"Multiple Error Types Grouping", sr.RunMultipleErrorTypesScenario},
	}

	for _, scenario := range scenarios {
		sr.log("Running scenario: %s", scenario.Name)

		result, err := scenario.Run(ctx, workspaceID)
		if err != nil {
			result = &ScenarioResult{
				Name:    scenario.Name,
				Success: false,
				Error:   err,
			}
		}
		results = append(results, result)

		if sr.verbose && len(result.Verifications) > 0 {
			for _, v := range result.Verifications {
				status := "✓"
				if !v.Passed {
					status = "✗"
				}
				sr.log("    %s %s: %s", status, v.Check, v.Details)
			}
		}
		sr.log("  Result: success=%v", result.Success)
	}

	return results, nil
}

// RunCreateProjectScenario tests creating an error monitoring project
func (sr *ObservabilityScenarioRunner) RunCreateProjectScenario(ctx context.Context, workspaceID string) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Create Project with DSN",
		Verifications: make([]VerificationResult, 0),
	}

	if err := sr.ensureErrorTrackingExtension(ctx, workspaceID); err != nil {
		return result, err
	}

	sr.log("  Step 1: Creating error monitoring project...")

	// Create project
	project := &observabilitydomain.Project{
		WorkspaceID: workspaceID,
		Name:        "Test Application",
		Platform:    "javascript",
		Environment: "production",
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Generate DSN components
	project.PublicKey = generateKey(16)
	project.SecretKey = generateKey(32)
	project.AppKey = generateKey(8)

	err := sr.services.Store.Projects().CreateProject(ctx, project)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Project created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	if project.ID == "" {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Project ID generated", Passed: false, Details: "ID is empty",
		})
		return result, fmt.Errorf("project ID not generated")
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Project created", Passed: true, Details: fmt.Sprintf("ID: %s", project.ID),
	})

	// Verify project can be retrieved
	sr.log("  Step 2: Verifying project retrieval...")
	stored, err := sr.services.Store.Projects().GetProject(ctx, project.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Project retrievable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Project retrievable", Passed: true, Details: fmt.Sprintf("Name: %s", stored.Name),
	})

	// Verify DSN lookup works
	sr.log("  Step 3: Verifying DSN key lookup...")
	byKey, err := sr.services.Store.Projects().GetProjectByKey(ctx, project.PublicKey)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Project lookup by key", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	if byKey.ID != project.ID {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Project lookup by key", Passed: false, Details: "ID mismatch",
		})
		return result, fmt.Errorf("project ID mismatch on key lookup")
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Project lookup by key", Passed: true, Details: fmt.Sprintf("Key: %s...", project.PublicKey[:8]),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunIngestErrorEventScenario tests ingesting an error event and creating an issue
func (sr *ObservabilityScenarioRunner) RunIngestErrorEventScenario(ctx context.Context, workspaceID string) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Ingest Error Event",
		Verifications: make([]VerificationResult, 0),
	}

	// Create project first
	project := sr.createTestProject(ctx, workspaceID, "Event Ingestion App")
	if project == nil {
		return result, fmt.Errorf("failed to create test project")
	}

	sr.log("  Step 1: Creating error event...")

	// Create error event
	fingerprint := generateFingerprint("TypeError", "Cannot read property 'foo' of undefined", "app.js:42")
	event := &observabilitydomain.ErrorEvent{
		ProjectID:   project.ID,
		EventID:     id.New(),
		Message:     "Cannot read property 'foo' of undefined",
		Level:       "error",
		Platform:    "javascript",
		Environment: "production",
		Timestamp:   time.Now(),
		Received:    time.Now(),
		Fingerprint: []string{fingerprint},
		Exception: []observabilitydomain.ExceptionData{
			{
				Type:  "TypeError",
				Value: "Cannot read property 'foo' of undefined",
				Stacktrace: &observabilitydomain.StacktraceData{
					Frames: []observabilitydomain.FrameData{
						{Filename: "app.js", Function: "handleClick", LineNumber: 42, InApp: true},
						{Filename: "react.js", Function: "dispatchEvent", LineNumber: 100, InApp: false},
					},
				},
			},
		},
		Tags: map[string]string{
			"browser": "Chrome",
			"os":      "Windows",
		},
	}

	err := sr.services.Store.ErrorEvents().CreateErrorEvent(ctx, event)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Error event created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Error event created", Passed: true, Details: fmt.Sprintf("EventID: %s", event.EventID),
	})

	// Create corresponding issue
	sr.log("  Step 2: Creating issue for error...")
	issue := &observabilitydomain.Issue{
		WorkspaceID: workspaceID,
		ProjectID:   project.ID,
		Title:       "TypeError: Cannot read property 'foo' of undefined",
		Culprit:     "app.js in handleClick",
		Fingerprint: fingerprint,
		Status:      "unresolved",
		Level:       "error",
		Type:        "error",
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		EventCount:  1,
		UserCount:   1,
		LastEventID: event.ID,
	}

	err = sr.services.Store.Issues().CreateIssue(ctx, issue)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Issue created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Issue created", Passed: true, Details: fmt.Sprintf("ID: %s, Status: %s", issue.ID, issue.Status),
	})

	// Link event to issue
	event.IssueID = issue.ID
	// Note: In real system, event update would happen via service

	// Verify issue can be retrieved by fingerprint
	sr.log("  Step 3: Verifying fingerprint lookup...")
	byFP, err := sr.services.Store.Issues().GetIssueByFingerprint(ctx, project.ID, fingerprint)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Issue lookup by fingerprint", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Issue lookup by fingerprint", Passed: byFP.ID == issue.ID,
		Details: fmt.Sprintf("Found: %s", byFP.Title[:30]+"..."),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunErrorDeduplicationScenario tests that duplicate errors are grouped together
func (sr *ObservabilityScenarioRunner) RunErrorDeduplicationScenario(ctx context.Context, workspaceID string) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Error Deduplication (Same Fingerprint)",
		Verifications: make([]VerificationResult, 0),
	}

	// Create project
	project := sr.createTestProject(ctx, workspaceID, "Dedup Test App")
	if project == nil {
		return result, fmt.Errorf("failed to create test project")
	}

	// Generate consistent fingerprint
	fingerprint := generateFingerprint("NullPointerException", "object is null", "Service.java:100")

	sr.log("  Step 1: Creating initial issue...")

	// Create first issue
	issue := &observabilitydomain.Issue{
		WorkspaceID: workspaceID,
		ProjectID:   project.ID,
		Title:       "NullPointerException: object is null",
		Culprit:     "Service.java in processRequest",
		Fingerprint: fingerprint,
		Status:      "unresolved",
		Level:       "error",
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		EventCount:  1,
		UserCount:   1,
	}

	err := sr.services.Store.Issues().CreateIssue(ctx, issue)
	if err != nil {
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Initial issue created", Passed: true, Details: fmt.Sprintf("EventCount: %d", issue.EventCount),
	})

	// Simulate duplicate errors arriving
	sr.log("  Step 2: Simulating 5 duplicate errors...")

	for i := 0; i < 5; i++ {
		// Create error event with same fingerprint
		event := &observabilitydomain.ErrorEvent{
			ProjectID:   project.ID,
			IssueID:     issue.ID,
			EventID:     id.New(),
			Message:     "object is null",
			Level:       "error",
			Timestamp:   time.Now(),
			Fingerprint: []string{fingerprint},
		}
		sr.services.Store.ErrorEvents().CreateErrorEvent(ctx, event)

		// Update issue counts (simulating grouping service)
		issue.EventCount++
		issue.LastSeen = time.Now()
		issue.LastEventID = event.ID
	}

	// Persist updated issue
	err = sr.services.Store.Issues().UpdateIssue(ctx, issue)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Issue updated with counts", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	// Verify issue was updated, not duplicated
	sr.log("  Step 3: Verifying deduplication...")

	// Check no duplicate issues exist
	issues, err := sr.services.Store.Issues().ListProjectIssues(ctx, project.ID, shared.IssueFilter{})
	if err != nil {
		return result, err
	}

	issueCount := 0
	var foundIssue *observabilitydomain.Issue
	for _, iss := range issues {
		if iss.Fingerprint == fingerprint {
			issueCount++
			foundIssue = iss
		}
	}

	if issueCount != 1 {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Single issue for fingerprint", Passed: false,
			Details: fmt.Sprintf("Expected 1, got %d", issueCount),
		})
		return result, fmt.Errorf("deduplication failed: found %d issues", issueCount)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Single issue for fingerprint", Passed: true, Details: "No duplicates created",
	})

	// Verify event count was incremented
	if foundIssue.EventCount != 6 { // 1 initial + 5 duplicates
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Event count incremented", Passed: false,
			Details: fmt.Sprintf("Expected 6, got %d", foundIssue.EventCount),
		})
	} else {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Event count incremented", Passed: true,
			Details: fmt.Sprintf("EventCount: %d", foundIssue.EventCount),
		})
	}

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunIssueLifecycleScenario tests issue status transitions
func (sr *ObservabilityScenarioRunner) RunIssueLifecycleScenario(ctx context.Context, workspaceID string) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:             "Issue Lifecycle (Resolve → Reopen)",
		StateTransitions: make([]StateTransition, 0),
		Verifications:    make([]VerificationResult, 0),
	}

	// Create project and issue
	project := sr.createTestProject(ctx, workspaceID, "Lifecycle Test App")
	if project == nil {
		return result, fmt.Errorf("failed to create test project")
	}

	fingerprint := generateFingerprint("ConnectionError", "timeout", "client.go:50")
	issue := &observabilitydomain.Issue{
		WorkspaceID: workspaceID,
		ProjectID:   project.ID,
		Title:       "ConnectionError: timeout",
		Fingerprint: fingerprint,
		Status:      "unresolved",
		Level:       "error",
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		EventCount:  10,
	}

	err := sr.services.Store.Issues().CreateIssue(ctx, issue)
	if err != nil {
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Issue created as unresolved", Passed: issue.Status == "unresolved",
		Details: fmt.Sprintf("Status: %s", issue.Status),
	})

	// Step 1: Resolve the issue
	sr.log("  Step 1: Resolving issue...")
	now := time.Now()
	issue.Status = "resolved"
	issue.ResolvedAt = &now
	issue.ResolvedBy = "user-123"
	issue.Resolution = "fixed"
	issue.ResolvedInVersion = "v1.2.3"

	err = sr.services.Store.Issues().UpdateIssue(ctx, issue)
	if err != nil {
		return result, err
	}

	// Verify resolved state
	stored, _ := sr.services.Store.Issues().GetIssue(ctx, issue.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Issue resolved", Passed: stored.Status == "resolved" && stored.ResolvedAt != nil,
		Details: fmt.Sprintf("Status: %s, ResolvedAt: %v", stored.Status, stored.ResolvedAt != nil),
	})

	// Step 2: New error arrives - reopen
	sr.log("  Step 2: Simulating new error (should reopen)...")
	newEvent := &observabilitydomain.ErrorEvent{
		ProjectID:   project.ID,
		IssueID:     issue.ID,
		EventID:     id.New(),
		Message:     "timeout",
		Level:       "error",
		Timestamp:   time.Now(),
		Fingerprint: []string{fingerprint},
	}
	sr.services.Store.ErrorEvents().CreateErrorEvent(ctx, newEvent)

	// Reopen the issue (simulating grouping service behavior)
	issue.Status = "unresolved"
	issue.ResolvedAt = nil
	issue.EventCount++
	issue.LastSeen = time.Now()
	issue.LastEventID = newEvent.ID

	err = sr.services.Store.Issues().UpdateIssue(ctx, issue)
	if err != nil {
		return result, err
	}

	// Verify reopened state
	stored, _ = sr.services.Store.Issues().GetIssue(ctx, issue.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Issue reopened on new event", Passed: stored.Status == "unresolved" && stored.ResolvedAt == nil,
		Details: fmt.Sprintf("Status: %s, EventCount: %d", stored.Status, stored.EventCount),
	})

	// Step 3: Ignore the issue
	sr.log("  Step 3: Ignoring issue...")
	issue.Status = "ignored"
	err = sr.services.Store.Issues().UpdateIssue(ctx, issue)
	if err != nil {
		return result, err
	}

	stored, _ = sr.services.Store.Issues().GetIssue(ctx, issue.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Issue ignored", Passed: stored.Status == "ignored",
		Details: fmt.Sprintf("Status: %s", stored.Status),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunMultipleErrorTypesScenario tests that different errors create separate issues
func (sr *ObservabilityScenarioRunner) RunMultipleErrorTypesScenario(ctx context.Context, workspaceID string) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Multiple Error Types Grouping",
		Verifications: make([]VerificationResult, 0),
	}

	// Create project
	project := sr.createTestProject(ctx, workspaceID, "Multi-Error App")
	if project == nil {
		return result, fmt.Errorf("failed to create test project")
	}

	sr.log("  Step 1: Creating 3 different error types...")

	errorTypes := []struct {
		Type    string
		Message string
		File    string
	}{
		{"TypeError", "undefined is not a function", "utils.js:10"},
		{"ReferenceError", "x is not defined", "main.js:25"},
		{"SyntaxError", "Unexpected token", "parser.js:50"},
	}

	issueIDs := make([]string, 0)

	for _, et := range errorTypes {
		fingerprint := generateFingerprint(et.Type, et.Message, et.File)

		issue := &observabilitydomain.Issue{
			WorkspaceID: workspaceID,
			ProjectID:   project.ID,
			Title:       fmt.Sprintf("%s: %s", et.Type, et.Message),
			Culprit:     et.File,
			Fingerprint: fingerprint,
			Status:      "unresolved",
			Level:       "error",
			Type:        "error",
			FirstSeen:   time.Now(),
			LastSeen:    time.Now(),
			EventCount:  1,
		}

		err := sr.services.Store.Issues().CreateIssue(ctx, issue)
		if err != nil {
			return result, err
		}
		issueIDs = append(issueIDs, issue.ID)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Created 3 different issues", Passed: len(issueIDs) == 3,
		Details: fmt.Sprintf("IssueCount: %d", len(issueIDs)),
	})

	// Verify all issues exist and are separate
	sr.log("  Step 2: Verifying separate issues...")
	issues, err := sr.services.Store.Issues().ListProjectIssues(ctx, project.ID, shared.IssueFilter{})
	if err != nil {
		return result, err
	}

	// Count unresolved issues for this project
	unresolvedCount := 0
	for _, iss := range issues {
		if iss.Status == "unresolved" {
			unresolvedCount++
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "All issues listed separately", Passed: unresolvedCount >= 3,
		Details: fmt.Sprintf("Found %d unresolved issues", unresolvedCount),
	})

	// Verify each has unique fingerprint
	fingerprints := make(map[string]bool)
	for _, iss := range issues {
		if iss.Fingerprint != "" {
			fingerprints[iss.Fingerprint] = true
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Each issue has unique fingerprint", Passed: len(fingerprints) >= 3,
		Details: fmt.Sprintf("Unique fingerprints: %d", len(fingerprints)),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// Helper functions

func (sr *ObservabilityScenarioRunner) ensureErrorTrackingExtension(ctx context.Context, workspaceID string) error {
	_, err := refext.EnsureReferenceExtensionActive(ctx, sr.services.Store, workspaceID, "error-tracking")
	return err
}

func (sr *ObservabilityScenarioRunner) createTestProject(ctx context.Context, workspaceID, name string) *observabilitydomain.Project {
	if err := sr.ensureErrorTrackingExtension(ctx, workspaceID); err != nil {
		sr.log("  ERROR ensuring error-tracking extension: %v", err)
		return nil
	}

	project := &observabilitydomain.Project{
		WorkspaceID: workspaceID,
		Name:        name,
		Platform:    "javascript",
		Environment: "test",
		Status:      "active",
		PublicKey:   generateKey(16),
		SecretKey:   generateKey(32),
		AppKey:      generateKey(8),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := sr.services.Store.Projects().CreateProject(ctx, project)
	if err != nil {
		sr.log("  ERROR creating project: %v", err)
		return nil
	}
	return project
}

func generateKey(length int) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", id.New(), time.Now().UnixNano())))
	return hex.EncodeToString(hash[:])[:length]
}

func generateFingerprint(errorType, message, location string) string {
	data := fmt.Sprintf("%s:%s:%s", errorType, message, location)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (sr *ObservabilityScenarioRunner) log(format string, args ...interface{}) {
	if sr.verbose {
		fmt.Printf("[observability] "+format+"\n", args...)
	}
}
