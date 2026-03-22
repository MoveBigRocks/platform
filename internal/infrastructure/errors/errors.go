package errors

import (
	"fmt"
	"net/http"
)

// ErrorType represents the type of error
type ErrorType string

const (
	ErrorTypeValidation     ErrorType = "validation"
	ErrorTypeAuthentication ErrorType = "authentication"
	ErrorTypeAuthorization  ErrorType = "authorization"
	ErrorTypeNotFound       ErrorType = "not_found"
	ErrorTypeConflict       ErrorType = "conflict"
	ErrorTypeInternal       ErrorType = "internal"
	ErrorTypeExternal       ErrorType = "external"
	ErrorTypeRateLimit      ErrorType = "rate_limit"
	ErrorTypeBadRequest     ErrorType = "bad_request"
	ErrorTypeTimeout        ErrorType = "timeout"
	ErrorTypeUnavailable    ErrorType = "unavailable"
)

// APIError represents a structured error response
type APIError struct {
	Type       ErrorType              `json:"type"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Code       string                 `json:"code,omitempty"`
	StatusCode int                    `json:"-"`
	Cause      error                  `json:"-"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *APIError) Unwrap() error {
	return e.Cause
}

// New creates a new APIError
func New(errorType ErrorType, message string) *APIError {
	return &APIError{
		Type:       errorType,
		Message:    message,
		StatusCode: getStatusCodeForType(errorType),
	}
}

// Newf creates a new APIError with formatted message
func Newf(errorType ErrorType, format string, args ...interface{}) *APIError {
	return &APIError{
		Type:       errorType,
		Message:    fmt.Sprintf(format, args...),
		StatusCode: getStatusCodeForType(errorType),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errorType ErrorType, message string) *APIError {
	return &APIError{
		Type:       errorType,
		Message:    message,
		Cause:      err,
		StatusCode: getStatusCodeForType(errorType),
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, errorType ErrorType, format string, args ...interface{}) *APIError {
	return &APIError{
		Type:       errorType,
		Message:    fmt.Sprintf(format, args...),
		Cause:      err,
		StatusCode: getStatusCodeForType(errorType),
	}
}

// WithDetails adds details to an APIError
func (e *APIError) WithDetails(details map[string]interface{}) *APIError {
	e.Details = details
	return e
}

// WithCode adds an error code to an APIError
func (e *APIError) WithCode(code string) *APIError {
	e.Code = code
	return e
}

// WithStatusCode overrides the status code
func (e *APIError) WithStatusCode(statusCode int) *APIError {
	e.StatusCode = statusCode
	return e
}

// getStatusCodeForType maps error types to HTTP status codes
func getStatusCodeForType(errorType ErrorType) int {
	switch errorType {
	case ErrorTypeValidation:
		return http.StatusBadRequest
	case ErrorTypeAuthentication:
		return http.StatusUnauthorized
	case ErrorTypeAuthorization:
		return http.StatusForbidden
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeConflict:
		return http.StatusConflict
	case ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case ErrorTypeBadRequest:
		return http.StatusBadRequest
	case ErrorTypeTimeout:
		return http.StatusRequestTimeout
	case ErrorTypeUnavailable:
		return http.StatusServiceUnavailable
	case ErrorTypeExternal:
		return http.StatusBadGateway
	case ErrorTypeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// Predefined errors for common cases
var (
	ErrNotFound           = New(ErrorTypeNotFound, "Resource not found")
	ErrUnauthorized       = New(ErrorTypeAuthentication, "Authentication required")
	ErrForbidden          = New(ErrorTypeAuthorization, "Access forbidden")
	ErrValidationFailed   = New(ErrorTypeValidation, "Validation failed")
	ErrInternalServer     = New(ErrorTypeInternal, "Internal server error")
	ErrBadRequest         = New(ErrorTypeBadRequest, "Bad request")
	ErrConflict           = New(ErrorTypeConflict, "Resource conflict")
	ErrRateLimit          = New(ErrorTypeRateLimit, "Rate limit exceeded")
	ErrTimeout            = New(ErrorTypeTimeout, "Request timeout")
	ErrServiceUnavailable = New(ErrorTypeUnavailable, "Service unavailable")
)

// Validation errors
type ValidationError struct {
	Field   string      `json:"field"`
	Value   interface{} `json:"value,omitempty"`
	Message string      `json:"message"`
	Code    string      `json:"code,omitempty"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

// NewValidationError creates a new validation error for a specific field
func NewValidationError(field, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewValidationErrors creates ValidationErrors from multiple field errors
func NewValidationErrors(errors ...ValidationError) *APIError {
	return &APIError{
		Type:       ErrorTypeValidation,
		Message:    "Validation failed",
		StatusCode: http.StatusBadRequest,
		Details: map[string]interface{}{
			"validation_errors": errors,
		},
	}
}

// Helper functions for common error patterns

// NotFoundError creates a not found error with resource info
func NotFoundError(resource, id string) *APIError {
	return Newf(ErrorTypeNotFound, "%s not found", resource).
		WithCode("RESOURCE_NOT_FOUND").
		WithDetails(map[string]interface{}{
			"resource": resource,
			"id":       id,
		})
}

// DatabaseError creates a database-related error
func DatabaseError(operation string, err error) *APIError {
	return Wrapf(err, ErrorTypeInternal, "Database %s failed", operation).
		WithCode("DATABASE_ERROR").
		WithDetails(map[string]interface{}{
			"operation": operation,
		})
}

// Is implements errors.Is for APIError.
// It returns true if the target is an APIError with the same type.
func (e *APIError) Is(target error) bool {
	t, ok := target.(*APIError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}
