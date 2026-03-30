package shareddomain

import (
	"fmt"
	"regexp"
)

// Validator is an interface for event types that can validate themselves
type Validator interface {
	Validate() error
}

// ValidationError represents a validation failure for a specific field
type ValidationError struct {
	Field    string
	Value    interface{}
	Expected []string
}

// Error implements the error interface
func (e ValidationError) Error() string {
	if len(e.Expected) == 0 {
		return fmt.Sprintf("validation failed for field %q: got %v", e.Field, e.Value)
	}
	if len(e.Expected) == 1 {
		return fmt.Sprintf("validation failed for field %q: got %v, expected %s", e.Field, e.Value, e.Expected[0])
	}
	return fmt.Sprintf("validation failed for field %q: got %v, expected one of %v", e.Field, e.Value, e.Expected)
}

// ErrInvalidField creates a ValidationError for an invalid field value
func ErrInvalidField(field string, value interface{}, expected []string) error {
	return ValidationError{
		Field:    field,
		Value:    value,
		Expected: expected,
	}
}

// ErrRequiredField creates a ValidationError for a missing required field
func ErrRequiredField(field string) error {
	return ValidationError{
		Field:    field,
		Expected: []string{"non-empty"},
	}
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// validateEnum checks if a value is in a list of valid options
func validateEnum(field string, value string, validOptions []string) error {
	if !contains(validOptions, value) {
		return ErrInvalidField(field, value, validOptions)
	}
	return nil
}

// validateNonEmpty checks if a string is non-empty
func validateNonEmpty(field string, value string) error {
	if value == "" {
		return ErrRequiredField(field)
	}
	return nil
}

// Email validation regex (RFC 5322 simplified)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// validateEmail validates an email address format
func validateEmail(field string, value string) error {
	if value == "" {
		return nil // Optional field, use validateNonEmpty for required emails
	}

	// Basic length check
	if len(value) > 254 {
		return ErrInvalidField(field, value, []string{"valid email address (max 254 chars)"})
	}

	if !emailRegex.MatchString(value) {
		return ErrInvalidField(field, value, []string{"valid email address"})
	}

	return nil
}

// validateEmailRequired validates a required email address
func validateEmailRequired(field string, value string) error {
	if err := validateNonEmpty(field, value); err != nil {
		return err
	}
	return validateEmail(field, value)
}

// validatePositiveInt validates that an integer is positive (> 0)
func validatePositiveInt(field string, value int) error {
	if value <= 0 {
		return ErrInvalidField(field, value, []string{"positive integer"})
	}
	return nil
}

// validateNonNegativeInt validates that an integer is non-negative (>= 0)
func validateNonNegativeInt(field string, value int) error {
	if value < 0 {
		return ErrInvalidField(field, value, []string{"non-negative integer"})
	}
	return nil
}

// ============================================================================
// Typed Enum Validators
// ============================================================================

var validCasePriorities = map[CasePriority]bool{
	CasePriorityLow:    true,
	CasePriorityMedium: true,
	CasePriorityHigh:   true,
	CasePriorityUrgent: true,
}

var validCaseChannels = map[CaseChannel]bool{
	CaseChannelEmail:    true,
	CaseChannelWeb:      true,
	CaseChannelAPI:      true,
	CaseChannelPhone:    true,
	CaseChannelChat:     true,
	CaseChannelInternal: true,
}

var validCaseStatuses = map[CaseStatus]bool{
	CaseStatusNew:      true,
	CaseStatusOpen:     true,
	CaseStatusPending:  true,
	CaseStatusResolved: true,
	CaseStatusClosed:   true,
	CaseStatusSpam:     true,
}

// validateCasePriority validates a CasePriority enum value
func validateCasePriority(field string, value CasePriority) error {
	if value == "" {
		return ErrRequiredField(field)
	}
	if !validCasePriorities[value] {
		return ErrInvalidField(field, string(value), []string{"low", "medium", "high", "urgent"})
	}
	return nil
}

// validateCaseChannel validates a CaseChannel enum value
func validateCaseChannel(field string, value CaseChannel) error {
	if value == "" {
		return ErrRequiredField(field)
	}
	if !validCaseChannels[value] {
		return ErrInvalidField(field, string(value), []string{"email", "web", "api", "phone", "chat", "internal"})
	}
	return nil
}

// validateCaseStatus validates a CaseStatus enum value
func validateCaseStatus(field string, value CaseStatus) error {
	if value == "" {
		return ErrRequiredField(field)
	}
	if !validCaseStatuses[value] {
		return ErrInvalidField(field, string(value), []string{"new", "open", "pending", "resolved", "closed", "spam"})
	}
	return nil
}
