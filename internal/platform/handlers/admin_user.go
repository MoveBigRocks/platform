package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

// User management handlers

// ShowUsers renders the users management page
func (h *AdminManagementHandler) ShowUsers(c *gin.Context) {
	ctx := c.Request.Context()

	users, err := h.userService.ListAllUsers(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", ErrorPageData{
			Error: "Failed to load users: ",
		})
		return
	}

	data := buildAdminTemplateContext(c, "users", "Users", "Manage user accounts and roles")
	data["Users"] = users
	c.HTML(http.StatusOK, "users.html", data)
}

// CreateUser handles user creation (API)
func (h *AdminManagementHandler) CreateUser(c *gin.Context) {
	var req struct {
		Name          string  `json:"name" binding:"required"`
		Email         string  `json:"email" binding:"required,email"`
		InstanceRole  *string `json:"instance_role"`
		IsActive      bool    `json:"is_active"`
		EmailVerified bool    `json:"email_verified"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Convert instance role string to domain type
	var instanceRole *platformdomain.InstanceRole
	if req.InstanceRole != nil && *req.InstanceRole != "" {
		role := platformdomain.InstanceRole(*req.InstanceRole)
		instanceRole = &role
	}

	// Create user via service
	user, err := h.userService.CreateUser(c.Request.Context(), req.Email, req.Name, instanceRole)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Update additional fields if specified (isActive defaults to true, so only update if false)
	if !req.IsActive || req.EmailVerified {
		err := h.userService.UpdateUser(c.Request.Context(), user.ID, req.Email, req.Name, instanceRole, req.IsActive, req.EmailVerified)
		if err != nil {
			// User created but failed to update fields - non-fatal
			c.JSON(http.StatusCreated, gin.H{
				"user":    user,
				"warning": "User created but failed to update some fields",
			})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"ID":           user.ID,
		"Email":        user.Email,
		"Name":         user.Name,
		"InstanceRole": user.InstanceRole,
		"IsActive":     user.IsActive,
	})
}

// UpdateUser handles user updates (API)
func (h *AdminManagementHandler) UpdateUser(c *gin.Context) {
	userID := middleware.ValidateUUIDParam(c, "id")
	if userID == "" {
		return
	}

	var req struct {
		Name          string  `json:"name" binding:"required"`
		Email         string  `json:"email" binding:"required,email"`
		InstanceRole  *string `json:"instance_role"`
		IsActive      bool    `json:"is_active"`
		EmailVerified bool    `json:"email_verified"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Convert instance role string to domain type
	var instanceRole *platformdomain.InstanceRole
	if req.InstanceRole != nil && *req.InstanceRole != "" {
		role := platformdomain.InstanceRole(*req.InstanceRole)
		instanceRole = &role
	}

	// Update user via service
	err := h.userService.UpdateUser(c.Request.Context(), userID, req.Email, req.Name, instanceRole, req.IsActive, req.EmailVerified)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to update user")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// UpdateUserStatus handles user status toggle (API)
func (h *AdminManagementHandler) UpdateUserStatus(c *gin.Context) {
	userID := middleware.ValidateUUIDParam(c, "id")
	if userID == "" {
		return
	}

	var req struct {
		IsActive bool `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Toggle user status via service
	err := h.userService.ToggleUserStatus(c.Request.Context(), userID, req.IsActive)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to update user status")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User status updated successfully"})
}

// GetUser returns a single user by ID (API)
func (h *AdminManagementHandler) GetUser(c *gin.Context) {
	userID := middleware.ValidateUUIDParam(c, "id")
	if userID == "" {
		return
	}

	// Get user via service
	user, err := h.userService.GetUser(c.Request.Context(), userID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "User not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ID":           user.ID,
		"Email":        user.Email,
		"Name":         user.Name,
		"InstanceRole": user.InstanceRole,
		"IsActive":     user.IsActive,
	})
}

// GetUserWorkspaces returns all workspaces a user belongs to (API)
func (h *AdminManagementHandler) GetUserWorkspaces(c *gin.Context) {
	userID := middleware.ValidateUUIDParam(c, "id")
	if userID == "" {
		return
	}

	// Get user with workspaces via service
	result, err := h.userService.GetUserWithWorkspaces(c.Request.Context(), userID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to get user workspaces")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workspaces": result.Workspaces,
		"count":      len(result.Workspaces),
	})
}

// DeleteUser handles user deletion (API)
func (h *AdminManagementHandler) DeleteUser(c *gin.Context) {
	userID := middleware.ValidateUUIDParam(c, "id")
	if userID == "" {
		return
	}

	// Delete user via service
	err := h.userService.DeleteUser(c.Request.Context(), userID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}
