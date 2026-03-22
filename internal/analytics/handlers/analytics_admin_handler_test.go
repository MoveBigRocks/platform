package analyticshandlers

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsAdminHandler_ShowAnalyticsProperties_UsesExtensionBasePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewAnalyticsAdminHandler()
	w := httptest.NewRecorder()
	ctx, router := gin.CreateTestContext(w)
	router.SetHTMLTemplate(mustParseTemplate(t, "analytics_properties.html", `{{define "analytics_properties.html"}}{{.AnalyticsBasePath}}|{{.AnalyticsPropertiesPath}}{{end}}`))
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/extensions/web-analytics", nil)

	handler.ShowAnalyticsProperties(ctx)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/admin/extensions/web-analytics|/admin/extensions/web-analytics", w.Body.String())
}

func TestAnalyticsAdminHandler_ShowPropertyDashboard_UsesExtensionPropertyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewAnalyticsAdminHandler()
	w := httptest.NewRecorder()
	ctx, router := gin.CreateTestContext(w)
	router.SetHTMLTemplate(mustParseTemplate(t, "analytics_dashboard.html", `{{define "analytics_dashboard.html"}}{{.AnalyticsPropertyPath}}|{{.AnalyticsPropertySettingsPath}}{{end}}`))
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/extensions/web-analytics/property_123", nil)
	ctx.Params = gin.Params{{Key: "id", Value: "property_123"}}

	handler.ShowPropertyDashboard(ctx)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/admin/extensions/web-analytics/property_123|/admin/extensions/web-analytics/property_123/settings", w.Body.String())
}

func mustParseTemplate(t *testing.T, name, body string) *template.Template {
	t.Helper()
	return template.Must(template.New(name).Parse(body))
}
