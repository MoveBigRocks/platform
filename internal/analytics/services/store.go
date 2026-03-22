package analyticsservices

import (
	"context"
	"time"

	analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"
)

// QueryStore contains the analytics persistence operations needed by dashboard queries.
type QueryStore interface {
	GetProperty(ctx context.Context, propertyID string) (*analyticsdomain.Property, error)
	ListPropertiesByWorkspace(ctx context.Context, workspaceID string) ([]*analyticsdomain.Property, error)
	ListAllProperties(ctx context.Context) ([]*analyticsdomain.Property, error)
	CreateProperty(ctx context.Context, p *analyticsdomain.Property) error
	UpdateProperty(ctx context.Context, p *analyticsdomain.Property) error
	DeleteProperty(ctx context.Context, propertyID string) error
	ResetPropertyStats(ctx context.Context, propertyID string) error
	CountGoalsByProperty(ctx context.Context, propertyID string) (int, error)
	CreateGoal(ctx context.Context, g *analyticsdomain.Goal) error
	DeleteGoal(ctx context.Context, goalID string) error
	GetGoal(ctx context.Context, goalID string) (*analyticsdomain.Goal, error)
	ListGoalsByProperty(ctx context.Context, propertyID string) ([]*analyticsdomain.Goal, error)
	CountHostnameRulesByProperty(ctx context.Context, propertyID string) (int, error)
	CreateHostnameRule(ctx context.Context, r *analyticsdomain.HostnameRule) error
	GetHostnameRule(ctx context.Context, ruleID string) (*analyticsdomain.HostnameRule, error)
	DeleteHostnameRule(ctx context.Context, ruleID string) error
	ListHostnameRulesByProperty(ctx context.Context, propertyID string) ([]*analyticsdomain.HostnameRule, error)
	GetMetrics(ctx context.Context, propertyID string, from, to time.Time) (*analyticsdomain.Metrics, error)
	GetTimeSeries(ctx context.Context, propertyID string, from, to time.Time, interval string) ([]*analyticsdomain.TimeSeriesPoint, error)
	GetBreakdown(ctx context.Context, propertyID string, from, to time.Time, dimension string, limit int) ([]*analyticsdomain.BreakdownRow, error)
	GetGoalResults(ctx context.Context, propertyID string, from, to time.Time) ([]*analyticsdomain.GoalResult, error)
	GetCurrentVisitors(ctx context.Context, propertyID string) (int, error)
	GetVisitorsLast24h(ctx context.Context, propertyID string) (int, error)
	HasEventsForProperty(ctx context.Context, propertyID string) (bool, error)
}

// IngestStore contains the analytics persistence operations needed by ingest processing.
type IngestStore interface {
	GetPropertyByDomain(ctx context.Context, domain string) (*analyticsdomain.Property, error)
	ListAllProperties(ctx context.Context) ([]*analyticsdomain.Property, error)
	ListHostnameRulesByProperty(ctx context.Context, propertyID string) ([]*analyticsdomain.HostnameRule, error)
	GetCurrentSalts(ctx context.Context) ([]*analyticsdomain.Salt, error)
	FindRecentSession(ctx context.Context, propertyID string, visitorIDs []int64, cutoff time.Time) (*analyticsdomain.Session, error)
	UpdateSession(ctx context.Context, sess *analyticsdomain.Session) error
	InsertSession(ctx context.Context, sess *analyticsdomain.Session) error
	InsertEvent(ctx context.Context, e *analyticsdomain.AnalyticsEvent) error
	UpdateProperty(ctx context.Context, p *analyticsdomain.Property) error
}
