package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	_ "modernc.org/sqlite"
)

func TestStatementDatabase(t *testing.T) {
	tests := []struct {
		id   database.StatementID
		name string
	}{
		{id: "LOGIN_SEL_REALMLIST", name: "auth"},
		{id: "CHAR_SEL_CHARACTER", name: "characters"},
		{id: "WORLD_SEL_COMMANDS", name: "world"},
	}
	for _, test := range tests {
		name, err := statementDatabase(test.id)
		if err != nil {
			t.Fatalf("statementDatabase(%q) failed: %v", test.id, err)
		}
		if name != test.name {
			t.Errorf("statementDatabase(%q) = %q, want %q", test.id, name, test.name)
		}
	}
	if _, err := statementDatabase("UNKNOWN"); err == nil {
		t.Fatal("statementDatabase accepted an unmapped statement")
	}
}

func TestAuditStatementDefinitions(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite memory database: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE account (id INTEGER, username TEXT)"); err != nil {
		t.Fatalf("failed to create statement fixture: %v", err)
	}
	definitions := []database.StatementDefinition{{ID: "LOGIN_SEL_ACCOUNT_ID_BY_NAME"}}
	prepared, failures := auditStatementDefinitions(context.Background(), db, database.BackendSQLite, definitions)
	if prepared != 1 || len(failures) != 0 {
		t.Fatalf("audit prepared=%d failures=%v, want one prepared statement", prepared, failures)
	}
}

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

func TestSchemaDifferencesDetectDefinitionDrift(t *testing.T) {
	left := schemaInventory{Objects: map[string]string{"auth/TABLE/account": "CREATE TABLE account (id INTEGER)"}}
	right := schemaInventory{Objects: map[string]string{"auth/TABLE/account": "CREATE TABLE account (id TEXT)", "auth/TABLE/extra": "CREATE TABLE extra (id INTEGER)"}}
	missing, extra, mismatched := schemaDifferences(left, right)
	if len(missing) != 0 || len(extra) != 1 || len(mismatched) != 1 || extra[0] != "auth/TABLE/extra" || mismatched[0] != "auth/TABLE/account" {
		t.Fatalf("missing=%v extra=%v mismatched=%v", missing, extra, mismatched)
	}
}

func TestSchemaIndexComparisonIgnoresDialectIndexNames(t *testing.T) {
	left := schemaInventory{Objects: map[string]string{"auth/INDEX/account/username": "INDEX/account/username"}}
	right := schemaInventory{Objects: map[string]string{"auth/INDEX/account/username": "INDEX/account/username"}}
	missing, extra, mismatched := schemaDifferences(left, right)
	if len(missing) != 0 || len(extra) != 0 || len(mismatched) != 0 {
		t.Fatalf("missing=%v extra=%v mismatched=%v", missing, extra, mismatched)
	}
}

func TestExerciseStatementRollsBackAndReportsResultShape(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE account (id INTEGER, username TEXT); INSERT INTO account VALUES (7, 'Tester')"); err != nil {
		t.Fatal(err)
	}
	fields, queried, err := exerciseStatement(context.Background(), db, database.StatementDefinition{ID: "LOGIN_SEL_ACCOUNT_ID_BY_NAME"})
	if err != nil || !queried || fields != 1 {
		t.Fatalf("fields=%d queried=%v err=%v", fields, queried, err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM account").Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback changed fixture count=%d err=%v", count, err)
	}
}
