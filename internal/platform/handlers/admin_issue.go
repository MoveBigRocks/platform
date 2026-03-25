package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
)

const errorTrackingIssuesBasePath = "/extensions/error-tracking/issues"

// ShowIssues renders the error tracking issues page
func (h *AdminManagementHandler) ShowIssues(c *gin.Context) {
	ctx := c.Request.Context()
	pageSubtitle := "View all error tracking issues across workspaces"
	workspaceNames := make(map[string]string)
	issuesBasePath := errorTrackingIssuesBasePath

	var (
		issues []*observabilitydomain.Issue
		err    error
	)

	if workspaceID, workspaceName, ok := currentWorkspaceScope(c); ok {
		issues, _, err = h.issueService.ListWorkspaceIssues(ctx, workspaceID, 100)
		pageSubtitle = "View error issues for " + workspaceName
		workspaceNames[workspaceID] = workspaceName
		issuesBasePath = errorTrackingIssuesBasePath
	} else {
		issues, _, err = h.issueService.ListAllIssues(ctx, contracts.IssueFilters{Limit: 100})
	}
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", ErrorPageData{Error: "Failed to load issues: "})
		return
	}

	// Build workspace names map
	projectNames := make(map[string]string)

	// Get project info for each issue
	projectIDs := make([]string, 0)
	for _, issue := range issues {
		projectIDs = append(projectIDs, issue.ProjectID)
	}
	if len(projectIDs) > 0 {
		projects, _ := h.projectService.GetProjectsByIDs(ctx, projectIDs)
		for _, p := range projects {
			projectNames[p.ID] = p.Name
			// Also collect workspace IDs for name lookup when cross-workspace.
			if len(workspaceNames) == 0 {
				if ws, err := h.workspaceService.GetWorkspace(ctx, p.WorkspaceID); err == nil {
					workspaceNames[p.WorkspaceID] = ws.Name
				}
			}
		}
	}

	base := buildBasePageData(c, "issues", "Error Issues", pageSubtitle)

	c.HTML(http.StatusOK, "issues.html", ConvertIssuesToPageData(issues, projectNames, workspaceNames, base, issuesBasePath))
}

// ShowIssueDetail renders the issue detail page with all events
func (h *AdminManagementHandler) ShowIssueDetail(c *gin.Context) {
	ctx := c.Request.Context()

	issueID := middleware.ValidateUUIDParam(c, "id")
	if issueID == "" {
		return
	}

	var (
		issue          *observabilitydomain.Issue
		project        *observabilitydomain.Project
		err            error
		workspaceName  string
		issuesBasePath = errorTrackingIssuesBasePath
	)

	if workspaceID, currentWorkspaceName, ok := currentWorkspaceScope(c); ok {
		issue, err = h.issueService.GetIssueInWorkspace(ctx, workspaceID, issueID)
		if err != nil || issue == nil {
			c.HTML(http.StatusNotFound, "error.html", ErrorPageData{Error: "Issue not found"})
			return
		}
		project, _ = h.projectService.GetProject(ctx, issue.ProjectID)
		if project == nil || project.WorkspaceID != workspaceID {
			c.HTML(http.StatusNotFound, "error.html", ErrorPageData{Error: "Issue not found"})
			return
		}
		workspaceName = currentWorkspaceName
		issuesBasePath = errorTrackingIssuesBasePath
	} else {
		issue, project, err = h.issueService.GetIssueWithProject(ctx, issueID)
		if err != nil {
			c.HTML(http.StatusNotFound, "error.html", ErrorPageData{Error: "Issue not found: "})
			return
		}
		if issue == nil {
			c.HTML(http.StatusNotFound, "error.html", ErrorPageData{Error: "Issue not found"})
			return
		}

		if project != nil {
			if ws, err := h.workspaceService.GetWorkspace(ctx, project.WorkspaceID); err == nil {
				workspaceName = ws.Name
			}
		}
	}

	// Get events for this issue
	events, _ := h.issueService.GetIssueEvents(ctx, issueID, 50)

	base := buildBasePageData(c, "issues", "Issue: "+issue.ShortID, issue.Title)

	c.HTML(http.StatusOK, "issue_detail.html", ConvertIssueDetailToPageData(issue, project, events, workspaceName, base, issuesBasePath))
}
