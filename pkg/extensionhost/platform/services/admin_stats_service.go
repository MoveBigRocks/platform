package platformservices

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/metrics"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/pkg/logger"
)

// AdminStatsService provides aggregated statistics for the admin dashboard
// PERFORMANCE: Uses Prometheus metrics when available (fast, pre-aggregated).
// Otherwise calculates stats by scanning storage (slower, for dev/testing).
type AdminStatsService struct {
	store            shared.Store
	logger           *logger.Logger
	prometheusClient *metrics.PrometheusClient
	usePrometheus    bool // If true, use pre-aggregated Prometheus metrics; if false, calculate from storage

	// Trend tracking for quick stats
	previousStats map[string]float64
	extensionGate adminStatsExtensionGate
}

type adminStatsExtensionGate interface {
	HasActiveExtension(ctx context.Context, slug string) (bool, error)
	HasActiveExtensionInWorkspace(ctx context.Context, workspaceID, slug string) (bool, error)
}

// NewAdminStatsService creates a new admin statistics service
// prometheusURL should be the base URL of the Prometheus server (e.g., "http://prometheus:9090")
// If empty, defaults to "http://prometheus:9090"
func NewAdminStatsService(store shared.Store, log *logger.Logger, prometheusURL string) *AdminStatsService {
	if log == nil {
		log = logger.New()
	}

	// Initialize Prometheus client
	promClient := metrics.NewPrometheusClient(prometheusURL)

	// Test if Prometheus is available by querying a simple metric
	ctx := context.Background()
	usePrometheus := false
	_, err := promClient.Query(ctx, "up")
	if err == nil {
		usePrometheus = true
		log.Info("Prometheus metrics available, will use for analytics")
	} else {
		log.WithError(err).Warn("Prometheus not available, will calculate stats from storage (slower)")
	}

	return &AdminStatsService{
		store:            store,
		logger:           log,
		prometheusClient: promClient,
		usePrometheus:    usePrometheus,
		previousStats:    make(map[string]float64),
	}
}

func (s *AdminStatsService) SetExtensionGate(gate adminStatsExtensionGate) {
	s.extensionGate = gate
}

// DashboardStats represents the key metrics displayed on the admin dashboard
type DashboardStats struct {
	WorkspaceCount    int
	UserCount         int
	OpenCases         int
	AvgResolutionTime string // human-readable duration
}

// RecentActivity represents a recent activity item
type RecentActivity struct {
	Type        string  // "case_created", "error_spike", "workspace_created", "user_invited"
	Description string  // Human-readable description
	WorkspaceID *string // Related workspace (if applicable)
	Timestamp   time.Time
	Icon        string // Icon identifier for UI
	Severity    string // "info", "warning", "error"
}

// SystemHealth represents the health status of a system component
type SystemHealth struct {
	Component string // "storage", "workers", "api"
	Status    string // "healthy", "degraded", "down"
	Message   string
	Details   map[string]interface{}
}

// QuickStat represents a quick statistic for the dashboard
type QuickStat struct {
	Label string
	Value string
	Icon  string
	Trend string // "up", "down", "stable"
}

// GetDashboardStats retrieves all key metrics for the dashboard
func (s *AdminStatsService) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Get workspace count (from Prometheus gauge or calculate from storage)
	if s.usePrometheus {
		activeWorkspaces, err := s.prometheusClient.GetActiveWorkspaces(ctx)
		if err == nil {
			stats.WorkspaceCount = int(activeWorkspaces)
		} else {
			s.logger.WithError(err).Warn("Failed to get workspace count from Prometheus, calculating from storage")
			stats.WorkspaceCount = s.getWorkspaceCountFromStore(ctx)
		}
	} else {
		stats.WorkspaceCount = s.getWorkspaceCountFromStore(ctx)
	}

	// Get user count (no Prometheus metric for this)
	users, err := s.store.Users().ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user count: %w", err)
	}
	stats.UserCount = len(users)

	// Get case statistics (no Prometheus metric yet)
	stats.OpenCases, stats.AvgResolutionTime = s.getCaseStats(ctx)

	return stats, nil
}

// Helper: Get workspace count by scanning storage
func (s *AdminStatsService) getWorkspaceCountFromStore(ctx context.Context) int {
	workspaces, err := s.store.Workspaces().ListWorkspaces(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get workspace count from store")
		return 0
	}
	return len(workspaces)
}

// Helper: Get case statistics by scanning storage
func (s *AdminStatsService) getCaseStats(ctx context.Context) (int, string) {
	allCases, _, err := s.store.Cases().ListCases(ctx, shared.CaseFilters{
		Limit: 10000, // Get all cases
	})
	if err != nil {
		return 0, "N/A"
	}

	openCount := 0
	var totalResolutionTime time.Duration
	var resolvedCount int

	for _, c := range allCases {
		if c.Status != "resolved" && c.Status != "ignored" {
			openCount++
		}
		if c.Status == "resolved" && c.ResolvedAt != nil {
			resolvedCount++
			totalResolutionTime += c.ResolvedAt.Sub(c.CreatedAt)
		}
	}

	// Calculate average resolution time
	avgResolutionTime := "N/A"
	if resolvedCount > 0 {
		avgDuration := totalResolutionTime / time.Duration(resolvedCount)
		if avgDuration < time.Hour {
			avgResolutionTime = fmt.Sprintf("%d minutes", int(avgDuration.Minutes()))
		} else if avgDuration < 24*time.Hour {
			avgResolutionTime = fmt.Sprintf("%.1f hours", avgDuration.Hours())
		} else {
			avgResolutionTime = fmt.Sprintf("%.1f days", avgDuration.Hours()/24)
		}
	}

	return openCount, avgResolutionTime
}

func (s *AdminStatsService) isSurfaceEnabled(ctx context.Context, workspaceID, slug string) bool {
	if s == nil || s.extensionGate == nil {
		return true
	}

	var (
		enabled bool
		err     error
	)
	if strings.TrimSpace(workspaceID) != "" {
		enabled, err = s.extensionGate.HasActiveExtensionInWorkspace(ctx, workspaceID, slug)
	} else {
		enabled, err = s.extensionGate.HasActiveExtension(ctx, slug)
	}
	if err != nil {
		s.logger.WithError(err).WithField("extension_slug", slug).Warn("Failed to resolve extension surface state")
		return false
	}
	return enabled
}

// GetRecentActivity retrieves the most recent activities across the platform
func (s *AdminStatsService) GetRecentActivity(ctx context.Context, limit int) ([]RecentActivity, error) {
	if limit == 0 {
		limit = 10
	}

	var activities []RecentActivity

	// Get recent cases created
	cases, _, err := s.store.Cases().ListCases(ctx, shared.CaseFilters{
		Limit: limit,
	})
	if err == nil {
		for _, c := range cases {
			// Get workspace name (ignore errors: use fallback on failure)
			workspace, _ := s.store.Workspaces().GetWorkspace(ctx, c.WorkspaceID) //nolint:errcheck
			workspaceName := "Unknown"
			if workspace != nil {
				workspaceName = workspace.Name
			}

			severityStr := "info"
			if c.Priority == "urgent" || c.Priority == "high" {
				severityStr = "error"
			} else if c.Priority == "medium" {
				severityStr = "warning"
			}

			activities = append(activities, RecentActivity{
				Type:        "case_created",
				Description: fmt.Sprintf("Case in %s: %s", workspaceName, c.Subject),
				WorkspaceID: &c.WorkspaceID,
				Timestamp:   c.CreatedAt,
				Icon:        "lucide--alert-circle",
				Severity:    severityStr,
			})
		}
	}

	// Get recent workspaces created
	workspaces, err := s.store.Workspaces().ListWorkspaces(ctx)
	if err == nil {
		// Take the 5 most recent workspaces
		startIdx := len(workspaces) - 5
		if startIdx < 0 {
			startIdx = 0
		}
		for i := startIdx; i < len(workspaces); i++ {
			w := workspaces[i]
			activities = append(activities, RecentActivity{
				Type:        "workspace_created",
				Description: fmt.Sprintf("Workspace created: %s", w.Name),
				WorkspaceID: &w.ID,
				Timestamp:   w.CreatedAt,
				Icon:        "lucide--building-2",
				Severity:    "info",
			})
		}
	}

	// Sort by timestamp (most recent first)
	sortActivitiesByTimestamp(activities)

	// Limit to requested amount
	if len(activities) > limit {
		activities = activities[:limit]
	}

	return activities, nil
}

// GetQuickStats retrieves quick statistics for the dashboard (Phase 3 Implementation)
func (s *AdminStatsService) GetQuickStats(ctx context.Context) ([]QuickStat, error) {
	var stats []QuickStat

	// Open Cases
	openCases, _ := s.getCaseStats(ctx)
	openCasesTrend := s.calculateTrend("open_cases", float64(openCases))
	stats = append(stats, QuickStat{
		Label: "Open Cases",
		Value: fmt.Sprintf("%d", openCases),
		Icon:  "lucide--inbox",
		Trend: openCasesTrend,
	})

	// Active Workspaces
	workspaceCount := s.getWorkspaceCountFromStore(ctx)
	workspaceTrend := s.calculateTrend("workspaces", float64(workspaceCount))
	stats = append(stats, QuickStat{
		Label: "Workspaces",
		Value: fmt.Sprintf("%d", workspaceCount),
		Icon:  "lucide--building-2",
		Trend: workspaceTrend,
	})

	// Average Resolution Time
	_, avgResolutionTime := s.getCaseStats(ctx)
	stats = append(stats, QuickStat{
		Label: "Avg Resolution",
		Value: avgResolutionTime,
		Icon:  "lucide--clock-3",
		Trend: "stable",
	})

	// Active Users (users who logged in last 24h)
	activeUsers := s.getActiveUsersCount(ctx)
	activeUsersTrend := s.calculateTrend("active_users", float64(activeUsers))
	stats = append(stats, QuickStat{
		Label: "Active Users",
		Value: fmt.Sprintf("%d", activeUsers),
		Icon:  "lucide--users",
		Trend: activeUsersTrend,
	})

	// Jobs Processed (last hour)
	jobsProcessed := s.getJobsProcessedCount(ctx)
	jobsTrend := s.calculateTrend("jobs_processed", float64(jobsProcessed))
	stats = append(stats, QuickStat{
		Label: "Jobs/Hour",
		Value: fmt.Sprintf("%d", jobsProcessed),
		Icon:  "lucide--zap",
		Trend: jobsTrend,
	})

	// Alerts Triggered (last 24h)
	alertsTriggered := s.getAlertsTriggeredCount(ctx)
	alertsTrend := s.calculateTrend("alerts_triggered", float64(alertsTriggered))
	trendIcon := "lucide--bell"
	if alertsTriggered > 10 {
		trendIcon = "lucide--bell-ring"
	}
	stats = append(stats, QuickStat{
		Label: "Alerts (24h)",
		Value: fmt.Sprintf("%d", alertsTriggered),
		Icon:  trendIcon,
		Trend: alertsTrend,
	})

	return stats, nil
}

// calculateTrend compares current value with previous and returns trend direction
func (s *AdminStatsService) calculateTrend(key string, currentValue float64) string {
	previousValue, exists := s.previousStats[key]
	s.previousStats[key] = currentValue

	if !exists {
		return "stable"
	}

	// Calculate percentage change
	if previousValue == 0 {
		if currentValue > 0 {
			return "up"
		}
		return "stable"
	}

	change := (currentValue - previousValue) / previousValue * 100

	if change > 5 {
		return "up"
	} else if change < -5 {
		return "down"
	}
	return "stable"
}

// getActiveUsersCount returns count of users active in last 24 hours
func (s *AdminStatsService) getActiveUsersCount(ctx context.Context) int {
	users, err := s.store.Users().ListUsers(ctx)
	if err != nil {
		return 0
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	activeCount := 0
	for _, user := range users {
		if user.LastLoginAt != nil && user.LastLoginAt.After(cutoff) {
			activeCount++
		}
	}
	return activeCount
}

// getJobsProcessedCount returns count of jobs processed in last hour
func (s *AdminStatsService) getJobsProcessedCount(ctx context.Context) int {
	if s.usePrometheus {
		// Try to get from Prometheus
		result, err := s.prometheusClient.Query(ctx, "increase(mbr_jobs_processed_total[1h])")
		if err == nil {
			if val, err := s.prometheusClient.GetScalarValue(result); err == nil && val > 0 {
				return int(val)
			}
		}
	}

	// Fallback: scan storage for recently completed jobs
	// This is expensive, so we limit to a rough estimate
	workspaces, err := s.store.Workspaces().ListWorkspaces(ctx)
	if err != nil {
		return 0
	}

	jobCount := 0
	cutoff := time.Now().Add(-1 * time.Hour)

	// Limit workspace scan
	limit := len(workspaces)
	if limit > 10 {
		limit = 10
	}

	for i := 0; i < limit; i++ {
		jobs, _, err := s.store.Jobs().ListWorkspaceJobs(ctx, workspaces[i].ID, "completed", "", 100, 0)
		if err != nil {
			continue
		}
		for _, job := range jobs {
			if job.CompletedAt != nil && job.CompletedAt.After(cutoff) {
				jobCount++
			}
		}
	}

	return jobCount
}

// getAlertsTriggeredCount returns count of alerts triggered in last 24 hours
func (s *AdminStatsService) getAlertsTriggeredCount(ctx context.Context) int {
	if s.usePrometheus {
		// Try to get from Prometheus
		result, err := s.prometheusClient.Query(ctx, "increase(mbr_alerts_triggered_total[24h])")
		if err == nil {
			if val, err := s.prometheusClient.GetScalarValue(result); err == nil && val > 0 {
				return int(val)
			}
		}
	}

	// Fallback: not easily calculable from storage without alert history
	// Return 0 as a safe default
	return 0
}

// Helper functions

func sortActivitiesByTimestamp(activities []RecentActivity) {
	// Sort by timestamp descending (most recent first) using built-in sort
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp.After(activities[j].Timestamp)
	})
}
