package platformhandlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
)

// ShowCases renders the cases management page
func (h *AdminManagementHandler) ShowCases(c *gin.Context) {
	ctx := c.Request.Context()
	filters := contracts.CaseFilters{Limit: 100}
	pageSubtitle := "View all support cases across workspaces"
	workspaceNames := make(map[string]string)

	if workspaceID, workspaceName, ok := currentWorkspaceScope(c); ok {
		filters.WorkspaceID = workspaceID
		pageSubtitle = "View support cases for " + workspaceName
		workspaceNames[workspaceID] = workspaceName
	}

	// Use service to list cases
	cases, total, err := h.caseService.ListCases(ctx, filters)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", ErrorPageData{Error: "Failed to load cases: "})
		return
	}

	// Build workspace names map when browsing across workspaces.
	if len(workspaceNames) == 0 {
		workspaceIDs := make([]string, 0, len(cases))
		for _, caseObj := range cases {
			workspaceIDs = append(workspaceIDs, caseObj.WorkspaceID)
		}
		workspaceNames = h.getWorkspaceNamesMap(ctx, workspaceIDs)
	}

	// Build user names map for assignees (batch query to avoid N+1)
	userNames := make(map[string]string)
	userIDs := make([]string, 0)
	for _, caseObj := range cases {
		if caseObj.AssignedToID != "" {
			userIDs = append(userIDs, caseObj.AssignedToID)
		}
	}
	if len(userIDs) > 0 {
		users, err := h.userService.GetUsersByIDs(ctx, userIDs)
		if err == nil {
			for _, user := range users {
				userNames[user.ID] = user.Name
			}
		}
	}

	base := buildBasePageData(c, "cases", "Support Cases", pageSubtitle)

	c.HTML(http.StatusOK, "cases.html", ConvertCasesToPageData(cases, total, workspaceNames, userNames, base))
}

// ShowCaseDetail renders the case detail page with all communications
func (h *AdminManagementHandler) ShowCaseDetail(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	ctx := c.Request.Context()

	var (
		caseObj       *servicedomain.Case
		err           error
		workspaceName string
	)

	if workspaceID, currentWorkspaceName, ok := currentWorkspaceScope(c); ok {
		caseObj, err = h.caseService.GetCaseInWorkspace(ctx, workspaceID, caseID)
		if err != nil {
			c.HTML(http.StatusNotFound, "error.html", ErrorPageData{Error: "Case not found: "})
			return
		}
		workspaceName = currentWorkspaceName
	} else {
		// Use service layer directly - admin context middleware already set up RLS bypass
		caseObj, err = h.caseService.GetCase(ctx, caseID)
		if err != nil {
			c.HTML(http.StatusNotFound, "error.html", ErrorPageData{Error: "Case not found: "})
			return
		}

		// Get workspace name
		if ws, err := h.workspaceService.GetWorkspace(ctx, caseObj.WorkspaceID); err == nil {
			workspaceName = ws.Name
		}
	}

	// Get assignee name
	assigneeName := ""
	if caseObj.AssignedToID != "" {
		if user, err := h.userService.GetUser(ctx, caseObj.AssignedToID); err == nil {
			assigneeName = user.Name
		}
	}

	// Get communications
	communications, err := h.caseService.GetCaseCommunications(ctx, caseID)
	if err != nil {
		slog.Warn("Failed to get case communications", "case_id", caseID, "error", err)
	}

	// Get available users for assignment
	availableUsers := []UserOptionItem{}
	if _, _, ok := currentWorkspaceScope(c); !ok {
		usersWithStats, err := h.userService.ListAllUsers(ctx) //nolint:govet // err shadow intentional
		if err != nil {
			slog.Warn("Failed to list users for assignment", "error", err)
		}
		availableUsers = make([]UserOptionItem, 0, len(usersWithStats))
		for _, u := range usersWithStats {
			availableUsers = append(availableUsers, UserOptionItem{
				ID:    u.User.ID,
				Email: u.User.Email,
				Name:  u.User.Name,
			})
		}
	}

	base := buildBasePageData(c, "cases", fmt.Sprintf("Case %s", caseObj.HumanID), "")

	c.HTML(http.StatusOK, "case_detail.html", ConvertCaseDetailToPageData(
		caseObj,
		workspaceName,
		assigneeName,
		communications,
		availableUsers,
		base,
	))
}

// ==================== CASE ACTION HANDLERS ====================

type AssignCaseRequest struct {
	UserID string `json:"user_id" form:"user_id"`
	TeamID string `json:"team_id" form:"team_id"`
}

func (h *AdminManagementHandler) AssignCase(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	var req AssignCaseRequest
	if err := c.ShouldBind(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	err := h.caseService.AssignCase(c.Request.Context(), caseID, req.UserID, req.TeamID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to assign case")
		return
	}

	RespondWithSuccessOrRedirect(c, "Case assigned successfully", "/cases/"+caseID)
}

func (h *AdminManagementHandler) UnassignCase(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	err := h.caseService.UnassignCase(c.Request.Context(), caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to unassign case")
		return
	}

	RespondWithSuccessOrRedirect(c, "Case unassigned successfully", "/cases/"+caseID)
}

type SetPriorityRequest struct {
	Priority string `json:"priority" form:"priority" binding:"required"`
}

func (h *AdminManagementHandler) SetCasePriority(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	var req SetPriorityRequest
	if err := c.ShouldBind(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	priority := servicedomain.CasePriority(req.Priority)
	err := h.caseService.SetCasePriority(c.Request.Context(), caseID, priority)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to update priority")
		return
	}

	RespondWithSuccessOrRedirect(c, "Priority updated successfully", "/cases/"+caseID)
}

type SetStatusRequest struct {
	Status string `json:"status" form:"status" binding:"required"`
}

func (h *AdminManagementHandler) SetCaseStatus(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	var req SetStatusRequest
	if err := c.ShouldBind(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	status := servicedomain.CaseStatus(req.Status)
	err := h.caseService.SetCaseStatus(c.Request.Context(), caseID, status)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to update status")
		return
	}

	RespondWithSuccessOrRedirect(c, "Status updated successfully", "/cases/"+caseID)
}

func (h *AdminManagementHandler) ResolveCase(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	err := h.caseService.MarkCaseResolved(c.Request.Context(), caseID, time.Now())
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to resolve case")
		return
	}

	RespondWithSuccessOrRedirect(c, "Case resolved successfully", "/cases/"+caseID)
}

func (h *AdminManagementHandler) CloseCase(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	err := h.caseService.CloseCase(c.Request.Context(), caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to close case")
		return
	}

	RespondWithSuccessOrRedirect(c, "Case closed successfully", "/cases/"+caseID)
}

func (h *AdminManagementHandler) ReopenCase(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	err := h.caseService.ReopenCase(c.Request.Context(), caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to reopen case")
		return
	}

	RespondWithSuccessOrRedirect(c, "Case reopened successfully", "/cases/"+caseID)
}

type AddNoteRequest struct {
	Body string `json:"body" form:"body" binding:"required"`
}

func (h *AdminManagementHandler) AddCaseNote(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	var req AddNoteRequest
	if err := c.ShouldBind(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Get case to find workspace
	caseObj, err := h.caseService.GetCase(c.Request.Context(), caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Case not found")
		return
	}

	ctxValues := GetContextValues(c)
	userID := MustGetUserID(c)
	if c.IsAborted() {
		return
	}

	userName := "Admin"
	if ctxValues.UserName != "" {
		userName = ctxValues.UserName
	}

	_, err = h.caseService.AddInternalNote(c.Request.Context(), caseID, caseObj.WorkspaceID, userID, userName, req.Body)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to add note")
		return
	}

	RespondWithSuccessOrRedirect(c, "Note added successfully", "/cases/"+caseID)
}

type ReplyRequest struct {
	Body    string `json:"body" form:"body" binding:"required"`
	Subject string `json:"subject" form:"subject"`
}

func (h *AdminManagementHandler) ReplyCaseToCustomer(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	var req ReplyRequest
	if err := c.ShouldBind(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	caseObj, err := h.caseService.GetCase(c.Request.Context(), caseID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Case not found")
		return
	}

	ctx := GetContextValues(c)
	userID := MustGetUserID(c)
	if c.IsAborted() {
		return
	}

	userNameStr := "Admin"
	if ctx.UserName != "" {
		userNameStr = ctx.UserName
	}

	subject := req.Subject
	if subject == "" {
		subject = "Re: " + caseObj.Subject
	}

	params := serviceapp.ReplyToCaseParams{
		CaseID:      caseID,
		WorkspaceID: caseObj.WorkspaceID,
		UserID:      userID,
		UserName:    userNameStr,
		UserEmail:   ctx.UserEmail,
		Body:        req.Body,
		ToEmails:    []string{caseObj.ContactEmail},
		Subject:     subject,
	}

	_, err = h.caseService.ReplyToCase(c.Request.Context(), params)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to send reply")
		return
	}

	RespondWithSuccessOrRedirect(c, "Reply sent successfully", "/cases/"+caseID)
}

type AddTagRequest struct {
	Tag string `json:"tag" form:"tag" binding:"required"`
}

func (h *AdminManagementHandler) AddCaseTag(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	var req AddTagRequest
	if err := c.ShouldBind(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	err := h.caseService.AddCaseTag(c.Request.Context(), caseID, req.Tag)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to add tag")
		return
	}

	RespondWithSuccessOrRedirect(c, "Tag added successfully", "/cases/"+caseID)
}

func (h *AdminManagementHandler) RemoveCaseTag(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}
	tag := c.Param("tag")
	if tag == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "tag is required")
		return
	}

	err := h.caseService.RemoveCaseTag(c.Request.Context(), caseID, tag)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to remove tag")
		return
	}

	RespondWithSuccessOrRedirect(c, "Tag removed successfully", "/cases/"+caseID)
}

type SetCategoryRequest struct {
	Category string `json:"category" form:"category"`
}

func (h *AdminManagementHandler) SetCaseCategory(c *gin.Context) {
	caseID := middleware.ValidateUUIDParam(c, "id")
	if caseID == "" {
		return
	}

	var req SetCategoryRequest
	if err := c.ShouldBind(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	err := h.caseService.SetCaseCategory(c.Request.Context(), caseID, req.Category)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Failed to update category")
		return
	}

	RespondWithSuccessOrRedirect(c, "Category updated successfully", "/cases/"+caseID)
}
