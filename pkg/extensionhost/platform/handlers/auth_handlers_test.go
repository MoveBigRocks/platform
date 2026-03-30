package platformhandlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewAuthHandler(t *testing.T) {
	handler := NewAuthHandler(
		nil, // sessionService
		"http://localhost:8080",
		"development",
		[]string{"admin@example.com", "super@example.com"},
		"", // cookieDomain
	)

	assert.NotNil(t, handler)
	assert.Equal(t, "http://localhost:8080", handler.baseURL)
	assert.Equal(t, "development", handler.environment)
	assert.True(t, handler.allowedEmails["admin@example.com"])
	assert.True(t, handler.allowedEmails["super@example.com"])
	assert.False(t, handler.allowedEmails["unknown@example.com"])
}

func TestAuthHandler_HandleMagicLinkRequest_InvalidEmail(t *testing.T) {
	handler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"development",
		nil,
		"",
	)

	router := gin.New()
	router.POST("/auth/magic-link", handler.HandleMagicLinkRequest)

	// Test missing email
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	// Just verify we got a response body
	assert.NotEmpty(t, w.Body.String())

	// Test invalid email format
	req = httptest.NewRequest(http.MethodPost, "/auth/magic-link", strings.NewReader(`{"email":"not-an-email"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_VerifyMagicLink_MissingToken(t *testing.T) {
	handler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"development",
		nil,
		"",
	)

	router := gin.New()
	router.GET("/auth/verify-magic-link", handler.VerifyMagicLink)

	// Test missing token
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-magic-link", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Just verify we got a 400 response - the error format may vary
	assert.NotEmpty(t, w.Body.String())
}

func TestAuthHandler_SwitchContext_NoSession(t *testing.T) {
	handler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"development",
		nil,
		"",
	)

	router := gin.New()
	router.POST("/auth/switch-context", handler.SwitchContext)

	req := httptest.NewRequest(http.MethodPost, "/auth/switch-context", strings.NewReader(`{"type":"workspace"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	// Just verify we got an error response
	assert.NotEmpty(t, w.Body.String())
}

func TestAuthHandler_SwitchContext_InvalidRequest(t *testing.T) {
	handler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"development",
		nil,
		"",
	)

	router := gin.New()

	// Add middleware to set session
	router.Use(func(c *gin.Context) {
		session := &platformdomain.Session{
			UserID: "user-1",
		}
		c.Set("session", session)
		c.Next()
	})

	router.POST("/auth/switch-context", handler.SwitchContext)

	// Test invalid type
	req := httptest.NewRequest(http.MethodPost, "/auth/switch-context", strings.NewReader(`{"type":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_SwitchContext_WorkspaceWithoutID(t *testing.T) {
	handler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"development",
		nil,
		"",
	)

	router := gin.New()

	// Add middleware to set session
	router.Use(func(c *gin.Context) {
		session := &platformdomain.Session{
			UserID: "user-1",
		}
		c.Set("session", session)
		c.Next()
	})

	router.POST("/auth/switch-context", handler.SwitchContext)

	// Test workspace without workspace_id
	req := httptest.NewRequest(http.MethodPost, "/auth/switch-context", strings.NewReader(`{"type":"workspace"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	// Just verify we got an error response
	assert.NotEmpty(t, w.Body.String())
}

func TestAuthHandler_GetCurrentContext_NoSession(t *testing.T) {
	handler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"development",
		nil,
		"",
	)

	router := gin.New()
	router.GET("/auth/context", handler.GetCurrentContext)

	req := httptest.NewRequest(http.MethodGet, "/auth/context", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	// Just verify we got an error response
	assert.NotEmpty(t, w.Body.String())
}

func TestAuthHandler_GetCurrentContext_WithSession(t *testing.T) {
	handler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"development",
		nil,
		"",
	)

	router := gin.New()

	// Add middleware to set session
	router.Use(func(c *gin.Context) {
		session := &platformdomain.Session{
			UserID: "user-1",
			Email:  "test@example.com",
			Name:   "Test User",
			CurrentContext: platformdomain.Context{
				Type: platformdomain.ContextTypeInstance,
			},
			AvailableContexts: []platformdomain.Context{
				{Type: platformdomain.ContextTypeInstance},
			},
		}
		c.Set("session", session)
		c.Next()
	})

	router.GET("/auth/context", handler.GetCurrentContext)

	req := httptest.NewRequest(http.MethodGet, "/auth/context", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	user := response["user"].(map[string]interface{})
	assert.Equal(t, "user-1", user["id"])
	assert.Equal(t, "test@example.com", user["email"])
	assert.Equal(t, "Test User", user["name"])
}

func TestAuthHandler_Logout(t *testing.T) {
	handler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"development",
		nil,
		"",
	)

	router := gin.New()
	router.POST("/auth/logout", handler.Logout)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Logged out successfully", response["message"])

	// Check that the cookie is being cleared
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "mbr_session" {
			sessionCookie = cookie
			break
		}
	}

	assert.NotNil(t, sessionCookie)
	assert.Equal(t, "", sessionCookie.Value)
	assert.Less(t, sessionCookie.MaxAge, 0) // MaxAge < 0 means delete
}

func TestAuthHandler_StartCLILogin(t *testing.T) {
	service := platformservices.NewCLILoginService()
	t.Cleanup(service.Close)

	handler := NewAuthHandler(
		nil,
		"https://movebigrocks.example.com",
		"development",
		nil,
		"",
	).WithCLILogin("https://admin.movebigrocks.example.com", service)

	router := gin.New()
	router.POST("/auth/cli/start", handler.StartCLILogin)

	req := httptest.NewRequest(http.MethodPost, "/auth/cli/start", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.NotEmpty(t, response["requestID"])
	assert.NotEmpty(t, response["pollToken"])
	assert.Equal(t, "https://admin.movebigrocks.example.com", response["adminBaseURL"])
	assert.Equal(t, "https://admin.movebigrocks.example.com/cli-login?request_id="+response["requestID"].(string), response["authorizeURL"])
}

func TestAuthHandler_PollCLILogin(t *testing.T) {
	service := platformservices.NewCLILoginService()
	t.Cleanup(service.Close)

	start, err := service.Start()
	require.NoError(t, err)

	handler := NewAuthHandler(
		nil,
		"https://movebigrocks.example.com",
		"development",
		nil,
		"",
	).WithCLILogin("https://admin.movebigrocks.example.com", service)

	router := gin.New()
	router.POST("/auth/cli/poll", handler.PollCLILogin)

	req := httptest.NewRequest(http.MethodPost, "/auth/cli/poll", strings.NewReader(`{"pollToken":"`+start.PollToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"pending"`)

	require.NoError(t, service.Authorize(start.RequestID, "user_123", "session_abc"))

	req = httptest.NewRequest(http.MethodPost, "/auth/cli/poll", strings.NewReader(`{"pollToken":"`+start.PollToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ready"`)
	assert.Contains(t, w.Body.String(), `"userID":"user_123"`)
	assert.Contains(t, w.Body.String(), `"sessionToken":"session_abc"`)
}

func TestAuthHandler_getRedirectURLForContext(t *testing.T) {
	handler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"development",
		nil,
		"",
	)

	// Admin panel always redirects to /dashboard regardless of context type
	// The context affects what data is shown, not the URL structure

	// Test instance context
	ctx := platformdomain.Context{
		Type: platformdomain.ContextTypeInstance,
	}
	assert.Equal(t, "/dashboard", handler.getRedirectURLForContext(ctx))

	// Test workspace context
	workspaceID := "workspace-123"
	ctx = platformdomain.Context{
		Type:        platformdomain.ContextTypeWorkspace,
		WorkspaceID: &workspaceID,
	}
	assert.Equal(t, "/dashboard", handler.getRedirectURLForContext(ctx))

	// Test workspace context without workspace ID
	ctx = platformdomain.Context{
		Type: platformdomain.ContextTypeWorkspace,
	}
	assert.Equal(t, "/dashboard", handler.getRedirectURLForContext(ctx))

	// Test unknown context
	ctx = platformdomain.Context{
		Type: "unknown",
	}
	assert.Equal(t, "/dashboard", handler.getRedirectURLForContext(ctx))
}
