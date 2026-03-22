package platformhandlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/platform/handlers/dtos"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
)

type workspaceIssueProvider interface {
	ListIssues(ctx context.Context, filters contracts.IssueFilters) ([]*observabilitydomain.Issue, int, error)
	GetIssueInWorkspace(ctx context.Context, workspaceID, issueID string) (*observabilitydomain.Issue, error)
	GetIssueEvents(ctx context.Context, issueID string, limit int) ([]*observabilitydomain.ErrorEvent, error)
	UpdateIssue(ctx context.Context, issue *observabilitydomain.Issue) error
	ResolveIssue(ctx context.Context, issueID string, resolution string, resolvedBy string) error
	GetIssue(ctx context.Context, issueID string) (*observabilitydomain.Issue, error)
}

type workspaceExtensionChecker interface {
	HasActiveExtensionInWorkspace(ctx context.Context, workspaceID, slug string) (bool, error)
}

// WorkspaceAPIHandler handles workspace API endpoints for cases and issues
type WorkspaceAPIHandler struct {
	caseService      *serviceapp.CaseService
	issueService     workspaceIssueProvider
	extensionChecker workspaceExtensionChecker
}

// NewWorkspaceAPIHandler creates a new workspace API handler
func NewWorkspaceAPIHandler(
	caseService *serviceapp.CaseService,
	issueService workspaceIssueProvider,
	extensionChecker workspaceExtensionChecker,
) *WorkspaceAPIHandler {
	return &WorkspaceAPIHandler{
		caseService:      caseService,
		issueService:     issueService,
		extensionChecker: extensionChecker,
	}
}

func (h *WorkspaceAPIHandler) ensureSurfaceEnabled(c *gin.Context, slug string) bool {
	if h == nil || h.extensionChecker == nil {
		return true
	}
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		middleware.RespondWithError(c, http.StatusNotFound, "Not found")
		return false
	}
	enabled, err := h.extensionChecker.HasActiveExtensionInWorkspace(c.Request.Context(), workspaceID, slug)
	if err != nil || !enabled {
		middleware.RespondWithError(c, http.StatusNotFound, "Not found")
		return false
	}
	return true
}

// Case Management APIs

// ListCases lists all cases for the workspace
func (h *WorkspaceAPIHandler) ListCases(c *gin.Context) {
	workspaceID := c.GetString("workspace_id")

	// Parse query parameters for filtering
	status := c.Query("status")
	priority := c.Query("priority")

	filters := contracts.CaseFilters{
		WorkspaceID: workspaceID,
		Status:      status,
		Priority:    priority,
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

// GetCase retrieves a single case by ID
func (h *WorkspaceAPIHandler) GetCase(c *gin.Context) {
	workspaceID := c.GetString("workspace_id")
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return // Error already sent by ValidateUUIDParam
	}

	// Use workspace-scoped query for atomic authorization (ADR-0003)
	// Returns same error for non-existent and wrong-workspace to prevent enumeration
	caseDetail, err := h.caseService.GetCaseInWorkspace(c.Request.Context(), workspaceID, caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Case not found")
		return
	}

	c.JSON(http.StatusOK, dtos.ToCaseResponse(caseDetail))
}

// CreateCase creates a new case
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

// UpdateCase updates an existing case
func (h *WorkspaceAPIHandler) UpdateCase(c *gin.Context) {
	workspaceID := c.GetString("workspace_id")
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return // Error already sent by ValidateUUIDParam
	}

	// Use workspace-scoped query for atomic authorization (ADR-0003)
	// Returns same error for non-existent and wrong-workspace to prevent enumeration
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

	// Use domain-validated service methods for status and priority changes.
	// These enforce valid transitions and publish events for automation rules.
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

	// Update text fields directly (no domain validation needed)
	if req.Subject != "" || req.Description != "" {
		// Re-fetch to get any changes from SetCaseStatus/SetCasePriority
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

	// Return updated case
	updated, err := h.caseService.GetCaseInWorkspace(ctx, workspaceID, caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to load updated case")
		return
	}
	c.JSON(http.StatusOK, dtos.ToCaseResponse(updated))
}

// DeleteCase deletes a case
func (h *WorkspaceAPIHandler) DeleteCase(c *gin.Context) {
	workspaceID := c.GetString("workspace_id")
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return // Error already sent by ValidateUUIDParam
	}

	// Use workspace-scoped query for atomic authorization (ADR-0003)
	// Returns same error for non-existent and wrong-workspace to prevent enumeration
	_, err := h.caseService.GetCaseInWorkspace(c.Request.Context(), workspaceID, caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Case not found")
		return
	}

	if err := h.caseService.DeleteCase(c.Request.Context(), workspaceID, caseID); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to delete case")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Case deleted successfully"})
}

// Issue Management APIs

// ListIssues lists all issues for the workspace
func (h *WorkspaceAPIHandler) ListIssues(c *gin.Context) {
	if !h.ensureSurfaceEnabled(c, "error-tracking") {
		return
	}
	workspaceID := c.GetString("workspace_id")
	ctx := c.Request.Context()

	// Parse query parameters for filtering
	status := c.Query("status")
	level := c.Query("level")

	// Get all issues for workspace in a single query (no N+1)
	filters := contracts.IssueFilters{
		WorkspaceID: workspaceID,
		Status:      status,
		Level:       level,
		Limit:       100,
	}
	issues, total, err := h.issueService.ListIssues(ctx, filters)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to list issues")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"issues": issues,
		"total":  total,
	})
}

// GetIssue retrieves a single issue by ID
func (h *WorkspaceAPIHandler) GetIssue(c *gin.Context) {
	if !h.ensureSurfaceEnabled(c, "error-tracking") {
		return
	}
	workspaceID := c.GetString("workspace_id")
	issueID := c.Param("id")
	if issueID == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "Issue ID is required")
		return
	}
	ctx := c.Request.Context()
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil || !authCtx.HasPermission(platformdomain.PermissionIssueWrite) {
		middleware.RespondWithError(c, http.StatusForbidden, "issue:write permission required")
		return
	}

	// Use workspace-scoped query for atomic authorization
	// Returns same error for non-existent and wrong-workspace to prevent enumeration
	issue, err := h.issueService.GetIssueInWorkspace(ctx, workspaceID, issueID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Issue not found")
		return
	}

	// Get issue events
	events, err := h.issueService.GetIssueEvents(ctx, issueID, 20)
	if err != nil {
		events = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"issue":  issue,
		"events": events,
	})
}

// UpdateIssue updates an issue (status, assignment, etc.)
func (h *WorkspaceAPIHandler) UpdateIssue(c *gin.Context) {
	if !h.ensureSurfaceEnabled(c, "error-tracking") {
		return
	}
	workspaceID := c.GetString("workspace_id")
	issueID := c.Param("id")
	if issueID == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "Issue ID is required")
		return
	}
	ctx := c.Request.Context()

	// Use workspace-scoped query for atomic authorization
	// Returns same error for non-existent and wrong-workspace to prevent enumeration
	issue, err := h.issueService.GetIssueInWorkspace(ctx, workspaceID, issueID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Issue not found")
		return
	}

	var req struct {
		Status     string `json:"status"`
		AssignedTo string `json:"assigned_to"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Update fields
	if req.Status != "" {
		issue.Status = req.Status
	}
	if req.AssignedTo != "" {
		issue.AssignedTo = req.AssignedTo
	}

	if err := h.issueService.UpdateIssue(ctx, issue); err != nil {
		var apiErr *apierrors.APIError
		if errors.As(err, &apiErr) {
			status := apiErr.StatusCode
			if status == 0 {
				status = http.StatusInternalServerError
			}
			middleware.RespondWithError(c, status, apiErr.Message)
			return
		}

		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to update issue")
		return
	}

	c.JSON(http.StatusOK, issue)
}

// ResolveIssue marks an issue as resolved
func (h *WorkspaceAPIHandler) ResolveIssue(c *gin.Context) {
	if !h.ensureSurfaceEnabled(c, "error-tracking") {
		return
	}
	workspaceID := c.GetString("workspace_id")
	issueID := c.Param("id")
	if issueID == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "Issue ID is required")
		return
	}
	ctx := c.Request.Context()
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil || !authCtx.HasPermission(platformdomain.PermissionIssueWrite) {
		middleware.RespondWithError(c, http.StatusForbidden, "issue:write permission required")
		return
	}

	// Verify workspace ownership with atomic authorization query
	// Returns same error for non-existent and wrong-workspace to prevent enumeration
	_, err := h.issueService.GetIssueInWorkspace(ctx, workspaceID, issueID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Issue not found")
		return
	}

	var req struct {
		Resolution string `json:"resolution"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Mark as resolved using service method
	if err := h.issueService.ResolveIssue(ctx, issueID, req.Resolution, ""); err != nil {
		var apiErr *apierrors.APIError
		if errors.As(err, &apiErr) {
			status := apiErr.StatusCode
			if status == 0 {
				status = http.StatusInternalServerError
			}
			middleware.RespondWithError(c, status, apiErr.Message)
			return
		}

		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to resolve issue")
		return
	}

	// Get updated issue
	updatedIssue, err := h.issueService.GetIssue(ctx, issueID)
	if err != nil {
		// Resolve succeeded but re-fetch failed — return success without issue body
		c.JSON(http.StatusOK, gin.H{
			"message": "Issue resolved successfully",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Issue resolved successfully",
		"issue":   updatedIssue,
	})
}
