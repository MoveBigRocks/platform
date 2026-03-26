package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	"github.com/movebigrocks/platform/internal/platform/handlers/dtos"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
)

// WorkspaceAPIHandler handles workspace API endpoints for cases.
type WorkspaceAPIHandler struct {
	caseService *serviceapp.CaseService
}

func NewWorkspaceAPIHandler(caseService *serviceapp.CaseService) *WorkspaceAPIHandler {
	return &WorkspaceAPIHandler{caseService: caseService}
}

func (h *WorkspaceAPIHandler) ListCases(c *gin.Context) {
	workspaceID := c.GetString("workspace_id")

	filters := contracts.CaseFilters{
		WorkspaceID: workspaceID,
		Status:      c.Query("status"),
		Priority:    c.Query("priority"),
		Limit:       100,
		Offset:      0,
	}

	cases, total, err := h.caseService.ListCases(c.Request.Context(), filters)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to list cases")
		return
	}

	caseResponses := make([]dtos.CaseResponse, len(cases))
	for i, caseObj := range cases {
		caseResponses[i] = dtos.ToCaseResponse(caseObj)
	}

	c.JSON(http.StatusOK, gin.H{
		"cases": caseResponses,
		"total": total,
	})
}

func (h *WorkspaceAPIHandler) GetCase(c *gin.Context) {
	workspaceID := c.GetString("workspace_id")
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	caseDetail, err := h.caseService.GetCaseInWorkspace(c.Request.Context(), workspaceID, caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Case not found")
		return
	}

	c.JSON(http.StatusOK, dtos.ToCaseResponse(caseDetail))
}

func (h *WorkspaceAPIHandler) CreateCase(c *gin.Context) {
	workspaceID := c.GetString("workspace_id")

	var req dtos.CreateCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	params := serviceapp.CreateCaseParams{
		WorkspaceID: workspaceID,
		Subject:     req.Subject,
		Description: req.Description,
		Priority:    servicedomain.CasePriority(req.Priority),
		ContactID:   req.ContactID,
	}

	newCase, err := h.caseService.CreateCase(c.Request.Context(), params)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to create case")
		return
	}

	c.JSON(http.StatusCreated, dtos.ToCaseResponse(newCase))
}

func (h *WorkspaceAPIHandler) UpdateCase(c *gin.Context) {
	workspaceID := c.GetString("workspace_id")
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	if _, err := h.caseService.GetCaseInWorkspace(c.Request.Context(), workspaceID, caseID); err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Case not found")
		return
	}

	var req struct {
		Subject     string                     `json:"subject"`
		Description string                     `json:"description"`
		Status      servicedomain.CaseStatus   `json:"status"`
		Priority    servicedomain.CasePriority `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	ctx := c.Request.Context()
	if req.Status != "" {
		if err := h.caseService.SetCaseStatus(ctx, caseID, req.Status); err != nil {
			middleware.RespondWithError(c, http.StatusBadRequest, "Invalid status transition")
			return
		}
	}
	if req.Priority != "" {
		if err := h.caseService.SetCasePriority(ctx, caseID, req.Priority); err != nil {
			middleware.RespondWithError(c, http.StatusBadRequest, "Invalid priority value")
			return
		}
	}
	if req.Subject != "" || req.Description != "" {
		caseDetail, err := h.caseService.GetCaseInWorkspace(ctx, workspaceID, caseID)
		if err != nil {
			middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to reload case")
			return
		}
		if req.Subject != "" {
			caseDetail.Subject = req.Subject
		}
		if req.Description != "" {
			caseDetail.Description = req.Description
		}
		if err := h.caseService.UpdateCase(ctx, caseDetail); err != nil {
			middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to update case")
			return
		}
	}

	updated, err := h.caseService.GetCaseInWorkspace(ctx, workspaceID, caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to load updated case")
		return
	}
	c.JSON(http.StatusOK, dtos.ToCaseResponse(updated))
}

func (h *WorkspaceAPIHandler) DeleteCase(c *gin.Context) {
	workspaceID := c.GetString("workspace_id")
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	if _, err := h.caseService.GetCaseInWorkspace(c.Request.Context(), workspaceID, caseID); err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Case not found")
		return
	}

	if err := h.caseService.DeleteCase(c.Request.Context(), workspaceID, caseID); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to delete case")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Case deleted successfully"})
}
