package platformhandlers

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/logger"
)

// AdminManagementHandler handles instance admin management pages and APIs
type AdminManagementHandler struct {
	workspaceService *platformservices.WorkspaceManagementService
	userService      *platformservices.UserManagementService
	statsService     *platformservices.AdminStatsService
	extensionService *platformservices.ExtensionService
	caseService      *serviceapp.CaseService
	ruleService      *automationservices.RuleService
	formService      *automationservices.FormService
	logger           *logger.Logger
}

// NewAdminManagementHandler creates a new admin management handler
func NewAdminManagementHandler(
	workspaceService *platformservices.WorkspaceManagementService,
	userService *platformservices.UserManagementService,
	statsService *platformservices.AdminStatsService,
	extensionService *platformservices.ExtensionService,
	caseService *serviceapp.CaseService,
	ruleService *automationservices.RuleService,
	formService *automationservices.FormService,
) *AdminManagementHandler {
	return &AdminManagementHandler{
		workspaceService: workspaceService,
		userService:      userService,
		statsService:     statsService,
		extensionService: extensionService,
		caseService:      caseService,
		ruleService:      ruleService,
		formService:      formService,
		logger:           logger.New().WithField("handler", "admin_management"),
	}
}

func (h *AdminManagementHandler) FeatureContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		workspaceID, _, ok := currentWorkspaceScope(c)
		c.Set(adminFeatureErrorTrackingKey, h.isSurfaceEnabled(c.Request.Context(), workspaceID, ok, "error-tracking"))
		c.Set(adminFeatureAnalyticsKey, h.isSurfaceEnabled(c.Request.Context(), workspaceID, ok, "web-analytics"))
		c.Set(adminExtensionNavKey, h.extensionNavigation(c.Request.Context(), workspaceID))
		c.Set(adminExtensionWidgetsKey, h.extensionWidgets(c.Request.Context(), workspaceID))
		c.Next()
	}
}

func (h *AdminManagementHandler) RequireSurface(slug string) gin.HandlerFunc {
	return func(c *gin.Context) {
		workspaceID, _, ok := currentWorkspaceScope(c)
		if !h.isSurfaceEnabled(c.Request.Context(), workspaceID, ok, slug) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Next()
	}
}

func (h *AdminManagementHandler) isSurfaceEnabled(ctx context.Context, workspaceID string, workspaceScoped bool, slug string) bool {
	if h.extensionService == nil {
		return true
	}
	var (
		enabled bool
		err     error
	)
	if workspaceScoped && workspaceID != "" {
		enabled, err = h.extensionService.HasActiveExtensionInWorkspace(ctx, workspaceID, slug)
	} else {
		enabled, err = h.extensionService.HasActiveExtension(ctx, slug)
	}
	if err != nil {
		return false
	}
	return enabled
}

func (h *AdminManagementHandler) extensionNavigation(ctx context.Context, workspaceID string) []AdminExtensionNavSection {
	if h.extensionService == nil {
		return nil
	}

	var (
		items []platformservices.ResolvedExtensionAdminNavigationItem
		err   error
	)
	if workspaceID != "" {
		items, err = h.extensionService.ListWorkspaceAdminNavigation(ctx, workspaceID)
	} else {
		items, err = h.extensionService.ListInstanceAdminNavigation(ctx)
	}
	if err != nil || len(items) == 0 {
		return nil
	}

	workspaceNames := map[string]string{}
	if workspaceID == "" {
		ids := make([]string, 0, len(items))
		for _, item := range items {
			if strings.TrimSpace(item.WorkspaceID) != "" {
				ids = append(ids, item.WorkspaceID)
			}
		}
		workspaceNames = h.getWorkspaceNamesMap(ctx, ids)
	}

	sectionOrder := make([]string, 0)
	sections := make(map[string][]AdminExtensionNavItem)
	for _, item := range items {
		section := item.Section
		if section == "" {
			section = "Extensions"
		}
		if _, exists := sections[section]; !exists {
			sectionOrder = append(sectionOrder, section)
		}
		title := item.Title
		if workspaceID == "" {
			if label := displayWorkspaceLabel(item.WorkspaceID, workspaceNames); label != "" {
				title = title + " · " + label
			}
		}
		sections[section] = append(sections[section], AdminExtensionNavItem{
			Title:      title,
			Icon:       item.Icon,
			Href:       item.Href,
			ActivePage: item.ActivePage,
		})
	}

	result := make([]AdminExtensionNavSection, 0, len(sectionOrder))
	for _, section := range sectionOrder {
		items := sections[section]
		slices.SortStableFunc(items, func(left, right AdminExtensionNavItem) int {
			return strings.Compare(left.Title, right.Title)
		})
		result = append(result, AdminExtensionNavSection{
			Title: section,
			Items: items,
		})
	}
	return result
}

func (h *AdminManagementHandler) extensionWidgets(ctx context.Context, workspaceID string) []AdminExtensionWidget {
	if h.extensionService == nil {
		return nil
	}

	var (
		widgets []platformservices.ResolvedExtensionDashboardWidget
		err     error
	)
	if workspaceID != "" {
		widgets, err = h.extensionService.ListWorkspaceDashboardWidgets(ctx, workspaceID)
	} else {
		widgets, err = h.extensionService.ListInstanceDashboardWidgets(ctx)
	}
	if err != nil || len(widgets) == 0 {
		return nil
	}

	workspaceNames := map[string]string{}
	if workspaceID == "" {
		ids := make([]string, 0, len(widgets))
		for _, widget := range widgets {
			if strings.TrimSpace(widget.WorkspaceID) != "" {
				ids = append(ids, widget.WorkspaceID)
			}
		}
		workspaceNames = h.getWorkspaceNamesMap(ctx, ids)
	}

	result := make([]AdminExtensionWidget, 0, len(widgets))
	for _, widget := range widgets {
		title := widget.Title
		description := widget.Description
		if workspaceID == "" {
			if label := displayWorkspaceLabel(widget.WorkspaceID, workspaceNames); label != "" {
				title = title + " · " + label
				if strings.TrimSpace(description) == "" {
					description = "Installed in " + label
				}
			}
		}
		result = append(result, AdminExtensionWidget{
			Title:       title,
			Description: description,
			Icon:        widget.Icon,
			Href:        widget.Href,
		})
	}
	return result
}

func displayWorkspaceLabel(workspaceID string, names map[string]string) string {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return ""
	}
	if label := strings.TrimSpace(names[workspaceID]); label != "" {
		return label
	}
	return workspaceID
}
