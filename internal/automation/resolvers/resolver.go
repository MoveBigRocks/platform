// Package resolvers provides GraphQL resolvers for the automation domain.
// This domain owns the Rule and Form API surface.
package resolvers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/graph/model"
	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	graphshared "github.com/movebigrocks/platform/pkg/extensionhost/graph/shared"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// Config holds the dependencies for automation domain resolvers
type Config struct {
	RuleService      *automationservices.RuleService
	FormService      *automationservices.FormService
	WorkspaceService *platformservices.WorkspaceManagementService
}

// Resolver handles all automation domain GraphQL operations
type Resolver struct {
	ruleService      *automationservices.RuleService
	formService      *automationservices.FormService
	workspaceService *platformservices.WorkspaceManagementService
}

// NewResolver creates a new automation domain resolver
func NewResolver(cfg Config) *Resolver {
	return &Resolver{
		ruleService:      cfg.RuleService,
		formService:      cfg.FormService,
		workspaceService: cfg.WorkspaceService,
	}
}

// =============================================================================
// Admin Query Resolvers - Rules
// =============================================================================

// AdminRules resolves all admin rules with filtering
func (r *Resolver) AdminRules(ctx context.Context, filter *model.AdminRuleFilterInput) (*AdminRuleConnectionResolver, error) {
	_, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return nil, err
	}

	rules, err := r.ruleService.ListAllRulesFiltered(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list rules: %w", err)
	}

	return &AdminRuleConnectionResolver{rules: rules, r: r}, nil
}

// AdminRule resolves a single rule by ID
func (r *Resolver) AdminRule(ctx context.Context, id string) (*AdminRuleResolver, error) {
	_, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return nil, err
	}

	rule, err := r.ruleService.GetRule(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule: %w", err)
	}

	return &AdminRuleResolver{rule: rule, r: r}, nil
}

// =============================================================================
// Admin Query Resolvers - Forms
// =============================================================================

// AdminForms resolves all admin forms with filtering
func (r *Resolver) AdminForms(ctx context.Context, filter *model.AdminFormFilterInput) (*AdminFormConnectionResolver, error) {
	_, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return nil, err
	}

	forms, err := r.formService.ListAllFormsFiltered(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list forms: %w", err)
	}

	return &AdminFormConnectionResolver{forms: forms, r: r}, nil
}

// AdminForm resolves a single form by ID
func (r *Resolver) AdminForm(ctx context.Context, formID string) (*AdminFormResolver, error) {
	_, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return nil, err
	}

	form, err := r.formService.GetForm(ctx, formID)
	if err != nil {
		return nil, fmt.Errorf("failed to get form: %w", err)
	}

	return &AdminFormResolver{form: form, r: r}, nil
}

// =============================================================================
// Admin Mutation Resolvers - Rules
// =============================================================================

// AdminCreateRule creates a new automation rule
func (r *Resolver) AdminCreateRule(ctx context.Context, input model.CreateRuleInput) (*AdminRuleResolver, error) {
	authCtx, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Parse conditions and actions from JSON
	var conditions []automationdomain.TypedCondition
	if input.Conditions != nil {
		condBytes, _ := json.Marshal(input.Conditions)
		if err := json.Unmarshal(condBytes, &conditions); err != nil {
			return nil, fmt.Errorf("invalid conditions format: %w", err)
		}
	}

	var actions []automationdomain.TypedAction
	if input.Actions != nil {
		actBytes, _ := json.Marshal(input.Actions)
		if err := json.Unmarshal(actBytes, &actions); err != nil {
			return nil, fmt.Errorf("invalid actions format: %w", err)
		}
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	priority := 0
	if input.Priority != nil {
		priority = int(*input.Priority)
	}

	maxPerHour := 0
	if input.MaxExecutionsPerHour != nil {
		maxPerHour = int(*input.MaxExecutionsPerHour)
	}

	maxPerDay := 0
	if input.MaxExecutionsPerDay != nil {
		maxPerDay = int(*input.MaxExecutionsPerDay)
	}

	description := ""
	if input.Description != nil {
		description = *input.Description
	}

	params := automationservices.CreateRuleParams{
		WorkspaceID:          input.WorkspaceID,
		Title:                input.Title,
		Description:          description,
		IsActive:             isActive,
		Priority:             priority,
		MaxExecutionsPerHour: maxPerHour,
		MaxExecutionsPerDay:  maxPerDay,
		Conditions:           conditions,
		Actions:              actions,
		CreatedByID:          authCtx.Principal.GetID(),
	}

	rule, err := r.ruleService.CreateRule(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create rule: %w", err)
	}

	return &AdminRuleResolver{rule: rule, r: r}, nil
}

// AdminUpdateRule updates an existing rule
func (r *Resolver) AdminUpdateRule(ctx context.Context, ruleID string, input model.UpdateRuleInput) (*AdminRuleResolver, error) {
	_, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Get existing rule
	existing, err := r.ruleService.GetRule(ctx, ruleID)
	if err != nil {
		return nil, fmt.Errorf("rule not found: %w", err)
	}

	// Build update params with existing values as defaults
	title := existing.Title
	if input.Title != nil {
		title = *input.Title
	}

	description := existing.Description
	if input.Description != nil {
		description = *input.Description
	}

	isActive := existing.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	priority := existing.Priority
	if input.Priority != nil {
		priority = int(*input.Priority)
	}

	maxPerHour := existing.MaxExecutionsPerHour
	if input.MaxExecutionsPerHour != nil {
		maxPerHour = int(*input.MaxExecutionsPerHour)
	}

	maxPerDay := existing.MaxExecutionsPerDay
	if input.MaxExecutionsPerDay != nil {
		maxPerDay = int(*input.MaxExecutionsPerDay)
	}

	conditions := existing.Conditions.Conditions
	if input.Conditions != nil {
		condBytes, _ := json.Marshal(input.Conditions)
		if err := json.Unmarshal(condBytes, &conditions); err != nil {
			return nil, fmt.Errorf("invalid conditions format: %w", err)
		}
	}

	actions := existing.Actions.Actions
	if input.Actions != nil {
		actBytes, _ := json.Marshal(input.Actions)
		if err := json.Unmarshal(actBytes, &actions); err != nil {
			return nil, fmt.Errorf("invalid actions format: %w", err)
		}
	}

	params := automationservices.UpdateRuleParams{
		Title:                title,
		Description:          description,
		IsActive:             isActive,
		Priority:             priority,
		MaxExecutionsPerHour: maxPerHour,
		MaxExecutionsPerDay:  maxPerDay,
		Conditions:           conditions,
		Actions:              actions,
	}

	rule, err := r.ruleService.UpdateRule(ctx, ruleID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}

	return &AdminRuleResolver{rule: rule, r: r}, nil
}

// AdminDeleteRule deletes a rule
func (r *Resolver) AdminDeleteRule(ctx context.Context, ruleID string) (bool, error) {
	_, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return false, err
	}

	if err := r.ruleService.DeleteRule(ctx, ruleID); err != nil {
		return false, fmt.Errorf("failed to delete rule: %w", err)
	}

	return true, nil
}

// =============================================================================
// Admin Mutation Resolvers - Forms
// =============================================================================

// AdminCreateForm creates a new form
func (r *Resolver) AdminCreateForm(ctx context.Context, input model.CreateFormInput) (*AdminFormResolver, error) {
	authCtx, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	form := &servicedomain.FormSchema{
		WorkspaceID: input.WorkspaceID,
		Name:        input.Name,
		Slug:        input.Slug,
		CryptoID:    id.NewPublicID(),
		Status:      servicedomain.FormStatusDraft,
		CreatedByID: authCtx.Principal.GetID(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if input.Description != nil {
		form.Description = *input.Description
	}
	if input.Status != nil {
		form.Status = servicedomain.FormStatus(*input.Status)
	}
	if input.IsPublic != nil {
		form.IsPublic = *input.IsPublic
	}
	if input.RequiresCaptcha != nil {
		form.RequiresCaptcha = *input.RequiresCaptcha
	}
	if input.CollectEmail != nil {
		form.CollectEmail = *input.CollectEmail
	}
	if input.AutoCreateCase != nil {
		form.AutoCreateCase = *input.AutoCreateCase
	}
	if input.AutoCasePriority != nil {
		form.AutoCasePriority = *input.AutoCasePriority
	}
	if input.AutoCaseType != nil {
		form.AutoCaseType = *input.AutoCaseType
	}
	if input.AutoAssignTeamID != nil {
		form.AutoAssignTeamID = *input.AutoAssignTeamID
	}
	if input.AutoTags != nil {
		form.AutoTags = *input.AutoTags
	}
	if input.NotifyOnSubmission != nil {
		form.NotifyOnSubmission = *input.NotifyOnSubmission
	}
	if input.NotificationEmails != nil {
		form.NotificationEmails = *input.NotificationEmails
	}
	if input.SubmissionMessage != nil {
		form.SubmissionMessage = *input.SubmissionMessage
	}
	if input.RedirectURL != nil {
		form.RedirectURL = *input.RedirectURL
	}

	if err := r.formService.CreateForm(ctx, form); err != nil {
		return nil, fmt.Errorf("failed to create form: %w", err)
	}

	return &AdminFormResolver{form: form, r: r}, nil
}

// AdminUpdateForm updates an existing form
func (r *Resolver) AdminUpdateForm(ctx context.Context, formID string, input model.UpdateFormInput) (*AdminFormResolver, error) {
	_, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return nil, err
	}

	form, err := r.formService.GetForm(ctx, formID)
	if err != nil {
		return nil, fmt.Errorf("form not found: %w", err)
	}

	if input.Name != nil {
		form.Name = *input.Name
	}
	if input.Slug != nil {
		form.Slug = *input.Slug
	}
	if input.Description != nil {
		form.Description = *input.Description
	}
	if input.Status != nil {
		form.Status = servicedomain.FormStatus(*input.Status)
	}
	if input.IsPublic != nil {
		form.IsPublic = *input.IsPublic
	}
	if input.RequiresCaptcha != nil {
		form.RequiresCaptcha = *input.RequiresCaptcha
	}
	if input.CollectEmail != nil {
		form.CollectEmail = *input.CollectEmail
	}
	if input.AutoCreateCase != nil {
		form.AutoCreateCase = *input.AutoCreateCase
	}
	if input.AutoCasePriority != nil {
		form.AutoCasePriority = *input.AutoCasePriority
	}
	if input.AutoCaseType != nil {
		form.AutoCaseType = *input.AutoCaseType
	}
	if input.AutoAssignTeamID != nil {
		form.AutoAssignTeamID = *input.AutoAssignTeamID
	}
	if input.AutoTags != nil {
		form.AutoTags = *input.AutoTags
	}
	if input.NotifyOnSubmission != nil {
		form.NotifyOnSubmission = *input.NotifyOnSubmission
	}
	if input.NotificationEmails != nil {
		form.NotificationEmails = *input.NotificationEmails
	}
	if input.SubmissionMessage != nil {
		form.SubmissionMessage = *input.SubmissionMessage
	}
	if input.RedirectURL != nil {
		form.RedirectURL = *input.RedirectURL
	}

	form.UpdatedAt = time.Now()

	if err := r.formService.UpdateForm(ctx, form); err != nil {
		return nil, fmt.Errorf("failed to update form: %w", err)
	}

	return &AdminFormResolver{form: form, r: r}, nil
}

// AdminDeleteForm deletes a form
func (r *Resolver) AdminDeleteForm(ctx context.Context, formID string) (bool, error) {
	_, err := graphshared.RequireInstanceAdmin(ctx)
	if err != nil {
		return false, err
	}

	// Fetch form to get workspace ID for workspace-scoped delete
	form, err := r.formService.GetForm(ctx, formID)
	if err != nil {
		return false, fmt.Errorf("form not found: %w", err)
	}

	if err := r.formService.DeleteForm(ctx, form.WorkspaceID, formID); err != nil {
		return false, fmt.Errorf("failed to delete form: %w", err)
	}

	return true, nil
}

// =============================================================================
// Type Resolvers - Rules
// =============================================================================

// AdminRuleResolver resolves AdminRule fields
type AdminRuleResolver struct {
	rule *automationdomain.Rule
	r    *Resolver
}

// ID returns the rule ID
func (r *AdminRuleResolver) ID() model.ID {
	return model.ID(r.rule.ID)
}

// WorkspaceID returns the workspace ID
func (r *AdminRuleResolver) WorkspaceID() model.ID {
	return model.ID(r.rule.WorkspaceID)
}

// WorkspaceName returns the workspace name (lazy load)
func (r *AdminRuleResolver) WorkspaceName(ctx context.Context) *string {
	if r.r.workspaceService == nil {
		return nil
	}
	workspace, err := r.r.workspaceService.GetWorkspace(ctx, r.rule.WorkspaceID)
	if err != nil {
		return nil
	}
	return &workspace.Name
}

// Title returns the rule title
func (r *AdminRuleResolver) Title() string {
	return r.rule.Title
}

// Description returns the rule description
func (r *AdminRuleResolver) Description() *string {
	if r.rule.Description == "" {
		return nil
	}
	return &r.rule.Description
}

// IsActive returns whether the rule is active
func (r *AdminRuleResolver) IsActive() bool {
	return r.rule.IsActive
}

// Priority returns the rule priority
func (r *AdminRuleResolver) Priority() int32 {
	return int32(r.rule.Priority)
}

// MaxExecutionsPerHour returns the max hourly executions
func (r *AdminRuleResolver) MaxExecutionsPerHour() int32 {
	return int32(r.rule.MaxExecutionsPerHour)
}

// MaxExecutionsPerDay returns the max daily executions
func (r *AdminRuleResolver) MaxExecutionsPerDay() int32 {
	return int32(r.rule.MaxExecutionsPerDay)
}

// Conditions returns the conditions as JSON
func (r *AdminRuleResolver) Conditions() *graphshared.JSON {
	data := map[string]interface{}{
		"operator":   r.rule.Conditions.Operator,
		"conditions": r.rule.Conditions.Conditions,
	}
	j := graphshared.JSON(data)
	return &j
}

// Actions returns the actions as JSON
func (r *AdminRuleResolver) Actions() *graphshared.JSON {
	data := map[string]interface{}{
		"actions": r.rule.Actions.Actions,
	}
	j := graphshared.JSON(data)
	return &j
}

// ExecutionCount returns the execution count
func (r *AdminRuleResolver) ExecutionCount() int32 {
	return int32(r.rule.TotalExecutions)
}

// LastExecutedAt returns the last execution timestamp
func (r *AdminRuleResolver) LastExecutedAt() *graphshared.DateTime {
	if r.rule.LastExecutedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.rule.LastExecutedAt}
}

// CreatedAt returns the creation timestamp
func (r *AdminRuleResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.rule.CreatedAt}
}

// UpdatedAt returns the update timestamp
func (r *AdminRuleResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.rule.UpdatedAt}
}

// CreatedByID returns the creator ID
func (r *AdminRuleResolver) CreatedByID() model.ID {
	return model.ID(r.rule.CreatedByID)
}

// =============================================================================
// Type Resolvers - Forms
// =============================================================================

// AdminFormResolver resolves AdminForm fields
type AdminFormResolver struct {
	form *servicedomain.FormSchema
	r    *Resolver
}

// ID returns the form ID
func (f *AdminFormResolver) ID() model.ID {
	return model.ID(f.form.ID)
}

// WorkspaceID returns the workspace ID
func (f *AdminFormResolver) WorkspaceID() model.ID {
	return model.ID(f.form.WorkspaceID)
}

// WorkspaceName returns the workspace name (lazy load)
func (f *AdminFormResolver) WorkspaceName(ctx context.Context) *string {
	if f.r.workspaceService == nil {
		return nil
	}
	workspace, err := f.r.workspaceService.GetWorkspace(ctx, f.form.WorkspaceID)
	if err != nil {
		return nil
	}
	return &workspace.Name
}

// Name returns the form name
func (f *AdminFormResolver) Name() string {
	return f.form.Name
}

// Slug returns the form slug
func (f *AdminFormResolver) Slug() string {
	return f.form.Slug
}

// Description returns the form description
func (f *AdminFormResolver) Description() *string {
	if f.form.Description == "" {
		return nil
	}
	return &f.form.Description
}

// Status returns the form status
func (f *AdminFormResolver) Status() string {
	return string(f.form.Status)
}

// CryptoID returns the crypto ID
func (f *AdminFormResolver) CryptoID() string {
	return f.form.CryptoID
}

// IsPublic returns whether the form is public
func (f *AdminFormResolver) IsPublic() bool {
	return f.form.IsPublic
}

// RequiresCaptcha returns whether captcha is required
func (f *AdminFormResolver) RequiresCaptcha() bool {
	return f.form.RequiresCaptcha
}

// CollectEmail returns whether email is collected
func (f *AdminFormResolver) CollectEmail() bool {
	return f.form.CollectEmail
}

// AutoCreateCase returns whether cases are auto-created
func (f *AdminFormResolver) AutoCreateCase() bool {
	return f.form.AutoCreateCase
}

// AutoCasePriority returns the auto case priority
func (f *AdminFormResolver) AutoCasePriority() *string {
	if f.form.AutoCasePriority == "" {
		return nil
	}
	return &f.form.AutoCasePriority
}

// AutoCaseType returns the auto case type
func (f *AdminFormResolver) AutoCaseType() *string {
	if f.form.AutoCaseType == "" {
		return nil
	}
	return &f.form.AutoCaseType
}

// AutoAssignTeamID returns the auto assign team ID
func (f *AdminFormResolver) AutoAssignTeamID() *string {
	if f.form.AutoAssignTeamID == "" {
		return nil
	}
	return &f.form.AutoAssignTeamID
}

// AutoTags returns the auto tags
func (f *AdminFormResolver) AutoTags() *[]string {
	if len(f.form.AutoTags) == 0 {
		return nil
	}
	return &f.form.AutoTags
}

// NotifyOnSubmission returns whether notifications are sent on submission
func (f *AdminFormResolver) NotifyOnSubmission() bool {
	return f.form.NotifyOnSubmission
}

// NotificationEmails returns the notification emails
func (f *AdminFormResolver) NotificationEmails() *[]string {
	if len(f.form.NotificationEmails) == 0 {
		return nil
	}
	return &f.form.NotificationEmails
}

// SubmissionMessage returns the submission message
func (f *AdminFormResolver) SubmissionMessage() *string {
	if f.form.SubmissionMessage == "" {
		return nil
	}
	return &f.form.SubmissionMessage
}

// RedirectURL returns the redirect URL
func (f *AdminFormResolver) RedirectURL() *string {
	if f.form.RedirectURL == "" {
		return nil
	}
	return &f.form.RedirectURL
}

// SchemaData returns the schema data as JSON
func (f *AdminFormResolver) SchemaData() *graphshared.JSON {
	if f.form.SchemaData.IsEmpty() {
		return nil
	}
	data := f.form.SchemaData.ToMap()
	j := graphshared.JSON(data)
	return &j
}

// SubmissionCount returns the submission count
func (f *AdminFormResolver) SubmissionCount() int32 {
	return int32(f.form.SubmissionCount)
}

// CreatedAt returns the creation timestamp
func (f *AdminFormResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: f.form.CreatedAt}
}

// UpdatedAt returns the update timestamp
func (f *AdminFormResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: f.form.UpdatedAt}
}

// CreatedByID returns the creator ID
func (f *AdminFormResolver) CreatedByID() model.ID {
	return model.ID(f.form.CreatedByID)
}

// =============================================================================
// Connection Resolvers
// =============================================================================

// AdminRuleConnectionResolver resolves AdminRuleConnection
type AdminRuleConnectionResolver struct {
	rules []*automationdomain.Rule
	r     *Resolver
}

// Edges returns the rule edges
func (c *AdminRuleConnectionResolver) Edges() []*AdminRuleEdgeResolver {
	edges := make([]*AdminRuleEdgeResolver, len(c.rules))
	for i, rule := range c.rules {
		edges[i] = &AdminRuleEdgeResolver{rule: rule, r: c.r}
	}
	return edges
}

// PageInfo returns pagination info
func (c *AdminRuleConnectionResolver) PageInfo() *PageInfoResolver {
	return &PageInfoResolver{}
}

// TotalCount returns the total count
func (c *AdminRuleConnectionResolver) TotalCount() int32 {
	return int32(len(c.rules))
}

// AdminRuleEdgeResolver resolves AdminRuleEdge
type AdminRuleEdgeResolver struct {
	rule *automationdomain.Rule
	r    *Resolver
}

// Node returns the admin rule
func (e *AdminRuleEdgeResolver) Node() *AdminRuleResolver {
	return &AdminRuleResolver{rule: e.rule, r: e.r}
}

// Cursor returns the cursor
func (e *AdminRuleEdgeResolver) Cursor() string {
	return e.rule.ID
}

// AdminFormConnectionResolver resolves AdminFormConnection
type AdminFormConnectionResolver struct {
	forms []*servicedomain.FormSchema
	r     *Resolver
}

// Edges returns the form edges
func (c *AdminFormConnectionResolver) Edges() []*AdminFormEdgeResolver {
	edges := make([]*AdminFormEdgeResolver, len(c.forms))
	for i, form := range c.forms {
		edges[i] = &AdminFormEdgeResolver{form: form, r: c.r}
	}
	return edges
}

// PageInfo returns pagination info
func (c *AdminFormConnectionResolver) PageInfo() *PageInfoResolver {
	return &PageInfoResolver{}
}

// TotalCount returns the total count
func (c *AdminFormConnectionResolver) TotalCount() int32 {
	return int32(len(c.forms))
}

// AdminFormEdgeResolver resolves AdminFormEdge
type AdminFormEdgeResolver struct {
	form *servicedomain.FormSchema
	r    *Resolver
}

// Node returns the admin form
func (e *AdminFormEdgeResolver) Node() *AdminFormResolver {
	return &AdminFormResolver{form: e.form, r: e.r}
}

// Cursor returns the cursor
func (e *AdminFormEdgeResolver) Cursor() string {
	return e.form.ID
}

// PageInfoResolver resolves PageInfo
type PageInfoResolver struct {
	hasNextPage     bool
	hasPreviousPage bool
	startCursor     *string
	endCursor       *string
}

// HasNextPage returns whether there's a next page
func (p *PageInfoResolver) HasNextPage() bool {
	return p.hasNextPage
}

// HasPreviousPage returns whether there's a previous page
func (p *PageInfoResolver) HasPreviousPage() bool {
	return p.hasPreviousPage
}

// StartCursor returns the start cursor
func (p *PageInfoResolver) StartCursor() *string {
	return p.startCursor
}

// EndCursor returns the end cursor
func (p *PageInfoResolver) EndCursor() *string {
	return p.endCursor
}
