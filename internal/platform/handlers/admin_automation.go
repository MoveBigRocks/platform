package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

// Automation management handlers (Rules, Workflows, Forms)

// ShowRules renders the automation rules list page
func (h *AdminManagementHandler) ShowRules(c *gin.Context) {
	ctx := c.Request.Context()

	rules, err := h.ruleService.ListAllRules(ctx)
	if err != nil {
		renderAdminError(c, http.StatusInternalServerError, "rules", err)
		return
	}

	// Build workspace names map
	workspaceNames := make(map[string]string)
	workspaceIDs := make([]string, 0)
	for _, rule := range rules {
		workspaceIDs = append(workspaceIDs, rule.WorkspaceID)
	}
	if len(workspaceIDs) > 0 {
		workspaces, _ := h.workspaceService.GetWorkspacesByIDs(ctx, workspaceIDs)
		for _, ws := range workspaces {
			workspaceNames[ws.ID] = ws.Name
		}
	}

	// Transform to view model
	type RuleWithWorkspace struct {
		ID              string
		WorkspaceID     string
		Name            string
		Description     string
		IsActive        bool
		Priority        int
		TotalExecutions int
		SuccessRate     float64
	}

	allRules := make([]RuleWithWorkspace, len(rules))
	for i, rule := range rules {
		allRules[i] = RuleWithWorkspace{
			ID:              rule.ID,
			WorkspaceID:     rule.WorkspaceID,
			Name:            rule.Title,
			Description:     rule.Description,
			IsActive:        rule.IsActive,
			Priority:        rule.Priority,
			TotalExecutions: 0, // Execution stats stored separately
			SuccessRate:     0.0,
		}
	}

	pageData := buildAdminPageData(c, "rules", "Automation Rules", "Manage automation rules across all workspaces", workspaceNames)
	renderAdminPage(c, "rules.html", pageData, gin.H{
		"Rules":      allRules,
		"TotalRules": len(allRules),
	})
}

// ShowForms renders the forms list page
func (h *AdminManagementHandler) ShowForms(c *gin.Context) {
	ctx := c.Request.Context()
	pageSubtitle := "Manage customer-facing support forms across all workspaces"
	workspaceNames := make(map[string]string)

	var (
		forms []*servicedomain.FormSchema
		err   error
	)

	if workspaceID, workspaceName, ok := currentWorkspaceScope(c); ok {
		forms, err = h.formService.ListWorkspaceForms(ctx, workspaceID)
		pageSubtitle = "View support forms for " + workspaceName
		workspaceNames[workspaceID] = workspaceName
	} else {
		forms, err = h.formService.ListAllForms(ctx)
	}
	if err != nil {
		renderAdminError(c, http.StatusInternalServerError, "forms", err)
		return
	}

	// Build workspace names map when browsing across workspaces.
	if len(workspaceNames) == 0 {
		workspaceIDs := make([]string, 0)
		for _, form := range forms {
			workspaceIDs = append(workspaceIDs, form.WorkspaceID)
		}
		if len(workspaceIDs) > 0 {
			workspaces, _ := h.workspaceService.GetWorkspacesByIDs(ctx, workspaceIDs)
			for _, ws := range workspaces {
				workspaceNames[ws.ID] = ws.Name
			}
		}
	}

	// Transform to view model
	type FormWithWorkspace struct {
		ID                 string
		WorkspaceID        string
		Name               string
		Slug               string
		Description        string
		Status             string
		IsPublic           bool
		AutoCreateCase     bool
		NotifyOnSubmission bool
		RequiresCaptcha    bool
		SubmissionCount    int
		ConversionRate     float64
		LastSubmissionAt   interface{}
	}

	allForms := make([]FormWithWorkspace, len(forms))
	for i, form := range forms {
		allForms[i] = FormWithWorkspace{
			ID:                 form.ID,
			WorkspaceID:        form.WorkspaceID,
			Name:               form.Name,
			Slug:               form.Slug,
			Description:        form.Description,
			Status:             string(form.Status),
			IsPublic:           form.IsPublic,
			AutoCreateCase:     form.AutoCreateCase,
			NotifyOnSubmission: form.NotifyOnSubmission,
			RequiresCaptcha:    form.RequiresCaptcha,
			SubmissionCount:    form.SubmissionCount,
			ConversionRate:     0.0,
			LastSubmissionAt:   nil,
		}
	}

	pageData := buildAdminPageData(c, "forms", "Support Forms", pageSubtitle, workspaceNames)
	renderAdminPage(c, "forms.html", pageData, gin.H{
		"Forms":      allForms,
		"TotalForms": len(allForms),
	})
}
