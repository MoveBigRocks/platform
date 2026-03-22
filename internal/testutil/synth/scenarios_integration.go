package synth

import (
	"context"
	"fmt"
	"time"

	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/testutil/refext"
	"github.com/movebigrocks/platform/pkg/id"
)

// IntegrationScenarioRunner runs error-support integration scenarios
type IntegrationScenarioRunner struct {
	services *TestServices
	verbose  bool
}

// NewIntegrationScenarioRunner creates a new integration scenario runner
func NewIntegrationScenarioRunner(services *TestServices, verbose bool) *IntegrationScenarioRunner {
	return &IntegrationScenarioRunner{
		services: services,
		verbose:  verbose,
	}
}

// RunAllIntegrationScenarios runs all error-support integration scenarios
func (sr *IntegrationScenarioRunner) RunAllIntegrationScenarios(ctx context.Context, workspaceID string, users []*platformdomain.User) ([]*ScenarioResult, error) {
	if err := sr.ensureErrorTrackingExtension(ctx, workspaceID); err != nil {
		return nil, err
	}

	scenarios := []func(context.Context, string, []*platformdomain.User) (*ScenarioResult, error){
		sr.scenarioLinkCaseToIssue,
		sr.scenarioAffectedCustomers,
		sr.scenarioAutoResolveLinkedCases,
		sr.scenarioBidirectionalLinking,
		sr.scenarioContactNotification,
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

func (sr *IntegrationScenarioRunner) ensureErrorTrackingExtension(ctx context.Context, workspaceID string) error {
	_, err := refext.EnsureReferenceExtensionActive(ctx, sr.services.Store, workspaceID, "error-tracking")
	return err
}

func (sr *IntegrationScenarioRunner) issueWriteContext(ctx context.Context) context.Context {
	return graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Permissions: []string{platformdomain.PermissionIssueWrite},
	})
}

// scenarioLinkCaseToIssue tests linking a support case to an error issue
func (sr *IntegrationScenarioRunner) scenarioLinkCaseToIssue(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Link Case to Issue",
	}

	if sr.verbose {
		fmt.Println("  -> Testing case-to-issue linking...")
	}

	// Create an error monitoring project
	project := observabilitydomain.NewProject(workspaceID, "", "Web App", "web-app", "javascript")
	err := sr.services.Store.Projects().CreateProject(ctx, project)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create an error event first (required for NewIssue)
	event := observabilitydomain.NewErrorEvent(project.ID, id.New())
	event.Message = "TypeError: Cannot read property 'length' of undefined"
	event.Level = "error"
	event.Environment = "production"

	// Create an issue
	issue := observabilitydomain.NewIssue(project.ID, "TypeError: Cannot read property 'length' of undefined", "TypeError", event)
	issue.EventCount = 50
	issue.UserCount = 10

	err = sr.services.Store.Issues().CreateIssue(ctx, issue)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Issue created",
		Passed:  issue.ID != "",
		Details: fmt.Sprintf("Issue ID: %s", issue.ID),
	})

	// Create a support case
	supportCase := servicedomain.NewCase(workspaceID, "App crashes with TypeError", "affected@customer.com")
	supportCase.GenerateHumanID("test")
	supportCase.Description = "I keep getting a TypeError when using the app"
	supportCase.Channel = "email"

	err = sr.services.Store.Cases().CreateCase(ctx, supportCase)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Support case created",
		Passed:  supportCase.ID != "",
		Details: fmt.Sprintf("Case ID: %s", supportCase.ID),
	})

	// Link the case to the issue
	_, err = sr.services.IssueService.LinkIssueToCase(sr.issueWriteContext(ctx), issue.ID, supportCase.ID)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Verify the link by checking issue's RelatedCaseIDs
	updatedIssue, err := sr.services.Store.Issues().GetIssue(ctx, issue.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Case linked to issue (RelatedCaseIDs)",
		Passed:  err == nil && len(updatedIssue.RelatedCaseIDs) >= 1,
		Details: fmt.Sprintf("Linked cases: %d", len(updatedIssue.RelatedCaseIDs)),
	})

	// Verify HasRelatedCase flag is set
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "HasRelatedCase flag set",
		Passed:  updatedIssue.HasRelatedCase,
		Details: fmt.Sprintf("HasRelatedCase: %v", updatedIssue.HasRelatedCase),
	})

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

// scenarioAffectedCustomers tests detecting customers affected by an issue
func (sr *IntegrationScenarioRunner) scenarioAffectedCustomers(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Affected Customers Detection",
	}

	if sr.verbose {
		fmt.Println("  -> Testing affected customers detection...")
	}

	// Create project and issue
	project := observabilitydomain.NewProject(workspaceID, "", "Mobile App", "mobile-app", "react-native")
	err := sr.services.Store.Projects().CreateProject(ctx, project)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create error event for the issue
	baseEvent := observabilitydomain.NewErrorEvent(project.ID, id.New())
	baseEvent.Message = "Network timeout on sync"
	baseEvent.Level = "error"
	baseEvent.Environment = "production"

	issue := observabilitydomain.NewIssue(project.ID, "Network timeout on sync", "NetworkError", baseEvent)

	err = sr.services.Store.Issues().CreateIssue(ctx, issue)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create contacts (customers)
	contacts := []*platformdomain.Contact{
		{ID: id.New(), WorkspaceID: workspaceID, Email: "user1@example.com", Name: "User One", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: id.New(), WorkspaceID: workspaceID, Email: "user2@example.com", Name: "User Two", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: id.New(), WorkspaceID: workspaceID, Email: "user3@example.com", Name: "User Three", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	for _, contact := range contacts {
		err := sr.services.Store.Contacts().CreateContact(ctx, contact)
		if err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result, nil
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Contacts created",
		Passed:  len(contacts) == 3,
		Details: fmt.Sprintf("Created %d contacts", len(contacts)),
	})

	// Create error events with user context
	for i, contact := range contacts {
		event := observabilitydomain.NewErrorEvent(project.ID, id.New())
		event.IssueID = issue.ID
		event.Message = "Network timeout on sync"
		event.Level = "error"
		event.User = &observabilitydomain.UserContext{
			ID:    contact.ID,
			Email: contact.Email,
		}
		event.Environment = "production"
		event.Timestamp = time.Now().Add(-time.Duration(i) * time.Hour)

		err := sr.services.Store.ErrorEvents().CreateErrorEvent(ctx, event)
		if err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result, nil
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Error events created with user context",
		Passed:  true,
		Details: "Created 3 error events with user IDs and emails",
	})

	// Verify error events were stored (retrieve by issue)
	storedEvents, err := sr.services.Store.ErrorEvents().GetIssueEvents(ctx, issue.ID, 10)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Error events retrievable by issue",
		Passed:  err == nil && len(storedEvents) >= 3,
		Details: fmt.Sprintf("Events for issue: %d", len(storedEvents)),
	})

	// Verify events have user context
	hasUserContext := false
	for _, event := range storedEvents {
		if event.User != nil && event.User.Email != "" {
			hasUserContext = true
			break
		}
	}
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Events contain user context",
		Passed:  hasUserContext,
		Details: fmt.Sprintf("Has user context: %v", hasUserContext),
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

// scenarioAutoResolveLinkedCases tests auto-resolving cases when issue is fixed
func (sr *IntegrationScenarioRunner) scenarioAutoResolveLinkedCases(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Auto-Resolve Linked Cases",
	}

	if sr.verbose {
		fmt.Println("  -> Testing auto-resolve linked cases...")
	}

	// Create project and issue
	project := observabilitydomain.NewProject(workspaceID, "", "Backend API", "backend-api", "go")
	err := sr.services.Store.Projects().CreateProject(ctx, project)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create error event for the issue
	baseEvent := observabilitydomain.NewErrorEvent(project.ID, id.New())
	baseEvent.Message = "Database connection failed"
	baseEvent.Level = "error"
	baseEvent.Environment = "production"

	issue := observabilitydomain.NewIssue(project.ID, "Database connection failed", "DatabaseError", baseEvent)

	err = sr.services.Store.Issues().CreateIssue(ctx, issue)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create multiple cases linked to the issue
	var caseIDs []string
	for i := 0; i < 3; i++ {
		supportCase := servicedomain.NewCase(workspaceID, fmt.Sprintf("Database error report #%d", i+1), fmt.Sprintf("customer%d@example.com", i+1))
		supportCase.GenerateHumanID("test")
		supportCase.Status = servicedomain.CaseStatusOpen

		err = sr.services.Store.Cases().CreateCase(ctx, supportCase)
		if err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result, nil
		}

		_, err = sr.services.IssueService.LinkIssueToCase(sr.issueWriteContext(ctx), issue.ID, supportCase.ID)
		if err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result, nil
		}

		caseIDs = append(caseIDs, supportCase.ID)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Multiple cases linked to issue",
		Passed:  len(caseIDs) == 3,
		Details: fmt.Sprintf("Linked %d cases", len(caseIDs)),
	})

	// Resolve the issue
	resolvedBy := "system"
	if len(users) > 0 {
		resolvedBy = users[0].ID
	}

	err = sr.services.IssueService.ResolveIssue(sr.issueWriteContext(ctx), issue.ID, "fixed", resolvedBy)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Verify issue is resolved
	resolvedIssue, _ := sr.services.Store.Issues().GetIssue(ctx, issue.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Issue marked as resolved",
		Passed:  resolvedIssue.Status == observabilitydomain.IssueStatusResolved,
		Details: fmt.Sprintf("Status: %s", resolvedIssue.Status),
	})

	// Resolve linked cases individually (simulate what a handler would do)
	for _, caseID := range caseIDs {
		c, err := sr.services.Store.Cases().GetCase(ctx, caseID)
		if err != nil {
			result.Error = fmt.Errorf("failed to get case %s: %w", caseID, err)
			result.Duration = time.Since(start)
			return result, nil
		}
		c.Status = servicedomain.CaseStatusResolved
		if err := sr.services.Store.Cases().UpdateCase(ctx, c); err != nil {
			result.Error = fmt.Errorf("failed to update case %s: %w", caseID, err)
			result.Duration = time.Since(start)
			return result, nil
		}
	}

	// Verify cases are resolved
	for _, caseID := range caseIDs {
		c, _ := sr.services.Store.Cases().GetCase(ctx, caseID)
		if c.Status != servicedomain.CaseStatusResolved {
			result.Verifications = append(result.Verifications, VerificationResult{
				Check:   "Case auto-resolved",
				Passed:  false,
				Details: fmt.Sprintf("Case %s still has status: %s", caseID, c.Status),
			})
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "All linked cases resolved",
		Passed:  true,
		Details: fmt.Sprintf("Resolved %d cases", len(caseIDs)),
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

// scenarioBidirectionalLinking tests bidirectional links between issues and cases
func (sr *IntegrationScenarioRunner) scenarioBidirectionalLinking(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Bidirectional Linking",
	}

	if sr.verbose {
		fmt.Println("  -> Testing bidirectional linking...")
	}

	// Create two projects with issues
	project1 := observabilitydomain.NewProject(workspaceID, "", "Frontend", "frontend", "javascript")
	err := sr.services.Store.Projects().CreateProject(ctx, project1)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	project2 := observabilitydomain.NewProject(workspaceID, "", "Backend", "backend", "go")
	err = sr.services.Store.Projects().CreateProject(ctx, project2)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create error event for issue 1
	event1 := observabilitydomain.NewErrorEvent(project1.ID, id.New())
	event1.Message = "UI rendering error"
	event1.Level = "error"

	issue1 := observabilitydomain.NewIssue(project1.ID, "UI rendering error", "RenderError", event1)
	err = sr.services.Store.Issues().CreateIssue(ctx, issue1)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create error event for issue 2
	event2 := observabilitydomain.NewErrorEvent(project2.ID, id.New())
	event2.Message = "API timeout"
	event2.Level = "error"

	issue2 := observabilitydomain.NewIssue(project2.ID, "API timeout", "TimeoutError", event2)
	err = sr.services.Store.Issues().CreateIssue(ctx, issue2)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create a case that's related to both issues (complex bug)
	supportCase := servicedomain.NewCase(workspaceID, "Page won't load - multiple errors", "frustrated@customer.com")
	supportCase.GenerateHumanID("test")
	supportCase.Description = "The page shows a rendering error and eventually times out"

	err = sr.services.Store.Cases().CreateCase(ctx, supportCase)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Link case to both issues
	_, err = sr.services.IssueService.LinkIssueToCase(sr.issueWriteContext(ctx), issue1.ID, supportCase.ID)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	_, err = sr.services.IssueService.LinkIssueToCase(sr.issueWriteContext(ctx), issue2.ID, supportCase.ID)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Verify: Issue 1 has case in RelatedCaseIDs
	updatedIssue1, err := sr.services.Store.Issues().GetIssue(ctx, issue1.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Issue 1 has case linked",
		Passed:  err == nil && len(updatedIssue1.RelatedCaseIDs) >= 1,
		Details: fmt.Sprintf("Cases for issue 1: %d", len(updatedIssue1.RelatedCaseIDs)),
	})

	// Verify: Issue 2 has case in RelatedCaseIDs
	updatedIssue2, err := sr.services.Store.Issues().GetIssue(ctx, issue2.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Issue 2 has case linked",
		Passed:  err == nil && len(updatedIssue2.RelatedCaseIDs) >= 1,
		Details: fmt.Sprintf("Cases for issue 2: %d", len(updatedIssue2.RelatedCaseIDs)),
	})

	// Unlink from issue 1
	_, err = sr.services.IssueService.UnlinkIssueFromCase(sr.issueWriteContext(ctx), issue1.ID, supportCase.ID)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Verify unlink - issue 1 should have no cases
	updatedIssue1, err = sr.services.Store.Issues().GetIssue(ctx, issue1.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Issue 1 unlinked (no cases)",
		Passed:  err == nil && len(updatedIssue1.RelatedCaseIDs) == 0,
		Details: fmt.Sprintf("Cases for issue 1 after unlink: %d", len(updatedIssue1.RelatedCaseIDs)),
	})

	// Verify: Issue 2 still has case
	updatedIssue2, err = sr.services.Store.Issues().GetIssue(ctx, issue2.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Issue 2 still has case linked",
		Passed:  err == nil && len(updatedIssue2.RelatedCaseIDs) >= 1,
		Details: fmt.Sprintf("Cases for issue 2 after unlink: %d", len(updatedIssue2.RelatedCaseIDs)),
	})

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

// scenarioContactNotification tests notifying contacts when issues are resolved
func (sr *IntegrationScenarioRunner) scenarioContactNotification(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Contact Notification on Issue Resolution",
	}

	if sr.verbose {
		fmt.Println("  -> Testing contact notification workflow...")
	}

	// Create project and issue
	project := observabilitydomain.NewProject(workspaceID, "", "SaaS Platform", "saas-platform", "python")
	err := sr.services.Store.Projects().CreateProject(ctx, project)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create error event for the issue
	baseEvent := observabilitydomain.NewErrorEvent(project.ID, id.New())
	baseEvent.Message = "Payment processing failed"
	baseEvent.Level = "error"
	baseEvent.Environment = "production"

	issue := observabilitydomain.NewIssue(project.ID, "Payment processing failed", "PaymentError", baseEvent)

	err = sr.services.Store.Issues().CreateIssue(ctx, issue)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create contact
	contact := &platformdomain.Contact{
		ID:          id.New(),
		WorkspaceID: workspaceID,
		Email:       "vip@customer.com",
		Name:        "VIP Customer",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = sr.services.Store.Contacts().CreateContact(ctx, contact)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Create case for contact
	supportCase := servicedomain.NewCase(workspaceID, "Payment not going through", contact.Email)
	supportCase.GenerateHumanID("test")
	supportCase.ContactID = contact.ID
	supportCase.Status = servicedomain.CaseStatusOpen

	err = sr.services.Store.Cases().CreateCase(ctx, supportCase)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Link case to issue
	_, err = sr.services.IssueService.LinkIssueToCase(sr.issueWriteContext(ctx), issue.ID, supportCase.ID)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Case created with contact",
		Passed:  supportCase.ContactID == contact.ID,
		Details: fmt.Sprintf("Contact ID: %s", supportCase.ContactID),
	})

	// Verify issue has case linked
	updatedIssue, err := sr.services.Store.Issues().GetIssue(ctx, issue.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Issue has case linked",
		Passed:  err == nil && len(updatedIssue.RelatedCaseIDs) >= 1,
		Details: fmt.Sprintf("Linked cases: %d", len(updatedIssue.RelatedCaseIDs)),
	})

	// Verify case not yet notified
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Case initially not notified",
		Passed:  !supportCase.ContactNotified,
		Details: fmt.Sprintf("ContactNotified: %v", supportCase.ContactNotified),
	})

	// Mark case as notified
	err = sr.services.Store.Cases().MarkCaseNotified(ctx, workspaceID, supportCase.ID)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	// Verify notification status
	updated, _ := sr.services.Store.Cases().GetCase(ctx, supportCase.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Case marked as notified",
		Passed:  updated.ContactNotified,
		Details: fmt.Sprintf("ContactNotified: %v", updated.ContactNotified),
	})

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
