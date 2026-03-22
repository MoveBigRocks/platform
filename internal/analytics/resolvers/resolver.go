package analyticsresolvers

import (
	"context"
	"fmt"
	"time"

	graphql "github.com/graph-gophers/graphql-go"

	analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"
	analyticsservices "github.com/movebigrocks/platform/internal/analytics/services"
	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
)

// Config holds the dependencies for analytics domain resolvers.
type Config struct {
	QueryService     *analyticsservices.QueryService
	APIBaseURL       string // For snippet HTML generation
	ExtensionChecker ExtensionChecker
}

type ExtensionChecker interface {
	HasActiveExtension(ctx context.Context, slug string) (bool, error)
	HasActiveExtensionInWorkspace(ctx context.Context, workspaceID, slug string) (bool, error)
}

// Resolver handles all analytics domain GraphQL operations.
type Resolver struct {
	queryService     *analyticsservices.QueryService
	apiBaseURL       string
	extensionChecker ExtensionChecker
}

// NewResolver creates a new analytics domain resolver.
func NewResolver(cfg Config) *Resolver {
	return &Resolver{
		queryService:     cfg.QueryService,
		apiBaseURL:       cfg.APIBaseURL,
		extensionChecker: cfg.ExtensionChecker,
	}
}

// =============================================================================
// Property Queries
// =============================================================================

// AnalyticsProperties returns properties for the current workspace,
// or all properties if the caller is an instance admin without workspace context.
func (r *Resolver) AnalyticsProperties(ctx context.Context) ([]*AnalyticsPropertyResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	var properties []*analyticsdomain.Property
	if authCtx.WorkspaceID != "" {
		if err := r.ensureWorkspaceEnabled(ctx, authCtx.WorkspaceID); err != nil {
			return nil, err
		}
		properties, err = r.queryService.ListProperties(ctx, authCtx.WorkspaceID)
	} else if authCtx.IsInstanceAdmin() {
		if err := r.ensureSurfaceEnabled(ctx); err != nil {
			return nil, err
		}
		properties, err = r.queryService.ListAllProperties(ctx)
	} else {
		return nil, fmt.Errorf("workspace context required")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list properties: %w", err)
	}

	resolvers := make([]*AnalyticsPropertyResolver, len(properties))
	for i, p := range properties {
		resolvers[i] = &AnalyticsPropertyResolver{property: p, r: r}
	}
	return resolvers, nil
}

// AnalyticsProperty returns a single property by ID.
func (r *Resolver) AnalyticsProperty(ctx context.Context, id string) (*AnalyticsPropertyResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	prop, err := r.queryService.GetProperty(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("property not found")
	}

	if err := graphshared.ValidateWorkspaceOwnership(prop.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("property not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, prop.WorkspaceID); err != nil {
		return nil, err
	}

	return &AnalyticsPropertyResolver{property: prop, r: r}, nil
}

// AnalyticsMetrics returns metrics for a property and period.
func (r *Resolver) AnalyticsMetrics(ctx context.Context, propertyID, period string, from, to *time.Time) (*AnalyticsMetricsResolver, error) {
	prop, err := r.validatePropertyAccess(ctx, propertyID)
	if err != nil {
		return nil, err
	}

	metrics, err := r.queryService.GetMetrics(ctx, propertyID, period, prop.Timezone, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	return &AnalyticsMetricsResolver{metrics: metrics}, nil
}

// AnalyticsTimeSeries returns time series data for a property.
func (r *Resolver) AnalyticsTimeSeries(ctx context.Context, propertyID, period, interval string, from, to *time.Time) ([]*AnalyticsTimeSeriesResolver, error) {
	prop, err := r.validatePropertyAccess(ctx, propertyID)
	if err != nil {
		return nil, err
	}

	rows, err := r.queryService.GetTimeSeries(ctx, propertyID, period, interval, prop.Timezone, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get time series: %w", err)
	}

	resolvers := make([]*AnalyticsTimeSeriesResolver, len(rows))
	for i, row := range rows {
		resolvers[i] = &AnalyticsTimeSeriesResolver{row: row}
	}
	return resolvers, nil
}

// AnalyticsBreakdown returns breakdown data for a property.
func (r *Resolver) AnalyticsBreakdown(ctx context.Context, propertyID, period, dimension string, limit int, from, to *time.Time) ([]*AnalyticsBreakdownRowResolver, error) {
	prop, err := r.validatePropertyAccess(ctx, propertyID)
	if err != nil {
		return nil, err
	}

	rows, err := r.queryService.GetBreakdown(ctx, propertyID, period, dimension, prop.Timezone, limit, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get breakdown: %w", err)
	}

	resolvers := make([]*AnalyticsBreakdownRowResolver, len(rows))
	for i, row := range rows {
		resolvers[i] = &AnalyticsBreakdownRowResolver{row: row}
	}
	return resolvers, nil
}

// AnalyticsGoals returns goal results for a property.
func (r *Resolver) AnalyticsGoals(ctx context.Context, propertyID, period string, from, to *time.Time) ([]*AnalyticsGoalResultResolver, error) {
	prop, err := r.validatePropertyAccess(ctx, propertyID)
	if err != nil {
		return nil, err
	}

	goals, err := r.queryService.ListGoals(ctx, propertyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list goals: %w", err)
	}

	results, err := r.queryService.GetGoalResults(ctx, propertyID, period, prop.Timezone, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get goal results: %w", err)
	}

	// Build goal lookup
	goalMap := make(map[string]*analyticsdomain.Goal, len(goals))
	for _, g := range goals {
		goalMap[g.ID] = g
	}

	resolvers := make([]*AnalyticsGoalResultResolver, 0, len(results))
	for _, result := range results {
		goal := goalMap[result.GoalID]
		if goal == nil {
			continue
		}
		resolvers = append(resolvers, &AnalyticsGoalResultResolver{
			goal:   goal,
			result: result,
		})
	}
	return resolvers, nil
}

// AnalyticsCurrentVisitors returns the count of current visitors.
func (r *Resolver) AnalyticsCurrentVisitors(ctx context.Context, propertyID string) (int32, error) {
	if _, err := r.validatePropertyAccess(ctx, propertyID); err != nil {
		return 0, err
	}

	count, err := r.queryService.GetCurrentVisitors(ctx, propertyID)
	if err != nil {
		return 0, fmt.Errorf("failed to get current visitors: %w", err)
	}
	return int32(count), nil
}

// AnalyticsVerifyInstallation checks if events have been received.
func (r *Resolver) AnalyticsVerifyInstallation(ctx context.Context, propertyID string) (bool, error) {
	if _, err := r.validatePropertyAccess(ctx, propertyID); err != nil {
		return false, err
	}
	return r.queryService.VerifyInstallation(ctx, propertyID)
}

// AnalyticsHostnameRules returns hostname rules for a property.
func (r *Resolver) AnalyticsHostnameRules(ctx context.Context, propertyID string) ([]*HostnameRuleResolver, error) {
	if _, err := r.validatePropertyAccess(ctx, propertyID); err != nil {
		return nil, err
	}

	rules, err := r.queryService.ListHostnameRules(ctx, propertyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list hostname rules: %w", err)
	}

	resolvers := make([]*HostnameRuleResolver, len(rules))
	for i, rule := range rules {
		resolvers[i] = &HostnameRuleResolver{rule: rule}
	}
	return resolvers, nil
}

// =============================================================================
// Mutations
// =============================================================================

// CreateAnalyticsProperty creates a new analytics property.
func (r *Resolver) CreateAnalyticsProperty(ctx context.Context, domain string, timezone *string, workspaceID *string) (*AnalyticsPropertyResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Resolve workspace: explicit param > auth context > error
	wsID := authCtx.WorkspaceID
	if workspaceID != nil && *workspaceID != "" {
		wsID = *workspaceID
	}
	if wsID == "" {
		return nil, fmt.Errorf("workspace context required")
	}
	if err := r.ensureWorkspaceEnabled(ctx, wsID); err != nil {
		return nil, err
	}

	// Verify the caller has access to the target workspace
	if err := graphshared.ValidateWorkspaceOwnership(wsID, authCtx); err != nil {
		return nil, fmt.Errorf("not authorized for this workspace")
	}

	tz := "UTC"
	if timezone != nil {
		tz = *timezone
	}

	prop, err := r.queryService.CreateProperty(ctx, wsID, domain, tz)
	if err != nil {
		return nil, fmt.Errorf("failed to create property: %w", err)
	}

	return &AnalyticsPropertyResolver{property: prop, r: r}, nil
}

// UpdateAnalyticsProperty updates an existing analytics property.
func (r *Resolver) UpdateAnalyticsProperty(ctx context.Context, id string, domain, timezone, status *string) (*AnalyticsPropertyResolver, error) {
	prop, err := r.validatePropertyAccess(ctx, id)
	if err != nil {
		return nil, err
	}

	if domain != nil {
		prop.Domain = *domain
	}
	if timezone != nil {
		prop.Timezone = *timezone
	}
	if status != nil {
		prop.Status = *status
	}
	prop.UpdatedAt = time.Now().UTC()

	if err := r.queryService.UpdateProperty(ctx, prop); err != nil {
		return nil, fmt.Errorf("failed to update property: %w", err)
	}

	return &AnalyticsPropertyResolver{property: prop, r: r}, nil
}

// DeleteAnalyticsProperty deletes an analytics property.
func (r *Resolver) DeleteAnalyticsProperty(ctx context.Context, id string) (bool, error) {
	if _, err := r.validatePropertyAccess(ctx, id); err != nil {
		return false, err
	}

	if err := r.queryService.DeleteProperty(ctx, id); err != nil {
		return false, fmt.Errorf("failed to delete property: %w", err)
	}
	return true, nil
}

// ResetAnalyticsStats resets stats for a property (keeps config + goals).
func (r *Resolver) ResetAnalyticsStats(ctx context.Context, id string) (*AnalyticsPropertyResolver, error) {
	if _, err := r.validatePropertyAccess(ctx, id); err != nil {
		return nil, err
	}

	if err := r.queryService.ResetPropertyStats(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to reset stats: %w", err)
	}

	// Refresh property after reset
	prop, err := r.queryService.GetProperty(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to reload property after reset: %w", err)
	}

	return &AnalyticsPropertyResolver{property: prop, r: r}, nil
}

// CreateAnalyticsGoal creates a new goal for a property.
func (r *Resolver) CreateAnalyticsGoal(ctx context.Context, propertyID, goalType string, eventName, pagePath *string) (*AnalyticsGoalResolver, error) {
	if _, err := r.validatePropertyAccess(ctx, propertyID); err != nil {
		return nil, err
	}

	en := ""
	if eventName != nil {
		en = *eventName
	}
	pp := ""
	if pagePath != nil {
		pp = *pagePath
	}

	goal, err := r.queryService.CreateGoal(ctx, propertyID, goalType, en, pp)
	if err != nil {
		return nil, fmt.Errorf("failed to create goal: %w", err)
	}

	return &AnalyticsGoalResolver{goal: goal}, nil
}

// DeleteAnalyticsGoal deletes a goal.
func (r *Resolver) DeleteAnalyticsGoal(ctx context.Context, id string) (bool, error) {
	// Validate goal ownership via property
	goal, err := r.queryService.GetGoal(ctx, id)
	if err != nil {
		return false, fmt.Errorf("goal not found")
	}
	if _, err := r.validatePropertyAccess(ctx, goal.PropertyID); err != nil {
		return false, err
	}

	if err := r.queryService.DeleteGoal(ctx, id); err != nil {
		return false, fmt.Errorf("failed to delete goal: %w", err)
	}
	return true, nil
}

// CreateHostnameRule creates a new hostname rule.
func (r *Resolver) CreateHostnameRule(ctx context.Context, propertyID, pattern string) (*HostnameRuleResolver, error) {
	if _, err := r.validatePropertyAccess(ctx, propertyID); err != nil {
		return nil, err
	}

	rule, err := r.queryService.CreateHostnameRule(ctx, propertyID, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create hostname rule: %w", err)
	}

	return &HostnameRuleResolver{rule: rule}, nil
}

// DeleteHostnameRule deletes a hostname rule.
func (r *Resolver) DeleteHostnameRule(ctx context.Context, id string) (bool, error) {
	rule, err := r.queryService.GetHostnameRule(ctx, id)
	if err != nil {
		return false, fmt.Errorf("hostname rule not found")
	}
	if _, err := r.validatePropertyAccess(ctx, rule.PropertyID); err != nil {
		return false, err
	}

	if err := r.queryService.DeleteHostnameRule(ctx, id); err != nil {
		return false, fmt.Errorf("failed to delete hostname rule: %w", err)
	}
	return true, nil
}

// =============================================================================
// Helpers
// =============================================================================

func (r *Resolver) validatePropertyAccess(ctx context.Context, propertyID string) (*analyticsdomain.Property, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	prop, err := r.queryService.GetProperty(ctx, propertyID)
	if err != nil {
		return nil, fmt.Errorf("property not found")
	}

	if err := graphshared.ValidateWorkspaceOwnership(prop.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("property not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, prop.WorkspaceID); err != nil {
		return nil, err
	}

	return prop, nil
}

func (r *Resolver) ensureSurfaceEnabled(ctx context.Context) error {
	if r == nil || r.extensionChecker == nil {
		return nil
	}
	enabled, err := r.extensionChecker.HasActiveExtension(ctx, "web-analytics")
	if err != nil {
		return fmt.Errorf("failed to resolve analytics extension state: %w", err)
	}
	if !enabled {
		return fmt.Errorf("web-analytics is not active")
	}
	return nil
}

func (r *Resolver) ensureWorkspaceEnabled(ctx context.Context, workspaceID string) error {
	if r == nil || r.extensionChecker == nil {
		return nil
	}
	enabled, err := r.extensionChecker.HasActiveExtensionInWorkspace(ctx, workspaceID, "web-analytics")
	if err != nil {
		return fmt.Errorf("failed to resolve analytics extension state: %w", err)
	}
	if !enabled {
		return fmt.Errorf("web-analytics is not active for workspace")
	}
	return nil
}

// =============================================================================
// Type Resolvers
// =============================================================================

// AnalyticsPropertyResolver resolves AnalyticsProperty fields.
type AnalyticsPropertyResolver struct {
	property *analyticsdomain.Property
	r        *Resolver
}

func (p *AnalyticsPropertyResolver) ID() graphql.ID   { return graphql.ID(p.property.ID) }
func (p *AnalyticsPropertyResolver) Domain() string   { return p.property.Domain }
func (p *AnalyticsPropertyResolver) Timezone() string { return p.property.Timezone }
func (p *AnalyticsPropertyResolver) Status() string   { return p.property.Status }
func (p *AnalyticsPropertyResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: p.property.CreatedAt}
}

func (p *AnalyticsPropertyResolver) SnippetHtml() string {
	return p.property.SnippetHTML(p.r.apiBaseURL)
}

func (p *AnalyticsPropertyResolver) VerifiedAt() *graphshared.DateTime {
	if p.property.VerifiedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *p.property.VerifiedAt}
}

func (p *AnalyticsPropertyResolver) CurrentVisitors(ctx context.Context) (int32, error) {
	count, err := p.r.queryService.GetCurrentVisitors(ctx, p.property.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get current visitors: %w", err)
	}
	return int32(count), nil
}

func (p *AnalyticsPropertyResolver) VisitorsLast24h(ctx context.Context) (int32, error) {
	count, err := p.r.queryService.GetVisitorsLast24h(ctx, p.property.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get visitors last 24h: %w", err)
	}
	return int32(count), nil
}

// AnalyticsMetricsResolver resolves AnalyticsMetrics fields.
type AnalyticsMetricsResolver struct {
	metrics *analyticsservices.MetricsWithChange
}

func (m *AnalyticsMetricsResolver) UniqueVisitors() int32 {
	return int32(m.metrics.Current.UniqueVisitors)
}
func (m *AnalyticsMetricsResolver) TotalVisits() int32 { return int32(m.metrics.Current.TotalVisits) }
func (m *AnalyticsMetricsResolver) TotalPageviews() int32 {
	return int32(m.metrics.Current.TotalPageviews)
}
func (m *AnalyticsMetricsResolver) ViewsPerVisit() float64 { return m.metrics.Current.ViewsPerVisit }
func (m *AnalyticsMetricsResolver) BounceRate() float64    { return m.metrics.Current.BounceRate }
func (m *AnalyticsMetricsResolver) AvgVisitDuration() int32 {
	return int32(m.metrics.Current.AvgVisitDuration)
}

func (m *AnalyticsMetricsResolver) UniqueVisitorsChange() *float64 {
	return percentChange(m.metrics.Previous.UniqueVisitors, m.metrics.Current.UniqueVisitors)
}
func (m *AnalyticsMetricsResolver) TotalVisitsChange() *float64 {
	return percentChange(m.metrics.Previous.TotalVisits, m.metrics.Current.TotalVisits)
}
func (m *AnalyticsMetricsResolver) TotalPageviewsChange() *float64 {
	return percentChange(m.metrics.Previous.TotalPageviews, m.metrics.Current.TotalPageviews)
}
func (m *AnalyticsMetricsResolver) ViewsPerVisitChange() *float64 {
	return percentChangeFloat(m.metrics.Previous.ViewsPerVisit, m.metrics.Current.ViewsPerVisit)
}
func (m *AnalyticsMetricsResolver) BounceRateChange() *float64 {
	return percentChangeFloat(m.metrics.Previous.BounceRate, m.metrics.Current.BounceRate)
}
func (m *AnalyticsMetricsResolver) AvgVisitDurationChange() *float64 {
	return percentChange(m.metrics.Previous.AvgVisitDuration, m.metrics.Current.AvgVisitDuration)
}

// AnalyticsTimeSeriesResolver resolves AnalyticsTimeSeries fields.
type AnalyticsTimeSeriesResolver struct {
	row *analyticsdomain.TimeSeriesPoint
}

func (t *AnalyticsTimeSeriesResolver) Date() string     { return t.row.Date }
func (t *AnalyticsTimeSeriesResolver) Visitors() int32  { return int32(t.row.Visitors) }
func (t *AnalyticsTimeSeriesResolver) Pageviews() int32 { return int32(t.row.Pageviews) }

// AnalyticsBreakdownRowResolver resolves AnalyticsBreakdownRow fields.
type AnalyticsBreakdownRowResolver struct {
	row *analyticsdomain.BreakdownRow
}

func (b *AnalyticsBreakdownRowResolver) Name() string    { return b.row.Name }
func (b *AnalyticsBreakdownRowResolver) Visitors() int32 { return int32(b.row.Visitors) }
func (b *AnalyticsBreakdownRowResolver) Pageviews() *int32 {
	if b.row.Pageviews == nil {
		return nil
	}
	v := int32(*b.row.Pageviews)
	return &v
}

// AnalyticsGoalResolver resolves AnalyticsGoal fields.
type AnalyticsGoalResolver struct {
	goal *analyticsdomain.Goal
}

func (g *AnalyticsGoalResolver) ID() graphql.ID   { return graphql.ID(g.goal.ID) }
func (g *AnalyticsGoalResolver) Name() string     { return g.goal.DisplayName() }
func (g *AnalyticsGoalResolver) GoalType() string { return g.goal.GoalType }

func (g *AnalyticsGoalResolver) EventName() *string {
	if g.goal.EventName == "" {
		return nil
	}
	return &g.goal.EventName
}

func (g *AnalyticsGoalResolver) PagePath() *string {
	if g.goal.PagePath == "" {
		return nil
	}
	return &g.goal.PagePath
}

// AnalyticsGoalResultResolver resolves AnalyticsGoalResult fields.
type AnalyticsGoalResultResolver struct {
	goal   *analyticsdomain.Goal
	result *analyticsdomain.GoalResult
}

func (r *AnalyticsGoalResultResolver) Goal() *AnalyticsGoalResolver {
	return &AnalyticsGoalResolver{goal: r.goal}
}
func (r *AnalyticsGoalResultResolver) Uniques() int32          { return int32(r.result.Uniques) }
func (r *AnalyticsGoalResultResolver) Total() int32            { return int32(r.result.Total) }
func (r *AnalyticsGoalResultResolver) ConversionRate() float64 { return r.result.ConversionRate }

// HostnameRuleResolver resolves HostnameRule fields.
type HostnameRuleResolver struct {
	rule *analyticsdomain.HostnameRule
}

func (h *HostnameRuleResolver) ID() graphql.ID  { return graphql.ID(h.rule.ID) }
func (h *HostnameRuleResolver) Pattern() string { return h.rule.Pattern }
func (h *HostnameRuleResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: h.rule.CreatedAt}
}

// =============================================================================
// Helpers
// =============================================================================

func percentChange(previous, current int) *float64 {
	if previous == 0 {
		if current == 0 {
			return nil
		}
		v := 100.0
		return &v
	}
	v := float64(current-previous) / float64(previous) * 100.0
	return &v
}

func percentChangeFloat(previous, current float64) *float64 {
	if previous == 0 {
		if current == 0 {
			return nil
		}
		v := 100.0
		return &v
	}
	v := (current - previous) / previous * 100.0
	return &v
}
