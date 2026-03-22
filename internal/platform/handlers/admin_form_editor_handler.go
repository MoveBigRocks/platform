package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

// Form API handlers

// ShowFormEdit renders the form create/edit page
func (h *AdminManagementHandler) ShowFormEdit(c *gin.Context) {
	ctx := c.Request.Context()
	formID := c.Param("id")

	// Get all workspaces for dropdown
	workspaces, err := h.workspaceService.ListAllWorkspaces(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", ErrorPageData{
			Error: "Failed to load workspaces: ",
		})
		return
	}

	// Default workspace ID (first workspace)
	workspaceID := ""
	if len(workspaces) > 0 {
		workspaceID = workspaces[0].ID
	}

	// Get teams for auto-assignment dropdown
	var teams []map[string]interface{}
	for _, ws := range workspaces {
		wsTeams, err := h.workspaceService.ListWorkspaceTeams(ctx, ws.ID)
		if err == nil {
			for _, team := range wsTeams {
				teams = append(teams, map[string]interface{}{
					"ID":          team.ID,
					"Name":        team.Name,
					"WorkspaceID": team.WorkspaceID,
				})
			}
		}
	}

	data := buildAdminTemplateContext(c, "forms", "New Form", "Create a new support form")
	data["Workspaces"] = workspaces
	data["WorkspaceID"] = workspaceID
	data["Teams"] = teams

	// If editing existing form
	if formID != "" && formID != "new" {
		form, err := h.formService.GetForm(ctx, formID)
		if err != nil {
			c.HTML(http.StatusNotFound, "error.html", ErrorPageData{
				Error: "Form not found: ",
			})
			return
		}

		// Parse fields from SchemaData for the template
		var fields []map[string]interface{}
		if !form.SchemaData.IsEmpty() {
			for _, name := range form.SchemaData.Properties() {
				fieldSchema, ok := form.SchemaData.GetField(name)
				if ok {
					fields = append(fields, map[string]interface{}{
						"Name":     name,
						"Type":     fieldSchema.Type(),
						"Label":    fieldSchema.Title(),
						"Required": form.SchemaData.IsRequired(name),
					})
				}
			}
		}

		data["Form"] = gin.H{
			"ID":                 form.ID,
			"Name":               form.Name,
			"Slug":               form.Slug,
			"Description":        form.Description,
			"WorkspaceID":        form.WorkspaceID,
			"Status":             string(form.Status),
			"IsPublic":           form.IsPublic,
			"RequiresCaptcha":    form.RequiresCaptcha,
			"CollectEmail":       form.CollectEmail,
			"AutoCreateCase":     form.AutoCreateCase,
			"AutoCasePriority":   form.AutoCasePriority,
			"AutoCaseType":       form.AutoCaseType,
			"AutoAssignTeamID":   form.AutoAssignTeamID,
			"AutoTags":           form.AutoTags,
			"NotifyOnSubmission": form.NotifyOnSubmission,
			"NotificationEmails": form.NotificationEmails,
			"SubmissionMessage":  form.SubmissionMessage,
			"RedirectURL":        form.RedirectURL,
			"Fields":             fields,
		}
		data["WorkspaceID"] = form.WorkspaceID
		data["PageTitle"] = "Edit Form"
		data["PageSubtitle"] = form.Name
	}

	c.HTML(http.StatusOK, "form_edit.html", data)
}

// ShowFormBuilder renders the visual form builder page
func (h *AdminManagementHandler) ShowFormBuilder(c *gin.Context) {
	ctx := c.Request.Context()
	formID := c.Param("id")

	// Get all workspaces for dropdown
	workspaces, err := h.workspaceService.ListAllWorkspaces(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", ErrorPageData{
			Error: "Failed to load workspaces: ",
		})
		return
	}

	// Default workspace ID
	workspaceID := ""
	if len(workspaces) > 0 {
		workspaceID = workspaces[0].ID
	}

	data := buildAdminTemplateContext(c, "forms", "Form Builder", "Create a new form")
	data["Workspaces"] = workspaces
	data["WorkspaceID"] = workspaceID

	// If editing existing form
	if formID != "" && formID != "new" {
		form, err := h.formService.GetForm(ctx, formID)
		if err != nil {
			c.HTML(http.StatusNotFound, "error.html", ErrorPageData{
				Error: "Form not found: ",
			})
			return
		}

		data["Form"] = gin.H{
			"ID":                form.ID,
			"Name":              form.Name,
			"Slug":              form.Slug,
			"Description":       form.Description,
			"WorkspaceID":       form.WorkspaceID,
			"Status":            string(form.Status),
			"IsPublic":          form.IsPublic,
			"RequiresCaptcha":   form.RequiresCaptcha,
			"AutoCreateCase":    form.AutoCreateCase,
			"SubmissionMessage": form.SubmissionMessage,
			"SchemaData":        form.SchemaData,
		}
		data["WorkspaceID"] = form.WorkspaceID
		data["PageTitle"] = "Edit Form"
		data["PageSubtitle"] = form.Name
	}

	c.HTML(http.StatusOK, "form_builder.html", data)
}

// CreateForm creates a new form
func (h *AdminManagementHandler) CreateForm(c *gin.Context) {
	var req struct {
		Name               string   `json:"name"`
		Slug               string   `json:"slug"`
		Description        string   `json:"description"`
		WorkspaceID        string   `json:"workspace_id"`
		Status             string   `json:"status"`
		IsPublic           bool     `json:"is_public"`
		RequiresCaptcha    bool     `json:"requires_captcha"`
		CollectEmail       bool     `json:"collect_email"`
		AutoCreateCase     bool     `json:"auto_create_case"`
		AutoCasePriority   string   `json:"auto_case_priority"`
		AutoCaseType       string   `json:"auto_case_type"`
		AutoAssignTeamID   string   `json:"auto_assign_team_id"`
		AutoTags           []string `json:"auto_tags"`
		NotifyOnSubmission bool     `json:"notify_on_submission"`
		NotificationEmails []string `json:"notification_emails"`
		SubmissionMessage  string   `json:"submission_message"`
		RedirectURL        string   `json:"redirect_url"`
		Fields             []struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Label    string `json:"label"`
			Required bool   `json:"required"`
			Order    int    `json:"order"`
		} `json:"fields"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.Name == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "Name is required")
		return
	}

	if req.Slug == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "Slug is required")
		return
	}

	if req.WorkspaceID == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "Workspace ID is required")
		return
	}

	// Build SchemaData from fields
	schemaDataMap := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}
	properties := schemaDataMap["properties"].(map[string]interface{})
	required := []string{}

	for _, f := range req.Fields {
		properties[f.Name] = map[string]interface{}{
			"type":  f.Type,
			"title": f.Label,
		}
		if f.Required {
			required = append(required, f.Name)
		}
	}
	schemaDataMap["required"] = required

	// Map status
	status := servicedomain.FormStatusDraft
	switch req.Status {
	case "active":
		status = servicedomain.FormStatusActive
	case "inactive":
		status = servicedomain.FormStatusInactive
	}

	form := &servicedomain.FormSchema{
		WorkspaceID:        req.WorkspaceID,
		Name:               req.Name,
		Slug:               req.Slug,
		Description:        req.Description,
		Status:             status,
		IsPublic:           req.IsPublic,
		RequiresCaptcha:    req.RequiresCaptcha,
		CollectEmail:       req.CollectEmail,
		AutoCreateCase:     req.AutoCreateCase,
		AutoCasePriority:   req.AutoCasePriority,
		AutoCaseType:       req.AutoCaseType,
		AutoAssignTeamID:   req.AutoAssignTeamID,
		AutoTags:           req.AutoTags,
		NotifyOnSubmission: req.NotifyOnSubmission,
		NotificationEmails: req.NotificationEmails,
		SubmissionMessage:  req.SubmissionMessage,
		RedirectURL:        req.RedirectURL,
		SchemaData:         shareddomain.TypedSchemaFromMap(schemaDataMap),
	}

	err := h.formService.CreateForm(c.Request.Context(), form)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to create form")
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": form.ID, "success": true})
}

// UpdateForm updates an existing form
func (h *AdminManagementHandler) UpdateForm(c *gin.Context) {
	formID := middleware.ValidateUUIDParam(c, "id")
	if formID == "" {
		return // Error already sent
	}

	var req struct {
		Name               string   `json:"name"`
		Slug               string   `json:"slug"`
		Description        string   `json:"description"`
		Status             string   `json:"status"`
		IsPublic           bool     `json:"is_public"`
		RequiresCaptcha    bool     `json:"requires_captcha"`
		CollectEmail       bool     `json:"collect_email"`
		AutoCreateCase     bool     `json:"auto_create_case"`
		AutoCasePriority   string   `json:"auto_case_priority"`
		AutoCaseType       string   `json:"auto_case_type"`
		AutoAssignTeamID   string   `json:"auto_assign_team_id"`
		AutoTags           []string `json:"auto_tags"`
		NotifyOnSubmission bool     `json:"notify_on_submission"`
		NotificationEmails []string `json:"notification_emails"`
		SubmissionMessage  string   `json:"submission_message"`
		RedirectURL        string   `json:"redirect_url"`
		Fields             []struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Label    string `json:"label"`
			Required bool   `json:"required"`
			Order    int    `json:"order"`
		} `json:"fields"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Get existing form
	form, err := h.formService.GetForm(c.Request.Context(), formID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Form not found")
		return
	}

	// Build SchemaData from fields
	schemaDataMap := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}
	updateProps := schemaDataMap["properties"].(map[string]interface{})
	updateRequired := []string{}

	for _, f := range req.Fields {
		updateProps[f.Name] = map[string]interface{}{
			"type":  f.Type,
			"title": f.Label,
		}
		if f.Required {
			updateRequired = append(updateRequired, f.Name)
		}
	}
	schemaDataMap["required"] = updateRequired

	// Map status
	status := form.Status
	switch req.Status {
	case "active":
		status = servicedomain.FormStatusActive
	case "inactive":
		status = servicedomain.FormStatusInactive
	case "draft":
		status = servicedomain.FormStatusDraft
	}

	// Update form fields
	form.Name = req.Name
	form.Slug = req.Slug
	form.Description = req.Description
	form.Status = status
	form.IsPublic = req.IsPublic
	form.RequiresCaptcha = req.RequiresCaptcha
	form.CollectEmail = req.CollectEmail
	form.AutoCreateCase = req.AutoCreateCase
	form.AutoCasePriority = req.AutoCasePriority
	form.AutoCaseType = req.AutoCaseType
	form.AutoAssignTeamID = req.AutoAssignTeamID
	form.AutoTags = req.AutoTags
	form.NotifyOnSubmission = req.NotifyOnSubmission
	form.NotificationEmails = req.NotificationEmails
	form.SubmissionMessage = req.SubmissionMessage
	form.RedirectURL = req.RedirectURL
	form.SchemaData = shareddomain.TypedSchemaFromMap(schemaDataMap)

	err = h.formService.UpdateForm(c.Request.Context(), form)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to update form")
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": form.ID, "success": true})
}

// DeleteForm deletes a form
func (h *AdminManagementHandler) DeleteForm(c *gin.Context) {
	formID := middleware.ValidateUUIDParam(c, "id")
	if formID == "" {
		return // Error already sent
	}

	ctx := c.Request.Context()

	// Fetch form to get workspace ID for workspace-scoped delete
	form, err := h.formService.GetForm(ctx, formID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Form not found")
		return
	}

	err = h.formService.DeleteForm(ctx, form.WorkspaceID, formID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to delete form")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
