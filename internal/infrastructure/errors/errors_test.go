package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		errorType      ErrorType
		message        string
		expectedStatus int
	}{
		{"validation error", ErrorTypeValidation, "field is required", http.StatusBadRequest},
		{"authentication error", ErrorTypeAuthentication, "invalid token", http.StatusUnauthorized},
		{"authorization error", ErrorTypeAuthorization, "access denied", http.StatusForbidden},
		{"not found error", ErrorTypeNotFound, "resource not found", http.StatusNotFound},
		{"conflict error", ErrorTypeConflict, "resource already exists", http.StatusConflict},
		{"internal error", ErrorTypeInternal, "unexpected error", http.StatusInternalServerError},
		{"rate limit error", ErrorTypeRateLimit, "too many requests", http.StatusTooManyRequests},
		{"bad request error", ErrorTypeBadRequest, "invalid request", http.StatusBadRequest},
		{"timeout error", ErrorTypeTimeout, "request timeout", http.StatusRequestTimeout},
		{"unavailable error", ErrorTypeUnavailable, "service unavailable", http.StatusServiceUnavailable},
		{"external error", ErrorTypeExternal, "external service error", http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.errorType, tt.message)
			assert.Equal(t, tt.errorType, err.Type)
			assert.Equal(t, tt.message, err.Message)
			assert.Equal(t, tt.expectedStatus, err.StatusCode)
		})
	}
}

func TestNewf(t *testing.T) {
	err := Newf(ErrorTypeNotFound, "%s with ID %s not found", "User", "123")
	assert.Equal(t, "User with ID 123 not found", err.Message)
	assert.Equal(t, ErrorTypeNotFound, err.Type)
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := Wrap(originalErr, ErrorTypeInternal, "wrapped message")

	assert.Equal(t, "wrapped message", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
	assert.Equal(t, originalErr, wrappedErr.Unwrap())
}

func TestWrapf(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := Wrapf(originalErr, ErrorTypeInternal, "operation %s failed", "save")

	assert.Equal(t, "operation save failed", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
}

func TestAPIError_Error(t *testing.T) {
	t.Run("without cause", func(t *testing.T) {
		err := New(ErrorTypeValidation, "field is required")
		assert.Equal(t, "validation: field is required", err.Error())
	})

	t.Run("with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := Wrap(cause, ErrorTypeInternal, "operation failed")
		assert.Contains(t, err.Error(), "operation failed")
		assert.Contains(t, err.Error(), "caused by")
		assert.Contains(t, err.Error(), "underlying error")
	})
}

func TestAPIError_WithDetails(t *testing.T) {
	err := New(ErrorTypeValidation, "validation failed").
		WithDetails(map[string]interface{}{
			"field": "email",
			"value": "invalid-email",
		})

	assert.Equal(t, "email", err.Details["field"])
	assert.Equal(t, "invalid-email", err.Details["value"])
}

func TestAPIError_WithCode(t *testing.T) {
	err := New(ErrorTypeNotFound, "not found").WithCode("RESOURCE_NOT_FOUND")
	assert.Equal(t, "RESOURCE_NOT_FOUND", err.Code)
}

func TestAPIError_WithStatusCode(t *testing.T) {
	err := New(ErrorTypeInternal, "custom error").WithStatusCode(http.StatusTeapot)
	assert.Equal(t, http.StatusTeapot, err.StatusCode)
}

func TestNewValidationError(t *testing.T) {
	ve := NewValidationError("email", "must be a valid email address")
	assert.Equal(t, "email", ve.Field)
	assert.Equal(t, "must be a valid email address", ve.Message)
}

func TestNewValidationErrors(t *testing.T) {
	err := NewValidationErrors(
		NewValidationError("email", "is required"),
		NewValidationError("name", "too short"),
	)

	assert.Equal(t, ErrorTypeValidation, err.Type)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.NotNil(t, err.Details["validation_errors"])
}

func TestNotFoundError(t *testing.T) {
	err := NotFoundError("User", "123")
	assert.Equal(t, ErrorTypeNotFound, err.Type)
	assert.Contains(t, err.Message, "not found")
	assert.Equal(t, "RESOURCE_NOT_FOUND", err.Code)
	assert.Equal(t, "User", err.Details["resource"])
	assert.Equal(t, "123", err.Details["id"])
}

func TestDatabaseError(t *testing.T) {
	cause := errors.New("connection timeout")
	err := DatabaseError("query", cause)
	assert.Equal(t, ErrorTypeInternal, err.Type)
	assert.Contains(t, err.Message, "query")
	assert.Equal(t, "DATABASE_ERROR", err.Code)
	assert.Equal(t, cause, err.Cause)
}

func TestGetStatusCodeForType_Default(t *testing.T) {
	// Test through New which uses getStatusCodeForType
	err := New(ErrorType("unknown"), "test")
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name           string
		err            *APIError
		expectedType   ErrorType
		expectedStatus int
	}{
		{"ErrNotFound", ErrNotFound, ErrorTypeNotFound, http.StatusNotFound},
		{"ErrUnauthorized", ErrUnauthorized, ErrorTypeAuthentication, http.StatusUnauthorized},
		{"ErrForbidden", ErrForbidden, ErrorTypeAuthorization, http.StatusForbidden},
		{"ErrValidationFailed", ErrValidationFailed, ErrorTypeValidation, http.StatusBadRequest},
		{"ErrInternalServer", ErrInternalServer, ErrorTypeInternal, http.StatusInternalServerError},
		{"ErrBadRequest", ErrBadRequest, ErrorTypeBadRequest, http.StatusBadRequest},
		{"ErrConflict", ErrConflict, ErrorTypeConflict, http.StatusConflict},
		{"ErrRateLimit", ErrRateLimit, ErrorTypeRateLimit, http.StatusTooManyRequests},
		{"ErrTimeout", ErrTimeout, ErrorTypeTimeout, http.StatusRequestTimeout},
		{"ErrServiceUnavailable", ErrServiceUnavailable, ErrorTypeUnavailable, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedType, tt.err.Type)
			assert.Equal(t, tt.expectedStatus, tt.err.StatusCode)
		})
	}
}
