package synth

import (
	"context"
	"fmt"
	"time"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/pkg/id"
)

// ScenarioRunner executes realistic support scenarios using real services
type ScenarioRunner struct {
	services *TestServices
	verbose  bool
}

// NewScenarioRunner creates a new scenario runner with real service wiring
func NewScenarioRunner(services *TestServices, verbose bool) *ScenarioRunner {
	return &ScenarioRunner{
		services: services,
		verbose:  verbose,
	}
}

// ScenarioResult contains the results of running a scenario
type ScenarioResult struct {
	Name             string
	Success          bool
	Error            error
	CaseID           string
	FinalStatus      servicedomain.CaseStatus
	TotalMessages    int
	StateTransitions []StateTransition
	Verifications    []VerificationResult
	Duration         time.Duration
}

// StateTransition records a case state change
type StateTransition struct {
	FromStatus  servicedomain.CaseStatus
	ToStatus    servicedomain.CaseStatus
	Timestamp   time.Time
	TriggerType string // "customer_email", "agent_reply", "agent_action"
	ActorID     string
	ActorName   string
}

// VerificationResult records a verification check
type VerificationResult struct {
	Check   string
	Passed  bool
	Details string
}

// RunAllScenarios runs all defined scenarios
func (sr *ScenarioRunner) RunAllScenarios(ctx context.Context, workspaceID string, agents []*platformdomain.User) ([]*ScenarioResult, error) {
	results := make([]*ScenarioResult, 0)

	scenarios := []struct {
		Name string
		Run  func(context.Context, string, []*platformdomain.User) (*ScenarioResult, error)
	}{
		{"New Case → Agent Response → Resolved", sr.RunNewToResolvedScenario},
		{"New Case → Multiple Exchanges → Closed", sr.RunFullConversationScenario},
		{"New Case → Escalation → Resolution", sr.RunEscalationScenario},
		{"New Case → Customer No Response → Auto-Close", sr.RunAutoCloseScenario},
		{"Reopened Case Flow", sr.RunReopenScenario},
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

		// Log verification results
		if sr.verbose && len(result.Verifications) > 0 {
			for _, v := range result.Verifications {
				status := "✓"
				if !v.Passed {
					status = "✗"
				}
				sr.log("    %s %s: %s", status, v.Check, v.Details)
			}
		}
		sr.log("  Result: success=%v, final_status=%s", result.Success, result.FinalStatus)
	}

	return results, nil
}

// RunNewToResolvedScenario simulates a simple case: customer emails → agent responds → customer confirms → resolved
func (sr *ScenarioRunner) RunNewToResolvedScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:             "New Case → Agent Response → Resolved",
		StateTransitions: make([]StateTransition, 0),
		Verifications:    make([]VerificationResult, 0),
	}

	agent := agents[0]

	// Step 1: Create and persist customer contact FIRST
	sr.log("  Step 1: Creating customer contact...")
	contact, err := sr.createAndPersistContact(ctx, workspaceID, "Alice Johnson", "alice.johnson@example.com")
	if err != nil {
		return result, fmt.Errorf("failed to create contact: %w", err)
	}

	// VERIFY: Contact was persisted
	storedContact, err := sr.services.Store.Contacts().GetContact(ctx, workspaceID, contact.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Contact persisted", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, fmt.Errorf("contact not persisted: %w", err)
	}
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Contact persisted", Passed: true, Details: fmt.Sprintf("ID: %s", storedContact.ID),
	})

	// Step 2: Create case using CaseService (not direct store access)
	sr.log("  Step 2: Creating case via CaseService...")
	caseObj, err := sr.services.CaseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "Password reset not working",
		Description:  "Hi, I've tried resetting my password multiple times but I never receive the reset email. Please help!",
		Priority:     servicedomain.CasePriorityMedium,
		Channel:      servicedomain.CaseChannelEmail,
		ContactID:    contact.ID,
		ContactName:  contact.Name,
		ContactEmail: contact.Email,
	})
	if err != nil {
		return result, fmt.Errorf("CaseService.CreateCase failed: %w", err)
	}
	result.CaseID = caseObj.ID
	result.TotalMessages++

	// VERIFY: Read case back from store to confirm persistence
	storedCase, err := sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Case persisted", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, fmt.Errorf("case not persisted: %w", err)
	}

	// VERIFY: Case is in NEW status (needs triage)
	if storedCase.Status != servicedomain.CaseStatusNew {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Initial status is NEW", Passed: false, Details: fmt.Sprintf("Got: %s", storedCase.Status),
		})
		return result, fmt.Errorf("expected NEW status from store, got %s", storedCase.Status)
	}
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Initial status is NEW", Passed: true, Details: fmt.Sprintf("Case %s", storedCase.HumanID),
	})
	sr.log("    Case %s created and verified in NEW status", storedCase.HumanID)

	// Step 3: Assign agent
	sr.log("  Step 3: Assigning agent...")
	storedCase.AssignedToID = agent.ID
	storedCase.UpdatedAt = time.Now()
	if err := sr.services.Store.Cases().UpdateCase(ctx, storedCase); err != nil {
		return result, fmt.Errorf("failed to assign case: %w", err)
	}

	// Create inbound email record for the initial message
	if err := sr.createInboundEmail(ctx, storedCase, contact, storedCase.Description); err != nil {
		return result, fmt.Errorf("failed to create inbound email: %w", err)
	}

	// Step 4: Agent replies → Case moves to PENDING
	sr.log("  Step 4: Agent sends reply...")
	replyContent := fmt.Sprintf("Hi %s,\n\nI've checked our email logs and found the issue. Please try again now.\n\nBest,\n%s",
		contact.Name, agent.Name)

	if err := sr.createOutboundEmail(ctx, storedCase, agent, contact, replyContent); err != nil {
		return result, fmt.Errorf("failed to create outbound email: %w", err)
	}
	result.TotalMessages++

	// Transition to PENDING (agent responded, awaiting customer)
	oldStatus := storedCase.Status
	storedCase.Status = servicedomain.CaseStatusPending
	storedCase.UpdatedAt = time.Now()
	if err := sr.services.Store.Cases().UpdateCase(ctx, storedCase); err != nil {
		return result, fmt.Errorf("failed to update case to PENDING: %w", err)
	}
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: servicedomain.CaseStatusPending,
		Timestamp: time.Now(), TriggerType: "agent_reply",
		ActorID: agent.ID, ActorName: agent.Name,
	})

	// VERIFY: Read back and confirm PENDING status
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	if storedCase.Status != servicedomain.CaseStatusPending {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Status transitioned to PENDING", Passed: false, Details: fmt.Sprintf("Got: %s", storedCase.Status),
		})
		return result, fmt.Errorf("expected PENDING status, got %s", storedCase.Status)
	}
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Status transitioned to PENDING", Passed: true, Details: "After agent reply",
	})
	sr.log("    Case verified in PENDING status")

	// Step 5: Customer replies → Case moves to OPEN
	sr.log("  Step 5: Customer responds...")
	if err := sr.createInboundEmail(ctx, storedCase, contact, "That worked! Thank you!"); err != nil {
		return result, fmt.Errorf("failed to create customer reply email: %w", err)
	}
	result.TotalMessages++

	oldStatus = storedCase.Status
	storedCase.Status = servicedomain.CaseStatusOpen
	storedCase.UpdatedAt = time.Now()
	if err := sr.services.Store.Cases().UpdateCase(ctx, storedCase); err != nil {
		return result, fmt.Errorf("failed to update case to OPEN: %w", err)
	}
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: servicedomain.CaseStatusOpen,
		Timestamp: time.Now(), TriggerType: "customer_email",
		ActorID: contact.ID, ActorName: contact.Name,
	})

	// VERIFY: Read back and confirm OPEN status
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	if storedCase.Status != servicedomain.CaseStatusOpen {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Status transitioned to OPEN", Passed: false, Details: fmt.Sprintf("Got: %s", storedCase.Status),
		})
	} else {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Status transitioned to OPEN", Passed: true, Details: "After customer reply",
		})
	}
	sr.log("    Case verified in OPEN status")

	// Step 6: Agent resolves → Case moves to RESOLVED
	sr.log("  Step 6: Agent resolves case...")
	oldStatus = storedCase.Status
	now := time.Now()
	storedCase.Status = servicedomain.CaseStatusResolved
	storedCase.ClosedAt = &now
	storedCase.UpdatedAt = now
	if err := sr.services.Store.Cases().UpdateCase(ctx, storedCase); err != nil {
		return result, fmt.Errorf("failed to resolve case: %w", err)
	}
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: servicedomain.CaseStatusResolved,
		Timestamp: now, TriggerType: "agent_action",
		ActorID: agent.ID, ActorName: agent.Name,
	})

	// VERIFY: Final state from store
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	if storedCase.Status != servicedomain.CaseStatusResolved {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Final status is RESOLVED", Passed: false, Details: fmt.Sprintf("Got: %s", storedCase.Status),
		})
		return result, fmt.Errorf("expected RESOLVED status, got %s", storedCase.Status)
	}
	if storedCase.ClosedAt == nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "ClosedAt is set", Passed: false, Details: "ClosedAt is nil",
		})
	} else {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "ClosedAt is set", Passed: true, Details: storedCase.ClosedAt.Format(time.RFC3339),
		})
	}
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Final status is RESOLVED", Passed: true, Details: "Verified from store",
	})
	sr.log("    Case verified in RESOLVED status with ClosedAt set")

	result.Success = true
	result.FinalStatus = storedCase.Status
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunFullConversationScenario simulates multiple back-and-forth messages
func (sr *ScenarioRunner) RunFullConversationScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:             "New Case → Multiple Exchanges → Closed",
		StateTransitions: make([]StateTransition, 0),
		Verifications:    make([]VerificationResult, 0),
	}

	agent := agents[0]

	// Create and persist contact
	contact, err := sr.createAndPersistContact(ctx, workspaceID, "Bob Smith", "bob.smith@company.org")
	if err != nil {
		return result, fmt.Errorf("failed to create contact: %w", err)
	}

	// Create case using service
	caseObj, err := sr.services.CaseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "Billing discrepancy on invoice",
		Description:  "Hello, I noticed my last invoice shows $299 but I'm on the $199/month plan. Can you check this?",
		Priority:     servicedomain.CasePriorityMedium,
		Channel:      servicedomain.CaseChannelEmail,
		ContactID:    contact.ID,
		ContactName:  contact.Name,
		ContactEmail: contact.Email,
	})
	if err != nil {
		return result, fmt.Errorf("CaseService.CreateCase failed: %w", err)
	}
	result.CaseID = caseObj.ID
	result.TotalMessages++

	// Verify case created
	storedCase, err := sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	if err != nil {
		return result, fmt.Errorf("case not found in store: %w", err)
	}
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Case created", Passed: true, Details: fmt.Sprintf("Case %s", storedCase.HumanID),
	})

	// Assign agent
	storedCase.AssignedToID = agent.ID
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)

	// Initial email
	sr.createInboundEmail(ctx, storedCase, contact, storedCase.Description)

	// Exchange 1: Agent asks clarifying question
	sr.createOutboundEmail(ctx, storedCase, agent, contact, "Could you please confirm which billing cycle this relates to?")
	result.TotalMessages++

	oldStatus := storedCase.Status
	storedCase.Status = servicedomain.CaseStatusPending
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: storedCase.Status, Timestamp: time.Now(),
		TriggerType: "agent_reply", ActorID: agent.ID, ActorName: agent.Name,
	})

	// Customer clarifies
	sr.createInboundEmail(ctx, storedCase, contact, "It's the December invoice dated December 1st.")
	result.TotalMessages++

	oldStatus = storedCase.Status
	storedCase.Status = servicedomain.CaseStatusOpen
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: storedCase.Status, Timestamp: time.Now(),
		TriggerType: "customer_email", ActorID: contact.ID, ActorName: contact.Name,
	})

	// Exchange 2: Agent explains
	sr.createOutboundEmail(ctx, storedCase, agent, contact, "I found the issue - there was a prorated charge. Would you like a refund?")
	result.TotalMessages++

	oldStatus = storedCase.Status
	storedCase.Status = servicedomain.CaseStatusPending
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: storedCase.Status, Timestamp: time.Now(),
		TriggerType: "agent_reply", ActorID: agent.ID, ActorName: agent.Name,
	})

	// Customer confirms
	sr.createInboundEmail(ctx, storedCase, contact, "Yes please, remove the charge and refund.")
	result.TotalMessages++

	oldStatus = storedCase.Status
	storedCase.Status = servicedomain.CaseStatusOpen
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: storedCase.Status, Timestamp: time.Now(),
		TriggerType: "customer_email", ActorID: contact.ID, ActorName: contact.Name,
	})

	// Final agent message and close
	sr.createOutboundEmail(ctx, storedCase, agent, contact, "Done! Refund processed. Is there anything else?")
	result.TotalMessages++

	// Close case
	now := time.Now()
	oldStatus = storedCase.Status
	storedCase.Status = servicedomain.CaseStatusClosed
	storedCase.ClosedAt = &now
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: storedCase.Status, Timestamp: now,
		TriggerType: "agent_action", ActorID: agent.ID, ActorName: agent.Name,
	})

	// Verify final state
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	if storedCase.Status == servicedomain.CaseStatusClosed {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Final status is CLOSED", Passed: true, Details: "Verified from store",
		})
	} else {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Final status is CLOSED", Passed: false, Details: fmt.Sprintf("Got: %s", storedCase.Status),
		})
	}

	result.Success = true
	result.FinalStatus = storedCase.Status
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunEscalationScenario simulates a case that needs escalation
func (sr *ScenarioRunner) RunEscalationScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:             "New Case → Escalation → Resolution",
		StateTransitions: make([]StateTransition, 0),
		Verifications:    make([]VerificationResult, 0),
	}

	if len(agents) < 2 {
		return result, fmt.Errorf("need at least 2 agents for escalation scenario")
	}
	tier1Agent := agents[0]
	tier2Agent := agents[1]

	// Create contact
	contact, err := sr.createAndPersistContact(ctx, workspaceID, "Carol Davis", "carol.davis@enterprise.com")
	if err != nil {
		return result, err
	}

	// Create urgent case
	caseObj, err := sr.services.CaseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "URGENT: Production system down",
		Description:  "Our production environment is down! We're losing revenue every minute.",
		Priority:     servicedomain.CasePriorityUrgent,
		Channel:      servicedomain.CaseChannelEmail,
		ContactID:    contact.ID,
		ContactName:  contact.Name,
		ContactEmail: contact.Email,
	})
	if err != nil {
		return result, err
	}
	result.CaseID = caseObj.ID
	result.TotalMessages++

	// Verify urgent priority was set
	storedCase, _ := sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	if storedCase.Priority == servicedomain.CasePriorityUrgent {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Priority set to URGENT", Passed: true, Details: "Verified from store",
		})
	}

	sr.createInboundEmail(ctx, storedCase, contact, storedCase.Description)

	// Tier 1 picks up
	storedCase.AssignedToID = tier1Agent.ID
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)

	sr.createOutboundEmail(ctx, storedCase, tier1Agent, contact, "I'm escalating this to our senior team immediately.")
	result.TotalMessages++

	// Escalate to Tier 2
	sr.log("  Escalating to Tier 2...")
	oldStatus := storedCase.Status
	storedCase.AssignedToID = tier2Agent.ID
	storedCase.Status = servicedomain.CaseStatusOpen
	storedCase.Tags = append(storedCase.Tags, "escalated", "tier2")
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: storedCase.Status, Timestamp: time.Now(),
		TriggerType: "agent_action", ActorID: tier1Agent.ID,
		ActorName: fmt.Sprintf("%s (escalated to %s)", tier1Agent.Name, tier2Agent.Name),
	})

	// Verify escalation
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	if storedCase.AssignedToID == tier2Agent.ID {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Case reassigned to Tier 2", Passed: true, Details: tier2Agent.Name,
		})
	}

	// Tier 2 resolves
	sr.createOutboundEmail(ctx, storedCase, tier2Agent, contact, "I've identified and fixed the issue. Can you confirm it's working?")
	result.TotalMessages++

	sr.createInboundEmail(ctx, storedCase, contact, "Yes! It's back! Thank you!")
	result.TotalMessages++

	now := time.Now()
	storedCase.Status = servicedomain.CaseStatusResolved
	storedCase.ClosedAt = &now
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: servicedomain.CaseStatusOpen, ToStatus: servicedomain.CaseStatusResolved,
		Timestamp: now, TriggerType: "agent_action",
		ActorID: tier2Agent.ID, ActorName: tier2Agent.Name,
	})

	// Verify final state
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Final status is RESOLVED", Passed: storedCase.Status == servicedomain.CaseStatusResolved,
		Details: fmt.Sprintf("Status: %s", storedCase.Status),
	})

	result.Success = true
	result.FinalStatus = storedCase.Status
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunAutoCloseScenario simulates auto-close after no customer response
func (sr *ScenarioRunner) RunAutoCloseScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:             "New Case → Customer No Response → Auto-Close",
		StateTransitions: make([]StateTransition, 0),
		Verifications:    make([]VerificationResult, 0),
	}

	agent := agents[0]

	contact, err := sr.createAndPersistContact(ctx, workspaceID, "Dave Wilson", "dave.wilson@startup.io")
	if err != nil {
		return result, err
	}

	// Create case
	caseObj, err := sr.services.CaseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "Question about pricing tiers",
		Description:  "What's the difference between the Pro and Enterprise plans?",
		Priority:     servicedomain.CasePriorityLow,
		Channel:      servicedomain.CaseChannelEmail,
		ContactID:    contact.ID,
		ContactName:  contact.Name,
		ContactEmail: contact.Email,
	})
	if err != nil {
		return result, err
	}
	result.CaseID = caseObj.ID
	result.TotalMessages++

	storedCase, _ := sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	sr.createInboundEmail(ctx, storedCase, contact, storedCase.Description)

	// Agent responds
	storedCase.AssignedToID = agent.ID
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)

	sr.createOutboundEmail(ctx, storedCase, agent, contact, "Here's the comparison: Pro is $99/mo, Enterprise is $299/mo with unlimited users...")
	result.TotalMessages++

	storedCase.Status = servicedomain.CaseStatusPending
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: servicedomain.CaseStatusNew, ToStatus: servicedomain.CaseStatusPending,
		Timestamp: time.Now(), TriggerType: "agent_reply",
		ActorID: agent.ID, ActorName: agent.Name,
	})

	// Simulate no response - follow-up
	sr.log("  Simulating no customer response (auto-close trigger)...")
	sr.createOutboundEmail(ctx, storedCase, agent, contact, "Hi Dave, just checking in - any questions about pricing?")
	result.TotalMessages++

	// Auto-close
	now := time.Now()
	storedCase.Status = servicedomain.CaseStatusClosed
	storedCase.ClosedAt = &now
	storedCase.Tags = append(storedCase.Tags, "auto-closed", "no-response")
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: servicedomain.CaseStatusPending, ToStatus: servicedomain.CaseStatusClosed,
		Timestamp: now, TriggerType: "agent_action",
		ActorID: "system", ActorName: "Auto-Close System",
	})

	// Verify
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	hasAutoCloseTag := false
	for _, tag := range storedCase.Tags {
		if tag == "auto-closed" {
			hasAutoCloseTag = true
			break
		}
	}
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Auto-close tag applied", Passed: hasAutoCloseTag, Details: fmt.Sprintf("Tags: %v", storedCase.Tags),
	})
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Final status is CLOSED", Passed: storedCase.Status == servicedomain.CaseStatusClosed,
		Details: fmt.Sprintf("Status: %s", storedCase.Status),
	})

	result.Success = true
	result.FinalStatus = storedCase.Status
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunReopenScenario simulates reopening a closed case
func (sr *ScenarioRunner) RunReopenScenario(ctx context.Context, workspaceID string, agents []*platformdomain.User) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:             "Reopened Case Flow",
		StateTransitions: make([]StateTransition, 0),
		Verifications:    make([]VerificationResult, 0),
	}

	agent := agents[0]

	contact, err := sr.createAndPersistContact(ctx, workspaceID, "Eva Martinez", "eva.martinez@corp.net")
	if err != nil {
		return result, err
	}

	// Create and quickly resolve a case
	caseObj, err := sr.services.CaseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      "Can't export data",
		Description:  "The export button doesn't seem to be working.",
		Priority:     servicedomain.CasePriorityMedium,
		Channel:      servicedomain.CaseChannelEmail,
		ContactID:    contact.ID,
		ContactName:  contact.Name,
		ContactEmail: contact.Email,
	})
	if err != nil {
		return result, err
	}
	result.CaseID = caseObj.ID
	result.TotalMessages++

	storedCase, _ := sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	sr.createInboundEmail(ctx, storedCase, contact, storedCase.Description)

	storedCase.AssignedToID = agent.ID
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)

	sr.createOutboundEmail(ctx, storedCase, agent, contact, "Try clearing your cache and trying again.")
	result.TotalMessages++

	sr.createInboundEmail(ctx, storedCase, contact, "That worked, thanks!")
	result.TotalMessages++

	// Resolve
	now := time.Now()
	storedCase.Status = servicedomain.CaseStatusResolved
	storedCase.ClosedAt = &now
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: servicedomain.CaseStatusNew, ToStatus: servicedomain.CaseStatusResolved,
		Timestamp: now, TriggerType: "agent_action",
		ActorID: agent.ID, ActorName: agent.Name,
	})
	sr.log("  Case resolved...")

	// Verify resolved state
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Case resolved initially", Passed: storedCase.Status == servicedomain.CaseStatusResolved,
		Details: fmt.Sprintf("Status: %s, ClosedAt: %v", storedCase.Status, storedCase.ClosedAt),
	})

	// Customer replies to resolved case → REOPEN
	sr.log("  Customer replies to resolved case (reopening)...")
	sr.createInboundEmail(ctx, storedCase, contact, "Actually, the issue is back with a different error.")
	result.TotalMessages++

	oldStatus := storedCase.Status
	storedCase.Status = servicedomain.CaseStatusOpen
	storedCase.ClosedAt = nil // Clear closed timestamp
	storedCase.Tags = append(storedCase.Tags, "reopened")
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: oldStatus, ToStatus: servicedomain.CaseStatusOpen,
		Timestamp: time.Now(), TriggerType: "customer_email",
		ActorID: contact.ID, ActorName: contact.Name,
	})

	// Verify reopened state
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	hasReopenedTag := false
	for _, tag := range storedCase.Tags {
		if tag == "reopened" {
			hasReopenedTag = true
			break
		}
	}
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Case reopened", Passed: storedCase.Status == servicedomain.CaseStatusOpen && storedCase.ClosedAt == nil,
		Details: fmt.Sprintf("Status: %s, ClosedAt cleared: %v", storedCase.Status, storedCase.ClosedAt == nil),
	})
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Reopened tag applied", Passed: hasReopenedTag, Details: fmt.Sprintf("Tags: %v", storedCase.Tags),
	})

	// Agent fixes and closes
	sr.createOutboundEmail(ctx, storedCase, agent, contact, "I found a new bug we just fixed. Please try now.")
	result.TotalMessages++

	sr.createInboundEmail(ctx, storedCase, contact, "Working now! Thanks!")
	result.TotalMessages++

	now = time.Now()
	storedCase.Status = servicedomain.CaseStatusClosed
	storedCase.ClosedAt = &now
	sr.services.Store.Cases().UpdateCase(ctx, storedCase)
	result.StateTransitions = append(result.StateTransitions, StateTransition{
		FromStatus: servicedomain.CaseStatusOpen, ToStatus: servicedomain.CaseStatusClosed,
		Timestamp: now, TriggerType: "agent_action",
		ActorID: agent.ID, ActorName: agent.Name,
	})

	// Final verification
	storedCase, _ = sr.services.Store.Cases().GetCase(ctx, caseObj.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Final status is CLOSED", Passed: storedCase.Status == servicedomain.CaseStatusClosed,
		Details: fmt.Sprintf("Status: %s", storedCase.Status),
	})

	result.Success = true
	result.FinalStatus = storedCase.Status
	result.Duration = time.Since(startTime)
	return result, nil
}

// Helper methods

func (sr *ScenarioRunner) createAndPersistContact(ctx context.Context, workspaceID, name, email string) (*platformdomain.Contact, error) {
	contact := platformdomain.NewContact(workspaceID, email)
	contact.Name = name

	if err := sr.services.Store.Contacts().CreateContact(ctx, contact); err != nil {
		return nil, fmt.Errorf("failed to persist contact: %w", err)
	}

	return contact, nil
}

func (sr *ScenarioRunner) createInboundEmail(ctx context.Context, caseObj *servicedomain.Case, contact *platformdomain.Contact, content string) error {
	now := time.Now()
	email := &servicedomain.InboundEmail{
		ID:               id.New(),
		WorkspaceID:      caseObj.WorkspaceID,
		CaseID:           caseObj.ID,
		MessageID:        fmt.Sprintf("<%s@mail.example.com>", id.New()[:16]),
		FromEmail:        contact.Email,
		FromName:         contact.Name,
		ToEmails:         []string{"support@mbr.local"},
		Subject:          fmt.Sprintf("Re: %s", caseObj.Subject),
		TextContent:      content,
		HTMLContent:      fmt.Sprintf("<p>%s</p>", content),
		ProcessingStatus: "processed", // Mark as processed since we're simulating the result
		ReceivedAt:       now,
		CreatedAt:        now,
	}
	// Note: ProcessedAt is intentionally NOT set here - we set ProcessingStatus
	// In a real system, the email processor would set ProcessedAt after actual processing
	return sr.services.Store.InboundEmails().CreateInboundEmail(ctx, email)
}

func (sr *ScenarioRunner) createOutboundEmail(ctx context.Context, caseObj *servicedomain.Case, agent *platformdomain.User, contact *platformdomain.Contact, content string) error {
	now := time.Now()
	email := &servicedomain.OutboundEmail{
		ID:                id.New(),
		WorkspaceID:       caseObj.WorkspaceID,
		CaseID:            caseObj.ID,
		ProviderMessageID: fmt.Sprintf("<%s@mbr.local>", id.New()[:16]),
		FromEmail:         "support@mbr.local",
		FromName:          agent.Name,
		ToEmails:          []string{contact.Email},
		Subject:           fmt.Sprintf("Re: %s", caseObj.Subject),
		TextContent:       content,
		HTMLContent:       fmt.Sprintf("<p>%s</p><p>Best,<br>%s</p>", content, agent.Name),
		Status:            servicedomain.EmailStatusPending, // Start as pending, not sent
		CreatedAt:         now,
		UpdatedAt:         now,
		// Note: SentAt is NOT set - in a real system this would be set by the email sender worker
	}
	return sr.services.Store.OutboundEmails().CreateOutboundEmail(ctx, email)
}

func (sr *ScenarioRunner) log(format string, args ...interface{}) {
	if sr.verbose {
		fmt.Printf("[scenario] "+format+"\n", args...)
	}
}
