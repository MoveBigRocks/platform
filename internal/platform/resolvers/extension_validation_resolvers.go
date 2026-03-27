package resolvers

import (
	"context"
	"fmt"
	"strings"

	"github.com/movebigrocks/platform/internal/graph/model"
	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
)

func (r *Resolver) WorkspaceExtensionAdminNavigation(ctx context.Context, workspaceID string) ([]*ResolvedExtensionAdminNavigationItemResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	if r.extensionService == nil {
		return []*ResolvedExtensionAdminNavigationItemResolver{}, nil
	}
	items, err := r.extensionService.ListWorkspaceAdminNavigation(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace extension navigation: %w", err)
	}
	return resolvedAdminNavigationResolvers(items), nil
}

func (r *Resolver) WorkspaceExtensionDashboardWidgets(ctx context.Context, workspaceID string) ([]*ResolvedExtensionDashboardWidgetResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	if r.extensionService == nil {
		return []*ResolvedExtensionDashboardWidgetResolver{}, nil
	}
	items, err := r.extensionService.ListWorkspaceDashboardWidgets(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace extension dashboard widgets: %w", err)
	}
	return resolvedDashboardWidgetResolvers(items), nil
}

func (r *Resolver) InstanceExtensionAdminNavigation(ctx context.Context) ([]*ResolvedExtensionAdminNavigationItemResolver, error) {
	if _, err := graphshared.RequireInstanceAdmin(ctx); err != nil {
		return nil, err
	}
	if r.extensionService == nil {
		return []*ResolvedExtensionAdminNavigationItemResolver{}, nil
	}
	items, err := r.extensionService.ListInstanceAdminNavigation(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance extension navigation: %w", err)
	}
	return resolvedAdminNavigationResolvers(items), nil
}

func (r *Resolver) InstanceExtensionDashboardWidgets(ctx context.Context) ([]*ResolvedExtensionDashboardWidgetResolver, error) {
	if _, err := graphshared.RequireInstanceAdmin(ctx); err != nil {
		return nil, err
	}
	if r.extensionService == nil {
		return []*ResolvedExtensionDashboardWidgetResolver{}, nil
	}
	items, err := r.extensionService.ListInstanceDashboardWidgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance extension dashboard widgets: %w", err)
	}
	return resolvedDashboardWidgetResolvers(items), nil
}

func (e *InstalledExtensionResolver) ResolvedAdminNavigation(ctx context.Context) ([]*ResolvedExtensionAdminNavigationItemResolver, error) {
	if e.r == nil || e.r.extensionService == nil {
		return []*ResolvedExtensionAdminNavigationItemResolver{}, nil
	}
	items, err := e.r.extensionService.ListExtensionResolvedAdminNavigation(ctx, e.extension.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve extension admin navigation: %w", err)
	}
	return resolvedAdminNavigationResolvers(items), nil
}

func (e *InstalledExtensionResolver) ResolvedDashboardWidgets(ctx context.Context) ([]*ResolvedExtensionDashboardWidgetResolver, error) {
	if e.r == nil || e.r.extensionService == nil {
		return []*ResolvedExtensionDashboardWidgetResolver{}, nil
	}
	items, err := e.r.extensionService.ListExtensionResolvedDashboardWidgets(ctx, e.extension.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve extension dashboard widgets: %w", err)
	}
	return resolvedDashboardWidgetResolvers(items), nil
}

func (e *InstalledExtensionResolver) SeededResources(ctx context.Context) (*ExtensionSeededResourcesResolver, error) {
	if e.r == nil || e.r.extensionService == nil {
		return &ExtensionSeededResourcesResolver{}, nil
	}
	report, err := e.r.extensionService.InspectExtensionSeededResources(ctx, e.extension.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect extension seeded resources: %w", err)
	}
	return &ExtensionSeededResourcesResolver{report: report}, nil
}

type ResolvedExtensionAdminNavigationItemResolver struct {
	item platformservices.ResolvedExtensionAdminNavigationItem
}

func (r *ResolvedExtensionAdminNavigationItemResolver) ExtensionID() model.ID {
	return model.ID(r.item.ExtensionID)
}

func (r *ResolvedExtensionAdminNavigationItemResolver) ExtensionSlug() string {
	return r.item.ExtensionSlug
}

func (r *ResolvedExtensionAdminNavigationItemResolver) WorkspaceID() *model.ID {
	if strings.TrimSpace(r.item.WorkspaceID) == "" {
		return nil
	}
	value := model.ID(r.item.WorkspaceID)
	return &value
}

func (r *ResolvedExtensionAdminNavigationItemResolver) Section() *string {
	return nullableTrimmedString(r.item.Section)
}

func (r *ResolvedExtensionAdminNavigationItemResolver) Title() string { return r.item.Title }

func (r *ResolvedExtensionAdminNavigationItemResolver) Icon() *string {
	return nullableTrimmedString(r.item.Icon)
}

func (r *ResolvedExtensionAdminNavigationItemResolver) Href() string { return r.item.Href }

func (r *ResolvedExtensionAdminNavigationItemResolver) ActivePage() *string {
	return nullableTrimmedString(r.item.ActivePage)
}

type ResolvedExtensionDashboardWidgetResolver struct {
	widget platformservices.ResolvedExtensionDashboardWidget
}

func (r *ResolvedExtensionDashboardWidgetResolver) ExtensionID() model.ID {
	return model.ID(r.widget.ExtensionID)
}

func (r *ResolvedExtensionDashboardWidgetResolver) ExtensionSlug() string {
	return r.widget.ExtensionSlug
}

func (r *ResolvedExtensionDashboardWidgetResolver) WorkspaceID() *model.ID {
	if strings.TrimSpace(r.widget.WorkspaceID) == "" {
		return nil
	}
	value := model.ID(r.widget.WorkspaceID)
	return &value
}

func (r *ResolvedExtensionDashboardWidgetResolver) Title() string { return r.widget.Title }

func (r *ResolvedExtensionDashboardWidgetResolver) Description() *string {
	return nullableTrimmedString(r.widget.Description)
}

func (r *ResolvedExtensionDashboardWidgetResolver) Icon() *string {
	return nullableTrimmedString(r.widget.Icon)
}

func (r *ResolvedExtensionDashboardWidgetResolver) Href() string { return r.widget.Href }

type ExtensionSeededResourcesResolver struct {
	report platformservices.ExtensionSeededResourcesReport
}

func (r *ExtensionSeededResourcesResolver) Queues() []*ExtensionSeededQueueStateResolver {
	result := make([]*ExtensionSeededQueueStateResolver, 0, len(r.report.Queues))
	for _, item := range r.report.Queues {
		result = append(result, &ExtensionSeededQueueStateResolver{state: item})
	}
	return result
}

func (r *ExtensionSeededResourcesResolver) Forms() []*ExtensionSeededFormStateResolver {
	result := make([]*ExtensionSeededFormStateResolver, 0, len(r.report.Forms))
	for _, item := range r.report.Forms {
		result = append(result, &ExtensionSeededFormStateResolver{state: item})
	}
	return result
}

func (r *ExtensionSeededResourcesResolver) AutomationRules() []*ExtensionSeededAutomationRuleStateResolver {
	result := make([]*ExtensionSeededAutomationRuleStateResolver, 0, len(r.report.AutomationRules))
	for _, item := range r.report.AutomationRules {
		result = append(result, &ExtensionSeededAutomationRuleStateResolver{state: item})
	}
	return result
}

type ExtensionSeededQueueStateResolver struct {
	state platformservices.ExtensionSeededQueueState
}

func (r *ExtensionSeededQueueStateResolver) Slug() string { return r.state.Slug }

func (r *ExtensionSeededQueueStateResolver) ResourceID() *model.ID {
	return nullableID(r.state.ResourceID)
}

func (r *ExtensionSeededQueueStateResolver) Exists() bool { return r.state.Exists }

func (r *ExtensionSeededQueueStateResolver) MatchesSeed() bool { return r.state.MatchesSeed }

func (r *ExtensionSeededQueueStateResolver) Problems() []string {
	return append([]string{}, r.state.Problems...)
}

func (r *ExtensionSeededQueueStateResolver) Expected() graphshared.JSON {
	return jsonValueOrEmpty(r.state.Expected)
}

func (r *ExtensionSeededQueueStateResolver) Actual() *graphshared.JSON {
	return nullableJSON(r.state.Actual)
}

type ExtensionSeededFormStateResolver struct {
	state platformservices.ExtensionSeededFormState
}

func (r *ExtensionSeededFormStateResolver) Slug() string { return r.state.Slug }

func (r *ExtensionSeededFormStateResolver) ResourceID() *model.ID {
	return nullableID(r.state.ResourceID)
}

func (r *ExtensionSeededFormStateResolver) Exists() bool { return r.state.Exists }

func (r *ExtensionSeededFormStateResolver) MatchesSeed() bool { return r.state.MatchesSeed }

func (r *ExtensionSeededFormStateResolver) Problems() []string {
	return append([]string{}, r.state.Problems...)
}

func (r *ExtensionSeededFormStateResolver) Expected() graphshared.JSON {
	return jsonValueOrEmpty(r.state.Expected)
}

func (r *ExtensionSeededFormStateResolver) Actual() *graphshared.JSON {
	return nullableJSON(r.state.Actual)
}

type ExtensionSeededAutomationRuleStateResolver struct {
	state platformservices.ExtensionSeededAutomationRuleState
}

func (r *ExtensionSeededAutomationRuleStateResolver) Key() string { return r.state.Key }

func (r *ExtensionSeededAutomationRuleStateResolver) ResourceID() *model.ID {
	return nullableID(r.state.ResourceID)
}

func (r *ExtensionSeededAutomationRuleStateResolver) Exists() bool { return r.state.Exists }

func (r *ExtensionSeededAutomationRuleStateResolver) MatchesSeed() bool { return r.state.MatchesSeed }

func (r *ExtensionSeededAutomationRuleStateResolver) Problems() []string {
	return append([]string{}, r.state.Problems...)
}

func (r *ExtensionSeededAutomationRuleStateResolver) Expected() graphshared.JSON {
	return jsonValueOrEmpty(r.state.Expected)
}

func (r *ExtensionSeededAutomationRuleStateResolver) Actual() *graphshared.JSON {
	return nullableJSON(r.state.Actual)
}

func resolvedAdminNavigationResolvers(items []platformservices.ResolvedExtensionAdminNavigationItem) []*ResolvedExtensionAdminNavigationItemResolver {
	result := make([]*ResolvedExtensionAdminNavigationItemResolver, 0, len(items))
	for _, item := range items {
		result = append(result, &ResolvedExtensionAdminNavigationItemResolver{item: item})
	}
	return result
}

func resolvedDashboardWidgetResolvers(items []platformservices.ResolvedExtensionDashboardWidget) []*ResolvedExtensionDashboardWidgetResolver {
	result := make([]*ResolvedExtensionDashboardWidgetResolver, 0, len(items))
	for _, item := range items {
		result = append(result, &ResolvedExtensionDashboardWidgetResolver{widget: item})
	}
	return result
}

func nullableTrimmedString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func nullableID(value string) *model.ID {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	id := model.ID(value)
	return &id
}

func nullableJSON(value map[string]any) *graphshared.JSON {
	if value == nil {
		return nil
	}
	jsonValue := graphshared.JSON(value)
	return &jsonValue
}

func jsonValueOrEmpty(value map[string]any) graphshared.JSON {
	if value == nil {
		return graphshared.JSON{}
	}
	return graphshared.JSON(value)
}
