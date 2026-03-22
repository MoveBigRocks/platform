package sql

import (
	"database/sql"
	"testing"
)

func TestNewSqlxDBRebindsForPostgres(t *testing.T) {
	sqlxDB := NewSqlxDB(&sql.DB{}, "postgres")

	got := sqlxDB.Rebind("SELECT * FROM core_identity.users WHERE id = ? AND email = ?")
	want := "SELECT * FROM core_identity.users WHERE id = $1 AND email = $2"
	if got != want {
		t.Fatalf("unexpected rebound query: got %q want %q", got, want)
	}
}

func TestNewSqlxDBDefaultsToPostgresRebinding(t *testing.T) {
	sqlxDB := NewSqlxDB(&sql.DB{}, "")

	got := sqlxDB.Rebind("SELECT * FROM core_identity.users WHERE id = ? AND email = ?")
	want := "SELECT * FROM core_identity.users WHERE id = $1 AND email = $2"
	if got != want {
		t.Fatalf("unexpected rebound query: got %q want %q", got, want)
	}
}
