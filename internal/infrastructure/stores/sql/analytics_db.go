package sql

import (
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

const webAnalyticsSchemaName = "ext_demandops_web_analytics"

// AnalyticsDB wraps the analytics database connection.
// Analytics now uses the shared PostgreSQL instance and writes into the
// extension-owned ext_demandops_web_analytics schema.
type AnalyticsDB struct {
	db     *sql.DB
	sqlx   *sqlx.DB
	schema string
}

// NewAnalyticsDB opens the shared PostgreSQL database for analytics access.
func NewAnalyticsDB(dsn string) (*AnalyticsDB, error) {
	if dsn == "" {
		dsn = getEnvOrDefault("DATABASE_DSN", "")
	}
	if dsn == "" {
		return nil, fmt.Errorf("analytics database requires DATABASE_DSN")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open analytics database: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping analytics database: %w", err)
	}

	sqlxDB := sqlx.NewDb(db, "postgres")
	analyticsDB := &AnalyticsDB{
		db:     db,
		sqlx:   sqlxDB,
		schema: webAnalyticsSchemaName,
	}
	if err := analyticsDB.ensureSchemaAvailable(); err != nil {
		db.Close()
		return nil, err
	}

	return analyticsDB, nil
}

func (a *AnalyticsDB) ensureSchemaAvailable() error {
	var regclass sql.NullString
	if err := a.sqlx.Get(&regclass, `SELECT to_regclass($1)`, a.schema+".properties"); err != nil {
		return fmt.Errorf("check analytics schema availability: %w", err)
	}
	if !regclass.Valid || regclass.String == "" {
		return fmt.Errorf("analytics extension schema %s is not available", a.schema)
	}
	return nil
}

// Sqlx returns the sqlx.DB wrapper.
func (a *AnalyticsDB) Sqlx() *sqlx.DB {
	return a.sqlx
}

// Schema returns the analytics schema name.
func (a *AnalyticsDB) Schema() string {
	return a.schema
}

// Close closes the analytics database connection.
func (a *AnalyticsDB) Close() error {
	return a.db.Close()
}
