package platformhandlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gosimple/slug"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
)

// Application management handlers

const errorTrackingApplicationsBasePath = "/admin/extensions/error-tracking/applications"

// ShowApplications renders the error monitoring applications page
func (h *AdminManagementHandler) ShowApplications(c *gin.Context) {
	ctx := c.Request.Context()
	pageSubtitle := "View all monitored applications across workspaces"
	applicationsBasePath := errorTrackingApplicationsBasePath

	workspaceNames := make(map[string]string)
	var (
		projects []*observabilitydomain.Project
		err      error
	)
	if workspaceID, workspaceName, ok := currentWorkspaceScope(c); ok {
		projects, err = h.projectService.ListWorkspaceProjects(ctx, workspaceID)
		pageSubtitle = "View monitored applications for " + workspaceName
		workspaceNames[workspaceID] = workspaceName
		applicationsBasePath = errorTrackingApplicationsBasePath
	} else {
		projects, err = h.projectService.ListAllProjects(ctx)
		if err == nil {
			workspaceIDs := make([]string, 0, len(projects))
			for _, p := range projects {
				workspaceIDs = append(workspaceIDs, p.WorkspaceID)
			}
			if len(workspaceIDs) > 0 {
				workspaces, _ := h.workspaceService.GetWorkspacesByIDs(ctx, workspaceIDs)
				for _, ws := range workspaces {
					workspaceNames[ws.ID] = ws.Name
				}
			}
		}
	}
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", ErrorPageData{Error: "Failed to load applications: "})
		return
	}

	base := buildBasePageData(c, "applications", "Monitored Applications", pageSubtitle)

	c.HTML(http.StatusOK, "applications.html", ConvertProjectsToPageData(projects, workspaceNames, base, applicationsBasePath))
}

// ShowApplicationDetail renders the application detail/edit page
func (h *AdminManagementHandler) ShowApplicationDetail(c *gin.Context) {
	ctx := c.Request.Context()

	appID := c.Param("id")
	applicationsBasePath := errorTrackingApplicationsBasePath
	var currentWorkspaceID, currentWorkspaceName string
	if workspaceID, workspaceName, ok := currentWorkspaceScope(c); ok {
		currentWorkspaceID = workspaceID
		currentWorkspaceName = workspaceName
		applicationsBasePath = errorTrackingApplicationsBasePath
	}

	// Get all workspaces for the dropdown
	workspaceOptions := make([]WorkspaceOption, 0)
	if currentWorkspaceID != "" {
		workspaceOptions = append(workspaceOptions, WorkspaceOption{
			ID:   currentWorkspaceID,
			Name: currentWorkspaceName,
		})
	} else {
		allWorkspaces, err := h.workspaceService.ListAllWorkspaces(ctx)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", ErrorPageData{Error: "Failed to load workspaces: "})
			return
		}
		workspaceOptions = make([]WorkspaceOption, 0, len(allWorkspaces))
		for _, ws := range allWorkspaces {
			workspaceOptions = append(workspaceOptions, WorkspaceOption{
				ID:   ws.ID,
				Name: ws.Name,
			})
		}
	}

	// Handle "new" case - check both param and path (route /applications/new has no :id param)
	isNewApplication := appID == "new" || strings.HasSuffix(c.Request.URL.Path, "/new")
	if isNewApplication {
		base := buildBasePageData(c, "applications", "New Application", "Create a new monitored application")
		c.HTML(http.StatusOK, "application_detail.html", ConvertProjectToDetailPageData(nil, currentWorkspaceName, workspaceOptions, base, applicationsBasePath))
		return
	}

	// Validate UUID
	appID = middleware.ValidateUUIDParam(c, "id")
	if appID == "" {
		return
	}

	project, err := h.projectService.GetProject(ctx, appID)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", ErrorPageData{Error: "Application not found: "})
		return
	}
	if currentWorkspaceID != "" && project.WorkspaceID != currentWorkspaceID {
		c.HTML(http.StatusNotFound, "error.html", ErrorPageData{Error: "Application not found"})
		return
	}

	// Get workspace name
	var workspaceName string
	if currentWorkspaceID != "" {
		workspaceName = currentWorkspaceName
	} else if ws, err := h.workspaceService.GetWorkspace(ctx, project.WorkspaceID); err == nil {
		workspaceName = ws.Name
	}

	base := buildBasePageData(c, "applications", project.Name, "Manage application settings")

	c.HTML(http.StatusOK, "application_detail.html", ConvertProjectToDetailPageData(project, workspaceName, workspaceOptions, base, applicationsBasePath))
}

// GetApplication returns a single application by ID (API)
func (h *AdminManagementHandler) GetApplication(c *gin.Context) {
	appID := middleware.ValidateUUIDParam(c, "id")
	if appID == "" {
		return
	}

	project, err := h.projectService.GetProject(c.Request.Context(), appID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}
	if workspaceID, _, ok := currentWorkspaceScope(c); ok && project.WorkspaceID != workspaceID {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":             project.ID,
		"name":           project.Name,
		"slug":           project.Slug,
		"platform":       project.Platform,
		"environment":    project.Environment,
		"status":         project.Status,
		"dsn":            project.DSN,
		"publicKey":      project.PublicKey,
		"workspaceID":    project.WorkspaceID,
		"eventsPerHour":  project.EventsPerHour,
		"storageQuotaMB": project.StorageQuotaMB,
		"retentionDays":  project.RetentionDays,
		"eventCount":     project.EventCount,
		"createdAt":      project.CreatedAt,
	})
}

// CreateApplication handles application creation (API)
func (h *AdminManagementHandler) CreateApplication(c *gin.Context) {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		WorkspaceID string `json:"workspaceID" binding:"required"`
		Platform    string `json:"platform"`
		Environment string `json:"environment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format: name and workspaceID are required")
		return
	}
	if workspaceID, _, ok := currentWorkspaceScope(c); ok {
		if req.WorkspaceID == "" {
			req.WorkspaceID = workspaceID
		}
		if req.WorkspaceID != workspaceID {
			middleware.RespondWithError(c, http.StatusForbidden, "Workspace mismatch")
			return
		}
	}

	// Generate slug from name
	appSlug := slug.Make(req.Name)

	// Create new project using the domain constructor (generates keys, DSN, etc.)
	project := observabilitydomain.NewProject(req.WorkspaceID, "", req.Name, appSlug, req.Platform)

	// Override environment if provided
	if req.Environment != "" {
		project.Environment = req.Environment
	}

	if err := h.projectService.CreateProject(c.Request.Context(), project); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to create application")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          project.ID,
		"name":        project.Name,
		"slug":        project.Slug,
		"dsn":         project.DSN,
		"publicKey":   project.PublicKey,
		"workspaceID": project.WorkspaceID,
	})
}

// UpdateApplication handles application updates (API)
func (h *AdminManagementHandler) UpdateApplication(c *gin.Context) {
	appID := middleware.ValidateUUIDParam(c, "id")
	if appID == "" {
		return
	}

	var req struct {
		Name           string `json:"name"`
		Platform       string `json:"platform"`
		Environment    string `json:"environment"`
		Status         string `json:"status"`
		EventsPerHour  int    `json:"eventsPerHour"`
		StorageQuotaMB int    `json:"storageQuotaMB"`
		RetentionDays  int    `json:"retentionDays"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	ctx := c.Request.Context()

	// Get existing project
	project, err := h.projectService.GetProject(ctx, appID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}
	if workspaceID, _, ok := currentWorkspaceScope(c); ok && project.WorkspaceID != workspaceID {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}

	// Update fields if provided
	if req.Name != "" {
		project.Name = req.Name
		project.Slug = slug.Make(req.Name)
	}
	if req.Platform != "" {
		project.Platform = req.Platform
	}
	if req.Environment != "" {
		project.Environment = req.Environment
	}
	if req.Status != "" {
		// Validate status
		validStatuses := map[string]bool{"active": true, "paused": true, "disabled": true}
		if !validStatuses[strings.ToLower(req.Status)] {
			middleware.RespondWithError(c, http.StatusBadRequest, "Invalid status. Must be: active, paused, or disabled")
			return
		}
		project.Status = strings.ToLower(req.Status)
	}
	if req.EventsPerHour > 0 {
		project.EventsPerHour = req.EventsPerHour
	}
	if req.StorageQuotaMB > 0 {
		project.StorageQuotaMB = req.StorageQuotaMB
	}
	if req.RetentionDays > 0 {
		project.RetentionDays = req.RetentionDays
	}

	if err := h.projectService.UpdateProject(ctx, project); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to update application")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Application updated successfully"})
}

// DeleteApplication handles application deletion (API)
func (h *AdminManagementHandler) DeleteApplication(c *gin.Context) {
	appID := middleware.ValidateUUIDParam(c, "id")
	if appID == "" {
		return
	}

	ctx := c.Request.Context()

	// Get the project to obtain its workspace ID for the scoped delete
	project, err := h.projectService.GetProject(ctx, appID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}
	if workspaceID, _, ok := currentWorkspaceScope(c); ok && project.WorkspaceID != workspaceID {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}

	if err := h.projectService.DeleteProject(ctx, project.WorkspaceID, appID); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to delete application")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Application deleted successfully"})
}
