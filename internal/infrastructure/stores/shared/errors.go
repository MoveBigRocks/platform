package shared

import (
	"errors"
	"fmt"

	"github.com/movebigrocks/platform/internal/shared/contracts"
)

// =============================================================================
// Base Store Errors
// =============================================================================

var (
	// ErrNotFound indicates the requested resource does not exist
	ErrNotFound = errors.New("not found")

	// ErrDuplicate indicates a duplicate record (generic)
	ErrDuplicate = errors.New("duplicate")

	// ErrAlreadyUsed indicates a one-time resource has already been consumed.
	// The canonical sentinel lives in contracts and is re-exported here.
	ErrAlreadyUsed = contracts.ErrAlreadyUsed

	// ErrInvalidInput indicates malformed or invalid input data
	ErrInvalidInput = errors.New("invalid input")

	// ErrOptimisticLock indicates a concurrent modification conflict
	ErrOptimisticLock = errors.New("optimistic lock failed")

	// ErrDatabaseUnavailable indicates the database is unreachable
	ErrDatabaseUnavailable = errors.New("database unavailable")
)

// =============================================================================
// Constraint Violation Errors
// =============================================================================

var (
	// ErrUniqueViolation indicates a unique constraint was violated
	ErrUniqueViolation = errors.New("unique constraint violation")

	// ErrForeignKeyViolation indicates a foreign key constraint was violated
	ErrForeignKeyViolation = errors.New("foreign key constraint violation")

	// ErrNotNullViolation indicates a not null constraint was violated
	ErrNotNullViolation = errors.New("not null constraint violation")

	// ErrCheckViolation indicates a check constraint was violated
	ErrCheckViolation = errors.New("check constraint violation")
)

// ConstraintError provides context about a database constraint violation
type ConstraintError struct {
	Constraint string      // "unique", "foreign_key", "not_null", "check"
	Table      string      // The database table
	Field      string      // The field that violated the constraint
	Value      interface{} // The value that caused the violation
	Err        error       // The underlying sentinel error
}

func (e *ConstraintError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s constraint violation on %s.%s", e.Constraint, e.Table, e.Field)
	}
	if e.Table != "" {
		return fmt.Sprintf("%s constraint violation on %s", e.Constraint, e.Table)
	}
	return fmt.Sprintf("%s constraint violation", e.Constraint)
}

func (e *ConstraintError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is for ConstraintError
func (e *ConstraintError) Is(target error) bool {
	// Check if target is the underlying sentinel
	if errors.Is(e.Err, target) {
		return true
	}
	// Check if target is another ConstraintError with same constraint type
	t, ok := target.(*ConstraintError)
	if ok {
		return e.Constraint == t.Constraint
	}
	return false
}

// =============================================================================
// Constraint Error Constructors
// =============================================================================

// NewUniqueViolation creates a unique constraint violation error
func NewUniqueViolation(table, field string, value interface{}) *ConstraintError {
	return &ConstraintError{
		Constraint: "unique",
		Table:      table,
		Field:      field,
		Value:      value,
		Err:        ErrUniqueViolation,
	}
}

// NewForeignKeyViolation creates a foreign key constraint violation error
func NewForeignKeyViolation(table, field string, value interface{}) *ConstraintError {
	return &ConstraintError{
		Constraint: "foreign_key",
		Table:      table,
		Field:      field,
		Value:      value,
		Err:        ErrForeignKeyViolation,
	}
}

// NewNotNullViolation creates a not null constraint violation error
func NewNotNullViolation(table, field string) *ConstraintError {
	return &ConstraintError{
		Constraint: "not_null",
		Table:      table,
		Field:      field,
		Err:        ErrNotNullViolation,
	}
}

// NewCheckViolation creates a check constraint violation error
func NewCheckViolation(table, field string, value interface{}) *ConstraintError {
	return &ConstraintError{
		Constraint: "check",
		Table:      table,
		Field:      field,
		Value:      value,
		Err:        ErrCheckViolation,
	}
}

// =============================================================================
// Error Predicates
// =============================================================================

// IsAlreadyUsed checks if an error indicates a one-time resource was already consumed.
// It delegates to the canonical helper in contracts.
var IsAlreadyUsed = contracts.IsAlreadyUsed
