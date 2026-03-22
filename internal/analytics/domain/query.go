package analyticsdomain

// Metrics captures aggregate dashboard metrics for a reporting window.
type Metrics struct {
	UniqueVisitors   int
	TotalVisits      int
	TotalPageviews   int
	ViewsPerVisit    float64
	BounceRate       float64
	AvgVisitDuration int
}

// TimeSeriesPoint captures analytics time-series output for dashboard charts.
type TimeSeriesPoint struct {
	Date      string
	Visitors  int
	Pageviews int
}

// BreakdownRow captures analytics breakdown output for grouped dashboard views.
type BreakdownRow struct {
	Name      string
	Visitors  int
	Pageviews *int
}

// GoalResult captures computed goal performance for a reporting window.
type GoalResult struct {
	GoalID         string
	Uniques        int
	Total          int
	ConversionRate float64
}
