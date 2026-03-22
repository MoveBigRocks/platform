package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
)

// Workspace management handlers

// ShowWorkspaces renders the workspaces management page
func (h *AdminManagementHandler) ShowWorkspaces(c *gin.Context) {
	ctx := c.Request.Context()

	workspaces, err := h.workspaceService.ListAllWorkspaces(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", ErrorPageData{
			Error: "Failed to load workspaces: ",
		})
		return
	}

	data := buildAdminTemplateContext(c, "workspaces", "Workspaces", "Manage all workspaces in this instance")
	data["Workspaces"] = workspaces

	c.HTML(http.StatusOK, "workspaces.html", data)
}

// CreateWorkspace handles workspace creation (API)
func (h *AdminManagementHandler) CreateWorkspace(c *gin.Context) {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		ShortCode   string `json:"slug" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	workspace, err := h.workspaceService.CreateWorkspace(c.Request.Context(), req.Name, req.ShortCode, req.Description)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to create workspace")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"ID":          workspace.ID,
		"Name":        workspace.Name,
		"Slug":        workspace.ShortCode,
		"Description": workspace.Description,
	})
}

// UpdateWorkspace handles workspace updates (API)
func (h *AdminManagementHandler) UpdateWorkspace(c *gin.Context) {
	workspaceID := middleware.ValidateUUIDParam(c, "id")
	if workspaceID == "" {
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	err := h.workspaceService.UpdateWorkspace(c.Request.Context(), workspaceID, req.Name, req.Slug, req.Description)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to update workspace")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Workspace updated successfully"})
}

// DeleteWorkspace handles workspace deletion (API)
func (h *AdminManagementHandler) DeleteWorkspace(c *gin.Context) {
	workspaceID := middleware.ValidateUUIDParam(c, "id")
	if workspaceID == "" {
		return
	}

	err := h.workspaceService.DeleteWorkspace(c.Request.Context(), workspaceID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to delete workspace")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Workspace deleted successfully"})
}

// GetWorkspace returns a single workspace by ID (API)
func (h *AdminManagementHandler) GetWorkspace(c *gin.Context) {
	workspaceID := middleware.ValidateUUIDParam(c, "id")
	if workspaceID == "" {
		return
	}

	workspace, err := h.workspaceService.GetWorkspace(c.Request.Context(), workspaceID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Workspace not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ID":          workspace.ID,
		"Name":        workspace.Name,
		"Slug":        workspace.ShortCode,
		"Description": workspace.Description,
	})
}
