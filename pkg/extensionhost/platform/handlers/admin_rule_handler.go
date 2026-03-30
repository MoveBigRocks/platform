package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/middleware"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// Rule API handlers

// ShowRuleEdit renders the rule create/edit page
func (h *AdminManagementHandler) ShowRuleEdit(c *gin.Context) {
	ctx := c.Request.Context()
	ruleID := c.Param("id")

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

	data := buildAdminTemplateContext(c, "rules", "New Rule", "Create a new automation rule")
	data["Workspaces"] = workspaces
	data["WorkspaceID"] = workspaceID

	// If editing existing rule
	if ruleID != "" && ruleID != "new" {
		rule, err := h.ruleService.GetRule(ctx, ruleID)
		if err != nil {
			c.HTML(http.StatusNotFound, "error.html", ErrorPageData{
				Error: "Rule not found: ",
			})
			return
		}

		// Extract conditions and actions for the template
		var parsedConditions []map[string]interface{}
		for _, cond := range rule.Conditions.Conditions {
			parsedConditions = append(parsedConditions, map[string]interface{}{
				"type":     cond.Type,
				"field":    cond.Field,
				"operator": cond.Operator,
				"value":    cond.Value,
			})
		}

		var parsedActions []map[string]interface{}
		for _, act := range rule.Actions.Actions {
			parsedActions = append(parsedActions, map[string]interface{}{
				"type":   act.Type,
				"target": act.Target,
				"value":  act.Value,
				"field":  act.Field,
			})
		}

		data["Rule"] = gin.H{
			"ID":                   rule.ID,
			"Title":                rule.Title,
			"Description":          rule.Description,
			"WorkspaceID":          rule.WorkspaceID,
			"IsActive":             rule.IsActive,
			"Priority":             rule.Priority,
			"MaxExecutionsPerHour": rule.MaxExecutionsPerHour,
			"MaxExecutionsPerDay":  rule.MaxExecutionsPerDay,
			"ParsedConditions":     parsedConditions,
			"ParsedActions":        parsedActions,
		}
		data["WorkspaceID"] = rule.WorkspaceID
		data["PageTitle"] = "Edit Rule"
		data["PageSubtitle"] = rule.Title
	}

	c.HTML(http.StatusOK, "rule_edit.html", data)
}

// CreateRule creates a new rule
func (h *AdminManagementHandler) CreateRule(c *gin.Context) {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		WorkspaceID          string                   `json:"workspace_id"`
		Title                string                   `json:"title"`
		Description          string                   `json:"description"`
		IsActive             bool                     `json:"is_active"`
		Priority             int                      `json:"priority"`
		MaxExecutionsPerHour int                      `json:"max_executions_per_hour"`
		MaxExecutionsPerDay  int                      `json:"max_executions_per_day"`
		Conditions           []map[string]interface{} `json:"conditions"`
		Actions              []map[string]interface{} `json:"actions"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Convert conditions to typed format
	conditions := make([]automationdomain.TypedCondition, 0, len(req.Conditions))
	for _, cond := range req.Conditions {
		conditions = append(conditions, automationdomain.TypedCondition{
			Type:     getStringFromMap(cond, "type"),
			Field:    getStringFromMap(cond, "field"),
			Operator: getStringFromMap(cond, "operator"),
			Value:    shareddomain.ValueFromInterface(cond["value"]),
		})
	}

	// Convert actions to typed format
	actions := make([]automationdomain.TypedAction, 0, len(req.Actions))
	for _, act := range req.Actions {
		actions = append(actions, automationdomain.TypedAction{
			Type:   getStringFromMap(act, "type"),
			Target: getStringFromMap(act, "target"),
			Field:  getStringFromMap(act, "field"),
			Value:  shareddomain.ValueFromInterface(act["value"]),
		})
	}

	// Get user ID for CreatedByID
	userID := ""
	if authCtx.Principal != nil {
		userID = authCtx.Principal.GetID()
	}

	rule, err := h.ruleService.CreateRule(c.Request.Context(), automationservices.CreateRuleParams{
		WorkspaceID:          req.WorkspaceID,
		Title:                req.Title,
		Description:          req.Description,
		IsActive:             req.IsActive,
		Priority:             req.Priority,
		MaxExecutionsPerHour: req.MaxExecutionsPerHour,
		MaxExecutionsPerDay:  req.MaxExecutionsPerDay,
		Conditions:           conditions,
		Actions:              actions,
		CreatedByID:          userID,
	})
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to create rule")
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": rule.ID, "success": true})
}

// UpdateRule updates an existing rule
func (h *AdminManagementHandler) UpdateRule(c *gin.Context) {
	ruleID := middleware.ValidateUUIDParam(c, "id")
	if ruleID == "" {
		return // Error already sent by ValidateUUIDParam
	}

	var req struct {
		Title                string                   `json:"title"`
		Description          string                   `json:"description"`
		IsActive             bool                     `json:"is_active"`
		Priority             int                      `json:"priority"`
		MaxExecutionsPerHour int                      `json:"max_executions_per_hour"`
		MaxExecutionsPerDay  int                      `json:"max_executions_per_day"`
		Conditions           []map[string]interface{} `json:"conditions"`
		Actions              []map[string]interface{} `json:"actions"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Convert conditions to typed format
	conditions := make([]automationdomain.TypedCondition, 0, len(req.Conditions))
	for _, cond := range req.Conditions {
		conditions = append(conditions, automationdomain.TypedCondition{
			Type:     getStringFromMap(cond, "type"),
			Field:    getStringFromMap(cond, "field"),
			Operator: getStringFromMap(cond, "operator"),
			Value:    shareddomain.ValueFromInterface(cond["value"]),
		})
	}

	// Convert actions to typed format
	actions := make([]automationdomain.TypedAction, 0, len(req.Actions))
	for _, act := range req.Actions {
		actions = append(actions, automationdomain.TypedAction{
			Type:   getStringFromMap(act, "type"),
			Target: getStringFromMap(act, "target"),
			Field:  getStringFromMap(act, "field"),
			Value:  shareddomain.ValueFromInterface(act["value"]),
		})
	}

	rule, err := h.ruleService.UpdateRule(c.Request.Context(), ruleID, automationservices.UpdateRuleParams{
		Title:                req.Title,
		Description:          req.Description,
		IsActive:             req.IsActive,
		Priority:             req.Priority,
		MaxExecutionsPerHour: req.MaxExecutionsPerHour,
		MaxExecutionsPerDay:  req.MaxExecutionsPerDay,
		Conditions:           conditions,
		Actions:              actions,
	})
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to update rule")
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": rule.ID, "success": true})
}

// DeleteRule deletes a rule
func (h *AdminManagementHandler) DeleteRule(c *gin.Context) {
	ruleID := middleware.ValidateUUIDParam(c, "id")
	if ruleID == "" {
		return // Error already sent by ValidateUUIDParam
	}

	err := h.ruleService.DeleteRule(c.Request.Context(), ruleID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to delete rule")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// getStringFromMap safely extracts a string value from a map
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
