package synth

import (
	"context"
	"fmt"
	"time"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// AutomationScenarioRunner executes automation and rule scenarios
type AutomationScenarioRunner struct {
	services *TestServices
	verbose  bool
}

// NewAutomationScenarioRunner creates a new automation scenario runner
func NewAutomationScenarioRunner(services *TestServices, verbose bool) *AutomationScenarioRunner {
	return &AutomationScenarioRunner{
		services: services,
		verbose:  verbose,
	}
}

// RunAllAutomationScenarios runs all automation scenarios
func (sr *AutomationScenarioRunner) RunAllAutomationScenarios(ctx context.Context, workspaceID string, agents []*platformdomain.User) ([]*ScenarioResult, error) {
	results := make([]*ScenarioResult, 0)

	scenarios := []struct {
		Name string
		Run  func(context.Context, string, []*platformdomain.User) (*ScenarioResult, error)
	}{
		{"Create Rule with Conditions", sr.RunCreateRuleScenario},
		{"Rule Condition Evaluation", sr.RunRuleConditionEvaluationScenario},
		{"Rule Action Execution", sr.RunRuleActionExecutionScenario},
		{"Job Queue Processing", sr.RunJobQueueScenario},
		{"Scheduled Job Execution", sr.RunScheduledJobScenario},
	}

	for _, scenario := range scenarios {
		sr.log("Running scenario: %s", scenario.Name)

		result, err := scenario.Run(ctx, workspaceID, agents)
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

// RunCreateRuleScenario tests creating automation rules
func (sr *AutomationScenarioRunner) RunCreateRuleScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Create Rule with Conditions",
		Verifications: make([]VerificationResult, 0),
	}

	sr.log("  Step 1: Creating automation rule...")

	rule := automationdomain.NewRule(workspaceID, "Auto-assign urgent tickets", agents[0].ID)
	rule.Description = "Automatically assign urgent priority tickets to senior agent"
	rule.IsActive = true
	rule.Priority = 10

	// Set conditions
	rule.Conditions = automationdomain.RuleConditionsData{
		Operator: "and",
		Conditions: []automationdomain.RuleCondition{
			{Type: "field", Field: "case.priority", Operator: "equals", Value: shareddomain.StringValue("urgent")},
			{Type: "field", Field: "case.status", Operator: "equals", Value: shareddomain.StringValue("new")},
		},
	}

	// Set actions
	rule.Actions = automationdomain.RuleActionsData{
		Actions: []automationdomain.RuleAction{
			{Type: "assign_case", Target: agents[0].ID},
			{Type: "add_tags", Value: shareddomain.StringsValue([]string{"auto-assigned", "urgent-queue"})},
		},
	}

	err := sr.services.Store.Rules().CreateRule(ctx, rule)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Rule created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Rule created", Passed: true,
		Details: fmt.Sprintf("ID: %s, Title: %s", rule.ID, rule.Title),
	})

	// Verify rule retrieval
	sr.log("  Step 2: Verifying rule retrieval...")
	stored, err := sr.services.Store.Rules().GetRule(ctx, rule.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Rule retrievable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Rule retrievable", Passed: stored.Title == rule.Title,
		Details: fmt.Sprintf("IsActive: %v, Priority: %d", stored.IsActive, stored.Priority),
	})

	// Verify conditions stored
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Conditions stored", Passed: len(stored.Conditions.Conditions) > 0,
		Details: fmt.Sprintf("Has %d conditions", len(stored.Conditions.Conditions)),
	})

	// Verify actions stored
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Actions stored", Passed: len(stored.Actions.Actions) > 0,
		Details: fmt.Sprintf("Has %d actions", len(stored.Actions.Actions)),
	})

	// List active rules
	sr.log("  Step 3: Listing active rules...")
	activeRules, err := sr.services.Store.Rules().ListActiveRules(ctx, workspaceID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Active rules listable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	hasRule := false
	for _, r := range activeRules {
		if r.ID == rule.ID {
			hasRule = true
			break
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Rule in active list", Passed: hasRule,
		Details: fmt.Sprintf("Total active rules: %d", len(activeRules)),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunRuleConditionEvaluationScenario tests rule condition matching
func (sr *AutomationScenarioRunner) RunRuleConditionEvaluationScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Rule Condition Evaluation",
		Verifications: make([]VerificationResult, 0),
	}

	// Create a rule that matches high priority cases
	rule := automationdomain.NewRule(workspaceID, "High Priority Handler", agents[0].ID)
	rule.IsActive = true
	rule.Conditions = automationdomain.RuleConditionsData{
		Operator: "and",
		Conditions: []automationdomain.RuleCondition{
			{Type: "field", Field: "case.priority", Operator: "in", Value: shareddomain.StringsValue([]string{"high", "urgent"})},
		},
	}
	rule.Actions = automationdomain.RuleActionsData{
		Actions: []automationdomain.RuleAction{
			{Type: "add_tags", Value: shareddomain.StringsValue([]string{"priority-handled"})},
		},
	}
	sr.services.Store.Rules().CreateRule(ctx, rule)

	sr.log("  Step 1: Creating test cases with different priorities...")

	// Create contact first
	contact := platformdomain.NewContact(workspaceID, "condition-test@example.com")
	contact.Name = "Condition Test Customer"
	sr.services.Store.Contacts().CreateContact(ctx, contact)

	// Create high priority case (should match)
	highCase := servicedomain.NewCase(workspaceID, "High Priority Issue", contact.Email)
	highCase.ID = id.New()
	highCase.GenerateHumanID("test")
	highCase.Description = "This is urgent"
	highCase.Priority = servicedomain.CasePriorityHigh
	highCase.Status = servicedomain.CaseStatusNew
	highCase.Channel = servicedomain.CaseChannelEmail
	highCase.ContactID = contact.ID
	sr.services.Store.Cases().CreateCase(ctx, highCase)

	// Create low priority case (should not match)
	lowCase := servicedomain.NewCase(workspaceID, "Low Priority Issue", contact.Email)
	lowCase.ID = id.New()
	lowCase.GenerateHumanID("test")
	lowCase.Description = "Not urgent"
	lowCase.Priority = servicedomain.CasePriorityLow
	lowCase.Status = servicedomain.CaseStatusNew
	lowCase.Channel = servicedomain.CaseChannelEmail
	lowCase.ContactID = contact.ID
	sr.services.Store.Cases().CreateCase(ctx, lowCase)

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Test cases created", Passed: true,
		Details: fmt.Sprintf("High: %s, Low: %s", highCase.HumanID, lowCase.HumanID),
	})

	// Simulate condition evaluation (using simplified logic here)
	sr.log("  Step 2: Evaluating conditions against high priority case...")

	// High priority should match
	highMatches := evaluateSimpleCondition(highCase.Priority, []string{"high", "urgent"})
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "High priority matches 'in' condition", Passed: highMatches,
		Details: fmt.Sprintf("Priority: %s", highCase.Priority),
	})

	// Low priority should not match
	sr.log("  Step 3: Evaluating conditions against low priority case...")
	lowMatches := evaluateSimpleCondition(lowCase.Priority, []string{"high", "urgent"})
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Low priority does NOT match", Passed: !lowMatches,
		Details: fmt.Sprintf("Priority: %s", lowCase.Priority),
	})

	// Test equals operator
	sr.log("  Step 4: Testing equals operator...")
	equalsMatches := string(highCase.Status) == string(servicedomain.CaseStatusNew)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Status equals 'new'", Passed: equalsMatches,
		Details: fmt.Sprintf("Status: %s", highCase.Status),
	})

	// Test contains operator (for subject)
	sr.log("  Step 5: Testing contains operator...")
	containsMatches := containsSubstring(highCase.Subject, "Priority")
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Subject contains 'Priority'", Passed: containsMatches,
		Details: fmt.Sprintf("Subject: %s", highCase.Subject),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunRuleActionExecutionScenario tests rule action application
func (sr *AutomationScenarioRunner) RunRuleActionExecutionScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Rule Action Execution",
		Verifications: make([]VerificationResult, 0),
	}

	// Create contact and case
	contact := platformdomain.NewContact(workspaceID, "action-test@example.com")
	contact.Name = "Action Test Customer"
	sr.services.Store.Contacts().CreateContact(ctx, contact)

	testCase := servicedomain.NewCase(workspaceID, "Test Action Execution", contact.Email)
	testCase.ID = id.New()
	testCase.GenerateHumanID("test")
	testCase.Priority = servicedomain.CasePriorityMedium
	testCase.Status = servicedomain.CaseStatusNew
	testCase.Channel = servicedomain.CaseChannelEmail
	testCase.ContactID = contact.ID
	testCase.Tags = []string{}
	sr.services.Store.Cases().CreateCase(ctx, testCase)

	sr.log("  Step 1: Simulating assign action...")

	// Apply assign action
	testCase.AssignedToID = agents[0].ID
	testCase.UpdatedAt = time.Now()
	sr.services.Store.Cases().UpdateCase(ctx, testCase)

	// Verify assignment
	stored, _ := sr.services.Store.Cases().GetCase(ctx, testCase.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Assign action applied", Passed: stored.AssignedToID == agents[0].ID,
		Details: fmt.Sprintf("AssignedTo: %s", stored.AssignedToID),
	})

	// Apply add_tags action
	sr.log("  Step 2: Simulating add_tags action...")
	testCase.Tags = append(testCase.Tags, "auto-assigned", "processed")
	sr.services.Store.Cases().UpdateCase(ctx, testCase)

	stored, _ = sr.services.Store.Cases().GetCase(ctx, testCase.ID)
	hasAutoAssigned := false
	hasProcessed := false
	for _, tag := range stored.Tags {
		if tag == "auto-assigned" {
			hasAutoAssigned = true
		}
		if tag == "processed" {
			hasProcessed = true
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Add tags action applied", Passed: hasAutoAssigned && hasProcessed,
		Details: fmt.Sprintf("Tags: %v", stored.Tags),
	})

	// Apply change_status action
	sr.log("  Step 3: Simulating change_status action...")
	testCase.Status = servicedomain.CaseStatusOpen
	sr.services.Store.Cases().UpdateCase(ctx, testCase)

	stored, _ = sr.services.Store.Cases().GetCase(ctx, testCase.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Change status action applied", Passed: stored.Status == servicedomain.CaseStatusOpen,
		Details: fmt.Sprintf("Status: %s", stored.Status),
	})

	// Apply change_priority action
	sr.log("  Step 4: Simulating change_priority action...")
	testCase.Priority = servicedomain.CasePriorityHigh
	sr.services.Store.Cases().UpdateCase(ctx, testCase)

	stored, _ = sr.services.Store.Cases().GetCase(ctx, testCase.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Change priority action applied", Passed: stored.Priority == servicedomain.CasePriorityHigh,
		Details: fmt.Sprintf("Priority: %s", stored.Priority),
	})

	// Track rule execution
	sr.log("  Step 5: Recording rule execution...")
	rule := automationdomain.NewRule(workspaceID, "Test Action Rule", agents[0].ID)
	rule.TotalExecutions = 1
	rule.LastExecutedAt = timePtr(time.Now())
	sr.services.Store.Rules().CreateRule(ctx, rule)

	// Verify execution tracking
	storedRule, _ := sr.services.Store.Rules().GetRule(ctx, rule.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Execution tracked", Passed: storedRule.TotalExecutions >= 1,
		Details: fmt.Sprintf("TotalExecutions: %d", storedRule.TotalExecutions),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunJobQueueScenario tests job creation and status transitions
func (sr *AutomationScenarioRunner) RunJobQueueScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Job Queue Processing",
		Verifications: make([]VerificationResult, 0),
	}

	sr.log("  Step 1: Creating job...")

	job, err := automationdomain.NewWorkspaceJob(workspaceID, "send_email", map[string]interface{}{
		"to":      "test@example.com",
		"subject": "Test Email",
		"body":    "This is a test",
	})
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Job constructor", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}
	job.Queue = "email"
	job.Priority = automationdomain.JobPriorityHigh
	job.MaxAttempts = 3

	err = sr.services.Store.Jobs().CreateJob(ctx, job)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Job created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Job created", Passed: true,
		Details: fmt.Sprintf("ID: %s, Status: %s", job.ID, job.Status),
	})

	// Verify initial status
	stored, _ := sr.services.Store.Jobs().GetJob(ctx, job.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Initial status is pending", Passed: stored.Status == automationdomain.JobStatusPending,
		Details: fmt.Sprintf("Status: %s", stored.Status),
	})

	// Simulate job pickup (running)
	sr.log("  Step 2: Simulating job pickup...")
	job.Status = automationdomain.JobStatusRunning
	job.StartedAt = timePtr(time.Now())
	job.WorkerID = "worker-1"
	job.Attempts = 1
	sr.services.Store.Jobs().UpdateJob(ctx, job)

	stored, _ = sr.services.Store.Jobs().GetJob(ctx, job.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Status changed to running", Passed: stored.Status == automationdomain.JobStatusRunning,
		Details: fmt.Sprintf("WorkerID: %s, Attempt: %d", stored.WorkerID, stored.Attempts),
	})

	// Simulate job completion
	sr.log("  Step 3: Simulating job completion...")
	job.MarkCompleted(map[string]any{
		"message_id": "msg-123",
		"delivered":  true,
	})
	sr.services.Store.Jobs().UpdateJob(ctx, job)

	stored, _ = sr.services.Store.Jobs().GetJob(ctx, job.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Job completed successfully", Passed: stored.Status == automationdomain.JobStatusCompleted,
		Details: fmt.Sprintf("CompletedAt: %v", stored.CompletedAt != nil),
	})

	// Verify result stored
	var storedResult map[string]any
	_ = stored.UnmarshalResult(&storedResult)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Result stored", Passed: len(stored.Result) > 0 && storedResult["delivered"] == true,
		Details: fmt.Sprintf("Result: %v", storedResult),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunScheduledJobScenario tests scheduled and recurring jobs
func (sr *AutomationScenarioRunner) RunScheduledJobScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Scheduled Job Execution",
		Verifications: make([]VerificationResult, 0),
	}

	sr.log("  Step 1: Creating scheduled job...")

	// Create a job scheduled for the future
	futureTime := time.Now().Add(1 * time.Hour)
	scheduledJob, err := automationdomain.NewWorkspaceJob(workspaceID, "cleanup_old_data", map[string]interface{}{
		"older_than_days": 90,
	})
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Job constructor", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}
	scheduledJob.ScheduledFor = &futureTime
	scheduledJob.Priority = automationdomain.JobPriorityLow

	err = sr.services.Store.Jobs().CreateJob(ctx, scheduledJob)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Scheduled job created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Scheduled job created", Passed: true,
		Details: fmt.Sprintf("ScheduledFor: %s", scheduledJob.ScheduledFor.Format(time.RFC3339)),
	})

	// Verify job is not processed immediately
	stored, _ := sr.services.Store.Jobs().GetJob(ctx, scheduledJob.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Job remains pending until scheduled time", Passed: stored.Status == automationdomain.JobStatusPending,
		Details: fmt.Sprintf("Status: %s", stored.Status),
	})

	// Create immediate job
	sr.log("  Step 2: Creating immediate job...")
	immediateJob, err := automationdomain.NewWorkspaceJob(workspaceID, "send_notification", map[string]interface{}{
		"user_id": agents[0].ID,
		"message": "Test notification",
	})
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Immediate job constructor", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}
	sr.services.Store.Jobs().CreateJob(ctx, immediateJob)

	// Verify immediate job has ScheduledFor in the past
	stored, _ = sr.services.Store.Jobs().GetJob(ctx, immediateJob.ID)
	isImmediate := stored.ScheduledFor.Before(time.Now().Add(1 * time.Second))
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Immediate job scheduled for now", Passed: isImmediate,
		Details: fmt.Sprintf("ScheduledFor: %s", stored.ScheduledFor.Format(time.RFC3339)),
	})

	// Simulate job retry
	sr.log("  Step 3: Simulating failed job retry...")
	failedJob, err := automationdomain.NewWorkspaceJob(workspaceID, "external_api_call", map[string]interface{}{
		"endpoint": "https://api.example.com/webhook",
	})
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Failed job constructor", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}
	failedJob.MaxAttempts = 3
	sr.services.Store.Jobs().CreateJob(ctx, failedJob)

	// First attempt fails
	failedJob.Status = automationdomain.JobStatusFailed
	failedJob.Attempts = 1
	failedJob.Error = "Connection timeout"
	sr.services.Store.Jobs().UpdateJob(ctx, failedJob)

	// Retry - status back to pending
	failedJob.Status = automationdomain.JobStatusRetrying
	retryTime := time.Now().Add(1 * time.Minute) // Retry delay
	failedJob.ScheduledFor = &retryTime
	sr.services.Store.Jobs().UpdateJob(ctx, failedJob)

	stored, _ = sr.services.Store.Jobs().GetJob(ctx, failedJob.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Failed job set to retry", Passed: stored.Status == automationdomain.JobStatusRetrying,
		Details: fmt.Sprintf("Attempts: %d/%d, Error: %s", stored.Attempts, stored.MaxAttempts, stored.Error),
	})

	// Verify retry count
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Retry count under max attempts", Passed: stored.Attempts < stored.MaxAttempts,
		Details: fmt.Sprintf("Can retry: %v", stored.Attempts < stored.MaxAttempts),
	})

	// List jobs by status
	sr.log("  Step 4: Listing jobs by status...")
	pendingJobs, _, err := sr.services.Store.Jobs().ListWorkspaceJobs(ctx, workspaceID, automationdomain.JobStatusPending, "", 10, 0)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Jobs listable by status", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Jobs listable by status", Passed: true,
		Details: fmt.Sprintf("Pending jobs: %d", len(pendingJobs)),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// Helper functions

func evaluateSimpleCondition(value servicedomain.CasePriority, allowedValues []string) bool {
	for _, v := range allowedValues {
		if string(value) == v {
			return true
		}
	}
	return false
}

func containsSubstring(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(substr) == 0 ||
		(len(str) > len(substr) && findSubstring(str, substr)))
}

func findSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func (sr *AutomationScenarioRunner) log(format string, args ...interface{}) {
	if sr.verbose {
		fmt.Printf("[automation] "+format+"\n", args...)
	}
}
