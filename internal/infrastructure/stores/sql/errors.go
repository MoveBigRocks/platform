package sql

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
)

// =============================================================================
// Error Translation
// =============================================================================

// TranslateSqlxError converts database/sql and SQLite errors to domain-appropriate store errors.
func TranslateSqlxError(err error, table string) error {
	if err == nil {
		return nil
	}

	// sql.ErrNoRows -> not found
	if errors.Is(err, sql.ErrNoRows) {
		return shared.ErrNotFound
	}

	// SQLite-specific constraint errors (parsed from error string)
	errStr := err.Error()

	// SQLite: UNIQUE constraint failed
	if strings.Contains(errStr, "UNIQUE constraint failed") {
		field := extractFieldFromSqliteUniqueError(errStr)
		return shared.NewUniqueViolation(table, field, nil)
	}

	// SQLite: FOREIGN KEY constraint failed
	if strings.Contains(errStr, "FOREIGN KEY constraint failed") {
		return shared.NewForeignKeyViolation(table, "", nil)
	}

	// SQLite: NOT NULL constraint failed
	if strings.Contains(errStr, "NOT NULL constraint failed") {
		field := extractFieldFromSqliteNotNullError(errStr)
		return shared.NewNotNullViolation(table, field)
	}

	// SQLite: CHECK constraint failed
	if strings.Contains(errStr, "CHECK constraint failed") {
		return shared.NewCheckViolation(table, "", nil)
	}

	// Database connection errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no connection") ||
		strings.Contains(errStr, "server closed the connection unexpectedly") ||
		strings.Contains(errStr, "relation") && strings.Contains(errStr, "does not exist") {
		return shared.ErrDatabaseUnavailable
	}

	return err
}

// extractFieldFromSqliteUniqueError parses SQLite unique constraint errors
// Format: UNIQUE constraint failed: table.column
func extractFieldFromSqliteUniqueError(errStr string) string {
	// SQLite format: "UNIQUE constraint failed: table.column"
	if idx := strings.Index(errStr, "UNIQUE constraint failed:"); idx != -1 {
		remainder := errStr[idx+len("UNIQUE constraint failed:"):]
		remainder = strings.TrimSpace(remainder)
		// Extract field from "table.column" format
		if dotIdx := strings.LastIndex(remainder, "."); dotIdx != -1 {
			return strings.TrimSpace(remainder[dotIdx+1:])
		}
	}
	return ""
}

// extractFieldFromSqliteNotNullError parses SQLite not-null constraint errors
// Format: NOT NULL constraint failed: table.column
func extractFieldFromSqliteNotNullError(errStr string) string {
	if idx := strings.Index(errStr, "NOT NULL constraint failed:"); idx != -1 {
		remainder := errStr[idx+len("NOT NULL constraint failed:"):]
		remainder = strings.TrimSpace(remainder)
		// Extract field from "table.column" format
		if dotIdx := strings.LastIndex(remainder, "."); dotIdx != -1 {
			return strings.TrimSpace(remainder[dotIdx+1:])
		}
	}
	return ""
}
