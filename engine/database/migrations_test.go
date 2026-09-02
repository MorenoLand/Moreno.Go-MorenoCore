package database

import (
	"context"
	"database/sql"
	"testing"
)

func TestApplyMigrationsIsOrderedAndIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	store := &Store{Name: "auth", Backend: BackendSQLite, DB: db}
	migrations := []Migration{{Version: 1, Name: "create", Statements: []string{"CREATE TABLE sample (id INTEGER PRIMARY KEY)"}}, {Version: 2, Name: "insert", Statements: []string{"INSERT INTO sample (id) VALUES (1)"}}}
	if err := ApplyMigrations(context.Background(), store, migrations); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(context.Background(), store, migrations); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sample").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("rows=%d", count)
	}
}
