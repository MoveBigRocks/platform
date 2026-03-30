package platformhandlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// =============================================================================
// GetContextValues Tests
// =============================================================================

func TestGetContextValues_AllPresent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	// Set all context values
	session := &platformdomain.Session{
		UserID: "user-123",
		Email:  "test@example.com",
		Name:   "Test User",
	}
	c.Set("user_id", "user-123")
	c.Set("name", "Test User")
	c.Set("email", "test@example.com")
	c.Set("session", session)

	cv := GetContextValues(c)

	assert.Equal(t, "user-123", cv.UserID)
	assert.Equal(t, "Test User", cv.UserName)
	assert.Equal(t, "test@example.com", cv.UserEmail)
	assert.Same(t, session, cv.Session)
}

func TestGetContextValues_NonePresent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	cv := GetContextValues(c)

	assert.Empty(t, cv.UserID)
	assert.Empty(t, cv.UserName)
	assert.Empty(t, cv.UserEmail)
	assert.Nil(t, cv.Session)
}

func TestGetContextValues_WrongTypes(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	// Set wrong types
	c.Set("user_id", 123)             // int instead of string
	c.Set("name", true)               // bool instead of string
	c.Set("email", []byte("test"))    // []byte instead of string
	c.Set("session", "not-a-session") // string instead of *Session

	cv := GetContextValues(c)

	assert.Empty(t, cv.UserID)
	assert.Empty(t, cv.UserName)
	assert.Empty(t, cv.UserEmail)
	assert.Nil(t, cv.Session)
}

// =============================================================================
// MustGetUserID Tests
// =============================================================================

func TestMustGetUserID_Present(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	c.Set("user_id", "user-123")

	result := MustGetUserID(c)

	assert.Equal(t, "user-123", result)
	assert.False(t, c.IsAborted())
}

func TestMustGetUserID_Missing(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	result := MustGetUserID(c)

	assert.Empty(t, result)
	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMustGetUserID_Empty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	c.Set("user_id", "")

	result := MustGetUserID(c)

	assert.Empty(t, result)
	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMustGetUserID_WrongType(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	c.Set("user_id", 12345)

	result := MustGetUserID(c)

	assert.Empty(t, result)
	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// =============================================================================
// RespondWithSuccessOrRedirect Tests
// =============================================================================

func TestRespondWithSuccessOrRedirect_JSONAccept(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Request.Header.Set("Accept", "application/json")

	RespondWithSuccessOrRedirect(c, "Operation successful", "/redirect-path")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Operation successful")
	assert.Contains(t, w.Body.String(), "success")
}

func TestRespondWithSuccessOrRedirect_HTMLAccept(t *testing.T) {
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		RespondWithSuccessOrRedirect(c, "Operation successful", "/redirect-path")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Accept", "text/html")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/redirect-path", w.Header().Get("Location"))
}

func TestRespondWithSuccessOrRedirect_NoAccept(t *testing.T) {
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		RespondWithSuccessOrRedirect(c, "Operation successful", "/redirect-path")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	router.ServeHTTP(w, req)

	// Default behavior is redirect
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/redirect-path", w.Header().Get("Location"))
}

// =============================================================================
// AdminPageData Tests
// =============================================================================

func TestBuildAdminPageData(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	c.Set("name", "Admin User")
	c.Set("email", "admin@example.com")

	workspaceNames := map[string]string{"ws-1": "Workspace One"}

	data := buildAdminPageData(c, "dashboard", "Dashboard", "Overview", workspaceNames)

	assert.Equal(t, "dashboard", data.ActivePage)
	assert.Equal(t, "Dashboard", data.PageTitle)
	assert.Equal(t, "Overview", data.PageSubtitle)
	assert.Equal(t, "Admin User", data.UserName)
	assert.Equal(t, "admin@example.com", data.UserEmail)
	assert.Equal(t, workspaceNames, data.WorkspaceNames)
}
