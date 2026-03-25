package analyticshandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AnalyticsAdminHandler handles the admin panel pages for web analytics.
// These are server-rendered pages; all data loading happens via GraphQL from client-side JS.
type AnalyticsAdminHandler struct{}

const analyticsAdminBasePath = "/extensions/web-analytics"

// NewAnalyticsAdminHandler creates a new analytics admin handler.
func NewAnalyticsAdminHandler() *AnalyticsAdminHandler {
	return &AnalyticsAdminHandler{}
}

// ShowAnalyticsProperties renders the properties list page.
func (h *AnalyticsAdminHandler) ShowAnalyticsProperties(c *gin.Context) {
	c.HTML(http.StatusOK, "analytics_properties.html", buildPageData(c, "Analytics Properties", "Manage web analytics properties"))
}

// ShowPropertyDashboard renders the analytics dashboard for a single property.
func (h *AnalyticsAdminHandler) ShowPropertyDashboard(c *gin.Context) {
	data := buildPageData(c, "Analytics Dashboard", "")
	data["PropertyID"] = c.Param("id")
	c.HTML(http.StatusOK, "analytics_dashboard.html", data)
}

// ShowPropertySetup renders the setup wizard for a property.
func (h *AnalyticsAdminHandler) ShowPropertySetup(c *gin.Context) {
	data := buildPageData(c, "Setup Analytics", "")
	data["PropertyID"] = c.Param("id")
	c.HTML(http.StatusOK, "analytics_setup.html", data)
}

// ShowPropertySettings renders the settings page for a property.
func (h *AnalyticsAdminHandler) ShowPropertySettings(c *gin.Context) {
	data := buildPageData(c, "Property Settings", "")
	data["PropertyID"] = c.Param("id")
	c.HTML(http.StatusOK, "analytics_settings.html", data)
}

// buildPageData creates the common template data for analytics admin pages.
func buildPageData(c *gin.Context, title, subtitle string) gin.H {
	data := gin.H{
		"ActivePage":              "analytics",
		"PageTitle":               title,
		"PageSubtitle":            subtitle,
		"AnalyticsBasePath":       analyticsAdminBasePath,
		"AnalyticsPropertiesPath": analyticsAdminBasePath,
	}

	if propertyID := c.Param("id"); propertyID != "" {
		data["AnalyticsPropertyPath"] = analyticsPropertyPath(propertyID)
		data["AnalyticsPropertySetupPath"] = analyticsPropertyPath(propertyID) + "/setup"
		data["AnalyticsPropertySettingsPath"] = analyticsPropertyPath(propertyID) + "/settings"
	}

	if name, ok := c.Get("name"); ok {
		data["UserName"] = name
	}
	if email, ok := c.Get("email"); ok {
		data["UserEmail"] = email
	}

	// Pass through sidebar context from feature middleware
	if nav, ok := c.Get("admin_extension_nav"); ok {
		data["ExtensionNav"] = nav
	}
	if widgets, ok := c.Get("admin_extension_widgets"); ok {
		data["ExtensionWidgets"] = widgets
	}
	if showET, ok := c.Get("admin_feature_error_tracking"); ok {
		data["ShowErrorTracking"] = showET
	}
	if showA, ok := c.Get("admin_feature_analytics"); ok {
		data["ShowAnalytics"] = showA
	}

	return data
}

func analyticsPropertyPath(propertyID string) string {
	return analyticsAdminBasePath + "/" + propertyID
}
