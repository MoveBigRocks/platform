package platformhandlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminHandler_GrafanaAccessTokenValidation(t *testing.T) {
	authHandler := NewAuthHandler(nil, "http://localhost:8080", "development", nil, "")
	handler := NewAdminHandler("", authHandler, nil, nil, nil, "http://localhost:8080", true, "test-secret", nil)

	now := time.Unix(1700000000, 0)
	token := handler.createGrafanaAccessToken(now)
	assert.True(t, handler.validateGrafanaAccessToken(token, now))
	assert.True(t, handler.validateGrafanaAccessToken(token, now.Add(grafanaAccessCookieTTL-time.Second)))
	assert.False(t, handler.validateGrafanaAccessToken(token, now.Add(grafanaAccessCookieTTL+time.Second)))
	assert.False(t, handler.validateGrafanaAccessToken(token+"tampered", now))
}

func TestAdminHandler_HasGrafanaAccess_FromCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authHandler := NewAuthHandler(nil, "http://localhost:8080", "development", nil, "")
	handler := NewAdminHandler("", authHandler, nil, nil, nil, "http://localhost:8080", true, "test-secret", nil)

	token := handler.createGrafanaAccessToken(time.Now())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/grafana/status", nil)
	req.AddCookie(&http.Cookie{Name: grafanaAccessCookieName, Value: token})
	c.Request = req

	assert.True(t, handler.hasGrafanaAccess(c))
}

func TestAdminHandler_GrafanaHealthCheck_RequiresAccessCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authHandler := NewAuthHandler(nil, "http://localhost:8080", "development", nil, "")
	handler := NewAdminHandler("http://127.0.0.1:3000", authHandler, nil, nil, nil, "http://localhost:8080", true, "test-secret", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/grafana-status", nil)
	handler.GrafanaHealthCheck(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminHandler_Logout_ClearsCookieWithConfiguredDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authHandler := NewAuthHandler(
		nil,
		"http://localhost:8080",
		"production",
		nil,
		".example.com",
	)
	handler := NewAdminHandler(
		"",
		authHandler,
		nil,
		nil,
		nil,
		"http://localhost:8080",
		false,
		"test-grafana-secret",
		nil,
	)

	router := gin.New()
	router.GET("/logout", handler.Logout)

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))

	var sessionCookie *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "mbr_session" {
			sessionCookie = cookie
			break
		}
	}
	require.NotNil(t, sessionCookie)
	assert.Equal(t, "", sessionCookie.Value)
	assert.Equal(t, "example.com", sessionCookie.Domain)
	assert.Less(t, sessionCookie.MaxAge, 0)
	assert.True(t, sessionCookie.HttpOnly)
	assert.True(t, sessionCookie.Secure)
}
