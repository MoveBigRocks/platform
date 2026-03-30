package synth

import (
	"context"
	"fmt"
	"time"

	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// KnowledgeScenarioRunner exercises the operational knowledge-resource model.
type KnowledgeScenarioRunner struct {
	services *TestServices
	verbose  bool
}

func NewKnowledgeScenarioRunner(services *TestServices, verbose bool) *KnowledgeScenarioRunner {
	return &KnowledgeScenarioRunner{
		services: services,
		verbose:  verbose,
	}
}

func (sr *KnowledgeScenarioRunner) ensureKnowledgeTeam(ctx context.Context, workspaceID string) (string, error) {
	teams, err := sr.services.Store.Workspaces().ListWorkspaceTeams(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if len(teams) > 0 {
		return teams[0].ID, nil
	}
	now := time.Now().UTC()
	team := &platformdomain.Team{
		WorkspaceID: workspaceID,
		Name:        "Knowledge Ops",
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := sr.services.Store.Workspaces().CreateTeam(ctx, team); err != nil {
		return "", err
	}
	return team.ID, nil
}

func (sr *KnowledgeScenarioRunner) RunAllKnowledgeScenarios(ctx context.Context, workspaceID string, users []*platformdomain.User) ([]*ScenarioResult, error) {
	scenarios := []func(context.Context, string, []*platformdomain.User) (*ScenarioResult, error){
		sr.scenarioCreateKnowledgeResource,
		sr.scenarioKnowledgeResourceLifecycle,
		sr.scenarioKnowledgeResourceSearch,
		sr.scenarioCaseKnowledgeLink,
	}

	results := make([]*ScenarioResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		result, err := scenario(ctx, workspaceID, users)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (sr *KnowledgeScenarioRunner) scenarioCreateKnowledgeResource(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Create Knowledge Resource",
	}

	if sr.verbose {
		fmt.Println("  -> Creating knowledge resource...")
	}

	teamID, err := sr.ensureKnowledgeTeam(ctx, workspaceID)
	if err != nil {
		return failScenario(result, start, err)
	}

	resource := knowledgedomain.NewKnowledgeResource(workspaceID, teamID, "help-center-"+id.New()[:8], "Help Center")
	resource.Kind = knowledgedomain.KnowledgeResourceKindGuide
	resource.Status = knowledgedomain.KnowledgeResourceStatusActive
	resource.Summary = "Customer-facing operational guidance"
	resource.BodyMarkdown = "# Help Center\n\nUse Move Big Rocks as the operational source of truth."
	resource.SupportedChannels = []string{
		string(servicedomain.ConversationChannelWebChat),
		string(servicedomain.ConversationChannelOperatorConsole),
	}
	resource.SearchKeywords = []string{"help", "operations"}
	resource.Frontmatter.Set("audience", "customer")
	if len(users) > 0 {
		resource.CreatedBy = users[0].ID
	}

	err = sr.services.Store.KnowledgeResources().CreateKnowledgeResource(ctx, resource)
	if err != nil {
		return failScenario(result, start, err)
	}

	retrieved, err := sr.services.Store.KnowledgeResources().GetKnowledgeResource(ctx, resource.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Knowledge resource created",
		Passed:  err == nil && retrieved != nil,
		Details: fmt.Sprintf("Resource ID: %s", resource.ID),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Kind stored as guide",
		Passed:  retrieved != nil && retrieved.Kind == knowledgedomain.KnowledgeResourceKindGuide,
		Details: fmt.Sprintf("Kind: %s", retrieved.Kind),
	})

	bySlug, err := sr.services.Store.KnowledgeResources().GetKnowledgeResourceBySlug(ctx, workspaceID, teamID, knowledgedomain.KnowledgeSurfacePrivate, resource.Slug)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Retrievable by slug",
		Passed:  err == nil && bySlug != nil && bySlug.ID == resource.ID,
		Details: fmt.Sprintf("Slug: %s", resource.Slug),
	})

	result.Success = allVerificationsPassed(result.Verifications)
	result.Duration = time.Since(start)
	return result, nil
}

func (sr *KnowledgeScenarioRunner) scenarioKnowledgeResourceLifecycle(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Knowledge Resource Lifecycle",
	}

	if sr.verbose {
		fmt.Println("  -> Testing knowledge resource lifecycle...")
	}

	teamID, err := sr.ensureKnowledgeTeam(ctx, workspaceID)
	if err != nil {
		return failScenario(result, start, err)
	}

	resource := knowledgedomain.NewKnowledgeResource(workspaceID, teamID, "incident-playbook-"+id.New()[:8], "Incident Playbook")
	resource.Kind = knowledgedomain.KnowledgeResourceKindPolicy
	resource.Summary = "Escalation policy for production incidents"
	resource.BodyMarkdown = "Draft incident policy"

	if err := sr.services.Store.KnowledgeResources().CreateKnowledgeResource(ctx, resource); err != nil {
		return failScenario(result, start, err)
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Resource starts in draft",
		Passed:  resource.Status == knowledgedomain.KnowledgeResourceStatusDraft,
		Details: fmt.Sprintf("Status: %s", resource.Status),
	})

	resource.Status = knowledgedomain.KnowledgeResourceStatusActive
	resource.BodyMarkdown = "Approved incident policy"
	now := time.Now().UTC()
	resource.ReviewedAt = &now
	resource.UpdatedAt = now
	if err := sr.services.Store.KnowledgeResources().UpdateKnowledgeResource(ctx, resource); err != nil {
		return failScenario(result, start, err)
	}

	retrieved, err := sr.services.Store.KnowledgeResources().GetKnowledgeResource(ctx, resource.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Resource activated",
		Passed:  err == nil && retrieved.Status == knowledgedomain.KnowledgeResourceStatusActive,
		Details: fmt.Sprintf("Status: %s", retrieved.Status),
	})

	retrieved.Status = knowledgedomain.KnowledgeResourceStatusArchived
	retrieved.UpdatedAt = time.Now().UTC()
	if err := sr.services.Store.KnowledgeResources().UpdateKnowledgeResource(ctx, retrieved); err != nil {
		return failScenario(result, start, err)
	}

	retrieved, err = sr.services.Store.KnowledgeResources().GetKnowledgeResource(ctx, resource.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Resource archived",
		Passed:  err == nil && retrieved.Status == knowledgedomain.KnowledgeResourceStatusArchived,
		Details: fmt.Sprintf("Status: %s", retrieved.Status),
	})

	result.Success = allVerificationsPassed(result.Verifications)
	result.Duration = time.Since(start)
	return result, nil
}

func (sr *KnowledgeScenarioRunner) scenarioKnowledgeResourceSearch(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Knowledge Resource Search",
	}

	if sr.verbose {
		fmt.Println("  -> Testing knowledge resource search...")
	}

	teamID, err := sr.ensureKnowledgeTeam(ctx, workspaceID)
	if err != nil {
		return failScenario(result, start, err)
	}

	resources := []*knowledgedomain.KnowledgeResource{
		knowledgedomain.NewKnowledgeResource(workspaceID, teamID, "refund-policy-"+id.New()[:8], "Refund Policy"),
		knowledgedomain.NewKnowledgeResource(workspaceID, teamID, "payment-guide-"+id.New()[:8], "Payment Guide"),
		knowledgedomain.NewKnowledgeResource(workspaceID, teamID, "access-control-"+id.New()[:8], "Access Control"),
	}

	resources[0].Kind = knowledgedomain.KnowledgeResourceKindPolicy
	resources[0].Status = knowledgedomain.KnowledgeResourceStatusActive
	resources[0].Summary = "How refunds are handled"
	resources[0].BodyMarkdown = "Refund requests require operator approval."

	resources[1].Kind = knowledgedomain.KnowledgeResourceKindGuide
	resources[1].Status = knowledgedomain.KnowledgeResourceStatusActive
	resources[1].Summary = "How to update payment methods"
	resources[1].BodyMarkdown = "Payment methods can be updated from billing settings."

	resources[2].Kind = knowledgedomain.KnowledgeResourceKindContext
	resources[2].Status = knowledgedomain.KnowledgeResourceStatusDraft
	resources[2].Summary = "Internal access model"
	resources[2].BodyMarkdown = "Access control is handled through workspace roles."

	for _, resource := range resources {
		if err := sr.services.Store.KnowledgeResources().CreateKnowledgeResource(ctx, resource); err != nil {
			return failScenario(result, start, err)
		}
	}

	activePolicies, totalPolicies, err := sr.services.Store.KnowledgeResources().ListWorkspaceKnowledgeResources(ctx, workspaceID, &knowledgedomain.KnowledgeResourceFilter{
		Kind:   knowledgedomain.KnowledgeResourceKindPolicy,
		Status: knowledgedomain.KnowledgeResourceStatusActive,
	})
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Filter by kind and status",
		Passed:  err == nil && len(activePolicies) == 1 && totalPolicies >= 1,
		Details: fmt.Sprintf("Policy count: %d / total: %d", len(activePolicies), totalPolicies),
	})

	searchResults, totalSearch, err := sr.services.Store.KnowledgeResources().ListWorkspaceKnowledgeResources(ctx, workspaceID, &knowledgedomain.KnowledgeResourceFilter{
		Search: "payment methods",
	})
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Full-text search returns matching guidance",
		Passed:  err == nil && len(searchResults) >= 1 && totalSearch >= 1,
		Details: fmt.Sprintf("Search results: %d / total: %d", len(searchResults), totalSearch),
	})

	result.Success = allVerificationsPassed(result.Verifications)
	result.Duration = time.Since(start)
	return result, nil
}

func (sr *KnowledgeScenarioRunner) scenarioCaseKnowledgeLink(ctx context.Context, workspaceID string, users []*platformdomain.User) (*ScenarioResult, error) {
	start := time.Now()
	result := &ScenarioResult{
		Name: "Case Knowledge Link",
	}

	if sr.verbose {
		fmt.Println("  -> Linking case to knowledge resource...")
	}

	teamID, err := sr.ensureKnowledgeTeam(ctx, workspaceID)
	if err != nil {
		return failScenario(result, start, err)
	}

	resource := knowledgedomain.NewKnowledgeResource(workspaceID, teamID, "reset-password-"+id.New()[:8], "Reset Password")
	resource.Kind = knowledgedomain.KnowledgeResourceKindGuide
	resource.Status = knowledgedomain.KnowledgeResourceStatusActive
	resource.BodyMarkdown = "Send the reset password flow to the contact and verify email deliverability."
	if err := sr.services.Store.KnowledgeResources().CreateKnowledgeResource(ctx, resource); err != nil {
		return failScenario(result, start, err)
	}

	caseObj := servicedomain.NewCase(workspaceID, "Password reset failed", "customer@example.com")
	caseObj.GenerateHumanID("synth")
	caseObj.Description = "Customer reports that the reset email never arrives."
	caseObj.CreatedAt = time.Now().UTC()
	caseObj.UpdatedAt = caseObj.CreatedAt
	if err := sr.services.Store.Cases().CreateCase(ctx, caseObj); err != nil {
		return failScenario(result, start, err)
	}

	if err := sr.services.Store.Cases().LinkCaseToKnowledgeResource(ctx, caseObj.ID, resource.ID); err != nil {
		return failScenario(result, start, err)
	}

	links, err := sr.services.Store.Cases().GetCaseKnowledgeResourceLinks(ctx, caseObj.ID)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check:   "Case linked to knowledge resource",
		Passed:  err == nil && len(links) == 1,
		Details: fmt.Sprintf("Link count: %d", len(links)),
	})

	if len(links) == 1 {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check:   "Link points to the right resource",
			Passed:  links[0].KnowledgeResourceID == resource.ID,
			Details: fmt.Sprintf("Resource ID: %s", links[0].KnowledgeResourceID),
		})
	}

	result.CaseID = caseObj.ID
	result.Success = allVerificationsPassed(result.Verifications)
	result.Duration = time.Since(start)
	return result, nil
}

func allVerificationsPassed(results []VerificationResult) bool {
	for _, result := range results {
		if !result.Passed {
			return false
		}
	}
	return true
}
