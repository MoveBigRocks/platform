package platformhandlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WorkspaceNamesProvider is a function that returns workspace names keyed by ID
type WorkspaceNamesProvider func(ctx context.Context, workspaceIDs []string) map[string]string

// getWorkspaceNamesMap fetches workspace names for the given IDs using batch query
func (h *AdminManagementHandler) getWorkspaceNamesMap(ctx context.Context, workspaceIDs []string) map[string]string {
	if len(workspaceIDs) == 0 {
		return make(map[string]string)
	}

	// Deduplicate IDs
	idSet := make(map[string]struct{})
	for _, id := range workspaceIDs {
		idSet[id] = struct{}{}
	}
	uniqueIDs := make([]string, 0, len(idSet))
	for id := range idSet {
		uniqueIDs = append(uniqueIDs, id)
	}

	workspaces, err := h.workspaceService.GetWorkspacesByIDs(ctx, uniqueIDs)
	if err != nil {
		return make(map[string]string)
	}

	names := make(map[string]string, len(workspaces))
	for _, ws := range workspaces {
		names[ws.ID] = ws.Name
	}
	return names
}

func currentWorkspaceScope(c *gin.Context) (workspaceID, workspaceName string, ok bool) {
	workspaceID, workspaceName, _, ok = GetContextValues(c).WorkspaceContext()
	return workspaceID, workspaceName, ok
}

// renderAdminError renders an error page with consistent formatting
func renderAdminError(c *gin.Context, statusCode int, resource string, err error) {
	c.HTML(statusCode, "error.html", ErrorPageData{
		Error: "Failed to load " + resource,
	})
}

// AdminPageData contains common data for admin pages
type AdminPageData struct {
	ActivePage         string
	PageTitle          string
	PageSubtitle       string
	UserName           string
	UserEmail          string
	UserRole           string
	CanManageUsers     bool
	IsWorkspaceScoped  bool
	ShowErrorTracking  bool
	ShowAnalytics      bool
	ExtensionNav       []AdminExtensionNavSection
	ExtensionWidgets   []AdminExtensionWidget
	CurrentWorkspaceID string
	CurrentWorkspace   string
	WorkspaceNames     map[string]string
}

type AdminExtensionNavSection struct {
	Title string
	Items []AdminExtensionNavItem
}

type AdminExtensionNavItem struct {
	Title      string
	Icon       string
	Href       string
	ActivePage string
}

type AdminExtensionWidget struct {
	Title       string
	Description string
	Icon        string
	Href        string
}

const (
	adminFeatureErrorTrackingKey = "admin_feature_error_tracking"
	adminFeatureAnalyticsKey     = "admin_feature_analytics"
	adminExtensionNavKey         = "admin_extension_nav"
	adminExtensionWidgetsKey     = "admin_extension_widgets"
)

// buildBasePageData creates common base page data from context.
func buildBasePageData(c *gin.Context, activePage, title, subtitle string) BasePageData {
	ctxValues := GetContextValues(c)
	workspaceID, workspaceName, _, isWorkspaceScoped := ctxValues.WorkspaceContext()
	showErrorTracking, _ := c.Get(adminFeatureErrorTrackingKey)
	showAnalytics, _ := c.Get(adminFeatureAnalyticsKey)
	extensionNav, _ := c.Get(adminExtensionNavKey)
	extensionWidgets, _ := c.Get(adminExtensionWidgetsKey)

	return BasePageData{
		ActivePage:         activePage,
		PageTitle:          title,
		PageSubtitle:       subtitle,
		UserName:           ctxValues.UserName,
		UserEmail:          ctxValues.UserEmail,
		UserRole:           ctxValues.UserRole(),
		CanManageUsers:     ctxValues.CanManageUsers(),
		IsWorkspaceScoped:  isWorkspaceScoped,
		ShowErrorTracking:  boolValueOrDefault(showErrorTracking, true),
		ShowAnalytics:      boolValueOrDefault(showAnalytics, true),
		ExtensionNav:       navSectionsOrDefault(extensionNav),
		ExtensionWidgets:   extensionWidgetsOrDefault(extensionWidgets),
		CurrentWorkspaceID: workspaceID,
		CurrentWorkspace:   workspaceName,
	}
}

// buildAdminPageData creates common admin page data from context
func buildAdminPageData(c *gin.Context, activePage, title, subtitle string, workspaceNames map[string]string) AdminPageData {
	base := buildBasePageData(c, activePage, title, subtitle)

	return AdminPageData{
		ActivePage:         base.ActivePage,
		PageTitle:          base.PageTitle,
		PageSubtitle:       base.PageSubtitle,
		UserName:           base.UserName,
		UserEmail:          base.UserEmail,
		UserRole:           base.UserRole,
		CanManageUsers:     base.CanManageUsers,
		IsWorkspaceScoped:  base.IsWorkspaceScoped,
		ShowErrorTracking:  base.ShowErrorTracking,
		ShowAnalytics:      base.ShowAnalytics,
		ExtensionNav:       base.ExtensionNav,
		ExtensionWidgets:   base.ExtensionWidgets,
		CurrentWorkspaceID: base.CurrentWorkspaceID,
		CurrentWorkspace:   base.CurrentWorkspace,
		WorkspaceNames:     workspaceNames,
	}
}

// buildAdminTemplateContext returns the shared template keys for any admin page.
func buildAdminTemplateContext(c *gin.Context, activePage, pageTitle, pageSubtitle string) gin.H {
	base := buildBasePageData(c, activePage, pageTitle, pageSubtitle)
	return gin.H{
		"ActivePage":         base.ActivePage,
		"PageTitle":          base.PageTitle,
		"PageSubtitle":       base.PageSubtitle,
		"UserName":           base.UserName,
		"UserEmail":          base.UserEmail,
		"UserRole":           base.UserRole,
		"CanManageUsers":     base.CanManageUsers,
		"IsWorkspaceScoped":  base.IsWorkspaceScoped,
		"ShowErrorTracking":  base.ShowErrorTracking,
		"ShowAnalytics":      base.ShowAnalytics,
		"ExtensionNav":       base.ExtensionNav,
		"ExtensionWidgets":   base.ExtensionWidgets,
		"CurrentWorkspaceID": base.CurrentWorkspaceID,
		"CurrentWorkspace":   base.CurrentWorkspace,
	}
}

// renderAdminPage renders an admin page with common data
func renderAdminPage(c *gin.Context, template string, pageData AdminPageData, extraData gin.H) {
	data := gin.H{
		"ActivePage":         pageData.ActivePage,
		"PageTitle":          pageData.PageTitle,
		"PageSubtitle":       pageData.PageSubtitle,
		"UserName":           pageData.UserName,
		"UserEmail":          pageData.UserEmail,
		"UserRole":           pageData.UserRole,
		"CanManageUsers":     pageData.CanManageUsers,
		"IsWorkspaceScoped":  pageData.IsWorkspaceScoped,
		"ShowErrorTracking":  pageData.ShowErrorTracking,
		"ShowAnalytics":      pageData.ShowAnalytics,
		"ExtensionNav":       pageData.ExtensionNav,
		"ExtensionWidgets":   pageData.ExtensionWidgets,
		"CurrentWorkspaceID": pageData.CurrentWorkspaceID,
		"CurrentWorkspace":   pageData.CurrentWorkspace,
		"WorkspaceNames":     pageData.WorkspaceNames,
	}

	// Merge extra data
	for k, v := range extraData {
		data[k] = v
	}

	c.HTML(http.StatusOK, template, data)
}

func boolValueOrDefault(value any, fallback bool) bool {
	flag, ok := value.(bool)
	if !ok {
		return fallback
	}
	return flag
}

func navSectionsOrDefault(value any) []AdminExtensionNavSection {
	sections, ok := value.([]AdminExtensionNavSection)
	if !ok {
		return nil
	}
	return sections
}

func extensionWidgetsOrDefault(value any) []AdminExtensionWidget {
	widgets, ok := value.([]AdminExtensionWidget)
	if !ok {
		return nil
	}
	return widgets
}
