package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestValidateUUIDParam_ValidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/resource/:id", func(c *gin.Context) {
		id := ValidateUUIDParam(c, "id")
		if id == "" {
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id})
	})

	// Valid UUID
	req := httptest.NewRequest("GET", "/resource/019a9388-2d96-7340-8848-31952671d2c1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "019a9388-2d96-7340-8848-31952671d2c1")
}

func TestValidateUUIDParam_EmptyParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/resource/", nil)
	// No param set

	id := ValidateUUIDParam(c, "id")

	assert.Empty(t, id)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "id is required")
}

func TestValidateUUIDParam_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/resource/:id", func(c *gin.Context) {
		id := ValidateUUIDParam(c, "id")
		if id == "" {
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id})
	})

	tests := []struct {
		name  string
		value string
	}{
		{"random string", "not-a-uuid"},
		{"too short", "12345"},
		{"wrong format", "xyz-123-456"},
		{"too long", "019a9388-2d96-7340-8848-31952671d2c1-extra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/resource/"+tt.value, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "must be a valid UUID")
		})
	}
}

func TestValidateUUIDParam_DifferentParamNames(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		paramName string
		route     string
		url       string
	}{
		{"case_id param", "case_id", "/cases/:case_id", "/cases/019a9388-2d96-7340-8848-31952671d2c1"},
		{"workspace_id param", "workspace_id", "/ws/:workspace_id", "/ws/019a9388-2d96-7340-8848-31952671d2c1"},
		{"user_id param", "user_id", "/users/:user_id", "/users/019a9388-2d96-7340-8848-31952671d2c1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET(tt.route, func(c *gin.Context) {
				id := ValidateUUIDParam(c, tt.paramName)
				if id == "" {
					return
				}
				c.JSON(http.StatusOK, gin.H{"id": id})
			})

			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestValidateUUIDParam_AbortsOnFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handlerCalled := false
	router.GET("/resource/:id", func(c *gin.Context) {
		id := ValidateUUIDParam(c, "id")
		if id == "" {
			return
		}
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"id": id})
	})

	req := httptest.NewRequest("GET", "/resource/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, handlerCalled, "Handler should not continue after validation failure")
}
