package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDatabaseStats(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "stats_test.db")
	db, err := sql.Open("sqlite", tempDB)
	if err != nil {
		t.Fatalf("failed to open sqlite temp db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT);
		INSERT INTO test_table (name) VALUES ('val1'), ('val2');
		CREATE VIEW test_view AS SELECT id FROM test_table;
	`)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	tables, views, rows, err := databaseStats(db)
	if err != nil {
		t.Fatalf("databaseStats failed: %v", err)
	}

	if tables != 1 {
		t.Errorf("expected 1 table, got %d", tables)
	}
	if views != 1 {
		t.Errorf("expected 1 view, got %d", views)
	}
	if rows != 2 {
		t.Errorf("expected 2 rows, got %d", rows)
	}
}
