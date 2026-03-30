package sql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/metrics"
)

// SlowQueryThreshold defines the duration above which queries trigger a warning log.
// Default is 100ms. Can be modified at runtime if needed.
var SlowQueryThreshold = 100 * time.Millisecond

type sqlxTxKey struct{}

// SqlxQuerier is the interface for sqlx operations that works with both *sqlx.DB and *sqlx.Tx
type SqlxQuerier interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
	QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)
	QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row
}

// SqlxDB wraps a sqlx.DB with transaction support via context injection
type SqlxDB struct {
	*sqlx.DB
	driver string
}

// NewSqlxDB creates a new SqlxDB wrapper from a standard sql.DB
func NewSqlxDB(db *sql.DB, driver string) *SqlxDB {
	driver = strings.TrimSpace(driver)
	if driver == "" {
		driver = "postgres"
	}
	return &SqlxDB{
		DB:     sqlx.NewDb(db, driver),
		driver: driver,
	}
}

// Get returns the transaction from context if present, otherwise returns the DB.
// The returned querier is instrumented to record query metrics.
func (db *SqlxDB) Get(ctx context.Context) SqlxQuerier {
	var querier SqlxQuerier
	if tx, ok := ctx.Value(sqlxTxKey{}).(*sqlx.Tx); ok {
		querier = tx
	} else {
		querier = db.DB
	}
	return &instrumentedQuerier{
		querier: querier,
		rebind:  db.DB.Rebind,
	}
}

// Transaction executes fn within a database transaction
func (db *SqlxDB) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	// Reuse the existing transaction when the caller is already operating
	// inside one. This lets higher-level workflows compose multiple service
	// operations atomically without attempting nested SQLite transactions.
	if _, ok := ctx.Value(sqlxTxKey{}).(*sqlx.Tx); ok {
		return fn(ctx)
	}

	tx, err := db.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, sqlxTxKey{}, tx)

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// instrumentedQuerier wraps a SqlxQuerier to record query metrics
type instrumentedQuerier struct {
	querier SqlxQuerier
	rebind  func(string) string
}

func (iq *instrumentedQuerier) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	query = iq.rebind(query)
	start := time.Now()
	err := iq.querier.GetContext(ctx, dest, query, args...)
	iq.recordMetrics(query, start, 1, err)
	return err
}

func (iq *instrumentedQuerier) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	query = iq.rebind(query)
	start := time.Now()
	err := iq.querier.SelectContext(ctx, dest, query, args...)
	resultCount := 0
	if err == nil {
		// Count results from the slice using reflection
		rv := reflect.ValueOf(dest)
		if rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.Slice {
			resultCount = rv.Elem().Len()
		}
	}
	iq.recordMetrics(query, start, resultCount, err)
	return err
}

func (iq *instrumentedQuerier) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	query = iq.rebind(query)
	start := time.Now()
	result, err := iq.querier.ExecContext(ctx, query, args...)
	rowsAffected := 0
	if err == nil && result != nil {
		if n, e := result.RowsAffected(); e == nil {
			rowsAffected = int(n)
		}
	}
	iq.recordMetrics(query, start, rowsAffected, err)
	return result, err
}

func (iq *instrumentedQuerier) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := iq.querier.NamedExecContext(ctx, query, arg)
	rowsAffected := 0
	if err == nil && result != nil {
		if n, e := result.RowsAffected(); e == nil {
			rowsAffected = int(n)
		}
	}
	iq.recordMetrics(query, start, rowsAffected, err)
	return result, err
}

func (iq *instrumentedQuerier) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	query = iq.rebind(query)
	start := time.Now()
	rows, err := iq.querier.QueryxContext(ctx, query, args...)
	iq.recordMetrics(query, start, 0, err) // Can't know result count until iteration
	return rows, err
}

func (iq *instrumentedQuerier) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	query = iq.rebind(query)
	start := time.Now()
	row := iq.querier.QueryRowxContext(ctx, query, args...)
	// QueryRowxContext doesn't return an error directly; error is deferred to Scan
	iq.recordMetrics(query, start, 1, nil)
	return row
}

func (iq *instrumentedQuerier) recordMetrics(query string, start time.Time, resultCount int, err error) {
	duration := time.Since(start)
	operation, table := parseQueryMeta(query)
	method := getCallerMethod(4) // skip: recordMetrics -> wrapper method -> Get() -> store method

	// Record to Prometheus with both operation/table and method dimensions
	metrics.RecordSQLQuery(operation, table, method, duration.Seconds(), resultCount, err)

	// Log slow queries with full context for debugging
	if duration > SlowQueryThreshold {
		slog.Warn("slow query detected",
			"method", method,
			"operation", operation,
			"table", table,
			"duration_ms", duration.Milliseconds(),
			"results", resultCount,
			"query_preview", truncateQuery(query, 200),
		)
	}
}

// getCallerMethod returns the name of the calling store method.
// skip determines how many stack frames to skip to reach the actual caller.
func getCallerMethod(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}

	// Full name example: "github.com/movebigrocks/platform/internal/cases/stores.(*Store).Get"
	// We want: "stores.Store.Get"
	name := fn.Name()

	// Remove package path prefix, keep last segment
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	// Remove pointer notation: (*Store) -> Store
	name = strings.ReplaceAll(name, "(*", "")
	name = strings.ReplaceAll(name, ")", "")

	return name
}

// truncateQuery truncates a query string for logging, preserving the beginning
func truncateQuery(query string, maxLen int) string {
	// Normalize whitespace
	query = strings.Join(strings.Fields(query), " ")
	if len(query) <= maxLen {
		return query
	}
	return query[:maxLen] + "..."
}

// tableNameRegex matches table names after FROM, INTO, UPDATE, JOIN keywords
var tableNameRegex = regexp.MustCompile(`(?i)(?:FROM|INTO|UPDATE|JOIN)\s+([a-z_][a-z0-9_]*)`)

// parseQueryMeta extracts the operation type and table name from a SQL query
func parseQueryMeta(query string) (operation, table string) {
	query = strings.TrimSpace(query)
	upper := strings.ToUpper(query)

	// Determine operation type
	switch {
	case strings.HasPrefix(upper, "SELECT"):
		operation = "select"
	case strings.HasPrefix(upper, "INSERT"):
		operation = "insert"
	case strings.HasPrefix(upper, "UPDATE"):
		operation = "update"
	case strings.HasPrefix(upper, "DELETE"):
		operation = "delete"
	default:
		operation = "other"
	}

	// Extract table name
	matches := tableNameRegex.FindStringSubmatch(query)
	if len(matches) >= 2 {
		table = strings.ToLower(matches[1])
	} else {
		table = "unknown"
	}

	return operation, table
}
