package sql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
)

// DB wraps the database connection
type DB struct {
	sqlDB  *sql.DB
	driver string
}

// DBConfig holds database connection configuration
type DBConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// NewDB creates a new PostgreSQL database connection.
func NewDB(dsn string) (*DB, error) {
	if dsn == "" {
		dsn = getEnvOrDefault("DATABASE_DSN", "")
	}
	return NewDBWithConfig(DBConfig{DSN: dsn})
}

// getEnvOrDefault returns the environment variable value or the default
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// NewDBFromConfig creates a new database connection from config.
func NewDBFromConfig(cfg config.DatabaseConfig) (*DB, error) {
	return NewDBWithConfig(DBConfig{
		DSN: cfg.EffectiveDSN(),
	})
}

// NewDBWithConfig creates a new database connection with explicit config.
func NewDBWithConfig(cfg DBConfig) (*DB, error) {
	maxOpenConns := cfg.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 1
	}

	maxIdleConns := cfg.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = maxOpenConns
	}

	connMaxLifetime := cfg.ConnMaxLifetime
	connMaxIdleTime := cfg.ConnMaxIdleTime

	if cfg.DSN == "" {
		return nil, fmt.Errorf("postgres connection requires DSN")
	}
	if !strings.HasPrefix(cfg.DSN, "postgres://") && !strings.HasPrefix(cfg.DSN, "postgresql://") {
		return nil, fmt.Errorf("postgres connection requires postgres:// or postgresql:// DSN")
	}

	sqlDB, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	if connMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := runPostgresMigrations(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("run postgres migrations: %w", err)
	}

	return &DB{sqlDB: sqlDB, driver: "postgres"}, nil
}

// Driver returns the database driver type
func (db *DB) Driver() string {
	return db.driver
}

// GetSQLDB returns the underlying *sql.DB connection
func (db *DB) GetSQLDB() (*sql.DB, error) {
	return db.sqlDB, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.sqlDB.Close()
}

// Ping verifies the database connection is alive
func (db *DB) Ping(ctx context.Context) error {
	return db.sqlDB.PingContext(ctx)
}

// Checkpoint is not supported for PostgreSQL-backed runtimes.
func (db *DB) Checkpoint(ctx context.Context, mode string) (int, int, error) {
	return 0, 0, fmt.Errorf("checkpoint is not supported for postgres")
}

// IntegrityCheck is not supported for PostgreSQL-backed runtimes.
func (db *DB) IntegrityCheck(ctx context.Context) error {
	return fmt.Errorf("integrity check is not supported for postgres")
}
