package analyticsservices

import (
	"context"
	"fmt"
	"time"

	analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"
)

// QueryService handles all analytics dashboard queries.
type QueryService struct {
	store QueryStore
}

// NewQueryService creates a new analytics query service.
func NewQueryService(store QueryStore) *QueryService {
	return &QueryService{store: store}
}

// --- Property management ---

func (s *QueryService) GetProperty(ctx context.Context, propertyID string) (*analyticsdomain.Property, error) {
	return s.store.GetProperty(ctx, propertyID)
}

func (s *QueryService) ListProperties(ctx context.Context, workspaceID string) ([]*analyticsdomain.Property, error) {
	return s.store.ListPropertiesByWorkspace(ctx, workspaceID)
}

func (s *QueryService) ListAllProperties(ctx context.Context) ([]*analyticsdomain.Property, error) {
	return s.store.ListAllProperties(ctx)
}

func (s *QueryService) CreateProperty(ctx context.Context, workspaceID, domain, timezone string) (*analyticsdomain.Property, error) {
	prop, err := analyticsdomain.NewProperty(workspaceID, domain, timezone)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateProperty(ctx, prop); err != nil {
		return nil, err
	}
	return prop, nil
}

func (s *QueryService) UpdateProperty(ctx context.Context, prop *analyticsdomain.Property) error {
	return s.store.UpdateProperty(ctx, prop)
}

func (s *QueryService) DeleteProperty(ctx context.Context, propertyID string) error {
	return s.store.DeleteProperty(ctx, propertyID)
}

func (s *QueryService) ResetPropertyStats(ctx context.Context, propertyID string) error {
	return s.store.ResetPropertyStats(ctx, propertyID)
}

// --- Goals ---

func (s *QueryService) CreateGoal(ctx context.Context, propertyID, goalType, eventName, pagePath string) (*analyticsdomain.Goal, error) {
	count, err := s.store.CountGoalsByProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	if err := analyticsdomain.ValidateGoalCount(count); err != nil {
		return nil, err
	}

	goal, err := analyticsdomain.NewGoal(propertyID, goalType, eventName, pagePath)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateGoal(ctx, goal); err != nil {
		return nil, err
	}
	return goal, nil
}

func (s *QueryService) DeleteGoal(ctx context.Context, goalID string) error {
	return s.store.DeleteGoal(ctx, goalID)
}

func (s *QueryService) GetGoal(ctx context.Context, goalID string) (*analyticsdomain.Goal, error) {
	return s.store.GetGoal(ctx, goalID)
}

func (s *QueryService) ListGoals(ctx context.Context, propertyID string) ([]*analyticsdomain.Goal, error) {
	return s.store.ListGoalsByProperty(ctx, propertyID)
}

// --- Hostname Rules ---

func (s *QueryService) CreateHostnameRule(ctx context.Context, propertyID, pattern string) (*analyticsdomain.HostnameRule, error) {
	count, err := s.store.CountHostnameRulesByProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	if err := analyticsdomain.ValidateHostnameRuleCount(count); err != nil {
		return nil, err
	}

	rule, err := analyticsdomain.NewHostnameRule(propertyID, pattern)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateHostnameRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *QueryService) GetHostnameRule(ctx context.Context, ruleID string) (*analyticsdomain.HostnameRule, error) {
	return s.store.GetHostnameRule(ctx, ruleID)
}

func (s *QueryService) DeleteHostnameRule(ctx context.Context, ruleID string) error {
	return s.store.DeleteHostnameRule(ctx, ruleID)
}

func (s *QueryService) ListHostnameRules(ctx context.Context, propertyID string) ([]*analyticsdomain.HostnameRule, error) {
	return s.store.ListHostnameRulesByProperty(ctx, propertyID)
}

// --- Dashboard queries ---

// PeriodRange resolves an analytics period enum to from/to timestamps.
func PeriodRange(period, timezone string, customFrom, customTo *time.Time) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	switch period {
	case "TODAY":
		return today.UTC(), now.UTC(), nil
	case "YESTERDAY":
		yesterday := today.AddDate(0, 0, -1)
		return yesterday.UTC(), today.UTC(), nil
	case "LAST_7_DAYS":
		return today.AddDate(0, 0, -7).UTC(), now.UTC(), nil
	case "LAST_28_DAYS":
		return today.AddDate(0, 0, -28).UTC(), now.UTC(), nil
	case "THIS_MONTH":
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		return firstOfMonth.UTC(), now.UTC(), nil
	case "LAST_MONTH":
		firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)
		return firstOfLastMonth.UTC(), firstOfThisMonth.UTC(), nil
	case "LAST_6_MONTHS":
		return today.AddDate(0, -6, 0).UTC(), now.UTC(), nil
	case "LAST_12_MONTHS":
		return today.AddDate(-1, 0, 0).UTC(), now.UTC(), nil
	case "CUSTOM":
		if customFrom != nil && customTo != nil {
			return customFrom.UTC(), customTo.UTC(), nil
		}
		return time.Time{}, time.Time{}, fmt.Errorf("custom period requires from and to")
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unknown period: %s", period)
	}
}

// PreviousPeriodRange computes the equivalent prior period for % change calculation.
func PreviousPeriodRange(from, to time.Time) (time.Time, time.Time) {
	duration := to.Sub(from)
	return from.Add(-duration), from
}

// MetricsWithChange holds current metrics and their % change vs the previous period.
type MetricsWithChange struct {
	Current  *analyticsdomain.Metrics
	Previous *analyticsdomain.Metrics
}

func (s *QueryService) GetMetrics(ctx context.Context, propertyID, period, timezone string, customFrom, customTo *time.Time) (*MetricsWithChange, error) {
	from, to, err := PeriodRange(period, timezone, customFrom, customTo)
	if err != nil {
		return nil, err
	}

	current, err := s.store.GetMetrics(ctx, propertyID, from, to)
	if err != nil {
		return nil, err
	}

	prevFrom, prevTo := PreviousPeriodRange(from, to)
	previous, err := s.store.GetMetrics(ctx, propertyID, prevFrom, prevTo)
	if err != nil {
		return nil, err
	}

	return &MetricsWithChange{Current: current, Previous: previous}, nil
}

func (s *QueryService) GetTimeSeries(ctx context.Context, propertyID, period, interval, timezone string, customFrom, customTo *time.Time) ([]*analyticsdomain.TimeSeriesPoint, error) {
	from, to, err := PeriodRange(period, timezone, customFrom, customTo)
	if err != nil {
		return nil, err
	}
	return s.store.GetTimeSeries(ctx, propertyID, from, to, interval)
}

func (s *QueryService) GetBreakdown(ctx context.Context, propertyID, period, dimension, timezone string, limit int, customFrom, customTo *time.Time) ([]*analyticsdomain.BreakdownRow, error) {
	from, to, err := PeriodRange(period, timezone, customFrom, customTo)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	return s.store.GetBreakdown(ctx, propertyID, from, to, dimension, limit)
}

func (s *QueryService) GetGoalResults(ctx context.Context, propertyID, period, timezone string, customFrom, customTo *time.Time) ([]*analyticsdomain.GoalResult, error) {
	from, to, err := PeriodRange(period, timezone, customFrom, customTo)
	if err != nil {
		return nil, err
	}
	return s.store.GetGoalResults(ctx, propertyID, from, to)
}

func (s *QueryService) GetCurrentVisitors(ctx context.Context, propertyID string) (int, error) {
	return s.store.GetCurrentVisitors(ctx, propertyID)
}

func (s *QueryService) GetVisitorsLast24h(ctx context.Context, propertyID string) (int, error) {
	return s.store.GetVisitorsLast24h(ctx, propertyID)
}

func (s *QueryService) VerifyInstallation(ctx context.Context, propertyID string) (bool, error) {
	return s.store.HasEventsForProperty(ctx, propertyID)
}
