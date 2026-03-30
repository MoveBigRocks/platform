package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

func TestContextAuthMiddleware_IsHTMLRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	tests := []struct {
		name      string
		acceptHdr string
		expected  bool
	}{
		{
			name:      "HTML accept - starts with text/html",
			acceptHdr: "text/html,application/xhtml+xml",
			expected:  true,
		},
		{
			name:      "JSON accept",
			acceptHdr: "application/json",
			expected:  false,
		},
		{
			name:      "empty",
			acceptHdr: "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/test", nil)
			c.Request.Header.Set("Accept", tt.acceptHdr)

			result := m.isHTMLRequest(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRequireInstanceAccess_NoSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	router := gin.New()
	router.GET("/instance-protected", m.RequireInstanceAccess(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/instance-protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Not authenticated")
}

func TestRequireInstanceAccess_WithInstanceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	router := gin.New()
	router.GET("/instance-protected", func(c *gin.Context) {
		// Simulate authenticated session in instance context
		session := &platformdomain.Session{
			UserID: "user-123",
			CurrentContext: platformdomain.Context{
				Type: platformdomain.ContextTypeInstance,
				Role: string(platformdomain.InstanceRoleAdmin),
			},
		}
		c.Set("session", session)
		c.Next()
	}, m.RequireInstanceAccess(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/instance-protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireInstanceAccess_WithWorkspaceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	router := gin.New()
	router.GET("/instance-protected", func(c *gin.Context) {
		// Session in workspace context - should be denied
		wsID := "ws-123"
		session := &platformdomain.Session{
			UserID: "user-123",
			CurrentContext: platformdomain.Context{
				Type:        platformdomain.ContextTypeWorkspace,
				WorkspaceID: &wsID,
				Role:        string(platformdomain.WorkspaceRoleAdmin),
			},
		}
		c.Set("session", session)
		c.Next()
	}, m.RequireInstanceAccess(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/instance-protected", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "switch to Instance Admin context")
}

func TestRequireInstanceAccess_WithRoleCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		userRole     platformdomain.InstanceRole
		allowedRoles []platformdomain.InstanceRole
		expectedCode int
	}{
		{
			name:         "super admin bypasses role check",
			userRole:     platformdomain.InstanceRoleSuperAdmin,
			allowedRoles: []platformdomain.InstanceRole{platformdomain.InstanceRoleAdmin},
			expectedCode: http.StatusOK,
		},
		{
			name:         "admin with admin allowed",
			userRole:     platformdomain.InstanceRoleAdmin,
			allowedRoles: []platformdomain.InstanceRole{platformdomain.InstanceRoleAdmin},
			expectedCode: http.StatusOK,
		},
		{
			name:         "operator denied when admin required",
			userRole:     platformdomain.InstanceRoleOperator,
			allowedRoles: []platformdomain.InstanceRole{platformdomain.InstanceRoleAdmin},
			expectedCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ContextAuthMiddleware{}

			router := gin.New()
			router.GET("/instance-protected", func(c *gin.Context) {
				session := &platformdomain.Session{
					UserID: "user-123",
					CurrentContext: platformdomain.Context{
						Type: platformdomain.ContextTypeInstance,
						Role: string(tt.userRole),
					},
				}
				c.Set("session", session)
				c.Next()
			}, m.RequireInstanceAccess(tt.allowedRoles...), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			req := httptest.NewRequest("GET", "/instance-protected", nil)
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
		})
	}
}

func TestRequireCurrentWorkspaceAccess_NoSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	router := gin.New()
	router.GET("/app/test", m.RequireCurrentWorkspaceAccess(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/app/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireCurrentWorkspaceAccess_NoWorkspaceContextValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	router := gin.New()
	router.GET("/app/test", func(c *gin.Context) {
		wsID := "ws-123"
		session := &platformdomain.Session{
			UserID: "user-123",
			CurrentContext: platformdomain.Context{
				Type:        platformdomain.ContextTypeWorkspace,
				WorkspaceID: &wsID,
			},
		}
		c.Set("session", session)
		c.Next()
	}, m.RequireCurrentWorkspaceAccess(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/app/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "workspace_id required")
}

func TestRequireCurrentWorkspaceAccess_MatchingWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	router := gin.New()
	router.GET("/app/test", func(c *gin.Context) {
		wsID := "ws-123"
		session := &platformdomain.Session{
			UserID: "user-123",
			CurrentContext: platformdomain.Context{
				Type:        platformdomain.ContextTypeWorkspace,
				WorkspaceID: &wsID,
				Role:        string(platformdomain.WorkspaceRoleAdmin),
			},
		}
		c.Set("session", session)
		c.Set("workspace_id", wsID)
		c.Next()
	}, m.RequireCurrentWorkspaceAccess(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/app/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireCurrentWorkspaceAccess_MismatchedWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	router := gin.New()
	router.GET("/app/test", func(c *gin.Context) {
		wsID := "ws-123"
		session := &platformdomain.Session{
			UserID: "user-123",
			CurrentContext: platformdomain.Context{
				Type:        platformdomain.ContextTypeWorkspace,
				WorkspaceID: &wsID,
				Role:        string(platformdomain.WorkspaceRoleAdmin),
			},
		}
		c.Set("session", session)
		c.Set("workspace_id", "ws-different")
		c.Next()
	}, m.RequireCurrentWorkspaceAccess(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/app/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Context mismatch")
}

func TestRequireCurrentWorkspaceAccess_RoleCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		userRole     platformdomain.WorkspaceRole
		allowedRoles []platformdomain.WorkspaceRole
		expectedCode int
	}{
		{
			name:         "owner bypasses role check",
			userRole:     platformdomain.WorkspaceRoleOwner,
			allowedRoles: []platformdomain.WorkspaceRole{platformdomain.WorkspaceRoleMember},
			expectedCode: http.StatusOK,
		},
		{
			name:         "admin bypasses role check",
			userRole:     platformdomain.WorkspaceRoleAdmin,
			allowedRoles: []platformdomain.WorkspaceRole{platformdomain.WorkspaceRoleMember},
			expectedCode: http.StatusOK,
		},
		{
			name:         "member with member allowed",
			userRole:     platformdomain.WorkspaceRoleMember,
			allowedRoles: []platformdomain.WorkspaceRole{platformdomain.WorkspaceRoleMember},
			expectedCode: http.StatusOK,
		},
		{
			name:         "viewer with member required",
			userRole:     platformdomain.WorkspaceRoleViewer,
			allowedRoles: []platformdomain.WorkspaceRole{platformdomain.WorkspaceRoleMember},
			expectedCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ContextAuthMiddleware{}

			router := gin.New()
			router.GET("/app/test", func(c *gin.Context) {
				wsID := "ws-123"
				session := &platformdomain.Session{
					UserID: "user-123",
					CurrentContext: platformdomain.Context{
						Type:        platformdomain.ContextTypeWorkspace,
						WorkspaceID: &wsID,
						Role:        string(tt.userRole),
					},
				}
				c.Set("session", session)
				c.Set("workspace_id", wsID)
				c.Next()
			}, m.RequireCurrentWorkspaceAccess(tt.allowedRoles...), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			req := httptest.NewRequest("GET", "/app/test", nil)
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
		})
	}
}

func TestRequireOperationalAccess_InstanceOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	router := gin.New()
	router.GET("/dashboard", func(c *gin.Context) {
		session := &platformdomain.Session{
			UserID: "user-123",
			CurrentContext: platformdomain.Context{
				Type: platformdomain.ContextTypeInstance,
				Role: string(platformdomain.InstanceRoleOperator),
			},
		}
		c.Set("session", session)
		c.Next()
	}, m.RequireOperationalAccess(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"instance_role": c.GetString("instance_role")})
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "operator")
}

func TestRequireOperationalAccess_WorkspaceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &ContextAuthMiddleware{}

	router := gin.New()
	router.GET("/dashboard", func(c *gin.Context) {
		wsID := "ws-123"
		session := &platformdomain.Session{
			UserID: "user-123",
			CurrentContext: platformdomain.Context{
				Type:        platformdomain.ContextTypeWorkspace,
				WorkspaceID: &wsID,
				Role:        string(platformdomain.WorkspaceRoleMember),
			},
		}
		c.Set("session", session)
		c.Next()
	}, m.RequireOperationalAccess(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"workspace_id": c.GetString("workspace_id"),
		})
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ws-123")
}

func TestNewContextAuthMiddleware(t *testing.T) {
	m := NewContextAuthMiddleware(nil)
	assert.NotNil(t, m)
	assert.Nil(t, m.sessionService)
	assert.Nil(t, m.store)
}

func TestContextAuthMiddleware_WithStore(t *testing.T) {
	m := NewContextAuthMiddleware(nil)
	assert.Nil(t, m.store)

	// WithStore should return same instance with store set
	result := m.WithStore(nil)
	assert.Same(t, m, result)
}
