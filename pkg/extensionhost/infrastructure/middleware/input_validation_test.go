package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestValidatePathParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		path       string
		paramValue string
		wantStatus int
	}{
		{
			name:       "valid path param",
			path:       "/users/:id",
			paramValue: "123e4567-e89b-12d3-a456-426614174000",
			wantStatus: http.StatusOK,
		},
		{
			name:       "path traversal attempt",
			path:       "/files/:name",
			paramValue: "../../../etc/passwd",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "long parameter rejected",
			path:       "/users/:id",
			paramValue: strings.Repeat("a", 1001),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(ValidatePathParams())
			router.GET(tt.path, func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			// Build the actual path with the param value
			actualPath := strings.Replace(tt.path, ":id", tt.paramValue, 1)
			actualPath = strings.Replace(actualPath, ":name", tt.paramValue, 1)

			req := httptest.NewRequest(http.MethodGet, actualPath, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestValidatePathParams_NullByte(t *testing.T) {
	// Note: We can't test null bytes via httptest.NewRequest as it rejects invalid URLs
	// The middleware logic for null byte detection is still valid and will work in production
	// where requests with null bytes would be received via the actual HTTP server.
	// This test verifies the middleware's null byte check logic directly.
	gin.SetMode(gin.TestMode)

	middleware := ValidatePathParams()

	// Create a mock context with a null byte in the param
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "user\x00id"}}

	// Run the middleware
	middleware(c)

	// Should have been aborted with bad request
	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInputValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		method      string
		contentType string
		wantStatus  int
	}{
		{
			name:        "valid JSON content type",
			method:      http.MethodPost,
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "valid form content type",
			method:      http.MethodPost,
			contentType: "application/x-www-form-urlencoded",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "valid multipart content type",
			method:      http.MethodPost,
			contentType: "multipart/form-data; boundary=something",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "GET request - no content type check",
			method:      http.MethodGet,
			contentType: "",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "valid sentry envelope content type",
			method:      http.MethodPost,
			contentType: "application/x-sentry-envelope",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "invalid content type",
			method:      http.MethodPost,
			contentType: "application/xml",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(InputValidation())
			router.Any("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestMaxBodySize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(MaxBodySize(100)) // 100 bytes limit
	router.POST("/test", func(c *gin.Context) {
		_, err := c.GetRawData()
		if err != nil {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})

	t.Run("body within limit", func(t *testing.T) {
		body := strings.NewReader(strings.Repeat("a", 50))
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("body exceeds limit", func(t *testing.T) {
		body := strings.NewReader(strings.Repeat("a", 200))
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})
}
