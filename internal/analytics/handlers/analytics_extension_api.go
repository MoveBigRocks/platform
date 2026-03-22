package analyticshandlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"
	analyticsservices "github.com/movebigrocks/platform/internal/analytics/services"
)

const defaultAnalyticsPeriod = "LAST_28_DAYS"

// AnalyticsExtensionAPIHandler exposes extension-owned JSON APIs for the web analytics pack.
// These endpoints replace the last core GraphQL dependency for the analytics admin UI.
type AnalyticsExtensionAPIHandler struct {
	query      *analyticsservices.QueryService
	apiBaseURL string
}

func NewAnalyticsExtensionAPIHandler(query *analyticsservices.QueryService, apiBaseURL string) *AnalyticsExtensionAPIHandler {
	if query == nil {
		return nil
	}
	return &AnalyticsExtensionAPIHandler{
		query:      query,
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
	}
}

func (h *AnalyticsExtensionAPIHandler) ListProperties(c *gin.Context) {
	workspaceID, ok := analyticsWorkspaceID(c)
	if !ok {
		return
	}
	properties, err := h.query.ListProperties(c.Request.Context(), workspaceID)
	if err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	result := make([]gin.H, 0, len(properties))
	for _, property := range properties {
		if property == nil {
			continue
		}
		visitorsLast24h, err := h.query.GetVisitorsLast24h(c.Request.Context(), property.ID)
		if err != nil {
			analyticsError(c, http.StatusInternalServerError, err)
			return
		}
		result = append(result, h.serializeProperty(property, visitorsLast24h, nil))
	}
	c.JSON(http.StatusOK, gin.H{"properties": result})
}

func (h *AnalyticsExtensionAPIHandler) CreateProperty(c *gin.Context) {
	workspaceID, ok := analyticsWorkspaceID(c)
	if !ok {
		return
	}
	var req struct {
		Domain   string `json:"domain"`
		Timezone string `json:"timezone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	property, err := h.query.CreateProperty(c.Request.Context(), workspaceID, strings.TrimSpace(req.Domain), strings.TrimSpace(req.Timezone))
	if err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"property": h.serializeProperty(property, 0, nil)})
}

func (h *AnalyticsExtensionAPIHandler) GetProperty(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	currentVisitors, err := h.query.GetCurrentVisitors(c.Request.Context(), property.ID)
	if err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	visitorsLast24h, err := h.query.GetVisitorsLast24h(c.Request.Context(), property.ID)
	if err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"property": h.serializeProperty(property, visitorsLast24h, &currentVisitors),
	})
}

func (h *AnalyticsExtensionAPIHandler) UpdateProperty(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	var req struct {
		Domain   *string `json:"domain"`
		Timezone *string `json:"timezone"`
		Status   *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	if req.Domain != nil {
		property.Domain = strings.TrimSpace(*req.Domain)
	}
	if req.Timezone != nil {
		property.Timezone = strings.TrimSpace(*req.Timezone)
	}
	if req.Status != nil {
		property.Status = strings.TrimSpace(*req.Status)
	}
	property.UpdatedAt = time.Now().UTC()
	if err := h.query.UpdateProperty(c.Request.Context(), property); err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"property": h.serializeProperty(property, 0, nil)})
}

func (h *AnalyticsExtensionAPIHandler) DeleteProperty(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	if err := h.query.DeleteProperty(c.Request.Context(), property.ID); err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *AnalyticsExtensionAPIHandler) ResetProperty(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	if err := h.query.ResetPropertyStats(c.Request.Context(), property.ID); err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"property": h.serializeProperty(property, 0, nil)})
}

func (h *AnalyticsExtensionAPIHandler) CurrentVisitors(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	count, err := h.query.GetCurrentVisitors(c.Request.Context(), property.ID)
	if err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *AnalyticsExtensionAPIHandler) VerifyInstallation(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	verified, err := h.query.VerifyInstallation(c.Request.Context(), property.ID)
	if err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"verified": verified})
}

func (h *AnalyticsExtensionAPIHandler) Metrics(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	period := analyticsQuery(c, "period", defaultAnalyticsPeriod)
	metrics, err := h.query.GetMetrics(c.Request.Context(), property.ID, period, property.Timezone, nil, nil)
	if err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"metrics": gin.H{
			"uniqueVisitors":       metrics.Current.UniqueVisitors,
			"totalVisits":          metrics.Current.TotalVisits,
			"totalPageviews":       metrics.Current.TotalPageviews,
			"viewsPerVisit":        metrics.Current.ViewsPerVisit,
			"bounceRate":           metrics.Current.BounceRate,
			"avgVisitDuration":     metrics.Current.AvgVisitDuration,
			"uniqueVisitorsChange": analyticsPercentChange(metrics.Previous.UniqueVisitors, metrics.Current.UniqueVisitors),
			"totalVisitsChange":    analyticsPercentChange(metrics.Previous.TotalVisits, metrics.Current.TotalVisits),
			"totalPageviewsChange": analyticsPercentChange(metrics.Previous.TotalPageviews, metrics.Current.TotalPageviews),
			"viewsPerVisitChange":  analyticsPercentChangeFloat(metrics.Previous.ViewsPerVisit, metrics.Current.ViewsPerVisit),
			"bounceRateChange":     analyticsPercentChangeFloat(metrics.Previous.BounceRate, metrics.Current.BounceRate),
			"avgVisitDurationChange": analyticsPercentChange(
				metrics.Previous.AvgVisitDuration,
				metrics.Current.AvgVisitDuration,
			),
		},
	})
}

func (h *AnalyticsExtensionAPIHandler) TimeSeries(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	period := analyticsQuery(c, "period", defaultAnalyticsPeriod)
	interval := analyticsQuery(c, "interval", "day")
	rows, err := h.query.GetTimeSeries(c.Request.Context(), property.ID, period, interval, property.Timezone, nil, nil)
	if err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	result := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		result = append(result, gin.H{
			"date":      row.Date,
			"visitors":  row.Visitors,
			"pageviews": row.Pageviews,
		})
	}
	c.JSON(http.StatusOK, gin.H{"rows": result})
}

func (h *AnalyticsExtensionAPIHandler) Breakdown(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	period := analyticsQuery(c, "period", defaultAnalyticsPeriod)
	dimension := analyticsQuery(c, "dimension", "SOURCE")
	limit := 10
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			analyticsError(c, http.StatusBadRequest, parseErr)
			return
		}
		limit = parsed
	}
	rows, err := h.query.GetBreakdown(c.Request.Context(), property.ID, period, dimension, property.Timezone, limit, nil, nil)
	if err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	result := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		item := gin.H{
			"name":     row.Name,
			"visitors": row.Visitors,
		}
		if row.Pageviews != nil {
			item["pageviews"] = *row.Pageviews
		}
		result = append(result, item)
	}
	c.JSON(http.StatusOK, gin.H{"rows": result})
}

func (h *AnalyticsExtensionAPIHandler) ListGoals(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	goals, err := h.query.ListGoals(c.Request.Context(), property.ID)
	if err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	result := make([]gin.H, 0, len(goals))
	for _, goal := range goals {
		result = append(result, serializeAnalyticsGoal(goal))
	}
	c.JSON(http.StatusOK, gin.H{"goals": result})
}

func (h *AnalyticsExtensionAPIHandler) GoalResults(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	period := analyticsQuery(c, "period", defaultAnalyticsPeriod)
	goals, err := h.query.ListGoals(c.Request.Context(), property.ID)
	if err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	results, err := h.query.GetGoalResults(c.Request.Context(), property.ID, period, property.Timezone, nil, nil)
	if err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	goalMap := make(map[string]*analyticsdomain.Goal, len(goals))
	for _, goal := range goals {
		goalMap[goal.ID] = goal
	}
	payload := make([]gin.H, 0, len(results))
	for _, result := range results {
		goal := goalMap[result.GoalID]
		if goal == nil {
			continue
		}
		payload = append(payload, gin.H{
			"goal":           serializeAnalyticsGoal(goal),
			"uniques":        result.Uniques,
			"total":          result.Total,
			"conversionRate": result.ConversionRate,
		})
	}
	c.JSON(http.StatusOK, gin.H{"results": payload})
}

func (h *AnalyticsExtensionAPIHandler) CreateGoal(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	var req struct {
		GoalType  string `json:"goalType"`
		EventName string `json:"eventName"`
		PagePath  string `json:"pagePath"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	goal, err := h.query.CreateGoal(c.Request.Context(), property.ID, strings.TrimSpace(req.GoalType), strings.TrimSpace(req.EventName), strings.TrimSpace(req.PagePath))
	if err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"goal": serializeAnalyticsGoal(goal)})
}

func (h *AnalyticsExtensionAPIHandler) DeleteGoal(c *gin.Context) {
	if _, err := h.ensureGoalInWorkspace(c, c.Param("goalID")); err != nil {
		return
	}
	if err := h.query.DeleteGoal(c.Request.Context(), c.Param("goalID")); err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *AnalyticsExtensionAPIHandler) ListHostnameRules(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	rules, err := h.query.ListHostnameRules(c.Request.Context(), property.ID)
	if err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	result := make([]gin.H, 0, len(rules))
	for _, rule := range rules {
		result = append(result, gin.H{
			"id":        rule.ID,
			"pattern":   rule.Pattern,
			"createdAt": rule.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"rules": result})
}

func (h *AnalyticsExtensionAPIHandler) CreateHostnameRule(c *gin.Context) {
	property, err := h.loadPropertyInWorkspace(c)
	if err != nil {
		return
	}
	var req struct {
		Pattern string `json:"pattern"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	rule, err := h.query.CreateHostnameRule(c.Request.Context(), property.ID, strings.TrimSpace(req.Pattern))
	if err != nil {
		analyticsError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"rule": gin.H{
			"id":        rule.ID,
			"pattern":   rule.Pattern,
			"createdAt": rule.CreatedAt,
		},
	})
}

func (h *AnalyticsExtensionAPIHandler) DeleteHostnameRule(c *gin.Context) {
	if _, err := h.ensureHostnameRuleInWorkspace(c, c.Param("ruleID")); err != nil {
		return
	}
	if err := h.query.DeleteHostnameRule(c.Request.Context(), c.Param("ruleID")); err != nil {
		analyticsError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *AnalyticsExtensionAPIHandler) loadPropertyInWorkspace(c *gin.Context) (*analyticsdomain.Property, error) {
	workspaceID, ok := analyticsWorkspaceID(c)
	if !ok {
		return nil, errors.New("workspace context required")
	}
	propertyID := strings.TrimSpace(c.Param("id"))
	if propertyID == "" {
		analyticsError(c, http.StatusBadRequest, errors.New("property id is required"))
		return nil, errors.New("property id is required")
	}
	property, err := h.query.GetProperty(c.Request.Context(), propertyID)
	if err != nil {
		analyticsError(c, http.StatusNotFound, err)
		return nil, err
	}
	if property.WorkspaceID != workspaceID {
		analyticsError(c, http.StatusNotFound, errors.New("property not found"))
		return nil, errors.New("property not found")
	}
	return property, nil
}

func (h *AnalyticsExtensionAPIHandler) ensureGoalInWorkspace(c *gin.Context, goalID string) (*analyticsdomain.Goal, error) {
	goal, err := h.query.GetGoal(c.Request.Context(), goalID)
	if err != nil {
		analyticsError(c, http.StatusNotFound, err)
		return nil, err
	}
	property, err := h.query.GetProperty(c.Request.Context(), goal.PropertyID)
	if err != nil {
		analyticsError(c, http.StatusNotFound, err)
		return nil, err
	}
	workspaceID, ok := analyticsWorkspaceID(c)
	if !ok {
		return nil, errors.New("workspace context required")
	}
	if property.WorkspaceID != workspaceID {
		analyticsError(c, http.StatusNotFound, errors.New("goal not found"))
		return nil, errors.New("goal not found")
	}
	return goal, nil
}

func (h *AnalyticsExtensionAPIHandler) ensureHostnameRuleInWorkspace(c *gin.Context, ruleID string) (*analyticsdomain.HostnameRule, error) {
	rule, err := h.query.GetHostnameRule(c.Request.Context(), ruleID)
	if err != nil {
		analyticsError(c, http.StatusNotFound, err)
		return nil, err
	}
	property, err := h.query.GetProperty(c.Request.Context(), rule.PropertyID)
	if err != nil {
		analyticsError(c, http.StatusNotFound, err)
		return nil, err
	}
	workspaceID, ok := analyticsWorkspaceID(c)
	if !ok {
		return nil, errors.New("workspace context required")
	}
	if property.WorkspaceID != workspaceID {
		analyticsError(c, http.StatusNotFound, errors.New("hostname rule not found"))
		return nil, errors.New("hostname rule not found")
	}
	return rule, nil
}

func (h *AnalyticsExtensionAPIHandler) serializeProperty(property *analyticsdomain.Property, visitorsLast24h int, currentVisitors *int) gin.H {
	payload := gin.H{
		"id":              property.ID,
		"domain":          property.Domain,
		"timezone":        property.Timezone,
		"status":          property.Status,
		"snippetHtml":     property.SnippetHTML(h.apiBaseURL),
		"verifiedAt":      property.VerifiedAt,
		"visitorsLast24h": visitorsLast24h,
		"createdAt":       property.CreatedAt,
	}
	if currentVisitors != nil {
		payload["currentVisitors"] = *currentVisitors
	}
	return payload
}

func serializeAnalyticsGoal(goal *analyticsdomain.Goal) gin.H {
	return gin.H{
		"id":        goal.ID,
		"name":      goal.DisplayName(),
		"goalType":  goal.GoalType,
		"eventName": goal.EventName,
		"pagePath":  goal.PagePath,
		"createdAt": goal.CreatedAt,
	}
}

func analyticsWorkspaceID(c *gin.Context) (string, bool) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		analyticsError(c, http.StatusBadRequest, errors.New("workspace context required"))
		return "", false
	}
	return workspaceID, true
}

func analyticsQuery(c *gin.Context, name, fallback string) string {
	if value := strings.TrimSpace(c.Query(name)); value != "" {
		return value
	}
	return fallback
}

func analyticsError(c *gin.Context, status int, err error) {
	if c == nil {
		return
	}
	message := "analytics request failed"
	if err != nil {
		message = err.Error()
	}
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}

func analyticsPercentChange(previous, current int) *float64 {
	if previous == 0 {
		if current == 0 {
			value := float64(0)
			return &value
		}
		return nil
	}
	value := (float64(current-previous) / float64(previous)) * 100
	return &value
}

func analyticsPercentChangeFloat(previous, current float64) *float64 {
	if previous == 0 {
		if current == 0 {
			value := float64(0)
			return &value
		}
		return nil
	}
	value := ((current - previous) / previous) * 100
	return &value
}
