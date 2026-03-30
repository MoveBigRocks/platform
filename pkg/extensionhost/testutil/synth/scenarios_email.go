package synth

import (
	"context"
	"fmt"
	"time"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// EmailScenarioRunner runs email processing scenarios
type EmailScenarioRunner struct {
	services *TestServices
	verbose  bool
}

// NewEmailScenarioRunner creates a new email scenario runner
func NewEmailScenarioRunner(services *TestServices, verbose bool) *EmailScenarioRunner {
	return &EmailScenarioRunner{
		services: services,
		verbose:  verbose,
	}
}

// RunAllEmailScenarios runs all email processing scenarios
func (sr *EmailScenarioRunner) RunAllEmailScenarios(ctx context.Context, workspaceID string, users []*platformdomain.User) ([]*ScenarioResult, error) {
	scenarios := []func(context.Context, string, []*platformdomain.User) (*ScenarioResult, error){
		sr.scenarioInboundEmailProcessing,
		sr.scenarioOutboundEmailSending,
		sr.scenarioEmailThreading,
		sr.scenarioEmailBlacklist,
		sr.scenarioEmailTemplates,
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

// scenarioInboundEmailProcessing tests processing incoming emails
func (sr *EmailScenarioRunner) scenarioInboundEmailProcessing(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Inbound Email Processing",
	}

	if sr.verbose {
		fmt.Println("  -> Testing inbound email processing...")
	}

	// Create an inbound email
	messageID := fmt.Sprintf("<%s@mail.example.com>", id.New())
	email := servicedomain.NewInboundEmail(workspaceID, messageID, "customer@example.com", "Need help with my order", "I placed an order yesterday but haven't received confirmation.")
	email.FromName = "John Customer"
	email.ToEmails = []string{"support@workspace.movebigrocks.com"}
	email.HTMLContent = "<p>I placed an order yesterday but haven't received confirmation.</p>"
	email.Headers = map[string]string{
		"Message-ID": messageID,
		"Date":       time.Now().Format(time.RFC1123Z),
	}

	err := sr.services.Store.InboundEmails().CreateInboundEmail(ctx, email)
	if err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Inbound email created",
		Passed:  email.ID != "",
		Details: fmt.Sprintf("Email ID: %s", email.ID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Initial status is pending",
		Passed:  email.ProcessingStatus == "pending",
		Details: fmt.Sprintf("Status: %s", email.ProcessingStatus),
	})

	// Process the email (simulate)
	email.ProcessingStatus = "processed"
	now := time.Now()
	email.ProcessedAt = &now
	email.SpamScore = 0.1
	email.IsSpam = false
	email.IsThreadStart = true

	// Create a case from the email
	newCase := servicedomain.NewCase(workspaceID, email.Subject, email.FromEmail)
	newCase.GenerateHumanID("test")
	newCase.Channel = "email"
	newCase.Description = email.TextContent

	err = sr.services.Store.Cases().CreateCase(ctx, newCase)
	if err != nil {
		return failScenario(result, start, err)
	}

	email.CaseID = newCase.ID
	err = sr.services.Store.InboundEmails().UpdateInboundEmail(ctx, email)
	if err != nil {
		return failScenario(result, start, err)
	}

	retrieved, _ := sr.services.Store.InboundEmails().GetInboundEmail(ctx, email.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Email processed successfully",
		Passed:  retrieved.ProcessingStatus == "processed",
		Details: fmt.Sprintf("Status: %s", retrieved.ProcessingStatus),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Email linked to case",
		Passed:  retrieved.CaseID == newCase.ID,
		Details: fmt.Sprintf("Case ID: %s", retrieved.CaseID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Not marked as spam",
		Passed:  !retrieved.IsSpam,
		Details: fmt.Sprintf("SpamScore: %.2f", retrieved.SpamScore),
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

// scenarioOutboundEmailSending tests sending outbound emails
func (sr *EmailScenarioRunner) scenarioOutboundEmailSending(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Outbound Email Sending",
	}

	if sr.verbose {
		fmt.Println("  -> Testing outbound email sending...")
	}

	// Create an outbound email
	email := servicedomain.NewOutboundEmail(workspaceID, "support@workspace.movebigrocks.com", "Re: Your support request", "Thank you for contacting us. We're looking into your issue.")
	email.ToEmails = []string{"customer@example.com"}
	email.FromName = "Support Team"
	email.HTMLContent = "<p>Thank you for contacting us. We're looking into your issue.</p>"
	email.Category = "support"

	if len(users) > 0 {
		email.CreatedByID = users[0].ID
	}

	err := sr.services.Store.OutboundEmails().CreateOutboundEmail(ctx, email)
	if err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Outbound email created",
		Passed:  email.ID != "",
		Details: fmt.Sprintf("Email ID: %s", email.ID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Initial status is pending",
		Passed:  email.Status == servicedomain.EmailStatusPending,
		Details: fmt.Sprintf("Status: %s", email.Status),
	})

	// Simulate sending
	email.Status = servicedomain.EmailStatusSending
	err = sr.services.Store.OutboundEmails().UpdateOutboundEmail(ctx, email)
	if err != nil {
		return failScenario(result, start, err)
	}

	// Simulate sent
	email.Status = servicedomain.EmailStatusSent
	now := time.Now()
	email.SentAt = &now
	email.ProviderMessageID = "msg_" + id.New()
	err = sr.services.Store.OutboundEmails().UpdateOutboundEmail(ctx, email)
	if err != nil {
		return failScenario(result, start, err)
	}

	retrieved, _ := sr.services.Store.OutboundEmails().GetOutboundEmail(ctx, email.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Email marked as sent",
		Passed:  retrieved.Status == servicedomain.EmailStatusSent,
		Details: fmt.Sprintf("Status: %s", retrieved.Status),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Sent timestamp recorded",
		Passed:  retrieved.SentAt != nil,
		Details: fmt.Sprintf("SentAt: %v", retrieved.SentAt),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Provider message ID recorded",
		Passed:  retrieved.ProviderMessageID != "",
		Details: fmt.Sprintf("Provider ID: %s", retrieved.ProviderMessageID),
	})

	// Simulate delivery confirmation
	email.Status = servicedomain.EmailStatusDelivered
	deliveredAt := time.Now()
	email.DeliveredAt = &deliveredAt
	err = sr.services.Store.OutboundEmails().UpdateOutboundEmail(ctx, email)
	if err != nil {
		return failScenario(result, start, err)
	}

	retrieved, _ = sr.services.Store.OutboundEmails().GetOutboundEmail(ctx, email.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Email marked as delivered",
		Passed:  retrieved.Status == servicedomain.EmailStatusDelivered,
		Details: fmt.Sprintf("Status: %s", retrieved.Status),
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

// scenarioEmailThreading tests email thread detection and management
func (sr *EmailScenarioRunner) scenarioEmailThreading(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Email Threading",
	}

	if sr.verbose {
		fmt.Println("  -> Testing email threading...")
	}

	// Create email thread first
	now := time.Now()
	thread := &servicedomain.EmailThread{
		ID:          id.New(),
		WorkspaceID: workspaceID,
		Subject:     "Product question",
		Participants: []servicedomain.ThreadParticipant{
			{Email: "customer@example.com", Role: "sender", FirstSeenAt: now, LastSeenAt: now, EmailCount: 1},
			{Email: "support@workspace.movebigrocks.com", Role: "recipient", IsInternal: true, FirstSeenAt: now, LastSeenAt: now},
		},
		EmailCount: 1,
		Status:     servicedomain.ThreadStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := sr.services.Store.EmailThreads().CreateEmailThread(ctx, thread)
	if err != nil {
		return failScenario(result, start, err)
	}

	// Create initial email (thread start) - with ThreadID set so index is updated
	messageID1 := fmt.Sprintf("<%s@mail.example.com>", id.New())
	email1 := servicedomain.NewInboundEmail(workspaceID, messageID1, "customer@example.com", "Product question", "Is this product available in blue?")
	email1.IsThreadStart = true
	email1.ThreadID = thread.ID

	err = sr.services.Store.InboundEmails().CreateInboundEmail(ctx, email1)
	if err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Thread created",
		Passed:  thread.ID != "",
		Details: fmt.Sprintf("Thread ID: %s", thread.ID),
	})

	// Create reply (references first email)
	messageID2 := fmt.Sprintf("<%s@mail.example.com>", id.New())
	email2 := servicedomain.NewInboundEmail(workspaceID, messageID2, "support@workspace.movebigrocks.com", "Re: Product question", "Yes, the product is available in blue.")
	email2.InReplyTo = messageID1
	email2.References = []string{messageID1}
	email2.ThreadID = thread.ID
	email2.IsThreadStart = false
	email2.PreviousEmailIDs = []string{email1.ID}

	err = sr.services.Store.InboundEmails().CreateInboundEmail(ctx, email2)
	if err != nil {
		return failScenario(result, start, err)
	}

	// Update thread
	thread.EmailCount = 2
	thread.LastActivity = time.Now()
	err = sr.services.Store.EmailThreads().UpdateEmailThread(ctx, thread)
	if err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Reply linked to thread",
		Passed:  email2.ThreadID == thread.ID,
		Details: fmt.Sprintf("Thread ID: %s", email2.ThreadID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Reply references original email",
		Passed:  email2.InReplyTo == messageID1,
		Details: fmt.Sprintf("In-Reply-To: %s", email2.InReplyTo),
	})

	// Get emails in thread
	threadEmails, err := sr.services.Store.InboundEmails().GetEmailsByThread(ctx, thread.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Thread contains both emails",
		Passed:  err == nil && len(threadEmails) >= 2,
		Details: fmt.Sprintf("Emails in thread: %d", len(threadEmails)),
	})

	// Get thread
	retrievedThread, _ := sr.services.Store.EmailThreads().GetEmailThread(ctx, thread.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Thread email count updated",
		Passed:  retrievedThread.EmailCount >= 2,
		Details: fmt.Sprintf("Email count: %d", retrievedThread.EmailCount),
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

// scenarioEmailBlacklist tests email blacklisting
func (sr *EmailScenarioRunner) scenarioEmailBlacklist(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Email Blacklist",
	}

	if sr.verbose {
		fmt.Println("  -> Testing email blacklist...")
	}

	createdByID := "system"
	if len(users) > 0 {
		createdByID = users[0].ID
	}

	// Create email blacklist entry
	blacklist := servicedomain.NewEmailBlacklist(workspaceID, "email", "spammer@badactor.com", "Known spammer", createdByID)
	blacklist.BlockInbound = true
	blacklist.BlockOutbound = false

	err := sr.services.Store.EmailSecurity().CreateEmailBlacklist(ctx, blacklist)
	if err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Blacklist entry created",
		Passed:  blacklist.ID != "",
		Details: fmt.Sprintf("Blacklist ID: %s", blacklist.ID),
	})

	// Create domain blacklist
	domainBlacklist := servicedomain.NewEmailBlacklist(workspaceID, "domain", "spamdomain.com", "Spam domain", createdByID)
	domainBlacklist.BlockInbound = true

	err = sr.services.Store.EmailSecurity().CreateEmailBlacklist(ctx, domainBlacklist)
	if err != nil {
		return failScenario(result, start, err)
	}

	// Check if email is blacklisted
	found, err := sr.services.Store.EmailSecurity().CheckEmailBlacklist(ctx, workspaceID, "spammer@badactor.com", "badactor.com")
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Blacklisted email detected",
		Passed:  err == nil && found != nil,
		Details: fmt.Sprintf("Found: %v", found != nil),
	})

	// Check if domain is blacklisted
	found, err = sr.services.Store.EmailSecurity().CheckEmailBlacklist(ctx, workspaceID, "anyone@spamdomain.com", "spamdomain.com")
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Blacklisted domain detected",
		Passed:  err == nil && found != nil,
		Details: fmt.Sprintf("Found: %v", found != nil),
	})

	// Check non-blacklisted email
	found, _ = sr.services.Store.EmailSecurity().CheckEmailBlacklist(ctx, workspaceID, "legitimate@gooddomain.com", "gooddomain.com")
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Non-blacklisted email passes",
		Passed:  found == nil,
		Details: fmt.Sprintf("Found: %v", found != nil),
	})

	// List blacklist entries
	entries, err := sr.services.Store.EmailSecurity().ListWorkspaceEmailBlacklists(ctx, workspaceID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Blacklist entries listed",
		Passed:  err == nil && len(entries) >= 2,
		Details: fmt.Sprintf("Found %d entries", len(entries)),
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

// scenarioEmailTemplates tests email template management
func (sr *EmailScenarioRunner) scenarioEmailTemplates(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Email Templates",
	}

	if sr.verbose {
		fmt.Println("  -> Testing email templates...")
	}

	createdByID := "system"
	if len(users) > 0 {
		createdByID = users[0].ID
	}

	// Create email template
	template := servicedomain.NewEmailTemplate(workspaceID, "Welcome Email", "Welcome to {{company_name}}!", createdByID)
	template.HTMLContent = "<h1>Welcome, {{customer_name}}!</h1><p>Thank you for joining {{company_name}}.</p>"
	template.TextContent = "Welcome, {{customer_name}}! Thank you for joining {{company_name}}."
	template.Category = "onboarding"
	template.Variables = []servicedomain.EmailTemplateVariable{
		{Name: "customer_name", Type: "string", Required: true, Description: "Customer's name"},
		{Name: "company_name", Type: "string", Required: true, Description: "Company name"},
	}
	template.SampleData = map[string]interface{}{
		"customer_name": "John Doe",
		"company_name":  "Acme Corp",
	}

	err := sr.services.Store.EmailTemplates().CreateEmailTemplate(ctx, template)
	if err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Template created",
		Passed:  template.ID != "",
		Details: fmt.Sprintf("Template ID: %s", template.ID),
	})

	// Retrieve template
	retrieved, err := sr.services.Store.EmailTemplates().GetEmailTemplate(ctx, template.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Template retrieved",
		Passed:  err == nil && retrieved != nil,
		Details: fmt.Sprintf("Name: %s", retrieved.Name),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Template has variables",
		Passed:  len(retrieved.Variables) >= 2,
		Details: fmt.Sprintf("Variables: %d", len(retrieved.Variables)),
	})

	// Update template (increment version)
	template.HTMLContent = "<h1>Welcome, {{customer_name}}!</h1><p>Thank you for joining {{company_name}}. We're excited to have you!</p>"
	template.Version = 2
	err = sr.services.Store.EmailTemplates().UpdateEmailTemplate(ctx, template)
	if err != nil {
		return failScenario(result, start, err)
	}

	retrieved, _ = sr.services.Store.EmailTemplates().GetEmailTemplate(ctx, template.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Template version updated",
		Passed:  retrieved.Version == 2,
		Details: fmt.Sprintf("Version: %d", retrieved.Version),
	})

	// List templates
	templates, err := sr.services.Store.EmailTemplates().ListWorkspaceEmailTemplates(ctx, workspaceID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Templates listed",
		Passed:  err == nil && len(templates) >= 1,
		Details: fmt.Sprintf("Found %d templates", len(templates)),
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
