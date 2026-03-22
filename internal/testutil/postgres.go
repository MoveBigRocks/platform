package testutil

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	sqlstore "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
)

var postgresTestDBCounter atomic.Int64

// SetupTestPostgresDatabase creates an isolated PostgreSQL database and returns
// its DSN plus a cleanup function.
func SetupTestPostgresDatabase(t testing.TB) (string, func()) {
	t.Helper()
	return setupTestPostgresDatabase(t)
}

// SetupTestPostgresStore creates an isolated PostgreSQL-backed store.
// It uses TEST_DATABASE_ADMIN_DSN when set, otherwise defaults to the local
// Postgres.app-style server on 127.0.0.1 for the current user.
func SetupTestPostgresStore(t testing.TB) (stores.Store, func()) {
	t.Helper()

	testDSN, cleanupDatabase := setupTestPostgresDatabase(t)

	db, err := sqlstore.NewDBWithConfig(sqlstore.DBConfig{
		DSN: testDSN,
	})
	if err != nil {
		cleanupDatabase()
		t.Fatalf("open postgres test database: %v", err)
	}

	store, err := sqlstore.NewStore(db)
	if err != nil {
		db.Close()
		cleanupDatabase()
		t.Fatalf("create postgres test store: %v", err)
	}

	cleanup := func() {
		if err := store.Close(); err != nil {
			t.Logf("warning: failed to close postgres test store: %v", err)
		}
		cleanupDatabase()
	}

	return store, cleanup
}

func setupTestPostgresDatabase(t testing.TB) (string, func()) {
	t.Helper()

	adminDSN := testPostgresAdminDSN(t)
	dbName := testPostgresDatabaseName()
	testDSN, err := postgresDSNWithDatabase(adminDSN, dbName)
	if err != nil {
		t.Fatalf("build test postgres dsn: %v", err)
	}

	adminDB, err := stdsql.Open("postgres", adminDSN)
	if err != nil {
		t.Fatalf("open postgres admin database: %v", err)
	}
	adminDB.SetConnMaxLifetime(30 * time.Second)
	adminDB.SetMaxOpenConns(2)
	adminDB.SetMaxIdleConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := adminDB.PingContext(ctx); err != nil {
		adminDB.Close()
		t.Skipf("postgres admin database unavailable: %v", err)
	}
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		adminDB.Close()
		t.Fatalf("create postgres test database %s: %v", dbName, err)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName)); err != nil {
			t.Logf("warning: failed to drop postgres test database %s: %v", dbName, err)
		}
		if err := adminDB.Close(); err != nil {
			t.Logf("warning: failed to close postgres admin connection: %v", err)
		}
	}

	return testDSN, cleanup
}

func testPostgresAdminDSN(t testing.TB) string {
	t.Helper()

	if dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_ADMIN_DSN")); dsn != "" {
		return dsn
	}

	user := strings.TrimSpace(os.Getenv("USER"))
	if user == "" {
		t.Skip("postgres test harness requires TEST_DATABASE_ADMIN_DSN or USER")
	}
	return fmt.Sprintf("postgres://%s@127.0.0.1:5432/postgres?sslmode=disable", url.QueryEscape(user))
}

func testPostgresDatabaseName() string {
	return fmt.Sprintf("mbr_test_%d_%d", time.Now().UnixNano(), postgresTestDBCounter.Add(1))
}

func postgresDSNWithDatabase(adminDSN, databaseName string) (string, error) {
	parsed, err := url.Parse(adminDSN)
	if err != nil {
		return "", err
	}
	parsed.Path = "/" + databaseName
	return parsed.String(), nil
}
